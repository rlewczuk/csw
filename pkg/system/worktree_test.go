package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveResumeTargetToSessionID(t *testing.T) {
	tmpRoot := filepath.Join("..", "..", "tmp", "cli_resume", t.Name())
	require.NoError(t, os.MkdirAll(tmpRoot, 0755))
	defer os.RemoveAll(tmpRoot)
	absTmpRoot, err := filepath.Abs(tmpRoot)
	require.NoError(t, err)

	logsDir := filepath.Join(absTmpRoot, ".cswdata", "logs")
	sessionsDir := filepath.Join(logsDir, "sessions")
	require.NoError(t, os.MkdirAll(sessionsDir, 0755))

	repoDir := filepath.Join(absTmpRoot, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	worktreeDir := filepath.Join(absTmpRoot, ".cswdata", "work", "0145-feature-work")
	require.NoError(t, os.MkdirAll(worktreeDir, 0755))

	directWorkDir := filepath.Join(repoDir)

	writeState := func(id string, workdir string) {
		sessionPath := filepath.Join(sessionsDir, id)
		require.NoError(t, os.MkdirAll(sessionPath, 0755))
		state := core.PersistedSessionState{
			SessionID:    id,
			ProviderName: "ollama",
			Model:        "test-model",
			WorkDir:      workdir,
		}
		bytes, err := json.Marshal(state)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(sessionPath, "session.json"), bytes, 0644))
	}

	writeState("018f6e30-3acb-7f24-bede-8d96cd157150", directWorkDir)
	writeState("018f6e30-3acb-7f24-bede-8d96cd157151", worktreeDir)
	writeState("018f6e30-3acb-7f24-bede-8d96cd157152", worktreeDir)

	t.Run("uuid passthrough", func(t *testing.T) {
		id, err := ResolveResumeTargetToSessionID("018f6e30-3acb-7f24-bede-8d96cd157150", repoDir, logsDir)
		require.NoError(t, err)
		assert.Equal(t, "018f6e30-3acb-7f24-bede-8d96cd157150", id)
	})

	t.Run("last passthrough", func(t *testing.T) {
		id, err := ResolveResumeTargetToSessionID("last", repoDir, logsDir)
		require.NoError(t, err)
		assert.Equal(t, "last", id)
	})

	t.Run("absolute workdir path resolves to matching session", func(t *testing.T) {
		id, err := ResolveResumeTargetToSessionID(worktreeDir, repoDir, logsDir)
		require.NoError(t, err)
		assert.Equal(t, "018f6e30-3acb-7f24-bede-8d96cd157152", id)
	})

	t.Run("workdir name resolves newest session", func(t *testing.T) {
		id, err := ResolveResumeTargetToSessionID("0145-feature-work", repoDir, logsDir)
		require.NoError(t, err)
		assert.Equal(t, "018f6e30-3acb-7f24-bede-8d96cd157152", id)
	})

	t.Run("branch resolves by worktree name", func(t *testing.T) {
		originalRunGit := vcs.RunGitCommand
		t.Cleanup(func() {
			vcs.RunGitCommand = originalRunGit
		})

		vcs.RunGitCommand = func(workDir string, args ...string) (string, error) {
			cleanWorkDir := filepath.Clean(workDir)
			require.Equal(t, filepath.Clean(repoDir), cleanWorkDir)
			joinedArgs := fmt.Sprintf("%v", args)
			switch joinedArgs {
			case "[show-ref --verify --quiet refs/heads/feature/existing]":
				return "", nil
			case "[worktree list --porcelain]":
				return fmt.Sprintf("worktree %s\nbranch refs/heads/feature/existing\n\n", worktreeDir), nil
			default:
				return "", fmt.Errorf("unexpected git command: %s | %s", cleanWorkDir, joinedArgs)
			}
		}

		id, err := ResolveResumeTargetToSessionID("feature/existing", repoDir, logsDir)
		require.NoError(t, err)
		assert.Equal(t, "018f6e30-3acb-7f24-bede-8d96cd157152", id)
	})
}
