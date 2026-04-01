package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatEditedFilesSummaryUsesWorktreeDir verifies edited files are collected from active worktree.
func TestFormatEditedFilesSummaryUsesWorktreeDir(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, runGitInDir(repoDir, "init", "-b", "main"))
	require.NoError(t, runGitInDir(repoDir, "config", "user.name", "Test User"))
	require.NoError(t, runGitInDir(repoDir, "config", "user.email", "test@example.com"))

	targetFile := filepath.Join(repoDir, "test.txt")
	require.NoError(t, os.WriteFile(targetFile, []byte("old\n"), 0644))
	require.NoError(t, runGitInDir(repoDir, "add", "test.txt"))
	require.NoError(t, runGitInDir(repoDir, "commit", "-m", "initial"))

	require.NoError(t, runGitInDir(repoDir, "branch", "feature/summary"))
	worktreeDir := filepath.Join(repoDir, ".cswdata", "work", "feature", "summary")
	require.NoError(t, os.MkdirAll(filepath.Dir(worktreeDir), 0755))
	require.NoError(t, runGitInDir(repoDir, "worktree", "add", worktreeDir, "feature/summary"))

	require.NoError(t, os.WriteFile(filepath.Join(worktreeDir, "test.txt"), []byte("old\nnew\n"), 0644))

	summary := system.FormatEditedFilesSummary(repoDir, worktreeDir)
	assert.NotEqual(t, "-", summary)
	assert.Contains(t, summary, "test.txt")
}

// TestFormatEditedFilesSummaryIncludesUntrackedFiles verifies new untracked files are listed.
func TestFormatEditedFilesSummaryIncludesUntrackedFiles(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, runGitInDir(repoDir, "init", "-b", "main"))

	newFile := filepath.Join(repoDir, "new.txt")
	require.NoError(t, os.WriteFile(newFile, []byte("content\n"), 0644))

	summary := system.FormatEditedFilesSummary(repoDir, repoDir)
	assert.NotEqual(t, "-", summary)
	assert.Contains(t, summary, "new.txt (new file)")
}

func runGitInDir(workDir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("runGitInDir() [cli_summary_worktree_test.go]: git %v failed: %w: %s", args, err, string(output))
	}

	return nil
}
