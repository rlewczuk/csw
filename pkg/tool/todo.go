package tool

import (
	"encoding/json"
	"fmt"
	"strings"
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

func (t *TodoWriteTool) GetDescription() (string, bool) {
	return "", false
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

func (t *TodoReadTool) GetDescription() (string, bool) {
	return "", false
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
// One-liner format: (6/11) Current task to be done.
// Full format: list of all todos with status indicators [ ], [X], [*].
func renderTodoList(todos []TodoItem) (string, string) {
	if len(todos) == 0 {
		return "(0/0) No current task.", ""
	}

	total := len(todos)
	completed := 0
	inProgressIdx := -1
	firstPendingIdx := -1

	for i, todo := range todos {
		switch todo.Status {
		case "completed":
			completed++
		case "in_progress":
			inProgressIdx = i
		case "pending":
			if firstPendingIdx == -1 {
				firstPendingIdx = i
			}
		}
	}

	// First number: completed + 1 when there is a current actionable task
	// (in_progress or pending).
	progress := completed
	if inProgressIdx != -1 {
		progress++
	} else if firstPendingIdx != -1 {
		progress++
	}

	// Determine current task description
	var currentTask string
	if inProgressIdx != -1 {
		currentTask = todos[inProgressIdx].Content
	} else if firstPendingIdx != -1 {
		currentTask = todos[firstPendingIdx].Content
	} else {
		currentTask = todos[total-1].Content
	}
	currentTask = strings.TrimSpace(currentTask)
	if currentTask == "" {
		currentTask = "No current task"
	}
	if !strings.HasSuffix(currentTask, ".") {
		currentTask += "."
	}

	oneLiner := fmt.Sprintf("(%d/%d) %s", progress, total, currentTask)

	// Build full description
	full := ""
	for _, todo := range todos {
		var statusIcon string
		switch todo.Status {
		case "completed":
			statusIcon = "[X]"
		case "in_progress":
			statusIcon = "[*]"
		default:
			statusIcon = "[ ]"
		}
		full += fmt.Sprintf("%s %s\n", statusIcon, todo.Content)
	}

	return oneLiner, full
}
