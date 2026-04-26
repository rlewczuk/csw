package vcs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			assert.ErrorIs(t, err, apis.ErrFileNotFound)
		})
	})

	t.Run("MergeNonExistentTarget", func(t *testing.T) {
		t.Parallel()

		runTestWithGitVCS(t, func(t *testing.T, fixture *GitTestFixture) {
			sourceBranch := getDefaultBranch(t, fixture)

			err := fixture.Repo.NewBranch("source-branch2", sourceBranch)
			require.NoError(t, err)

			err = fixture.Repo.MergeBranches("nonexistent", "source-branch2")
			assert.ErrorIs(t, err, apis.ErrFileNotFound)
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
		assert.ErrorIs(t, err, apis.ErrMergeConflict)
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

		parentsLine, err := runGitCommand(fixture.Root, "rev-list", "--parents", "-n", "1", targetBranch)
		require.NoError(t, err)
		parentFields := strings.Fields(strings.TrimSpace(parentsLine))
		require.Len(t, parentFields, 2)

		message, err := runGitCommand(fixture.Root, "log", "-1", "--format=%s", targetBranch)
		require.NoError(t, err)
		assert.Equal(t, "Feature commit for fast-forward", strings.TrimSpace(message))
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

		parentsLine, err := runGitCommand(fixture.Root, "rev-list", "--parents", "-n", "1", targetBranch)
		require.NoError(t, err)
		parentFields := strings.Fields(strings.TrimSpace(parentsLine))
		require.Len(t, parentFields, 2)

		headMessage, err := runGitCommand(fixture.Root, "log", "-1", "--format=%s", targetBranch)
		require.NoError(t, err)
		assert.Equal(t, "Feature branch commit", strings.TrimSpace(headMessage))

		targetCommitHashOutput, err := runGitCommand(fixture.Root, "rev-list", "--max-count=1", "--grep", "^Target branch commit$", targetBranch)
		require.NoError(t, err)
		targetCommitHash := strings.TrimSpace(targetCommitHashOutput)
		assert.Equal(t, targetCommitHash, parentFields[1])

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
		assert.ErrorIs(t, err, apis.ErrMergeConflict)
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

	t.Run("GitVCSUpdatesAllCheckedOutTargetBranchWorktreesAfterMerge", func(t *testing.T) {
		t.Parallel()

		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()

		repo, ok := fixture.Repo.(*GitVCS)
		require.True(t, ok)

		targetBranch := getDefaultBranch(t, fixture)

		err := repo.NewBranch("feature-update-all-checkouts", targetBranch)
		require.NoError(t, err)

		featureWorktree, err := repo.GetWorktree("feature-update-all-checkouts")
		require.NoError(t, err)

		err = featureWorktree.WriteFile("merged-everywhere.txt", []byte("synced in all checkouts\n"))
		require.NoError(t, err)
		err = repo.CommitWorktree("feature-update-all-checkouts", "Add merged file for all checkouts")
		require.NoError(t, err)

		_, err = repo.GetWorktree(targetBranch)
		require.NoError(t, err)

		ideWorktreePath := filepath.Join(fixture.Root, "ide-main")
		_, err = runGitCommand(fixture.Root, "worktree", "add", "--force", ideWorktreePath, targetBranch)
		require.NoError(t, err)

		err = repo.MergeBranches(targetBranch, "feature-update-all-checkouts")
		require.NoError(t, err)

		ideContent, err := os.ReadFile(filepath.Join(ideWorktreePath, "merged-everywhere.txt"))
		require.NoError(t, err)
		assert.Equal(t, "synced in all checkouts\n", string(ideContent))
	})
}
