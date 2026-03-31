package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
)

const (
	defaultLLMRetryMaxAttempts       = 10
	defaultLLMRetryMaxBackoffSeconds = 60
	defaultContextCompactionThreshold = 0.95
	defaultMaxToolThreads             = 8
	toolExecutionStartDelay           = 250 * time.Millisecond
	sessionMessageTypeInfo            = "info"
	sessionMessageTypeWarning         = "warning"
	sessionMessageTypeError           = "error"
)

// SubAgentTaskRunner executes delegated subagent tasks for a parent session.
type SubAgentTaskRunner interface {
	ExecuteSubAgentTask(parent *SweSession, request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error)
}

type SweSession struct {
	id            string
	parentID      string
	slug          string
	provider      models.ModelProvider
	providerName  string
	model         string
	modelSpec     string
	rolesUsed     []string
	toolsUsed     []string
	messages      []*models.ChatMessage
	role          *conf.AgentRoleConfig
	VFS           vfs.VFS
	baseVFS       vfs.VFS
	LSP           lsp.LSP
	Tools         *tool.ToolRegistry
	outputHandler SessionThreadOutput
	workDir       string
	shadowDir     string
	todoList      []tool.TodoItem
	todoMu        sync.Mutex
	logger        *slog.Logger
	llmLogger     *slog.Logger

	modelProviders  map[string]models.ModelProvider
	modelTags       *models.ModelTagRegistry
	toolSelection   conf.ToolSelectionConfig
	promptGenerator PromptGenerator
	roles           *AgentRoleRegistry
	configStore     conf.ConfigStore
	systemTools     *tool.ToolRegistry
	logBaseDir      string
	thinking        string
	maxToolThreads  int

	// pendingPermissionToolCall stores the tool call that was blocked by a permission query
	// This is used to re-execute the tool after permission is granted
	pendingPermissionToolCalls []*tool.ToolCall
	// pendingToolResponses stores tool responses that were executed before a permission query
	// so they can be sent together after permissions are granted
	pendingToolResponses []*tool.ToolResponse
	// loadedAgentFiles keeps track of AGENTS.md files already injected into context.
	loadedAgentFiles map[string]struct{}
	tokenUsage       models.TokenUsage
	contextLength    int
	compactionCount  int
	subAgentSlugs    map[string]struct{}
	subAgentSlugsMu  sync.Mutex
	subAgentRunner   SubAgentTaskRunner
	hookFeedbackExec tool.HookFeedbackExecutor
}

// SweSessionParams stores dependencies and initial values used to create a SweSession.
type SweSessionParams struct {
	ID           string
	ParentID     string
	Slug         string
	Provider     models.ModelProvider
	ProviderName string
	Model        string
	ModelSpec    string

	VFS         vfs.VFS
	BaseVFS     vfs.VFS
	LSP         lsp.LSP
	SystemTools *tool.ToolRegistry

	ModelProviders  map[string]models.ModelProvider
	ModelTags       *models.ModelTagRegistry
	ToolSelection   conf.ToolSelectionConfig
	PromptGenerator PromptGenerator
	Roles           *AgentRoleRegistry
	ConfigStore     conf.ConfigStore

	OutputHandler  SessionThreadOutput
	WorkDir        string
	ShadowDir      string
	LogBaseDir     string
	Thinking       string
	MaxToolThreads int

	Logger    *slog.Logger
	LLMLogger *slog.Logger

	Role             *conf.AgentRoleConfig
	Messages         []*models.ChatMessage
	TodoList         []tool.TodoItem
	RolesUsed        []string
	ToolsUsed        []string
	LoadedAgentFiles map[string]struct{}

	PendingPermissionToolCalls []*tool.ToolCall
	PendingToolResponses       []*tool.ToolResponse
	TokenUsage                 models.TokenUsage
	ContextLength              int
	CompactionCount            int
	UsedSubAgentSlugs          map[string]struct{}
	SubAgentRunner             SubAgentTaskRunner
	HookFeedbackExecutor       tool.HookFeedbackExecutor
}

// NewSweSession creates a new SweSession from provided parameters.
func NewSweSession(params *SweSessionParams) *SweSession {
	if params == nil {
		params = &SweSessionParams{}
	}

	session := &SweSession{
		id:              params.ID,
		parentID:        strings.TrimSpace(params.ParentID),
		slug:            strings.TrimSpace(params.Slug),
		provider:        params.Provider,
		providerName:    params.ProviderName,
		model:           params.Model,
		modelSpec:       strings.TrimSpace(params.ModelSpec),
		rolesUsed:       append([]string(nil), params.RolesUsed...),
		toolsUsed:       append([]string(nil), params.ToolsUsed...),
		messages:        make([]*models.ChatMessage, 0, len(params.Messages)),
		role:            params.Role,
		VFS:             params.VFS,
		baseVFS:         params.BaseVFS,
		LSP:             params.LSP,
		Tools:           nil,
		outputHandler:   params.OutputHandler,
		workDir:         params.WorkDir,
		shadowDir:       params.ShadowDir,
		todoList:        make([]tool.TodoItem, len(params.TodoList)),
		logger:          params.Logger,
		llmLogger:       params.LLMLogger,
		modelProviders:  params.ModelProviders,
		modelTags:       params.ModelTags,
		toolSelection:   params.ToolSelection,
		promptGenerator: params.PromptGenerator,
		roles:           params.Roles,
		configStore:     params.ConfigStore,
		systemTools:     params.SystemTools,
		logBaseDir:      params.LogBaseDir,
		thinking:        params.Thinking,
		maxToolThreads:  params.MaxToolThreads,

		pendingPermissionToolCalls: make([]*tool.ToolCall, 0, len(params.PendingPermissionToolCalls)),
		pendingToolResponses:       make([]*tool.ToolResponse, 0, len(params.PendingToolResponses)),
		loadedAgentFiles:           make(map[string]struct{}, len(params.LoadedAgentFiles)),
		tokenUsage:                 params.TokenUsage,
		contextLength:              params.ContextLength,
		compactionCount:            params.CompactionCount,
		subAgentSlugs:              make(map[string]struct{}, len(params.UsedSubAgentSlugs)),
		subAgentRunner:             params.SubAgentRunner,
		hookFeedbackExec:           params.HookFeedbackExecutor,
	}

	if session.modelSpec == "" {
		session.modelSpec = composeProviderModel(session.providerName, session.model)
	}

	if session.baseVFS == nil {
		session.baseVFS = session.VFS
	}

	copy(session.todoList, params.TodoList)
	session.messages = append(session.messages, params.Messages...)
	session.pendingPermissionToolCalls = append(session.pendingPermissionToolCalls, params.PendingPermissionToolCalls...)
	session.pendingToolResponses = append(session.pendingToolResponses, params.PendingToolResponses...)
	for path := range params.LoadedAgentFiles {
		session.loadedAgentFiles[path] = struct{}{}
	}
	for slug := range params.UsedSubAgentSlugs {
		trimmedSlug := strings.TrimSpace(slug)
		if trimmedSlug == "" {
			continue
		}
		session.subAgentSlugs[trimmedSlug] = struct{}{}
	}

	session.applyModelTagToolSelection()
	if session.role != nil && session.role.ToolsAccess != nil {
		session.Tools = wrapToolsWithAccessControl(session.Tools, session.role.ToolsAccess)
	}

	return session
}

