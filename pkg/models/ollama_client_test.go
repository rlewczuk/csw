package models

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultOllamaHost        = "http://localhost:11434"
	testOllamaModelName      = "devstral-small-2:latest"
	testOllamaEmbedModelName = "nomic-embed-text:latest"
	testOllamaTimeout        = 30 * time.Second
	connectOllamaTimeout     = 5 * time.Second
)

// getOllamaHost returns the Ollama host URL from config file or default
func getOllamaHost() string {
	if host := testutil.IntegCfgReadFile("ollama.url"); host != "" {
		return host
	}
	return defaultOllamaHost
}

// ollamaTestClient holds either a real or mock client and provides cleanup
type ollamaTestClient struct {
	Client *OllamaClient
	Mock   *testutil.MockHTTPServer
}

// Close cleans up the test client resources
func (tc *ollamaTestClient) Close() {
	if tc.Mock != nil {
		tc.Mock.Close()
	}
}

// getOllamaTestClient returns a client for testing - either real or mock based on integration mode
// For mock mode, it also returns the mock server for adding responses
func getOllamaTestClient(t *testing.T) *ollamaTestClient {
	t.Helper()

	if testutil.IntegTestEnabled("ollama") {
		url := testutil.IntegCfgReadFile("ollama.url")
		if url == "" {
			t.Skip("Skipping test: _integ/ollama.url not configured")
		}

		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:            url,
			ConnectTimeout: connectOllamaTimeout,
			RequestTimeout: testOllamaTimeout,
		})
		require.NoError(t, err)

		return &ollamaTestClient{Client: client}
	}

	// Create mock server
	mock := testutil.NewMockHTTPServer()
	client, err := NewOllamaClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	return &ollamaTestClient{Client: client, Mock: mock}
}

func TestNewOllamaClient(t *testing.T) {
	t.Run("creates client with valid configuration", func(t *testing.T) {
		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:            getOllamaHost(),
			ConnectTimeout: connectOllamaTimeout,
			RequestTimeout: testOllamaTimeout,
		})

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := NewOllamaClient(nil)

		assert.Error(t, err)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		_, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL: "",
		})

		assert.Error(t, err)
	})
}

func TestNewOllamaClientWithHTTPClient(t *testing.T) {
	t.Run("creates client with custom HTTP client", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOllamaClientWithHTTPClient(mock.URL(), mock.Client())

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for empty host", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		_, err := NewOllamaClientWithHTTPClient("", mock.Client())

		assert.Error(t, err)
	})

	t.Run("returns error for nil HTTP client", func(t *testing.T) {
		_, err := NewOllamaClientWithHTTPClient("http://localhost:11434", nil)

		assert.Error(t, err)
	})
}

func TestOllamaClient_ListModels(t *testing.T) {
	tc := getOllamaTestClient(t)
	defer tc.Close()

	// Setup mock response if using mock
	if tc.Mock != nil {
		modelsResponse := `{"models":[{"name":"devstral-small-2:latest","model":"devstral-small-2:latest","modified_at":"2024-01-01T00:00:00Z","size":1000000000,"details":{"family":"llama"}},{"name":"nomic-embed-text:latest","model":"nomic-embed-text:latest","modified_at":"2024-01-01T00:00:00Z","size":500000000,"details":{"family":"nomic"}}]}`
		// Add response for each subtest
		tc.Mock.AddRestResponse("/api/tags", "GET", modelsResponse)
		tc.Mock.AddRestResponse("/api/tags", "GET", modelsResponse)
	}

	t.Run("lists available models", func(t *testing.T) {
		modelList, err := tc.Client.ListModels()

		require.NoError(t, err)
		assert.NotNil(t, modelList)
		assert.NotEmpty(t, modelList, "expected at least one model to be available")

		// Verify model info structure
		for _, model := range modelList {
			assert.NotEmpty(t, model.Name)
			assert.NotEmpty(t, model.Model)
			assert.Greater(t, model.Size, int64(0))
		}
	})

	t.Run("finds test model in list", func(t *testing.T) {
		modelList, err := tc.Client.ListModels()

		require.NoError(t, err)

		found := false
		for _, model := range modelList {
			if model.Name == testOllamaModelName {
				found = true
				assert.NotEmpty(t, model.Family)
				break
			}
		}

		assert.True(t, found, "expected test model %s to be available", testOllamaModelName)
	})
}

