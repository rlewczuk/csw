package models

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultOpenAITestURL     = "http://localhost:11434/v1"
	testOpenAIModelName      = "devstral-small-2:latest"
	testOpenAIEmbedModelName = "nomic-embed-text:latest"
	testOpenAITimeout        = 30 * time.Second
	connectOpenAITimeout     = 5 * time.Second
)

// openaiTestClient holds either a real or mock client and provides cleanup
type openaiTestClient struct {
	Client *OpenAIClient
	Mock   *testutil.MockHTTPServer
}

// Close cleans up the test client resources
func (tc *openaiTestClient) Close() {
	if tc.Mock != nil {
		tc.Mock.Close()
	}
}

// getOpenAITestClient returns a client for testing - either real or mock based on integration mode
// For mock mode, it also returns the mock server for adding responses
func getOpenAITestClient(t *testing.T) *openaiTestClient {
	t.Helper()

	if testutil.IntegTestEnabled("openai") {
		url := testutil.IntegCfgReadFile("openai.url")
		if url == "" {
			t.Skip("Skipping test: _integ/openai.url not configured")
		}
		apiKey := testutil.IntegCfgReadFile("openai.key")

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:            url,
			APIKey:         apiKey,
			ConnectTimeout: connectOpenAITimeout,
			RequestTimeout: testOpenAITimeout,
		})
		require.NoError(t, err)

		return &openaiTestClient{Client: client}
	}

	// Create mock server
	mock := testutil.NewMockHTTPServer()
	client, err := NewOpenAIClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	return &openaiTestClient{Client: client, Mock: mock}
}

func TestNewOpenAIClient(t *testing.T) {
	t.Run("creates client with valid configuration", func(t *testing.T) {
		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:            defaultOpenAITestURL,
			ConnectTimeout: connectOpenAITimeout,
			RequestTimeout: testOpenAITimeout,
		})

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := NewOpenAIClient(nil)

		assert.Error(t, err)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		_, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: "",
		})

		assert.Error(t, err)
	})
}

func TestOpenAIClient_ListModels(t *testing.T) {
	tc := getOpenAITestClient(t)
	defer tc.Close()

	// Setup mock response if using mock
	if tc.Mock != nil {
		modelsResponse := `{"data":[{"id":"devstral-small-2:latest","object":"model","created":1640000000,"owned_by":"openai"},{"id":"nomic-embed-text:latest","object":"model","created":1640000000,"owned_by":"openai"}]}`
		// Add response for each subtest
		tc.Mock.AddRestResponse("/models", "GET", modelsResponse)
		tc.Mock.AddRestResponse("/models", "GET", modelsResponse)
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
		}
	})

	t.Run("finds test model in list", func(t *testing.T) {
		modelList, err := tc.Client.ListModels()

		require.NoError(t, err)

		found := false
		for _, model := range modelList {
			if model.Name == testOpenAIModelName {
				found = true
				break
			}
		}

		assert.True(t, found, "expected test model %s to be available", testOpenAIModelName)
	})
}

func TestOpenAIClient_ChatModel(t *testing.T) {
	tc := getOpenAITestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("creates chat model with model name and options", func(t *testing.T) {
		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, options)

		assert.NotNil(t, chatModel)
	})

	t.Run("sends chat message and gets response", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-123","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"4"},"finish_reason":"stop"}]}`)
		}

		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, options)

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

	t.Run("sends tool response as role tool", func(t *testing.T) {
		if tc.Mock == nil {
			t.Skip("Skipping test: mock server required")
		}
		tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-125","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
		initialRequests := tc.Mock.GetRequests()

		callID := "vfsMove:1"
		toolCall := &tool.ToolCall{
			ID:       callID,
			Function: "vfsMove",
			Arguments: tool.NewToolValue(map[string]any{
				"path":        "/tmp/source.txt",
				"destination": "/tmp/dest.txt",
			}),
		}
		toolResp := &tool.ToolResponse{
			Call: toolCall,
			Done: true,
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "do it"),
			NewToolCallMessage(toolCall),
			NewToolResponseMessage(toolResp),
		}

		_, err := chatModel.Chat(ctx, messages, nil, nil)
		require.NoError(t, err)

		reqs := tc.Mock.GetRequests()
		require.Greater(t, len(reqs), len(initialRequests))
		lastReq := reqs[len(reqs)-1]
		require.Equal(t, "/chat/completions", lastReq.Path)
		require.Equal(t, "POST", lastReq.Method)

		var chatReq OpenaiChatCompletionRequest
		require.NoError(t, json.Unmarshal(lastReq.Body, &chatReq))
		require.NotEmpty(t, chatReq.Messages)

		last := chatReq.Messages[len(chatReq.Messages)-1]
		assert.Equal(t, "tool", last.Role)
		assert.Equal(t, callID, last.ToolCallID)
		assert.Equal(t, "null", last.Content)
	})

	t.Run("handles context with timeout", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-124","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}]}`)
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)

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
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-125","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"HELLO"},"finish_reason":"stop"}]}`)
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)

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
		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)

		response, err := chatModel.Chat(ctx, []*ChatMessage{}, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("returns error for nil messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)

		response, err := chatModel.Chat(ctx, nil, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("uses default options when none provided to Chat", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-126","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}]}`)
		}

		defaultOptions := &ChatOptions{
			Temperature: 0.5,
			TopP:        0.8,
			TopK:        30,
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, defaultOptions)

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

