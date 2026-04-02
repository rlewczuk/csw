package tool

import (
	"fmt"
	"strings"
)

func resolveTaskIdentifier(args *ToolCall, session TaskSessionRef) (string, string) {
	if args == nil {
		return "", ""
	}

	uuid := strings.TrimSpace(args.Arguments.String("uuid"))
	name := strings.TrimSpace(args.Arguments.String("name"))
	if uuid != "" {
		return uuid, uuid
	}
	if name != "" {
		return name, name
	}

	fallbackTaskID := ""
	if session != nil {
		fallbackTaskID = strings.TrimSpace(session.TaskID())
	}

	return "", fallbackTaskID
}

func parseStringArrayArgument(args *ToolCall, key string) ([]string, error) {
	if args == nil {
		return nil, fmt.Errorf("parseStringArrayArgument() [task_helpers.go]: args is nil")
	}

	value, ok := args.Arguments.GetOK(key)
	if !ok {
		return nil, nil
	}

	items, ok := value.ArrayOK()
	if !ok {
		return nil, fmt.Errorf("parseStringArrayArgument() [task_helpers.go]: %s must be an array", key)
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, strings.TrimSpace(item.AsString()))
	}

	return result, nil
}

func taskRecordToToolValue(task TaskRecord) ToolValue {
	value := NewToolValue(map[string]any{})
	value.Set("uuid", task.UUID)
	value.Set("name", task.Name)
	value.Set("description", task.Description)
	value.Set("status", task.Status)
	value.Set("branch", task.FeatureBranch)
	value.Set("parent_branch", task.ParentBranch)
	value.Set("role", task.Role)
	value.Set("state", task.State)
	value.Set("deps", task.Deps)
	value.Set("session_ids", task.SessionIDs)
	value.Set("subtask_ids", task.SubtaskIDs)
	value.Set("parent_task_id", task.ParentTaskID)
	value.Set("created_at", task.CreatedAt)
	value.Set("updated_at", task.UpdatedAt)

	return value
}
