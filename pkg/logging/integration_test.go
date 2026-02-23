package logging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogDirectoryStructure verifies the complete log directory structure
func TestLogDirectoryStructure(t *testing.T) {
	tmpDir := filepath.Join("../../tmp", "test_integration_logs")
	defer os.RemoveAll(tmpDir)

	// Set logs directory
	err := SetLogsDirectory(tmpDir, false)
	require.NoError(t, err)

	// Get global logger and write a log to create the file
	globalLogger := GetGlobalLogger()
	globalLogger.Info("test_global_log")

	// Create session loggers (all three types)
	sessionID := "integration-test-session"
	sessionLogger := GetSessionLogger(sessionID, LogTypeSession)
	llmLogger := GetSessionLogger(sessionID, LogTypeLLM)
	defer CloseSessionLogger(sessionID)

	// Write some logs to each logger
	sessionLogger.Info("test_session_log")
	sessionLogger.Info("user_input", "input", "test input")
	sessionLogger.Debug("assistant_output_chunk", "chunk", "test output")
	llmLogger.Info("test_llm_log")

	// Flush to ensure writes complete
	FlushLogs()

	// Verify directory structure
	// Main log file
	mainLogPath := filepath.Join(tmpDir, "csw.jsonl")
	_, err = os.Stat(mainLogPath)
	assert.NoError(t, err, "main log file should exist")

	// Session directory
	sessionDir := filepath.Join(tmpDir, "sessions", sessionID)
	info, err := os.Stat(sessionDir)
	require.NoError(t, err, "session directory should exist")
	assert.True(t, info.IsDir(), "session path should be a directory")

	// Session log files
	sessionLogPath := filepath.Join(sessionDir, "logs.json")
	_, err = os.Stat(sessionLogPath)
	assert.NoError(t, err, "logs.json should exist at: %s", sessionLogPath)

	llmLogPath := filepath.Join(sessionDir, "llm.jsonl")
	_, err = os.Stat(llmLogPath)
	assert.NoError(t, err, "llm.jsonl should exist at: %s", llmLogPath)

	t.Logf("Log directory structure verified successfully at: %s", tmpDir)
	t.Logf("Main log: %s", mainLogPath)
	t.Logf("Session dir: %s", sessionDir)
	t.Logf("Session log: %s", sessionLogPath)
	t.Logf("LLM log: %s", llmLogPath)
}
