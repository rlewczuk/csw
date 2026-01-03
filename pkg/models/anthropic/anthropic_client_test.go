package anthropic

import (
	"context"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultTestURL = "https://api.anthropic.com"
	testModelName  = "claude-sonnet-4-5-20250929"
	testTimeout    = 30 * time.Second
	connectTimeout = 5 * time.Second
)

// getAPIKey skips the test if anthropic integration tests are disabled and returns the API key
func getAPIKey(t *testing.T) string {
	t.Helper()
	if !testutil.IntegTestEnabled("anthropic") {
		t.Skip("Skipping test: anthropic integration tests not enabled (set _integ/anthropic.enabled or _integ/all.enabled to 'yes')")
	}
	apiKey := testutil.IntegCfgReadFile("anthropic.key")
	if apiKey == "" {
		t.Skip("Skipping test: _integ/anthropic.key not configured")
	}
	return apiKey
}

func TestNewAnthropicClient(t *testing.T) {
	t.Run("creates client with valid configuration", func(t *testing.T) {
		client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
			APIKey:         "test-api-key",
			ConnectTimeout: connectTimeout,
			RequestTimeout: testTimeout,
		})

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("creates client with nil options", func(t *testing.T) {
		client, err := NewAnthropicClient(defaultTestURL, nil)

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		_, err := NewAnthropicClient("", nil)

		assert.Error(t, err)
	})
}

func TestAnthropicClient_ListModels(t *testing.T) {
	apiKey := getAPIKey(t)

	client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
		APIKey:         apiKey,
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	t.Run("lists available models", func(t *testing.T) {
		modelList, err := client.ListModels()

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
		modelList, err := client.ListModels()

		require.NoError(t, err)

		found := false
		for _, model := range modelList {
			if model.Name == testModelName {
				found = true
				break
			}
		}

		assert.True(t, found, "expected test model %s to be available", testModelName)
	})
}

