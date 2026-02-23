package vfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GitTestFixture provides test fixtures for git repository tests
type GitTestFixture struct {
	Root   string
	Repo   VCS
	IsMock bool
}

// setupGitRepoFixture creates a temporary git repository for testing
func setupGitRepoFixture(t *testing.T) *GitTestFixture {
	t.Helper()

	// Create a temporary directory in project root tmp/
	tempDir, err := os.MkdirTemp("../../tmp", "git-test-*")
	require.NoError(t, err, "Failed to create temp directory")

	// Create a worktrees directory
	worktreesDir := filepath.Join(tempDir, "worktrees")

	// Initialize a git repository
	repo, err := git.PlainInit(tempDir, false)
	require.NoError(t, err, "Failed to initialize git repository")

	// Create an initial commit
	w, err := repo.Worktree()
	require.NoError(t, err, "Failed to get worktree")

	// Create an initial file
	initialFile := filepath.Join(tempDir, "README.md")
	err = os.WriteFile(initialFile, []byte("# Initial Commit\n"), 0644)
	require.NoError(t, err, "Failed to create initial file")

	_, err = w.Add("README.md")
	require.NoError(t, err, "Failed to add file")

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err, "Failed to create initial commit")

	// Create the GitVCS with worktrees directory
	gitRepo, err := NewGitRepo(tempDir, worktreesDir, nil)
	require.NoError(t, err, "Failed to create GitVCS")

	return &GitTestFixture{
		Root:   tempDir,
		Repo:   gitRepo,
		IsMock: false,
	}
}

// setupMockRepoFixture creates a mock repository for testing
func setupMockRepoFixture(t *testing.T) *GitTestFixture {
	t.Helper()

	mockRepo := NewMockRepo()
	require.NotNil(t, mockRepo, "Failed to create MockRepo")

	return &GitTestFixture{
		Root:   "",
		Repo:   mockRepo,
		IsMock: true,
	}
}

// Cleanup removes the temporary directory
func (f *GitTestFixture) Cleanup() {
	if f.Root != "" {
		os.RemoveAll(f.Root)
	}
}

// runTestWithBothRepos runs a test function with both GitVCS and MockRepo
func runTestWithBothRepos(t *testing.T, testFunc func(*testing.T, *GitTestFixture)) {
	t.Run("GitVCS", func(t *testing.T) {
		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()
		testFunc(t, fixture)
	})

	t.Run("MockRepo", func(t *testing.T) {
		fixture := setupMockRepoFixture(t)
		defer fixture.Cleanup()
		testFunc(t, fixture)
	})
}

func TestNewGitRepo(t *testing.T) {
	t.Run("ValidGitRepository", func(t *testing.T) {
		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()

		assert.NotNil(t, fixture.Repo, "Expected non-nil VCS")
	})

	t.Run("NonExistentDirectory", func(t *testing.T) {
		_, err := NewGitRepo("/path/that/does/not/exist", "../../tmp/worktrees", nil)
		assert.ErrorIs(t, err, ErrFileNotFound)
	})

	t.Run("NotAGitRepository", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("../../tmp", "not-git-*")
		require.NoError(t, err, "Failed to create temp directory")
		defer os.RemoveAll(tempDir)

		_, err = NewGitRepo(tempDir, "../../tmp/worktrees", nil)
		assert.ErrorIs(t, err, ErrFileNotFound)
	})

	t.Run("PathIsFile", func(t *testing.T) {
		tempFile, err := os.CreateTemp("../../tmp", "git-test-file-*")
		require.NoError(t, err, "Failed to create temp file")
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		_, err = NewGitRepo(tempFile.Name(), "../../tmp/worktrees", nil)
		assert.ErrorIs(t, err, ErrNotADir)
	})
}

func TestNewMockRepo(t *testing.T) {
	t.Run("CreateMockRepo", func(t *testing.T) {
		repo := NewMockRepo()
		assert.NotNil(t, repo, "Expected non-nil MockRepo")
		assert.NotNil(t, repo.branches, "Expected branches map to be initialized")
		assert.NotNil(t, repo.worktrees, "Expected worktrees map to be initialized")

		// Should have a default "main" branch
		branches, err := repo.ListBranches("")
		require.NoError(t, err)
		assert.Contains(t, branches, "main", "Expected main branch to exist")
	})
}

