package core

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/logging"
	"github.com/codesnort/codesnort-swe/pkg/lsp"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/shared"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// SessionLoggerFactory is a function that creates a session logger.
// This allows tests to provide in-memory loggers instead of file-based ones.
// Returns a *slog.Logger that will be used for session logging.
type SessionLoggerFactory func(sessionID string, logBaseDir string) (*slog.Logger, error)

// SweSystem represents the core system for managing conversations, tools, and models.
type SweSystem struct {

	// Map of model providers by name
	ModelProviders map[string]models.ModelProvider

	// Model tag registry for assigning tags to models
	ModelTags *models.ModelTagRegistry

	// Prompt generator
	PromptGenerator PromptGenerator

	// Tool registry
	Tools *tool.ToolRegistry

	// Virtual filesystem
	VFS vfs.VFS

	// Roles
	Roles *AgentRoleRegistry

	// LSP client for code validation and analysis (optional, can be nil)
	LSP lsp.LSP

	// Map of sessions by ID
	sessions map[string]*SweSession

	// Map of session threads by session ID
	threads map[string]*SessionThread

	// Mutex for thread-safe session access
	sessionsMu sync.RWMutex

	// Base directory for log files
	LogBaseDir string

	// SessionLoggerFactory is used to create session loggers.
	// If nil, defaults to file-based logging.
	SessionLoggerFactory SessionLoggerFactory
}

func (s *SweSystem) NewSession(model string, outputHandler SessionThreadOutput) (*SweSession, error) {
	// Parse provider/model format (e.g., "ollama/devstral-small-2:latest")
	var providerName, modelName string
	for i, c := range model {
		if c == '/' {
			providerName = model[:i]
			modelName = model[i+1:]
			break
		}
	}

	if providerName == "" || modelName == "" {
		return nil, fmt.Errorf("SweSystem.NewSession() [system.go]: invalid model format, expected 'provider/model', got '%s'", model)
	}

	provider, ok := s.ModelProviders[providerName]
	if !ok {
		return nil, fmt.Errorf("SweSystem.NewSession() [system.go]: provider not found: %s", providerName)
	}

	// Create a new tool registry for the session by copying system tools
	sessionTools := tool.NewToolRegistry()
	for _, name := range s.Tools.List() {
		t, _ := s.Tools.Get(name)
		sessionTools.Register(name, t)
	}

	sessionID := shared.GenerateUUIDv7()

	// Create session logger
	var sessionLogger *slog.Logger

	// Use custom logger factory if provided, otherwise use default file-based logger
	if s.SessionLoggerFactory != nil {
		var err error
		sessionLogger, err = s.SessionLoggerFactory(sessionID, s.LogBaseDir)
		if err != nil {
			// Log error but don't fail session creation
			logger := logging.GetGlobalLogger()
			logger.Error("failed to create session logger", "session_id", sessionID, "error", err)
		}
	} else {
		// Use the new GetSessionLogger function
		sessionLogger = logging.GetSessionLogger(sessionID, logging.LogTypeSession)
	}

	session := &SweSession{
		id:            sessionID,
		system:        s,
		provider:      provider,
		providerName:  providerName,
		model:         modelName,
		messages:      []*models.ChatMessage{},
		role:          nil,
		VFS:           s.VFS,
		LSP:           s.LSP,
		Tools:         sessionTools,
		outputHandler: outputHandler,
		workDir:       ".",
		todoList:      make([]tool.TodoItem, 0),
		logger:        sessionLogger,
	}

	// Log session creation
	if sessionLogger != nil {
		sessionLogger.Info("session_created", "session_id", sessionID, "provider", providerName, "model", modelName)
	}

	// Register session-specific tools (like todo tools)
	session.registerSessionTools()

	// Store session
	s.sessionsMu.Lock()
	if s.sessions == nil {
		s.sessions = make(map[string]*SweSession)
	}
	s.sessions[session.id] = session
	s.sessionsMu.Unlock()

	return session, nil
}

// GetSession returns the session with the given ID.
// Returns an error if the session is not found.
func (s *SweSystem) GetSession(id string) (*SweSession, error) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("SweSystem.GetSession() [system.go]: session not found: %s", id)
	}

	return session, nil
}

// GetSessionThread returns the SessionThread for the given session ID.
// If the thread doesn't exist yet, it creates a new one with the session.
// Returns an error if the session is not found.
func (s *SweSystem) GetSessionThread(id string) (*SessionThread, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	// Check if session exists
	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("SweSystem.GetSessionThread() [system.go]: session not found: %s", id)
	}

	// Check if thread already exists
	if s.threads == nil {
		s.threads = make(map[string]*SessionThread)
	}

	thread, ok := s.threads[id]
	if !ok {
		// Create a new thread with the existing session
		// Use the session's existing output handler
		thread = NewSessionThreadWithSession(s, session, session.outputHandler)
		s.threads[id] = thread
	}

	return thread, nil
}

// ListSessions returns a list of all active sessions.
func (s *SweSystem) ListSessions() []*SweSession {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	sessions := make([]*SweSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// DeleteSession deletes the session with the given ID.
// Returns an error if the session is not found.
func (s *SweSystem) DeleteSession(id string) error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if _, ok := s.sessions[id]; !ok {
		return fmt.Errorf("SweSystem.DeleteSession() [system.go]: session not found: %s", id)
	}

	// Close session loggers
	logging.CloseSessionLogger(id)

	delete(s.sessions, id)
	return nil
}

// Shutdown interrupts all running sessions and deletes all sessions and threads.
// This method is thread-safe and will attempt to interrupt all threads even if some fail.
func (s *SweSystem) Shutdown() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	// Interrupt all running threads
	for _, thread := range s.threads {
		// Ignore errors since thread might not be running
		_ = thread.Interrupt()
	}

	// Flush and close all loggers
	logging.FlushLogs()
	logging.CloseSessionLoggers()

	// Clear all threads
	s.threads = make(map[string]*SessionThread)

	// Clear all sessions
	s.sessions = make(map[string]*SweSession)
}
