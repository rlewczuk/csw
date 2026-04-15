package system

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
)

// SessionLoggerFactory is a function that creates a session logger.
// This allows tests to provide in-memory loggers instead of file-based ones.
type SessionLoggerFactory func(sessionID string, logBaseDir string) (*slog.Logger, error)

// SweSystem represents the core system for managing conversations, tools, and models.
type SweSystem struct {
	ModelProviders  map[string]models.ModelProvider
	ModelAliases    map[string][]string
	ModelTags       *models.ModelTagRegistry
	ToolSelection   conf.ToolSelectionConfig
	PromptGenerator core.PromptGenerator
	Tools           *tool.ToolRegistry
	VFS             apis.VFS
	VCS             apis.VCS
	Roles           *core.AgentRoleRegistry
	LSP             lsp.LSP
	ConfigStore     conf.ConfigStore
	TaskBackend     tool.TaskBackend
	mcpManager      interface{ Close() error }

	sessions   map[string]*core.SweSession
	threads    map[string]*core.SessionThread
	sessionsMu sync.RWMutex

	LogBaseDir           string
	SessionLoggerFactory SessionLoggerFactory
	WorkDir              string
	ShadowDir            string
	LogLLMRequests       bool
	LogLLMRequestsRaw    bool
	Thinking             string
	MaxToolThreads       int
	AllowAllPermissions  bool
}

// NewSession creates a new session for selected model.
func (s *SweSystem) NewSession(model string, outputHandler core.SessionThreadOutput) (*core.SweSession, error) {
	return s.newSessionWithOptions(model, outputHandler, "", "", "")
}

func (s *SweSystem) newSessionWithOptions(model string, outputHandler core.SessionThreadOutput, parentID string, slug string, thinking string) (*core.SweSession, error) {
	modelRefs, err := models.ExpandProviderModelChain(model, s.ModelAliases)
	if err != nil || len(modelRefs) == 0 {
		return nil, fmt.Errorf("SweSystem.NewSession() [system.go]: invalid model format, expected provider/model, comma-separated provider/model list, or model alias, got '%s'", model)
	}
	modelSpec := models.ComposeProviderModelSpec(modelRefs)

	for _, ref := range modelRefs {
		if _, exists := s.ModelProviders[ref.Provider]; !exists {
			return nil, fmt.Errorf("SweSystem.NewSession() [system.go]: provider not found: %s", ref.Provider)
		}
	}

	primaryRef := modelRefs[0]
	providerName := primaryRef.Provider
	modelName := primaryRef.Model

	provider := s.ModelProviders[providerName]

	sessionID := shared.GenerateUUIDv7()

	sessionLogger, llmLogger := s.createSessionLoggers(sessionID)
	if s.LogLLMRequestsRaw && strings.TrimSpace(s.LogBaseDir) != "" {
		rawPath := filepath.Join(s.LogBaseDir, "sessions", sessionID, "llm_requests_raw.log")
		if err := attachProviderRawLLMLogger(provider, providerName, modelName, rawPath); err != nil {
			if sessionLogger != nil {
				sessionLogger.Warn("failed to setup raw llm logger", "error", err)
			}
		}
	}
		session := core.NewSweSession(&core.SweSessionParams{
		ID:                   sessionID,
		ParentID:             strings.TrimSpace(parentID),
		Slug:                 strings.TrimSpace(slug),
		ModelSpec:            modelSpec,
		Provider:             provider,
		ProviderName:         providerName,
		Model:                modelName,
		VFS:                  s.VFS,
		BaseVFS:              s.VFS,
		LSP:                  s.LSP,
		SystemTools:          s.Tools,
		ModelProviders:       s.ModelProviders,
		ModelAliases:         s.ModelAliases,
		ModelTags:            s.ModelTags,
		ToolSelection:        s.ToolSelection,
		PromptGenerator:      s.PromptGenerator,
		Roles:                s.Roles,
		ConfigStore:          s.ConfigStore,
		OutputHandler:        outputHandler,
		WorkDir:              s.WorkDir,
		ShadowDir:            s.ShadowDir,
		LogBaseDir:           s.LogBaseDir,
		Thinking:             firstNonEmpty(strings.TrimSpace(thinking), strings.TrimSpace(s.Thinking)),
		MaxToolThreads:       s.MaxToolThreads,
		AllowAllPermissions:  s.AllowAllPermissions,
		TaskBackend:          s.TaskBackend,
		Logger:               sessionLogger,
		LLMLogger:            llmLogger,
		Messages:             []*models.ChatMessage{},
		TodoList:             []tool.TodoItem{},
		SubAgentRunner:       s,
	})

	if sessionLogger != nil {
		sessionLogger.Info("session_created", "session_id", sessionID, "provider", providerName, "model", modelName, "model_spec", modelSpec)
	}

	defaultRole, err := s.resolveDefaultRole()
	if err == nil && defaultRole != "" {
		if err := session.SetRole(defaultRole); err != nil {
			if sessionLogger != nil {
				sessionLogger.Warn("failed to set default role", "role", defaultRole, "error", err)
			}
		} else if sessionLogger != nil {
			sessionLogger.Info("default_role_set", "role", defaultRole)
		}
	}

	s.sessionsMu.Lock()
	if s.sessions == nil {
		s.sessions = make(map[string]*core.SweSession)
	}
	s.sessions[session.ID()] = session
	s.sessionsMu.Unlock()

	session.PersistSessionState()

	return session, nil
}

