package tool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTodoSession is a test double for TodoSession interface.
type MockTodoSession struct {
	todoList []TodoItem
}

func NewMockTodoSession() *MockTodoSession {
	return &MockTodoSession{
		todoList: make([]TodoItem, 0),
	}
}

func (m *MockTodoSession) GetTodoList() []TodoItem {
	list := make([]TodoItem, len(m.todoList))
	copy(list, m.todoList)
	return list
}

func (m *MockTodoSession) SetTodoList(todos []TodoItem) {
	m.todoList = make([]TodoItem, len(todos))
	copy(m.todoList, todos)
}

func (m *MockTodoSession) CountPendingTodos() int {
	count := 0
	for _, item := range m.todoList {
		if item.Status == "pending" || item.Status == "in_progress" {
			count++
		}
	}
	return count
}

func TestTodoWriteTool(t *testing.T) {
	t.Run("should write todo list successfully", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		tool := NewTodoWriteTool(mockSession)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "todoWrite",
			Arguments: NewToolValue(map[string]any{
				"todos": []any{
					map[string]any{
						"id":       "1",
						"content":  "Task 1",
						"status":   "pending",
						"priority": "high",
					},
					map[string]any{
						"id":       "2",
						"content":  "Task 2",
						"status":   "in_progress",
						"priority": "medium",
					},
					map[string]any{
						"id":       "3",
						"content":  "Task 3",
						"status":   "completed",
						"priority": "low",
					},
				},
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "Todo list updated with 3 items", response.Result.Get("message").AsString())
		assert.Equal(t, int64(2), response.Result.Get("pending").AsInt())

		// Verify the todo list was updated in the session
		todos := mockSession.GetTodoList()
		require.Len(t, todos, 3)
		assert.Equal(t, "1", todos[0].ID)
		assert.Equal(t, "Task 1", todos[0].Content)
		assert.Equal(t, "pending", todos[0].Status)
		assert.Equal(t, "high", todos[0].Priority)
	})

	t.Run("should return error for missing todos argument", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		tool := NewTodoWriteTool(mockSession)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "todoWrite",
			Arguments: NewToolValue(map[string]any{}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "missing required argument: todos")
	})

	t.Run("should return error for non-array todos", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		tool := NewTodoWriteTool(mockSession)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "todoWrite",
			Arguments: NewToolValue(map[string]any{
				"todos": "not an array",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "todos must be an array")
	})

	t.Run("should return error for invalid todo item object", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		tool := NewTodoWriteTool(mockSession)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "todoWrite",
			Arguments: NewToolValue(map[string]any{
				"todos": []any{
					"not an object",
				},
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "todo item at index 0 is not an object")
	})

	t.Run("should return error for missing id field", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		tool := NewTodoWriteTool(mockSession)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "todoWrite",
			Arguments: NewToolValue(map[string]any{
				"todos": []any{
					map[string]any{
						"content":  "Task 1",
						"status":   "pending",
						"priority": "high",
					},
				},
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "missing or invalid 'id'")
	})

	t.Run("should return error for invalid status", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		tool := NewTodoWriteTool(mockSession)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "todoWrite",
			Arguments: NewToolValue(map[string]any{
				"todos": []any{
					map[string]any{
						"id":       "1",
						"content":  "Task 1",
						"status":   "invalid_status",
						"priority": "high",
					},
				},
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "invalid status")
	})

	t.Run("should return error for invalid priority", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		tool := NewTodoWriteTool(mockSession)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "todoWrite",
			Arguments: NewToolValue(map[string]any{
				"todos": []any{
					map[string]any{
						"id":       "1",
						"content":  "Task 1",
						"status":   "pending",
						"priority": "invalid_priority",
					},
				},
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "invalid priority")
	})
}

