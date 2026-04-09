package io

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextSessionOutput_ShowMessage(t *testing.T) {
	tests := []struct {
		name        string
		messageType string
		expected    string
	}{
		{name: "info", messageType: "info", expected: "\x1b[90m[main]\x1b[0m [INFO] hello\n"},
		{name: "warning", messageType: "warning", expected: "\x1b[90m[main]\x1b[0m [WARNING] hello\n"},
		{name: "error", messageType: "error", expected: "\x1b[90m[main]\x1b[0m [ERROR] hello\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}
			output := NewTextSessionOutput(buffer)

			output.ShowMessage("hello", tt.messageType)

			assert.Equal(t, tt.expected, buffer.String())
		})
	}
}

func TestTextSessionOutput_AddAssistantMessage(t *testing.T) {
	tests := []struct {
		name            string
		text            string
		thinking        string
		expectedContain []string
	}{
		{
			name:            "text and thinking",
			text:            "Done",
			thinking:        "Let me think",
			expectedContain: []string{"*Let me think*", "Assistant: Done"},
		},
		{
			name:            "only text",
			text:            "Done",
			thinking:        "",
			expectedContain: []string{"Assistant: Done"},
		},
		{
			name:            "only thinking",
			text:            "",
			thinking:        "Reasoning",
			expectedContain: []string{"*Reasoning*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}
			output := NewTextSessionOutput(buffer)

			output.AddAssistantMessage(tt.text, tt.thinking)

			for _, fragment := range tt.expectedContain {
				assert.Contains(t, buffer.String(), fragment)
			}
		})
	}
}

func TestTextSessionOutput_AddToolCallResult(t *testing.T) {
	tests := []struct {
		name           string
		result         *tool.ToolResponse
		expectedOutput string
	}{
		{
			name: "succeeded with summary",
			result: &tool.ToolResponse{
				Call: &tool.ToolCall{ID: "c1", Function: "vfsRead"},
				Result: tool.NewToolValue(map[string]any{
					"summary": "read notes.txt",
				}),
				Done: true,
			},
			expectedOutput: "✅ read notes.txt",
		},
		{
			name: "failed with details fallback",
			result: &tool.ToolResponse{
				Call:  &tool.ToolCall{ID: "c2", Function: "runBash"},
				Error: errors.New("exit 1"),
				Result: tool.NewToolValue(map[string]any{
					"details": "command failed",
				}),
				Done: true,
			},
			expectedOutput: "❌ command failed",
		},
		{
			name: "fallback to function name",
			result: &tool.ToolResponse{
				Call:   &tool.ToolCall{ID: "c3", Function: "todoRead"},
				Result: tool.NewToolValue(map[string]any{}),
				Done:   true,
			},
			expectedOutput: "✅ todoRead",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}
			output := NewTextSessionOutput(buffer)

			output.AddToolCallResult(tt.result)

			assert.Contains(t, buffer.String(), tt.expectedOutput)
		})
	}
}

func TestTextSessionOutput_AddToolCallResultDeduplicates(t *testing.T) {
	buffer := &bytes.Buffer{}
	output := NewTextSessionOutput(buffer)

	result := &tool.ToolResponse{
		Call: &tool.ToolCall{ID: "call-1", Function: "vfsRead"},
		Result: tool.NewToolValue(map[string]any{
			"summary": "read notes.txt",
		}),
		Done: true,
	}

	output.AddToolCallResult(result)
	output.AddToolCallResult(result)

	count := strings.Count(buffer.String(), "✅ read notes.txt")
	assert.Equal(t, 1, count)
}

func TestTextSessionOutput_NoPanicsOnNilInputs(t *testing.T) {
	buffer := &bytes.Buffer{}
	output := NewTextSessionOutput(buffer)

	require.NotPanics(t, func() {
		output.AddToolCall(nil)
		output.AddToolCallResult(nil)
		output.OnPermissionQuery(nil)
		output.OnRateLimitError(3)
		output.RunFinished(nil)
	})
	assert.False(t, output.ShouldRetryAfterFailure("failed"))
}
