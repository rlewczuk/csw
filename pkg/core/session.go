package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
)

const (
	defaultLLMRetryMaxAttempts        = 10
	defaultLLMRetryMaxBackoffSeconds  = 60
	defaultContextCompactionThreshold = 0.95
	defaultKimiCompactorMessagesToKeep = 2
	defaultMaxToolThreads             = 8
	toolExecutionStartDelay           = 250 * time.Millisecond
	sessionMessageTypeInfo            = "info"
	sessionMessageTypeWarning         = "warning"
	sessionMessageTypeError           = "error"
)

// SubAgentTaskRunner executes delegated child-session tasks.
type SubAgentTaskRunner interface {
	ExecuteSubAgentTask(parent *SweSession, request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error)
}

type SweSession struct {
	id            string
	parentID      string
	taskID        string
	task          *Task
	slug          string
	provider      models.ModelProvider
	providerName  string
	model         string
	modelSpec     string
	rolesUsed     []string
	toolsUsed     []string
	messages      []*models.ChatMessage
	role          *conf.AgentRoleConfig
	VFS           apis.VFS
	baseVFS       apis.VFS
	LSP           lsp.LSP
	Tools         *tool.ToolRegistry
	outputHandler SessionThreadOutput
	queuedUserPromptDrainer func() []string
	workDir       string
	shadowDir     string
	todoList      []tool.TodoItem
	todoMu        sync.Mutex
	logger        *slog.Logger
	llmLogger     *slog.Logger

	modelProviders  map[string]models.ModelProvider
	modelAliases    map[string][]string
	modelTags       *models.ModelTagRegistry
	toolSelection   conf.ToolSelectionConfig
	promptGenerator PromptGenerator
	roles           *AgentRoleRegistry
	config          *conf.CswConfig
	systemTools     *tool.ToolRegistry
	logBaseDir      string
	thinking        string
	maxToolThreads  int
	toolStartDelay  time.Duration
	llmRetryPolicyOverride *models.RetryPolicy
	allowAllPerms   bool

	// pendingToolResponses stores tool responses that were executed before a permission query
	// so they can be sent together after permissions are granted
	pendingToolResponses []*tool.ToolResponse
	// loadedAgentFiles keeps track of AGENTS.md files already injected into context.
	loadedAgentFiles map[string]struct{}
	tokenUsage       models.TokenUsage
	contextLength    int
	compactionCount int
	compactor       ChatCompactor
	taskManager     *TaskManager
	taskVCS         apis.VCS
	taskStatusUpdatedInSession bool

	subAgentRunner       SubAgentTaskRunner
	subAgentSlugs        map[string]struct{}
	subAgentSlugsMu      sync.Mutex
}

// SweSessionParams stores dependencies and initial values used to create a SweSession.
type SweSessionParams struct {
	ID           string
	ParentID     string
	TaskID       string
	Task         *Task
	Slug         string
	Provider     models.ModelProvider
	ProviderName string
	Model        string
	ModelSpec    string

	VFS         apis.VFS
	BaseVFS     apis.VFS
	LSP         lsp.LSP
	SystemTools *tool.ToolRegistry

	ModelProviders  map[string]models.ModelProvider
	ModelAliases    map[string][]string
	ModelTags       *models.ModelTagRegistry
	ToolSelection   conf.ToolSelectionConfig
	PromptGenerator PromptGenerator
	Roles           *AgentRoleRegistry
	Config          *conf.CswConfig

	OutputHandler  SessionThreadOutput
	WorkDir        string
	ShadowDir      string
	LogBaseDir     string
	Thinking       string
	MaxToolThreads int
	// AllowAllPermissions disables VFS role-based access control wrapping.
	AllowAllPermissions bool

	Logger    *slog.Logger
	LLMLogger *slog.Logger

	Role             *conf.AgentRoleConfig
	RoleName         string
	Messages         []*models.ChatMessage
	TodoList         []tool.TodoItem
	RolesUsed        []string
	ToolsUsed        []string
	LoadedAgentFiles map[string]struct{}

	PendingToolResponses []*tool.ToolResponse
	TokenUsage           models.TokenUsage
	ContextLength        int
	CompactionCount      int
	TaskStatusUpdatedInSession bool
	TaskManager          *TaskManager
	TaskVCS              apis.VCS
	SubAgentRunner       SubAgentTaskRunner
	UsedSubAgentSlugs    map[string]struct{}
}