func TestTodoReadTool(t *testing.T) {
	t.Run("should read empty todo list", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		tool := NewTodoReadTool(mockSession)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "todoRead",
			Arguments: NewToolValue(map[string]any{}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		todosArray := response.Result.Get("todos").Array()
		assert.Len(t, todosArray, 0)
		assert.Equal(t, int64(0), response.Result.Get("pending").AsInt())
	})

	t.Run("should read todo list with items", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		mockSession.SetTodoList([]TodoItem{
			{
				ID:       "1",
				Content:  "Task 1",
				Status:   "pending",
				Priority: "high",
			},
			{
				ID:       "2",
				Content:  "Task 2",
				Status:   "in_progress",
				Priority: "medium",
			},
			{
				ID:       "3",
				Content:  "Task 3",
				Status:   "completed",
				Priority: "low",
			},
		})
		tool := NewTodoReadTool(mockSession)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "todoRead",
			Arguments: NewToolValue(map[string]any{}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		todosArray := response.Result.Get("todos").Array()
		require.Len(t, todosArray, 3)

		// Check first item
		firstItem := todosArray[0].Object()
		assert.Equal(t, "1", firstItem["id"].AsString())
		assert.Equal(t, "Task 1", firstItem["content"].AsString())
		assert.Equal(t, "pending", firstItem["status"].AsString())
		assert.Equal(t, "high", firstItem["priority"].AsString())

		// Check pending count (2 items: pending + in_progress)
		assert.Equal(t, int64(2), response.Result.Get("pending").AsInt())
	})
}

func TestTodoIntegration(t *testing.T) {
	t.Run("should write and then read todo list", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		writeTool := NewTodoWriteTool(mockSession)
		readTool := NewTodoReadTool(mockSession)

		// Write todos
		writeResponse := writeTool.Execute(&ToolCall{
			ID:       "write-id",
			Function: "todoWrite",
			Arguments: NewToolValue(map[string]any{
				"todos": []any{
					map[string]any{
						"id":       "1",
						"content":  "First task",
						"status":   "pending",
						"priority": "high",
					},
					map[string]any{
						"id":       "2",
						"content":  "Second task",
						"status":   "in_progress",
						"priority": "medium",
					},
				},
			}),
		})
		require.NoError(t, writeResponse.Error)

		// Read todos
		readResponse := readTool.Execute(&ToolCall{
			ID:        "read-id",
			Function:  "todoRead",
			Arguments: NewToolValue(map[string]any{}),
		})
		require.NoError(t, readResponse.Error)

		// Verify
		todosArray := readResponse.Result.Get("todos").Array()
		require.Len(t, todosArray, 2)
		assert.Equal(t, int64(2), readResponse.Result.Get("pending").AsInt())

		firstItem := todosArray[0].Object()
		assert.Equal(t, "1", firstItem["id"].AsString())
		assert.Equal(t, "First task", firstItem["content"].AsString())
	})

	t.Run("should update pending count when status changes", func(t *testing.T) {
		// Setup
		mockSession := NewMockTodoSession()
		writeTool := NewTodoWriteTool(mockSession)

		// Write initial todos with 2 pending
		writeResponse := writeTool.Execute(&ToolCall{
			ID:       "write-1",
			Function: "todoWrite",
			Arguments: NewToolValue(map[string]any{
				"todos": []any{
					map[string]any{
						"id":       "1",
						"content":  "Task 1",
						"status":   "pending",
						"priority": "high",
					},
					map[string]any{
						"id":       "2",
						"content":  "Task 2",
						"status":   "pending",
						"priority": "medium",
					},
				},
			}),
		})
		require.NoError(t, writeResponse.Error)
		assert.Equal(t, int64(2), writeResponse.Result.Get("pending").AsInt())

		// Update one to completed
		writeResponse = writeTool.Execute(&ToolCall{
			ID:       "write-2",
			Function: "todoWrite",
			Arguments: NewToolValue(map[string]any{
				"todos": []any{
					map[string]any{
						"id":       "1",
						"content":  "Task 1",
						"status":   "completed",
						"priority": "high",
					},
					map[string]any{
						"id":       "2",
						"content":  "Task 2",
						"status":   "pending",
						"priority": "medium",
					},
				},
			}),
		})
		require.NoError(t, writeResponse.Error)
		assert.Equal(t, int64(1), writeResponse.Result.Get("pending").AsInt())
	})
}