func TestOpenAIClient_ChatTokenUsage(t *testing.T) {
	tc := getOpenAITestClient(t)
	defer tc.Close()

	if tc.Mock == nil {
		t.Skip("Skipping token usage assertions against real provider")
	}

	tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-usage","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":7,"total_tokens":19,"prompt_tokens_details":{"cached_tokens":5}}}`)

	chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)
	resp, err := chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.TokenUsage)
	assert.Equal(t, 12, resp.TokenUsage.InputTokens)
	assert.Equal(t, 5, resp.TokenUsage.InputCachedTokens)
	assert.Equal(t, 7, resp.TokenUsage.InputNonCachedTokens)
	assert.Equal(t, 7, resp.TokenUsage.OutputTokens)
	assert.Equal(t, 19, resp.TokenUsage.TotalTokens)
	assert.Equal(t, 19, resp.ContextLengthTokens)
}

func TestOpenAIClient_ChatModelStream(t *testing.T) {
	tc := getOpenAITestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("streams chat message and gets fragments", func(t *testing.T) {
		// Setup mock streaming response if using mock
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-stream-1","object":"chat.completion.chunk","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","content":"1"}}]}`,
				`data: {"id":"chatcmpl-stream-1","object":"chat.completion.chunk","created":1640000001,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"content":"\n2\n3"}}]}`,
				`data: {"id":"chatcmpl-stream-1","object":"chat.completion.chunk","created":1640000002,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
				"data: [DONE]",
			)
		}

		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, options)

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
		// Setup mock streaming response if using mock
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-stream-2","object":"chat.completion.chunk","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","content":"Once upon a time"}}]}`,
				"data: [DONE]",
			)
		}

		ctxWithCancel, cancel := context.WithCancel(ctx)

		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)

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
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-stream-3","object":"chat.completion.chunk","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello!"}}]}`,
				`data: {"id":"chatcmpl-stream-3","object":"chat.completion.chunk","created":1640000001,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
				"data: [DONE]",
			)
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)

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

	t.Run("returns no fragments for empty messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)

		iterator := chatModel.ChatStream(ctx, []*ChatMessage{}, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("returns no fragments for nil messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)

		iterator := chatModel.ChatStream(ctx, nil, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("iterator can be stopped early", func(t *testing.T) {
		// Setup mock streaming response if using mock
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-stream-4","object":"chat.completion.chunk","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello!"}}]}`,
				"data: [DONE]",
			)
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)

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


func TestOpenAIClient_ErrorHandling(t *testing.T) {
	t.Run("handles endpoint not found", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponseWithStatus("/models", "GET", `{"error":{"message":"not found"}}`, 404)

		client, err := NewOpenAIClientWithHTTPClient(mock.URL(), mock.Client())
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.ErrorIs(t, err, ErrEndpointNotFound)
	})

	t.Run("handles endpoint unavailable", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponseWithStatus("/models", "GET", `{"error":{"message":"unavailable"}}`, 503)

		client, err := NewOpenAIClientWithHTTPClient(mock.URL(), mock.Client())
		require.NoError(t, err)

		_, err = client.ListModels()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEndpointUnavailable)
	})
}

func TestOpenAIClient_MaxTokens(t *testing.T) {
	t.Run("Chat method uses MaxTokens as max_tokens", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 2048,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-maxtokens","object":"chat.completion","created":1640000000,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}]}`)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Hello"}},
			},
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		// Verify max_tokens was set in the request
		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		assert.Equal(t, 2048, chatReq.MaxTokens)
	})

	t.Run("ChatStream method uses MaxTokens as max_tokens", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 4096,
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/chat/completions", "POST", true,
			`data: {"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1640000000,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`,
			`data: {"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1640000001,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			"data: [DONE]",
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

		// Verify max_tokens was set in the request
		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		assert.Equal(t, 4096, chatReq.MaxTokens)
	})

	t.Run("Chat method uses default max_tokens when MaxTokens is zero", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 0,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-notokens","object":"chat.completion","created":1640000000,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}]}`)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Hello"}},
			},
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		// Verify default max_tokens was set in the request
		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		assert.Equal(t, DefaultMaxTokens, chatReq.MaxTokens)
	})
}