func TestOllamaClient_ChatModel(t *testing.T) {
	tc := getOllamaTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("creates chat model with model name and options", func(t *testing.T) {
		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, options)

		assert.NotNil(t, chatModel)
	})

	t.Run("sends chat message and gets response", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"4"},"done":true,"done_reason":"stop"}`)
		}

		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, options)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "What is 2+2? Answer with just the number."}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, ChatRoleAssistant, response.Role)
		assert.NotEmpty(t, response.Parts)
		assert.Greater(t, len(response.GetText()), 0)
	})

	t.Run("handles context with timeout", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello!"},"done":true,"done_reason":"stop"}`)
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Say hello"}},
			},
		}

		response, err := chatModel.Chat(ctxWithTimeout, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("handles system and user messages", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"HELLO!"},"done":true,"done_reason":"stop"}`)
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleSystem,
				Parts: []ChatMessagePart{{Text: "You are a helpful assistant that always responds in uppercase."}},
			},
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "hello"}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, ChatRoleAssistant, response.Role)
	})

	t.Run("returns error for empty messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		response, err := chatModel.Chat(ctx, []*ChatMessage{}, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("returns error for nil messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		response, err := chatModel.Chat(ctx, nil, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("uses default options when none provided to Chat", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello!"},"done":true,"done_reason":"stop"}`)
		}

		defaultOptions := &ChatOptions{
			Temperature: 0.5,
			TopP:        0.8,
			TopK:        30,
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, defaultOptions)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Say hello"}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
	})
}