func TestGetWorktree(t *testing.T) {
	t.Run("GetExistingBranch", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			var vfs VFS
			var err error

			if fixture.IsMock {
				vfs, err = fixture.Repo.GetWorktree("main")
			} else {
				// For GitVCS, use master or main depending on what's available
				vfs, err = fixture.Repo.GetWorktree("master")
				if err != nil {
					vfs, err = fixture.Repo.GetWorktree("main")
				}
			}

			require.NoError(t, err, "Failed to get worktree")
			assert.NotNil(t, vfs, "Expected non-nil VFS")
		})
	})

	t.Run("GetWorktreeTwiceReturnsSame", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			var vfs1, vfs2 VFS
			var err error

			if fixture.IsMock {
				vfs1, err = fixture.Repo.GetWorktree("main")
				require.NoError(t, err)
				vfs2, err = fixture.Repo.GetWorktree("main")
				require.NoError(t, err)
			} else {
				vfs1, err = fixture.Repo.GetWorktree("master")
				if err != nil {
					vfs1, err = fixture.Repo.GetWorktree("main")
				}
				require.NoError(t, err)

				vfs2, err = fixture.Repo.GetWorktree("master")
				if err != nil {
					vfs2, err = fixture.Repo.GetWorktree("main")
				}
				require.NoError(t, err)
			}

			assert.Equal(t, vfs1, vfs2, "Expected same VFS instance for same branch")
		})
	})

	t.Run("GetNonExistentBranch", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			_, err := fixture.Repo.GetWorktree("nonexistent-branch")
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})
}

func TestDropWorktree(t *testing.T) {
	t.Run("DropExistingWorktree", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			// First get a worktree
			var branchName string
			if fixture.IsMock {
				branchName = "main"
			} else {
				branchName = "master"
			}

			_, err := fixture.Repo.GetWorktree(branchName)
			require.NoError(t, err, "Failed to get worktree")

			// Now drop it
			err = fixture.Repo.DropWorktree(branchName)
			require.NoError(t, err, "Failed to drop worktree")
		})
	})

	t.Run("DropNonExistentWorktree", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			err := fixture.Repo.DropWorktree("nonexistent-worktree")
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})
}

func TestNewBranch(t *testing.T) {
	t.Run("CreateNewBranch", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			var sourceBranch string
			if fixture.IsMock {
				sourceBranch = "main"
			} else {
				sourceBranch = "master"
			}

			err := fixture.Repo.NewBranch("feature-branch", sourceBranch)
			require.NoError(t, err, "Failed to create new branch")

			// Verify the branch exists
			branches, err := fixture.Repo.ListBranches("")
			require.NoError(t, err)
			assert.Contains(t, branches, "feature-branch", "Expected new branch to exist")
		})
	})

	t.Run("CreateBranchFromNonExistent", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			err := fixture.Repo.NewBranch("new-branch", "nonexistent")
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})

	t.Run("CreateDuplicateBranch", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			var sourceBranch string
			if fixture.IsMock {
				sourceBranch = "main"
			} else {
				sourceBranch = "master"
			}

			// Create a branch first
			err := fixture.Repo.NewBranch("duplicate-branch", sourceBranch)
			require.NoError(t, err)

			// Try to create it again
			err = fixture.Repo.NewBranch("duplicate-branch", sourceBranch)
			assert.ErrorIs(t, err, ErrFileExists)
		})
	})
}

func TestDeleteBranch(t *testing.T) {
	t.Run("DeleteExistingBranch", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			var sourceBranch string
			if fixture.IsMock {
				sourceBranch = "main"
			} else {
				sourceBranch = "master"
			}

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
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			err := fixture.Repo.DeleteBranch("nonexistent-branch")
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})

	t.Run("DeleteOnlyBranch", func(t *testing.T) {
		// This test is only for MockRepo since GitVCS has different behavior
		t.Run("MockRepo", func(t *testing.T) {
			fixture := setupMockRepoFixture(t)
			defer fixture.Cleanup()

			// Try to delete the only branch (main)
			err := fixture.Repo.DeleteBranch("main")
			assert.ErrorIs(t, err, ErrPermissionDenied)
		})
	})
}

