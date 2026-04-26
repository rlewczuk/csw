package vcs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/testutil/fixture"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitRepo(t *testing.T) {
	t.Run("ValidGitRepository", func(t *testing.T) {
		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()

		assert.NotNil(t, fixture.Repo, "Expected non-nil VCS")
	})

	t.Run("NonExistentDirectory", func(t *testing.T) {
		_, err := NewGitRepo("/path/that/does/not/exist", fixture.ProjectPath("tmp", "worktrees"), nil, nil, "", "")
		assert.ErrorIs(t, err, apis.ErrFileNotFound)
	})

	t.Run("NotAGitRepository", func(t *testing.T) {
		tempDir := fixture.MkProjectTempDir(t, "not-git-*")

		_, err := NewGitRepo(tempDir, fixture.ProjectPath("tmp", "worktrees"), nil, nil, "", "")
		assert.ErrorIs(t, err, apis.ErrFileNotFound)
	})

	t.Run("PathIsFile", func(t *testing.T) {
		tmpDir := fixture.ProjectTmpDir(t)
		tempFile, err := os.CreateTemp(tmpDir, "git-test-file-*")
		require.NoError(t, err, "Failed to create temp file")
		t.Cleanup(func() {
			_ = os.Remove(tempFile.Name())
		})
		tempFile.Close()

		_, err = NewGitRepo(tempFile.Name(), fixture.ProjectPath("tmp", "worktrees"), nil, nil, "", "")
		assert.ErrorIs(t, err, apis.ErrNotADir)
	})
}

func TestGetWorktree(t *testing.T) {
	t.Run("GetExistingBranch", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			branch := getDefaultBranch(t, fixture)
			vfs, err := fixture.Repo.GetWorktree(branch)
			require.NoError(t, err, "Failed to get worktree")
			assert.NotNil(t, vfs, "Expected non-nil VFS")
		})
	})

	t.Run("GetWorktreeTwiceReturnsSame", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			branch := getDefaultBranch(t, fixture)
			vfs1, err := fixture.Repo.GetWorktree(branch)
			require.NoError(t, err)
			vfs2, err := fixture.Repo.GetWorktree(branch)
			require.NoError(t, err)

			assert.Equal(t, vfs1, vfs2, "Expected same VFS instance for same branch")
		})
	})

	t.Run("GetNonExistentBranch", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			_, err := fixture.Repo.GetWorktree("nonexistent-branch")
			assert.ErrorIs(t, err, apis.ErrFileNotFound)
		})
	})

	t.Run("GetWorktreeRespectsAllowedPaths", func(t *testing.T) {
		allowedDir := fixture.MkProjectTempDir(t, "git-allowed-*")
		allowedFile := filepath.Join(allowedDir, "allowed.txt")
		err := os.WriteFile(allowedFile, []byte("allowed content"), 0644)
		require.NoError(t, err)

		gitFixture := setupGitRepoFixtureWithAllowedPaths(t, []string{allowedDir})
		defer gitFixture.Cleanup()

		branch := getDefaultBranch(t, gitFixture)
		worktree, err := gitFixture.Repo.GetWorktree(branch)
		require.NoError(t, err)

		data, err := worktree.ReadFile(allowedFile)
		require.NoError(t, err)
		assert.Equal(t, "allowed content", string(data))
	})
}

func TestDropWorktree(t *testing.T) {
	t.Run("DropExistingWorktree", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			branch := getDefaultBranch(t, fixture)

			_, err := fixture.Repo.GetWorktree(branch)
			require.NoError(t, err, "Failed to get worktree")

			// Now drop it
			err = fixture.Repo.DropWorktree(branch)
			require.NoError(t, err, "Failed to drop worktree")
		})
	})

	t.Run("DropNonExistentWorktree", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			err := fixture.Repo.DropWorktree("nonexistent-worktree")
			assert.ErrorIs(t, err, apis.ErrFileNotFound)
		})
	})

	t.Run("DropWorktreeFromFreshInstance", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			baseBranch := getDefaultBranch(t, fixture)
			worktreeBranch := "feature-clean"

			err := fixture.Repo.NewBranch(worktreeBranch, baseBranch)
			require.NoError(t, err)

			_, err = fixture.Repo.GetWorktree(worktreeBranch)
			require.NoError(t, err)

			freshRepo, err := NewGitRepo(fixture.Root, fixture.WorktreesDir, nil, nil, "", "")
			require.NoError(t, err)

			err = freshRepo.DropWorktree(worktreeBranch)
			require.NoError(t, err)
		})
	})
}