// NewSweSession creates a new SweSession from provided parameters.
func NewSweSession(params *SweSessionParams) *SweSession {
	if params == nil {
		params = &SweSessionParams{}
	}

	session := &SweSession{
		id:              params.ID,
		parentID:        strings.TrimSpace(params.ParentID),
		taskID:          strings.TrimSpace(params.TaskID),
		task:            cloneTask(params.Task),
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
		modelAliases:    params.ModelAliases,
		modelTags:       params.ModelTags,
		toolSelection:   params.ToolSelection,
		promptGenerator: params.PromptGenerator,
		roles:           params.Roles,
		config:          params.Config,
		systemTools:     params.SystemTools,
		logBaseDir:      params.LogBaseDir,
		thinking:        params.Thinking,
		maxToolThreads:  params.MaxToolThreads,
		allowAllPerms:   params.AllowAllPermissions,

			pendingToolResponses: make([]*tool.ToolResponse, 0, len(params.PendingToolResponses)),
			loadedAgentFiles:     make(map[string]struct{}, len(params.LoadedAgentFiles)),
		tokenUsage:           params.TokenUsage,
		contextLength:        params.ContextLength,
			compactionCount:      params.CompactionCount,
			compactor:            NewKimiCompactor(nil, defaultKimiCompactorMessagesToKeep, params.Config),
			taskManager:          params.TaskManager,
			taskVCS:              params.TaskVCS,
			taskStatusUpdatedInSession: params.TaskStatusUpdatedInSession,
			subAgentRunner:       params.SubAgentRunner,
			subAgentSlugs:        make(map[string]struct{}, len(params.UsedSubAgentSlugs)),
	}

	if session.baseVFS == nil {
		session.baseVFS = session.VFS
	}

	copy(session.todoList, params.TodoList)
	session.messages = append(session.messages, params.Messages...)
	session.pendingToolResponses = append(session.pendingToolResponses, params.PendingToolResponses...)
	if session.role == nil && strings.TrimSpace(params.RoleName) != "" && session.roles != nil {
		if resolvedRole, ok := session.roles.Get(params.RoleName); ok {
			session.role = &resolvedRole
		}
	}
	if session.role != nil {
		session.rolesUsed = appendUniqueString(session.rolesUsed, session.role.Name)
		if !session.allowAllPerms && session.role.VFSPrivileges != nil {
			session.VFS = vfs.NewAccessControlVFS(session.baseVFS, session.role.VFSPrivileges)
		} else {
			session.VFS = session.baseVFS
		}
	}
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
	if session.role != nil {
		if err := session.updateSystemPromptForRole(*session.role); err != nil && session.logger != nil {
			session.logger.Warn("role_prompt_initialization_failed", "role", session.role.Name, "error", err)
		}
	}

	return session
}

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

	chatModelImpl, chatModelErr := NewGenerationChatModelFromSpec(
		s.ModelWithProvider(),
		providerMap,
		chatOptions,
		s.config,
		s.provider,
		s.modelAliases,
		s.llmRetryPolicyOverride,
		s.handleRetryChatModelMessage,
	)
	if chatModelErr != nil {
		return fmt.Errorf("SweSession.Run() [session.go]: failed to create chat model chain: %w", chatModelErr)
	}
	chatModel := models.NewUnstreamingChatModel(chatModelImpl)
	s.configureCompactor(chatModel, chatModelImpl)

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
		if len(s.messages) > 0 {
			lastMsg := s.messages[len(s.messages)-1]
			if lastMsg.Role == models.ChatRoleAssistant {
				toolCalls := lastMsg.GetToolCalls()
				if len(toolCalls) > 0 {
					if err := s.executeToolCalls(toolCalls); err != nil {
						return err
					}
				}
			}
		}

		s.flushQueuedUserPrompts()

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
			if s.flushQueuedUserPrompts() == 0 {
				// No tool calls and no queued user prompts, we're done
				break
			}

			continue
		}

		// Execute tool calls
		if err := s.executeToolCalls(toolCalls); err != nil {
			return err
		}

		s.flushQueuedUserPrompts()
	}

	return nil
}

func (s *SweSession) flushQueuedUserPrompts() int {
	if s == nil || s.queuedUserPromptDrainer == nil {
		return 0
	}

	prompts := s.queuedUserPromptDrainer()
	if len(prompts) == 0 {
		return 0
	}

	for _, prompt := range prompts {
		if s.outputHandler != nil {
			s.outputHandler.AddUserMessage(prompt)
		}
		s.appendConversationMessage(models.NewTextMessage(models.ChatRoleUser, prompt), "incoming", "queued_user_prompt")
	}

	return len(prompts)
}

func (s *SweSession) configureCompactor(chatModel models.ChatModel, compactorProvider models.ChatModel) {
	if s == nil {
		return
	}

	if compactorProvider != nil {
		if modelCompactor := compactorProvider.Compactor(); modelCompactor != nil {
			s.compactor = &modelChatCompactorAdapter{
				modelCompactor: modelCompactor,
				fallback:       s.compactor,
			}
			return
		}
	}

	if kimiCompactor, ok := s.compactor.(*KimiCompactor); ok && kimiCompactor.model == nil {
		s.compactor = NewKimiCompactor(chatModel, kimiCompactor.nmessages, s.config)
	}
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
