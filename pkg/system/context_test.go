package system

import (
	"testing"

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