func TestTodoWriteTool_Render(t *testing.T) {
		t.Run("should render empty todo list", func(t *testing.T) {
		mockSession := NewMockTodoSession()
		tool := NewTodoWriteTool(mockSession)
		call := &ToolCall{ID: "test-id", Function: "todoWrite", Arguments: NewToolValue(map[string]any{})}

		oneLiner, full, _, extra := tool.Render(call)

		assert.Equal(t, "(0/0) No current task.", oneLiner)
		assert.Equal(t, "", full)
		assert.Empty(t, extra)
	})

	t.Run("should render todo list with all pending tasks", func(t *testing.T) {
		mockSession := NewMockTodoSession()
		mockSession.SetTodoList([]TodoItem{
			{ID: "1", Content: "First task", Status: "pending", Priority: "high"},
			{ID: "2", Content: "Second task", Status: "pending", Priority: "medium"},
		})
		tool := NewTodoWriteTool(mockSession)
		call := &ToolCall{ID: "test-id", Function: "todoWrite", Arguments: NewToolValue(map[string]any{})}

		oneLiner, full, _, extra := tool.Render(call)

		assert.Equal(t, "(1/2) First task.", oneLiner)
		assert.Contains(t, full, "[ ] First task")
		assert.Contains(t, full, "[ ] Second task")
		assert.Empty(t, extra)
	})

	t.Run("should render todo list with in_progress task", func(t *testing.T) {
		mockSession := NewMockTodoSession()
		mockSession.SetTodoList([]TodoItem{
			{ID: "1", Content: "First task", Status: "completed", Priority: "high"},
			{ID: "2", Content: "Second task", Status: "in_progress", Priority: "medium"},
			{ID: "3", Content: "Third task", Status: "pending", Priority: "low"},
		})
		tool := NewTodoWriteTool(mockSession)
		call := &ToolCall{ID: "test-id", Function: "todoWrite", Arguments: NewToolValue(map[string]any{})}

		oneLiner, full, _, extra := tool.Render(call)

		assert.Equal(t, "(2/3) Second task.", oneLiner)
		assert.Contains(t, full, "[X] First task")
		assert.Contains(t, full, "[*] Second task")
		assert.Contains(t, full, "[ ] Third task")
		assert.Empty(t, extra)
	})

	t.Run("should render todo list with all completed tasks", func(t *testing.T) {
		mockSession := NewMockTodoSession()
		mockSession.SetTodoList([]TodoItem{
			{ID: "1", Content: "First task", Status: "completed", Priority: "high"},
			{ID: "2", Content: "Second task", Status: "completed", Priority: "medium"},
		})
		tool := NewTodoWriteTool(mockSession)
		call := &ToolCall{ID: "test-id", Function: "todoWrite", Arguments: NewToolValue(map[string]any{})}

		oneLiner, full, _, extra := tool.Render(call)

		assert.Equal(t, "(2/2) Second task.", oneLiner)
		assert.Contains(t, full, "[X] First task")
		assert.Contains(t, full, "[X] Second task")
		assert.Empty(t, extra)
	})

	t.Run("should render todo list with cancelled tasks", func(t *testing.T) {
		mockSession := NewMockTodoSession()
		mockSession.SetTodoList([]TodoItem{
			{ID: "1", Content: "First task", Status: "completed", Priority: "high"},
			{ID: "2", Content: "Cancelled task", Status: "cancelled", Priority: "medium"},
			{ID: "3", Content: "Third task", Status: "pending", Priority: "low"},
		})
		tool := NewTodoWriteTool(mockSession)
		call := &ToolCall{ID: "test-id", Function: "todoWrite", Arguments: NewToolValue(map[string]any{})}

		oneLiner, full, _, extra := tool.Render(call)

		assert.Equal(t, "(2/3) Third task.", oneLiner)
		assert.Contains(t, full, "[X] First task")
		assert.Contains(t, full, "[ ] Cancelled task")
		assert.Contains(t, full, "[ ] Third task")
		assert.Empty(t, extra)
	})
}

