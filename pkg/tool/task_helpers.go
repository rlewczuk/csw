package tool

import (
	"fmt"
	"strings"
)

var taskUpdateRenderFields = []struct {
	argument string
	label    string
}{
	{argument: "name", label: "name"},
	{argument: "description", label: "description"},
	{argument: "status", label: "status"},
	{argument: "branch", label: "branch"},
	{argument: "parent-branch", label: "parent-branch"},
	{argument: "role", label: "role"},
	{argument: "deps", label: "deps"},
	{argument: "prompt", label: "prompt"},
}

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
	value.Set("deps", task.Deps)
	value.Set("session_ids", task.SessionIDs)
	value.Set("subtask_ids", task.SubtaskIDs)
	value.Set("parent_task_id", task.ParentTaskID)
	value.Set("created_at", task.CreatedAt)
	value.Set("updated_at", task.UpdatedAt)

	return value
}

func taskRenderTarget(call *ToolCall) string {
	if call == nil {
		return "current-session-task"
	}

	if uuid := strings.TrimSpace(call.Arguments.String("uuid")); uuid != "" {
		return "uuid=" + uuid
	}

	if name := strings.TrimSpace(call.Arguments.String("name")); name != "" {
		return "name=" + name
	}

	taskObject, ok := call.Arguments.Get("task").ObjectOK()
	if ok {
		if uuidValue, exists := taskObject["uuid"]; exists {
			uuid := strings.TrimSpace(uuidValue.AsString())
			if uuid != "" {
				return "uuid=" + uuid
			}
		}

		if nameValue, exists := taskObject["name"]; exists {
			name := strings.TrimSpace(nameValue.AsString())
			if name != "" {
				return "name=" + name
			}
		}
	}

	return "current-session-task"
}

func taskRenderStatus(call *ToolCall) string {
	if call == nil {
		return ""
	}

	if status := strings.TrimSpace(call.Arguments.String("status")); status != "" {
		return status
	}

	taskObject, ok := call.Arguments.Get("task").ObjectOK()
	if !ok {
		return ""
	}

	statusValue, exists := taskObject["status"]
	if !exists {
		return ""
	}

	return strings.TrimSpace(statusValue.AsString())
}

func taskRenderCount(call *ToolCall, key string) int {
	if call == nil {
		return 0
	}

	items, ok := call.Arguments.Get(key).ArrayOK()
	if !ok {
		return 0
	}

	return len(items)
}

func taskRenderUpdatedFields(call *ToolCall) []string {
	if call == nil {
		return nil
	}

	updatedFields := make([]string, 0, len(taskUpdateRenderFields))
	for _, candidate := range taskUpdateRenderFields {
		if _, ok := call.Arguments.GetOK(candidate.argument); ok {
			updatedFields = append(updatedFields, candidate.label)
		}
	}

	return updatedFields
}
