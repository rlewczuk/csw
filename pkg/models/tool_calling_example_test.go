package models_test

import (
	"context"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/models/mock"
	"github.com/codesnort/codesnort-swe/pkg/tool"
)

// TestToolCallingExample demonstrates how tool calling works with the new API
func TestToolCallingExample(t *testing.T) {
	// Create a mock provider
	provider := mock.NewMockProvider(nil)

	// Define a tool
	toolInfo := tool.ToolInfo{
		Name:        "get_weather",
		Description: "Get the current weather in a location",
		Schema: tool.ToolSchema{
			Type: tool.SchemaTypeObject,
			Properties: map[string]tool.PropertySchema{
				"location": {
					Type:        tool.SchemaTypeString,
					Description: "The city and state, e.g. San Francisco, CA",
				},
			},
			Required:             []string{"location"},
			AdditionalProperties: false,
		},
	}

	// Configure the mock to return a tool call
	toolCall := &tool.ToolCall{
		ID:       "call_123",
		Function: "get_weather",
		Arguments: tool.NewToolValue(map[string]interface{}{
			"location": "San Francisco, CA",
		}),
	}

	provider.SetChatResponse("test-model", &mock.ChatResponse{
		Response: models.NewToolCallMessage(toolCall),
	})

	// Create chat model
	chatModel := provider.ChatModel("test-model", nil)

	// Send a message asking about weather
	messages := []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleUser, "What's the weather in San Francisco?"),
	}

	// Call with tools
	response, err := chatModel.Chat(context.Background(), messages, nil, []tool.ToolInfo{toolInfo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the response contains a tool call
	toolCalls := response.GetToolCalls()
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function != "get_weather" {
		t.Errorf("expected function 'get_weather', got '%s'", toolCalls[0].Function)
	}

	// Verify the mock provider recorded the tool call
	if len(provider.RecordedToolCalls) != 1 {
		t.Errorf("expected 1 recorded tool call, got %d", len(provider.RecordedToolCalls))
	}

	// Simulate sending tool response back
	toolResponse := &tool.ToolResponse{
		Call:   toolCall,
		Result: tool.NewToolValue("Sunny, 72°F"),
		Done:   true,
	}

	provider.SetChatResponse("test-model", &mock.ChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, "The weather in San Francisco is sunny and 72°F."),
	})

	// Send the tool response back
	messages = append(messages, response)
	messages = append(messages, models.NewToolResponseMessage(toolResponse))

	finalResponse, err := chatModel.Chat(context.Background(), messages, nil, []tool.ToolInfo{toolInfo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we got a text response
	if finalResponse.GetText() == "" {
		t.Error("expected text response, got empty string")
	}

	// Verify the mock provider recorded the tool response
	if len(provider.RecordedToolResponses) != 1 {
		t.Errorf("expected 1 recorded tool response, got %d", len(provider.RecordedToolResponses))
	}
}
