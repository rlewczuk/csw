package system

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreparePromptWithContext(t *testing.T) {
	tests := []struct {
		name           string
		params         *RunParams
		expectedPrompt string
		expectError    string
	}{
		{
			name: "template rendering uses context data",
			params: &RunParams{
				Prompt:      "Hello {{.NAME}}",
				ContextData: map[string]any{"NAME": "FromContext"},
			},
			expectedPrompt: "Hello FromContext",
		},
		{
			name: "prompt still renders from context",
			params: &RunParams{
				Prompt:      "Hello {{.NAME}}",
				ContextData: map[string]any{"NAME": "FromCLI"},
			},
			expectedPrompt: "Hello FromCLI",
		},
		{
			name: "template renders nested agent state fields",
			params: &RunParams{
				Prompt: "Task file: {{.Task.TaskDir}}; WorkDir: {{.Info.WorkDir}}",
				ContextData: BuildPromptContextData(nil, core.AgentState{
					Info: core.AgentStateCommonInfo{WorkDir: "/workspace/project"},
					Task: &core.Task{TaskDir: "/workspace/project/.cswdata/tasks/task-1"},
				}),
			},
			expectedPrompt: "Task file: /workspace/project/.cswdata/tasks/task-1; WorkDir: /workspace/project",
		},
		{
			name: "template uses struct field names instead of json tags",
			params: &RunParams{
				Prompt: "{{.Task.TaskDir}} / {{.Task.FeatureBranch}}",
				ContextData: BuildPromptContextData(nil, core.AgentState{
					Task: &core.Task{TaskDir: "/tmp/task", FeatureBranch: "feature/test"},
				}),
			},
			expectedPrompt: "/tmp/task / feature/test",
		},
		{
			name:        "nil params returns error",
			params:      nil,
			expectError: "params is nil",
		},
		{
			name: "empty prompt is allowed",
			params: &RunParams{
				Prompt:      "",
				ContextData: map[string]any{"NAME": "Ignored"},
			},
			expectedPrompt: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := PreparePromptWithContext(tc.params)
			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedPrompt, tc.params.Prompt)
			}
		})
	}
}
