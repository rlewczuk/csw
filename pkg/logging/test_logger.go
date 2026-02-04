package logging

import (
	"bytes"
	"log/slog"
	"sync"
	"testing"
)

// TestLoggerData holds the in-memory log data for a test session.
type TestLoggerData struct {
	t         *testing.T
	sessionID string
	mu        sync.Mutex

	// Buffers for each logger
	sessionBuf *bytes.Buffer
	llmBuf     *bytes.Buffer
}

var (
	testLoggers   map[string]*TestLoggerData
	testLoggersMu sync.RWMutex
)

func init() {
	testLoggers = make(map[string]*TestLoggerData)
}

// NewTestLogger creates test loggers that store logs in memory.
// Returns three *slog.Logger instances: session, chat, and llm.
// These loggers will store all logs in memory and can be inspected using GetTestSessionLogs, etc.
func NewTestLogger(t *testing.T, sessionID string) (*slog.Logger, *slog.Logger) {
	data := &TestLoggerData{
		t:          t,
		sessionID:  sessionID,
		sessionBuf: &bytes.Buffer{},
		llmBuf:     &bytes.Buffer{},
	}

	// Create handlers for each buffer
	sessionHandler := slog.NewJSONHandler(data.sessionBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	llmHandler := slog.NewJSONHandler(data.llmBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	sessionLogger := slog.New(sessionHandler).With("session_id", sessionID)
	llmLogger := slog.New(llmHandler).With("session_id", sessionID)

	// Store data for later retrieval
	testLoggersMu.Lock()
	testLoggers[sessionID] = data
	testLoggersMu.Unlock()

	return sessionLogger, llmLogger
}

// NewTestLoggerFactory creates a SessionLoggerFactory that returns test logger instances.
// This is useful for tests that need to use the SweSystem with in-memory logging.
// The logBaseDir parameter is ignored since test loggers don't write to files.
func NewTestLoggerFactory(t *testing.T) func(sessionID string, logBaseDir string) (*slog.Logger, error) {
	return func(sessionID string, logBaseDir string) (*slog.Logger, error) {
		sessionLog, _ := NewTestLogger(t, sessionID)
		return sessionLog, nil
	}
}

// FlushTestLogger prints all buffered logs to the test output.
// This should be called in test cleanup or when a test fails.
func FlushTestLogger(sessionID string) {
	testLoggersMu.RLock()
	data, ok := testLoggers[sessionID]
	testLoggersMu.RUnlock()

	if !ok {
		return
	}

	data.mu.Lock()
	defer data.mu.Unlock()

	if data.sessionBuf.Len() > 0 {
		data.t.Logf("Session logs:\n%s", data.sessionBuf.String())
	}
	if data.llmBuf.Len() > 0 {
		data.t.Logf("LLM logs:\n%s", data.llmBuf.String())
	}
}

// GetTestSessionBuffer returns the session log buffer for inspection
func GetTestSessionBuffer(sessionID string) *bytes.Buffer {
	testLoggersMu.RLock()
	data, ok := testLoggers[sessionID]
	testLoggersMu.RUnlock()

	if !ok {
		return nil
	}

	data.mu.Lock()
	defer data.mu.Unlock()

	return data.sessionBuf
}

// GetTestLLMBuffer returns the LLM log buffer for inspection
func GetTestLLMBuffer(sessionID string) *bytes.Buffer {
	testLoggersMu.RLock()
	data, ok := testLoggers[sessionID]
	testLoggersMu.RUnlock()

	if !ok {
		return nil
	}

	data.mu.Lock()
	defer data.mu.Unlock()

	return data.llmBuf
}
