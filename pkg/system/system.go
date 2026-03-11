package system

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
)

type subAgentSummaryJSON struct {
	SessionID       string          `json:"session_id"`
	ParentSessionID string          `json:"parent_session_id,omitempty"`
	Status          string          `json:"status"`
	Summary         string          `json:"summary,omitempty"`
	FinalTodoList   []tool.TodoItem `json:"final_todo_list"`
	ModelUsed       string          `json:"model_used,omitempty"`
	ThinkingLevel   string          `json:"thinking_level,omitempty"`
	CompletedAt     string          `json:"completed_at"`
}

// SessionLoggerFactory is a function that creates a session logger.
// This allows tests to provide in-memory loggers instead of file-based ones.
type SessionLoggerFactory func(sessionID string, logBaseDir string) (*slog.Logger, error)

// SweSystem represents the core system for managing conversations, tools, and models.
type SweSystem struct {
	ModelProviders  map[string]models.ModelProvider
	ModelTags       *models.ModelTagRegistry
	ToolSelection   conf.ToolSelectionConfig
	PromptGenerator core.PromptGenerator
	Tools           *tool.ToolRegistry
	VFS             vfs.VFS
	Roles           *core.AgentRoleRegistry
	LSP             lsp.LSP
	ConfigStore     conf.ConfigStore

	sessions   map[string]*core.SweSession
	threads    map[string]*core.SessionThread
	sessionsMu sync.RWMutex

	LogBaseDir           string
	SessionLoggerFactory SessionLoggerFactory
	WorkDir              string
	ShadowDir            string
	LogLLMRequests       bool
	Thinking             string
}

// LoadSession loads a persisted session from disk and registers it in memory.
func (s *SweSystem) LoadSession(id string, outputHandler core.SessionThreadOutput) (*core.SweSession, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if existing, ok := s.sessions[id]; ok {
		existing.SetOutputHandler(outputHandler)
		return existing, nil
	}

	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("SweSystem.LoadSession() [system.go]: session id cannot be empty")
	}

	statePath := filepath.Join(s.LogBaseDir, "sessions", id, "session.json")
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("SweSystem.LoadSession() [system.go]: failed to read session state file: %w", err)
	}

	var state core.PersistedSessionState
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		return nil, fmt.Errorf("SweSystem.LoadSession() [system.go]: failed to unmarshal session state: %w", err)
	}

	params := s.buildSessionParams()
	params.Logger = logging.GetSessionLogger(state.SessionID, logging.LogTypeSession)
	if s.LogLLMRequests {
		params.LLMLogger = logging.GetSessionLogger(state.SessionID, logging.LogTypeLLM)
	}

	session, err := core.RestoreSessionFromPersistedState(params, state, outputHandler)
	if err != nil {
		return nil, fmt.Errorf("SweSystem.LoadSession() [system.go]: failed to restore session: %w", err)
	}

	if s.sessions == nil {
		s.sessions = make(map[string]*core.SweSession)
	}
	s.sessions[session.ID()] = session

	return session, nil
}

// LoadLastSession loads the most recently updated persisted session.
func (s *SweSystem) LoadLastSession(outputHandler core.SessionThreadOutput) (*core.SweSession, error) {
	if strings.TrimSpace(s.LogBaseDir) == "" {
		return nil, fmt.Errorf("SweSystem.LoadLastSession() [system.go]: LogBaseDir is empty")
	}

	sessionsRoot := filepath.Join(s.LogBaseDir, "sessions")
	entries, err := os.ReadDir(sessionsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("SweSystem.LoadLastSession() [system.go]: no persisted sessions found")
		}
		return nil, fmt.Errorf("SweSystem.LoadLastSession() [system.go]: failed to read sessions directory: %w", err)
	}

	type persistedSessionFile struct {
		id      string
		modTime int64
	}

	latest := persistedSessionFile{}
	found := false

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		statePath := filepath.Join(sessionsRoot, sessionID, "session.json")
		info, statErr := os.Stat(statePath)
		if statErr != nil {
			continue
		}

		modTime := info.ModTime().UnixNano()
		if !found || modTime > latest.modTime {
			latest = persistedSessionFile{id: sessionID, modTime: modTime}
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("SweSystem.LoadLastSession() [system.go]: no persisted sessions found")
	}

	return s.LoadSession(latest.id, outputHandler)
}

// NewSession creates a new session for selected model.
func (s *SweSystem) NewSession(model string, outputHandler core.SessionThreadOutput) (*core.SweSession, error) {
	return s.newSessionWithOptions(model, outputHandler, "", "", "")
}

