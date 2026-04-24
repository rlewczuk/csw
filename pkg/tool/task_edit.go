package tool

import (
	"context"
	"fmt"
	"strings"
)

// TaskEditFunc edits task prompt content using string replacement.
type TaskEditFunc func(ctx context.Context, identifier string, fallbackTaskID string, oldString string, newString string, replaceAll bool) (TaskRecord, error)

// TaskEditTool edits task prompt fragments.
type TaskEditTool struct {
	editTask TaskEditFunc
	session  TaskSessionRef
}

// NewTaskEditTool creates a new TaskEditTool instance.
func NewTaskEditTool(editTask TaskEditFunc, session TaskSessionRef) *TaskEditTool {
	return &TaskEditTool{editTask: editTask, session: session}
}

// GetDescription returns additional dynamic description.
func (t *TaskEditTool) GetDescription() (string, bool) { return "", false }

// Execute executes task prompt edit.
func (t *TaskEditTool) Execute(args *ToolCall) *ToolResponse {
	if t.editTask == nil {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskEditTool.Execute() [task_edit.go]: editTask is nil"), Done: true}
	}

	identifier, fallback := resolveTaskIdentifier(args, t.session)
	if strings.TrimSpace(identifier) == "" && strings.TrimSpace(fallback) == "" {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskEditTool.Execute() [task_edit.go]: missing task identifier (name or uuid)"), Done: true}
	}

	oldString, ok := args.Arguments.StringOK("oldString")
	if !ok {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskEditTool.Execute() [task_edit.go]: missing required argument: oldString"), Done: true}
	}

	newString, ok := args.Arguments.StringOK("newString")
	if !ok {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskEditTool.Execute() [task_edit.go]: missing required argument: newString"), Done: true}
	}

	updated, err := t.editTask(context.Background(), identifier, fallback, oldString, newString, args.Arguments.Bool("replaceAll"))
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	result := NewToolValue(map[string]any{"task": taskRecordToToolValue(updated).Raw(), "uuid": updated.UUID})

	return &ToolResponse{Call: args, Result: result, Done: true}
}

// Render returns human-readable representation.
func (t *TaskEditTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	target := taskRenderTarget(call)
	replaceAll := false
	if call != nil {
		replaceAll = call.Arguments.Bool("replaceAll")
	}

	summary := truncateString("taskEdit "+target, 128)
	details := summary
	jsonlExtra := map[string]any{"target": target, "replace_all": replaceAll}
	if replaceAll {
		details += "\nreplace_all=true"
	}

	jsonl := buildToolRenderJSONL("taskEdit", call, jsonlExtra)
	return summary, details, jsonl, map[string]string{}
}