type persistedToolResponse struct {
	Call   *tool.ToolCall `json:"call,omitempty"`
	Error  string         `json:"error,omitempty"`
	Result tool.ToolValue `json:"result"`
	Done   bool           `json:"done"`
}

type persistedChatMessagePart struct {
	Text             string                 `json:"text,omitempty"`
	ReasoningContent string                 `json:"reasoning_content,omitempty"`
	ToolCall         *tool.ToolCall         `json:"tool_call,omitempty"`
	ToolResponse     *persistedToolResponse `json:"tool_response,omitempty"`
}

type persistedChatMessage struct {
	Role  string                     `json:"role"`
	Parts []persistedChatMessagePart `json:"parts"`
}

type persistedSessionState struct {
	SessionID                  string                  `json:"session_id"`
	ParentSessionID            string                  `json:"parent_session_id,omitempty"`
	Slug                       string                  `json:"slug,omitempty"`
	ModelSpec                  string                  `json:"model_spec,omitempty"`
	ProviderName               string                  `json:"provider_name"`
	Model                      string                  `json:"model"`
	Thinking                   string                  `json:"thinking,omitempty"`
	RolesUsed                  []string                `json:"roles_used,omitempty"`
	ToolsUsed                  []string                `json:"tools_used,omitempty"`
	RoleName                   string                  `json:"role_name,omitempty"`
	WorkDir                    string                  `json:"workdir"`
	TodoList                   []tool.TodoItem         `json:"todo_list"`
	Messages                   []persistedChatMessage  `json:"messages"`
	PendingPermissionToolCalls []tool.ToolCall         `json:"pending_permission_tool_calls"`
	PendingToolResponses       []persistedToolResponse `json:"pending_tool_responses"`
	LoadedAgentFiles           []string                `json:"loaded_agent_files"`
	TokenUsage                 models.TokenUsage       `json:"token_usage"`
	ContextLengthTokens        int                     `json:"context_length_tokens"`
	ContextCompactionCount     int                     `json:"context_compaction_count"`
	UsedSubAgentSlugs          []string                `json:"used_subagent_slugs,omitempty"`
	UpdatedAt                  string                  `json:"updated_at"`
}

// PersistedSessionState is a persisted session state model used for loading sessions.
type PersistedSessionState = persistedSessionState

// Prompt adds user prompt to the conversation and starts processing if processing is not already in progress.
// If processing is already in progress, if will be added at the end of conversation after current LLM request is completed,
// its tool calls are executed etc. Returns immediately.
func (s *SweSession) UserPrompt(prompt string) error {
	if s.logger != nil {
		s.logger.Info("user_input", "input", prompt)
	}
	s.appendConversationMessage(models.NewTextMessage(models.ChatRoleUser, prompt), "incoming", "user_prompt")
	return nil
}

func (s *SweSession) Run(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Debug("session_run_start", "session_id", s.id, "model", s.model)
	}

	chatOptions := s.buildChatOptions()

	providerMap := s.modelProviders
	if providerMap == nil {
		providerMap = map[string]models.ModelProvider{}
		if strings.TrimSpace(s.providerName) != "" && s.provider != nil {
			providerMap[s.providerName] = s.provider
		}
	}

	retryPolicy := s.llmRetryPolicy()
	chatModelImpl, chatModelErr := models.NewChatModelFromProviderChain(
		s.ModelWithProvider(),
		providerMap,
		chatOptions,
		&retryPolicy,
		s.handleRetryChatModelMessage,
	)
	if chatModelErr != nil {
		return fmt.Errorf("SweSession.Run() [session.go]: failed to create chat model chain: %w", chatModelErr)
	}
	chatModel := models.NewUnstreamingChatModel(chatModelImpl)

	// Build tools list using PromptGenerator.GetToolInfo()
	tools := []tool.ToolInfo{}
	toolNames := s.Tools.List()

	// Get model tags for this model
	tags := s.GetModelTags()

	// Get agent state for template processing
	state := s.GetState()

	for _, toolName := range toolNames {
		toolInfo, err := s.promptGenerator.GetToolInfo(tags, toolName, s.role, &state)
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

		if sessionTool, getErr := s.Tools.Get(toolName); getErr == nil {
			dynamicDescription, overwrite := sessionTool.GetDescription()
			if strings.TrimSpace(dynamicDescription) != "" {
				if overwrite {
					toolInfo.Description = dynamicDescription
				} else if strings.TrimSpace(toolInfo.Description) == "" {
					toolInfo.Description = dynamicDescription
				} else {
					toolInfo.Description = strings.TrimRight(toolInfo.Description, "\n") + dynamicDescription
				}
			}
		}

		tools = append(tools, toolInfo)
	}

	var (
		responseMsg *models.ChatMessage
		err         error
	)

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

		if err := s.maybeCompactContext(); err != nil {
			return err
		}

		responseMsg, err = s.runNonStreamingChat(ctx, chatModel, tools, chatOptions)
		if err != nil {
			return err
		}

		// Add the response to messages
		s.appendConversationMessage(responseMsg, "outgoing", "assistant_response")

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
	if s.llmLogger == nil && s.id == "" && s.thinking == "" {
		return nil
	}

	return &models.ChatOptions{
		Logger:    s.llmLogger,
		SessionID: s.id,
		Thinking:  s.thinking,
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
	attempt := 0

	for {
		attempt++
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

		if errors.Is(err, models.ErrTooManyInputTokens) {
			if s.logger != nil {
				s.logger.Warn("chat_non_streaming_too_many_input_tokens", "attempt", attempt, "max_attempts", maxAttempts, "error", err)
			}

			if s.outputHandler != nil {
				s.outputHandler.ShowMessage("LLM rejected input because context is too large. Compacting messages and retrying...", sessionMessageTypeWarning)
			}

			if compactErr := s.compactContext("Context exceeded model input token limit. Compacting messages..."); compactErr != nil {
				return nil, fmt.Errorf("SweSession.runNonStreamingChat() [session.go]: failed to compact context after token limit error: %w", compactErr)
			}

			if attempt >= maxAttempts {
				return nil, fmt.Errorf("SweSession.runNonStreamingChat() [session.go]: too many input tokens after %d attempts: %w", maxAttempts, err)
			}

			continue
		}

		if s.logger != nil {
			s.logLLMRequestError("chat_non_streaming_error", err)
		}
		if s.outputHandler != nil {
			if s.outputHandler.ShouldRetryAfterFailure(fmt.Sprintf("LLM API request failed after %d attempts: %v", maxAttempts, err)) {
				s.outputHandler.ShowMessage("Retry requested by user. Starting another retry cycle.", sessionMessageTypeInfo)
				attempt = 0
				continue
			}
		}

		return nil, fmt.Errorf("SweSession.runNonStreamingChat() [session.go]: chat request failed: %w", err)
	}
}

