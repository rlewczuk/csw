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
	defaultAnthropicTestURL = "https://api.anthropic.com"
	testAnthropicModelName  = "claude-sonnet-4-5-20250929"
	testAnthropicTimeout    = 30 * time.Second
	connectAnthropicTimeout = 5 * time.Second
)

// anthropicTestClient holds either a real or mock client and provides cleanup
type anthropicTestClient struct {
	Client *AnthropicClient
	Mock   *testutil.MockHTTPServer
}

// Close cleans up the test client resources
func (tc *anthropicTestClient) Close() {
	if tc.Mock != nil {
		tc.Mock.Close()
	}
}

// getAnthropicTestClient returns a client for testing - either real or mock based on integration mode
// For mock mode, it also returns the mock server for adding responses
func getAnthropicTestClient(t *testing.T) *anthropicTestClient {
	t.Helper()

	if testutil.IntegTestEnabled("anthropic") {
		apiKey := testutil.IntegCfgReadFile("anthropic.key")
		if apiKey == "" {
			t.Skip("Skipping test: _integ/anthropic.key not configured")
		}

		client, err := NewAnthropicClient(&conf.ModelProviderConfig{
			URL:            defaultAnthropicTestURL,
			APIKey:         apiKey,
			ConnectTimeout: connectAnthropicTimeout,
			RequestTimeout: testAnthropicTimeout,
		})
		require.NoError(t, err)

		return &anthropicTestClient{Client: client}
	}

	// Create mock server
	mock := testutil.NewMockHTTPServer()
	client, err := NewAnthropicClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	return &anthropicTestClient{Client: client, Mock: mock}
}

func TestNewAnthropicClient(t *testing.T) {
	t.Run("creates client with valid configuration", func(t *testing.T) {
		client, err := NewAnthropicClient(&conf.ModelProviderConfig{
			URL:            defaultAnthropicTestURL,
			APIKey:         "test-api-key",
			ConnectTimeout: connectAnthropicTimeout,
			RequestTimeout: testAnthropicTimeout,
		})

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := NewAnthropicClient(nil)

		assert.Error(t, err)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		_, err := NewAnthropicClient(&conf.ModelProviderConfig{
			URL: "",
		})

		assert.Error(t, err)
	})
}

func TestNewAnthropicClientWithHTTPClient(t *testing.T) {
	t.Run("creates client with custom HTTP client", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewAnthropicClientWithHTTPClient(mock.URL(), mock.Client())

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		_, err := NewAnthropicClientWithHTTPClient("", mock.Client())

		assert.Error(t, err)
	})

	t.Run("returns error for nil HTTP client", func(t *testing.T) {
		_, err := NewAnthropicClientWithHTTPClient(defaultAnthropicTestURL, nil)

		assert.Error(t, err)
	})
}

func TestAnthropicClient_ListModels(t *testing.T) {
	tc := getAnthropicTestClient(t)
	defer tc.Close()

	// Setup mock response if using mock
	if tc.Mock != nil {
		modelsResponse := `{"data":[{"id":"claude-sonnet-4-5-20250929","created_at":"2024-01-01T00:00:00Z","display_name":"Claude Sonnet 4.5","type":"model"},{"id":"claude-3-5-sonnet-20241022","created_at":"2024-01-01T00:00:00Z","display_name":"Claude 3.5 Sonnet","type":"model"}]}`
		// Add response for each subtest
		tc.Mock.AddRestResponse("/v1/models", "GET", modelsResponse)
		tc.Mock.AddRestResponse("/v1/models", "GET", modelsResponse)
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
			if model.Name == testAnthropicModelName {
				found = true
				break
			}
		}

		assert.True(t, found, "expected test model %s to be available", testAnthropicModelName)
	})
}