func TestTodoReadTool_Render(t *testing.T) {
	t.Run("should render empty todo list", func(t *testing.T) {
		mockSession := NewMockTodoSession()
		tool := NewTodoReadTool(mockSession)
		call := &ToolCall{ID: "test-id", Function: "todoRead", Arguments: NewToolValue(map[string]any{})}

		oneLiner, full, _, extra := tool.Render(call)

		assert.Equal(t, "(0/0) No current task.", oneLiner)
		assert.Equal(t, "", full)
		assert.Empty(t, extra)
	})

	t.Run("should render todo list with mixed statuses", func(t *testing.T) {
		mockSession := NewMockTodoSession()
		mockSession.SetTodoList([]TodoItem{
			{ID: "1", Content: "Completed task", Status: "completed", Priority: "high"},
			{ID: "2", Content: "In progress task", Status: "in_progress", Priority: "medium"},
			{ID: "3", Content: "Pending task", Status: "pending", Priority: "low"},
			{ID: "4", Content: "Cancelled task", Status: "cancelled", Priority: "low"},
		})
		tool := NewTodoReadTool(mockSession)
		call := &ToolCall{ID: "test-id", Function: "todoRead", Arguments: NewToolValue(map[string]any{})}

		oneLiner, full, _, extra := tool.Render(call)

		assert.Equal(t, "(2/4) In progress task.", oneLiner)
		assert.Contains(t, full, "[X] Completed task")
		assert.Contains(t, full, "[*] In progress task")
		assert.Contains(t, full, "[ ] Pending task")
		assert.Contains(t, full, "[ ] Cancelled task")
		assert.Empty(t, extra)
	})
}

func TestRenderTodoList(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		oneLiner, full := renderTodoList([]TodoItem{})
		assert.Equal(t, "(0/0) No current task.", oneLiner)
		assert.Equal(t, "", full)
	})

	t.Run("single pending task", func(t *testing.T) {
		todos := []TodoItem{
			{ID: "1", Content: "Only task", Status: "pending", Priority: "high"},
		}
		oneLiner, full := renderTodoList(todos)
		assert.Equal(t, "(1/1) Only task.", oneLiner)
		assert.Contains(t, full, "[ ] Only task")
	})

	t.Run("single in_progress task", func(t *testing.T) {
		todos := []TodoItem{
			{ID: "1", Content: "Current task", Status: "in_progress", Priority: "high"},
		}
		oneLiner, full := renderTodoList(todos)
		assert.Equal(t, "(1/1) Current task.", oneLiner)
		assert.Contains(t, full, "[*] Current task")
	})

	t.Run("single completed task", func(t *testing.T) {
		todos := []TodoItem{
			{ID: "1", Content: "Done task", Status: "completed", Priority: "high"},
		}
		oneLiner, full := renderTodoList(todos)
		assert.Equal(t, "(1/1) Done task.", oneLiner)
		assert.Contains(t, full, "[X] Done task")
	})

	t.Run("progressive completion", func(t *testing.T) {
		// All pending - first task shown
		todos := []TodoItem{
			{ID: "1", Content: "Task 1", Status: "pending", Priority: "high"},
			{ID: "2", Content: "Task 2", Status: "pending", Priority: "medium"},
			{ID: "3", Content: "Task 3", Status: "pending", Priority: "low"},
		}
		oneLiner, _ := renderTodoList(todos)
		assert.Equal(t, "(1/3) Task 1.", oneLiner)

		// First completed
		todos[0].Status = "completed"
		oneLiner, _ = renderTodoList(todos)
		assert.Equal(t, "(2/3) Task 2.", oneLiner)

		// Second in progress
		todos[1].Status = "in_progress"
		oneLiner, _ = renderTodoList(todos)
		assert.Equal(t, "(2/3) Task 2.", oneLiner)

		// Second completed
		todos[1].Status = "completed"
		oneLiner, _ = renderTodoList(todos)
		assert.Equal(t, "(3/3) Task 3.", oneLiner)

		// All completed
		todos[2].Status = "completed"
		oneLiner, _ = renderTodoList(todos)
		assert.Equal(t, "(3/3) Task 3.", oneLiner)
	})
}
