package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
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

	chatModel := models.NewUnstreamingChatModel(s.provider.ChatModel(s.model, chatOptions))

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

		responseMsg, err := s.runNonStreamingChat(ctx, chatModel, tools, chatOptions)
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
	if s.llmLogger == nil && s.id == "" && s.system.Thinking == "" {
		return nil
	}

	return &models.ChatOptions{
		Logger:    s.llmLogger,
		SessionID: s.id,
		Thinking:  s.system.Thinking,
	}
}

func (s *SweSession) emitAssistantMessage(responseMsg *models.ChatMessage) {
	if s.outputHandler == nil || responseMsg == nil {
		return
	}

	var textBuilder strings.Builder
	var thinkingBuilder strings.Builder
	for _, part := range responseMsg.Parts {
		if part.Text != "" {
			textBuilder.WriteString(part.Text)
		}
		if part.ReasoningContent != "" {
			thinkingBuilder.WriteString(part.ReasoningContent)
		}
	}

	s.outputHandler.AddAssistantMessage(textBuilder.String(), thinkingBuilder.String())
	for _, part := range responseMsg.Parts {
		if part.ToolCall != nil {
			s.outputHandler.AddToolCall(part.ToolCall)
			logging.LogToolCall(s.logger, part.ToolCall)
		}
	}
}

// runNonStreamingChat executes a non-streaming chat request and returns the response.
func (s *SweSession) runNonStreamingChat(ctx context.Context, chatModel models.ChatModel, tools []tool.ToolInfo, chatOptions *models.ChatOptions) (*models.ChatMessage, error) {
	maxRetries := s.maxRetries()
	backoffScale := models.DefaultRetryBackoffScale
	if configProvider, ok := s.provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		config := configProvider.GetConfig()
		if config != nil && config.RateLimitBackoffScale > 0 {
			backoffScale = config.RateLimitBackoffScale
		}
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if s.logger != nil {
			s.logger.Debug("chat_non_streaming_request", "num_messages", len(s.messages), "num_tools", len(tools), "attempt", attempt)
		}

		// Use non-streaming chat API
		responseMsg, err := chatModel.Chat(ctx, s.messages, chatOptions, tools)
		if err == nil {
			if s.logger != nil {
				s.logger.Debug("chat_non_streaming_complete", "num_parts", len(responseMsg.Parts))
			}

			s.emitAssistantMessage(responseMsg)

			return responseMsg, nil
		}

		// Check if this is a rate limit error
		var rateLimitErr *models.RateLimitError
		if errors.As(err, &rateLimitErr) {
			if s.logger != nil {
				s.logger.Warn("rate_limit_error", "error", err, "retry_after", rateLimitErr.RetryAfterSeconds, "attempt", attempt)
			}

			// Notify the UI about rate limit
			if s.outputHandler != nil {
				s.outputHandler.OnRateLimitError(rateLimitErr.RetryAfterSeconds)
			}

			// If we've exhausted retries, return the error
			if attempt >= maxRetries {
				return nil, fmt.Errorf("SweSession.runNonStreamingChat() [session.go]: rate limit exceeded after %d retries: %w", maxRetries, err)
			}

			// Calculate backoff time
			backoffSeconds := rateLimitErr.RetryAfterSeconds
			if backoffSeconds == 0 {
				// Use exponential backoff: 1s, 2s, 4s, 8s, etc.
				backoffSeconds = int(math.Pow(2, float64(attempt)))
			}
			backoffDuration := time.Duration(backoffSeconds) * backoffScale

			if s.logger != nil {
				s.logger.Info("retrying_after_rate_limit", "backoff_duration", backoffDuration, "attempt", attempt+1, "max_retries", maxRetries)
			}

			// Wait for backoff duration
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDuration):
				// Continue to next retry
			}
			continue
		}

		// Check if this is a retryable network error
		var networkErr *models.NetworkError
		if errors.As(err, &networkErr) && networkErr.IsRetryable {
			if s.logger != nil {
				s.logger.Warn("network_error", "error", err, "attempt", attempt)
			}

			// Notify the UI about network retry
			if s.outputHandler != nil {
				s.outputHandler.OnRateLimitError(0) // Use 0 to indicate exponential backoff
			}

			// If we've exhausted retries, return the error
			if attempt >= maxRetries {
				return nil, fmt.Errorf("SweSession.runNonStreamingChat() [session.go]: network error after %d retries: %w", maxRetries, err)
			}

			// Calculate backoff time using exponential backoff: 1s, 2s, 4s, 8s, etc.
			backoffSeconds := int(math.Pow(2, float64(attempt)))
			backoffDuration := time.Duration(backoffSeconds) * backoffScale

			if s.logger != nil {
				s.logger.Info("retrying_after_network_error", "backoff_duration", backoffDuration, "attempt", attempt+1, "max_retries", maxRetries)
			}

			// Wait for backoff duration
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDuration):
				// Continue to next retry
			}
			continue
		}

		// Not a retryable error, return immediately
		if s.logger != nil {
			s.logger.Error("chat_non_streaming_error", "error", err)
		}
		return nil, fmt.Errorf("SweSession.runNonStreamingChat() [session.go]: chat request failed: %w", err)
	}

	return nil, fmt.Errorf("SweSession.runNonStreamingChat() [session.go]: chat request failed after %d retries", maxRetries)
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
	s.applyModelTagToolSelection()
	if s.role != nil && s.role.ToolsAccess != nil {
		s.Tools = wrapToolsWithAccessControl(s.Tools, s.role.ToolsAccess)
	}
	return nil
}

// applyModelTagToolSelection rebuilds tools and applies model-tag based tool selection rules.
func (s *SweSession) applyModelTagToolSelection() {
	baseTools := buildSessionToolRegistry(s.system.Tools, s.VFS, s.LSP, s)
	if s.system.ModelTags == nil {
		s.Tools = baseTools.FilterByModelTags(nil, s.system.ToolSelection)
		return
	}
	tags := s.system.ModelTags.GetTagsForModel(s.providerName, s.model)
	s.Tools = baseTools.FilterByModelTags(tags, s.system.ToolSelection)
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

	// Rebuild tools with the session's VFS and role and apply model-tag selection
	s.applyModelTagToolSelection()

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

// maxRetries returns the maximum number of retries for rate limit/network errors.
// Returns default value from models.DefaultMaxRetries if not configured.
func (s *SweSession) maxRetries() int {
	if configProvider, ok := s.provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		config := configProvider.GetConfig()
		if config != nil && config.MaxRetries > 0 {
			return config.MaxRetries
		}
	}
	return models.DefaultMaxRetries
}
