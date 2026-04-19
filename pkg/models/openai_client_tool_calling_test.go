package models

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIClient_ToolCalling(t *testing.T) {
	tc := getOpenAIToolCallingTestClient(t)
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

		chatModel := tc.Client.ChatModel(testOpenAIToolCallingModelName, &ChatOptions{
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

		chatModel := tc.Client.ChatModel(testOpenAIToolCallingModelName, &ChatOptions{
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

		chatModel := tc.Client.ChatModel(testOpenAIToolCallingModelName, &ChatOptions{
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

		chatModel := tc.Client.ChatModel(testOpenAIToolCallingModelName, &ChatOptions{
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

		chatModel := tc.Client.ChatModel(testOpenAIToolCallingModelName, &ChatOptions{
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

const testOpenAIToolCallingModelName = "devstral-small-2:latest"

type openaiToolCallingTestClient struct {
	Client *OpenAIClient
	Mock   *testutil.MockHTTPServer
}

func (tc *openaiToolCallingTestClient) Close() {
	if tc.Mock != nil {
		tc.Mock.Close()
	}
}

func getOpenAIToolCallingTestClient(t *testing.T) *openaiToolCallingTestClient {
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

		return &openaiToolCallingTestClient{Client: client}
	}

	mock := testutil.NewMockHTTPServer()
	client, err := NewOpenAIClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	return &openaiToolCallingTestClient{Client: client, Mock: mock}
}
