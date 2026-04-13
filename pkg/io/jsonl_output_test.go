package io

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonlSessionOutput_ShowMessage(t *testing.T) {
	tests := []struct {
		name        string
		messageType string
		expected    string
	}{
		{name: "info", messageType: "info", expected: "[INFO] hello\n"},
		{name: "warning", messageType: "warning", expected: "[WARNING] hello\n"},
		{name: "error", messageType: "error", expected: "[ERROR] hello\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}
			output := NewJsonlSessionOutput(buffer)

			output.ShowMessage("hello", tt.messageType)

			assert.Equal(t, tt.expected, buffer.String())
		})
	}
}

func TestJsonlSessionOutput_AddAssistantMessage(t *testing.T) {
	buffer := &bytes.Buffer{}
	output := NewJsonlSessionOutput(buffer)

	output.AddAssistantMessage("Done", "Thinking")

	actual := buffer.String()
	assert.Contains(t, actual, "*Thinking*")
	assert.Contains(t, actual, "Assistant: Done")
	assert.NotContains(t, actual, "[main]")
}

func TestJsonlSessionOutput_AddToolCallResult(t *testing.T) {
	tests := []struct {
		name            string
		result          *tool.ToolResponse
		expectedContain string
	}{
		{
			name: "prefers jsonl field",
			result: &tool.ToolResponse{
				Call: &tool.ToolCall{ID: "c1", Function: "vfsRead"},
				Result: tool.NewToolValue(map[string]any{
					"jsonl":   `{"tool":"vfsRead"}`,
					"summary": "read notes.txt",
				}),
				Done: true,
			},
			expectedContain: `{"tool":"vfsRead"}`,
		},
		{
			name: "fallback to summary",
			result: &tool.ToolResponse{
				Call: &tool.ToolCall{ID: "c2", Function: "todoRead"},
				Result: tool.NewToolValue(map[string]any{
					"summary": "todoRead",
				}),
				Done: true,
			},
			expectedContain: "todoRead",
		},
		{
			name: "fallback to details",
			result: &tool.ToolResponse{
				Call:  &tool.ToolCall{ID: "c3", Function: "runBash"},
				Error: errors.New("boom"),
				Result: tool.NewToolValue(map[string]any{
					"details": "command failed",
				}),
				Done: true,
			},
			expectedContain: "command failed",
		},
		{
			name: "fallback to function name",
			result: &tool.ToolResponse{
				Call:   &tool.ToolCall{ID: "c4", Function: "taskList"},
				Result: tool.NewToolValue(map[string]any{}),
				Done:   true,
			},
			expectedContain: "taskList",
		},
		{
			name: "appends notification json",
			result: &tool.ToolResponse{
				Call: &tool.ToolCall{ID: "c5", Function: "vfsRead"},
				Result: tool.NewToolValue(map[string]any{
					"jsonl": `{"tool":"vfsRead","status":"success"}`,
				}),
				Notifications: []tool.ToolNotification{{
					Type:    "agents_auto_loaded",
					Path:    "pkg/foo/AGENTS.md",
					Message: `AGENTS.md from "pkg/foo" was automatically loaded.`,
				}},
				Done: true,
			},
			expectedContain: `"type":"notification"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}
			output := NewJsonlSessionOutput(buffer)

			output.AddToolCallResult(tt.result)

			assert.Contains(t, buffer.String(), tt.expectedContain)
		})
	}
}

func TestJsonlSessionOutput_AddToolCallResultDeduplicates(t *testing.T) {
	buffer := &bytes.Buffer{}
	output := NewJsonlSessionOutput(buffer)

	result := &tool.ToolResponse{
		Call: &tool.ToolCall{ID: "call-1", Function: "vfsRead"},
		Result: tool.NewToolValue(map[string]any{
			"jsonl": `{"tool":"vfsRead","status":"success"}`,
		}),
		Done: true,
	}

	output.AddToolCallResult(result)
	output.AddToolCallResult(result)

	count := strings.Count(buffer.String(), `{"tool":"vfsRead","status":"success"}`)
	assert.Equal(t, 1, count)
}

func TestJsonlSessionOutput_AddToolCallResultNotificationOnly(t *testing.T) {
	buffer := &bytes.Buffer{}
	output := NewJsonlSessionOutput(buffer)

	result := &tool.ToolResponse{
		Call: &tool.ToolCall{ID: "call-2", Function: "vfsRead"},
		Notifications: []tool.ToolNotification{{
			Type:    "agents_auto_loaded",
			Path:    "pkg/AGENTS.md",
			Message: `AGENTS.md from "pkg" was automatically loaded.`,
		}},
		Done: true,
	}

	output.AddToolCallResult(result)

	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	require.NotEmpty(t, lines)
	line := lines[len(lines)-1]
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &payload))
	assert.Equal(t, "notification", payload["type"])
	notificationObj, ok := payload["notification"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "agents_auto_loaded", notificationObj["Type"])
	assert.Equal(t, "pkg/AGENTS.md", notificationObj["Path"])
}

func TestJsonlSessionOutput_NoPanicsOnNilInputs(t *testing.T) {
	buffer := &bytes.Buffer{}
	output := NewJsonlSessionOutput(buffer)

	require.NotPanics(t, func() {
		output.AddToolCall(nil)
		output.AddToolCallResult(nil)
		output.OnRateLimitError(3)
		output.RunFinished(nil)
	})
	assert.False(t, output.ShouldRetryAfterFailure("failed"))
}
