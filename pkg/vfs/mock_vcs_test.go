package vfs

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMockVCS_ListWorktrees tests the ListWorktrees method on MockVCS.
func TestMockVCS_ListWorktrees(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		mockVCS := NewMockVCS(nil)
		worktrees, err := mockVCS.ListWorktrees()
		require.NoError(t, err)
		assert.Empty(t, worktrees)
	})

	t.Run("with worktrees", func(t *testing.T) {
		mockVCS := NewMockVCS(nil)
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

	t.Run("after drop", func(t *testing.T) {
		mockVCS := NewMockVCS(nil)
		_, _ = mockVCS.GetWorktree("branch-1")
		_, _ = mockVCS.GetWorktree("branch-2")

		_ = mockVCS.DropWorktree("branch-1")

		worktrees, err := mockVCS.ListWorktrees()
		require.NoError(t, err)
		assert.Len(t, worktrees, 1)
		assert.Equal(t, "branch-2", worktrees[0])
	})
}

// TestVCSInterfaceCompliance verifies that all VCS implementations implement the interface.
func TestVCSInterfaceCompliance(t *testing.T) {
	var _ apis.VCS = (*MockVCS)(nil)
	// Note: NullVCS compliance is covered in pkg/vcs to avoid import cycles.
}