func TestOpenAIClient_CustomHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOpenAIClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "test-key",
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Request-ID":    "req-123",
			"X-Organization":  "my-org",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-custom","object":"chat.completion","created":1640000000,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}]}`)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hello"),
	}

	_, err = chatModel.Chat(context.Background(), messages, nil, nil)
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "Bearer test-key", request.Header.Get("Authorization"))
	assert.Equal(t, "custom-value", request.Header.Get("X-Custom-Header"))
	assert.Equal(t, "req-123", request.Header.Get("X-Request-ID"))
	assert.Equal(t, "my-org", request.Header.Get("X-Organization"))
}

func TestOpenAIClient_CustomHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOpenAIClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "test-key",
		Headers: map[string]string{
			"X-Stream-Header": "stream-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/chat/completions", "POST", true,
		`data: {"id":"chatcmpl-stream-custom","object":"chat.completion.chunk","created":1640000000,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}`,
		`data: {"id":"chatcmpl-stream-custom","object":"chat.completion.chunk","created":1640000001,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"data: [DONE]",
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

	assert.Equal(t, "Bearer test-key", request.Header.Get("Authorization"))
	assert.Equal(t, "stream-value", request.Header.Get("X-Stream-Header"))
}

func TestOpenAIClient_ListModelsIncludesConfiguredQueryParams(t *testing.T) {
	t.Run("list models includes configured query params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "0.1.0", r.URL.Query().Get("client_version"))

			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"data":[{"id":"test-model","object":"model","created":1640000000,"owned_by":"openai"}]}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:    server.URL,
			APIKey: "test-key",
			QueryParams: map[string]string{
				"client_version": "0.1.0",
			},
		})
		require.NoError(t, err)

		models, err := client.ListModels()
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "test-model", models[0].Name)
	})
}

func TestOpenAIClient_OptionsHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOpenAIClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "config-api-key",
		Headers: map[string]string{
			"X-Config-Header": "config-value",
			"X-Shared-Header": "config-shared",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-opts","object":"chat.completion","created":1640000000,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}]}`)

	options := &ChatOptions{
		Headers: map[string]string{
			"X-Options-Header": "options-value",
			"X-Shared-Header":  "options-shared",
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
	assert.Equal(t, "Bearer config-api-key", request.Header.Get("Authorization"), "authorization header should NOT be overridden by options")
}

func TestOpenAIClient_OptionsHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOpenAIClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "config-api-key",
		Headers: map[string]string{
			"X-Config-Header": "config-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/chat/completions", "POST", true,
		`data: {"id":"chatcmpl-opts-stream","object":"chat.completion.chunk","created":1640000000,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}`,
		`data: {"id":"chatcmpl-opts-stream","object":"chat.completion.chunk","created":1640000001,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"data: [DONE]",
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

func TestOpenAIClient_RateLimitError(t *testing.T) {
	t.Run("returns rate limit error with retry-after header", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		headers := http.Header{}
		headers.Set("Retry-After", "60")
		mock.AddRestResponseWithStatusAndHeaders("/chat/completions", "POST", `{"error":{"message":"Rate limit exceeded","type":"rate_limit_error"}}`, http.StatusTooManyRequests, headers)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		assert.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 60, rateLimitErr.RetryAfterSeconds)
		assert.Contains(t, rateLimitErr.Error(), "Rate limit exceeded")
	})

	t.Run("returns usage-limit retry after parsed from reset at timestamp", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		resetAt := time.Now().Add(95 * time.Second).Format("2006-01-02 15:04:05")
		body := `{"error":{"code":"1308","message":"Usage limit reached for 5 hour. Your limit will reset at ` + resetAt + `"}}`
		mock.AddRestResponseWithStatus("/chat/completions", "POST", body, http.StatusTooManyRequests)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.True(t, errors.As(err, &rateLimitErr))
		assert.GreaterOrEqual(t, rateLimitErr.RetryAfterSeconds, 90)
		assert.LessOrEqual(t, rateLimitErr.RetryAfterSeconds, 100)
		assert.Contains(t, rateLimitErr.Message, "Usage limit reached")
	})

	t.Run("keeps larger retry-after when usage-limit parsed value is smaller", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		resetAt := time.Now().Add(30 * time.Second).Format("2006-01-02 15:04:05")
		body := `{"error":{"code":"1308","message":"Usage limit reached. Your limit will reset at ` + resetAt + `"}}`
		headers := http.Header{}
		headers.Set("Retry-After", "120")
		mock.AddRestResponseWithStatusAndHeaders("/chat/completions", "POST", body, http.StatusTooManyRequests, headers)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 120, rateLimitErr.RetryAfterSeconds)
	})

	t.Run("fallbacks to plain body when error payload is not json", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		mock.AddRestResponseWithStatus("/chat/completions", "POST", "rate limited", http.StatusTooManyRequests)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, "rate limited", rateLimitErr.Message)
		assert.Equal(t, 0, rateLimitErr.RetryAfterSeconds)
		assert.True(t, errors.Is(err, ErrRateExceeded))
	})
}
