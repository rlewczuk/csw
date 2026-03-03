package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareSessionVFSWithoutWorktree(t *testing.T) {
	tmpDir := t.TempDir()

	repo, selectedVFS, err := prepareSessionVFS(tmpDir, "", nil, "", "", nil)
	require.NoError(t, err)
	require.NotNil(t, repo)
	require.NotNil(t, selectedVFS)

	_, isNull := repo.(*vfs.NullVCS)
	assert.True(t, isNull)
	assert.Equal(t, tmpDir, selectedVFS.WorktreePath())
}

func TestPrepareSessionVFSWithWorktreeCreatesBranchAndWorktree(t *testing.T) {
	repoDir := initTestGitRepository(t)

	repo, selectedVFS, err := prepareSessionVFS(repoDir, "feature/worktree", nil, "", "", nil)
	require.NoError(t, err)
	require.NotNil(t, repo)
	require.NotNil(t, selectedVFS)

	_, isGit := repo.(*vfs.GitVCS)
	assert.True(t, isGit)

	expectedWorktreePath := filepath.Join(repoDir, ".cswdata", "work", "feature", "worktree")
	assert.Equal(t, expectedWorktreePath, selectedVFS.WorktreePath())

	_, err = selectedVFS.ReadFile("README.md")
	assert.NoError(t, err)

	branchCheck := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", "refs/heads/feature/worktree")
	require.NoError(t, branchCheck.Run())
}

func TestPrepareSessionVFSRecreatesExistingWorktreePath(t *testing.T) {
	repoDir := initTestGitRepository(t)
	stalePath := filepath.Join(repoDir, ".cswdata", "work", "feature-recreate")
	require.NoError(t, os.MkdirAll(stalePath, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(stalePath, "stale.txt"), []byte("stale"), 0644))

	_, selectedVFS, err := prepareSessionVFS(repoDir, "feature-recreate", nil, "", "", nil)
	require.NoError(t, err)
	require.NotNil(t, selectedVFS)

	_, err = os.Stat(filepath.Join(stalePath, "stale.txt"))
	assert.True(t, os.IsNotExist(err))

	_, err = selectedVFS.ReadFile("README.md")
	assert.NoError(t, err)
}

func TestPrepareSessionVFSWithWorktreeRespectsAllowedPaths(t *testing.T) {
	repoDir := initTestGitRepository(t)
	allowedDir := t.TempDir()
	allowedFile := filepath.Join(allowedDir, "allowed.txt")
	require.NoError(t, os.WriteFile(allowedFile, []byte("allowed-content"), 0644))

	_, selectedVFS, err := prepareSessionVFS(repoDir, "feature/vfs-allow", nil, "", "", []string{allowedDir})
	require.NoError(t, err)
	require.NotNil(t, selectedVFS)

	data, err := selectedVFS.ReadFile(allowedFile)
	require.NoError(t, err)
	assert.Equal(t, "allowed-content", string(data))
}

func initTestGitRepository(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()

	initCmd := exec.Command("git", "-C", repoDir, "init", "-b", "main")
	require.NoError(t, initCmd.Run())

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello"), 0644))

	addCmd := exec.Command("git", "-C", repoDir, "add", "README.md")
	require.NoError(t, addCmd.Run())

	commitCmd := exec.Command("git", "-C", repoDir, "commit", "-m", "initial")
	commitCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	require.NoError(t, commitCmd.Run())

	return repoDir
}
