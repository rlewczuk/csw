package vfs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GitTestFixture provides test fixtures for git repository tests
type GitTestFixture struct {
	Root         string
	WorktreesDir string
	Repo         VCS
}

// setupGitRepoFixture creates a temporary git repository for testing
func setupGitRepoFixture(t *testing.T) *GitTestFixture {
	t.Helper()
	return setupGitRepoFixtureWithIdentity(t, "", "")
}

// setupGitRepoFixtureWithIdentity creates a temporary git repository for testing with explicit git identity.
func setupGitRepoFixtureWithIdentity(t *testing.T, gitUserName string, gitUserEmail string) *GitTestFixture {
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
	gitRepo, err := NewGitRepo(tempDir, worktreesDir, nil, gitUserName, gitUserEmail)
	require.NoError(t, err, "Failed to create GitVCS")

	return &GitTestFixture{
		Root:         tempDir,
		WorktreesDir: worktreesDir,
		Repo:         gitRepo,
	}
}

// Cleanup removes the temporary directory
func (f *GitTestFixture) Cleanup() {
	if f.Root != "" {
		os.RemoveAll(f.Root)
	}
}

// runTestWithGitVCS runs a test function with GitVCS only
func runTestWithGitVCS(t *testing.T, testFunc func(*testing.T, *GitTestFixture)) {
	t.Run("GitVCS", func(t *testing.T) {
		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()
		testFunc(t, fixture)
	})
}

// getDefaultBranch returns the default branch name (master or main)
func getDefaultBranch(t *testing.T, fixture *GitTestFixture) string {
	// Try master first
	_, err := fixture.Repo.GetWorktree("master")
	if err == nil {
		return "master"
	}
	// Try main
	_, err = fixture.Repo.GetWorktree("main")
	if err == nil {
		return "main"
	}
	t.Fatal("Neither master nor main branch found")
	return ""
}

// runGitCommand executes git with the provided arguments in a repository path.
func runGitCommand(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func TestNewGitRepo(t *testing.T) {
	t.Run("ValidGitRepository", func(t *testing.T) {
		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()

		assert.NotNil(t, fixture.Repo, "Expected non-nil VCS")
	})

	t.Run("NonExistentDirectory", func(t *testing.T) {
		_, err := NewGitRepo("/path/that/does/not/exist", "../../tmp/worktrees", nil, "", "")
		assert.ErrorIs(t, err, ErrFileNotFound)
	})

	t.Run("NotAGitRepository", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("../../tmp", "not-git-*")
		require.NoError(t, err, "Failed to create temp directory")
		defer os.RemoveAll(tempDir)

		_, err = NewGitRepo(tempDir, "../../tmp/worktrees", nil, "", "")
		assert.ErrorIs(t, err, ErrFileNotFound)
	})

	t.Run("PathIsFile", func(t *testing.T) {
		tempFile, err := os.CreateTemp("../../tmp", "git-test-file-*")
		require.NoError(t, err, "Failed to create temp file")
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		_, err = NewGitRepo(tempFile.Name(), "../../tmp/worktrees", nil, "", "")
		assert.ErrorIs(t, err, ErrNotADir)
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
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
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
			assert.ErrorIs(t, err, ErrFileNotFound)
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

			freshRepo, err := NewGitRepo(fixture.Root, fixture.WorktreesDir, nil, "", "")
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
			assert.ErrorIs(t, err, ErrFileNotFound)
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
			assert.ErrorIs(t, err, ErrFileExists)
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
			assert.ErrorIs(t, err, ErrFileNotFound)
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
			assert.ErrorIs(t, err, ErrFileNotFound)
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

		gitRepo, err := git.PlainOpen(fixture.Root)
		require.NoError(t, err)

		head, err := gitRepo.Head()
		require.NoError(t, err)

		commitObject, err := gitRepo.CommitObject(head.Hash())
		require.NoError(t, err)

		assert.Equal(t, "Configured User", commitObject.Author.Name)
		assert.Equal(t, "configured@example.com", commitObject.Author.Email)
		assert.Equal(t, "Configured User", commitObject.Committer.Name)
		assert.Equal(t, "configured@example.com", commitObject.Committer.Email)
	})
}

