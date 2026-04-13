package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGlobalLogger(t *testing.T) {
	logger := GetGlobalLogger()
	require.NotNil(t, logger, "GetGlobalLogger() should return non-nil logger")

	// Test logging
	logger.Info("test_message", "key", "value")
}

func TestGetSessionLogger(t *testing.T) {
	sessionID := "test-session-123"

	sessionLogger := GetSessionLogger(sessionID, LogTypeSession)
	require.NotNil(t, sessionLogger, "GetSessionLogger() should return non-nil logger")
	defer CloseSessionLogger(sessionID)

	llmLogger := GetSessionLogger(sessionID, LogTypeLLM)
	require.NotNil(t, llmLogger, "GetSessionLogger() for llm should return non-nil logger")

	// Test that subsequent calls return the same logger
	sessionLogger2 := GetSessionLogger(sessionID, LogTypeSession)
	assert.Equal(t, sessionLogger, sessionLogger2, "GetSessionLogger() should return cached logger")
}

func TestSetLogsDirectory(t *testing.T) {
	tmpDir := filepath.Join("../../tmp", "test_logs", t.Name())
	defer os.RemoveAll(tmpDir)

	err := SetLogsDirectory(tmpDir, false)
	require.NoError(t, err, "SetLogsDirectory() should not return error")

	logger := GetGlobalLogger()
	require.NotNil(t, logger, "GetGlobalLogger() should return non-nil logger")

	// Test logging
	logger.Info("test_message", "key", "value")

	// Check that log file was created
	logPath := filepath.Join(tmpDir, "csw.jsonl")
	_, err = os.Stat(logPath)
	require.NoError(t, err, "log file should exist")

	// Flush to ensure writes complete
	FlushLogs()

	// Read log file and verify content
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "should be able to read log file")
	assert.Contains(t, string(content), "test_message")
	assert.Contains(t, string(content), "key")
	assert.Contains(t, string(content), "value")
}

func TestSessionLoggerUserInput(t *testing.T) {
	tmpDir := filepath.Join("../../tmp", "test_logs", t.Name())
	defer os.RemoveAll(tmpDir)

	err := SetLogsDirectory(tmpDir, false)
	require.NoError(t, err)

	sessionID := "test-session-123"
	sessionLog := GetSessionLogger(sessionID, LogTypeSession)
	defer CloseSessionLogger(sessionID)

	// Log user input
	userInput := "Hello, world!"
	LogUserInput(sessionLog, userInput)

	// Flush to ensure writes complete
	FlushLogs()

	// Read session log
	sessionLogPath := filepath.Join(tmpDir, "sessions", sessionID, "logs.json")
	content, err := os.ReadFile(sessionLogPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "user_messag")
	assert.Contains(t, string(content), userInput)
}

func TestSessionLoggerToolCall(t *testing.T) {
	tmpDir := filepath.Join("../../tmp", "test_logs", t.Name())
	defer os.RemoveAll(tmpDir)

	err := SetLogsDirectory(tmpDir, false)
	require.NoError(t, err)

	sessionID := "test-session-123"
	sessionLog := GetSessionLogger(sessionID, LogTypeSession)
	defer CloseSessionLogger(sessionID)

	// Log tool call
	toolCall := &tool.ToolCall{
		ID:       "call-123",
		Function: "test_tool",
		Arguments: tool.NewToolValue(map[string]any{
			"arg1": "value1",
		}),
	}
	LogToolCall(sessionLog, toolCall)

	// Flush to ensure writes complete
	FlushLogs()

	// Read chat log
	chatLogPath := filepath.Join(tmpDir, "sessions", sessionID, "logs.json")
	chatContent, err := os.ReadFile(chatLogPath)
	require.NoError(t, err)
	assert.Contains(t, string(chatContent), "tool_call")
	assert.Contains(t, string(chatContent), "call-123")
	assert.Contains(t, string(chatContent), "test_tool")
}