func (s *SweSession) handleRetryChatModelMessage(message string, msgType shared.MessageType) {
	if s == nil {
		return
	}

	if s.outputHandler != nil {
		s.outputHandler.ShowMessage(message, mapSharedMessageTypeToSessionMessageType(msgType))
	}

	if msgType == shared.MessageTypeWarning {
		if retryAfterSeconds, ok := extractRetryAfterSeconds(message); ok && s.outputHandler != nil {
			s.outputHandler.OnRateLimitError(retryAfterSeconds)
		}
	}
}

func mapSharedMessageTypeToSessionMessageType(msgType shared.MessageType) string {
	switch msgType {
	case shared.MessageTypeError:
		return sessionMessageTypeError
	case shared.MessageTypeWarning:
		return sessionMessageTypeWarning
	default:
		return sessionMessageTypeInfo
	}
}

func extractRetryAfterSeconds(message string) (int, bool) {
	if strings.TrimSpace(message) == "" {
		return 0, false
	}

	matches := regexp.MustCompile(`\(in\s+(\d+)\s+seconds\)`).FindStringSubmatch(message)
	if len(matches) < 2 {
		return 0, false
	}

	retryAfterSeconds, err := strconv.Atoi(matches[1])
	if err != nil || retryAfterSeconds < 0 {
		return 0, false
	}

	return retryAfterSeconds, true
}

func (s *SweSession) logLLMRequestError(event string, err error) {
	if s == nil || s.logger == nil || err == nil {
		return
	}

	var llmRequestErr *models.LLMRequestError
	if errors.As(err, &llmRequestErr) {
		s.logger.Error(event, "error", err, "llm_raw_response", llmRequestErr.RawResponse)
		return
	}

	s.logger.Error(event, "error", err)
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
	for _, toolCall := range toolCalls {
		s.toolsUsed = appendUniqueString(s.toolsUsed, toolCall.Function)
	}

	type toolExecutionResult struct {
		index    int
		toolCall *tool.ToolCall
		response *tool.ToolResponse
	}

	results := make([]toolExecutionResult, len(toolCalls))

	maxToolThreads := s.maxToolThreadsLimit()
	if maxToolThreads > len(toolCalls) {
		maxToolThreads = len(toolCalls)
	}
	if maxToolThreads <= 0 {
		maxToolThreads = 1
	}

	type indexedToolCall struct {
		index int
		call  *tool.ToolCall
	}

	jobs := make(chan indexedToolCall, len(toolCalls))
	for i, toolCall := range toolCalls {
		jobs <- indexedToolCall{index: i, call: toolCall}
	}
	close(jobs)

	var (
		wg            sync.WaitGroup
		startGateMu   sync.Mutex
		lastStartTime time.Time
	)

	waitForStartSlot := func() {
		startGateMu.Lock()
		defer startGateMu.Unlock()
		if !lastStartTime.IsZero() {
			nextStartAt := lastStartTime.Add(toolExecutionStartDelay)
			if sleepFor := time.Until(nextStartAt); sleepFor > 0 {
				time.Sleep(sleepFor)
			}
		}
		lastStartTime = time.Now()
	}

	for i := 0; i < maxToolThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				waitForStartSlot()

				if s.logger != nil {
					s.logger.Info("executing_tool_call", "tool", job.call.Function, "args", job.call.Arguments)
				}

				response := s.Tools.Execute(job.call)

				if s.logger != nil {
					s.logger.Info("tool_call_executed", "tool", job.call.Function, "response", response)
				}

				results[job.index] = toolExecutionResult{index: job.index, toolCall: job.call, response: response}
			}
		}()
	}

	wg.Wait()

	pendingPermissionToolCalls := make([]*tool.ToolCall, 0)
	var firstPermissionQuery error
	for _, result := range results {
		response := result.response
		if response == nil {
			continue
		}

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

			pendingPermissionToolCalls = append(pendingPermissionToolCalls, result.toolCall)
			if firstPermissionQuery == nil {
				firstPermissionQuery = response.Error
			}
			continue
		}

		toolResponses = append(toolResponses, response)
		logging.LogToolResult(s.logger, response)

		newAgentMessages, err := s.buildAdditionalAgentMessages(result.toolCall, response)
		if err != nil {
			return fmt.Errorf("SweSession.executeToolCalls() [session.go]: failed to load additional AGENTS.md instructions: %w", err)
		}
		agentMessages = append(agentMessages, newAgentMessages...)

		if s.outputHandler != nil {
			s.outputHandler.AddToolCallResult(response)
		}
	}

	if len(pendingPermissionToolCalls) > 0 {
		s.pendingToolResponses = toolResponses
		s.pendingPermissionToolCalls = pendingPermissionToolCalls
		s.persistSessionState()
		return firstPermissionQuery
	}

	// Add tool responses to the conversation
	s.appendConversationMessage(models.NewToolResponseMessage(toolResponses...), "incoming", "tool_response")

	if len(agentMessages) > 0 {
		for _, agentMessage := range agentMessages {
			s.appendConversationMessage(agentMessage, "incoming", "agent_instructions")
		}
	}

	// Clear pending state since all tools executed successfully
	s.pendingPermissionToolCalls = nil
	s.pendingToolResponses = nil
	s.persistSessionState()

	return nil
}

