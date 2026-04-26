package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/testutil/fixture"
	"github.com/stretchr/testify/require"
)

// GitTestFixture provides test fixtures for git repository tests.
type GitTestFixture struct {
	Root         string
	WorktreesDir string
	Repo         apis.VCS
}

// setupGitRepoFixture creates a temporary git repository for testing.
func setupGitRepoFixture(t *testing.T) *GitTestFixture {
	t.Helper()
	return setupGitRepoFixtureWithOptions(t, nil, "", "")
}

// setupGitRepoFixtureWithIdentity creates a temporary git repository for testing with explicit git identity.
func setupGitRepoFixtureWithIdentity(t *testing.T, gitUserName string, gitUserEmail string) *GitTestFixture {
	t.Helper()
	return setupGitRepoFixtureWithOptions(t, nil, gitUserName, gitUserEmail)
}

// setupGitRepoFixtureWithAllowedPaths creates a temporary git repository for testing with allowed paths.
func setupGitRepoFixtureWithAllowedPaths(t *testing.T, allowedPaths []string) *GitTestFixture {
	t.Helper()
	return setupGitRepoFixtureWithOptions(t, allowedPaths, "", "")
}

// setupGitRepoFixtureWithOptions creates a temporary git repository for testing with explicit configuration.
func setupGitRepoFixtureWithOptions(t *testing.T, allowedPaths []string, gitUserName string, gitUserEmail string) *GitTestFixture {
	t.Helper()

	// Create a temporary directory in project root tmp/
	tempDir := fixture.MkProjectTempDir(t, "git-test-*")
	var err error

	// Create a worktrees directory
	worktreesDir := filepath.Join(tempDir, "worktrees")
	err = os.MkdirAll(worktreesDir, 0o755)
	require.NoError(t, err, "Failed to create temp directory")

	// Initialize a git repository
	_, err = runGitCommand(tempDir, "init")
	require.NoError(t, err, "Failed to initialize git repository")

	// Create an initial file
	initialFile := filepath.Join(tempDir, "README.md")
	err = os.WriteFile(initialFile, []byte("# Initial Commit\n"), 0644)
	require.NoError(t, err, "Failed to create initial file")

	_, err = runGitCommand(tempDir, "add", "README.md")
	require.NoError(t, err, "Failed to add file")

	_, err = runGitCommand(tempDir, "commit", "-m", "Initial commit")
	require.NoError(t, err, "Failed to create initial commit")

	// Create the GitVCS with worktrees directory
	gitRepo, err := NewGitRepo(tempDir, worktreesDir, nil, allowedPaths, gitUserName, gitUserEmail)
	require.NoError(t, err, "Failed to create GitVCS")

	return &GitTestFixture{
		Root:         tempDir,
		WorktreesDir: worktreesDir,
		Repo:         gitRepo,
	}
}

// Cleanup removes the temporary directory.
func (f *GitTestFixture) Cleanup() {
	if f.Root != "" {
		_ = os.RemoveAll(f.Root)
	}
}

// runTestWithGitVCS runs a test function with GitVCS only.
func runTestWithGitVCS(t *testing.T, testFunc func(*testing.T, *GitTestFixture)) {
	t.Run("GitVCS", func(t *testing.T) {
		fixture := setupGitRepoFixture(t)
		defer fixture.Cleanup()
		testFunc(t, fixture)
	})
}

// getDefaultBranch returns the default branch name (master or main).
func getDefaultBranch(t *testing.T, fixture *GitTestFixture) string {
	t.Helper()

	branch, err := runGitCommand(fixture.Root, "branch", "--show-current")
	require.NoError(t, err)

	branch = strings.TrimSpace(branch)
	if branch == "" {
		t.Fatal("getDefaultBranch() [git_vcs_fixture_test.go]: empty current branch")
	}

	return branch
}

// runGitCommand executes git with the provided arguments in a repository path.
func runGitCommand(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	cmd.Env = gitTestCommandEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func gitTestCommandEnv() []string {
	env := os.Environ()
	env = upsertEnvValue(env, "GIT_AUTHOR_NAME", "Test")
	env = upsertEnvValue(env, "GIT_AUTHOR_EMAIL", "test@example.com")
	env = upsertEnvValue(env, "GIT_COMMITTER_NAME", "Test")
	env = upsertEnvValue(env, "GIT_COMMITTER_EMAIL", "test@example.com")
	return env
}