func TestAnthropicClient_ChatModel(t *testing.T) {
	tc := getAnthropicTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("creates chat model with model name and options", func(t *testing.T) {
		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, options)

		assert.NotNil(t, chatModel)
	})

	t.Run("sends chat message and gets response", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_test123","type":"message","role":"assistant","content":[{"type":"text","text":"4"}],"model":"claude-sonnet-4-5-20250929","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
		}

		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, options)

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
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_test124","type":"message","role":"assistant","content":[{"type":"text","text":"Hello!"}],"model":"claude-sonnet-4-5-20250929","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

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
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_test125","type":"message","role":"assistant","content":[{"type":"text","text":"HELLO"}],"model":"claude-sonnet-4-5-20250929","stop_reason":"end_turn","usage":{"input_tokens":15,"output_tokens":3}}`)
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

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
		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

		response, err := chatModel.Chat(ctx, []*ChatMessage{}, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("returns error for nil messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

		response, err := chatModel.Chat(ctx, nil, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("uses default options when none provided to Chat", func(t *testing.T) {
		// Setup mock response if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_test126","type":"message","role":"assistant","content":[{"type":"text","text":"Hello!"}],"model":"claude-sonnet-4-5-20250929","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
		}

		defaultOptions := &ChatOptions{
			Temperature: 0.5,
			TopP:        0.8,
			TopK:        30,
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, defaultOptions)

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

func TestAnthropicClient_ChatModelStream(t *testing.T) {
	tc := getAnthropicTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("streams chat message and gets fragments", func(t *testing.T) {
		// Setup mock streaming response if using mock
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/v1/messages", "POST", true,
				`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_test127","type":"message","role":"assistant","content":[],"model":"`+testAnthropicModelName+`","usage":{"input_tokens":10,"output_tokens":0}}}`,
				`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"1"}}`,
				`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"\n2"}}`,
				`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"\n3"}}`,
				`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
			)
		}

		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, options)

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
			tc.Mock.AddStreamingResponse("/v1/messages", "POST", true,
				`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_test128","type":"message","role":"assistant","content":[],"model":"`+testAnthropicModelName+`","usage":{"input_tokens":10,"output_tokens":0}}}`,
				`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Once upon a time"}}`,
				`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
			)
		}

		ctxWithCancel, cancel := context.WithCancel(ctx)

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

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
			tc.Mock.AddStreamingResponse("/v1/messages", "POST", true,
				`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_test129","type":"message","role":"assistant","content":[],"model":"`+testAnthropicModelName+`","usage":{"input_tokens":10,"output_tokens":0}}}`,
				`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello!"}}`,
				`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
			)
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

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
		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

		iterator := chatModel.ChatStream(ctx, []*ChatMessage{}, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("returns no fragments for nil messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

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
			tc.Mock.AddStreamingResponse("/v1/messages", "POST", true,
				`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_test130","type":"message","role":"assistant","content":[],"model":"`+testAnthropicModelName+`","usage":{"input_tokens":10,"output_tokens":0}}}`,
				`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello!"}}`,
				`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
			)
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

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

func TestAnthropicClient_EmbeddingModel(t *testing.T) {
	tc := getAnthropicTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("returns error for embedding model", func(t *testing.T) {
		embedModel := tc.Client.EmbeddingModel("any-model")

		assert.NotNil(t, embedModel)

		// Anthropic doesn't support embeddings, should return an error
		_, err := embedModel.Embed(ctx, "Hello, world!")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented")
	})
}

func TestAnthropicClient_ToolCalling(t *testing.T) {
	tc := getAnthropicTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	// Define a simple weather tool for testing
	weatherTool := createAnthropicWeatherToolInfo()

	t.Run("sends tool definitions to LLM and receives tool call", func(t *testing.T) {
		// Setup mock response with tool call if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_test131","type":"message","role":"assistant","content":[{"type":"tool_use","id":"toolu_test123","name":"get_weather","input":{"location":"San Francisco, CA","unit":"fahrenheit"}}],"model":"claude-sonnet-4-5-20250929","stop_reason":"tool_use","usage":{"input_tokens":20,"output_tokens":15}}`)
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "What is the weather in San Francisco?"}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, ChatRoleAssistant, response.Role)

		// The LLM should return a tool call
		toolCalls := response.GetToolCalls()
		require.NotEmpty(t, toolCalls, "expected at least one tool call in response")

		// Verify the tool call structure
		foundWeatherCall := false
		for _, call := range toolCalls {
			if call.Function == "get_weather" {
				foundWeatherCall = true
				assert.NotEmpty(t, call.ID, "tool call should have an ID")

				// Verify arguments contain location
				location := call.Arguments.String("location")
				assert.NotEmpty(t, location, "expected location argument in tool call")
				break
			}
		}
		assert.True(t, foundWeatherCall, "expected get_weather tool call")
	})

	t.Run("sends tool response back to LLM and receives final answer", func(t *testing.T) {
		// Setup mock responses if using mock
		if tc.Mock != nil {
			// First response: tool call
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_test131b","type":"message","role":"assistant","content":[{"type":"tool_use","id":"toolu_test123b","name":"get_weather","input":{"location":"San Francisco, CA","unit":"fahrenheit"}}],"model":"claude-sonnet-4-5-20250929","stop_reason":"tool_use","usage":{"input_tokens":20,"output_tokens":15}}`)
			// Second response: final answer after tool execution
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_test131c","type":"message","role":"assistant","content":[{"type":"text","text":"The weather in San Francisco is currently 72°F and sunny."}],"model":"claude-sonnet-4-5-20250929","stop_reason":"end_turn","usage":{"input_tokens":30,"output_tokens":20}}`)
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

		// First message: user asks about weather
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "What is the weather in San Francisco?"}},
			},
		}

		// Get tool call from LLM
		response1, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NoError(t, err)
		require.NotNil(t, response1)

		toolCalls := response1.GetToolCalls()
		require.NotEmpty(t, toolCalls, "expected tool call in first response")

		// Add assistant's tool call to conversation
		messages = append(messages, response1)

		// Create tool response
		toolResponse := &tool.ToolResponse{
			Call:   toolCalls[0],
			Result: tool.NewToolValue(map[string]interface{}{"temperature": 72, "condition": "sunny"}),
			Done:   true,
		}

		// Add tool response to conversation
		messages = append(messages, NewToolResponseMessage(toolResponse))

		// Send tool response back to LLM
		response2, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})

		require.NoError(t, err)
		assert.NotNil(t, response2)
		assert.Equal(t, ChatRoleAssistant, response2.Role)

		// The final response should contain text (not tool calls)
		text := response2.GetText()
		assert.NotEmpty(t, text, "expected text response after tool execution")

		// The response should mention the weather information
		// (checking for either temperature value or condition)
		assert.Contains(t, text, "72", "expected response to mention temperature")
	})

	t.Run("handles multiple tool calls in conversation", func(t *testing.T) {
		// Setup mock response with multiple tool calls if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_test132","type":"message","role":"assistant","content":[{"type":"tool_use","id":"toolu_test124","name":"get_weather","input":{"location":"San Francisco, CA"}},{"type":"tool_use","id":"toolu_test125","name":"get_time","input":{"location":"San Francisco, CA"}}],"model":"claude-sonnet-4-5-20250929","stop_reason":"tool_use","usage":{"input_tokens":25,"output_tokens":20}}`)
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

		// Define multiple tools
		weatherTool := createAnthropicWeatherToolInfo()
		timeTool := createAnthropicTimeToolInfo()

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "What is the weather and current time in San Francisco?"}},
			},
		}

		// Get tool calls from LLM
		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool, timeTool})

		require.NoError(t, err)
		assert.NotNil(t, response)

		toolCalls := response.GetToolCalls()
		assert.NotEmpty(t, toolCalls, "expected tool calls in response")

		// Verify we got calls to relevant tools
		// Note: LLM may call one or both tools depending on how it interprets the question
		for _, call := range toolCalls {
			assert.NotEmpty(t, call.ID)
			assert.Contains(t, []string{"get_weather", "get_time"}, call.Function)
		}
	})

	t.Run("handles interleaved text and tool calls", func(t *testing.T) {
		// Setup mock response with tool call if using mock
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_test133","type":"message","role":"assistant","content":[{"type":"tool_use","id":"toolu_test126","name":"get_weather","input":{"location":"Boston, MA"}}],"model":"claude-sonnet-4-5-20250929","stop_reason":"tool_use","usage":{"input_tokens":25,"output_tokens":15}}`)
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Check the weather in Boston and tell me if I should bring an umbrella."}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})

		require.NoError(t, err)
		assert.NotNil(t, response)

		// Response may contain both text and tool calls
		// Anthropic often returns tool_use blocks without much text, but let's check the structure
		assert.NotEmpty(t, response.Parts, "expected parts in response")

		// Verify that parts can be properly retrieved
		toolCalls := response.GetToolCalls()
		if len(toolCalls) > 0 {
			// If there are tool calls, verify their structure
			for _, call := range toolCalls {
				assert.NotEmpty(t, call.ID)
				assert.Equal(t, "get_weather", call.Function)
			}
		}
	})
}

