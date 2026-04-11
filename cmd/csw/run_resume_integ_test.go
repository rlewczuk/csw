package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIResumeFlagsAndPromptRules(t *testing.T) {
	tests := []struct {
		name                     string
		args                     []string
		expectError              bool
		expectedError            string
		expectedResume           string
		expectedPrompt           string
		expectedCont             bool
		expectedForce            bool
		expectedForceCompact     bool
		expectedModelOverride    bool
		expectedRoleOverride     bool
		expectedThinkingOverride bool
		expectedThinking         string
		expectedWorktree         string
		expectContinueWt         bool
	}{
		{
			name:           "resume no value defaults to last",
			args:           []string{"--resume"},
			expectedResume: "last",
		},
		{
			name:           "resume with prompt continues session",
			args:           []string{"--resume", "next message"},
			expectedResume: "last",
			expectedPrompt: "next message",
			expectedCont:   true,
		},
		{
			name:           "resume explicit branch value",
			args:           []string{"--resume=feature/existing", "next"},
			expectedResume: "feature/existing",
			expectedPrompt: "next",
			expectedCont:   true,
		},
		{
			name:           "resume positional session id without prompt",
			args:           []string{"--resume", "018f6e30-3acb-7f24-bede-8d96cd157152"},
			expectedResume: "018f6e30-3acb-7f24-bede-8d96cd157152",
		},
		{
			name:           "resume positional session id with prompt",
			args:           []string{"--resume", "018f6e30-3acb-7f24-bede-8d96cd157152", "Please continue."},
			expectedResume: "018f6e30-3acb-7f24-bede-8d96cd157152",
			expectedPrompt: "Please continue.",
			expectedCont:   true,
		},
		{
			name:                 "resume force without prompt",
			args:                 []string{"--resume", "--force", "--force-compact"},
			expectedResume:       "last",
			expectedForce:        true,
			expectedForceCompact: true,
		},
		{
			name:                     "resume overrides model role and thinking",
			args:                     []string{"--resume=last", "--model=ollama/custom", "--role=developer", "--thinking=high", "next"},
			expectedResume:           "last",
			expectedPrompt:           "next",
			expectedCont:             true,
			expectedModelOverride:    true,
			expectedRoleOverride:     true,
			expectedThinkingOverride: true,
			expectedThinking:         "high",
		},
		{
			name:           "resume as is without force",
			args:           []string{"--resume"},
			expectedResume: "last",
		},
		{
			name:           "fresh session prompt",
			args:           []string{"hello"},
			expectedPrompt: "hello",
		},
		{
			name:          "fresh session empty prompt error",
			args:          []string{"   "},
			expectError:   true,
			expectedError: "prompt cannot be empty",
		},
		{
			name:             "continue worktree branch",
			args:             []string{"--continue", "feature/existing", "hello"},
			expectedPrompt:   "hello",
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
			name:          "resume cannot be used with worktree",
			args:          []string{"--resume=last", "--worktree", "feature/new", "hello"},
			expectError:   true,
			expectedError: "--worktree cannot be used with --resume",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedCall string
			originalRun := runFunc
			originalResolveDefaults := resolveRunDefaultsFunc
			t.Cleanup(func() {
				runFunc = originalRun
				resolveRunDefaultsFunc = originalResolveDefaults
			})
			resolveRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
				return conf.RunDefaultsConfig{}, nil
			}

			runFunc = func(params *RunParams) error {
				capturedCall = fmt.Sprintf("prompt=%s,resume=%s,continue=%t,force=%t,forcecompact=%t,modeloverride=%t,roleoverride=%t,thinkingoverride=%t,thinking=%s,worktree=%s,continuewt=%t", params.Prompt, params.ResumeTarget, params.ContinueSession, params.ForceResume, params.ForceCompact, params.ModelOverridden, params.RoleOverridden, params.ThinkingOverridden, params.Thinking, params.WorktreeBranch, params.ContinueWorktree)
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
			assert.Contains(t, capturedCall, "prompt="+tc.expectedPrompt)
			assert.Contains(t, capturedCall, "resume="+tc.expectedResume)
			assert.Contains(t, capturedCall, fmt.Sprintf("continue=%t", tc.expectedCont))
			assert.Contains(t, capturedCall, fmt.Sprintf("force=%t", tc.expectedForce))
			assert.Contains(t, capturedCall, fmt.Sprintf("forcecompact=%t", tc.expectedForceCompact))
			assert.Contains(t, capturedCall, fmt.Sprintf("modeloverride=%t", tc.expectedModelOverride))
			assert.Contains(t, capturedCall, fmt.Sprintf("roleoverride=%t", tc.expectedRoleOverride))
			assert.Contains(t, capturedCall, fmt.Sprintf("thinkingoverride=%t", tc.expectedThinkingOverride))
			assert.Contains(t, capturedCall, "thinking="+tc.expectedThinking)
			assert.Contains(t, capturedCall, "worktree="+tc.expectedWorktree)
			assert.Contains(t, capturedCall, fmt.Sprintf("continuewt=%t", tc.expectContinueWt))
		})
	}
}