func TestOllamaClient_ChatTokenUsage(t *testing.T) {
	tc := getOllamaTestClient(t)
	defer tc.Close()

	if tc.Mock == nil {
		t.Skip("Skipping token usage assertions against real provider")
	}

	tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"ok"},"done":true,"prompt_eval_count":11,"eval_count":9}`)

	chatModel := tc.Client.ChatModel(testOllamaModelName, nil)
	resp, err := chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.TokenUsage)
	assert.Equal(t, 11, resp.TokenUsage.InputTokens)
	assert.Equal(t, 0, resp.TokenUsage.InputCachedTokens)
	assert.Equal(t, 11, resp.TokenUsage.InputNonCachedTokens)
	assert.Equal(t, 9, resp.TokenUsage.OutputTokens)
	assert.Equal(t, 20, resp.TokenUsage.TotalTokens)
	assert.Equal(t, 20, resp.ContextLengthTokens)
}

func TestOllamaClient_ChatModelStream(t *testing.T) {
	tc := getOllamaTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("streams chat message and gets fragments", func(t *testing.T) {
		// Setup mock streaming response if using mock
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/api/chat", "POST", true,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"1"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"\n2"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"\n3"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
			)
		}

		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, options)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Count from 1 to 5, one number per line."}},
			},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			assert.Equal(t, ChatRoleAssistant, fragment.Role)
			assert.NotEmpty(t, fragment.Parts)
			fragments = append(fragments, fragment)
		}

		// Should have received multiple fragments
		assert.Greater(t, len(fragments), 0, "expected to receive at least one fragment")
	})

	t.Run("handles context cancellation during streaming", func(t *testing.T) {
		// Setup mock streaming response if using mock (longer response for cancellation test)
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/api/chat", "POST", true,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Once upon a time"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":" there was a cat"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
			)
		}

		ctxWithCancel, cancel := context.WithCancel(ctx)

		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Write a long story about a cat."}},
			},
		}

		iterator := chatModel.ChatStream(ctxWithCancel, messages, nil, nil)
		require.NotNil(t, iterator)

		// Read first fragment
		fragmentReceived := false
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			fragmentReceived = true
			// Cancel the context after first fragment
			cancel()
			// Iterator should stop gracefully
			break
		}

		assert.True(t, fragmentReceived, "expected to receive at least one fragment before cancellation")
	})

	t.Run("handles context with timeout during streaming", func(t *testing.T) {
		// Setup mock streaming response if using mock
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/api/chat", "POST", true,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello!"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
			)
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Say hello"}},
			},
		}

		iterator := chatModel.ChatStream(ctxWithTimeout, messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Greater(t, len(fragments), 0)
	})

	t.Run("handles system and user messages in streaming", func(t *testing.T) {
		// Setup mock streaming response if using mock
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/api/chat", "POST", true,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"OK"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
			)
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleSystem,
				Parts: []ChatMessagePart{{Text: "You are a helpful assistant that responds concisely."}},
			},
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Say 'OK'"}},
			},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Greater(t, len(fragments), 0)
		// All fragments should have assistant role
		for _, fragment := range fragments {
			assert.Equal(t, ChatRoleAssistant, fragment.Role)
		}
	})

	t.Run("returns no fragments for empty messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		iterator := chatModel.ChatStream(ctx, []*ChatMessage{}, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("returns no fragments for nil messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		iterator := chatModel.ChatStream(ctx, nil, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("uses default options when none provided to ChatStream", func(t *testing.T) {
		// Setup mock streaming response if using mock
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/api/chat", "POST", true,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello!"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
			)
		}

		defaultOptions := &ChatOptions{
			Temperature: 0.5,
			TopP:        0.8,
			TopK:        30,
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, defaultOptions)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Say hello"}},
			},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		require.NotNil(t, iterator)

		// Should get at least one fragment
		fragmentReceived := false
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			fragmentReceived = true
			break
		}

		assert.True(t, fragmentReceived, "expected to receive at least one fragment")
	})

	t.Run("iterator can be stopped early", func(t *testing.T) {
		// Setup mock streaming response if using mock
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/api/chat", "POST", true,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello!"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
			)
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, nil)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Say hello"}},
			},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		require.NotNil(t, iterator)

		// Stop reading after first fragment (if any)
		fragmentReceived := false
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			fragmentReceived = true
			break
		}

		// Breaking from range should work gracefully
		assert.True(t, fragmentReceived, "expected to receive at least one fragment")
	})
}

func TestOllamaClient_ErrorHandling(t *testing.T) {
	t.Run("handles endpoint not found", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponseWithStatus("/api/tags", "GET", `{"error":"not found"}`, 404)

		client, err := NewOllamaClientWithHTTPClient(mock.URL(), mock.Client())
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.ErrorIs(t, err, ErrEndpointNotFound)
	})

	t.Run("handles endpoint unavailable", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponseWithStatus("/api/tags", "GET", `{"error":"unavailable"}`, 503)

		client, err := NewOllamaClientWithHTTPClient(mock.URL(), mock.Client())
		require.NoError(t, err)

		_, err = client.ListModels()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEndpointUnavailable)
	})
}

func TestOllamaClient_Logging(t *testing.T) {
	tc := getOllamaTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("logs request and response in Chat method", func(t *testing.T) {
		// Setup mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Logged response"},"done":true,"done_reason":"stop"}`)
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

		chatModel := tc.Client.ChatModel(testOllamaModelName, options)

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
			tc.Mock.AddStreamingResponse("/api/chat", "POST", true,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Chunk1"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"Chunk2"},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
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

		chatModel := tc.Client.ChatModel(testOllamaModelName, options)

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
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"No log"},"done":true,"done_reason":"stop"}`)
		}

		options := &ChatOptions{
			Temperature: 0.7,
			Logger:      nil,
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, options)

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
}

func TestOllamaClient_MaxTokens(t *testing.T) {
	t.Run("Chat method uses MaxTokens as num_predict", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 2048,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Hello"}},
			},
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		// Verify num_predict was set in the request
		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OllamaChatRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		require.NotNil(t, chatReq.Options)
		assert.Equal(t, 2048, chatReq.Options.NumPredict)
	})

	t.Run("ChatStream method uses MaxTokens as num_predict", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 4096,
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
		)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Hello"}},
			},
		}

		iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
		// Consume the iterator
		for range iterator {
		}

		// Verify num_predict was set in the request
		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OllamaChatRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		require.NotNil(t, chatReq.Options)
		assert.Equal(t, 4096, chatReq.Options.NumPredict)
	})

	t.Run("Chat method uses default num_predict when MaxTokens is zero", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 0,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Hello"}},
			},
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		// Verify default num_predict was set in the request
		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OllamaChatRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		require.NotNil(t, chatReq.Options)
		assert.Equal(t, DefaultMaxTokens, chatReq.Options.NumPredict)
	})

	t.Run("Chat method sets num_predict with options when MaxTokens is set", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 1024,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}
		chatModel := client.ChatModel("test-model", options)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Hello"}},
			},
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		// Verify both options and num_predict were set in the request
		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OllamaChatRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		require.NotNil(t, chatReq.Options)
		assert.InDelta(t, 0.7, chatReq.Options.Temperature, 0.01)
		assert.InDelta(t, 0.9, chatReq.Options.TopP, 0.01)
		assert.Equal(t, 40, chatReq.Options.TopK)
		assert.Equal(t, 1024, chatReq.Options.NumPredict)
	})
}

func TestOllamaClient_CustomHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Request-ID":    "req-123",
			"X-Organization":  "my-org",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hello"),
	}

	_, err = chatModel.Chat(context.Background(), messages, nil, nil)
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "custom-value", request.Header.Get("X-Custom-Header"))
	assert.Equal(t, "req-123", request.Header.Get("X-Request-ID"))
	assert.Equal(t, "my-org", request.Header.Get("X-Organization"))
}

func TestOllamaClient_CustomHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		Headers: map[string]string{
			"X-Stream-Header": "stream-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":true,"done_reason":"stop"}`,
	)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hi"),
	}

	iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
	for range iterator {
	}

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "stream-value", request.Header.Get("X-Stream-Header"))
}

