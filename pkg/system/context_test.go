package system

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRunContextEntries(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		expected    map[string]string
		expectError bool
		errorText   string
	}{
		{
			name:     "empty input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "single value",
			input: []string{"NAME=World"},
			expected: map[string]string{
				"NAME": "World",
			},
		},
		{
			name:  "multiple values",
			input: []string{"NAME=World", "TASK=tests"},
			expected: map[string]string{
				"NAME": "World",
				"TASK": "tests",
			},
		},
		{
			name:  "duplicate key uses latest value",
			input: []string{"NAME=World", "NAME=Team"},
			expected: map[string]string{
				"NAME": "Team",
			},
		},
		{
			name:  "value can contain equals",
			input: []string{"FILTER=a=b=c"},
			expected: map[string]string{
				"FILTER": "a=b=c",
			},
		},
		{
			name:        "empty entry rejected",
			input:       []string{""},
			expectError: true,
			errorText:   "context entry cannot be empty",
		},
		{
			name:        "missing equals rejected",
			input:       []string{"NAME"},
			expectError: true,
			errorText:   "expected KEY=VAL format",
		},
		{
			name:        "empty key rejected",
			input:       []string{"=value"},
			expectError: true,
			errorText:   "key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ParseRunContextEntries(tt.input)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestBuildPromptContextData(t *testing.T) {
	t.Run("includes agent state recursively with struct field names", func(t *testing.T) {
		contextData := BuildPromptContextData(map[string]string{"NAME": "FromCLI"}, core.AgentState{
			Info: core.AgentStateCommonInfo{WorkDir: "/workspace", ShadowDir: "/shadow"},
			Task: &core.Task{TaskDir: "/workspace/.cswdata/tasks/t1", FeatureBranch: "feature/x"},
		})

		require.NotNil(t, contextData)
		assert.Equal(t, "FromCLI", contextData["NAME"])

		taskValue, ok := contextData["Task"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "/workspace/.cswdata/tasks/t1", taskValue["TaskDir"])
		assert.Equal(t, "feature/x", taskValue["FeatureBranch"])
		_, hasJSONTagName := taskValue["feature_branch"]
		assert.False(t, hasJSONTagName)

		infoValue, ok := contextData["Info"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "/workspace", infoValue["WorkDir"])
		assert.NotEmpty(t, infoValue["CurrentTime"])
	})

	t.Run("cli context keys are overridden by agent state keys", func(t *testing.T) {
		contextData := BuildPromptContextData(map[string]any{"Task": "override"}, core.AgentState{
			Task: &core.Task{TaskDir: "/task-dir"},
		})

		taskValue, ok := contextData["Task"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "/task-dir", taskValue["TaskDir"])
	})
}
