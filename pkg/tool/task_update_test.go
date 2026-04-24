package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type taskUpdateTestSession struct {
	taskID                   string
	taskStatusUpdatedSession bool
}

func (s *taskUpdateTestSession) TaskID() string { return s.taskID }

func (s *taskUpdateTestSession) SetTaskID(taskID string) { s.taskID = taskID }

func (s *taskUpdateTestSession) TaskStatusUpdatedInSession() bool {
	return s.taskStatusUpdatedSession
}

func (s *taskUpdateTestSession) SetTaskStatusUpdatedInSession(updated bool) {
	s.taskStatusUpdatedSession = updated
}

func TestTaskUpdateToolExecuteTracksStatusFieldPresence(t *testing.T) {
	tests := []struct {
		name              string
		arguments         map[string]any
		expectedStatus    string
		expectedStatusSet bool
	}{
		{
			name: "status omitted keeps StatusSet false",
			arguments: map[string]any{
				"uuid": "task-1",
			},
			expectedStatus:    "",
			expectedStatusSet: false,
		},
		{
			name: "status provided sets StatusSet true",
			arguments: map[string]any{
				"uuid":   "task-1",
				"status": "open",
			},
			expectedStatus:    "open",
			expectedStatusSet: true,
		},
		{
			name: "empty status still marks StatusSet true",
			arguments: map[string]any{
				"uuid":   "task-1",
				"status": "",
			},
			expectedStatus:    "",
			expectedStatusSet: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			captured := TaskRecord{}
			updateTool := NewTaskUpdateTool(func(ctx context.Context, identifier string, params TaskRecord, prompt *string) (TaskRecord, error) {
				_ = ctx
				_ = prompt
				captured = params
				params.UUID = identifier
				return params, nil
			}, &taskUpdateTestSession{})

			response := updateTool.Execute(&ToolCall{ID: "call-1", Function: "taskUpdate", Arguments: NewToolValue(tc.arguments)})
			require.NoError(t, response.Error)
			assert.Equal(t, tc.expectedStatus, captured.Status)
			assert.Equal(t, tc.expectedStatusSet, captured.StatusSet)
		})
	}
}
