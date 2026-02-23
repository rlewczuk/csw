package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"regexp"
	"sort"
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

const (
	defaultLLMRetryMaxAttempts       = 10
	defaultLLMRetryMaxBackoffSeconds = 60
	sessionMessageTypeInfo           = "info"
	sessionMessageTypeWarning        = "warning"
	sessionMessageTypeError          = "error"
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
	// loadedAgentFiles keeps track of AGENTS.md files already injected into context.
	loadedAgentFiles map[string]struct{}
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
	maxAttempts := s.llmRetryMaxAttempts()
	maxBackoffSeconds := s.llmRetryMaxBackoffSeconds()
	backoffScale := models.DefaultRetryBackoffScale
	if configProvider, ok := s.provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		config := configProvider.GetConfig()
		if config != nil && config.RateLimitBackoffScale > 0 {
			backoffScale = config.RateLimitBackoffScale
		}
	}

	for {
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			if s.logger != nil {
				s.logger.Debug("chat_non_streaming_request", "num_messages", len(s.messages), "num_tools", len(tools), "attempt", attempt)
			}

			responseMsg, err := chatModel.Chat(ctx, s.messages, chatOptions, tools)
			if err == nil {
				if s.logger != nil {
					s.logger.Debug("chat_non_streaming_complete", "num_parts", len(responseMsg.Parts))
				}

				s.emitAssistantMessage(responseMsg)
				return responseMsg, nil
			}

			if !isTemporaryLLMError(err) {
				if s.logger != nil {
					s.logger.Error("chat_non_streaming_error", "error", err)
				}
				return nil, fmt.Errorf("SweSession.runNonStreamingChat() [session.go]: chat request failed: %w", err)
			}

			retryAfterSeconds := 0
			var rateLimitErr *models.RateLimitError
			if errors.As(err, &rateLimitErr) {
				retryAfterSeconds = rateLimitErr.RetryAfterSeconds
				if s.outputHandler != nil {
					s.outputHandler.OnRateLimitError(retryAfterSeconds)
				}
			}

			if s.outputHandler != nil {
				s.outputHandler.ShowMessage(fmt.Sprintf("LLM API temporary error (attempt %d/%d): %v", attempt, maxAttempts, err), sessionMessageTypeError)
			}

			if attempt >= maxAttempts {
				if s.outputHandler != nil {
					if s.outputHandler.ShouldRetryAfterFailure(fmt.Sprintf("LLM API request failed after %d attempts: %v", maxAttempts, err)) {
						s.outputHandler.ShowMessage("Retry requested by user. Starting another retry cycle.", sessionMessageTypeInfo)
						break
					}
				}
				return nil, fmt.Errorf("SweSession.runNonStreamingChat() [session.go]: temporary LLM API failure after %d attempts: %w", maxAttempts, err)
			}

			backoffSeconds := retryAfterSeconds
			if backoffSeconds <= 0 {
				backoffSeconds = 1 << (attempt - 1)
			}
			if backoffSeconds > maxBackoffSeconds {
				backoffSeconds = maxBackoffSeconds
			}
			backoffDuration := time.Duration(backoffSeconds) * backoffScale

			if s.outputHandler != nil {
				s.outputHandler.ShowMessage(fmt.Sprintf("Retrying in %s...", backoffDuration.Round(time.Second)), sessionMessageTypeWarning)
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDuration):
			}
		}
	}
}

// executeToolCalls executes the given tool calls and appends the results to the conversation.
func (s *SweSession) executeToolCalls(toolCalls []*tool.ToolCall) error {
	if s.logger != nil {
		s.logger.Debug("execute_tool_calls_start", "count", len(toolCalls))
	}

	toolResponses := make([]*tool.ToolResponse, 0, len(toolCalls)+len(s.pendingToolResponses))
	agentMessages := make([]*models.ChatMessage, 0)
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

		newAgentMessages, err := s.buildAdditionalAgentMessages(toolCall, response)
		if err != nil {
			return fmt.Errorf("SweSession.executeToolCalls() [session.go]: failed to load additional AGENTS.md instructions: %w", err)
		}
		agentMessages = append(agentMessages, newAgentMessages...)

		// Notify UI handler about tool result
		if s.outputHandler != nil {
			s.outputHandler.AddToolCallResult(response)
		}
	}

	if len(agentMessages) > 0 {
		s.messages = append(s.messages, agentMessages...)
	}

	// Add tool responses to the conversation
	s.messages = append(s.messages, models.NewToolResponseMessage(toolResponses...))

	// Clear pending state since all tools executed successfully
	s.pendingPermissionToolCalls = nil
	s.pendingToolResponses = nil

	return nil
}

// buildAdditionalAgentMessages builds user messages with extra AGENTS.md instructions for vfsRead/vfsGrep tool calls.
func (s *SweSession) buildAdditionalAgentMessages(toolCall *tool.ToolCall, response *tool.ToolResponse) ([]*models.ChatMessage, error) {
	if s == nil || s.system == nil || s.system.PromptGenerator == nil || toolCall == nil || response == nil || response.Error != nil {
		return nil, nil
	}

	var dirs []string
	switch toolCall.Function {
	case "vfsRead":
		path, ok := toolCall.Arguments.StringOK("path")
		if !ok || strings.TrimSpace(path) == "" {
			return nil, nil
		}
		dirs = append(dirs, filepath.Dir(path))
	case "vfsGrep":
		dirs = append(dirs, parseDirsFromGrepResult(response.Result.Get("content").AsString())...)
	default:
		return nil, nil
	}

	messages := make([]*models.ChatMessage, 0)
	for _, dir := range uniqueStrings(dirs) {
		dirMessages, err := s.buildAdditionalAgentMessageForDir(dir)
		if err != nil {
			return nil, err
		}
		if len(dirMessages) > 0 {
			messages = append(messages, dirMessages...)
		}
	}

	return messages, nil
}