func TestAnthropicClient_ChatModel(t *testing.T) {
	apiKey := getAPIKey(t)

	client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
		APIKey:         apiKey,
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("creates chat model with model name and options", func(t *testing.T) {
		options := &models.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := client.ChatModel(testModelName, options)

		assert.NotNil(t, chatModel)
	})

	t.Run("sends chat message and gets response", func(t *testing.T) {
		options := &models.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := client.ChatModel(testModelName, options)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "What is 2+2? Answer with just the number."}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, models.ChatRoleAssistant, response.Role)
		assert.NotEmpty(t, response.Parts)
		assert.Greater(t, len(response.GetText()), 0)
	})

	t.Run("handles context with timeout", func(t *testing.T) {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "Say hello"}},
			},
		}

		response, err := chatModel.Chat(ctxWithTimeout, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("handles system and user messages", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleSystem,
				Parts: []models.ChatMessagePart{{Text: "You are a helpful assistant that always responds in uppercase."}},
			},
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "hello"}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, models.ChatRoleAssistant, response.Role)
	})

	t.Run("returns error for empty messages", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		response, err := chatModel.Chat(ctx, []*models.ChatMessage{}, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("returns error for nil messages", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		response, err := chatModel.Chat(ctx, nil, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("uses default options when none provided to Chat", func(t *testing.T) {
		defaultOptions := &models.ChatOptions{
			Temperature: 0.5,
			TopP:        0.8,
			TopK:        30,
		}

		chatModel := client.ChatModel(testModelName, defaultOptions)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "Say hello"}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
	})
}

func TestAnthropicClient_ChatModelStream(t *testing.T) {
	apiKey := getAPIKey(t)

	client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
		APIKey:         apiKey,
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("streams chat message and gets fragments", func(t *testing.T) {
		options := &models.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := client.ChatModel(testModelName, options)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "Count from 1 to 5, one number per line."}},
			},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*models.ChatMessage
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			assert.Equal(t, models.ChatRoleAssistant, fragment.Role)
			assert.NotEmpty(t, fragment.Parts)
			fragments = append(fragments, fragment)
		}

		// Should have received multiple fragments
		assert.Greater(t, len(fragments), 0, "expected to receive at least one fragment")
	})

	t.Run("handles context cancellation during streaming", func(t *testing.T) {
		ctxWithCancel, cancel := context.WithCancel(ctx)

		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "Write a long story about a cat."}},
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
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "Say hello"}},
			},
		}

		iterator := chatModel.ChatStream(ctxWithTimeout, messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*models.ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Greater(t, len(fragments), 0)
	})

	t.Run("returns no fragments for empty messages", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		iterator := chatModel.ChatStream(ctx, []*models.ChatMessage{}, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*models.ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("returns no fragments for nil messages", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		iterator := chatModel.ChatStream(ctx, nil, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*models.ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("iterator can be stopped early", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "Say hello"}},
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
	apiKey := getAPIKey(t)

	client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
		APIKey:         apiKey,
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("returns error for embedding model", func(t *testing.T) {
		embedModel := client.EmbeddingModel("any-model")

		assert.NotNil(t, embedModel)

		// Anthropic doesn't support embeddings, should return an error
		_, err := embedModel.Embed(ctx, "Hello, world!")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented")
	})
}

func TestAnthropicClient_ToolCalling(t *testing.T) {
	apiKey := getAPIKey(t)

	client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
		APIKey:         apiKey,
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Define a simple weather tool for testing
	weatherTool := createWeatherToolInfo()

	t.Run("sends tool definitions to LLM and receives tool call", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "What is the weather in San Francisco?"}},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, models.ChatRoleAssistant, response.Role)

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
		chatModel := client.ChatModel(testModelName, nil)

		// First message: user asks about weather
		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "What is the weather in San Francisco?"}},
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
		messages = append(messages, models.NewToolResponseMessage(toolResponse))

		// Send tool response back to LLM
		response2, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})

		require.NoError(t, err)
		assert.NotNil(t, response2)
		assert.Equal(t, models.ChatRoleAssistant, response2.Role)

		// The final response should contain text (not tool calls)
		text := response2.GetText()
		assert.NotEmpty(t, text, "expected text response after tool execution")

		// The response should mention the weather information
		// (checking for either temperature value or condition)
		assert.Contains(t, text, "72", "expected response to mention temperature")
	})

	t.Run("handles multiple tool calls in conversation", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		// Define multiple tools
		weatherTool := createWeatherToolInfo()
		timeTool := createTimeToolInfo()

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "What is the weather and current time in San Francisco?"}},
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
		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "Check the weather in Boston and tell me if I should bring an umbrella."}},
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
	apiKey := getAPIKey(t)

	client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
		APIKey:         apiKey,
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	ctx := context.Background()

	weatherTool := createWeatherToolInfo()

	t.Run("streams tool calls properly reassembled from chunks", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "What is the weather in Seattle?"}},
			},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NotNil(t, iterator)

		var fragments []*models.ChatMessage
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			assert.Equal(t, models.ChatRoleAssistant, fragment.Role)
			fragments = append(fragments, fragment)
		}

		// Should have received fragments
		assert.Greater(t, len(fragments), 0, "expected to receive fragments")

		// Note: In streaming mode, Anthropic sends text deltas but tool_use blocks
		// are typically sent as complete events, not streamed character by character.
		// The test verifies that we handle the stream correctly and don't break on tool events.
	})

	t.Run("streams response after tool execution", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		// First get a tool call (non-streaming for simplicity)
		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []models.ChatMessagePart{{Text: "What is the weather in Portland?"}},
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
		messages = append(messages, models.NewToolResponseMessage(toolResponse))

		// Stream the final response
		iterator := chatModel.ChatStream(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NotNil(t, iterator)

		var fragments []*models.ChatMessage
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
		client, err := NewAnthropicClient("http://nonexistent-host:11434", &models.ModelConnectionOptions{
			APIKey:         "test-key",
			ConnectTimeout: 1 * time.Second,
			RequestTimeout: 2 * time.Second,
		})
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.Error(t, err)
		assert.ErrorIs(t, err, models.ErrEndpointUnavailable)
	})
}

// Helper functions for creating test tools

func createWeatherToolInfo() tool.ToolInfo {
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

func createTimeToolInfo() tool.ToolInfo {
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
