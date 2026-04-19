package models

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIClient_Logging(t *testing.T) {
	tc := getOpenAITestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("logs request and response in Chat method", func(t *testing.T) {
		// Setup mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-log-1","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"Logged response"},"finish_reason":"stop"}]}`)
		}

		// Create a test logger
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		testLogger := slog.New(handler)

		options := &ChatOptions{
			Temperature: 0.7,
			Logger:      testLogger,
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, options)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Test logging"}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)

		// Check that logs were written
		logOutput := buf.String()
		assert.Contains(t, logOutput, "llm_request")
		assert.Contains(t, logOutput, "llm_response")
		assert.Contains(t, logOutput, "url")
		assert.Contains(t, logOutput, "method")
		assert.Contains(t, logOutput, "headers")
		assert.Contains(t, logOutput, "body")
		assert.Contains(t, logOutput, "status")

		// Verify request body contains expected fields
		assert.Contains(t, logOutput, "model")
		assert.Contains(t, logOutput, "messages")
	})

	t.Run("logs request and each chunk in ChatStream method", func(t *testing.T) {
		// Setup mock streaming response
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-stream-log","object":"chat.completion.chunk","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","content":"Chunk1"}}]}`,
				`data: {"id":"chatcmpl-stream-log","object":"chat.completion.chunk","created":1640000001,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"content":"Chunk2"}}]}`,
				`data: {"id":"chatcmpl-stream-log","object":"chat.completion.chunk","created":1640000002,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{}},"finish_reason":"stop"]}`,
				"data: [DONE]",
			)
		}

		// Create a test logger
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		testLogger := slog.New(handler)

		options := &ChatOptions{
			Temperature: 0.7,
			Logger:      testLogger,
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, options)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Test streaming logging"}},
			},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		require.NotNil(t, iterator)

		// Consume the iterator
		for range iterator {
			// Just consume
		}

		// Check that logs were written
		logOutput := buf.String()
		assert.Contains(t, logOutput, "llm_request")
		assert.Contains(t, logOutput, "llm_response")

		// Should have multiple response logs (one per chunk)
		responseCount := strings.Count(logOutput, `"msg":"llm_response"`)
		assert.GreaterOrEqual(t, responseCount, 1, "expected at least one response log entry")
	})

	t.Run("does not log when logger is nil", func(t *testing.T) {
		// Setup mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-nolog","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"No log"},"finish_reason":"stop"}]}`)
		}

		options := &ChatOptions{
			Temperature: 0.7,
			Logger:      nil,
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, options)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Test no logging"}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
		// No assertions needed - if it doesn't panic, the test passes
	})

	t.Run("obfuscates sensitive headers in logs", func(t *testing.T) {
		// Setup mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-obf","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"Obfuscated"},"finish_reason":"stop"}]}`)
		}

		// Create a test logger
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		testLogger := slog.New(handler)

		options := &ChatOptions{
			Temperature: 0.7,
			Logger:      testLogger,
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, options)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Test obfuscation"}},
			},
		}

		_, err := chatModel.Chat(ctx, messages, nil, nil)
		require.NoError(t, err)

		// Check that logs don't contain the full API key
		logOutput := buf.String()
		// The Authorization header should be obfuscated
		assert.NotContains(t, logOutput, "Bearer test")
	})

	t.Run("logs error response when API returns error", func(t *testing.T) {
		// Setup mock error response
		if tc.Mock != nil {
			tc.Mock.AddRestResponseWithStatus("/chat/completions", "POST", `{"error":{"message":"Invalid request","type":"invalid_request_error"}}`, 400)
		}

		// Create a test logger
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		testLogger := slog.New(handler)

		options := &ChatOptions{
			Temperature: 0.7,
			Logger:      testLogger,
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, options)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Test error logging"}},
			},
		}

		_, err := chatModel.Chat(ctx, messages, nil, nil)
		require.Error(t, err)

		// Check that error response was logged
		logOutput := buf.String()
		assert.Contains(t, logOutput, "llm_response_error")
		assert.Contains(t, logOutput, "Invalid request")
	})
}

func TestOpenAIClient_RawLLMCallback_ObfuscatesRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Authorization", "Bearer response-secret-token")
		_, err := w.Write([]byte(`{"id":"chatcmpl-raw","object":"chat.completion","created":1640000000,"model":"test-model","api_key":"response-secret-key","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewOpenAIClient(&conf.ModelProviderConfig{
		URL:    server.URL,
		APIKey: "request-secret-api-key",
	})
	require.NoError(t, err)

	rawLines := make([]string, 0)
	client.SetRawLLMCallback(func(line string) {
		rawLines = append(rawLines, line)
	})

	chatModel := client.ChatModel("test-model", nil)
	_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hello")}, nil, nil)
	require.NoError(t, err)

	joined := strings.Join(rawLines, "\n")
	assert.Contains(t, joined, ">>> REQUEST POST ")
	assert.Contains(t, joined, ">>> HEADER Authorization: Bear...-key")
	assert.NotContains(t, joined, "request-secret-api-key")
	assert.Contains(t, joined, "<<< RESPONSE 200")
	assert.Contains(t, joined, "<<< HEADER Authorization: Bear...oken")
	assert.NotContains(t, joined, "response-secret-token")
	assert.Contains(t, joined, "api_key")
	assert.NotContains(t, joined, "response-secret-key")
}

func TestOpenAIClient_RawLLMCallback_LogsStreamingChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, err := w.Write([]byte("data: {\"id\":\"chatcmpl-stream-A\",\"object\":\"chat.completion.chunk\",\"created\":1640000000,\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"A\"}}]}\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("data: {\"id\":\"chatcmpl-stream-B\",\"object\":\"chat.completion.chunk\",\"created\":1640000001,\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"B\"}}]}\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("data: [DONE]\n"))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewOpenAIClient(&conf.ModelProviderConfig{
		URL:    server.URL,
		APIKey: "stream-secret-key",
	})
	require.NoError(t, err)

	rawLines := make([]string, 0)
	client.SetRawLLMCallback(func(line string) {
		rawLines = append(rawLines, line)
	})

	chatModel := client.ChatModel("test-model", nil)
	for range chatModel.ChatStream(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hello")}, nil, nil) {
	}

	joined := strings.Join(rawLines, "\n")
	assert.Contains(t, joined, "<<< CHUNK data: {\"id\":\"chatcmpl-stream-A\"")
	assert.Contains(t, joined, "<<< CHUNK data: {\"id\":\"chatcmpl-stream-B\"")
	assert.Contains(t, joined, "<<< CHUNK data: [DONE]")
}
