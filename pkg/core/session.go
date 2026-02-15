package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/logging"
	"github.com/codesnort/codesnort-swe/pkg/lsp"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

type SweSession struct {
	id            string
	system        *SweSystem
	provider      models.ModelProvider
	providerName  string
	model         string
	messages      []*models.ChatMessage
	role          *conf.AgentRoleConfig
	VFS           vfs.VFS
	LSP           lsp.LSP
	Tools         *tool.ToolRegistry
	outputHandler SessionThreadOutput
	workDir       string
	todoList      []tool.TodoItem
	todoMu        sync.Mutex
	logger        *slog.Logger
	llmLogger     *slog.Logger
	streaming     bool
	// pendingPermissionToolCall stores the tool call that was blocked by a permission query
	// This is used to re-execute the tool after permission is granted
	pendingPermissionToolCalls []*tool.ToolCall
	// pendingToolResponses stores tool responses that were executed before a permission query
	// so they can be sent together after permissions are granted
	pendingToolResponses []*tool.ToolResponse
}

// Prompt adds user prompt to the conversation and starts processing if processing is not already in progress.
// If processing is already in progress, if will be added at the end of conversation after current LLM request is completed,
// its tool calls are executed etc. Returns immediately.
func (s *SweSession) UserPrompt(prompt string) error {
	if s.logger != nil {
		s.logger.Info("user_input", "input", prompt)
	}
	s.messages = append(s.messages, models.NewTextMessage(models.ChatRoleUser, prompt))
	return nil
}

func (s *SweSession) Run(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Debug("session_run_start", "session_id", s.id, "model", s.model)
	}

	chatOptions := s.buildChatOptions()

	chatModel := s.provider.ChatModel(s.model, chatOptions)

	// Build tools list using PromptGenerator.GetToolInfo()
	tools := []tool.ToolInfo{}
	toolNames := s.Tools.List()

	// Get model tags for this model
	tags := s.system.ModelTags.GetTagsForModel(s.providerName, s.model)

	// Get agent state for template processing
	state := s.GetState()

	for _, toolName := range toolNames {
		toolInfo, err := s.system.PromptGenerator.GetToolInfo(tags, toolName, s.role, &state)
		if err != nil {
			// Log warning and skip this tool if description not found
			if s.logger != nil {
				s.logger.Warn("failed to get tool info, skipping tool",
					"tool_name", toolName,
					"error", err,
				)
			}
			continue
		}
		tools = append(tools, toolInfo)
	}

	// Keep processing until the assistant doesn't make any tool calls
	for {
		// Check if there's a pending tool call from a previous permission query
		// This happens when permission was granted and we need to re-execute the blocked tool
		if len(s.pendingPermissionToolCalls) > 0 {
			// Execute pending tool calls with updated permissions
			if err := s.executeToolCalls(s.pendingPermissionToolCalls); err != nil {
				return err
			}
			// After executing the pending tool call, continue to get the next LLM response
			// Don't check for more tool calls in the assistant message since we just executed the pending one
		} else {
			// Check for pending tool calls from previous run (e.g. after permission grant)
			// Only do this if we didn't just execute a pending tool call
			if len(s.messages) > 0 {
				lastMsg := s.messages[len(s.messages)-1]
				if lastMsg.Role == models.ChatRoleAssistant {
					toolCalls := lastMsg.GetToolCalls()
					if len(toolCalls) > 0 {
						// Execute pending tools
						if err := s.executeToolCalls(toolCalls); err != nil {
							return err
						}
					}
				}
			}
		}

		// Use streaming or non-streaming API based on session configuration
		var responseMsg *models.ChatMessage
		var err error

		if s.streaming {
			// Use streaming chat API
			responseMsg, err = s.runStreamingChat(ctx, chatModel, tools, chatOptions)
		} else {
			// Use non-streaming chat API
			responseMsg, err = s.runNonStreamingChat(ctx, chatModel, tools, chatOptions)
		}

		if err != nil {
			return err
		}

		// Add the response to messages
		s.messages = append(s.messages, responseMsg)

		// Check if there are any tool calls in the response
		toolCalls := responseMsg.GetToolCalls()
		if len(toolCalls) == 0 {
			// No tool calls, we're done
			break
		}

		// Execute tool calls
		if err := s.executeToolCalls(toolCalls); err != nil {
			return err
		}
	}

	return nil
}