// buildAdditionalAgentMessages builds user messages with extra AGENTS.md instructions for vfsRead/vfsGrep tool calls.
func (s *SweSession) buildAdditionalAgentMessages(toolCall *tool.ToolCall, response *tool.ToolResponse) ([]*models.ChatMessage, error) {
	if s == nil || s.promptGenerator == nil || toolCall == nil || response == nil || response.Error != nil {
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

	files, err := s.promptGenerator.GetAgentFiles(relDir)
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

func appendUniqueString(values []string, value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return values
	}

	for _, existing := range values {
		if existing == trimmed {
			return values
		}
	}

	return append(values, trimmed)
}

func (s *SweSession) ChatMessages() []*models.ChatMessage {
	return s.messages
}

// ID returns the unique identifier for this session.
func (s *SweSession) ID() string {
	return s.id
}

// ParentID returns the parent session identifier for delegated child sessions.
func (s *SweSession) ParentID() string {
	if s == nil {
		return ""
	}

	return s.parentID
}

// Slug returns session slug used by UI views and logs.
func (s *SweSession) Slug() string {
	if s == nil {
		return ""
	}

	return s.slug
}

// ReserveSubAgentSlug marks slug as used in parent session.
func (s *SweSession) ReserveSubAgentSlug(slug string) error {
	if s == nil {
		return fmt.Errorf("SweSession.ReserveSubAgentSlug() [session.go]: session is nil")
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return fmt.Errorf("SweSession.ReserveSubAgentSlug() [session.go]: slug cannot be empty")
	}

	s.subAgentSlugsMu.Lock()
	defer s.subAgentSlugsMu.Unlock()
	if s.subAgentSlugs == nil {
		s.subAgentSlugs = make(map[string]struct{})
	}
	if _, exists := s.subAgentSlugs[trimmedSlug]; exists {
		return fmt.Errorf("SweSession.ReserveSubAgentSlug() [session.go]: slug already used in session: %s", trimmedSlug)
	}
	s.subAgentSlugs[trimmedSlug] = struct{}{}
	s.persistSessionState()

	return nil
}

// ReserveUniqueSubAgentSlug reserves slug in parent session and adds numeric suffix when needed.
func (s *SweSession) ReserveUniqueSubAgentSlug(slug string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("SweSession.ReserveUniqueSubAgentSlug() [session.go]: session is nil")
	}

	baseSlug := strings.TrimSpace(slug)
	if baseSlug == "" {
		return "", fmt.Errorf("SweSession.ReserveUniqueSubAgentSlug() [session.go]: slug cannot be empty")
	}

	s.subAgentSlugsMu.Lock()
	defer s.subAgentSlugsMu.Unlock()
	if s.subAgentSlugs == nil {
		s.subAgentSlugs = make(map[string]struct{})
	}

	resolvedSlug := baseSlug
	if _, exists := s.subAgentSlugs[resolvedSlug]; exists {
		for suffix := 2; ; suffix++ {
			candidate := fmt.Sprintf("%s-%d", baseSlug, suffix)
			if _, exists := s.subAgentSlugs[candidate]; exists {
				continue
			}
			resolvedSlug = candidate
			break
		}
	}

	s.subAgentSlugs[resolvedSlug] = struct{}{}
	s.persistSessionState()

	return resolvedSlug, nil
}

// SetLogger sets a custom logger for this session.
// This is useful for testing or when you want to use a different logger implementation.
func (s *SweSession) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// SetOutputHandler sets output handler used by session callbacks.
func (s *SweSession) SetOutputHandler(handler SessionThreadOutput) {
	s.outputHandler = handler
}

// OutputHandler returns currently configured output handler.
func (s *SweSession) OutputHandler() SessionThreadOutput {
	return s.outputHandler
}

// Model returns the model name (without provider prefix) used for this session.
func (s *SweSession) Model() string {
	return s.model
}

// ModelWithProvider returns the provider-qualified model name used for this session.
func (s *SweSession) ModelWithProvider() string {
	if strings.TrimSpace(s.modelSpec) != "" {
		return s.modelSpec
	}

	if strings.TrimSpace(s.providerName) == "" {
		return s.model
	}

	if strings.TrimSpace(s.model) == "" {
		return s.providerName
	}

	return s.providerName + "/" + s.model
}

func composeProviderModel(providerName string, modelName string) string {
	trimmedProvider := strings.TrimSpace(providerName)
	trimmedModel := strings.TrimSpace(modelName)
	if trimmedProvider == "" {
		return trimmedModel
	}
	if trimmedModel == "" {
		return trimmedProvider
	}

	return trimmedProvider + "/" + trimmedModel
}

// GetState returns the current agent state for this session.
func (s *SweSession) GetState() AgentState {
	shadowDir := ""
	if s != nil {
		shadowDir = strings.TrimSpace(s.shadowDir)
		if shadowDir == "" {
			shadowDir = strings.TrimSpace(s.workDir)
		}
	}
	if shadowDir == "" {
		shadowDir = strings.TrimSpace(s.workDir)
	}
	if shadowDir != "" && !filepath.IsAbs(shadowDir) {
		if absShadowDir, err := filepath.Abs(shadowDir); err == nil {
			shadowDir = absShadowDir
		}
	}

	return AgentState{
		Info: AgentStateCommonInfo{
			WorkDir:             s.workDir,
			ShadowDir:           shadowDir,
			CurrentTime:         time.Now().Format(time.RFC3339),
			AgentName:           "CSW Coding Agent",
			TokenUsage:          s.tokenUsage,
			ContextLengthTokens: s.contextLength,
		},
	}
}

// TokenUsage returns aggregated token usage for this session.
func (s *SweSession) TokenUsage() models.TokenUsage {
	return s.tokenUsage
}

// ContextLengthTokens returns latest known context length in tokens.
func (s *SweSession) ContextLengthTokens() int {
	return s.contextLength
}

// CompactionCount returns number of performed context compactions.
func (s *SweSession) CompactionCount() int {
	if s == nil {
		return 0
	}

	return s.compactionCount
}

// SetWorkDir sets the working directory for this session.
func (s *SweSession) SetWorkDir(dir string) {
	s.workDir = dir
	s.persistSessionState()
}

// PersistSessionState persists current session state to disk.
func (s *SweSession) PersistSessionState() {
	s.persistSessionState()
}

// Role returns the current agent role for this session.
func (s *SweSession) Role() *conf.AgentRoleConfig {
	return s.role
}

// ProviderName returns the name of the provider used for this session.
func (s *SweSession) ProviderName() string {
	return s.providerName
}

