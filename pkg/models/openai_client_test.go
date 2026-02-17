package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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

func TestOpenAIClient_EmbeddingModel(t *testing.T) {
	tc := getOpenAITestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("creates embedding model with model name", func(t *testing.T) {
		embedModel := tc.Client.EmbeddingModel(testOpenAIEmbedModelName)

		assert.NotNil(t, embedModel)
	})

	t.Run("generates embeddings for text", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/embeddings", "POST", `{"object":"list","data":[{"object":"embedding","embedding":[0.1,0.2,0.3,0.4,0.5],"index":0}],"model":"nomic-embed-text:latest"}`)
		}

		embedModel := tc.Client.EmbeddingModel(testOpenAIEmbedModelName)

		embedding, err := embedModel.Embed(ctx, "Hello, world!")

		require.NoError(t, err)
		assert.NotNil(t, embedding)
		assert.NotEmpty(t, embedding)
		assert.Greater(t, len(embedding), 0)
	})

	t.Run("generates embeddings for different texts", func(t *testing.T) {
		// Setup mock responses if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/embeddings", "POST", `{"object":"list","data":[{"object":"embedding","embedding":[0.1,0.2,0.3,0.4,0.5],"index":0}],"model":"nomic-embed-text:latest"}`)
			tc.Mock.AddRestResponse("/embeddings", "POST", `{"object":"list","data":[{"object":"embedding","embedding":[0.2,0.3,0.4,0.5,0.6],"index":0}],"model":"nomic-embed-text:latest"}`)
		}

		embedModel := tc.Client.EmbeddingModel(testOpenAIEmbedModelName)

		embedding1, err := embedModel.Embed(ctx, "The quick brown fox")
		require.NoError(t, err)
		assert.NotNil(t, embedding1)

		embedding2, err := embedModel.Embed(ctx, "jumps over the lazy dog")
		require.NoError(t, err)
		assert.NotNil(t, embedding2)

		// Embeddings should have the same dimension
		assert.Equal(t, len(embedding1), len(embedding2))
	})

	t.Run("returns error for empty input", func(t *testing.T) {
		embedModel := tc.Client.EmbeddingModel(testOpenAIEmbedModelName)

		embedding, err := embedModel.Embed(ctx, "")

		assert.Error(t, err)
		assert.Nil(t, embedding)
	})
}

