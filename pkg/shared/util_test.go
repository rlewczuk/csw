package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderPromptWithContext(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		context     map[string]string
		expected    string
		expectError bool
		errorText   string
	}{
		{
			name:     "template renders single value",
			prompt:   "Hello {{.NAME}}",
			context:  map[string]string{"NAME": "World"},
			expected: "Hello World",
		},
		{
			name:     "template renders multiple values",
			prompt:   "Fix {{.TASK}} in {{.PROJECT}}",
			context:  map[string]string{"TASK": "tests", "PROJECT": "csw"},
			expected: "Fix tests in csw",
		},
		{
			name:        "missing key returns error",
			prompt:      "Hello {{.MISSING}}",
			context:     map[string]string{"NAME": "World"},
			expectError: true,
			errorText:   "failed to render prompt template",
		},
		{
			name:        "invalid template returns error",
			prompt:      "Hello {{.NAME",
			context:     map[string]string{"NAME": "World"},
			expectError: true,
			errorText:   "failed to parse prompt template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := RenderTextWithContext(tt.prompt, tt.context)
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
