package tool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hookFeedbackExecutorMock struct {
	request  HookFeedbackRequest
	response HookFeedbackResponse
}

func (m *hookFeedbackExecutorMock) ExecuteHookFeedback(request HookFeedbackRequest) HookFeedbackResponse {
	m.request = request
	return m.response
}

func TestHookFeedbackToolExecute(t *testing.T) {
	t.Run("returns error when executor is nil", func(t *testing.T) {
		tool := NewHookFeedbackTool(nil, nil, nil)
		response := tool.Execute(&ToolCall{Function: "hookFeedback", Arguments: NewToolValue(map[string]any{"fn": "context"})})
		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "executor is nil")
	})

	t.Run("returns validation error when fn is missing", func(t *testing.T) {
		tool := NewHookFeedbackTool(&hookFeedbackExecutorMock{}, nil, nil)
		response := tool.Execute(&ToolCall{Function: "hookFeedback", Arguments: NewToolValue(map[string]any{})})
		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "missing required argument: fn")
	})

	t.Run("returns validation error when args is not object", func(t *testing.T) {
		tool := NewHookFeedbackTool(&hookFeedbackExecutorMock{}, nil, nil)
		response := tool.Execute(&ToolCall{Function: "hookFeedback", Arguments: NewToolValue(map[string]any{"fn": "context", "args": []any{"bad"}})})
		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "args must be an object")
	})

	t.Run("passes context payload with id", func(t *testing.T) {
		executor := &hookFeedbackExecutorMock{
			response: HookFeedbackResponse{ID: "ctx-1", Fn: "context", OK: true, Result: map[string]any{"alpha": "one"}},
		}
		tool := NewHookFeedbackTool(executor, nil, nil)
		response := tool.Execute(&ToolCall{Function: "hookFeedback", Arguments: NewToolValue(map[string]any{
			"fn":   "context",
			"id":   "ctx-1",
			"args": map[string]any{"alpha": "one"},
		})})

		require.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "context", executor.request.Fn)
		assert.Equal(t, "ctx-1", executor.request.ID)
		assert.Equal(t, "one", executor.request.Args["alpha"])
		assert.Equal(t, "ctx-1", response.Result.String("id"))
		assert.True(t, response.Result.Bool("ok"))
	})

	t.Run("injects defaults for llm model and thinking", func(t *testing.T) {
		executor := &hookFeedbackExecutorMock{response: HookFeedbackResponse{Fn: "llm", OK: true, Result: map[string]any{"text": "ok"}}}
		tool := NewHookFeedbackTool(executor, func() string { return "mock/test-model" }, func() string { return "high" })
		response := tool.Execute(&ToolCall{Function: "hookFeedback", Arguments: NewToolValue(map[string]any{
			"fn":   "llm",
			"args": map[string]any{"prompt": "Hello"},
		})})

		require.NoError(t, response.Error)
		assert.Equal(t, "mock/test-model", executor.request.Args["model"])
		assert.Equal(t, "high", executor.request.Args["thinking"])
	})

	t.Run("returns failure when executor reports error", func(t *testing.T) {
		executor := &hookFeedbackExecutorMock{response: HookFeedbackResponse{Fn: "llm", OK: false, Error: "boom"}}
		tool := NewHookFeedbackTool(executor, nil, nil)
		response := tool.Execute(&ToolCall{Function: "hookFeedback", Arguments: NewToolValue(map[string]any{
			"fn":   "llm",
			"args": map[string]any{"prompt": "Hello"},
		})})

		require.Error(t, response.Error)
		assert.Equal(t, "boom", response.Error.Error())
		assert.False(t, response.Result.Bool("ok"))
		assert.Equal(t, "boom", response.Result.String("error"))
	})

	t.Run("returns fallback error when executor response missing error text", func(t *testing.T) {
		executor := &hookFeedbackExecutorMock{response: HookFeedbackResponse{Fn: "context", OK: false}}
		tool := NewHookFeedbackTool(executor, nil, nil)
		response := tool.Execute(&ToolCall{Function: "hookFeedback", Arguments: NewToolValue(map[string]any{
			"fn": "context",
		})})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "hook feedback request failed")
	})
}

func TestHookFeedbackToolRender(t *testing.T) {
	tool := NewHookFeedbackTool(&hookFeedbackExecutorMock{}, nil, nil)
	oneLiner, full, meta := tool.Render(&ToolCall{Function: "hookFeedback", Arguments: NewToolValue(map[string]any{"fn": "context"})})
	assert.Equal(t, "hookFeedback context", oneLiner)
	assert.Equal(t, oneLiner, full)
	assert.Empty(t, meta)
}

func TestHookFeedbackArgString(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want string
	}{
		{name: "nil args", args: nil, key: "x", want: ""},
		{name: "missing key", args: map[string]any{"x": "1"}, key: "y", want: ""},
		{name: "string value", args: map[string]any{"x": "1"}, key: "x", want: "1"},
		{name: "non-string value", args: map[string]any{"x": 2}, key: "x", want: "2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hookFeedbackArgString(tc.args, tc.key))
		})
	}
}

func TestHookFeedbackToolExecuteErrorMessageCoverage(t *testing.T) {
	executor := &hookFeedbackExecutorMock{response: HookFeedbackResponse{Fn: "llm", OK: false, Error: "bad-request"}}
	tool := NewHookFeedbackTool(executor, nil, nil)
	response := tool.Execute(&ToolCall{Function: "hookFeedback", Arguments: NewToolValue(map[string]any{
		"fn":   "llm",
		"args": map[string]any{"prompt": "Hello"},
	})})
	require.Error(t, response.Error)
	assert.Equal(t, fmt.Errorf("bad-request").Error(), response.Error.Error())
}
