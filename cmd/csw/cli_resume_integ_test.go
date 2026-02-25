package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLINormalizeResumeTarget(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{name: "empty", input: "", expected: ""},
		{name: "last", input: "last", expected: "last"},
		{name: "last uppercase", input: "LAST", expected: "last"},
		{name: "valid uuid", input: "018f6e30-3acb-7f24-bede-8d96cd157152", expected: "018f6e30-3acb-7f24-bede-8d96cd157152"},
		{name: "invalid", input: "abc", expectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := normalizeResumeTarget(tc.input)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestCLIResumeFlagsAndPromptRules(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectError     bool
		expectedError   string
		expectedResume  string
		expectedPrompt  string
		expectedCont    bool
		expectedForce   bool
	}{
		{
			name:          "continue defaults to last",
			args:          []string{"--continue", "next message"},
			expectedResume: "last",
			expectedPrompt: "next message",
			expectedCont:   true,
		},
		{
			name:          "resume no value defaults to last",
			args:          []string{"--resume", "--continue", "next message"},
			expectedResume: "last",
			expectedPrompt: "next message",
			expectedCont:   true,
		},
		{
			name:          "resume explicit uuid",
			args:          []string{"--resume=018f6e30-3acb-7f24-bede-8d96cd157152", "--continue", "next"},
			expectedResume: "018f6e30-3acb-7f24-bede-8d96cd157152",
			expectedPrompt: "next",
			expectedCont:   true,
		},
		{
			name:          "resume force without prompt",
			args:          []string{"--resume", "--force"},
			expectedResume: "last",
			expectedForce:  true,
		},
		{
			name:          "resume as is without force",
			args:          []string{"--resume"},
			expectedResume: "last",
		},
		{
			name:          "fresh session prompt",
			args:          []string{"hello"},
			expectedPrompt: "hello",
		},
		{
			name:          "fresh session empty prompt error",
			args:          []string{"   "},
			expectError:   true,
			expectedError: "prompt cannot be empty",
		},
		{
			name:          "continue without prompt error",
			args:          []string{"--continue"},
			expectError:   true,
			expectedError: "prompt cannot be empty when --continue is set",
		},
		{
			name:          "resume with prompt but no continue error",
			args:          []string{"--resume=last", "hello"},
			expectError:   true,
			expectedError: "prompt requires --continue when --resume is set",
		},
		{
			name:          "invalid resume value",
			args:          []string{"--resume=bad", "--continue", "hello"},
			expectError:   true,
			expectedError: "invalid --resume value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedCall string
			originalRun := runCLIFunc
			t.Cleanup(func() {
				runCLIFunc = originalRun
			})

			runCLIFunc = func(prompt, modelName, roleName, workDir, worktreeBranch string, merge bool, containerImage, commitMessageTemplate, configPath string, allowAllPerms, interactive bool, saveSessionTo string, saveSession, logLLMRequests bool, lspServer, thinking, resumeTarget string, continueSession, forceResume bool) error {
				capturedCall = fmt.Sprintf("prompt=%s,resume=%s,continue=%t,force=%t", prompt, resumeTarget, continueSession, forceResume)
				return nil
			}

			cmd := CliCommand()
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
			assert.Contains(t, capturedCall, "prompt="+tc.expectedPrompt)
			assert.Contains(t, capturedCall, "resume="+tc.expectedResume)
			assert.Contains(t, capturedCall, fmt.Sprintf("continue=%t", tc.expectedCont))
			assert.Contains(t, capturedCall, fmt.Sprintf("force=%t", tc.expectedForce))
		})
	}
}