func attachProviderRawLLMLogger(provider models.ModelProvider, providerName string, modelName string, logPath string) error {
	if provider == nil {
		return fmt.Errorf("attachProviderRawLLMLogger() [system.go]: provider is nil")
	}
	if strings.TrimSpace(logPath) == "" {
		return fmt.Errorf("attachProviderRawLLMLogger() [system.go]: log path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("attachProviderRawLLMLogger() [system.go]: failed to create log directory: %w", err)
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("attachProviderRawLLMLogger() [system.go]: failed to open log file: %w", err)
	}
	if closeErr := file.Close(); closeErr != nil {
		return fmt.Errorf("attachProviderRawLLMLogger() [system.go]: failed to close log file probe: %w", closeErr)
	}
	prefix := strings.TrimSpace(providerName) + "/" + strings.TrimSpace(modelName)
	provider.SetRawLLMCallback(func(line string) {
		trimmed := strings.TrimRight(line, "\r\n")
		f, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if openErr != nil {
			return
		}
		if _, writeErr := f.WriteString(prefix + " " + trimmed + "\n"); writeErr != nil {
			_ = f.Close()
			return
		}
		_ = f.Close()
	})

	return nil
}

// ExecuteSubAgentTask executes delegated child-session task synchronously.
func (s *SweSystem) ExecuteSubAgentTask(parent *core.SweSession, request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error) {
	if parent == nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: parent session is nil")
	}

	if strings.TrimSpace(request.Slug) == "" {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: slug cannot be empty")
	}
	if strings.TrimSpace(request.Title) == "" {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: title cannot be empty")
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: prompt cannot be empty")
	}

	resolvedSlug, err := parent.ReserveUniqueSubAgentSlug(request.Slug)
	if err != nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: failed to reserve subagent slug: %w", err)
	}

	modelName := strings.TrimSpace(request.Model)
	resolvedRoleName := strings.TrimSpace(request.Role)
	if resolvedRoleName == "" {
		if parentRole := parent.Role(); parentRole != nil {
			resolvedRoleName = strings.TrimSpace(parentRole.Name)
		}
	}

	if modelName == "" && resolvedRoleName != "" && s.Roles != nil {
		if roleConfig, ok := s.Roles.Get(resolvedRoleName); ok {
			modelName = strings.TrimSpace(roleConfig.Model)
		}
	}
	if modelName == "" {
		modelName = strings.TrimSpace(parent.ModelWithProvider())
	}
	if modelName == "" {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: unable to resolve child model")
	}

	thinking := strings.TrimSpace(request.Thinking)
	if thinking == "" {
		thinking = strings.TrimSpace(parent.ThinkingLevel())
	}
	childOutput := &subAgentOutputHandler{delegate: parent.OutputHandler(), slug: resolvedSlug}
	child, err := s.newSessionWithOptions(modelName, childOutput, parent.ID(), resolvedSlug, thinking)
	if err != nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: failed to create child session: %w", err)
	}

	if roleName := strings.TrimSpace(request.Role); roleName != "" {
		if err := child.SetRole(roleName); err != nil {
			return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: failed to set child role: %w", err)
		}
	} else if parentRole := parent.Role(); parentRole != nil {
		if err := child.SetRole(parentRole.Name); err != nil {
			return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: failed to inherit parent role: %w", err)
		}
	}

	if err := child.UserPrompt(request.Prompt); err != nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: failed to submit child prompt: %w", err)
	}

	runErr := child.Run(context.Background())
	summaryText := core.LastAssistantMessageText(child)
	finalTodo := child.GetTodoList()
	status := "completed"
	if runErr != nil {
		status = "error"
	}

	if err := core.WriteSubAgentSummary(s.LogBaseDir, child, core.SubAgentSummaryJSON{
		SessionID:       child.ID(),
		ParentSessionID: parent.ID(),
		Status:          status,
		Summary:         summaryText,
		FinalTodoList:   finalTodo,
		ModelUsed:       strings.TrimSpace(child.ModelWithProvider()),
		ThinkingLevel:   strings.TrimSpace(child.ThinkingLevel()),
		CompletedAt:     time.Now().Format(time.RFC3339Nano),
	}); err != nil && child.OutputHandler() != nil {
		child.OutputHandler().ShowMessage(fmt.Sprintf("Failed to write subagent summary: %v", err), "warning")
	}

	if runErr != nil {
		return tool.SubAgentTaskResult{Status: "error", Summary: summaryText, Error: runErr.Error()}, nil
	}

	return tool.SubAgentTaskResult{Status: "completed", Summary: summaryText}, nil
}

