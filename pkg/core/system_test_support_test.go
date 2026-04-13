package core

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

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
)

// SessionLoggerFactory creates session logger used by tests.
type SessionLoggerFactory func(sessionID string, logBaseDir string) (*slog.Logger, error)

// SweSystem is a test-only system implementation used by core package tests.
type SweSystem struct {
	ModelProviders  map[string]models.ModelProvider
	ModelAliases    map[string][]string
	ModelTags       *models.ModelTagRegistry
	ToolSelection   conf.ToolSelectionConfig
	PromptGenerator PromptGenerator
	Tools           *tool.ToolRegistry
	VFS             apis.VFS
	Roles           *AgentRoleRegistry
	LSP             lsp.LSP
	ConfigStore     conf.ConfigStore

	sessions   map[string]*SweSession
	threads    map[string]*SessionThread
	sessionsMu sync.RWMutex

	LogBaseDir           string
	SessionLoggerFactory SessionLoggerFactory
	WorkDir              string
	ShadowDir            string
	LogLLMRequests       bool
	Thinking             string
}

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

// NewSession creates new session for tests.
func (s *SweSystem) NewSession(model string, outputHandler SessionThreadOutput) (*SweSession, error) {
	return s.newSessionWithOptions(model, outputHandler, "", "", "")
}

func (s *SweSystem) newSessionWithOptions(model string, outputHandler SessionThreadOutput, parentID string, slug string, thinking string) (*SweSession, error) {
	modelRefs, err := models.ExpandProviderModelChain(model, s.ModelAliases)
	if err != nil || len(modelRefs) == 0 {
		return nil, fmt.Errorf("SweSystem.NewSession() [system_test_support_test.go]: invalid model format, expected provider/model, comma-separated provider/model list, or model alias, got '%s'", model)
	}
	for _, ref := range modelRefs {
		if _, ok := s.ModelProviders[ref.Provider]; !ok {
			return nil, fmt.Errorf("SweSystem.NewSession() [system_test_support_test.go]: provider not found: %s", ref.Provider)
		}
	}

	providerName := modelRefs[0].Provider
	modelName := modelRefs[0].Model

	provider := s.ModelProviders[providerName]

	sessionID := shared.GenerateUUIDv7()
	sessionLogger, llmLogger := s.createSessionLoggers(sessionID)

	session := NewSweSession(&SweSessionParams{
		ID:              sessionID,
		ParentID:        strings.TrimSpace(parentID),
		Slug:            strings.TrimSpace(slug),
		ModelSpec:       strings.TrimSpace(model),
		Provider:        provider,
		ProviderName:    providerName,
		Model:           modelName,
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

	defaultRole, err := s.resolveDefaultRole()
	if err == nil && defaultRole != "" {
		_ = session.SetRole(defaultRole)
	}

	s.sessionsMu.Lock()
	if s.sessions == nil {
		s.sessions = make(map[string]*SweSession)
	}
	if s.threads == nil {
		s.threads = make(map[string]*SessionThread)
	}
	s.sessions[session.ID()] = session
	s.sessionsMu.Unlock()

	session.PersistSessionState()

	return session, nil
}

// ExecuteSubAgentTask executes delegated child-session task synchronously.
func (s *SweSystem) ExecuteSubAgentTask(parent *SweSession, request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error) {
	if parent == nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system_test_support_test.go]: parent session is nil")
	}

	if strings.TrimSpace(request.Slug) == "" {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system_test_support_test.go]: slug cannot be empty")
	}
	if strings.TrimSpace(request.Title) == "" {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system_test_support_test.go]: title cannot be empty")
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system_test_support_test.go]: prompt cannot be empty")
	}

	resolvedSlug, err := parent.ReserveUniqueSubAgentSlug(request.Slug)
	if err != nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system_test_support_test.go]: failed to reserve subagent slug: %w", err)
	}

	modelName := strings.TrimSpace(request.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(parent.ModelWithProvider())
	}
	if modelName == "" {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system_test_support_test.go]: unable to resolve child model")
	}

	thinking := firstNonEmpty(strings.TrimSpace(request.Thinking), strings.TrimSpace(parent.ThinkingLevel()))
	childOutput := &subAgentOutputHandler{delegate: parent.OutputHandler(), slug: resolvedSlug}
	child, err := s.newSessionWithOptions(modelName, childOutput, parent.ID(), resolvedSlug, thinking)
	if err != nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system_test_support_test.go]: failed to create child session: %w", err)
	}

	if roleName := strings.TrimSpace(request.Role); roleName != "" {
		if err := child.SetRole(roleName); err != nil {
			return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system_test_support_test.go]: failed to set child role: %w", err)
		}
	} else if parentRole := parent.Role(); parentRole != nil {
		if err := child.SetRole(parentRole.Name); err != nil {
			return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system_test_support_test.go]: failed to inherit parent role: %w", err)
		}
	}

	if err := child.UserPrompt(request.Prompt); err != nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSystem.ExecuteSubAgentTask() [system_test_support_test.go]: failed to submit child prompt: %w", err)
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
		return tool.SubAgentTaskResult{
			Status:  "error",
			Summary: summaryText,
		}, nil
	}

	return tool.SubAgentTaskResult{Status: "completed", Summary: summaryText}, nil
}