func (s *SweSession) buildChatOptions() *models.ChatOptions {
	if s.llmLogger == nil && s.id == "" {
		return nil
	}

	return &models.ChatOptions{
		Logger:    s.llmLogger,
		SessionID: s.id,
	}
}

func (s *SweSession) emitAssistantParts(parts []models.ChatMessagePart, streaming bool) {
	if s.outputHandler == nil {
		return
	}

	var seenToolCalls map[string]bool
	if streaming {
		seenToolCalls = make(map[string]bool)
	}

	for _, part := range parts {
		if part.Text != "" {
			s.outputHandler.AddMarkdownChunk(part.Text)
			if s.logger != nil {
				s.logger.Debug("assistant_output_chunk", "chunk", part.Text)
			}
		}
		if part.ToolCall != nil {
			if streaming {
				if !seenToolCalls[part.ToolCall.ID] {
					s.outputHandler.AddToolCallStart(part.ToolCall)
					seenToolCalls[part.ToolCall.ID] = true
					logging.LogToolCall(s.logger, part.ToolCall)
				}
				s.outputHandler.AddToolCallDetails(part.ToolCall)
			} else {
				s.outputHandler.AddToolCallStart(part.ToolCall)
				s.outputHandler.AddToolCallDetails(part.ToolCall)
				logging.LogToolCall(s.logger, part.ToolCall)
			}
		}
	}
}

// runStreamingChat executes a streaming chat request and returns the accumulated response.
func (s *SweSession) runStreamingChat(ctx context.Context, chatModel models.ChatModel, tools []tool.ToolInfo, chatOptions *models.ChatOptions) (*models.ChatMessage, error) {
	stream := chatModel.ChatStream(ctx, s.messages, chatOptions, tools)

	if s.logger != nil {
		s.logger.Debug("chat_stream_created", "num_messages", len(s.messages), "num_tools", len(tools))
	}

	// Accumulate the response from the stream
	responseMsg := &models.ChatMessage{
		Role:  models.ChatRoleAssistant,
		Parts: []models.ChatMessagePart{},
	}

	if s.logger != nil {
		s.logger.Debug("starting_stream_iteration")
	}

	fragmentCount := 0
	for fragment := range stream {
		fragmentCount++
		if s.logger != nil {
			s.logger.Debug("stream_fragment_received", "fragment_num", fragmentCount, "num_parts", len(fragment.Parts))
		}
		// Merge the fragment parts into the accumulated response
		responseMsg.Parts = append(responseMsg.Parts, fragment.Parts...)
	}

	if s.logger != nil {
		s.logger.Debug("stream_iteration_complete", "fragment_count", fragmentCount, "num_parts", len(responseMsg.Parts))
	}

	// Check if we got an empty response
	if fragmentCount == 0 {
		if s.logger != nil {
			s.logger.Warn("stream_returned_no_fragments", "num_messages", len(s.messages), "num_tools", len(tools))
		}
		return nil, fmt.Errorf("SweSession.runStreamingChat() [session.go]: stream returned no fragments - this usually indicates a silent error in the model provider")
	}

	s.emitAssistantParts(responseMsg.Parts, true)

	return responseMsg, nil
}

