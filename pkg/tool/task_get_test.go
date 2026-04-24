package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskGetToolExecutePromptOnly(t *testing.T) {
	tests := []struct {
		name              string
		arguments         map[string]any
		sessionTaskID     string
		expectedIdentifier string
		expectedFallback  string
		expectedPromptOnly bool
	}{
		{
			name: "promptOnly boolean true",
			arguments: map[string]any{
				"uuid":       "task-uuid",
				"promptOnly": true,
			},
			expectedIdentifier: "task-uuid",
			expectedFallback:   "task-uuid",
			expectedPromptOnly: true,
		},
		{
			name: "promptOnly string yes",
			arguments: map[string]any{
				"name":       "task-name",
				"promptOnly": "yes",
			},
			expectedIdentifier: "task-name",
			expectedFallback:   "task-name",
			expectedPromptOnly: true,
		},
		{
			name: "promptOnly string true with session fallback",
			arguments: map[string]any{
				"promptOnly": "true",
			},
			sessionTaskID:      "session-task-id",
			expectedIdentifier: "",
			expectedFallback:   "session-task-id",
			expectedPromptOnly: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			session := &taskUpdateTestSession{taskID: tc.sessionTaskID}
			capturedIdentifier := ""
			capturedFallback := ""
			capturedPromptOnly := false

			getTool := NewTaskGetTool(func(ctx context.Context, identifier string, fallbackTaskID string, includeSummary bool, promptOnly bool) (TaskRecord, *TaskSessionSummary, string, string, error) {
				_ = ctx
				capturedIdentifier = identifier
				capturedFallback = fallbackTaskID
				capturedPromptOnly = promptOnly
				assert.False(t, includeSummary)
				return TaskRecord{UUID: "task-uuid"}, nil, "", "first line\nsecond line\n", nil
			}, session)

			response := getTool.Execute(&ToolCall{ID: "call-1", Function: "taskGet", Arguments: NewToolValue(tc.arguments)})

			require.NoError(t, response.Error)
			assert.Equal(t, tc.expectedIdentifier, capturedIdentifier)
			assert.Equal(t, tc.expectedFallback, capturedFallback)
			assert.Equal(t, tc.expectedPromptOnly, capturedPromptOnly)
			assert.Equal(t, "    1  first line\n    2  second line\n", response.Result.Get("content").AsString())
		})
	}
}

func TestTaskGetToolExecuteDefaultMode(t *testing.T) {
	getTool := NewTaskGetTool(func(ctx context.Context, identifier string, fallbackTaskID string, includeSummary bool, promptOnly bool) (TaskRecord, *TaskSessionSummary, string, string, error) {
		_ = ctx
		assert.Equal(t, "task-uuid", identifier)
		assert.Equal(t, "task-uuid", fallbackTaskID)
		assert.True(t, includeSummary)
		assert.False(t, promptOnly)
		return TaskRecord{UUID: "task-uuid", Name: "task-name"}, &TaskSessionSummary{SessionID: "ses-1", Status: "completed"}, " summary text ", "", nil
	}, nil)

	response := getTool.Execute(&ToolCall{ID: "call-1", Function: "taskGet", Arguments: NewToolValue(map[string]any{"uuid": "task-uuid", "summary": true})})

	require.NoError(t, response.Error)
	assert.Equal(t, "task-name", response.Result.Get("task").Get("name").AsString())
	assert.Equal(t, "ses-1", response.Result.Get("summary_meta").Get("session_id").AsString())
	assert.Equal(t, "summary text", response.Result.Get("summary").AsString())
}