func TestOllamaClient_OptionsHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		Headers: map[string]string{
			"X-Config-Header": "config-value",
			"X-Shared-Header": "config-shared",
			"X-Custom-Auth":   "should-be-overridden",
			"Authorization":   "Bearer config-auth",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

	options := &ChatOptions{
		Headers: map[string]string{
			"X-Options-Header": "options-value",
			"X-Shared-Header":  "options-shared",
			"X-Custom-Auth":    "new-value",
			"Authorization":    "Bearer options-auth",
		},
	}

	chatModel := client.ChatModel("test-model", options)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hello"),
	}

	_, err = chatModel.Chat(context.Background(), messages, nil, nil)
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "config-value", request.Header.Get("X-Config-Header"))
	assert.Equal(t, "options-value", request.Header.Get("X-Options-Header"))
	assert.Equal(t, "options-shared", request.Header.Get("X-Shared-Header"), "options headers should override config headers")
	assert.Equal(t, "new-value", request.Header.Get("X-Custom-Auth"), "non-auth headers can be overridden")
	assert.Equal(t, "Bearer config-auth", request.Header.Get("Authorization"), "authorization header should NOT be overridden by options")
}

func TestOllamaClient_OptionsHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		Headers: map[string]string{
			"X-Config-Header": "config-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":true,"done_reason":"stop"}`,
	)

	options := &ChatOptions{
		Headers: map[string]string{
			"X-Options-Header": "options-value",
			"X-Api-Key":        "should-not-override",
		},
	}

	chatModel := client.ChatModel("test-model", options)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hi"),
	}

	iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
	for range iterator {
	}

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "config-value", request.Header.Get("X-Config-Header"))
	assert.Equal(t, "options-value", request.Header.Get("X-Options-Header"))
	assert.Empty(t, request.Header.Get("X-Api-Key"), "api-key header should NOT be set from options")
}

func TestOllamaClient_QueryParams(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		QueryParams: map[string]string{
			"api-version": "2024-01-01",
			"deployment":  "my-deployment",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/tags", "GET", `{"models":[{"name":"test-model","model":"test-model","modified_at":"2024-01-01T00:00:00Z","size":1000000000,"details":{"family":"llama"}}]}`)

	_, err = client.ListModels()
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Contains(t, request.Query, "api-version=2024-01-01")
	assert.Contains(t, request.Query, "deployment=my-deployment")
}

func TestOllamaClient_QueryParamsChat(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		QueryParams: map[string]string{
			"api-version": "2024-01-01",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hello"),
	}

	_, err = chatModel.Chat(context.Background(), messages, nil, nil)
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Contains(t, request.Query, "api-version=2024-01-01")
}

func TestOllamaClient_QueryParamsChatStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		QueryParams: map[string]string{
			"stream-format": "json",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":true,"done_reason":"stop"}`,
	)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hi"),
	}

	iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
	for range iterator {
	}

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Contains(t, request.Query, "stream-format=json")
}

func TestOllamaClient_QueryParamsDoesNotOverrideExisting(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		QueryParams: map[string]string{
			"existing-param": "from-config",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/tags", "GET", `{"models":[]}`)

	_, err = client.ListModels()
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	// The config query param should be present
	assert.Contains(t, request.Query, "existing-param=from-config")
}

func TestOllamaClient_QueryParamsEmptyOrNil(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	// Test with nil QueryParams
	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL:         mock.URL(),
		QueryParams: nil,
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/tags", "GET", `{"models":[]}`)

	_, err = client.ListModels()
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	// Query should be empty when no params are set
	assert.Empty(t, request.Query)
}

func TestOllamaClient_QueryParamsEmptyValues(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		QueryParams: map[string]string{
			"":          "value-with-empty-key",
			"valid-key": "",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/tags", "GET", `{"models":[]}`)

	_, err = client.ListModels()
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	// Empty keys and values should be skipped
	assert.Empty(t, request.Query, "empty keys and values should not be added to query")
}

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
