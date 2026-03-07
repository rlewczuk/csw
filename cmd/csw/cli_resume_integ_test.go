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
		expectedWorktree string
		expectContinueWt bool
	}{
		{
			name:          "resume continue defaults to last",
			args:          []string{"--resume-continue", "next message"},
			expectedResume: "last",
			expectedPrompt: "next message",
			expectedCont:   true,
		},
		{
			name:          "resume no value defaults to last",
			args:          []string{"--resume", "--resume-continue", "next message"},
			expectedResume: "last",
			expectedPrompt: "next message",
			expectedCont:   true,
		},
		{
			name:          "resume explicit uuid",
			args:          []string{"--resume=018f6e30-3acb-7f24-bede-8d96cd157152", "--resume-continue", "next"},
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
			name:          "resume continue without prompt error",
			args:          []string{"--resume-continue"},
			expectError:   true,
			expectedError: "prompt cannot be empty when --resume-continue is set",
		},
		{
			name:          "resume with prompt but no continue error",
			args:          []string{"--resume=last", "hello"},
			expectError:   true,
			expectedError: "prompt requires --resume-continue when --resume is set",
		},
		{
			name:          "invalid resume value",
			args:          []string{"--resume=bad", "--resume-continue", "hello"},
			expectError:   true,
			expectedError: "invalid --resume value",
		},
		{
			name:          "continue worktree branch",
			args:          []string{"--continue", "feature/existing", "hello"},
			expectedPrompt: "hello",
			expectedWorktree: "feature/existing",
			expectContinueWt: true,
		},
		{
			name:          "continue branch cannot be used with worktree",
			args:          []string{"--continue", "feature/existing", "--worktree", "feature/new", "hello"},
			expectError:   true,
			expectedError: "--continue and --worktree cannot be used together",
		},
		{
			name:          "continue branch cannot be used with resume",
			args:          []string{"--continue=feature/existing", "--resume=last", "hello"},
			expectError:   true,
			expectedError: "--continue <branch> cannot be used with --resume",
		},
		{
			name:          "continue branch cannot be used with resume continue",
			args:          []string{"--continue=feature/existing", "--resume-continue", "hello"},
			expectError:   true,
			expectedError: "--continue <branch> cannot be used with --resume-continue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedCall string
			originalRun := runCLIFunc
			t.Cleanup(func() {
				runCLIFunc = originalRun
			})

			runCLIFunc = func(params *CLIParams) error {
				capturedCall = fmt.Sprintf("prompt=%s,resume=%s,continue=%t,force=%t,worktree=%s,continuewt=%t", params.Prompt, params.ResumeTarget, params.ContinueSession, params.ForceResume, params.WorktreeBranch, params.ContinueWorktree)
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
			assert.Contains(t, capturedCall, "worktree="+tc.expectedWorktree)
			assert.Contains(t, capturedCall, fmt.Sprintf("continuewt=%t", tc.expectContinueWt))
		})
	}
}