func TestSessionLoggerLLMRequest(t *testing.T) {
	tmpDir := filepath.Join("../../tmp", "test_logs", t.Name())
	defer os.RemoveAll(tmpDir)

	err := SetLogsDirectory(tmpDir, false)
	require.NoError(t, err)

	sessionID := "test-session-123"
	llmLog := GetSessionLogger(sessionID, LogTypeLLM)
	defer CloseSessionLogger(sessionID)

	// Log LLM request
	requestBody := map[string]any{
		"model":  "test-model",
		"prompt": "test prompt",
	}
	LogLLMRequest(llmLog, "test-provider", "test-model", requestBody)

	// Flush to ensure writes complete
	FlushLogs()

	// Read LLM log
	llmLogPath := filepath.Join(tmpDir, "sessions", sessionID, "llm.jsonl")
	content, err := os.ReadFile(llmLogPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "llm_request")
	assert.Contains(t, string(content), "test-provider")
	assert.Contains(t, string(content), "test-model")
	assert.Contains(t, string(content), "test prompt")
}

func TestSessionLoggerAppendMode(t *testing.T) {
	tmpDir := filepath.Join("../../tmp", "test_logs", t.Name())
	defer os.RemoveAll(tmpDir)

	err := SetLogsDirectory(tmpDir, false)
	require.NoError(t, err)

	sessionID := "test-session-123"

	// First session logger
	sessionLog1 := GetSessionLogger(sessionID, LogTypeSession)
	sessionLog1.Info("message_1")
	CloseSessionLogger(sessionID)

	// Second session logger (should append, not overwrite)
	sessionLog2 := GetSessionLogger(sessionID, LogTypeSession)
	sessionLog2.Info("message_2")
	CloseSessionLogger(sessionID)

	// Flush to ensure writes complete
	FlushLogs()

	// Give a moment for writes to complete
	time.Sleep(10 * time.Millisecond)

	// Read session log
	sessionLogPath := filepath.Join(tmpDir, "sessions", sessionID, "logs.json")
	content, err := os.ReadFile(sessionLogPath)
	require.NoError(t, err)

	// Both messages should be present
	assert.Contains(t, string(content), "message_1")
	assert.Contains(t, string(content), "message_2")

	// Verify it's valid JSONL (one JSON object per line)
	lines := 0
	for _, line := range []byte(content) {
		if line == '\n' {
			lines++
		}
	}
	assert.GreaterOrEqual(t, lines, 2, "should have at least 2 log lines")
}

func TestLogFormatIsValidJSON(t *testing.T) {
	tmpDir := filepath.Join("../../tmp", "test_logs", t.Name())
	defer os.RemoveAll(tmpDir)

	err := SetLogsDirectory(tmpDir, false)
	require.NoError(t, err)

	sessionID := "test-session-123"
	sessionLog := GetSessionLogger(sessionID, LogTypeSession)
	defer CloseSessionLogger(sessionID)

	// Log various types of messages
	sessionLog.Info("test_message", "key1", "value1", "key2", 123)
	LogUserInput(sessionLog, "test input")
	LogAssistantOutput(sessionLog, "test output")

	// Flush to ensure writes complete
	FlushLogs()

	// Read session log
	sessionLogPath := filepath.Join(tmpDir, "sessions", sessionID, "logs.json")
	content, err := os.ReadFile(sessionLogPath)
	require.NoError(t, err)

	// Split by lines and verify each line is valid JSON
	lines := []byte{}
	for _, b := range content {
		if b == '\n' {
			if len(lines) > 0 {
				var obj map[string]any
				err := json.Unmarshal(lines, &obj)
				assert.NoError(t, err, "each log line should be valid JSON")
				lines = []byte{}
			}
		} else {
			lines = append(lines, b)
		}
	}
}

func TestInMemoryLogging(t *testing.T) {
	// Don't call SetLogsDirectory - this should default to in-memory logging
	sessionID := "test-session-inmem"

	sessionLog := GetSessionLogger(sessionID, LogTypeSession)
	defer CloseSessionLogger(sessionID)

	// Log some messages
	sessionLog.Info("test_message_1")

	// Verify no files were created
	_, err := os.Stat(filepath.Join(".cswdata/logs/sessions", sessionID))
	assert.True(t, os.IsNotExist(err), "session directory should not exist for in-memory logging")
}

func TestSyncWrites(t *testing.T) {
	tmpDir := filepath.Join("../../tmp", "test_logs", t.Name())
	defer os.RemoveAll(tmpDir)

	// Enable sync writes
	err := SetLogsDirectory(tmpDir, true)
	require.NoError(t, err)

	logger := GetGlobalLogger()
	logger.Info("test_message_sync")

	// With sync=true, the file should be immediately readable
	logPath := filepath.Join(tmpDir, "csw.jsonl")
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test_message_sync")
}