func TestNewBranch(t *testing.T) {
	t.Run("CreateNewBranch", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			sourceBranch := getDefaultBranch(t, fixture)

			err := fixture.Repo.NewBranch("feature-branch", sourceBranch)
			require.NoError(t, err, "Failed to create new branch")

			// Verify the branch exists
			branches, err := fixture.Repo.ListBranches("")
			require.NoError(t, err)
			assert.Contains(t, branches, "feature-branch", "Expected new branch to exist")
		})
	})

	t.Run("CreateBranchFromNonExistent", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			err := fixture.Repo.NewBranch("new-branch", "nonexistent")
			assert.ErrorIs(t, err, apis.ErrFileNotFound)
		})
	})

	t.Run("CreateDuplicateBranch", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			sourceBranch := getDefaultBranch(t, fixture)

			// Create a branch first
			err := fixture.Repo.NewBranch("duplicate-branch", sourceBranch)
			require.NoError(t, err)

			// Try to create it again
			err = fixture.Repo.NewBranch("duplicate-branch", sourceBranch)
			assert.ErrorIs(t, err, apis.ErrFileExists)
		})
	})
}

func TestDeleteBranch(t *testing.T) {
	t.Run("DeleteExistingBranch", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			sourceBranch := getDefaultBranch(t, fixture)

			// Create a branch first
			err := fixture.Repo.NewBranch("delete-me", sourceBranch)
			require.NoError(t, err)

			// Delete it
			err = fixture.Repo.DeleteBranch("delete-me")
			require.NoError(t, err, "Failed to delete branch")

			// Verify it's gone
			branches, err := fixture.Repo.ListBranches("")
			require.NoError(t, err)
			assert.NotContains(t, branches, "delete-me", "Expected branch to be deleted")
		})
	})

	t.Run("DeleteNonExistentBranch", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			err := fixture.Repo.DeleteBranch("nonexistent-branch")
			assert.ErrorIs(t, err, apis.ErrFileNotFound)
		})
	})
}

func TestListBranches(t *testing.T) {
	t.Run("ListAllBranches", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			sourceBranch := getDefaultBranch(t, fixture)

			// Create some branches
			err := fixture.Repo.NewBranch("feature-1", sourceBranch)
			require.NoError(t, err)
			err = fixture.Repo.NewBranch("feature-2", sourceBranch)
			require.NoError(t, err)

			// List all branches
			branches, err := fixture.Repo.ListBranches("")
			require.NoError(t, err)

			// Should contain at least the default branch and our new branches
			assert.GreaterOrEqual(t, len(branches), 3, "Expected at least 3 branches")
		})
	})

	t.Run("ListBranchesWithPrefix", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			sourceBranch := getDefaultBranch(t, fixture)

			// Create branches with different prefixes
			err := fixture.Repo.NewBranch("feature/login", sourceBranch)
			require.NoError(t, err)
			err = fixture.Repo.NewBranch("feature/signup", sourceBranch)
			require.NoError(t, err)
			err = fixture.Repo.NewBranch("bugfix/crash", sourceBranch)
			require.NoError(t, err)

			// List branches with "feature/" prefix
			branches, err := fixture.Repo.ListBranches("feature/")
			require.NoError(t, err)

			assert.Len(t, branches, 2, "Expected 2 feature branches")
			assert.Contains(t, branches, "feature/login")
			assert.Contains(t, branches, "feature/signup")
		})
	})
}

func TestCommitWorktree(t *testing.T) {
	t.Run("CommitNonExistentWorktree", func(t *testing.T) {
		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			err := fixture.Repo.CommitWorktree("nonexistent", "message")
			assert.ErrorIs(t, err, apis.ErrFileNotFound)
		})
	})

	t.Run("CommitUsesConfiguredAuthorAndCommitter", func(t *testing.T) {
		fixture := setupGitRepoFixtureWithIdentity(t, "Configured User", "configured@example.com")
		defer fixture.Cleanup()

		repo, ok := fixture.Repo.(*GitVCS)
		require.True(t, ok)

		branch := getDefaultBranch(t, fixture)
		worktree, err := repo.GetWorktree(branch)
		require.NoError(t, err)

		err = worktree.WriteFile("identity.txt", []byte("identity check\n"))
		require.NoError(t, err)

		err = repo.CommitWorktree(branch, "identity commit")
		require.NoError(t, err)

		commitData, err := runGitCommand(fixture.Root, "log", "-1", "--format=%an%n%ae%n%cn%n%ce")
		require.NoError(t, err)
		lines := strings.Split(strings.TrimSpace(commitData), "\n")
		require.Len(t, lines, 4)

		assert.Equal(t, "Configured User", lines[0])
		assert.Equal(t, "configured@example.com", lines[1])
		assert.Equal(t, "Configured User", lines[2])
		assert.Equal(t, "configured@example.com", lines[3])
	})
}

func TestRepoInterfaceCompliance(t *testing.T) {
	// This test ensures GitVCS and MockVCS implement the VCS interface
	var _ apis.VCS = (*GitVCS)(nil)
	var _ apis.VCS = (*vfs.MockVCS)(nil)
}
