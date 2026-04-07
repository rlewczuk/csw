package vcs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestResolveGitIdentity tests the resolveGitIdentity function.
func TestResolveGitIdentity(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		gitConfigKey   string
		lookPathErr    error
		gitConfigValue string
		gitConfigErr   error
		expected       string
	}{
		{
			name:         "returns provided value when not empty",
			value:        "Provided User",
			gitConfigKey: "user.name",
			expected:     "Provided User",
		},
		{
			name:           "falls back to git config when value is empty",
			value:          "",
			gitConfigKey:   "user.name",
			gitConfigValue: "Git Config User",
			expected:       "Git Config User",
		},
		{
			name:           "returns empty when both value and git config are empty",
			value:          "",
			gitConfigKey:   "user.email",
			gitConfigValue: "",
			expected:       "",
		},
		{
			name:         "returns empty when git is not available",
			value:        "",
			gitConfigKey: "user.name",
			lookPathErr:  errors.New("git not found"),
			expected:     "",
		},
		{
			name:           "returns empty when git config fails",
			value:          "",
			gitConfigKey:   "user.email",
			gitConfigValue: "",
			gitConfigErr:   errors.New("config error"),
			expected:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalLookPath := GitLookPath
			originalConfigValue := GitConfigValue
			t.Cleanup(func() {
				GitLookPath = originalLookPath
				GitConfigValue = originalConfigValue
			})

			GitLookPath = func(file string) (string, error) {
				if tt.lookPathErr != nil {
					return "", tt.lookPathErr
				}
				return "/usr/bin/git", nil
			}
			GitConfigValue = func(key string) (string, error) {
				return tt.gitConfigValue, tt.gitConfigErr
			}

			result := ResolveGitIdentity(tt.value, tt.gitConfigKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCollectEditedFilesFallsBackToRepoDirAfterWorktreeMerge verifies commit-range diff still works after worktree removal.
func TestCollectEditedFilesFallsBackToRepoDirAfterWorktreeMerge(t *testing.T) {
	repoDir := t.TempDir()
	requireGitMergeTestNoError(t, runGitForMergeTest(repoDir, "init", "-b", "main"))
	requireGitMergeTestNoError(t, runGitForMergeTest(repoDir, "config", "user.name", "Test User"))
	requireGitMergeTestNoError(t, runGitForMergeTest(repoDir, "config", "user.email", "test@example.com"))

	filePath := filepath.Join(repoDir, "test.txt")
	requireGitMergeTestNoError(t, os.WriteFile(filePath, []byte("old\n"), 0644))
	requireGitMergeTestNoError(t, runGitForMergeTest(repoDir, "add", "test.txt"))
	requireGitMergeTestNoError(t, runGitForMergeTest(repoDir, "commit", "-m", "initial"))

	baseCommitID := ResolveGitCommitID(repoDir, "HEAD")
	assert.NotEmpty(t, baseCommitID)

	requireGitMergeTestNoError(t, runGitForMergeTest(repoDir, "branch", "feature/edited-files"))
	worktreeDir := filepath.Join(repoDir, ".cswdata", "work", "feature", "edited-files")
	requireGitMergeTestNoError(t, os.MkdirAll(filepath.Dir(worktreeDir), 0755))
	requireGitMergeTestNoError(t, runGitForMergeTest(repoDir, "worktree", "add", worktreeDir, "feature/edited-files"))

	requireGitMergeTestNoError(t, os.WriteFile(filepath.Join(worktreeDir, "test.txt"), []byte("old\nnew\n"), 0644))
	requireGitMergeTestNoError(t, runGitForMergeTest(worktreeDir, "add", "test.txt"))
	requireGitMergeTestNoError(t, runGitForMergeTest(worktreeDir, "commit", "-m", "feature change"))

	requireGitMergeTestNoError(t, runGitForMergeTest(repoDir, "merge", "--ff-only", "feature/edited-files"))
	headCommitID := ResolveGitCommitID(repoDir, "HEAD")
	assert.NotEmpty(t, headCommitID)
	assert.NotEqual(t, baseCommitID, headCommitID)

	requireGitMergeTestNoError(t, runGitForMergeTest(repoDir, "worktree", "remove", "--force", worktreeDir))

	editedFiles := CollectEditedFiles(repoDir, worktreeDir, baseCommitID, headCommitID)
	assert.Contains(t, editedFiles, "test.txt")
}

func runGitForMergeTest(workDir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("runGitForMergeTest() [git_merge_test.go]: git %v failed: %w: %s", args, err, string(output))
	}

	return nil
}

func requireGitMergeTestNoError(t *testing.T, err error) {
	t.Helper()
	assert.NoError(t, err)
}