func TestOpenAIClient_ToolCalling(t *testing.T) {
	tc := getOpenAITestClient(t)
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
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-tool-1","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_abc123","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"Paris, France\",\"unit\":\"celsius\"}"}}]},"finish_reason":"tool_calls"}]}`)
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, &ChatOptions{
			Temperature: 0.0,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleSystem, "You are a helpful assistant. When asked about weather, you MUST use the get_weather tool. Do not answer weather questions without using the tool."),
			NewTextMessage(ChatRoleUser, "Use the get_weather tool to check the weather in Paris, France."),
		}

		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, ChatRoleAssistant, response.Role)

		// The LLM should return a tool call
		toolCalls := response.GetToolCalls()
		if len(toolCalls) == 0 {
			t.Skip("LLM did not return a tool call - this can happen due to model non-determinism")
		}

		call := toolCalls[0]
		assert.NotEmpty(t, call.ID, "tool call ID should not be empty")
		assert.Equal(t, "get_weather", call.Function, "expected tool call to get_weather")
		assert.NotNil(t, call.Arguments, "tool call arguments should not be nil")

		// Verify the location argument is present
		location, ok := call.Arguments.StringOK("location")
		assert.True(t, ok, "expected location argument to be present")
		assert.NotEmpty(t, location, "location should not be empty")
	})

	t.Run("tool responses are properly passed back to LLM", func(t *testing.T) {
		// Setup mock responses if using mock
		if tc.Mock != nil {
			// First response: tool call
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-tool-2","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"call_def456","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"Tokyo\"}"}}]},"finish_reason":"tool_calls"}]}`)
			// Second response: final answer after tool execution
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-tool-2b","object":"chat.completion","created":1640000001,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"The weather in Tokyo is currently 18°C and cloudy."},"finish_reason":"stop"}]}`)
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, &ChatOptions{
			Temperature: 0.0,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleSystem, "You are a helpful assistant. When asked about weather, you MUST use the get_weather tool. Do not answer weather questions without using the tool."),
			NewTextMessage(ChatRoleUser, "Use the get_weather tool to check the weather in Tokyo."),
		}

		// First call - get tool call from LLM
		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NoError(t, err)
		require.NotNil(t, response)

		toolCalls := response.GetToolCalls()
		if len(toolCalls) == 0 {
			t.Skip("LLM did not return a tool call - this can happen due to model non-determinism")
		}

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

		// The response should contain text (not tool calls)
		responseText := finalResponse.GetText()
		assert.NotEmpty(t, responseText, "expected LLM to return text response after tool execution")
		// Check that response mentions the temperature in some form (could be "18", "18°", "18 degrees", etc.)
		assert.True(t, strings.Contains(responseText, "18") || strings.Contains(responseText, "cloudy"),
			"expected response to mention the temperature or condition, got: %s", responseText)
	})

	t.Run("tool calls and responses interleaved with text chunks in streaming", func(t *testing.T) {
		// Setup mock responses if using mock
		if tc.Mock != nil {
			// First response: streaming tool call
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-tool-stream-1","object":"chat.completion.chunk","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_ghi789","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"London\"}"}}]}}]}`,
				`data: {"id":"chatcmpl-tool-stream-1","object":"chat.completion.chunk","created":1640000001,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
				"data: [DONE]",
			)
			// Second response: streaming text response after tool execution
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-tool-stream-1b","object":"chat.completion.chunk","created":1640000002,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","content":"The temperature in London is 15°C and it's rainy."}}]}`,
				`data: {"id":"chatcmpl-tool-stream-1b","object":"chat.completion.chunk","created":1640000003,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
				"data: [DONE]",
			)
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, &ChatOptions{
			Temperature: 0.0,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleSystem, "You are a helpful assistant. When asked about weather, you MUST use the get_weather tool. Do not answer weather questions without using the tool."),
			NewTextMessage(ChatRoleUser, "Use the get_weather tool to check the weather in London."),
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

		// OpenaiTool calls might be split across fragments or come in one fragment
		// Skip if no tool calls were returned (model non-determinism)
		if len(collectedToolCalls) == 0 {
			t.Skip("LLM did not return a tool call in streaming response - this can happen due to model non-determinism")
		}

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
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-tool-3","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"call_jkl012","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"New York\"}"}},{"id":"call_mno345","type":"function","function":{"name":"get_time","arguments":"{\"location\":\"New York\"}"}}]},"finish_reason":"tool_calls"}]}`)
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, &ChatOptions{
			Temperature: 0.0,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleSystem, "You are a helpful assistant. When asked about weather, you MUST use the get_weather tool. When asked about time, you MUST use the get_time tool. Do not answer these questions without using the tools."),
			NewTextMessage(ChatRoleUser, "Use the get_weather and get_time tools to check the weather and current time in New York."),
		}

		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool, timeTool})

		require.NoError(t, err)
		assert.NotNil(t, response)

		// The LLM might return one or two tool calls
		toolCalls := response.GetToolCalls()
		if len(toolCalls) == 0 {
			t.Skip("LLM did not return any tool calls - this can happen due to model non-determinism")
		}

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
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-tool-4","object":"chat.completion","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"call_pqr678","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"Berlin\"}"}}]},"finish_reason":"tool_calls"}]}`)
			// Second response: handling error
			tc.Mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-tool-4b","object":"chat.completion","created":1640000001,"model":"devstral-small-2:latest","choices":[{"index":0,"message":{"role":"assistant","content":"I apologize, but I encountered an error: location not found."},"finish_reason":"stop"}]}`)
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, &ChatOptions{
			Temperature: 0.0,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleSystem, "You are a helpful assistant. When asked about weather, you MUST use the get_weather tool. Do not answer weather questions without using the tool."),
			NewTextMessage(ChatRoleUser, "Use the get_weather tool to check the weather in Berlin."),
		}

		// First call - get tool call
		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NoError(t, err)
		require.NotNil(t, response)

		toolCalls := response.GetToolCalls()
		if len(toolCalls) == 0 {
			t.Skip("LLM did not return a tool call - this can happen due to model non-determinism")
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

func TestOpenAIClient_ToolChoice(t *testing.T) {
	t.Run("chat includes tool_choice when tools provided", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-toolchoice","object":"chat.completion","created":1640000000,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "use tool"),
		}
		tools := []tool.ToolInfo{
			{
				Name:        "ping",
				Description: "Ping tool",
				Schema:      tool.NewToolSchema(),
			},
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, tools)
		require.NoError(t, err)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)

		assert.Equal(t, "auto", chatReq.ToolChoice)
	})

	t.Run("chat stream includes tool_choice when tools provided", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/chat/completions", "POST", true,
			`data: {"id":"chatcmpl-toolchoice-stream","object":"chat.completion.chunk","created":1640000000,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"}}]}`,
			`data: {"id":"chatcmpl-toolchoice-stream","object":"chat.completion.chunk","created":1640000001,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			"data: [DONE]",
		)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "use tool"),
		}
		tools := []tool.ToolInfo{
			{
				Name:        "ping",
				Description: "Ping tool",
				Schema:      tool.NewToolSchema(),
			},
		}

		iterator := chatModel.ChatStream(context.Background(), messages, nil, tools)
		for range iterator {
		}

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)

		assert.Equal(t, "auto", chatReq.ToolChoice)
	})
}

func TestOpenAIClient_ErrorHandling(t *testing.T) {
	t.Run("handles endpoint not found", func(t *testing.T) {
		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:            "http://localhost:11434/v1/nonexistent",
			ConnectTimeout: connectOpenAITimeout,
			RequestTimeout: testOpenAITimeout,
		})
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrEndpointNotFound)
	})

	t.Run("handles endpoint unavailable", func(t *testing.T) {
		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:            "http://nonexistent-host:11434/v1",
			ConnectTimeout: 1 * time.Second,
			RequestTimeout: 2 * time.Second,
		})
		require.NoError(t, err)

		_, err = client.ListModels()
		require.Error(t, err)
		// Network errors are now wrapped in NetworkError for retry support
		var networkErr *NetworkError
		if assert.True(t, errors.As(err, &networkErr), "Should be a NetworkError, got: %v", err) {
			assert.True(t, networkErr.IsRetryable, "Network error should be retryable")
		}
	})
}

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

func TestOpenAIClient_ReasoningContent(t *testing.T) {
	t.Run("streams reasoning content in same message as text", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/chat/completions", "POST", true,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"The user"}}]}`,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"reasoning_content":" wants"}}]}`,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"reasoning_content":" help."}}]}`,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"content":"Sure!"}}]}`,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"content":" I can help."}}]}`,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"finish_reason":"stop","delta":{"role":"assistant","content":""}}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`,
			"data: [DONE]",
		)

		chatModel := client.ChatModel("glm-5", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Help me"),
		}

		iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage

		for fragment := range iterator {
			require.NotNil(t, fragment)
			fragments = append(fragments, fragment)
		}

		assert.NotEmpty(t, fragments, "expected fragments")

		// All fragments should be assistant role
		for _, f := range fragments {
			assert.Equal(t, ChatRoleAssistant, f.Role, "all fragments should have assistant role")
		}

		// Check that reasoning content is accumulated in the first text fragment
		var reasoningContent string
		var textContent string
		for _, f := range fragments {
			for _, part := range f.Parts {
				if part.ReasoningContent != "" {
					reasoningContent += part.ReasoningContent
				}
				if part.Text != "" {
					textContent += part.Text
				}
			}
		}

		assert.Equal(t, "The user wants help.", reasoningContent, "reasoning content should be accumulated")
		assert.Equal(t, "Sure! I can help.", textContent, "text content should be accumulated")
	})

	t.Run("streams reasoning content with tool calls in same message", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/chat/completions", "POST", true,
			`data: {"id":"chatcmpl-reasoning-2","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"I need"}}]}`,
			`data: {"id":"chatcmpl-reasoning-2","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"reasoning_content":" to read"}}]}`,
			`data: {"id":"chatcmpl-reasoning-2","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"reasoning_content":" the file."}}]}`,
			`data: {"id":"chatcmpl-reasoning-2","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_123","index":0,"type":"function","function":{"name":"read","arguments":"{\"filePath\":\"/test/file.go\"}"}}]}}]}`,
			`data: {"id":"chatcmpl-reasoning-2","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"finish_reason":"tool_calls","delta":{"role":"assistant","content":""}}]}`,
			"data: [DONE]",
		)

		chatModel := client.ChatModel("glm-5", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Read the file"),
		}

		iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage

		for fragment := range iterator {
			require.NotNil(t, fragment)
			fragments = append(fragments, fragment)
		}

		assert.NotEmpty(t, fragments, "expected fragments")

		// All fragments should be assistant role
		for _, f := range fragments {
			assert.Equal(t, ChatRoleAssistant, f.Role, "all fragments should have assistant role")
		}

		// Collect reasoning content and tool calls from fragments
		var reasoningContent string
		var toolCalls []*tool.ToolCall
		for _, f := range fragments {
			for _, part := range f.Parts {
				if part.ReasoningContent != "" {
					reasoningContent += part.ReasoningContent
				}
				if part.ToolCall != nil {
					toolCalls = append(toolCalls, part.ToolCall)
				}
			}
		}

		assert.Equal(t, "I need to read the file.", reasoningContent, "reasoning content should be accumulated")
		assert.Len(t, toolCalls, 1, "expected one tool call")
		assert.Equal(t, "call_123", toolCalls[0].ID)
		assert.Equal(t, "read", toolCalls[0].Function)
	})

	t.Run("includes stream_options in request", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/chat/completions", "POST", true,
			`data: {"id":"chatcmpl-opts","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"content":"Hi"}}]}`,
			`data: {"id":"chatcmpl-opts","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			"data: [DONE]",
		)

		chatModel := client.ChatModel("glm-5", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Hi"),
		}

		iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
		for range iterator {
		}

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)

		assert.True(t, chatReq.Stream, "stream should be true")
		require.NotNil(t, chatReq.StreamOptions, "stream_options should not be nil")
		assert.True(t, chatReq.StreamOptions.IncludeUsage, "include_usage should be true")
	})

	t.Run("non-streaming response includes reasoning content", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-reasoning-3","object":"chat.completion","created":1771161940,"model":"glm-5","choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"I should help.","content":"Sure!"},"finish_reason":"stop"}]}`)

		chatModel := client.ChatModel("glm-5", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Help me"),
		}

		response, err := chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, response)

		assert.Equal(t, ChatRoleAssistant, response.Role)
		require.NotEmpty(t, response.Parts, "expected at least one part")

		var reasoningContent string
		var textContent string
		for _, part := range response.Parts {
			if part.ReasoningContent != "" {
				reasoningContent += part.ReasoningContent
			}
			if part.Text != "" {
				textContent += part.Text
			}
		}

		assert.Equal(t, "I should help.", reasoningContent, "reasoning content should be extracted")
		assert.Equal(t, "Sure!", textContent, "text content should be extracted")
	})

	t.Run("sends reasoning content back to LLM in subsequent request", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-reasoning-4","object":"chat.completion","created":1771161940,"model":"glm-5","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`)

		chatModel := client.ChatModel("glm-5", nil)

		previousAssistantMsg := &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{ReasoningContent: "I thought about this.", Text: "Hello"},
			},
		}
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Hi"),
			previousAssistantMsg,
			NewTextMessage(ChatRoleUser, "Continue"),
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)

		require.Len(t, chatReq.Messages, 3)
		assistantMsg := chatReq.Messages[1]
		assert.Equal(t, "assistant", assistantMsg.Role)
		assert.Equal(t, "Hello", assistantMsg.Content)
		assert.Equal(t, "I thought about this.", assistantMsg.ReasoningContent, "reasoning content should be sent back to LLM")
	})

	t.Run("sends reasoning content with tool calls back to LLM", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-reasoning-5","object":"chat.completion","created":1771161940,"model":"glm-5","choices":[{"index":0,"message":{"role":"assistant","content":"Done"},"finish_reason":"stop"}]}`)

		chatModel := client.ChatModel("glm-5", nil)

		previousAssistantMsg := &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{ReasoningContent: "I need to call a tool."},
				{ToolCall: &tool.ToolCall{
					ID:        "call_abc",
					Function:  "test_func",
					Arguments: tool.NewToolValue(map[string]any{"arg": "value"}),
				}},
			},
		}
		toolResultMsg := NewToolResponseMessage(&tool.ToolResponse{
			Call:   &tool.ToolCall{ID: "call_abc"},
			Result: tool.NewToolValue("result"),
			Done:   true,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Do something"),
			previousAssistantMsg,
			toolResultMsg,
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)

		require.Len(t, chatReq.Messages, 3)
		assistantMsg := chatReq.Messages[1]
		assert.Equal(t, "assistant", assistantMsg.Role)
		assert.Equal(t, "I need to call a tool.", assistantMsg.ReasoningContent, "reasoning content should be sent back with tool calls")
		require.Len(t, assistantMsg.ToolCalls, 1)
		assert.Equal(t, "call_abc", assistantMsg.ToolCalls[0].ID)
	})
}
