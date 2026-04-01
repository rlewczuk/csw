package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCLIContextEntries(t *testing.T) {
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
			actual, err := system.ParseCLIContextEntries(tt.input)
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
			actual, err := shared.RenderTextWithContext(tt.prompt, tt.context)
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
			originalRun := runCLIFunc
			t.Cleanup(func() {
				runCLIFunc = originalRun
			})

			captured := ""
			runCLIFunc = func(params *CLIParams) error {
				captured = fmt.Sprintf("prompt=%s,context=%v", params.Prompt, params.ContextData)
				return nil
			}

			cmd := CliCommand()
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
	cmd := CliCommand()

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
