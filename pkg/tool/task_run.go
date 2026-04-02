package tool

import (
	"context"
	"fmt"
)

// TaskRunTool runs task sessions.
type TaskRunTool struct {
	backend TaskBackend
	session TaskSessionRef
}

// NewTaskRunTool creates a new TaskRunTool instance.
func NewTaskRunTool(backend TaskBackend, session TaskSessionRef) *TaskRunTool {
	return &TaskRunTool{backend: backend, session: session}
}

// GetDescription returns additional dynamic description.
func (t *TaskRunTool) GetDescription() (string, bool) { return "", false }

// Execute executes task run.
func (t *TaskRunTool) Execute(args *ToolCall) *ToolResponse {
	if t.backend == nil {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskRunTool.Execute() [task_run.go]: backend is nil"), Done: true}
	}

	identifier, fallback := resolveTaskIdentifier(args, t.session)
	outcome, err := t.backend.RunTask(context.Background(), identifier, fallback, args.Arguments.Bool("merge"), args.Arguments.Bool("reset"))
	result := NewToolValue(map[string]any{
		"task":        taskRecordToToolValue(outcome.Task).Raw(),
		"session_id":  outcome.SessionID,
		"summary":     outcome.SummaryText,
		"merged":      outcome.Merged,
		"task_branch": outcome.TaskBranchName,
	})
	if outcome.SummaryMeta != nil {
		result.Set("summary_meta", NewToolValue(map[string]any{
			"session_id":   outcome.SummaryMeta.SessionID,
			"status":       outcome.SummaryMeta.Status,
			"started_at":   outcome.SummaryMeta.StartedAt,
			"completed_at": outcome.SummaryMeta.CompletedAt,
			"task_id":      outcome.SummaryMeta.TaskID,
		}).Raw())
	}
	if err != nil {
		return &ToolResponse{Call: args, Result: result, Error: err, Done: true}
	}

	return &ToolResponse{Call: args, Result: result, Done: true}
}

// Render returns human-readable representation.
func (t *TaskRunTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	one := truncateString("taskRun", 128)
	jsonl := buildToolRenderJSONL("taskRun", call, map[string]any{})
	return one, one, jsonl, map[string]string{}
}
