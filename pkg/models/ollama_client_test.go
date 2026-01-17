package models

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
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

func TestOllamaClient_EmbeddingModel(t *testing.T) {
	tc := getOllamaTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("creates embedding model with model name", func(t *testing.T) {
		embedModel := tc.Client.EmbeddingModel(testOllamaEmbedModelName)

		assert.NotNil(t, embedModel)
	})

	t.Run("generates embeddings for text", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/embed", "POST", `{"model":"nomic-embed-text:latest","embeddings":[[0.1,0.2,0.3,0.4,0.5]]}`)
		}

		embedModel := tc.Client.EmbeddingModel(testOllamaEmbedModelName)

		embedding, err := embedModel.Embed(ctx, "Hello, world!")

		require.NoError(t, err)
		assert.NotNil(t, embedding)
		assert.NotEmpty(t, embedding)
		assert.Greater(t, len(embedding), 0)
	})

	t.Run("generates embeddings for different texts", func(t *testing.T) {
		// Setup mock responses if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/embed", "POST", `{"model":"nomic-embed-text:latest","embeddings":[[0.1,0.2,0.3,0.4,0.5]]}`)
		}

		embedModel := tc.Client.EmbeddingModel(testOllamaEmbedModelName)

		embedding1, err := embedModel.Embed(ctx, "The quick brown fox")
		require.NoError(t, err)
		assert.NotNil(t, embedding1)

		// For mock mode, we need to add another response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/embed", "POST", `{"model":"nomic-embed-text:latest","embeddings":[[0.2,0.3,0.4,0.5,0.6]]}`)
		}

		embedding2, err := embedModel.Embed(ctx, "jumps over the lazy dog")
		require.NoError(t, err)
		assert.NotNil(t, embedding2)

		// Embeddings should have the same dimension
		assert.Equal(t, len(embedding1), len(embedding2))
	})

	t.Run("returns error for empty input", func(t *testing.T) {
		embedModel := tc.Client.EmbeddingModel(testOllamaEmbedModelName)

		embedding, err := embedModel.Embed(ctx, "")

		assert.Error(t, err)
		assert.Nil(t, embedding)
	})
}

func TestOllamaClient_ErrorHandling(t *testing.T) {
	t.Run("handles endpoint not found", func(t *testing.T) {
		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:            "http://beha:11434/nonexistent",
			ConnectTimeout: connectOllamaTimeout,
			RequestTimeout: testOllamaTimeout,
		})
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrEndpointNotFound)
	})

	t.Run("handles endpoint unavailable", func(t *testing.T) {
		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:            "http://nonexistent-host:11434",
			ConnectTimeout: 1 * time.Second,
			RequestTimeout: 2 * time.Second,
		})
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrEndpointUnavailable)
	})
}