func TestListBranches(t *testing.T) {
	t.Run("ListAllBranches", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			var sourceBranch string
			if fixture.IsMock {
				sourceBranch = "main"
			} else {
				sourceBranch = "master"
			}

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
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			var sourceBranch string
			if fixture.IsMock {
				sourceBranch = "main"
			} else {
				sourceBranch = "master"
			}

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
	t.Run("CommitChanges", func(t *testing.T) {
		// This test is only for MockRepo since GitVCS requires more setup
		t.Run("MockRepo", func(t *testing.T) {
			fixture := setupMockRepoFixture(t)
			defer fixture.Cleanup()

			// Get worktree
			vfs, err := fixture.Repo.GetWorktree("main")
			require.NoError(t, err)

			// Make some changes
			err = vfs.WriteFile("newfile.txt", []byte("new content"))
			require.NoError(t, err)

			// Commit
			err = fixture.Repo.CommitWorktree("main", "Add new file")
			require.NoError(t, err)
		})
	})

	t.Run("CommitNonExistentWorktree", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			err := fixture.Repo.CommitWorktree("nonexistent", "message")
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})
}

func TestMergeBranches(t *testing.T) {
	t.Run("MergeBranches", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			var sourceBranch string
			if fixture.IsMock {
				sourceBranch = "main"
			} else {
				sourceBranch = "master"
			}

			// Create a source branch
			err := fixture.Repo.NewBranch("source-branch", sourceBranch)
			require.NoError(t, err)

			// Merge into default branch
			err = fixture.Repo.MergeBranches(sourceBranch, "source-branch")
			require.NoError(t, err, "Failed to merge branches")
		})
	})

	t.Run("MergeNonExistentSource", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			var targetBranch string
			if fixture.IsMock {
				targetBranch = "main"
			} else {
				targetBranch = "master"
			}

			err := fixture.Repo.MergeBranches(targetBranch, "nonexistent")
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})

	t.Run("MergeNonExistentTarget", func(t *testing.T) {
		runTestWithBothRepos(t, func(t *testing.T, fixture *GitTestFixture) {
			var sourceBranch string
			if fixture.IsMock {
				sourceBranch = "main"
			} else {
				sourceBranch = "master"
			}

			err := fixture.Repo.NewBranch("source-branch2", sourceBranch)
			require.NoError(t, err)

			err = fixture.Repo.MergeBranches("nonexistent", "source-branch2")
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})

	t.Run("GitVCSMergeConflict", func(t *testing.T) {
		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()

		repo, ok := fixture.Repo.(*GitVCS)
		require.True(t, ok)

		sourceBranch := "master"
		if _, err := repo.GetWorktree(sourceBranch); err != nil {
			sourceBranch = "main"
			_, err = repo.GetWorktree(sourceBranch)
			require.NoError(t, err)
		}

		err := repo.NewBranch("feature-conflict", sourceBranch)
		require.NoError(t, err)

		mainWorktree, err := repo.GetWorktree(sourceBranch)
		require.NoError(t, err)
		err = mainWorktree.WriteFile("README.md", []byte("main branch content\n"))
		require.NoError(t, err)
		err = repo.CommitWorktree(sourceBranch, "Update readme on main")
		require.NoError(t, err)

		featureWorktree, err := repo.GetWorktree("feature-conflict")
		require.NoError(t, err)
		err = featureWorktree.WriteFile("README.md", []byte("feature branch content\n"))
		require.NoError(t, err)
		err = repo.CommitWorktree("feature-conflict", "Update readme on feature")
		require.NoError(t, err)

		err = repo.MergeBranches(sourceBranch, "feature-conflict")
		assert.ErrorIs(t, err, ErrMergeConflict)
	})
}

func TestRepoInterfaceCompliance(t *testing.T) {
	// This test ensures GitVCS and MockRepo implement the VCS interface
	var _ VCS = (*GitVCS)(nil)
	var _ VCS = (*MockRepo)(nil)
}