type subAgentOutputHandler struct {
	delegate core.SessionThreadOutput
	slug     string
}

func (h *subAgentOutputHandler) ShowMessage(message string, messageType string) {
	if h.delegate == nil {
		return
	}
	h.delegate.ShowMessage(prefixSubAgentMessage(h.slug, message), messageType)
}

func (h *subAgentOutputHandler) AddAssistantMessage(text string, thinking string) {
	if h.delegate == nil {
		return
	}
	h.delegate.AddAssistantMessage(prefixSubAgentMessage(h.slug, text), prefixSubAgentMessage(h.slug, thinking))
}

func (h *subAgentOutputHandler) AddUserMessage(text string) {
	if h.delegate == nil {
		return
	}
	h.delegate.AddUserMessage(prefixSubAgentMessage(h.slug, text))
}

func (h *subAgentOutputHandler) AddToolCall(call *tool.ToolCall) {
	if h.delegate == nil {
		return
	}
	h.delegate.AddToolCall(withSubAgentSlugOnToolCall(call, h.slug))
}

func (h *subAgentOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	if h.delegate == nil {
		return
	}
	if result == nil {
		h.delegate.AddToolCallResult(nil)
		return
	}

	h.delegate.AddToolCallResult(&tool.ToolResponse{
		Call:   withSubAgentSlugOnToolCall(result.Call, h.slug),
		Error:  result.Error,
		Result: result.Result,
		Done:   result.Done,
	})
}

func (h *subAgentOutputHandler) RunFinished(err error) {
	if h.delegate == nil {
		return
	}
	h.delegate.RunFinished(err)
}

func (h *subAgentOutputHandler) OnRateLimitError(retryAfterSeconds int) {
	if h.delegate == nil {
		return
	}
	h.delegate.OnRateLimitError(retryAfterSeconds)
}

func (h *subAgentOutputHandler) ShouldRetryAfterFailure(message string) bool {
	if h.delegate == nil {
		return false
	}
	return h.delegate.ShouldRetryAfterFailure(prefixSubAgentMessage(h.slug, message))
}

func prefixSubAgentMessage(slug string, message string) string {
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" || strings.TrimSpace(message) == "" {
		return message
	}
	lines := strings.Split(message, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = fmt.Sprintf("*%s* %s", trimmedSlug, line)
	}
	return strings.Join(lines, "\n")
}

func withSubAgentSlugOnToolCall(call *tool.ToolCall, slug string) *tool.ToolCall {
	if call == nil {
		return nil
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return call
	}

	args := make(map[string]any)
	if obj := call.Arguments.Object(); obj != nil {
		for key, value := range obj {
			args[key] = value.Raw()
		}
	}
	args["__subagent_slug"] = trimmedSlug

	return &tool.ToolCall{
		ID:        call.ID,
		Function:  call.Function,
		Arguments: tool.NewToolValue(args),
		Access:    call.Access,
	}
}

