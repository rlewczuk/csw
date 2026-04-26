package models

import (
	"bytes"
	"context"
	"log/slog"
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
	testOllamaTimeout        = 30
	connectOllamaTimeout     = 5
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