// ThinkingLevel returns configured thinking level for this session.
func (s *SweSession) ThinkingLevel() string {
	if s == nil {
		return ""
	}

	return strings.TrimSpace(s.thinking)
}

// SetThinkingLevel updates configured thinking mode for this session.
func (s *SweSession) SetThinkingLevel(thinking string) {
	if s == nil {
		return
	}

	s.thinking = strings.TrimSpace(thinking)
	s.persistSessionState()
}

// UsedRoles returns roles used during this session in first-seen order.
func (s *SweSession) UsedRoles() []string {
	if s == nil || len(s.rolesUsed) == 0 {
		return nil
	}

	result := make([]string, len(s.rolesUsed))
	copy(result, s.rolesUsed)
	return result
}

// UsedTools returns tools used during this session in first-seen order.
func (s *SweSession) UsedTools() []string {
	if s == nil || len(s.toolsUsed) == 0 {
		return nil
	}

	result := make([]string, len(s.toolsUsed))
	copy(result, s.toolsUsed)
	return result
}

// GetModelTags returns all tags assigned to the current model.
// Tags are determined by matching the model name against regexp patterns
// from both global config and provider-specific config.
func (s *SweSession) GetModelTags() []string {
	if s.modelTags == nil {
		return nil
	}
	return s.modelTags.GetTagsForModel(s.providerName, s.model)
}

// SetModel sets the model used for the session.
// model string should be formatted as `provider/model-name`
// or a comma-separated `provider/model-name` list for fallback.
func (s *SweSession) SetModel(modelStr string) error {
	if s.logger != nil {
		s.logger.Info("set_model", "model", modelStr)
	}

	refs, parseErr := models.ParseProviderModelChain(modelStr)
	if parseErr != nil || len(refs) == 0 {
		if s.logger != nil {
			s.logger.Error("set_model_failed", "model", modelStr, "error", "invalid format")
		}
		return fmt.Errorf("SweSession.SetModel() [session.go]: invalid model format: %s, expected provider/model or comma-separated provider/model list", modelStr)
	}

	for _, ref := range refs {
		if _, exists := s.modelProviders[ref.Provider]; !exists {
			if s.logger != nil {
				s.logger.Error("set_model_failed", "model", modelStr, "error", "provider not found")
			}
			return fmt.Errorf("SweSession.SetModel() [session.go]: provider not found: %s", ref.Provider)
		}
	}
	providerName := refs[0].Provider
	modelName := refs[0].Model
	provider := s.modelProviders[providerName]

	s.provider = provider
	s.providerName = providerName
	s.model = modelName
	s.modelSpec = strings.TrimSpace(modelStr)
	s.applyModelTagToolSelection()
	if s.role != nil && s.role.ToolsAccess != nil {
		s.Tools = wrapToolsWithAccessControl(s.Tools, s.role.ToolsAccess)
	}
	s.persistSessionState()
	return nil
}

// applyModelTagToolSelection rebuilds tools and applies model-tag based tool selection rules.
func (s *SweSession) applyModelTagToolSelection() {
	baseTools := buildSessionToolRegistry(s.systemTools, s.VFS, s.LSP, s)
	if s.modelTags == nil {
		s.Tools = filterToolsForRole(baseTools.FilterByModelTags(nil, s.toolSelection), s.role)
		return
	}
	tags := s.modelTags.GetTagsForModel(s.providerName, s.model)
	s.Tools = filterToolsForRole(baseTools.FilterByModelTags(tags, s.toolSelection), s.role)
}

// SetRole changes the agent role for this session.
// It updates the VFS and Tools with access controls based on the new role,
// and adds or updates the system prompt at the beginning of the conversation.
func (s *SweSession) SetRole(roleName string) error {
	if s.logger != nil {
		s.logger.Info("set_role", "role", roleName)
	}

	role, ok := s.roles.Get(roleName)
	if !ok {
		if s.logger != nil {
			s.logger.Error("set_role_failed", "role", roleName, "error", "role not found")
		}
		return fmt.Errorf("SweSession.SetRole() [session.go]: role not found: %s", roleName)
	}

	// Store the new role
	s.role = &role
	s.rolesUsed = appendUniqueString(s.rolesUsed, role.Name)

	// Wrap VFS with access control based on role privileges
	if role.VFSPrivileges != nil {
		s.VFS = vfs.NewAccessControlVFS(s.baseVFS, role.VFSPrivileges)
	} else {
		s.VFS = s.baseVFS
	}

	// Rebuild tools with the session's VFS and role and apply model-tag selection
	s.applyModelTagToolSelection()

	// Create a new tool registry with access-controlled tools if needed
	if role.ToolsAccess != nil {
		s.Tools = wrapToolsWithAccessControl(s.Tools, role.ToolsAccess)
	}

	// Generate and update system prompt using the prompt generator
	if s.promptGenerator != nil {
		state := s.GetState()

		// Get model tags from registry
		tags := s.GetModelTags()
		// If no specific tags are assigned, use empty list
		// The prompt system will include fragments with tag "all" by default
		if tags == nil {
			tags = []string{}
		}

		renderedPrompt, err := s.promptGenerator.GetPrompt(tags, &role, &state)
		if err != nil {
			return fmt.Errorf("SweSession.SetRole() [session.go]: failed to generate system prompt: %w", err)
		}

		// Check if there's already a system message
		if len(s.messages) > 0 && s.messages[0].Role == models.ChatRoleSystem {
			// Replace the existing system message
			systemMessage := models.NewTextMessage(models.ChatRoleSystem, renderedPrompt)
			s.messages[0] = systemMessage
			s.persistSessionState()
		} else {
			// Insert system message at the beginning
			systemMessage := models.NewTextMessage(models.ChatRoleSystem, renderedPrompt)
			s.messages = append([]*models.ChatMessage{systemMessage}, s.messages...)
			s.persistSessionState()
		}
	}

	s.persistSessionState()

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

	s.persistSessionState()

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

func filterToolsForRole(registry *tool.ToolRegistry, role *conf.AgentRoleConfig) *tool.ToolRegistry {
	if registry == nil || role == nil {
		return registry
	}

	filtered := tool.NewToolRegistry()
	for _, name := range registry.List() {
		t, err := registry.Get(name)
		if err != nil {
			continue
		}
		if restricted, ok := t.(tool.RoleRestrictedTool); ok {
			if !restricted.IsRoleAllowed(role.Name) {
				continue
			}
		}
		filtered.Register(name, t)
	}

	return filtered
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
	s.todoList = make([]tool.TodoItem, len(todos))
	copy(s.todoList, todos)
	s.todoMu.Unlock()

	s.persistSessionState()
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
	registry.Register("subAgent", tool.NewSubAgentTool(s))
	if s.hookFeedbackExec != nil {
		registry.Register("hookFeedback", tool.NewHookFeedbackTool(s.hookFeedbackExec, s.ModelWithProvider, s.ThinkingLevel))
	}
}

// ExecuteSubAgentTask executes delegated subagent task for this parent session.
func (s *SweSession) ExecuteSubAgentTask(request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error) {
	if s == nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSession.ExecuteSubAgentTask() [session.go]: session is nil")
	}

	if s.subAgentRunner == nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSession.ExecuteSubAgentTask() [session.go]: subagent runner is nil")
	}

	return s.subAgentRunner.ExecuteSubAgentTask(s, request)
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

