package system

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
)

// SessionLoggerFactory is a function that creates a session logger.
// This allows tests to provide in-memory loggers instead of file-based ones.
type SessionLoggerFactory func(sessionID string, logBaseDir string) (*slog.Logger, error)

// SweSystem represents the core system for managing conversations, tools, and models.
type SweSystem struct {
	ModelProviders map[string]models.ModelProvider
	ModelTags      *models.ModelTagRegistry
	ToolSelection  conf.ToolSelectionConfig
	PromptGenerator core.PromptGenerator
	Tools          *tool.ToolRegistry
	VFS            vfs.VFS
	Roles          *core.AgentRoleRegistry
	LSP            lsp.LSP
	ConfigStore    conf.ConfigStore

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
		Thinking:        s.Thinking,
		Logger:          sessionLogger,
		LLMLogger:       llmLogger,
		Messages:        []*models.ChatMessage{},
		TodoList:        []tool.TodoItem{},
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