func TestMergeBranches(t *testing.T) {
	t.Run("MergeBranches", func(t *testing.T) {
		t.Parallel()

		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			sourceBranch := getDefaultBranch(t, fixture)

			// Create a source branch
			err := fixture.Repo.NewBranch("source-branch", sourceBranch)
			require.NoError(t, err)

			// Merge into default branch
			err = fixture.Repo.MergeBranches(sourceBranch, "source-branch")
			require.NoError(t, err, "Failed to merge branches")
		})
	})

	t.Run("MergeNonExistentSource", func(t *testing.T) {
		t.Parallel()

		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			targetBranch := getDefaultBranch(t, fixture)

			err := fixture.Repo.MergeBranches(targetBranch, "nonexistent")
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})

	t.Run("MergeNonExistentTarget", func(t *testing.T) {
		t.Parallel()

		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			sourceBranch := getDefaultBranch(t, fixture)

			err := fixture.Repo.NewBranch("source-branch2", sourceBranch)
			require.NoError(t, err)

			err = fixture.Repo.MergeBranches("nonexistent", "source-branch2")
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})

	t.Run("GitVCSMergeConflict", func(t *testing.T) {
		t.Parallel()

		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()

		repo, ok := fixture.Repo.(*GitVCS)
		require.True(t, ok)

		sourceBranch := getDefaultBranch(t, fixture)

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

	t.Run("GitVCSFastForwardWhenPossible", func(t *testing.T) {
		t.Parallel()

		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()

		repo, ok := fixture.Repo.(*GitVCS)
		require.True(t, ok)

		targetBranch := getDefaultBranch(t, fixture)

		err := repo.NewBranch("feature-ff", targetBranch)
		require.NoError(t, err)

		featureWorktree, err := repo.GetWorktree("feature-ff")
		require.NoError(t, err)

		err = featureWorktree.WriteFile("feature-fast-forward.txt", []byte("fast-forward\n"))
		require.NoError(t, err)
		err = repo.CommitWorktree("feature-ff", "Feature commit for fast-forward")
		require.NoError(t, err)

		err = repo.MergeBranches(targetBranch, "feature-ff")
		require.NoError(t, err)

		gitRepo, err := git.PlainOpen(fixture.Root)
		require.NoError(t, err)

		targetRef, err := gitRepo.Reference(plumbing.NewBranchReferenceName(targetBranch), true)
		require.NoError(t, err)

		mergedCommit, err := gitRepo.CommitObject(targetRef.Hash())
		require.NoError(t, err)
		assert.Len(t, mergedCommit.ParentHashes, 1)
		assert.Equal(t, "Feature commit for fast-forward", strings.TrimSpace(mergedCommit.Message))
	})

	t.Run("GitVCSRebasesSourceThenFastForwardsWhenFastForwardNotPossible", func(t *testing.T) {
		t.Parallel()

		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()

		repo, ok := fixture.Repo.(*GitVCS)
		require.True(t, ok)

		targetBranch := getDefaultBranch(t, fixture)

		err := repo.NewBranch("feature-rebase", targetBranch)
		require.NoError(t, err)

		targetWorktree, err := repo.GetWorktree(targetBranch)
		require.NoError(t, err)
		err = targetWorktree.WriteFile("target-only.txt", []byte("target change\n"))
		require.NoError(t, err)
		err = repo.CommitWorktree(targetBranch, "Target branch commit")
		require.NoError(t, err)

		featureWorktree, err := repo.GetWorktree("feature-rebase")
		require.NoError(t, err)
		err = featureWorktree.WriteFile("feature-only.txt", []byte("feature change\n"))
		require.NoError(t, err)
		err = repo.CommitWorktree("feature-rebase", "Feature branch commit")
		require.NoError(t, err)

		err = repo.MergeBranches(targetBranch, "feature-rebase")
		require.NoError(t, err)

		gitRepo, err := git.PlainOpen(fixture.Root)
		require.NoError(t, err)

		targetRef, err := gitRepo.Reference(plumbing.NewBranchReferenceName(targetBranch), true)
		require.NoError(t, err)

		headCommit, err := gitRepo.CommitObject(targetRef.Hash())
		require.NoError(t, err)
		assert.Len(t, headCommit.ParentHashes, 1)
		assert.Equal(t, "Feature branch commit", strings.TrimSpace(headCommit.Message))

		targetCommitHashOutput, err := runGitCommand(fixture.Root, "rev-list", "--max-count=1", "--grep", "^Target branch commit$", targetBranch)
		require.NoError(t, err)
		targetCommitHash := plumbing.NewHash(strings.TrimSpace(targetCommitHashOutput))
		assert.Equal(t, targetCommitHash, headCommit.ParentHashes[0])

		targetContent, err := os.ReadFile(filepath.Join(fixture.Root, "target-only.txt"))
		require.NoError(t, err)
		assert.Equal(t, "target change\n", string(targetContent))

		featureContent, err := os.ReadFile(filepath.Join(fixture.Root, "feature-only.txt"))
		require.NoError(t, err)
		assert.Equal(t, "feature change\n", string(featureContent))
	})

	t.Run("GitVCSReturnsMergeConflictWhenRebaseFails", func(t *testing.T) {
		t.Parallel()

		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()

		repo, ok := fixture.Repo.(*GitVCS)
		require.True(t, ok)

		targetBranch := getDefaultBranch(t, fixture)

		err := repo.NewBranch("feature-rebase-fail", targetBranch)
		require.NoError(t, err)

		targetWorktree, err := repo.GetWorktree(targetBranch)
		require.NoError(t, err)
		err = targetWorktree.WriteFile("conflict.txt", []byte("target content\n"))
		require.NoError(t, err)
		err = repo.CommitWorktree(targetBranch, "Target conflict commit")
		require.NoError(t, err)

		featureWorktree, err := repo.GetWorktree("feature-rebase-fail")
		require.NoError(t, err)
		err = featureWorktree.WriteFile("conflict.txt", []byte("feature content\n"))
		require.NoError(t, err)
		err = repo.CommitWorktree("feature-rebase-fail", "Feature conflict commit")
		require.NoError(t, err)

		err = repo.MergeBranches(targetBranch, "feature-rebase-fail")
		assert.ErrorIs(t, err, ErrMergeConflict)
	})

	t.Run("GitVCSUpdatesCheckedOutFilesAfterMerge", func(t *testing.T) {
		t.Parallel()

		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()

		repo, ok := fixture.Repo.(*GitVCS)
		require.True(t, ok)

		targetBranch := getDefaultBranch(t, fixture)

		err := repo.NewBranch("feature-update-files", targetBranch)
		require.NoError(t, err)

		featureWorktree, err := repo.GetWorktree("feature-update-files")
		require.NoError(t, err)

		err = featureWorktree.WriteFile("merged-file.txt", []byte("updated after merge\n"))
		require.NoError(t, err)
		err = repo.CommitWorktree("feature-update-files", "Add merged file")
		require.NoError(t, err)

		err = repo.MergeBranches(targetBranch, "feature-update-files")
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(fixture.Root, "merged-file.txt"))
		require.NoError(t, err)
		assert.Equal(t, "updated after merge\n", string(content))
	})
}

func TestRepoInterfaceCompliance(t *testing.T) {
	// This test ensures GitVCS and MockVCS implement the VCS interface
	var _ VCS = (*GitVCS)(nil)
	var _ VCS = (*MockVCS)(nil)
}
