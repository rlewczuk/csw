package main

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCliWorktreeFlagDefinition(t *testing.T) {
	cmd := CliCommand()
	flag := cmd.Flags().Lookup("worktree")
	require.NotNil(t, flag)
	assert.Equal(t, "", flag.DefValue)
	assert.Equal(t, "string", flag.Value.Type())
}

func TestFinalizeWorktreeSession(t *testing.T) {
	tests := []struct {
		name             string
		worktreeBranch   string
		commitErr        error
		expectCommitCall bool
		expectDropCall   bool
		expectStderr     string
	}{
		{
			name:             "commit and drop worktree",
			worktreeBranch:   "feature/test",
			expectCommitCall: true,
			expectDropCall:   true,
		},
		{
			name:             "no changes to commit still drops worktree",
			worktreeBranch:   "feature/no-changes",
			commitErr:        vfs.ErrNoChangesToCommit,
			expectCommitCall: true,
			expectDropCall:   true,
		},
		{
			name:             "commit error is logged and worktree is still dropped",
			worktreeBranch:   "feature/error",
			commitErr:        errors.New("commit failed"),
			expectCommitCall: true,
			expectDropCall:   true,
			expectStderr:     "worktree commit failed",
		},
		{
			name:             "no worktree branch skips finalization",
			worktreeBranch:   "",
			expectCommitCall: false,
			expectDropCall:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVCS := vfs.NewMockVCS(vfs.NewMockVFS())
			mockVCS.SetCommitError(tt.commitErr)

			var stderr bytes.Buffer
			timestamp := time.Date(2026, 2, 23, 10, 30, 0, 0, time.UTC)
			finalizeWorktreeSession(mockVCS, tt.worktreeBranch, "session-123", timestamp, &stderr)

			commitCalls := mockVCS.GetCommitCalls()
			if tt.expectCommitCall {
				require.Len(t, commitCalls, 1)
				assert.Equal(t, tt.worktreeBranch, commitCalls[0].Branch)
				assert.Equal(t, buildWorktreeCommitMessage(tt.worktreeBranch, "session-123", timestamp), commitCalls[0].Message)
			} else {
				assert.Len(t, commitCalls, 0)
			}

			dropCalls := mockVCS.GetDropCalls()
			if tt.expectDropCall {
				require.Len(t, dropCalls, 1)
				assert.Equal(t, tt.worktreeBranch, dropCalls[0])
			} else {
				assert.Len(t, dropCalls, 0)
			}

			if tt.expectStderr != "" {
				assert.Contains(t, stderr.String(), tt.expectStderr)
			} else {
				assert.NotContains(t, stderr.String(), "worktree commit failed")
			}
		})
	}
}

func TestBuildWorktreeCommitMessage(t *testing.T) {
	timestamp := time.Date(2026, 2, 23, 10, 45, 0, 0, time.UTC)
	msg := buildWorktreeCommitMessage("feature/branch", "session-1", timestamp)
	assert.Contains(t, msg, "branch=feature/branch")
	assert.Contains(t, msg, "session=session-1")
	assert.Contains(t, msg, "timestamp=2026-02-23T10:45:00Z")
}
