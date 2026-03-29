package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/system"
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
		{name: "valid uuid", input: "018F6E30-3ACB-7F24-BEDE-8D96CD157152", expected: "018f6e30-3acb-7f24-bede-8d96cd157152"},
		{name: "branch name", input: "feature/existing", expected: "feature/existing"},
		{name: "workdir name", input: "0145-resume-by-branch-dir", expected: "0145-resume-by-branch-dir"},
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
			name:                 "resume force without prompt",
			args:                 []string{"--resume", "--force", "--force-compact"},
			expectedResume:       "last",
			expectedForce:        true,
			expectedForceCompact: true,
		},
		{
			name:                     "resume overrides model role and thinking mode",
			args:                     []string{"--resume=last", "--model=ollama/custom", "--role=developer", "--thinking-mode=high", "next"},
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
			originalRun := runCLIFunc
			originalResolveDefaults := resolveCLIDefaultsFunc
			t.Cleanup(func() {
				runCLIFunc = originalRun
				resolveCLIDefaultsFunc = originalResolveDefaults
			})
			resolveCLIDefaultsFunc = func(params system.ResolveCLIDefaultsParams) (conf.CLIDefaultsConfig, error) {
				return conf.CLIDefaultsConfig{}, nil
			}

			runCLIFunc = func(params *CLIParams) error {
				capturedCall = fmt.Sprintf("prompt=%s,resume=%s,continue=%t,force=%t,forcecompact=%t,modeloverride=%t,roleoverride=%t,thinkingoverride=%t,thinking=%s,worktree=%s,continuewt=%t", params.Prompt, params.ResumeTarget, params.ContinueSession, params.ForceResume, params.ForceCompact, params.ModelOverridden, params.RoleOverridden, params.ThinkingOverridden, params.Thinking, params.WorktreeBranch, params.ContinueWorktree)
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

func TestCLIResolveResumeTargetToSessionID(t *testing.T) {
	tmpRoot := filepath.Join("..", "..", "tmp", "cli_resume", t.Name())
	require.NoError(t, os.MkdirAll(tmpRoot, 0755))
	defer os.RemoveAll(tmpRoot)
	absTmpRoot, err := filepath.Abs(tmpRoot)
	require.NoError(t, err)

	logsDir := filepath.Join(absTmpRoot, ".cswdata", "logs")
	sessionsDir := filepath.Join(logsDir, "sessions")
	require.NoError(t, os.MkdirAll(sessionsDir, 0755))

	repoDir := filepath.Join(absTmpRoot, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	worktreeDir := filepath.Join(absTmpRoot, ".cswdata", "work", "0145-feature-work")
	require.NoError(t, os.MkdirAll(worktreeDir, 0755))

	directWorkDir := filepath.Join(repoDir)

	writeState := func(id string, workdir string) {
		sessionPath := filepath.Join(sessionsDir, id)
		require.NoError(t, os.MkdirAll(sessionPath, 0755))
		state := core.PersistedSessionState{
			SessionID:    id,
			ProviderName: "ollama",
			Model:        "test-model",
			WorkDir:      workdir,
		}
		bytes, err := json.Marshal(state)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(sessionPath, "session.json"), bytes, 0644))
	}

	writeState("018f6e30-3acb-7f24-bede-8d96cd157150", directWorkDir)
	writeState("018f6e30-3acb-7f24-bede-8d96cd157151", worktreeDir)
	writeState("018f6e30-3acb-7f24-bede-8d96cd157152", worktreeDir)

	t.Run("uuid passthrough", func(t *testing.T) {
		id, err := resolveResumeTargetToSessionID("018f6e30-3acb-7f24-bede-8d96cd157150", repoDir, logsDir)
		require.NoError(t, err)
		assert.Equal(t, "018f6e30-3acb-7f24-bede-8d96cd157150", id)
	})

	t.Run("last passthrough", func(t *testing.T) {
		id, err := resolveResumeTargetToSessionID("last", repoDir, logsDir)
		require.NoError(t, err)
		assert.Equal(t, "last", id)
	})

	t.Run("absolute workdir path resolves to matching session", func(t *testing.T) {
		id, err := resolveResumeTargetToSessionID(worktreeDir, repoDir, logsDir)
		require.NoError(t, err)
		assert.Equal(t, "018f6e30-3acb-7f24-bede-8d96cd157152", id)
	})

	t.Run("workdir name resolves newest session", func(t *testing.T) {
		id, err := resolveResumeTargetToSessionID("0145-feature-work", repoDir, logsDir)
		require.NoError(t, err)
		assert.Equal(t, "018f6e30-3acb-7f24-bede-8d96cd157152", id)
	})

	t.Run("branch resolves by worktree name", func(t *testing.T) {
		originalRunGit := runGitCommandFunc
		t.Cleanup(func() {
			runGitCommandFunc = originalRunGit
		})

		runGitCommandFunc = func(workDir string, args ...string) (string, error) {
			cleanWorkDir := filepath.Clean(workDir)
			require.Equal(t, filepath.Clean(repoDir), cleanWorkDir)
			joinedArgs := fmt.Sprintf("%v", args)
			switch joinedArgs {
			case "[show-ref --verify --quiet refs/heads/feature/existing]":
				return "", nil
			case "[worktree list --porcelain]":
				return fmt.Sprintf("worktree %s\nbranch refs/heads/feature/existing\n\n", worktreeDir), nil
			default:
				return "", fmt.Errorf("unexpected git command: %s | %s", cleanWorkDir, joinedArgs)
			}
		}

		id, err := resolveResumeTargetToSessionID("feature/existing", repoDir, logsDir)
		require.NoError(t, err)
		assert.Equal(t, "018f6e30-3acb-7f24-bede-8d96cd157152", id)
	})
}
