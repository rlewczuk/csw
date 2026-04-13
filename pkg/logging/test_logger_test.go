package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestLoggerBasicLogging(t *testing.T) {
	sessionID := "test-session-123"
	sessionLog, _ := NewTestLogger(t, sessionID)

	// Test basic logging
	sessionLog.Info("test_message", "key", "value")
	sessionLog.Debug("debug_message")
	sessionLog.Warn("warn_message")
	sessionLog.Error("error_message")

	// Verify logs were written to buffer
	buf := GetTestSessionBuffer(sessionID)
	require.NotNil(t, buf)
	assert.Contains(t, buf.String(), "test_message")
	assert.Contains(t, buf.String(), "debug_message")
}

func TestTestLoggerLLMRequest(t *testing.T) {
	sessionID := "test-session-123"
	_, llmLog := NewTestLogger(t, sessionID)

	requestBody := map[string]any{
		"model":  "test-model",
		"prompt": "test prompt",
	}
	LogLLMRequest(llmLog, "test-provider", "test-model", requestBody)

	// Check LLM logs
	llmBuf := GetTestLLMBuffer(sessionID)
	require.NotNil(t, llmBuf)
	assert.Contains(t, llmBuf.String(), "llm_request")
	assert.Contains(t, llmBuf.String(), "test-provider")
	assert.Contains(t, llmBuf.String(), "test-model")
}

func TestTestLoggerLLMResponse(t *testing.T) {
	sessionID := "test-session-123"
	_, llmLog := NewTestLogger(t, sessionID)

	responseBody := map[string]any{
		"result": "test result",
	}
	LogLLMResponse(llmLog, "test-provider", "test-model", responseBody)

	// Check LLM logs
	llmBuf := GetTestLLMBuffer(sessionID)
	require.NotNil(t, llmBuf)
	assert.Contains(t, llmBuf.String(), "llm_response")
	assert.Contains(t, llmBuf.String(), "test-provider")
	assert.Contains(t, llmBuf.String(), "test-model")
}

func TestFlushTestLogger(t *testing.T) {
	sessionID := "test-session-123"
	sessionLog, _ := NewTestLogger(t, sessionID)

	sessionLog.Info("test message")

	// Flush should not panic
	FlushTestLogger(sessionID)
}

func TestTestLoggerFactory(t *testing.T) {
	factory := NewTestLoggerFactory(t)
	logger, err := factory("test-session", "")
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Can use the logger
	logger.Info("test message")
}
