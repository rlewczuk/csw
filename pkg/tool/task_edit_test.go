package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskEditToolExecute(t *testing.T) {
	tests := []struct {
		name               string
		arguments          map[string]any
		sessionTaskID      string
		expectedIdentifier string
		expectedFallback   string
		expectedOldString  string
		expectedNewString  string
		expectedReplaceAll bool
		expectedError      string
	}{
		{
			name: "uses uuid identifier",
			arguments: map[string]any{
				"uuid":      "task-uuid",
				"oldString": "old",
				"newString": "new",
			},
			expectedIdentifier: "task-uuid",
			expectedFallback:   "task-uuid",
			expectedOldString:  "old",
			expectedNewString:  "new",
		},
		{
			name: "uses name identifier",
			arguments: map[string]any{
				"name":      "task-name",
				"oldString": "old",
				"newString": "new",
			},
			expectedIdentifier: "task-name",
			expectedFallback:   "task-name",
			expectedOldString:  "old",
			expectedNewString:  "new",
		},
		{
			name: "uses session task fallback",
			arguments: map[string]any{
				"oldString": "old",
				"newString": "new",
			},
			sessionTaskID:      "session-task-id",
			expectedIdentifier: "",
			expectedFallback:   "session-task-id",
			expectedOldString:  "old",
			expectedNewString:  "new",
		},
		{
			name: "passes replaceAll",
			arguments: map[string]any{
				"uuid":       "task-uuid",
				"oldString":  "old",
				"newString":  "new",
				"replaceAll": true,
			},
			expectedIdentifier: "task-uuid",
			expectedFallback:   "task-uuid",
			expectedOldString:  "old",
			expectedNewString:  "new",
			expectedReplaceAll: true,
		},
		{
			name: "fails when oldString missing",
			arguments: map[string]any{
				"uuid":      "task-uuid",
				"newString": "new",
			},
			expectedError: "missing required argument: oldString",
		},
		{
			name: "fails when newString missing",
			arguments: map[string]any{
				"uuid":      "task-uuid",
				"oldString": "old",
			},
			expectedError: "missing required argument: newString",
		},
		{
			name: "fails when identifier is missing",
			arguments: map[string]any{
				"oldString": "old",
				"newString": "new",
			},
			expectedError: "missing task identifier",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			session := &taskUpdateTestSession{taskID: tc.sessionTaskID}

			capturedIdentifier := ""
			capturedFallback := ""
			capturedOldString := ""
			capturedNewString := ""
			capturedReplaceAll := false

			editTool := NewTaskEditTool(func(ctx context.Context, identifier string, fallbackTaskID string, oldString string, newString string, replaceAll bool) (TaskRecord, error) {
				_ = ctx
				capturedIdentifier = identifier
				capturedFallback = fallbackTaskID
				capturedOldString = oldString
				capturedNewString = newString
				capturedReplaceAll = replaceAll
				return TaskRecord{UUID: "task-uuid", Name: "task-name"}, nil
			}, session)

			response := editTool.Execute(&ToolCall{ID: "call-1", Function: "taskEdit", Arguments: NewToolValue(tc.arguments)})

			if tc.expectedError != "" {
				require.Error(t, response.Error)
				assert.Contains(t, response.Error.Error(), tc.expectedError)
				return
			}

			require.NoError(t, response.Error)
			assert.Equal(t, tc.expectedIdentifier, capturedIdentifier)
			assert.Equal(t, tc.expectedFallback, capturedFallback)
			assert.Equal(t, tc.expectedOldString, capturedOldString)
			assert.Equal(t, tc.expectedNewString, capturedNewString)
			assert.Equal(t, tc.expectedReplaceAll, capturedReplaceAll)
		})
	}
}