func (s *SweSession) appendConversationMessage(message *models.ChatMessage, direction string, source string) {
	if message == nil {
		return
	}

	s.messages = append(s.messages, message)
	s.applyMessageTokenStats(message)
	s.persistSessionState()
}

func (s *SweSession) applyMessageTokenStats(message *models.ChatMessage) {
	if message == nil {
		return
	}

	if message.TokenUsage != nil {
		s.tokenUsage.InputTokens += message.TokenUsage.InputTokens
		s.tokenUsage.InputCachedTokens += message.TokenUsage.InputCachedTokens
		s.tokenUsage.InputNonCachedTokens += message.TokenUsage.InputNonCachedTokens
		s.tokenUsage.OutputTokens += message.TokenUsage.OutputTokens
		s.tokenUsage.TotalTokens += message.TokenUsage.TotalTokens
	}

	if message.ContextLengthTokens > 0 {
		s.contextLength = message.ContextLengthTokens
	} else if message.TokenUsage != nil && message.TokenUsage.TotalTokens > 0 {
		s.contextLength = message.TokenUsage.TotalTokens
	}
}

func (s *SweSession) persistSessionState() {
	if err := s.persistSessionStateFile(); err != nil {
		if s.logger != nil {
			s.logger.Warn("failed_to_persist_session_state", "error", err)
		}
	}
}

func (s *SweSession) persistSessionStateFile() error {
	sessionLogDir := s.getSessionLogDirectory()
	if sessionLogDir == "" {
		return nil
	}

	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return fmt.Errorf("SweSession.persistSessionStateFile() [session.go]: failed to create session log directory: %w", err)
	}

	state := s.buildPersistedSessionState()
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("SweSession.persistSessionStateFile() [session.go]: failed to marshal session state: %w", err)
	}

	tempPath := filepath.Join(sessionLogDir, "session.json.tmp")
	finalPath := filepath.Join(sessionLogDir, "session.json")
	if err := os.WriteFile(tempPath, stateJSON, 0644); err != nil {
		return fmt.Errorf("SweSession.persistSessionStateFile() [session.go]: failed to write temporary session state file: %w", err)
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("SweSession.persistSessionStateFile() [session.go]: failed to replace session state file: %w", err)
	}

	return nil
}

func (s *SweSession) buildPersistedSessionState() persistedSessionState {
	state := persistedSessionState{
		SessionID:                  s.id,
		ParentSessionID:            s.parentID,
		Slug:                       s.slug,
		ModelSpec:                  s.ModelWithProvider(),
		ProviderName:               s.providerName,
		Model:                      s.model,
		Thinking:                   strings.TrimSpace(s.thinking),
		RolesUsed:                  append([]string(nil), s.rolesUsed...),
		ToolsUsed:                  append([]string(nil), s.toolsUsed...),
		WorkDir:                    s.workDir,
		TodoList:                   s.GetTodoList(),
		Messages:                   make([]persistedChatMessage, 0, len(s.messages)),
		PendingPermissionToolCalls: make([]tool.ToolCall, 0, len(s.pendingPermissionToolCalls)),
		PendingToolResponses:       make([]persistedToolResponse, 0, len(s.pendingToolResponses)),
		LoadedAgentFiles:           make([]string, 0, len(s.loadedAgentFiles)),
		UsedSubAgentSlugs:          make([]string, 0, len(s.subAgentSlugs)),
		TokenUsage:                 s.tokenUsage,
		ContextLengthTokens:        s.contextLength,
		ContextCompactionCount:     s.compactionCount,
		UpdatedAt:                  time.Now().Format(time.RFC3339Nano),
	}

	if s.role != nil {
		state.RoleName = s.role.Name
	}

	for _, message := range s.messages {
		state.Messages = append(state.Messages, serializeChatMessage(message))
	}

	for _, toolCall := range s.pendingPermissionToolCalls {
		if toolCall == nil {
			continue
		}
		state.PendingPermissionToolCalls = append(state.PendingPermissionToolCalls, *toolCall)
	}

	for _, toolResponse := range s.pendingToolResponses {
		if toolResponse == nil {
			continue
		}
		state.PendingToolResponses = append(state.PendingToolResponses, serializeToolResponse(toolResponse))
	}

	for path := range s.loadedAgentFiles {
		state.LoadedAgentFiles = append(state.LoadedAgentFiles, path)
	}
	sort.Strings(state.LoadedAgentFiles)

	for slug := range s.subAgentSlugs {
		state.UsedSubAgentSlugs = append(state.UsedSubAgentSlugs, slug)
	}
	sort.Strings(state.UsedSubAgentSlugs)

	return state
}

func (s *SweSession) getSessionLogDirectory() string {
	if s == nil || s.id == "" {
		return ""
	}

	if s.logBaseDir != "" {
		return filepath.Join(s.logBaseDir, "sessions", s.id)
	}

	dir := logging.GetSessionLogDirectory(s.id)
	if dir != "" {
		return dir
	}

	return ""
}

func serializeChatMessage(message *models.ChatMessage) persistedChatMessage {
	serialized := persistedChatMessage{
		Role:  string(message.Role),
		Parts: make([]persistedChatMessagePart, 0, len(message.Parts)),
	}

	for _, part := range message.Parts {
		serializedPart := persistedChatMessagePart{
			Text:             part.Text,
			ReasoningContent: part.ReasoningContent,
			ToolCall:         part.ToolCall,
		}
		if part.ToolResponse != nil {
			serializedToolResponse := serializeToolResponse(part.ToolResponse)
			serializedPart.ToolResponse = &serializedToolResponse
		}
		serialized.Parts = append(serialized.Parts, serializedPart)
	}

	return serialized
}

