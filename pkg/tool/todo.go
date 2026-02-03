package tool

import (
	"encoding/json"
	"fmt"
)

// TodoSession is an interface for accessing todo list functionality from a session.
type TodoSession interface {
	GetTodoList() []TodoItem
	SetTodoList(todos []TodoItem)
	CountPendingTodos() int
}

// TodoItem represents a single task in the todo list.
type TodoItem struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

// TodoWriteTool implements the todoWrite tool for updating the todo list.
type TodoWriteTool struct {
	session TodoSession
}

// NewTodoWriteTool creates a new TodoWriteTool instance.
func NewTodoWriteTool(session TodoSession) *TodoWriteTool {
	return &TodoWriteTool{session: session}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *TodoWriteTool) Execute(args ToolCall) ToolResponse {
	todosValue, ok := args.Arguments.GetOK("todos")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: missing required argument: todos"),
			Done:  true,
		}
	}

	todosArray, ok := todosValue.ArrayOK()
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todos must be an array"),
			Done:  true,
		}
	}

	// Parse todo items from the array
	todos := make([]TodoItem, 0, len(todosArray))
	for i, todoValue := range todosArray {
		todoObj, ok := todoValue.ObjectOK()
		if !ok {
			return ToolResponse{
				Call:  &args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d is not an object", i),
				Done:  true,
			}
		}

		id, ok := todoObj["id"].AsStringOK()
		if !ok {
			return ToolResponse{
				Call:  &args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d missing or invalid 'id'", i),
				Done:  true,
			}
		}

		content, ok := todoObj["content"].AsStringOK()
		if !ok {
			return ToolResponse{
				Call:  &args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d missing or invalid 'content'", i),
				Done:  true,
			}
		}

		status, ok := todoObj["status"].AsStringOK()
		if !ok {
			return ToolResponse{
				Call:  &args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d missing or invalid 'status'", i),
				Done:  true,
			}
		}

		priority, ok := todoObj["priority"].AsStringOK()
		if !ok {
			return ToolResponse{
				Call:  &args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d missing or invalid 'priority'", i),
				Done:  true,
			}
		}

		// Validate status
		if status != "pending" && status != "in_progress" && status != "completed" && status != "cancelled" {
			return ToolResponse{
				Call:  &args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d has invalid status: %s", i, status),
				Done:  true,
			}
		}

		// Validate priority
		if priority != "low" && priority != "medium" && priority != "high" {
			return ToolResponse{
				Call:  &args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d has invalid priority: %s", i, priority),
				Done:  true,
			}
		}

		todos = append(todos, TodoItem{
			ID:       id,
			Content:  content,
			Status:   status,
			Priority: priority,
		})
	}

	// Update the session's todo list
	t.session.SetTodoList(todos)

	// Return success message
	var result ToolValue
	result.Set("message", fmt.Sprintf("Todo list updated with %d items", len(todos)))
	result.Set("pending", t.session.CountPendingTodos())

	return ToolResponse{
		Call:   &args,
		Result: result,
		Done:   true,
	}
}

// TodoReadTool implements the todoRead tool for retrieving the current todo list.
type TodoReadTool struct {
	session TodoSession
}

// NewTodoReadTool creates a new TodoReadTool instance.
func NewTodoReadTool(session TodoSession) *TodoReadTool {
	return &TodoReadTool{session: session}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *TodoReadTool) Execute(args ToolCall) ToolResponse {
	todos := t.session.GetTodoList()
	pending := t.session.CountPendingTodos()

	// Convert todos to ToolValue format
	todosJSON, err := json.Marshal(todos)
	if err != nil {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("TodoReadTool.Execute() [todo.go]: failed to marshal todos: %w", err),
			Done:  true,
		}
	}

	var todosArray []any
	if err := json.Unmarshal(todosJSON, &todosArray); err != nil {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("TodoReadTool.Execute() [todo.go]: failed to unmarshal todos: %w", err),
			Done:  true,
		}
	}

	var result ToolValue
	result.Set("todos", todosArray)
	result.Set("pending", pending)

	return ToolResponse{
		Call:   &args,
		Result: result,
		Done:   true,
	}
}