// runNonStreamingChat executes a non-streaming chat request and returns the response.
func (s *SweSession) runNonStreamingChat(ctx context.Context, chatModel models.ChatModel, tools []tool.ToolInfo, chatOptions *models.ChatOptions) (*models.ChatMessage, error) {
	if s.logger != nil {
		s.logger.Debug("chat_non_streaming_request", "num_messages", len(s.messages), "num_tools", len(tools))
	}

	// Use non-streaming chat API
	responseMsg, err := chatModel.Chat(ctx, s.messages, chatOptions, tools)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("chat_non_streaming_error", "error", err)
		}
		return nil, fmt.Errorf("SweSession.runNonStreamingChat() [session.go]: chat request failed: %w", err)
	}

	if s.logger != nil {
		s.logger.Debug("chat_non_streaming_complete", "num_parts", len(responseMsg.Parts))
	}

	s.emitAssistantParts(responseMsg.Parts, false)

	return responseMsg, nil
}

// executeToolCalls executes the given tool calls and appends the results to the conversation.
func (s *SweSession) executeToolCalls(toolCalls []*tool.ToolCall) error {
	if s.logger != nil {
		s.logger.Debug("execute_tool_calls_start", "count", len(toolCalls))
	}

	toolResponses := make([]*tool.ToolResponse, 0, len(toolCalls)+len(s.pendingToolResponses))
	if len(s.pendingToolResponses) > 0 {
		toolResponses = append(toolResponses, s.pendingToolResponses...)
	}
	for i, toolCall := range toolCalls {
		// Use s.Tools which might have access control wrappers
		s.logger.Info("executing_tool_call", "tool", toolCall.Function, "args", toolCall.Arguments)
		response := s.Tools.Execute(toolCall)
		s.logger.Info("tool_call_executed", "tool", toolCall.Function, "response", response)

		// Check for permission query
		if permQuery, ok := response.Error.(*tool.ToolPermissionsQuery); ok {
			if s.logger != nil {
				toolFunc := ""
				if permQuery.Tool != nil {
					toolFunc = permQuery.Tool.Function
				}
				s.logger.Info("permission_query",
					"query_id", permQuery.Id,
					"tool", toolFunc,
					"title", permQuery.Title,
					"details", permQuery.Details,
				)
			}
			// Store executed responses and pending tool calls so we can resume after permission is granted
			s.pendingToolResponses = toolResponses
			s.pendingPermissionToolCalls = append([]*tool.ToolCall{toolCall}, toolCalls[i+1:]...)
			return response.Error
		}

		toolResponses = append(toolResponses, response)
		logging.LogToolResult(s.logger, response)

		// Notify UI handler about tool result
		if s.outputHandler != nil {
			s.outputHandler.AddToolCallResult(response)
		}
	}

	// Add tool responses to the conversation
	s.messages = append(s.messages, models.NewToolResponseMessage(toolResponses...))

	// Clear pending state since all tools executed successfully
	s.pendingPermissionToolCalls = nil
	s.pendingToolResponses = nil

	return nil
}

func (s *SweSession) ChatMessages() []*models.ChatMessage {
	return s.messages
}

// ID returns the unique identifier for this session.
func (s *SweSession) ID() string {
	return s.id
}

// SetLogger sets a custom logger for this session.
// This is useful for testing or when you want to use a different logger implementation.
func (s *SweSession) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// Model returns the model name (without provider prefix) used for this session.
func (s *SweSession) Model() string {
	return s.model
}

// GetState returns the current agent state for this session.
func (s *SweSession) GetState() AgentState {
	return AgentState{
		Info: AgentStateCommonInfo{
			WorkDir:     s.workDir,
			CurrentTime: time.Now().Format(time.RFC3339),
			AgentName:   "CSW Coding Agent",
		},
	}
}

// SetWorkDir sets the working directory for this session.
func (s *SweSession) SetWorkDir(dir string) {
	s.workDir = dir
}

// Role returns the current agent role for this session.
func (s *SweSession) Role() *conf.AgentRoleConfig {
	return s.role
}

// ProviderName returns the name of the provider used for this session.
func (s *SweSession) ProviderName() string {
	return s.providerName
}

