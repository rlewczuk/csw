package tool

import (
	"context"
	"fmt"
	"strconv"
)

// TaskListFunc lists task records.
type TaskListFunc func(ctx context.Context, identifier string, fallbackTaskID string, recursive bool) ([]TaskRecord, error)

// TaskListTool lists tasks.
type TaskListTool struct {
	listTasks TaskListFunc
	session   TaskSessionRef
}

// NewTaskListTool creates a new TaskListTool instance.
func NewTaskListTool(listTasks TaskListFunc, session TaskSessionRef) *TaskListTool {
	return &TaskListTool{listTasks: listTasks, session: session}
}

// GetDescription returns additional dynamic description.
func (t *TaskListTool) GetDescription() (string, bool) { return "", false }

// Execute executes task list.
func (t *TaskListTool) Execute(args *ToolCall) *ToolResponse {
	if t.listTasks == nil {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskListTool.Execute() [task_list.go]: listTasks is nil"), Done: true}
	}

	identifier, fallback := resolveTaskIdentifier(args, t.session)
	tasks, err := t.listTasks(context.Background(), identifier, fallback, args.Arguments.Bool("recursive"))
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
	target := taskRenderTarget(call)
	recursive := false
	if call != nil {
		recursive = call.Arguments.Bool("recursive")
	}
	count := taskRenderCount(call, "tasks")

	summary := "taskList " + target
	if recursive {
		summary += " recursive"
	}
	if count > 0 {
		summary += formatResultCount(count)
	}
	summary = truncateString(summary, 128)

	details := summary + "\nreturned=" + strconv.Itoa(count)
	jsonl := buildToolRenderJSONL("taskList", call, map[string]any{"target": target, "recursive": recursive, "count": count})
	return summary, details, jsonl, map[string]string{}
}