func (s *SweSystem) createSessionLoggers(sessionID string) (*slog.Logger, *slog.Logger) {
	var sessionLogger *slog.Logger
	if s.SessionLoggerFactory != nil {
		logger, err := s.SessionLoggerFactory(sessionID, s.LogBaseDir)
		if err != nil {
			logging.GetGlobalLogger().Error("failed to create session logger", "session_id", sessionID, "error", err)
		} else {
			sessionLogger = logger
		}
	} else {
		sessionLogger = logging.GetSessionLogger(sessionID, logging.LogTypeSession)
	}

	var llmLogger *slog.Logger
	if s.LogLLMRequests {
		llmLogger = logging.GetSessionLogger(sessionID, logging.LogTypeLLM)
	}

	return sessionLogger, llmLogger
}

func (s *SweSystem) buildSessionParams() *core.SweSessionParams {
	return &core.SweSessionParams{
		VFS:             s.VFS,
		BaseVFS:         s.VFS,
		LSP:             s.LSP,
		SystemTools:     s.Tools,
		ModelProviders:  s.ModelProviders,
		ModelAliases:    s.ModelAliases,
		ModelTags:       s.ModelTags,
		ToolSelection:   s.ToolSelection,
		PromptGenerator: s.PromptGenerator,
		Roles:           s.Roles,
		ConfigStore:     s.ConfigStore,
		LogBaseDir:      s.LogBaseDir,
		ShadowDir:       s.ShadowDir,
		Thinking:        s.Thinking,
		MaxToolThreads:  s.MaxToolThreads,
		AllowAllPermissions: s.AllowAllPermissions,
		SubAgentRunner:  s,
		TaskBackend:     s.TaskBackend,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func (s *SweSystem) resolveDefaultRole() (string, error) {
	if s.ConfigStore != nil {
		globalConfig, err := s.ConfigStore.GetGlobalConfig()
		if err == nil && globalConfig != nil && globalConfig.Defaults.DefaultRole != "" {
			if s.Roles != nil {
				if _, ok := s.Roles.Get(globalConfig.Defaults.DefaultRole); ok {
					return globalConfig.Defaults.DefaultRole, nil
				}
			}
		}
	}

	if s.Roles != nil {
		if _, ok := s.Roles.Get("developer"); ok {
			return "developer", nil
		}
	}

	return "", nil
}

// GetSession returns the session with the given ID.
func (s *SweSystem) GetSession(id string) (*core.SweSession, error) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("SweSystem.GetSession() [system.go]: session not found: %s", id)
	}

	return session, nil
}

// GetSessionThread returns the SessionThread for the given session ID.
func (s *SweSystem) GetSessionThread(id string) (*core.SessionThread, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("SweSystem.GetSessionThread() [system.go]: session not found: %s", id)
	}

	if s.threads == nil {
		s.threads = make(map[string]*core.SessionThread)
	}

	thread, ok := s.threads[id]
	if !ok {
		thread = core.NewSessionThreadWithSession(s, session, session.OutputHandler())
		s.threads[id] = thread
	}

	return thread, nil
}

// ListSessions returns a list of all active sessions.
func (s *SweSystem) ListSessions() []*core.SweSession {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	sessions := make([]*core.SweSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// DeleteSession deletes the session with the given ID.
func (s *SweSystem) DeleteSession(id string) error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if _, ok := s.sessions[id]; !ok {
		return fmt.Errorf("SweSystem.DeleteSession() [system.go]: session not found: %s", id)
	}

	delete(s.threads, id)
	logging.CloseSessionLogger(id)
	delete(s.sessions, id)
	return nil
}

// Shutdown interrupts all running sessions and deletes all sessions and threads.
func (s *SweSystem) Shutdown() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	for _, thread := range s.threads {
		_ = thread.Interrupt()
	}

	logging.FlushLogs()
	logging.CloseSessionLoggers()
	if s.mcpManager != nil {
		_ = s.mcpManager.Close()
	}

	s.threads = make(map[string]*core.SessionThread)
	s.sessions = make(map[string]*core.SweSession)
}
