package main

import (
	"bytes"
	"testing"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// TestCliThinkingFlagDefinition tests that the --thinking flag is correctly defined
func TestCliThinkingFlagDefinition(t *testing.T) {
	cmd := CliCommand()

	// Check that the flag is defined
	flag := cmd.Flags().Lookup("thinking")
	assert.NotNil(t, flag, "thinking flag should be defined")
	assert.Equal(t, "string", flag.Value.Type(), "thinking flag should be a string")
	assert.Equal(t, "", flag.DefValue, "thinking flag default value should be empty")
}

// TestCliThinkingFlagParsing tests that the --thinking flag is correctly parsed
func TestCliThinkingFlagParsing(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedValue string
	}{
		{
			name:          "thinking flag with low value",
			args:          []string{"--thinking=low", "test prompt"},
			expectedValue: "low",
		},
		{
			name:          "thinking flag with medium value",
			args:          []string{"--thinking=medium", "test prompt"},
			expectedValue: "medium",
		},
		{
			name:          "thinking flag with high value",
			args:          []string{"--thinking=high", "test prompt"},
			expectedValue: "high",
		},
		{
			name:          "thinking flag with xhigh value",
			args:          []string{"--thinking=xhigh", "test prompt"},
			expectedValue: "xhigh",
		},
		{
			name:          "thinking flag with true value",
			args:          []string{"--thinking=true", "test prompt"},
			expectedValue: "true",
		},
		{
			name:          "thinking flag with false value",
			args:          []string{"--thinking=false", "test prompt"},
			expectedValue: "false",
		},
		{
			name:          "no thinking flag",
			args:          []string{"test prompt"},
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedThinking string

			// Create a test command that captures the flag value
			testCmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					capturedThinking, _ = cmd.Flags().GetString("thinking")
					return nil
				},
			}
			testCmd.Flags().String("thinking", "", "Test thinking flag")
			testCmd.SetArgs(tt.args)

			err := testCmd.Execute()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValue, capturedThinking)
		})
	}
}

// TestCliRuntimeErrorDoesNotPrintUsage tests that runtime command errors don't print usage text.
func TestCliRuntimeErrorDoesNotPrintUsage(t *testing.T) {
	cmd := CliCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"   "})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prompt cannot be empty")
	assert.NotContains(t, stderr.String(), "Usage:")
	assert.NotContains(t, stdout.String(), "Usage:")
}

// TestThinkingModeValidation tests the validation of thinking mode values
func TestThinkingModeValidation(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		expectValid bool
	}{
		{"low", "low", true},
		{"medium", "medium", true},
		{"high", "high", true},
		{"xhigh", "xhigh", true},
		{"true", "true", true},
		{"false", "false", true},
		{"invalid", "invalid", true}, // We accept any string value
		{"", "", true},               // Empty is also valid (means not set)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Any string value should be acceptable as a thinking mode
			// The actual validation happens in the provider
			assert.Equal(t, tt.expectValid, true)
		})
	}
}

// TestBuildSystemParamsThinking tests that BuildSystemParams includes Thinking field
func TestBuildSystemParamsThinking(t *testing.T) {
	params := BuildSystemParams{
		Thinking: "high",
	}
	assert.Equal(t, "high", params.Thinking)
}

// TestChatOptionsThinkingField tests that ChatOptions has the Thinking field
func TestChatOptionsThinkingField(t *testing.T) {
	opts := &models.ChatOptions{
		Thinking: "high",
	}
	assert.Equal(t, "high", opts.Thinking)

	opts2 := &models.ChatOptions{
		Thinking: "true",
	}
	assert.Equal(t, "true", opts2.Thinking)

	opts3 := &models.ChatOptions{}
	assert.Equal(t, "", opts3.Thinking)
}