// buildAdditionalAgentMessageForDir creates user messages from AGENTS.md files in the
// provided directory and its parent directories if not loaded yet.
func (s *SweSession) buildAdditionalAgentMessageForDir(dir string) ([]*models.ChatMessage, error) {
	rootPath := ""
	if s.VFS != nil {
		rootPath = s.VFS.WorktreePath()
	}
	if strings.TrimSpace(rootPath) == "" {
		rootPath = s.workDir
	}
	workDirAbs, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("SweSession.buildAdditionalAgentMessageForDir() [session.go]: failed to resolve root path %q: %w", rootPath, err)
	}

	resolvedDir := dir
	if strings.TrimSpace(resolvedDir) == "" {
		resolvedDir = "."
	}
	if !filepath.IsAbs(resolvedDir) {
		resolvedDir = filepath.Join(workDirAbs, resolvedDir)
	}
	resolvedDir, err = filepath.Abs(resolvedDir)
	if err != nil {
		return nil, fmt.Errorf("SweSession.buildAdditionalAgentMessageForDir() [session.go]: failed to resolve dir %q: %w", dir, err)
	}

	relDir, err := filepath.Rel(workDirAbs, resolvedDir)
	if err != nil {
		return nil, fmt.Errorf("SweSession.buildAdditionalAgentMessageForDir() [session.go]: failed to get relative dir for %q: %w", resolvedDir, err)
	}
	if relDir == "." || relDir == "" || strings.HasPrefix(relDir, "..") || filepath.IsAbs(relDir) {
		return nil, nil
	}

	if s.loadedAgentFiles == nil {
		s.loadedAgentFiles = make(map[string]struct{})
	}

	files, err := s.system.PromptGenerator.GetAgentFiles(relDir)
	if err != nil {
		return nil, fmt.Errorf("SweSession.buildAdditionalAgentMessageForDir() [session.go]: failed to get agent files for %q: %w", relDir, err)
	}
	if len(files) == 0 {
		return nil, nil
	}

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		depthI := strings.Count(filepath.Clean(paths[i]), string(filepath.Separator))
		depthJ := strings.Count(filepath.Clean(paths[j]), string(filepath.Separator))
		if depthI != depthJ {
			return depthI > depthJ
		}
		return paths[i] < paths[j]
	})

	messages := make([]*models.ChatMessage, 0, len(paths))
	for _, agentsPath := range paths {
		if _, loaded := s.loadedAgentFiles[agentsPath]; loaded {
			continue
		}
		s.loadedAgentFiles[agentsPath] = struct{}{}
		wrapped := "<system>\n" + files[agentsPath] + "\n</system>"
		messages = append(messages, models.NewTextMessage(models.ChatRoleUser, wrapped))
	}

	if len(messages) == 0 {
		return nil, nil
	}

	return messages, nil
}

// parseDirsFromGrepResult extracts directories from vfsGrep result content.
func parseDirsFromGrepResult(content string) []string {
	lines := strings.Split(content, "\n")
	dirs := make([]string, 0, len(lines))
	linePattern := regexp.MustCompile(`^(.+):(\d+)$`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "(") || line == "No files found" {
			continue
		}
		matches := linePattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		dirs = append(dirs, filepath.Dir(matches[1]))
	}
	return dirs
}

// uniqueStrings returns a deduplicated slice preserving input order.
func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
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
func (s *SweSession) llmRetryMaxAttempts() int {
	if s.system != nil && s.system.ConfigStore != nil {
		globalConfig, err := s.system.ConfigStore.GetGlobalConfig()
		if err == nil && globalConfig != nil && globalConfig.LLMRetryMaxAttempts > 0 {
			return globalConfig.LLMRetryMaxAttempts
		}
	}

	if configProvider, ok := s.provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		config := configProvider.GetConfig()
		if config != nil && config.MaxRetries > 0 {
			return config.MaxRetries + 1
		}
	}

	return defaultLLMRetryMaxAttempts
}

// llmRetryMaxBackoffSeconds returns the maximum backoff in seconds for temporary failures.
func (s *SweSession) llmRetryMaxBackoffSeconds() int {
	if s.system != nil && s.system.ConfigStore != nil {
		globalConfig, err := s.system.ConfigStore.GetGlobalConfig()
		if err == nil && globalConfig != nil && globalConfig.LLMRetryMaxBackoffSeconds > 0 {
			return globalConfig.LLMRetryMaxBackoffSeconds
		}
	}
	return defaultLLMRetryMaxBackoffSeconds
}

// isTemporaryLLMError returns true when an LLM error indicates temporary condition.
func isTemporaryLLMError(err error) bool {
	if err == nil {
		return false
	}

	var rateLimitErr *models.RateLimitError
	if errors.As(err, &rateLimitErr) {
		return true
	}

	var networkErr *models.NetworkError
	if errors.As(err, &networkErr) {
		return networkErr.IsRetryable
	}

	if errors.Is(err, models.ErrEndpointUnavailable) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	return false
}