func (s *SweSystem) newSessionWithOptions(model string, outputHandler core.SessionThreadOutput, parentID string, slug string, thinking string) (*core.SweSession, error) {
	providerName, modelName, err := parseProviderModel(model)
	if err != nil {
		return nil, err
	}

	provider, ok := s.ModelProviders[providerName]
	if !ok {
		return nil, fmt.Errorf("SweSystem.NewSession() [system.go]: provider not found: %s", providerName)
	}

	sessionID := shared.GenerateUUIDv7()

	sessionLogger, llmLogger := s.createSessionLoggers(sessionID)
		session := core.NewSweSession(&core.SweSessionParams{
		ID:              sessionID,
		ParentID:        strings.TrimSpace(parentID),
		Slug:            strings.TrimSpace(slug),
		Provider:        provider,
		ProviderName:    providerName,
		Model:           modelName,
		VFS:             s.VFS,
		BaseVFS:         s.VFS,
		LSP:             s.LSP,
		SystemTools:     s.Tools,
		ModelProviders:  s.ModelProviders,
		ModelTags:       s.ModelTags,
		ToolSelection:   s.ToolSelection,
		PromptGenerator: s.PromptGenerator,
		Roles:           s.Roles,
		ConfigStore:     s.ConfigStore,
		OutputHandler:   outputHandler,
		WorkDir:         s.WorkDir,
		ShadowDir:       s.ShadowDir,
		LogBaseDir:      s.LogBaseDir,
		Thinking:        firstNonEmpty(strings.TrimSpace(thinking), strings.TrimSpace(s.Thinking)),
		Logger:          sessionLogger,
		LLMLogger:       llmLogger,
		Messages:        []*models.ChatMessage{},
		TodoList:        []tool.TodoItem{},
		SubAgentRunner:  s,
	})

	if sessionLogger != nil {
		sessionLogger.Info("session_created", "session_id", sessionID, "provider", providerName, "model", modelName)
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

	if err := parent.ReserveSubAgentSlug(request.Slug); err != nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system.go]: failed to reserve subagent slug: %w", err)
	}

	modelName := strings.TrimSpace(request.Model)
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
	childOutput := &subAgentOutputHandler{delegate: parent.OutputHandler(), slug: request.Slug}
	child, err := s.newSessionWithOptions(modelName, childOutput, parent.ID(), request.Slug, thinking)
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
	summaryText := lastAssistantMessageText(child)
	finalTodo := child.GetTodoList()
	status := "completed"
	if runErr != nil {
		status = "error"
	}

	if err := writeSubAgentSummary(s.LogBaseDir, child, subAgentSummaryJSON{
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
		return tool.SubAgentTaskResult{Status: "error", Summary: summaryText}, nil
	}

	return tool.SubAgentTaskResult{Status: "completed", Summary: summaryText}, nil
}

func writeSubAgentSummary(logBaseDir string, session *core.SweSession, summary subAgentSummaryJSON) error {
	if session == nil || strings.TrimSpace(logBaseDir) == "" {
		return nil
	}
	dir := filepath.Join(logBaseDir, "sessions", session.ID())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("writeSubAgentSummary() [system.go]: failed to create session summary dir: %w", err)
	}
	jsonData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("writeSubAgentSummary() [system.go]: failed to marshal summary json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), jsonData, 0644); err != nil {
		return fmt.Errorf("writeSubAgentSummary() [system.go]: failed to write summary json: %w", err)
	}

	markdown := strings.TrimSpace(summary.Summary)
	if markdown == "" {
		markdown = "(no summary)"
	}
	content := fmt.Sprintf("# Summary\n\n%s\n\n# Session Info\n\nSession ID: %s\nParent Session ID: %s\nStatus: %s\n", markdown, summary.SessionID, summary.ParentSessionID, summary.Status)
	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("writeSubAgentSummary() [system.go]: failed to write summary markdown: %w", err)
	}

	return nil
}

func lastAssistantMessageText(session *core.SweSession) string {
	if session == nil {
		return ""
	}
	for i := len(session.ChatMessages()) - 1; i >= 0; i-- {
		message := session.ChatMessages()[i]
		if message == nil || message.Role != models.ChatRoleAssistant {
			continue
		}
		var textBuilder strings.Builder
		for _, part := range message.Parts {
			if part.Text != "" {
				textBuilder.WriteString(part.Text)
			}
		}
		if textBuilder.Len() > 0 {
			return textBuilder.String()
		}
		for _, part := range message.Parts {
			if part.ReasoningContent != "" {
				textBuilder.WriteString(part.ReasoningContent)
			}
		}
		return textBuilder.String()
	}
	return ""
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

func (h *subAgentOutputHandler) AddToolCall(call *tool.ToolCall) {
	if h.delegate == nil {
		return
	}
	h.delegate.AddToolCall(call)
}

func (h *subAgentOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	if h.delegate == nil {
		return
	}
	h.delegate.AddToolCallResult(result)
}

func (h *subAgentOutputHandler) RunFinished(err error) {
	if h.delegate == nil {
		return
	}
	h.delegate.RunFinished(err)
}

func (h *subAgentOutputHandler) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	if h.delegate == nil {
		return
	}
	h.delegate.OnPermissionQuery(query)
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
		ModelTags:       s.ModelTags,
		ToolSelection:   s.ToolSelection,
		PromptGenerator: s.PromptGenerator,
		Roles:           s.Roles,
		ConfigStore:     s.ConfigStore,
		LogBaseDir:      s.LogBaseDir,
		ShadowDir:       s.ShadowDir,
		Thinking:        s.Thinking,
		SubAgentRunner:  s,
	}
}

func parseProviderModel(model string) (string, string, error) {
	for i, c := range model {
		if c == '/' {
			providerName := model[:i]
			modelName := model[i+1:]
			if providerName == "" || modelName == "" {
				break
			}
			return providerName, modelName, nil
		}
	}

	return "", "", fmt.Errorf("SweSystem.NewSession() [system.go]: invalid model format, expected 'provider/model', got '%s'", model)
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
		if err == nil && globalConfig != nil && globalConfig.DefaultRole != "" {
			if s.Roles != nil {
				if _, ok := s.Roles.Get(globalConfig.DefaultRole); ok {
					return globalConfig.DefaultRole, nil
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

	s.threads = make(map[string]*core.SessionThread)
	s.sessions = make(map[string]*core.SweSession)
}
