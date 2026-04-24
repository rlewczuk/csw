package tool

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskToolRender(t *testing.T) {
	tests := []struct {
		name           string
		tool           Tool
		call           *ToolCall
		wantSummary    []string
		wantDetails    []string
		wantJSONLField map[string]any
	}{
		{
			name: "taskGet includes target status summary and prompt marker",
			tool: NewTaskGetTool(nil, nil),
			call: &ToolCall{Function: "taskGet", Arguments: NewToolValue(map[string]any{
				"uuid":    "task-123",
				"summary": "line one\nline two",
				"promptOnly": "yes",
				"content": "    1  line one\n",
				"task": map[string]any{
					"status": "open",
				},
			})},
			wantSummary: []string{"taskGet", "uuid=task-123", "+summary", "+prompt"},
			wantDetails: []string{"status=open", "summary_chars=", "content_chars="},
			wantJSONLField: map[string]any{
				"tool":        "taskGet",
				"target":      "uuid=task-123",
				"summary":     true,
				"prompt_only": true,
				"task_status": "open",
			},
		},
		{
			name: "taskList includes target recursion and result count",
			tool: NewTaskListTool(nil, nil),
			call: &ToolCall{Function: "taskList", Arguments: NewToolValue(map[string]any{
				"name":      "parent-task",
				"recursive": true,
				"tasks": []any{
					map[string]any{"uuid": "a"},
					map[string]any{"uuid": "b"},
				},
			})},
			wantSummary: []string{"taskList", "name=parent-task", "recursive", "(2 results)"},
			wantDetails: []string{"returned=2"},
			wantJSONLField: map[string]any{
				"tool":      "taskList",
				"target":    "name=parent-task",
				"recursive": true,
				"count":     float64(2),
			},
		},
		{
			name: "taskMerge includes target and resulting status",
			tool: NewTaskMergeTool(nil, nil),
			call: &ToolCall{Function: "taskMerge", Arguments: NewToolValue(map[string]any{
				"name": "task-merge-target",
				"task": map[string]any{
					"status": "merged",
				},
			})},
			wantSummary: []string{"taskMerge", "name=task-merge-target"},
			wantDetails: []string{"status=merged"},
			wantJSONLField: map[string]any{
				"tool":        "taskMerge",
				"target":      "name=task-merge-target",
				"task_status": "merged",
			},
		},
		{
			name: "taskNew includes target parent deps and status",
			tool: NewTaskNewTool(nil, nil),
			call: &ToolCall{Function: "taskNew", Arguments: NewToolValue(map[string]any{
				"name":   "new-task-name",
				"parent": "parent-task-id",
				"deps":   []any{"dep-1", "dep-2"},
				"task": map[string]any{
					"status": "created",
				},
			})},
			wantSummary: []string{"taskNew", "name=new-task-name"},
			wantDetails: []string{"parent=parent-task-id", "deps=2", "status=created"},
			wantJSONLField: map[string]any{
				"tool":        "taskNew",
				"target":      "name=new-task-name",
				"parent":      "parent-task-id",
				"deps_count":  float64(2),
				"task_status": "created",
			},
		},
		{
			name: "taskEdit includes target and replaceAll flag",
			tool: NewTaskEditTool(nil, nil),
			call: &ToolCall{Function: "taskEdit", Arguments: NewToolValue(map[string]any{
				"name":       "task-edit-target",
				"oldString":  "before",
				"newString":  "after",
				"replaceAll": true,
			})},
			wantSummary: []string{"taskEdit", "name=task-edit-target"},
			wantDetails: []string{"replace_all=true"},
			wantJSONLField: map[string]any{
				"tool":        "taskEdit",
				"target":      "name=task-edit-target",
				"replace_all": true,
			},
		},
		{
			name: "taskUpdate includes target changed fields and status",
			tool: NewTaskUpdateTool(nil, nil),
			call: &ToolCall{Function: "taskUpdate", Arguments: NewToolValue(map[string]any{
				"uuid":   "task-update-id",
				"status": "running",
				"role":   "developer",
				"prompt": "updated prompt",
			})},
			wantSummary: []string{"taskUpdate", "uuid=task-update-id", "fields=status,role,prompt"},
			wantDetails: []string{"status=running", "updated_count=3"},
			wantJSONLField: map[string]any{
				"tool":          "taskUpdate",
				"target":        "uuid=task-update-id",
				"updated_count": float64(3),
				"task_status":   "running",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summary, details, jsonl, meta := tc.tool.Render(tc.call)

			assert.NotEmpty(t, summary)
			assert.NotEmpty(t, details)
			assert.NotEmpty(t, jsonl)
			assert.NotNil(t, meta)

			for _, want := range tc.wantSummary {
				assert.Contains(t, summary, want)
			}
			for _, want := range tc.wantDetails {
				assert.Contains(t, details, want)
			}

			parsedJSONL := map[string]any{}
			require.NoError(t, json.Unmarshal([]byte(jsonl), &parsedJSONL))
			for field, expected := range tc.wantJSONLField {
				assert.Equal(t, expected, parsedJSONL[field])
			}
		})
	}
}
