package main

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBashRunTimeout(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    time.Duration
		expectError bool
	}{
		{name: "empty uses default", input: "", expected: 120 * time.Second},
		{name: "seconds without unit", input: "45", expected: 45 * time.Second},
		{name: "duration with unit", input: "1500ms", expected: 1500 * time.Millisecond},
		{name: "duration with minutes", input: "2m", expected: 2 * time.Minute},
		{name: "zero rejected", input: "0", expectError: true},
		{name: "negative rejected", input: "-5", expectError: true},
		{name: "invalid rejected", input: "abc", expectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := parseBashRunTimeout(tc.input)
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestCliBashRunTimeoutFlagPropagation(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedTimeout time.Duration
		expectError     bool
		expectedError   string
	}{
		{
			name:            "default timeout",
			args:            []string{"prompt"},
			expectedTimeout: 120 * time.Second,
		},
		{
			name:            "numeric seconds timeout",
			args:            []string{"--bash-run-timeout=45", "prompt"},
			expectedTimeout: 45 * time.Second,
		},
		{
			name:            "duration timeout",
			args:            []string{"--bash-run-timeout=1500ms", "prompt"},
			expectedTimeout: 1500 * time.Millisecond,
		},
		{
			name:          "invalid timeout value",
			args:          []string{"--bash-run-timeout=bad", "prompt"},
			expectError:   true,
			expectedError: "invalid --bash-run-timeout value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			originalRun := runFunc
			t.Cleanup(func() {
				runFunc = originalRun
			})

			captured := ""
			runFunc = func(params *RunParams) error {
				captured = fmt.Sprintf("timeout=%s", params.BashRunTimeout)
				return nil
			}

			cmd := RunCommand()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				return
			}

			require.NoError(t, err)
			assert.Contains(t, captured, fmt.Sprintf("timeout=%s", tc.expectedTimeout))
		})
	}
}
