package models

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