func writeSubAgentSummary(logBaseDir string, session *SweSession, summary subAgentSummaryJSON) error {
	if session == nil {
		return fmt.Errorf("writeSubAgentSummary() [system_test_support_test.go]: session is nil")
	}
	if strings.TrimSpace(logBaseDir) == "" {
		return nil
	}

	dir := filepath.Join(logBaseDir, "sessions", session.ID())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("writeSubAgentSummary() [system_test_support_test.go]: failed to create session summary dir: %w", err)
	}

	jsonData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("writeSubAgentSummary() [system_test_support_test.go]: failed to marshal summary json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), jsonData, 0644); err != nil {
		return fmt.Errorf("writeSubAgentSummary() [system_test_support_test.go]: failed to write summary json: %w", err)
	}

	markdown := strings.TrimSpace(summary.Summary)
	if markdown == "" {
		markdown = "(no summary)"
	}
	content := fmt.Sprintf("# Summary\n\n%s\n\n# Session Info\n\nSession ID: %s\nParent Session ID: %s\nStatus: %s\n", markdown, summary.SessionID, summary.ParentSessionID, summary.Status)
	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("writeSubAgentSummary() [system_test_support_test.go]: failed to write summary markdown: %w", err)
	}

	return nil
}

func lastAssistantMessageText(session *SweSession) string {
	if session == nil {
		return ""
	}

	messages := session.ChatMessages()
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type subAgentOutputHandler struct {
	delegate SessionThreadOutput
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
		if err == nil {
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

func (s *SweSystem) buildSessionParams() *SweSessionParams {
	return &SweSessionParams{
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

// GetSession returns stored session.
func (s *SweSystem) GetSession(id string) (*SweSession, error) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("SweSystem.GetSession() [system_test_support_test.go]: session not found: %s", id)
	}

	return session, nil
}

// GetSessionThread returns session thread for given id.
func (s *SweSystem) GetSessionThread(id string) (*SessionThread, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("SweSystem.GetSessionThread() [system_test_support_test.go]: session not found: %s", id)
	}

	if s.threads == nil {
		s.threads = make(map[string]*SessionThread)
	}

	thread, ok := s.threads[id]
	if !ok {
		thread = NewSessionThreadWithSession(s, session, session.OutputHandler())
		s.threads[id] = thread
	}

	return thread, nil
}

// ListSessions returns all sessions.
func (s *SweSystem) ListSessions() []*SweSession {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	sessions := make([]*SweSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// DeleteSession deletes stored session.
func (s *SweSystem) DeleteSession(id string) error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if _, ok := s.sessions[id]; !ok {
		return fmt.Errorf("SweSystem.DeleteSession() [system_test_support_test.go]: session not found: %s", id)
	}

	delete(s.threads, id)
	logging.CloseSessionLogger(id)
	delete(s.sessions, id)
	return nil
}

// Shutdown clears sessions and threads.
func (s *SweSystem) Shutdown() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	for _, thread := range s.threads {
		_ = thread.Interrupt()
	}

	logging.FlushLogs()
	logging.CloseSessionLoggers()
	s.threads = make(map[string]*SessionThread)
	s.sessions = make(map[string]*SweSession)
}
