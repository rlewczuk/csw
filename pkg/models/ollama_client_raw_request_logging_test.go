package models

import (
	"context"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaClient_RawRequestLogging(t *testing.T) {
	tc := getOllamaTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("logs raw request and response in Chat method", func(t *testing.T) {
		// Setup mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Raw logged response"},"done":true,"done_reason":"stop"}`)
		}

		var loggedLines []string
		tc.Client.SetRawLLMCallback(func(line string) {
			loggedLines = append(loggedLines, line)
		})

		options := &ChatOptions{
			Temperature: 0.7,
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, options)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Test raw logging"}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)

		// Verify raw logs were captured
		assert.NotEmpty(t, loggedLines, "expected raw log lines to be captured")

		// Check for request log
		var hasRequestLog bool
		for _, line := range loggedLines {
			if strings.Contains(line, ">>> REQUEST") {
				hasRequestLog = true
				break
			}
		}
		assert.True(t, hasRequestLog, "expected >>> REQUEST log line")

		// Check for response log
		var hasResponseLog bool
		for _, line := range loggedLines {
			if strings.Contains(line, "<<< RESPONSE") {
				hasResponseLog = true
				break
			}
		}
		assert.True(t, hasResponseLog, "expected <<< RESPONSE log line")
	})

	t.Run("logs raw request with headers and body", func(t *testing.T) {
		// Setup mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Test"},"done":true,"done_reason":"stop"}`)
		}

		var loggedLines []string
		tc.Client.SetRawLLMCallback(func(line string) {
			loggedLines = append(loggedLines, line)
		})

		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Hello"),
		}

		_, err := chatModel.Chat(ctx, messages, nil, nil)
		require.NoError(t, err)

		// Check for header logs
		var hasHeaderLog bool
		for _, line := range loggedLines {
			if strings.Contains(line, ">>> HEADER") {
				hasHeaderLog = true
				break
			}
		}
		assert.True(t, hasHeaderLog, "expected >>> HEADER log line")

		// Check for body log
		var hasBodyLog bool
		for _, line := range loggedLines {
			if strings.Contains(line, ">>> BODY") {
				hasBodyLog = true
				break
			}
		}
		assert.True(t, hasBodyLog, "expected >>> BODY log line")
	})

	t.Run("logs raw response with headers and body", func(t *testing.T) {
		// Setup mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Response body"},"done":true,"done_reason":"stop"}`)
		}

		var loggedLines []string
		tc.Client.SetRawLLMCallback(func(line string) {
			loggedLines = append(loggedLines, line)
		})

		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Test"),
		}

		_, err := chatModel.Chat(ctx, messages, nil, nil)
		require.NoError(t, err)

		// Check for response header logs
		var hasResponseHeaderLog bool
		for _, line := range loggedLines {
			if strings.Contains(line, "<<< HEADER") {
				hasResponseHeaderLog = true
				break
			}
		}
		assert.True(t, hasResponseHeaderLog, "expected <<< HEADER log line")

		// Check for response body log
		var hasResponseBodyLog bool
		for _, line := range loggedLines {
			if strings.Contains(line, "<<< BODY") {
				hasResponseBodyLog = true
				break
			}
		}
		assert.True(t, hasResponseBodyLog, "expected <<< BODY log line")
	})

	t.Run("obfuscates sensitive headers in raw logs", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
			Headers: map[string]string{
				"Authorization": "Bearer secret-token-12345",
				"X-Api-Key":     "api-key-secret",
				"Content-Type":  "application/json",
			},
		})
		require.NoError(t, err)

		mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Test"},"done":true,"done_reason":"stop"}`)

		var loggedLines []string
		client.SetRawLLMCallback(func(line string) {
			loggedLines = append(loggedLines, line)
		})

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Test"),
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		// Check that sensitive headers are obfuscated
		for _, line := range loggedLines {
			if strings.Contains(line, ">>> HEADER") {
				// Authorization should be obfuscated
				if strings.Contains(line, "Authorization") {
					assert.NotContains(t, line, "secret-token-12345", "authorization token should be obfuscated")
					assert.Contains(t, line, "...", "authorization should show obfuscation pattern")
				}
				// X-Api-Key should be obfuscated
				if strings.Contains(line, "X-Api-Key") {
					assert.NotContains(t, line, "api-key-secret", "api key should be obfuscated")
				}
				// Content-Type should NOT be obfuscated
				if strings.Contains(line, "Content-Type") {
					assert.Contains(t, line, "application/json", "content-type should not be obfuscated")
				}
			}
		}
	})

	t.Run("does not log when callback is nil", func(t *testing.T) {
		// Setup mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"No raw log"},"done":true,"done_reason":"stop"}`)
		}

		// Set callback to nil
		tc.Client.SetRawLLMCallback(nil)

		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Test no callback"),
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
		// No assertions needed - if it doesn't panic, the test passes
	})

	t.Run("callback can be set and reset", func(t *testing.T) {
		var firstCallbackCalled bool
		var secondCallbackCalled bool

		// Set first callback
		tc.Client.SetRawLLMCallback(func(line string) {
			firstCallbackCalled = true
		})

		// Setup mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"First callback"},"done":true,"done_reason":"stop"}`)
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Test first callback"),
		}

		_, err := chatModel.Chat(ctx, messages, nil, nil)
		require.NoError(t, err)

		assert.True(t, firstCallbackCalled, "first callback should have been called")

		// Reset to second callback
		tc.Client.SetRawLLMCallback(func(line string) {
			secondCallbackCalled = true
		})

		// Setup another mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Second callback"},"done":true,"done_reason":"stop"}`)
		}

		_, err = chatModel.Chat(ctx, messages, nil, nil)
		require.NoError(t, err)

		assert.True(t, secondCallbackCalled, "second callback should have been called")
	})
}