// GetModelTags returns all tags assigned to the current model.
// Tags are determined by matching the model name against regexp patterns
// from both global config and provider-specific config.
func (s *SweSession) GetModelTags() []string {
	if s.system.ModelTags == nil {
		return nil
	}
	return s.system.ModelTags.GetTagsForModel(s.providerName, s.model)
}

// SetModel sets the model used for the session.
// model string should be formatted as `provider/model-name`.
func (s *SweSession) SetModel(modelStr string) error {
	if s.logger != nil {
		s.logger.Info("set_model", "model", modelStr)
	}

	parts := strings.SplitN(modelStr, "/", 2)
	if len(parts) != 2 {
		if s.logger != nil {
			s.logger.Error("set_model_failed", "model", modelStr, "error", "invalid format")
		}
		return fmt.Errorf("SweSession.SetModel() [session.go]: invalid model format: %s, expected provider/model-name", modelStr)
	}
	providerName := parts[0]
	modelName := parts[1]

	provider, ok := s.system.ModelProviders[providerName]
	if !ok {
		if s.logger != nil {
			s.logger.Error("set_model_failed", "model", modelStr, "error", "provider not found")
		}
		return fmt.Errorf("SweSession.SetModel() [session.go]: provider not found: %s", providerName)
	}

	s.provider = provider
	s.providerName = providerName
	s.model = modelName
	return nil
}

// SetRole changes the agent role for this session.
// It updates the VFS and Tools with access controls based on the new role,
// and adds or updates the system prompt at the beginning of the conversation.
func (s *SweSession) SetRole(roleName string) error {
	if s.logger != nil {
		s.logger.Info("set_role", "role", roleName)
	}

	role, ok := s.system.Roles.Get(roleName)
	if !ok {
		if s.logger != nil {
			s.logger.Error("set_role_failed", "role", roleName, "error", "role not found")
		}
		return fmt.Errorf("SweSession.SetRole() [session.go]: role not found: %s", roleName)
	}

	// Store the new role
	s.role = &role

	// Wrap VFS with access control based on role privileges
	if role.VFSPrivileges != nil {
		s.VFS = vfs.NewAccessControlVFS(s.system.VFS, role.VFSPrivileges)
	} else {
		s.VFS = s.system.VFS
	}

	// Rebuild tools with the session's VFS and role
	s.Tools = buildSessionToolRegistry(s.system.Tools, s.VFS, s.LSP, s)

	// Create a new tool registry with access-controlled tools if needed
	if role.ToolsAccess != nil {
		s.Tools = wrapToolsWithAccessControl(s.Tools, role.ToolsAccess)
	}

	// Generate and update system prompt using the prompt generator
	if s.system.PromptGenerator != nil {
		state := s.GetState()

		// Get model tags from registry
		tags := s.GetModelTags()
		// If no specific tags are assigned, use empty list
		// The prompt system will include fragments with tag "all" by default
		if tags == nil {
			tags = []string{}
		}

		renderedPrompt, err := s.system.PromptGenerator.GetPrompt(tags, &role, &state)
		if err != nil {
			return fmt.Errorf("SweSession.SetRole() [session.go]: failed to generate system prompt: %w", err)
		}

		// Check if there's already a system message
		if len(s.messages) > 0 && s.messages[0].Role == models.ChatRoleSystem {
			// Replace the existing system message
			s.messages[0] = models.NewTextMessage(models.ChatRoleSystem, renderedPrompt)
		} else {
			// Insert system message at the beginning
			s.messages = append([]*models.ChatMessage{models.NewTextMessage(models.ChatRoleSystem, renderedPrompt)}, s.messages...)
		}
	}

	return nil
}

