package tool

import (
	"context"
	"fmt"
)

// TaskListTool lists tasks.
type TaskListTool struct {
	backend TaskBackend
	session TaskSessionRef
}

// NewTaskListTool creates a new TaskListTool instance.
func NewTaskListTool(backend TaskBackend, session TaskSessionRef) *TaskListTool {
	return &TaskListTool{backend: backend, session: session}
}

// GetDescription returns additional dynamic description.
func (t *TaskListTool) GetDescription() (string, bool) { return "", false }

// Execute executes task list.
func (t *TaskListTool) Execute(args *ToolCall) *ToolResponse {
	if t.backend == nil {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskListTool.Execute() [task_list.go]: backend is nil"), Done: true}
	}

	identifier, fallback := resolveTaskIdentifier(args, t.session)
	tasks, err := t.backend.ListTasks(context.Background(), identifier, fallback, args.Arguments.Bool("recursive"))
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	list := make([]any, 0, len(tasks))
	for _, item := range tasks {
		list = append(list, taskRecordToToolValue(item).Raw())
	}

	return &ToolResponse{Call: args, Result: NewToolValue(map[string]any{"tasks": list}), Done: true}
}

// Render returns human-readable representation.
func (t *TaskListTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	one := truncateString("taskList", 128)
	jsonl := buildToolRenderJSONL("taskList", call, map[string]any{})
	return one, one, jsonl, map[string]string{}
}
