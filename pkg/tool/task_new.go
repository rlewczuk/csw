package tool

import (
	"context"
	"fmt"
	"strings"
)

// TaskCreateFunc creates a persistent task record.
type TaskCreateFunc func(ctx context.Context, params TaskRecord, prompt string, parentTaskID string) (TaskRecord, error)

// TaskNewTool creates persistent tasks.
type TaskNewTool struct {
	createTask TaskCreateFunc
	session    TaskSessionRef
}

// NewTaskNewTool creates a new TaskNewTool instance.
func NewTaskNewTool(createTask TaskCreateFunc, session TaskSessionRef) *TaskNewTool {
	return &TaskNewTool{createTask: createTask, session: session}
}

// GetDescription returns additional dynamic description.
func (t *TaskNewTool) GetDescription() (string, bool) {
	return "", false
}

// Execute executes task creation.
func (t *TaskNewTool) Execute(args *ToolCall) *ToolResponse {
	if t.createTask == nil {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskNewTool.Execute() [task_new.go]: createTask is nil"), Done: true}
	}

	prompt, ok := args.Arguments.StringOK("prompt")
	if !ok || strings.TrimSpace(prompt) == "" {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskNewTool.Execute() [task_new.go]: missing required argument: prompt"), Done: true}
	}

	deps, err := parseStringArrayArgument(args, "deps")
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	record, err := t.createTask(context.Background(), TaskRecord{
		Name:          strings.TrimSpace(args.Arguments.String("name")),
		Description:   strings.TrimSpace(args.Arguments.String("description")),
		FeatureBranch: strings.TrimSpace(args.Arguments.String("branch")),
		ParentBranch:  strings.TrimSpace(args.Arguments.String("parent-branch")),
		Role:          strings.TrimSpace(args.Arguments.String("role")),
		Deps:          deps,
	}, prompt, strings.TrimSpace(args.Arguments.String("parent")))
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	if t.session != nil {
		t.session.SetTaskID(strings.TrimSpace(record.UUID))
	}

	result := NewToolValue(map[string]any{"task": taskRecordToToolValue(record).Raw(), "uuid": record.UUID})

	return &ToolResponse{Call: args, Result: result, Done: true}
}

// Render returns human-readable representation.
func (t *TaskNewTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	one := truncateString("taskNew", 128)
	jsonl := buildToolRenderJSONL("taskNew", call, map[string]any{"uuid": call.Arguments.String("uuid")})
	return one, one, jsonl, map[string]string{}
}
