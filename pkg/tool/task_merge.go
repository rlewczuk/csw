package tool

import (
	"context"
	"fmt"
)

// TaskMergeTool merges tasks.
type TaskMergeTool struct {
	backend TaskBackend
	session TaskSessionRef
}

// NewTaskMergeTool creates a new TaskMergeTool instance.
func NewTaskMergeTool(backend TaskBackend, session TaskSessionRef) *TaskMergeTool {
	return &TaskMergeTool{backend: backend, session: session}
}

// GetDescription returns additional dynamic description.
func (t *TaskMergeTool) GetDescription() (string, bool) { return "", false }

// Execute executes task merge.
func (t *TaskMergeTool) Execute(args *ToolCall) *ToolResponse {
	if t.backend == nil {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskMergeTool.Execute() [task_merge.go]: backend is nil"), Done: true}
	}

	identifier, fallback := resolveTaskIdentifier(args, t.session)
	taskData, err := t.backend.MergeTask(context.Background(), identifier, fallback)
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
