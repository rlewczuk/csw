package tool

import (
	"context"
	"fmt"
	"strconv"
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
	target := taskRenderTarget(call)
	includeSummary := false
	if call != nil {
		includeSummary = call.Arguments.Bool("summary")
		if !includeSummary {
			if _, ok := call.Arguments.Get("summary_meta").ObjectOK(); ok {
				includeSummary = true
			} else {
				includeSummary = strings.TrimSpace(call.Arguments.String("summary")) != ""
			}
		}
	}

	summary := truncateString("taskGet "+target, 128)
	if includeSummary {
		summary = truncateString(summary+" +summary", 128)
	}

	details := summary
	jsonlExtra := map[string]any{"target": target, "summary": includeSummary}
	if taskStatus := taskRenderStatus(call); taskStatus != "" {
		details += "\nstatus=" + taskStatus
		jsonlExtra["task_status"] = taskStatus
	}
	if includeSummary {
		summaryText := strings.TrimSpace(call.Arguments.String("summary"))
		if summaryText != "" {
			details += "\nsummary_chars=" + strconv.Itoa(len(summaryText))
			jsonlExtra["summary_chars"] = len(summaryText)
		}
	}

	jsonl := buildToolRenderJSONL("taskGet", call, jsonlExtra)
	return summary, details, jsonl, map[string]string{}
}