func (s *SweSession) maybeCompactContext() error {
	maxContextLength := s.maxContextLengthLimit()
	if maxContextLength <= 0 || s.contextLength <= 0 {
		return nil
	}

	threshold := s.contextCompactionThreshold()
	if threshold <= 0 {
		threshold = defaultContextCompactionThreshold
	}

	if float64(s.contextLength) <= float64(maxContextLength)*threshold {
		return nil
	}

	return s.compactContext("Context is near maximum length. Compacting messages...")
}

func (s *SweSession) compactContext(statusMessage string) error {
	compactionNumber := s.compactionCount + 1
	if s.outputHandler != nil && strings.TrimSpace(statusMessage) != "" {
		s.outputHandler.ShowMessage(statusMessage, sessionMessageTypeInfo)
	}

	if err := s.persistCompactionMessagesSnapshot("pre", compactionNumber, s.messages); err != nil {
		return fmt.Errorf("SweSession.maybeCompactContext() [session.go]: failed to persist pre-compaction snapshot: %w", err)
	}

	compacted := CompactMessages(s.messages)
	if err := s.persistCompactionMessagesSnapshot("post", compactionNumber, compacted); err != nil {
		return fmt.Errorf("SweSession.maybeCompactContext() [session.go]: failed to persist post-compaction snapshot: %w", err)
	}

	s.messages = compacted
	s.compactionCount = compactionNumber
	s.persistSessionState()

	return nil
}

// ForceCompactContext compacts session context regardless of current threshold.
func (s *SweSession) ForceCompactContext() error {
	if s == nil {
		return fmt.Errorf("SweSession.ForceCompactContext() [session.go]: session is nil")
	}

	return s.compactContext("Compacting resumed session context...")
}

func (s *SweSession) contextCompactionThreshold() float64 {
	if s.configStore != nil {
		globalConfig, err := s.configStore.GetGlobalConfig()
		if err == nil && globalConfig != nil && globalConfig.ContextCompactionThreshold > 0 && globalConfig.ContextCompactionThreshold <= 1 {
			return globalConfig.ContextCompactionThreshold
		}
	}

	return defaultContextCompactionThreshold
}

func (s *SweSession) maxContextLengthLimit() int {
	if configProvider, ok := s.provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		providerConfig := configProvider.GetConfig()
		if providerConfig != nil && providerConfig.ContextLengthLimit > 0 {
			return providerConfig.ContextLengthLimit
		}
	}

	return 0
}

func (s *SweSession) persistCompactionMessagesSnapshot(phase string, compactionNumber int, messages []*models.ChatMessage) error {
	sessionLogDir := s.getSessionLogDirectory()
	if strings.TrimSpace(sessionLogDir) == "" {
		return nil
	}

	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return fmt.Errorf("SweSession.persistCompactionMessagesSnapshot() [session.go]: failed to create session log directory: %w", err)
	}

	filePath := filepath.Join(sessionLogDir, fmt.Sprintf("messages-%s-%d.jsonl", phase, compactionNumber))
	if err := writeMessagesJSONL(filePath, messages); err != nil {
		return fmt.Errorf("SweSession.persistCompactionMessagesSnapshot() [session.go]: failed to write %s snapshot: %w", phase, err)
	}

	return nil
}

func writeMessagesJSONL(path string, messages []*models.ChatMessage) error {
	var builder strings.Builder
	for _, message := range messages {
		if message == nil {
			continue
		}
		serializedMessage := serializeChatMessage(message)
		line, err := json.Marshal(serializedMessage)
		if err != nil {
			return fmt.Errorf("writeMessagesJSONL() [session.go]: failed to marshal chat message: %w", err)
		}
		builder.Write(line)
		builder.WriteByte('\n')
	}

	if err := os.WriteFile(path, []byte(builder.String()), 0644); err != nil {
		return fmt.Errorf("writeMessagesJSONL() [session.go]: failed to write jsonl file: %w", err)
	}

	return nil
}

func serializeToolResponse(response *tool.ToolResponse) persistedToolResponse {
	serialized := persistedToolResponse{
		Result: response.Result,
		Done:   response.Done,
	}
	if response.Call != nil {
		serializedCall := *response.Call
		serialized.Call = &serializedCall
	}
	if response.Error != nil {
		serialized.Error = response.Error.Error()
	}

	return serialized
}

// maxRetries returns the maximum number of retries for rate limit/network errors.
// Returns default value from models.DefaultMaxRetries if not configured.
func (s *SweSession) llmRetryMaxAttempts() int {
	if s.configStore != nil {
		globalConfig, err := s.configStore.GetGlobalConfig()
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
	if s.configStore != nil {
		globalConfig, err := s.configStore.GetGlobalConfig()
		if err == nil && globalConfig != nil && globalConfig.LLMRetryMaxBackoffSeconds > 0 {
			return globalConfig.LLMRetryMaxBackoffSeconds
		}
	}
	return defaultLLMRetryMaxBackoffSeconds
}

func (s *SweSession) llmRetryPolicy() models.RetryPolicy {
	attempts := s.llmRetryMaxAttempts()
	if attempts <= 0 {
		attempts = defaultLLMRetryMaxAttempts
	}

	retries := attempts - 1
	if retries < 0 {
		retries = 0
	}

	backoffScale := models.DefaultRetryBackoffScale
	if configProvider, ok := s.provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		config := configProvider.GetConfig()
		if config != nil && config.RateLimitBackoffScale > 0 {
			backoffScale = config.RateLimitBackoffScale
		}
	}

	maxBackoffDuration := time.Duration(s.llmRetryMaxBackoffSeconds()) * backoffScale
	if maxBackoffDuration <= 0 {
		maxBackoffDuration = 60 * backoffScale
	}

	return models.RetryPolicy{
		InitialDelay: backoffScale,
		MaxRetries:   retries,
		MaxDelay:     maxBackoffDuration,
	}
}

// maxToolThreadsLimit returns max number of parallel tool executions.
func (s *SweSession) maxToolThreadsLimit() int {
	if s.maxToolThreads > 0 {
		return s.maxToolThreads
	}

	if s.configStore != nil {
		globalConfig, err := s.configStore.GetGlobalConfig()
		if err == nil && globalConfig != nil && globalConfig.MaxToolThreads > 0 {
			return globalConfig.MaxToolThreads
		}
	}

	return defaultMaxToolThreads
}

