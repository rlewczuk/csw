package logging

import (
	"strings"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestLoggerBasicLogging(t *testing.T) {
	sessionID := "test-session-123"
	sessionLog, _, _ := NewTestLogger(t, sessionID)

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

func TestTestLoggerUserInput(t *testing.T) {
	sessionID := "test-session-123"
	sessionLog, chatLog, _ := NewTestLogger(t, sessionID)

	userInput := "Hello, world!"
	LogUserInput(sessionLog, chatLog, userInput)

	// Check chat logs
	chatBuf := GetTestChatBuffer(sessionID)
	require.NotNil(t, chatBuf)
	assert.Contains(t, chatBuf.String(), "user_message")
	assert.Contains(t, chatBuf.String(), userInput)
}

func TestTestLoggerAssistantOutput(t *testing.T) {
	sessionID := "test-session-123"
	sessionLog, chatLog, _ := NewTestLogger(t, sessionID)

	output := "Assistant response"
	LogAssistantOutput(sessionLog, chatLog, output)

	// Check chat logs
	chatBuf := GetTestChatBuffer(sessionID)
	require.NotNil(t, chatBuf)
	assert.Contains(t, chatBuf.String(), "assistant_chunk")
	assert.Contains(t, chatBuf.String(), output)
}

func TestTestLoggerToolCall(t *testing.T) {
	sessionID := "test-session-123"
	sessionLog, chatLog, _ := NewTestLogger(t, sessionID)

	toolCall := &tool.ToolCall{
		ID:       "call-123",
		Function: "test_tool",
		Arguments: tool.NewToolValue(map[string]any{
			"arg1": "value1",
		}),
	}
	LogToolCall(sessionLog, chatLog, toolCall)

	// Check chat logs
	chatBuf := GetTestChatBuffer(sessionID)
	require.NotNil(t, chatBuf)
	assert.Contains(t, chatBuf.String(), "tool_call")
	assert.Contains(t, chatBuf.String(), "call-123")
	assert.Contains(t, chatBuf.String(), "test_tool")
}

func TestTestLoggerToolResult(t *testing.T) {
	sessionID := "test-session-123"
	sessionLog, chatLog, _ := NewTestLogger(t, sessionID)

	toolResponse := &tool.ToolResponse{
		Call: &tool.ToolCall{
			ID:       "call-123",
			Function: "test_tool",
		},
		Result: tool.NewToolValue("success"),
		Error:  nil,
		Done:   true,
	}
	LogToolResult(sessionLog, chatLog, toolResponse)

	// Check chat logs
	chatBuf := GetTestChatBuffer(sessionID)
	require.NotNil(t, chatBuf)
	assert.Contains(t, chatBuf.String(), "tool_response")
	assert.Contains(t, chatBuf.String(), "call-123")
}

func TestTestLoggerLLMRequest(t *testing.T) {
	sessionID := "test-session-123"
	_, _, llmLog := NewTestLogger(t, sessionID)

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
	_, _, llmLog := NewTestLogger(t, sessionID)

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

func TestTestLoggerPermissionQuery(t *testing.T) {
	sessionID := "test-session-123"
	sessionLog, _, _ := NewTestLogger(t, sessionID)

	query := &tool.ToolPermissionsQuery{
		Id: "query-123",
		Tool: &tool.ToolCall{
			Function: "test_tool",
		},
		Title:   "Permission Required",
		Details: "This tool needs permission",
	}
	LogPermissionQuery(sessionLog, query)

	// Verify it was logged
	sessionBuf := GetTestSessionBuffer(sessionID)
	require.NotNil(t, sessionBuf)
	assert.Contains(t, sessionBuf.String(), "permission_query")
	assert.Contains(t, sessionBuf.String(), "query-123")
}

func TestTestLoggerChatMessages(t *testing.T) {
	sessionID := "test-session-123"
	_, chatLog, _ := NewTestLogger(t, sessionID)

	messages := []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleUser, "test message 1"),
		models.NewTextMessage(models.ChatRoleAssistant, "test message 2"),
	}
	LogChatMessages(chatLog, messages)

	// Check chat logs
	chatBuf := GetTestChatBuffer(sessionID)
	require.NotNil(t, chatBuf)
	content := chatBuf.String()
	assert.Contains(t, content, "chat_message")
	assert.Contains(t, content, "test message 1")
	assert.Contains(t, content, "test message 2")
	// Count occurrences of "chat_message"
	count := strings.Count(content, "chat_message")
	assert.Equal(t, 2, count, "should have 2 chat messages")
}

func TestFlushTestLogger(t *testing.T) {
	sessionID := "test-session-123"
	sessionLog, _, _ := NewTestLogger(t, sessionID)

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
