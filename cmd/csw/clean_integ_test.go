package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clean_integ_test.go contains integration tests for the clean command.

// TestCleanCommand_SingleWorktree tests cleaning up a specific worktree.
func TestCleanCommand_SingleWorktree(t *testing.T) {
	// Create a mock VCS with worktrees
	mockVCS := vfs.NewMockVCS(nil)

	// Add some worktrees
	_, _ = mockVCS.GetWorktree("feature-1")
	_, _ = mockVCS.GetWorktree("feature-2")
	_, _ = mockVCS.GetWorktree("feature-3")

	// Verify we have 3 worktrees
	worktrees, err := mockVCS.ListWorktrees()
	require.NoError(t, err)
	assert.Len(t, worktrees, 3)

	// Create output buffer
	output := &bytes.Buffer{}

	// Clean up a single worktree
	err = cleanSingleWorktree(mockVCS, "feature-2", output)
	require.NoError(t, err)
	assert.Contains(t, output.String(), "Cleaned up worktree: feature-2")

	// Verify worktree was removed
	worktrees, err = mockVCS.ListWorktrees()
	require.NoError(t, err)
	assert.Len(t, worktrees, 2)

	// Verify the remaining worktrees
	remainingMap := make(map[string]bool)
	for _, wt := range worktrees {
		remainingMap[wt] = true
	}
	assert.True(t, remainingMap["feature-1"])
	assert.True(t, remainingMap["feature-3"])
	assert.False(t, remainingMap["feature-2"])
}

// TestCleanCommand_SingleWorktree_NotFound tests error handling when worktree doesn't exist.
func TestCleanCommand_SingleWorktree_NotFound(t *testing.T) {
	mockVCS := vfs.NewMockVCS(nil)

	output := &bytes.Buffer{}
	err := cleanSingleWorktree(mockVCS, "nonexistent", output)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worktree \"nonexistent\" not found")
}

// TestCleanCommand_AllWorktrees tests cleaning up all worktrees.
func TestCleanCommand_AllWorktrees(t *testing.T) {
	mockVCS := vfs.NewMockVCS(nil)

	// Add some worktrees
	_, _ = mockVCS.GetWorktree("feature-a")
	_, _ = mockVCS.GetWorktree("feature-b")
	_, _ = mockVCS.GetWorktree("feature-c")

	// Verify we have 3 worktrees
	worktrees, err := mockVCS.ListWorktrees()
	require.NoError(t, err)
	assert.Len(t, worktrees, 3)

	// Create output buffer
	output := &bytes.Buffer{}

	// Clean up all worktrees
	err = cleanAllWorktrees(mockVCS, output)
	require.NoError(t, err)
	assert.Contains(t, output.String(), "Cleaned up 3 worktree(s)")

	// Verify all worktrees were removed
	worktrees, err = mockVCS.ListWorktrees()
	require.NoError(t, err)
	assert.Len(t, worktrees, 0)
}

// TestCleanCommand_AllWorktrees_Empty tests cleaning up when no worktrees exist.
func TestCleanCommand_AllWorktrees_Empty(t *testing.T) {
	mockVCS := vfs.NewMockVCS(nil)

	output := &bytes.Buffer{}
	err := cleanAllWorktrees(mockVCS, output)
	require.NoError(t, err)
	assert.Contains(t, output.String(), "No worktrees to clean up")
}

// TestCleanCommand_AllWorktrees_WithErrors tests error handling during cleanup.
func TestCleanCommand_AllWorktrees_WithErrors(t *testing.T) {
	mockVCS := vfs.NewMockVCS(nil)

	// Add worktrees
	_, _ = mockVCS.GetWorktree("feature-1")
	_, _ = mockVCS.GetWorktree("feature-2")

	// Set up error for dropping one worktree
	mockVCS.SetDropError(os.ErrPermission)

	output := &bytes.Buffer{}
	err := cleanAllWorktrees(mockVCS, output)
	require.NoError(t, err) // Should not return error, just log warnings
	assert.Contains(t, output.String(), "Warning:")
	assert.Contains(t, output.String(), "failed to clean up worktree")
}

// TestCleanCommand_DropWorktreeError tests error when dropping a specific worktree fails.
func TestCleanCommand_DropWorktreeError(t *testing.T) {
	mockVCS := vfs.NewMockVCS(nil)

	// Add a worktree
	_, _ = mockVCS.GetWorktree("feature-error")

	// Set up error for dropping
	mockVCS.SetDropError(os.ErrPermission)

	output := &bytes.Buffer{}
	err := cleanSingleWorktree(mockVCS, "feature-error", output)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to drop worktree")
}

// TestListWorktrees tests the ListWorktrees method on MockVCS.
func TestListWorktrees(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		mockVCS := vfs.NewMockVCS(nil)
		worktrees, err := mockVCS.ListWorktrees()
		require.NoError(t, err)
		assert.Empty(t, worktrees)
	})

	t.Run("with worktrees", func(t *testing.T) {
		mockVCS := vfs.NewMockVCS(nil)
		_, _ = mockVCS.GetWorktree("branch-1")
		_, _ = mockVCS.GetWorktree("branch-2")

		worktrees, err := mockVCS.ListWorktrees()
		require.NoError(t, err)
		assert.Len(t, worktrees, 2)

		// Check that both branches are in the list
		worktreeMap := make(map[string]bool)
		for _, wt := range worktrees {
			worktreeMap[wt] = true
		}
		assert.True(t, worktreeMap["branch-1"])
		assert.True(t, worktreeMap["branch-2"])
	})
}

// TestNullVCS_ListWorktrees tests that NullVCS returns empty list.
func TestNullVCS_ListWorktrees(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	nullVCS, err := vfs.NewNullVFS(mockVFS)
	require.NoError(t, err)

	worktrees, err := nullVCS.ListWorktrees()
	require.NoError(t, err)
	assert.Empty(t, worktrees)
}
