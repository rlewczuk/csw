package tool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type subAgentExecutorMock struct {
	request SubAgentTaskRequest
	result  SubAgentTaskResult
	err     error
}

func (m *subAgentExecutorMock) ExecuteSubAgentTask(request SubAgentTaskRequest) (SubAgentTaskResult, error) {
	m.request = request
	if m.err != nil {
		return SubAgentTaskResult{}, m.err
	}
	return m.result, nil
}

func TestSubAgentToolExecute(t *testing.T) {
	t.Run("returns error when executor is nil", func(t *testing.T) {
		tool := NewSubAgentTool(nil)
		response := tool.Execute(&ToolCall{Function: "subAgent", Arguments: NewToolValue(map[string]any{"slug": "child", "title": "Task", "prompt": "Do task"})})
		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "executor is nil")
	})

	t.Run("returns validation error for missing slug", func(t *testing.T) {
		tool := NewSubAgentTool(&subAgentExecutorMock{})
		response := tool.Execute(&ToolCall{Function: "subAgent", Arguments: NewToolValue(map[string]any{"title": "t", "prompt": "p"})})
		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "missing required argument: slug")
	})

	t.Run("returns validation error for missing title", func(t *testing.T) {
		tool := NewSubAgentTool(&subAgentExecutorMock{})
		response := tool.Execute(&ToolCall{Function: "subAgent", Arguments: NewToolValue(map[string]any{"slug": "child", "prompt": "p"})})
		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "missing required argument: title")
	})

	t.Run("returns validation error for missing prompt", func(t *testing.T) {
		tool := NewSubAgentTool(&subAgentExecutorMock{})
		response := tool.Execute(&ToolCall{Function: "subAgent", Arguments: NewToolValue(map[string]any{"slug": "child", "title": "t"})})
		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "missing required argument: prompt")
	})

	t.Run("returns executor error", func(t *testing.T) {
		executor := &subAgentExecutorMock{err: fmt.Errorf("boom")}
		tool := NewSubAgentTool(executor)
		response := tool.Execute(&ToolCall{Function: "subAgent", Arguments: NewToolValue(map[string]any{"slug": "child", "title": "Task", "prompt": "Do task"})})
		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "boom")
	})

	t.Run("returns summary payload from executor", func(t *testing.T) {
		executor := &subAgentExecutorMock{result: SubAgentTaskResult{Status: "completed", Summary: "child done"}}
		tool := NewSubAgentTool(executor)
		response := tool.Execute(&ToolCall{Function: "subAgent", Arguments: NewToolValue(map[string]any{"slug": "child-task", "title": "Task", "prompt": "Do task", "role": "developer", "model": "mock/test-model", "thinking": "high"})})
		require.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "completed", response.Result.String("status"))
		assert.Equal(t, "child done", response.Result.String("summary"))
		assert.False(t, response.Result.Has("error"))
		assert.False(t, response.Result.Has("final_todo_list"))
		assert.Equal(t, "child-task", executor.request.Slug)
		assert.Equal(t, "Task", executor.request.Title)
		assert.Equal(t, "Do task", executor.request.Prompt)
		assert.Equal(t, "developer", executor.request.Role)
		assert.Equal(t, "mock/test-model", executor.request.Model)
		assert.Equal(t, "high", executor.request.Thinking)
	})

	t.Run("includes error field when provided by executor", func(t *testing.T) {
		executor := &subAgentExecutorMock{result: SubAgentTaskResult{Status: "error", Summary: "failed", Error: "subagent failed"}}
		tool := NewSubAgentTool(executor)
		response := tool.Execute(&ToolCall{Function: "subAgent", Arguments: NewToolValue(map[string]any{"slug": "child-task", "title": "Task", "prompt": "Do task"})})
		require.NoError(t, response.Error)
		assert.Equal(t, "error", response.Result.String("status"))
		assert.Equal(t, "failed", response.Result.String("summary"))
		assert.Equal(t, "subagent failed", response.Result.String("error"))
	})

	t.Run("trims text arguments before passing to executor", func(t *testing.T) {
		executor := &subAgentExecutorMock{result: SubAgentTaskResult{Status: "completed", Summary: "ok"}}
		tool := NewSubAgentTool(executor)
		response := tool.Execute(&ToolCall{Function: "subAgent", Arguments: NewToolValue(map[string]any{
			"slug":     "  child-task  ",
			"title":    "  Task  ",
			"prompt":   "Do task",
			"role":     "  developer  ",
			"model":    "  mock/test-model  ",
			"thinking": "  high  ",
		})})
		require.NoError(t, response.Error)
		assert.Equal(t, "child-task", executor.request.Slug)
		assert.Equal(t, "Task", executor.request.Title)
		assert.Equal(t, "developer", executor.request.Role)
		assert.Equal(t, "mock/test-model", executor.request.Model)
		assert.Equal(t, "high", executor.request.Thinking)
	})
}
