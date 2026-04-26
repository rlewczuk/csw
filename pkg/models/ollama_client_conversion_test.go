package models

import (
	"errors"
	"testing"

	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToOllamaMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     *ChatMessage
		assertF func(t *testing.T, got OllamaMessage)
	}{
		{
			name: "converts plain text message",
			msg:  NewTextMessage(ChatRoleUser, "hello"),
			assertF: func(t *testing.T, got OllamaMessage) {
				assert.Equal(t, "user", got.Role)
				assert.Equal(t, "hello", got.Content)
				assert.Empty(t, got.ToolCalls)
			},
		},
		{
			name: "converts tool call message",
			msg: &ChatMessage{
				Role: ChatRoleAssistant,
				Parts: []ChatMessagePart{
					{Text: "calling tool"},
					{ToolCall: &tool.ToolCall{ID: "call1", Function: "get_weather", Arguments: tool.NewToolValue(map[string]interface{}{"city": "Paris"})}},
				},
			},
			assertF: func(t *testing.T, got OllamaMessage) {
				require.Len(t, got.ToolCalls, 1)
				assert.Equal(t, "assistant", got.Role)
				assert.Equal(t, "calling tool", got.Content)
				assert.Equal(t, "get_weather", got.ToolCalls[0].Function.Name)
				assert.Equal(t, "Paris", got.ToolCalls[0].Function.Arguments["city"])
			},
		},
		{
			name: "converts tool response result message",
			msg: NewToolResponseMessage(&tool.ToolResponse{
				Call:   &tool.ToolCall{ID: "call2", Function: "get_weather"},
				Result: tool.NewToolValue(map[string]interface{}{"temp": 21}),
				Done:   true,
			}),
			assertF: func(t *testing.T, got OllamaMessage) {
				assert.Equal(t, "tool", got.Role)
				assert.Equal(t, "get_weather", got.ToolName)
				assert.Contains(t, got.Content, "temp")
			},
		},
		{
			name: "converts tool response error message",
			msg: NewToolResponseMessage(&tool.ToolResponse{
				Call:  &tool.ToolCall{ID: "call3", Function: "get_weather"},
				Error: errors.New("failed"),
				Done:  true,
			}),
			assertF: func(t *testing.T, got OllamaMessage) {
				assert.Equal(t, "tool", got.Role)
				assert.Equal(t, "get_weather", got.ToolName)
				assert.Equal(t, "Error: failed", got.Content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToOllamaMessage(tt.msg)
			tt.assertF(t, got)
		})
	}
}

func TestConvertFromOllamaMessage(t *testing.T) {
	msg := OllamaMessage{
		Role:    "assistant",
		Content: "hello",
		ToolCalls: []OllamaToolCall{
			{Function: OllamaToolCallFunction{Name: "get_weather", Arguments: map[string]interface{}{"city": "Paris"}}},
		},
	}

	got := convertFromOllamaMessage(msg)
	require.NotNil(t, got)
	assert.Equal(t, ChatRoleAssistant, got.Role)
	assert.Equal(t, "hello", got.GetText())

	toolCalls := got.GetToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "get_weather", toolCalls[0].Function)
	city, ok := toolCalls[0].Arguments.StringOK("city")
	assert.True(t, ok)
	assert.Equal(t, "Paris", city)
}

func TestConvertToolsToOllama(t *testing.T) {
	tools := []tool.ToolInfo{{
		Name:        "get_weather",
		Description: "Get weather",
		Schema: tool.ToolSchema{
			Type: tool.SchemaTypeObject,
			Properties: map[string]tool.PropertySchema{
				"city": {Type: tool.SchemaTypeString},
			},
			Required:             []string{"city"},
			AdditionalProperties: false,
		},
	}}

	got := convertToolsToOllama(tools)
	require.Len(t, got, 1)
	assert.Equal(t, "function", got[0].Type)
	assert.Equal(t, "get_weather", got[0].Function.Name)
	assert.Equal(t, "Get weather", got[0].Function.Description)
	require.NotNil(t, got[0].Function.Parameters)
	assert.Contains(t, got[0].Function.Parameters, "properties")
}

func TestGenerateOllamaToolCallID(t *testing.T) {
	id1 := generateOllamaToolCallID()
	id2 := generateOllamaToolCallID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "call_")
}