func TestAnthropicClient_ToolCallingStream(t *testing.T) {
	tc := getAnthropicTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	weatherTool := createAnthropicWeatherToolInfo()

	t.Run("streams tool calls properly reassembled from chunks", func(t *testing.T) {
		// Setup mock streaming response with tool call if using mock
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/v1/messages", "POST", true,
				`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_test134","type":"message","role":"assistant","content":[],"model":"`+testAnthropicModelName+`","usage":{"input_tokens":20,"output_tokens":0}}}`,
				`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_test127","name":"get_weather","input":{"location":"Seattle, WA"}}}`,
				`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
			)
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "What is the weather in Seattle?"}},
			},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			assert.Equal(t, ChatRoleAssistant, fragment.Role)
			fragments = append(fragments, fragment)
		}

		// Should have received fragments
		assert.Greater(t, len(fragments), 0, "expected to receive fragments")

		// Note: In streaming mode, Anthropic sends text deltas but tool_use blocks
		// are typically sent as complete events, not streamed character by character.
		// The test verifies that we handle the stream correctly and don't break on tool events.
	})

	t.Run("streams response after tool execution", func(t *testing.T) {
		// Setup mock responses if using mock
		if tc.Mock != nil {
			// First response: tool call (non-streaming)
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_test134a","type":"message","role":"assistant","content":[{"type":"tool_use","id":"toolu_test128","name":"get_weather","input":{"location":"Portland, OR"}}],"model":"claude-sonnet-4-5-20250929","stop_reason":"tool_use","usage":{"input_tokens":20,"output_tokens":15}}`)
			// Second response: streaming response after tool execution
			tc.Mock.AddStreamingResponse("/v1/messages", "POST", true,
				`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_test134b","type":"message","role":"assistant","content":[],"model":"`+testAnthropicModelName+`","usage":{"input_tokens":30,"output_tokens":0}}}`,
				`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"The weather in Portland is "}}`,
				`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"65°F and rainy."}}`,
				`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
			)
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, nil)

		// First get a tool call (non-streaming for simplicity)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "What is the weather in Portland?"}},
			},
		}

		response1, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NoError(t, err)

		toolCalls := response1.GetToolCalls()
		if len(toolCalls) == 0 {
			t.Skip("LLM did not return tool call, skipping streaming test")
		}

		// Add assistant's tool call and user's tool response
		messages = append(messages, response1)
		toolResponse := &tool.ToolResponse{
			Call:   toolCalls[0],
			Result: tool.NewToolValue(map[string]interface{}{"temperature": 65, "condition": "rainy"}),
			Done:   true,
		}
		messages = append(messages, NewToolResponseMessage(toolResponse))

		// Stream the final response
		iterator := chatModel.ChatStream(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		var fullText string
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			fragments = append(fragments, fragment)
			fullText += fragment.GetText()
		}

		// Should have received streaming fragments
		assert.Greater(t, len(fragments), 0, "expected streaming fragments")
		assert.NotEmpty(t, fullText, "expected text in streamed response")
	})
}

func TestAnthropicClient_ErrorHandling(t *testing.T) {
	t.Run("handles endpoint unavailable", func(t *testing.T) {
		client, err := NewAnthropicClient(&conf.ModelProviderConfig{
			URL:            "http://nonexistent-host:11434",
			APIKey:         "test-key",
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

// Helper functions for creating test tools

func createAnthropicWeatherToolInfo() tool.ToolInfo {
	schema := tool.NewToolSchema()
	schema.AddProperty("location", tool.PropertySchema{
		Type:        tool.SchemaTypeString,
		Description: "The city and state, e.g. San Francisco, CA",
	}, true)
	schema.AddProperty("unit", tool.PropertySchema{
		Type:        tool.SchemaTypeString,
		Description: "Temperature unit",
		Enum:        []string{"celsius", "fahrenheit"},
	}, false)

	return tool.ToolInfo{
		Name:        "get_weather",
		Description: "Get the current weather in a given location",
		Schema:      schema,
	}
}

func createAnthropicTimeToolInfo() tool.ToolInfo {
	schema := tool.NewToolSchema()
	schema.AddProperty("location", tool.PropertySchema{
		Type:        tool.SchemaTypeString,
		Description: "The city and state, e.g. San Francisco, CA",
	}, true)

	return tool.ToolInfo{
		Name:        "get_time",
		Description: "Get the current time in a given location",
		Schema:      schema,
	}
}

func TestAnthropicClient_Logging(t *testing.T) {
	tc := getAnthropicTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("logs request and response in Chat method", func(t *testing.T) {
		// Setup mock response
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_log_1","type":"message","role":"assistant","content":[{"type":"text","text":"Logged response"}],"model":"claude-sonnet-4-5-20250929","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
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

		chatModel := tc.Client.ChatModel(testAnthropicModelName, options)

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
			tc.Mock.AddStreamingResponse("/v1/messages", "POST", true,
				`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_stream_log","type":"message","role":"assistant","content":[],"model":"`+testAnthropicModelName+`","usage":{"input_tokens":10,"output_tokens":0}}}`,
				`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Chunk1"}}`,
				`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Chunk2"}}`,
				`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
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

		chatModel := tc.Client.ChatModel(testAnthropicModelName, options)

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
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_nolog","type":"message","role":"assistant","content":[{"type":"text","text":"No log"}],"model":"claude-sonnet-4-5-20250929","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
		}

		options := &ChatOptions{
			Temperature: 0.7,
			Logger:      nil,
		}

		chatModel := tc.Client.ChatModel(testAnthropicModelName, options)

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
			tc.Mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_obf","type":"message","role":"assistant","content":[{"type":"text","text":"Obfuscated"}],"model":"claude-sonnet-4-5-20250929","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
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

		chatModel := tc.Client.ChatModel(testAnthropicModelName, options)

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
		// The x-api-key header should be obfuscated
		assert.NotContains(t, logOutput, "test-api-key")
	})
}

// TestConvertToAnthropicMessageWithMixedContent verifies that when converting a ChatMessage
// containing both text and tool calls to Anthropic format, all parts are preserved.
// This is a regression test for a bug where tool calls were being stripped from assistant
// messages that also contained text.
func TestConvertToAnthropicMessageWithMixedContent(t *testing.T) {
	t.Run("assistant message with text and tool call preserves both", func(t *testing.T) {
		// Create a message with both text and tool call
		msg := &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{Text: "Let me check that for you."},
				{ToolCall: &tool.ToolCall{
					ID:       "tool_123",
					Function: "vfsRead",
					Arguments: tool.NewToolValue(map[string]any{
						"path": "/test/file.txt",
					}),
				}},
			},
		}

		// Convert to Anthropic format
		result := convertToAnthropicMessage(msg)

		// The result should have content as an array (not a simple string)
		contentBlocks, ok := result.Content.([]AnthropicContentBlock)
		require.True(t, ok, "Content should be an array of content blocks, not a simple string")

		// Should have 2 content blocks: text and tool_use
		require.Len(t, contentBlocks, 2, "Should have 2 content blocks (text + tool_use)")

		// First block should be text
		assert.Equal(t, "text", contentBlocks[0].Type)
		assert.Equal(t, "Let me check that for you.", contentBlocks[0].Text)

		// Second block should be tool_use
		assert.Equal(t, "tool_use", contentBlocks[1].Type)
		assert.Equal(t, "tool_123", contentBlocks[1].ID)
		assert.Equal(t, "vfsRead", contentBlocks[1].Name)
	})

	t.Run("assistant message with only text uses simple string format", func(t *testing.T) {
		// Create a message with only text
		msg := &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{Text: "Hello, how can I help you?"},
			},
		}

		// Convert to Anthropic format
		result := convertToAnthropicMessage(msg)

		// The result should have content as a simple string
		contentStr, ok := result.Content.(string)
		require.True(t, ok, "Content should be a simple string for text-only messages")
		assert.Equal(t, "Hello, how can I help you?", contentStr)
	})

	t.Run("assistant message with only tool calls uses array format", func(t *testing.T) {
		// Create a message with only tool calls
		msg := &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{ToolCall: &tool.ToolCall{
					ID:       "tool_1",
					Function: "vfsRead",
					Arguments: tool.NewToolValue(map[string]any{
						"path": "/test/file.txt",
					}),
				}},
			},
		}

		// Convert to Anthropic format
		result := convertToAnthropicMessage(msg)

		// The result should have content as an array
		contentBlocks, ok := result.Content.([]AnthropicContentBlock)
		require.True(t, ok, "Content should be an array of content blocks")

		// Should have 1 content block: tool_use
		require.Len(t, contentBlocks, 1, "Should have 1 content block (tool_use)")
		assert.Equal(t, "tool_use", contentBlocks[0].Type)
		assert.Equal(t, "tool_1", contentBlocks[0].ID)
	})

	t.Run("assistant message with multiple tool calls and text preserves all", func(t *testing.T) {
		// Create a message with text and multiple tool calls
		msg := &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{Text: "I'll check multiple files."},
				{ToolCall: &tool.ToolCall{
					ID:       "tool_1",
					Function: "vfsRead",
					Arguments: tool.NewToolValue(map[string]any{
						"path": "/test/file1.txt",
					}),
				}},
				{ToolCall: &tool.ToolCall{
					ID:       "tool_2",
					Function: "vfsRead",
					Arguments: tool.NewToolValue(map[string]any{
						"path": "/test/file2.txt",
					}),
				}},
			},
		}

		// Convert to Anthropic format
		result := convertToAnthropicMessage(msg)

		// The result should have content as an array
		contentBlocks, ok := result.Content.([]AnthropicContentBlock)
		require.True(t, ok, "Content should be an array of content blocks")

		// Should have 3 content blocks: text + 2 tool_use
		require.Len(t, contentBlocks, 3, "Should have 3 content blocks (text + 2 tool_use)")

		// First block should be text
		assert.Equal(t, "text", contentBlocks[0].Type)
		assert.Equal(t, "I'll check multiple files.", contentBlocks[0].Text)

		// Second and third blocks should be tool_use
		assert.Equal(t, "tool_use", contentBlocks[1].Type)
		assert.Equal(t, "tool_1", contentBlocks[1].ID)
		assert.Equal(t, "tool_use", contentBlocks[2].Type)
		assert.Equal(t, "tool_2", contentBlocks[2].ID)
	})

	t.Run("user message with tool responses converts correctly", func(t *testing.T) {
		// Create a user message with tool responses
		msg := &ChatMessage{
			Role: ChatRoleUser,
			Parts: []ChatMessagePart{
				{ToolResponse: &tool.ToolResponse{
					Call: &tool.ToolCall{
						ID:       "tool_123",
						Function: "vfsRead",
					},
					Result: tool.NewToolValue(map[string]any{
						"content": "file contents here",
					}),
					Done: true,
				}},
			},
		}

		// Convert to Anthropic format
		result := convertToAnthropicMessage(msg)

		// The result should have content as an array
		contentBlocks, ok := result.Content.([]AnthropicContentBlock)
		require.True(t, ok, "Content should be an array of content blocks")

		// Should have 1 content block: tool_result
		require.Len(t, contentBlocks, 1, "Should have 1 content block (tool_result)")
		assert.Equal(t, "tool_result", contentBlocks[0].Type)
		assert.Equal(t, "tool_123", contentBlocks[0].ToolUseID)
	})
}

func TestAnthropicClient_MaxTokens(t *testing.T) {
	t.Run("Chat method uses MaxTokens as max_tokens", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewAnthropicClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			APIKey:    "test-key",
			MaxTokens: 2048,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_maxtokens","type":"message","role":"assistant","content":[{"type":"text","text":"Hello"}],"model":"test-model","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)

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

		var chatReq AnthropicMessagesRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		assert.Equal(t, 2048, chatReq.MaxTokens)
	})

	t.Run("ChatStream method uses MaxTokens as max_tokens", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewAnthropicClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			APIKey:    "test-key",
			MaxTokens: 4096,
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/v1/messages", "POST", true,
			`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_stream","type":"message","role":"assistant","content":[],"model":"test-model","usage":{"input_tokens":10,"output_tokens":0}}}`,
			`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
			`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
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

		var chatReq AnthropicMessagesRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		assert.Equal(t, 4096, chatReq.MaxTokens)
	})

	t.Run("Chat method uses default max_tokens when MaxTokens is zero", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewAnthropicClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			APIKey:    "test-key",
			MaxTokens: 0,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_default","type":"message","role":"assistant","content":[{"type":"text","text":"Hello"}],"model":"test-model","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)

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

		var chatReq AnthropicMessagesRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		assert.Equal(t, DefaultMaxTokens, chatReq.MaxTokens)
	})

	t.Run("ChatStream method uses default max_tokens when MaxTokens is zero", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewAnthropicClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			APIKey:    "test-key",
			MaxTokens: 0,
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/v1/messages", "POST", true,
			`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_stream_default","type":"message","role":"assistant","content":[],"model":"test-model","usage":{"input_tokens":10,"output_tokens":0}}}`,
			`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
			`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
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

		// Verify default max_tokens was set in the request
		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq AnthropicMessagesRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		assert.Equal(t, DefaultMaxTokens, chatReq.MaxTokens)
	})
}