// UpdatePermission updates the permission for a tool or VFS operation based on user response.
func (s *SweSession) UpdatePermission(query *tool.ToolPermissionsQuery, response string) error {
	if s.logger != nil {
		s.logger.Info("permission_response",
			"tool", query.Tool.Function,
			"response", response,
		)
	}

	allow := strings.ToLower(response) == "allow"
	flag := conf.AccessDeny
	if allow {
		flag = conf.AccessAllow
	}

	if query.Meta != nil && query.Meta["type"] == "vfs" {
		path := query.Meta["path"]
		op := query.Meta["operation"]
		if op == "find" && path == "**" {
			path = "*"
		}

		ac, ok := s.VFS.(*vfs.AccessControlVFS)
		if ok {
			ac.SetPermission(path, op, flag)
		}
	} else {
		// Tool permission
		toolName := query.Tool.Function

		// We need to find the AccessControlTool for this tool
		t, err := s.Tools.Get(toolName)
		if err != nil {
			return err
		}

		ac, ok := t.(*tool.AccessControlTool)
		if ok {
			// We set permission for this specific tool name
			ac.SetPermission(toolName, flag)
		}
	}

	// Update the pending tool call's access flag so it will be re-executed with the new permission
	if len(s.pendingPermissionToolCalls) > 0 {
		for _, pending := range s.pendingPermissionToolCalls {
			if pending.ID == query.Tool.ID {
				pending.Access = flag
				break
			}
		}
	}

	return nil
}

// wrapToolsWithAccessControl creates a new tool registry with all tools wrapped in access control.
func wrapToolsWithAccessControl(registry *tool.ToolRegistry, privileges map[string]conf.AccessFlag) *tool.ToolRegistry {
	newRegistry := tool.NewToolRegistry()

	// Get all tool names from the original registry
	for _, name := range registry.List() {
		t, err := registry.Get(name)
		if err != nil {
			// This shouldn't happen since we just got the name from List()
			continue
		}

		// Wrap the tool with access control
		wrappedTool := tool.NewAccessControlTool(t, privileges)
		newRegistry.Register(name, wrappedTool)
	}

	return newRegistry
}

// GetTodoList returns a copy of the current todo list.
func (s *SweSession) GetTodoList() []tool.TodoItem {
	s.todoMu.Lock()
	defer s.todoMu.Unlock()

	// Return a copy to prevent external modification
	list := make([]tool.TodoItem, len(s.todoList))
	copy(list, s.todoList)
	return list
}

// SetTodoList replaces the entire todo list with a new list.
func (s *SweSession) SetTodoList(todos []tool.TodoItem) {
	s.todoMu.Lock()
	defer s.todoMu.Unlock()

	s.todoList = make([]tool.TodoItem, len(todos))
	copy(s.todoList, todos)
}

// CountPendingTodos returns the number of pending or in_progress todos.
func (s *SweSession) CountPendingTodos() int {
	s.todoMu.Lock()
	defer s.todoMu.Unlock()

	count := 0
	for _, item := range s.todoList {
		if item.Status == "pending" || item.Status == "in_progress" {
			count++
		}
	}
	return count
}

// registerSessionTools registers session-specific tools that need access to the session.
func (s *SweSession) registerSessionTools(registry *tool.ToolRegistry) {
	// Register todo tools
	registry.Register("todoRead", tool.NewTodoReadTool(s))
	registry.Register("todoWrite", tool.NewTodoWriteTool(s))
}

func buildSessionToolRegistry(systemTools *tool.ToolRegistry, vfsImpl vfs.VFS, lspClient lsp.LSP, session *SweSession) *tool.ToolRegistry {
	registry := tool.NewToolRegistry()
	if systemTools != nil {
		for _, name := range systemTools.List() {
			t, _ := systemTools.Get(name)
			registry.Register(name, t)
		}
	}

	var logger *slog.Logger
	if session != nil {
		logger = session.logger
	}
	tool.RegisterVFSTools(registry, vfsImpl, lspClient, logger)

	if session != nil {
		session.registerSessionTools(registry)
	}

	registry.ApplyLogger(logger)

	return registry
}