// HasPendingWork returns true when the session has pending work that can be resumed
// without adding a new user message.
func (s *SweSession) HasPendingWork() bool {
	if s == nil {
		return false
	}

	if len(s.pendingPermissionToolCalls) > 0 || len(s.pendingToolResponses) > 0 {
		return true
	}

	if len(s.messages) == 0 {
		return false
	}

	last := s.messages[len(s.messages)-1]
	if last == nil {
		return false
	}

	if last.Role == models.ChatRoleUser {
		return true
	}

	if last.Role == models.ChatRoleAssistant && len(last.GetToolCalls()) > 0 {
		return true
	}

	return false
}

// RestoreSessionFromPersistedState restores session state from persisted data.
func RestoreSessionFromPersistedState(params *SweSessionParams, state persistedSessionState, outputHandler SessionThreadOutput) (*SweSession, error) {
	if strings.TrimSpace(state.SessionID) == "" {
		return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session.go]: missing session_id in persisted state")
	}

	if params == nil {
		return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session.go]: params cannot be nil")
	}

	provider, ok := params.ModelProviders[state.ProviderName]
	if !ok {
		return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session.go]: provider not found: %s", state.ProviderName)
	}

	session := NewSweSession(&SweSessionParams{
		ID:              state.SessionID,
		ParentID:        state.ParentSessionID,
		Slug:            state.Slug,
		ModelSpec: func() string {
			if strings.TrimSpace(state.ModelSpec) != "" {
				return strings.TrimSpace(state.ModelSpec)
			}

			return composeProviderModel(state.ProviderName, state.Model)
		}(),
		Provider:        provider,
		ProviderName:    state.ProviderName,
		Model:           state.Model,
		RolesUsed:       state.RolesUsed,
		ToolsUsed:       state.ToolsUsed,
		Messages:        []*models.ChatMessage{},
		Role:            nil,
		VFS:             params.VFS,
		BaseVFS:         params.VFS,
		LSP:             params.LSP,
		SystemTools:     params.SystemTools,
		ModelProviders:  params.ModelProviders,
		ModelTags:       params.ModelTags,
		ToolSelection:   params.ToolSelection,
		PromptGenerator: params.PromptGenerator,
		Roles:           params.Roles,
		ConfigStore:     params.ConfigStore,
		OutputHandler:   outputHandler,
		WorkDir:         state.WorkDir,
		ShadowDir:       params.ShadowDir,
		LogBaseDir:      params.LogBaseDir,
		Thinking: func() string {
			if strings.TrimSpace(state.Thinking) != "" {
				return strings.TrimSpace(state.Thinking)
			}

			return params.Thinking
		}(),
		Logger:          params.Logger,
		LLMLogger:       params.LLMLogger,
		TodoList:        state.TodoList,
		TokenUsage:      state.TokenUsage,
		ContextLength:   state.ContextLengthTokens,
		CompactionCount: state.ContextCompactionCount,
		UsedSubAgentSlugs: func() map[string]struct{} {
			result := make(map[string]struct{}, len(state.UsedSubAgentSlugs))
			for _, slug := range state.UsedSubAgentSlugs {
				trimmedSlug := strings.TrimSpace(slug)
				if trimmedSlug == "" {
					continue
				}
				result[trimmedSlug] = struct{}{}
			}
			return result
		}(),
	})

	if strings.TrimSpace(state.RoleName) != "" {
		role, roleOK := params.Roles.Get(state.RoleName)
		if !roleOK {
			return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session.go]: role not found: %s", state.RoleName)
		}

		session.role = &role

		if role.VFSPrivileges != nil {
			session.VFS = vfs.NewAccessControlVFS(params.VFS, role.VFSPrivileges)
		} else {
			session.VFS = params.VFS
		}

		session.applyModelTagToolSelection()
		if role.ToolsAccess != nil {
			session.Tools = wrapToolsWithAccessControl(session.Tools, role.ToolsAccess)
		}
		if len(session.rolesUsed) == 0 {
			session.rolesUsed = append(session.rolesUsed, role.Name)
		}
	}

	for _, persistedMsg := range state.Messages {
		message, err := deserializeChatMessage(persistedMsg)
		if err != nil {
			return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session.go]: failed to deserialize message: %w", err)
		}
		session.messages = append(session.messages, message)
	}

	session.pendingPermissionToolCalls = make([]*tool.ToolCall, 0, len(state.PendingPermissionToolCalls))
	for _, pendingCall := range state.PendingPermissionToolCalls {
		copiedCall := pendingCall
		session.pendingPermissionToolCalls = append(session.pendingPermissionToolCalls, &copiedCall)
	}

	session.pendingToolResponses = make([]*tool.ToolResponse, 0, len(state.PendingToolResponses))
	for _, pendingResponse := range state.PendingToolResponses {
		response, err := deserializeToolResponse(pendingResponse)
		if err != nil {
			return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session.go]: failed to deserialize pending tool response: %w", err)
		}
		session.pendingToolResponses = append(session.pendingToolResponses, response)
	}

	session.loadedAgentFiles = make(map[string]struct{}, len(state.LoadedAgentFiles))
	for _, path := range state.LoadedAgentFiles {
		session.loadedAgentFiles[path] = struct{}{}
	}

	return session, nil
}

func deserializeChatMessage(persisted persistedChatMessage) (*models.ChatMessage, error) {
	message := &models.ChatMessage{
		Role:  models.ChatRole(persisted.Role),
		Parts: make([]models.ChatMessagePart, 0, len(persisted.Parts)),
	}

	for _, persistedPart := range persisted.Parts {
		part := models.ChatMessagePart{
			Text:             persistedPart.Text,
			ReasoningContent: persistedPart.ReasoningContent,
		}

		if persistedPart.ToolCall != nil {
			callCopy := *persistedPart.ToolCall
			part.ToolCall = &callCopy
		}

		if persistedPart.ToolResponse != nil {
			toolResponse, err := deserializeToolResponse(*persistedPart.ToolResponse)
			if err != nil {
				return nil, fmt.Errorf("deserializeChatMessage() [session.go]: failed to deserialize tool response: %w", err)
			}
			part.ToolResponse = toolResponse
		}

		message.Parts = append(message.Parts, part)
	}

	return message, nil
}

func deserializeToolResponse(persisted persistedToolResponse) (*tool.ToolResponse, error) {
	response := &tool.ToolResponse{
		Result: persisted.Result,
		Done:   persisted.Done,
	}

	if persisted.Call != nil {
		callCopy := *persisted.Call
		response.Call = &callCopy
	}

	if strings.TrimSpace(persisted.Error) != "" {
		response.Error = errors.New(persisted.Error)
	}

	return response, nil
}
