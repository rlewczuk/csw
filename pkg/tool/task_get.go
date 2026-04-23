package tool

import (
	"context"
	"fmt"
	"strings"
)

// TaskGetFunc gets task details and optional summary.
type TaskGetFunc func(ctx context.Context, identifier string, fallbackTaskID string, includeSummary bool) (TaskRecord, *TaskSessionSummary, string, error)

// TaskGetTool retrieves task details.
type TaskGetTool struct {
	getTask TaskGetFunc
	session TaskSessionRef
}

// NewTaskGetTool creates a new TaskGetTool instance.
func NewTaskGetTool(getTask TaskGetFunc, session TaskSessionRef) *TaskGetTool {
	return &TaskGetTool{getTask: getTask, session: session}
}

// GetDescription returns additional dynamic description.
func (t *TaskGetTool) GetDescription() (string, bool) { return "", false }

// Execute executes task get.
func (t *TaskGetTool) Execute(args *ToolCall) *ToolResponse {
	if t.getTask == nil {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskGetTool.Execute() [task_get.go]: getTask is nil"), Done: true}
	}

	identifier, fallback := resolveTaskIdentifier(args, t.session)
	taskData, summaryMeta, summaryText, err := t.getTask(context.Background(), identifier, fallback, args.Arguments.Bool("summary"))
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	result := NewToolValue(map[string]any{"task": taskRecordToToolValue(taskData).Raw()})
	if summaryMeta != nil {
		result.Set("summary_meta", NewToolValue(map[string]any{"session_id": summaryMeta.SessionID, "status": summaryMeta.Status, "started_at": summaryMeta.StartedAt, "completed_at": summaryMeta.CompletedAt, "task_id": summaryMeta.TaskID}).Raw())
	}
	if strings.TrimSpace(summaryText) != "" {
		result.Set("summary", strings.TrimSpace(summaryText))
	}

	return &ToolResponse{Call: args, Result: result, Done: true}
}

// Render returns human-readable representation.
func (t *TaskGetTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	one := truncateString("taskGet", 128)
	jsonl := buildToolRenderJSONL("taskGet", call, map[string]any{})
	return one, one, jsonl, map[string]string{}
}
