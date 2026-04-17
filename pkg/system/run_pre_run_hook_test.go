package system

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
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
				ContextData: map[string]string{"NAME": "FromContext"},
			},
			expectedPrompt: "Hello FromContext",
		},
		{
			name: "prompt still renders from context",
			params: &RunParams{
				Prompt:      "Hello {{.NAME}}",
				ContextData: map[string]string{"NAME": "FromCLI"},
			},
			expectedPrompt: "Hello FromCLI",
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
				ContextData: map[string]string{"NAME": "Ignored"},
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
