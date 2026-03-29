package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMergeCLIParams(t *testing.T) {
	tests := []struct {
		name           string
		params         *CLIParams
		expectError    bool
		errorSubstring string
	}{
		{
			name: "merge without worktree or resume is rejected",
			params: &CLIParams{
				Merge: true,
			},
			expectError:    true,
			errorSubstring: "--merge requires --worktree",
		},
		{
			name: "merge with worktree is allowed",
			params: &CLIParams{
				Merge:          true,
				WorktreeBranch: "feature/test",
			},
		},
		{
			name: "merge with resume is allowed",
			params: &CLIParams{
				Merge:        true,
				ResumeTarget: "last",
			},
		},
		{
			name: "non merge without worktree is allowed",
			params: &CLIParams{
				Merge: false,
			},
		},
		{
			name:           "nil params are rejected",
			params:         nil,
			expectError:    true,
			errorSubstring: "params cannot be nil",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMergeCLIParams(tc.params)
			if tc.expectError {
				require.Error(t, err)
				if tc.errorSubstring != "" {
					assert.Contains(t, err.Error(), tc.errorSubstring)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestInferResumeWorktreeBranch(t *testing.T) {
	tmpDir := t.TempDir()
	workDirRoot := filepath.Join(tmpDir, "repo")
	shadowDir := filepath.Join(tmpDir, "shadow")
	require.NoError(t, os.MkdirAll(workDirRoot, 0755))
	require.NoError(t, os.MkdirAll(shadowDir, 0755))

	tests := []struct {
		name          string
		root          string
		shadow        string
		sessionWorkDir string
		expected      string
		expectedOK    bool
	}{
		{
			name:           "worktree under repo root resolves branch",
			root:           workDirRoot,
			sessionWorkDir: filepath.Join(workDirRoot, ".cswdata", "work", "0148-feature"),
			expected:       "0148-feature",
			expectedOK:     true,
		},
		{
			name:           "nested worktree branch path resolves with slash",
			root:           workDirRoot,
			sessionWorkDir: filepath.Join(workDirRoot, ".cswdata", "work", "feature", "nested"),
			expected:       "feature/nested",
			expectedOK:     true,
		},
		{
			name:           "shadow directory is preferred for worktree root",
			root:           workDirRoot,
			shadow:         shadowDir,
			sessionWorkDir: filepath.Join(shadowDir, ".cswdata", "work", "0148-shadow-feature"),
			expected:       "0148-shadow-feature",
			expectedOK:     true,
		},
		{
			name:           "session workdir outside worktree root is rejected",
			root:           workDirRoot,
			sessionWorkDir: filepath.Join(workDirRoot, "not-a-worktree"),
			expectedOK:     false,
		},
		{
			name:           "empty session workdir is rejected",
			root:           workDirRoot,
			sessionWorkDir: "",
			expectedOK:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			branch, ok := inferResumeWorktreeBranch(tc.root, tc.shadow, tc.sessionWorkDir)
			assert.Equal(t, tc.expectedOK, ok)
			if tc.expectedOK {
				assert.Equal(t, tc.expected, branch)
			}
		})
	}
}