func TestOllamaClient_ToolCalling(t *testing.T) {
	tc := getOllamaTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	// Define a weather tool for testing
	weatherTool := tool.ToolInfo{
		Name:        "get_weather",
		Description: "Get the current weather in a given location",
		Schema: tool.ToolSchema{
			Type: tool.SchemaTypeObject,
			Properties: map[string]tool.PropertySchema{
				"location": {
					Type:        tool.SchemaTypeString,
					Description: "The city and state, e.g. San Francisco, CA",
				},
				"unit": {
					Type:        tool.SchemaTypeString,
					Description: "Temperature unit",
					Enum:        []string{"celsius", "fahrenheit"},
				},
			},
			Required:             []string{"location"},
			AdditionalProperties: false,
		},
	}

	t.Run("tool calls are properly passed to LLM", func(t *testing.T) {
		// Setup mock response with tool call if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"get_weather","arguments":{"location":"Paris, France","unit":"celsius"}}}]},"done":true,"done_reason":"stop"}`)
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, &ChatOptions{
			Temperature: 0.1,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "What's the weather like in Paris, France?"),
		}

		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, ChatRoleAssistant, response.Role)

		// The LLM should return a tool call
		toolCalls := response.GetToolCalls()
		assert.NotEmpty(t, toolCalls, "expected LLM to return at least one tool call")

		if len(toolCalls) > 0 {
			call := toolCalls[0]
			assert.NotEmpty(t, call.ID, "tool call ID should not be empty")
			assert.Equal(t, "get_weather", call.Function, "expected tool call to get_weather")
			assert.NotNil(t, call.Arguments, "tool call arguments should not be nil")

			// Verify the location argument is present
			location, ok := call.Arguments.StringOK("location")
			assert.True(t, ok, "expected location argument to be present")
			assert.NotEmpty(t, location, "location should not be empty")
		}
	})

	t.Run("tool responses are properly passed back to LLM", func(t *testing.T) {
		// Setup mock responses if using mock
		if tc.Mock != nil {
			// First response: tool call
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"get_weather","arguments":{"location":"Tokyo"}}}]},"done":true,"done_reason":"stop"}`)
			// Second response: final answer after tool execution
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"The weather in Tokyo is currently 18°C and cloudy."},"done":true,"done_reason":"stop"}`)
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, &ChatOptions{
			Temperature: 0.1,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "What's the weather like in Tokyo?"),
		}

		// First call - get tool call from LLM
		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NoError(t, err)
		require.NotNil(t, response)

		toolCalls := response.GetToolCalls()
		require.NotEmpty(t, toolCalls, "expected LLM to return a tool call")

		// Add the assistant's tool call to conversation
		messages = append(messages, response)

		// Simulate tool execution
		toolResponse := &tool.ToolResponse{
			Call:   toolCalls[0],
			Result: tool.NewToolValue(map[string]interface{}{"temperature": 18, "condition": "cloudy", "unit": "celsius"}),
			Done:   true,
		}

		// Add tool response to conversation
		messages = append(messages, NewToolResponseMessage(toolResponse))

		// Second call - LLM should process tool response
		finalResponse, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})

		require.NoError(t, err)
		assert.NotNil(t, finalResponse)
		assert.Equal(t, ChatRoleAssistant, finalResponse.Role)

		// The response should contain text (not tool calls) about weather
		responseText := finalResponse.GetText()
		assert.NotEmpty(t, responseText, "expected LLM to return text response after tool execution")
		// The LLM should reference the weather data we provided - check for any weather-related terms
		containsWeatherInfo := strings.Contains(strings.ToLower(responseText), "18") ||
			strings.Contains(strings.ToLower(responseText), "cloudy") ||
			strings.Contains(strings.ToLower(responseText), "celsius") ||
			strings.Contains(strings.ToLower(responseText), "tokyo") ||
			strings.Contains(strings.ToLower(responseText), "weather")
		assert.True(t, containsWeatherInfo, "expected response to reference weather information, got: %s", responseText)
	})

	t.Run("tool calls and responses interleaved with text chunks in streaming", func(t *testing.T) {
		// Setup mock responses if using mock
		if tc.Mock != nil {
			// First response: streaming tool call
			tc.Mock.AddStreamingResponse("/api/chat", "POST", true,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"get_weather","arguments":{"location":"London"}}}]},"done":true,"done_reason":"stop"}`,
			)
			// Second response: streaming text response after tool execution
			tc.Mock.AddStreamingResponse("/api/chat", "POST", true,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"The temperature in London is "},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"15°C and it's rainy."},"done":false}`,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
			)
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, &ChatOptions{
			Temperature: 0.1,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "What's the weather in London? Please tell me the temperature."),
		}

		// First streaming call - get tool call from LLM
		iterator := chatModel.ChatStream(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		var collectedToolCalls []*tool.ToolCall

		for fragment := range iterator {
			assert.NotNil(t, fragment)
			fragments = append(fragments, fragment)

			// Collect tool calls as they arrive
			toolCalls := fragment.GetToolCalls()
			collectedToolCalls = append(collectedToolCalls, toolCalls...)
		}

		assert.Greater(t, len(fragments), 0, "expected to receive fragments")

		// OllamaTool calls might be split across fragments or come in one fragment
		// We need to check if we received any tool calls
		assert.NotEmpty(t, collectedToolCalls, "expected to receive tool calls in streaming response")

		if len(collectedToolCalls) > 0 {
			// Reconstruct the complete response
			completeResponse := &ChatMessage{
				Role:  ChatRoleAssistant,
				Parts: []ChatMessagePart{},
			}

			// Merge all fragments
			for _, fragment := range fragments {
				completeResponse.Parts = append(completeResponse.Parts, fragment.Parts...)
			}

			// Add to conversation
			messages = append(messages, completeResponse)

			// Simulate tool execution
			toolResponse := &tool.ToolResponse{
				Call:   collectedToolCalls[0],
				Result: tool.NewToolValue(map[string]interface{}{"temperature": 15, "condition": "rainy", "unit": "celsius"}),
				Done:   true,
			}

			messages = append(messages, NewToolResponseMessage(toolResponse))

			// Second streaming call - LLM processes tool response
			iterator2 := chatModel.ChatStream(ctx, messages, nil, []tool.ToolInfo{weatherTool})
			require.NotNil(t, iterator2)

			var textFragments []string
			for fragment := range iterator2 {
				assert.NotNil(t, fragment)
				text := fragment.GetText()
				if text != "" {
					textFragments = append(textFragments, text)
				}
			}

			// Should have received text fragments
			assert.NotEmpty(t, textFragments, "expected to receive text fragments after tool response")

			// Combine all text
			fullText := ""
			for _, txt := range textFragments {
				fullText += txt
			}

			assert.NotEmpty(t, fullText, "expected non-empty final response")
		}
	})

	t.Run("multiple tool calls in single response", func(t *testing.T) {
		// Define another tool
		timeTool := tool.ToolInfo{
			Name:        "get_time",
			Description: "Get the current time in a given location",
			Schema: tool.ToolSchema{
				Type: tool.SchemaTypeObject,
				Properties: map[string]tool.PropertySchema{
					"location": {
						Type:        tool.SchemaTypeString,
						Description: "The city and state",
					},
				},
				Required:             []string{"location"},
				AdditionalProperties: false,
			},
		}

		// Setup mock response with multiple tool calls if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"get_weather","arguments":{"location":"New York"}}},{"function":{"name":"get_time","arguments":{"location":"New York"}}}]},"done":true,"done_reason":"stop"}`)
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, &ChatOptions{
			Temperature: 0.1,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "What's the weather and current time in New York?"),
		}

		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool, timeTool})

		require.NoError(t, err)
		assert.NotNil(t, response)

		// The LLM might return one or two tool calls
		toolCalls := response.GetToolCalls()
		assert.NotEmpty(t, toolCalls, "expected at least one tool call")

		// Verify each tool call has proper structure
		for _, call := range toolCalls {
			assert.NotEmpty(t, call.ID, "tool call ID should not be empty")
			assert.Contains(t, []string{"get_weather", "get_time"}, call.Function, "unexpected tool function")
			assert.NotNil(t, call.Arguments, "tool call arguments should not be nil")
		}
	})

	t.Run("tool call with error response", func(t *testing.T) {
		// Setup mock responses if using mock
		if tc.Mock != nil {
			// First response: tool call
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"get_weather","arguments":{"location":"Berlin"}}}]},"done":true,"done_reason":"stop"}`)
			// Second response: handling error
			tc.Mock.AddRestResponse("/api/chat", "POST", `{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"I apologize, but I encountered an error: location not found."},"done":true,"done_reason":"stop"}`)
		}

		chatModel := tc.Client.ChatModel(testOllamaModelName, &ChatOptions{
			Temperature: 0.1,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Use the get_weather tool to check the weather in Berlin."),
		}

		// First call - get tool call
		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NoError(t, err)
		require.NotNil(t, response)

		toolCalls := response.GetToolCalls()
		if len(toolCalls) == 0 {
			t.Skip("LLM did not return a tool call, skipping error response test")
		}

		messages = append(messages, response)

		// Simulate tool execution error
		toolResponse := &tool.ToolResponse{
			Call:  toolCalls[0],
			Error: errors.New("location not found"),
			Done:  true,
		}

		messages = append(messages, NewToolResponseMessage(toolResponse))

		// Second call - LLM should handle the error
		finalResponse, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})

		require.NoError(t, err)
		assert.NotNil(t, finalResponse)

		// The response should contain text explaining the error
		responseText := finalResponse.GetText()
		assert.NotEmpty(t, responseText, "expected LLM to return text response after tool error")
	})
}