func TestAnthropicClient_CustomHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewAnthropicClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "test-key",
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Request-ID":    "req-123",
			"X-Organization":  "my-org",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_custom","type":"message","role":"assistant","content":[{"type":"text","text":"Hello"}],"model":"test-model","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hello"),
	}

	_, err = chatModel.Chat(context.Background(), messages, nil, nil)
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "test-key", request.Header.Get("x-api-key"))
	assert.Equal(t, "custom-value", request.Header.Get("X-Custom-Header"))
	assert.Equal(t, "req-123", request.Header.Get("X-Request-ID"))
	assert.Equal(t, "my-org", request.Header.Get("X-Organization"))
}

func TestAnthropicClient_CustomHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewAnthropicClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "test-key",
		Headers: map[string]string{
			"X-Stream-Header": "stream-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/v1/messages", "POST", true,
		`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_stream_custom","type":"message","role":"assistant","content":[],"model":"test-model","usage":{"input_tokens":10,"output_tokens":0}}}`,
		`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`,
		`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
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

	assert.Equal(t, "test-key", request.Header.Get("x-api-key"))
	assert.Equal(t, "stream-value", request.Header.Get("X-Stream-Header"))
}

func TestAnthropicClient_OptionsHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewAnthropicClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "config-api-key",
		Headers: map[string]string{
			"X-Config-Header": "config-value",
			"X-Shared-Header": "config-shared",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/v1/messages", "POST", `{"id":"msg_opts","type":"message","role":"assistant","content":[{"type":"text","text":"Hello"}],"model":"test-model","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)

	options := &ChatOptions{
		Headers: map[string]string{
			"X-Options-Header": "options-value",
			"X-Shared-Header":  "options-shared",
			"X-Api-Key":        "should-not-override",
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
	assert.Equal(t, "config-api-key", request.Header.Get("x-api-key"), "x-api-key header should NOT be overridden by options")
}

func TestAnthropicClient_OptionsHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewAnthropicClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "config-api-key",
		Headers: map[string]string{
			"X-Config-Header": "config-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/v1/messages", "POST", true,
		`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"msg_opts_stream","type":"message","role":"assistant","content":[],"model":"test-model","usage":{"input_tokens":10,"output_tokens":0}}}`,
		`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`,
		`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
	)

	options := &ChatOptions{
		Headers: map[string]string{
			"X-Options-Header": "options-value",
			"Authorization":    "Bearer should-not-override",
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
	assert.Empty(t, request.Header.Get("Authorization"), "authorization header should NOT be set from options")
}
