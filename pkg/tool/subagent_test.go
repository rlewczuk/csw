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
		executor := &subAgentExecutorMock{result: SubAgentTaskResult{Status: "completed", Summary: "child done", FinalTodoList: []TodoItem{{ID: "1", Content: "done", Status: "completed", Priority: "low"}}}}
		tool := NewSubAgentTool(executor)
		response := tool.Execute(&ToolCall{Function: "subAgent", Arguments: NewToolValue(map[string]any{"slug": "child-task", "title": "Task", "prompt": "Do task", "role": "developer", "model": "mock/test-model", "thinking": "high"})})
		require.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "completed", response.Result.String("status"))
		assert.Equal(t, "child done", response.Result.String("summary"))
		require.Len(t, response.Result.Get("final_todo_list").Array(), 1)
		assert.Equal(t, "child-task", executor.request.Slug)
		assert.Equal(t, "Task", executor.request.Title)
		assert.Equal(t, "Do task", executor.request.Prompt)
		assert.Equal(t, "developer", executor.request.Role)
		assert.Equal(t, "mock/test-model", executor.request.Model)
		assert.Equal(t, "high", executor.request.Thinking)
	})
}
