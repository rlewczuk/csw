package system

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIContextFlagPropagation(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		expectedPrompt    string
		expectedContext   map[string]string
		expectError       bool
		expectedErrorText string
	}{
		{
			name:           "long flag passes template prompt and context",
			args:           []string{"--context=NAME=World", "Hello {{.NAME}}"},
			expectedPrompt: "Hello {{.NAME}}",
			expectedContext: map[string]string{
				"NAME": "World",
			},
		},
		{
			name:           "short flag and repeated entries pass raw template",
			args:           []string{"-c", "PROJECT=csw", "-c", "TASK=tests", "Fix {{.TASK}} in {{.PROJECT}}"},
			expectedPrompt: "Fix {{.TASK}} in {{.PROJECT}}",
			expectedContext: map[string]string{
				"PROJECT": "csw",
				"TASK":    "tests",
			},
		},
		{
			name:              "invalid context format fails",
			args:              []string{"-c", "INVALID", "Hello"},
			expectError:       true,
			expectedErrorText: "expected KEY=VAL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalRun := runCommandFunc
			t.Cleanup(func() {
				runCommandFunc = originalRun
			})

			captured := ""
			runCommandFunc = func(params *RunParams) error {
				captured = fmt.Sprintf("prompt=%s,context=%v", params.Prompt, params.ContextData)
				return nil
			}

			cmd := RunCommand()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorText)
				return
			}

			require.NoError(t, err)
			assert.Contains(t, captured, "prompt="+tt.expectedPrompt)
			for key, value := range tt.expectedContext {
				assert.Contains(t, captured, fmt.Sprintf("%s:%s", key, value))
			}
		})
	}
}

func TestContextFlagInHelp(t *testing.T) {
	cmd := RunCommand()

	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "--context")
	assert.Contains(t, helpOutput, "-c")
	assert.Contains(t, helpOutput, "KEY=VAL")
}
