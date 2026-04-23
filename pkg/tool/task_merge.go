package tool

import (
	"context"
	"fmt"
)

// TaskMergeFunc merges a task record.
type TaskMergeFunc func(ctx context.Context, identifier string, fallbackTaskID string) (TaskRecord, error)

// TaskMergeTool merges tasks.
type TaskMergeTool struct {
	mergeTask TaskMergeFunc
	session   TaskSessionRef
}

// NewTaskMergeTool creates a new TaskMergeTool instance.
func NewTaskMergeTool(mergeTask TaskMergeFunc, session TaskSessionRef) *TaskMergeTool {
	return &TaskMergeTool{mergeTask: mergeTask, session: session}
}

// GetDescription returns additional dynamic description.
func (t *TaskMergeTool) GetDescription() (string, bool) { return "", false }

// Execute executes task merge.
func (t *TaskMergeTool) Execute(args *ToolCall) *ToolResponse {
	if t.mergeTask == nil {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskMergeTool.Execute() [task_merge.go]: mergeTask is nil"), Done: true}
	}

	identifier, fallback := resolveTaskIdentifier(args, t.session)
	taskData, err := t.mergeTask(context.Background(), identifier, fallback)
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	return &ToolResponse{Call: args, Result: NewToolValue(map[string]any{"task": taskRecordToToolValue(taskData).Raw()}), Done: true}
}

// Render returns human-readable representation.
func (t *TaskMergeTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	one := truncateString("taskMerge", 128)
	jsonl := buildToolRenderJSONL("taskMerge", call, map[string]any{})
	return one, one, jsonl, map[string]string{}
}
