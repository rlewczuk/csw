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
func (t *TodoWriteTool) Execute(args *ToolCall) *ToolResponse {
	todosValue, ok := args.Arguments.GetOK("todos")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: missing required argument: todos"),
			Done:  true,
		}
	}

	todosArray, ok := todosValue.ArrayOK()
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todos must be an array"),
			Done:  true,
		}
	}

	// Parse todo items from the array
	todos := make([]TodoItem, 0, len(todosArray))
	for i, todoValue := range todosArray {
		todoObj, ok := todoValue.ObjectOK()
		if !ok {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d is not an object", i),
				Done:  true,
			}
		}

		id, ok := todoObj["id"].AsStringOK()
		if !ok {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d missing or invalid 'id'", i),
				Done:  true,
			}
		}

		content, ok := todoObj["content"].AsStringOK()
		if !ok {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d missing or invalid 'content'", i),
				Done:  true,
			}
		}

		status, ok := todoObj["status"].AsStringOK()
		if !ok {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d missing or invalid 'status'", i),
				Done:  true,
			}
		}

		priority, ok := todoObj["priority"].AsStringOK()
		if !ok {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d missing or invalid 'priority'", i),
				Done:  true,
			}
		}

		// Validate status
		if status != "pending" && status != "in_progress" && status != "completed" && status != "cancelled" {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("TodoWriteTool.Execute() [todo.go]: todo item at index %d has invalid status: %s", i, status),
				Done:  true,
			}
		}

		// Validate priority
		if priority != "low" && priority != "medium" && priority != "high" {
			return &ToolResponse{
				Call:  args,
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

	return &ToolResponse{
		Call:   args,
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
func (t *TodoReadTool) Execute(args *ToolCall) *ToolResponse {
	todos := t.session.GetTodoList()
	pending := t.session.CountPendingTodos()

	// Convert todos to ToolValue format
	todosJSON, err := json.Marshal(todos)
	if err != nil {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("TodoReadTool.Execute() [todo.go]: failed to marshal todos: %w", err),
			Done:  true,
		}
	}

	var todosArray []any
	if err := json.Unmarshal(todosJSON, &todosArray); err != nil {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("TodoReadTool.Execute() [todo.go]: failed to unmarshal todos: %w", err),
			Done:  true,
		}
	}

	var result ToolValue
	result.Set("todos", todosArray)
	result.Set("pending", pending)

	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Render returns a string representation of the tool call.
func (t *TodoWriteTool) Render(call *ToolCall) (string, string, map[string]string) {
	todos := t.session.GetTodoList()
	oneLiner, full := renderTodoList(todos)
	return oneLiner, full, make(map[string]string)
}

// Render returns a string representation of the tool call.
func (t *TodoReadTool) Render(call *ToolCall) (string, string, map[string]string) {
	todos := t.session.GetTodoList()
	oneLiner, full := renderTodoList(todos)
	return oneLiner, full, make(map[string]string)
}

// renderTodoList renders the todo list in one-liner and full description formats.
// One-liner format: (5/12 current task description)
// Full format: Todo list header followed by tasks with status indicators.
func renderTodoList(todos []TodoItem) (string, string) {
	if len(todos) == 0 {
		return "(0/0 no tasks)", "Todo list: 0/0 tasks completed\n"
	}

	total := len(todos)
	completed := 0
	inProgressIdx := -1
	lastCompletedIdx := -1

	for i, todo := range todos {
		switch todo.Status {
		case "completed":
			completed++
			lastCompletedIdx = i
		case "in_progress":
			inProgressIdx = i
		}
	}

	// First number: tasks done + task in progress (starting from 1)
	// If there's an in_progress task, count all completed + 1 (the in_progress one)
	donePlusInProgress := completed
	if inProgressIdx != -1 {
		donePlusInProgress++
	}
	// If no in_progress but we have completed tasks, the "current" is the last completed
	// If nothing completed and no in_progress, current is the first task
	if donePlusInProgress == 0 {
		donePlusInProgress = 1
	}

	// Determine current task description
	var currentTask string
	if inProgressIdx != -1 {
		currentTask = todos[inProgressIdx].Content
	} else if lastCompletedIdx != -1 {
		currentTask = todos[lastCompletedIdx].Content
	} else {
		currentTask = todos[0].Content
	}

	oneLiner := fmt.Sprintf("(%d/%d %s)", donePlusInProgress, total, currentTask)

	// Build full description
	full := fmt.Sprintf("Todo list: %d/%d tasks completed\n", completed, total)
	for _, todo := range todos {
		var statusIcon string
		switch todo.Status {
		case "completed":
			statusIcon = "[✓]"
		case "in_progress":
			statusIcon = "[*]"
		case "cancelled":
			statusIcon = "[-]"
		default: // pending
			statusIcon = "[ ]"
		}
		full += fmt.Sprintf("%s %s\n", statusIcon, todo.Content)
	}

	return oneLiner, full
}
