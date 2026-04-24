package tool

import (
	"context"
	"fmt"
	"strings"
)

// TaskUpdateFunc updates a persistent task record.
type TaskUpdateFunc func(ctx context.Context, identifier string, params TaskRecord, prompt *string) (TaskRecord, error)

// TaskUpdateTool updates persistent tasks.
type TaskUpdateTool struct {
	updateTask TaskUpdateFunc
	session    TaskSessionRef
}

// NewTaskUpdateTool creates a new TaskUpdateTool instance.
func NewTaskUpdateTool(updateTask TaskUpdateFunc, session TaskSessionRef) *TaskUpdateTool {
	return &TaskUpdateTool{updateTask: updateTask, session: session}
}

// GetDescription returns additional dynamic description.
func (t *TaskUpdateTool) GetDescription() (string, bool) { return "", false }

// Execute executes task update.
func (t *TaskUpdateTool) Execute(args *ToolCall) *ToolResponse {
	if t.updateTask == nil {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskUpdateTool.Execute() [task_update.go]: updateTask is nil"), Done: true}
	}

	identifier, fallback := resolveTaskIdentifier(args, t.session)
	if strings.TrimSpace(identifier) == "" && strings.TrimSpace(fallback) == "" {
		return &ToolResponse{Call: args, Error: fmt.Errorf("TaskUpdateTool.Execute() [task_update.go]: missing task identifier (name or uuid)"), Done: true}
	}

	deps, err := parseStringArrayArgument(args, "deps")
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	var prompt *string
	if value, ok := args.Arguments.StringOK("prompt"); ok {
		trimmed := strings.TrimSpace(value)
		prompt = &trimmed
	}

	updated, err := t.updateTask(context.Background(), firstNonEmptyTool(identifier, fallback), TaskRecord{
		Name:          strings.TrimSpace(args.Arguments.String("name")),
		Description:   strings.TrimSpace(args.Arguments.String("description")),
		Status:        strings.TrimSpace(args.Arguments.String("status")),
		StatusSet:     hasToolArgument(args, "status"),
		FeatureBranch: strings.TrimSpace(args.Arguments.String("branch")),
		ParentBranch:  strings.TrimSpace(args.Arguments.String("parent-branch")),
		Role:          strings.TrimSpace(args.Arguments.String("role")),
		Deps:          deps,
	}, prompt)
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	result := NewToolValue(map[string]any{"task": taskRecordToToolValue(updated).Raw(), "uuid": updated.UUID})

	return &ToolResponse{Call: args, Result: result, Done: true}
}

// Render returns human-readable representation.
func (t *TaskUpdateTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	one := truncateString("taskUpdate", 128)
	jsonl := buildToolRenderJSONL("taskUpdate", call, map[string]any{})
	return one, one, jsonl, map[string]string{}
}

func firstNonEmptyTool(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func hasToolArgument(args *ToolCall, key string) bool {
	if args == nil {
		return false
	}

	_, ok := args.Arguments.GetOK(key)

	return ok
}
