package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTaskIdentifierFromPosition(t *testing.T) {
	t.Run("returns not recognized for empty candidate", func(t *testing.T) {
		resolved, recognized, err := resolveTaskIdentifierFromPosition(nil, "   ")
		require.NoError(t, err)
		assert.False(t, recognized)
		assert.Equal(t, "", resolved)
	})

	t.Run("recognizes UUID candidate", func(t *testing.T) {
		resolved, recognized, err := resolveTaskIdentifierFromPosition(nil, "01234567-89ab-cdef-0123-456789abcdef")
		require.NoError(t, err)
		assert.True(t, recognized)
		assert.Equal(t, "01234567-89ab-cdef-0123-456789abcdef", resolved)
	})

	t.Run("recognizes exact feature branch", func(t *testing.T) {
		manager, err := core.NewTaskManager(t.TempDir(), nil)
		require.NoError(t, err)

		taskData, err := manager.CreateTask(core.TaskCreateParams{Name: "task-one", FeatureBranch: "feature/run-task", Prompt: "prompt"})
		require.NoError(t, err)

		resolved, recognized, err := resolveTaskIdentifierFromPosition(manager, "feature/run-task")
		require.NoError(t, err)
		assert.True(t, recognized)
		assert.Equal(t, taskData.UUID, resolved)
	})

	t.Run("returns not recognized when no feature branch matches", func(t *testing.T) {
		manager, err := core.NewTaskManager(t.TempDir(), nil)
		require.NoError(t, err)

		_, err = manager.CreateTask(core.TaskCreateParams{Name: "task-one", FeatureBranch: "feature/other", Prompt: "prompt"})
		require.NoError(t, err)

		resolved, recognized, err := resolveTaskIdentifierFromPosition(manager, "feature/missing")
		require.NoError(t, err)
		assert.False(t, recognized)
		assert.Equal(t, "", resolved)
	})

	t.Run("returns error when multiple tasks share the same feature branch", func(t *testing.T) {
		manager, err := core.NewTaskManager(t.TempDir(), nil)
		require.NoError(t, err)

		_, err = manager.CreateTask(core.TaskCreateParams{Name: "task-one", FeatureBranch: "feature/shared", Prompt: "prompt"})
		require.NoError(t, err)
		_, err = manager.CreateTask(core.TaskCreateParams{Name: "task-two", FeatureBranch: "feature/shared", Prompt: "prompt"})
		require.NoError(t, err)

		resolved, recognized, err := resolveTaskIdentifierFromPosition(manager, "feature/shared")
		require.Error(t, err)
		assert.False(t, recognized)
		assert.Equal(t, "", resolved)
		assert.Contains(t, err.Error(), "multiple tasks match feature branch")
	})
}

func TestResolveTaskRunIdentifierForRun(t *testing.T) {
	t.Run("returns error when both --last and --next are set", func(t *testing.T) {
		identifier, err := resolveTaskRunIdentifierForRun(nil, "", true, true)
		require.Error(t, err)
		assert.Equal(t, "", identifier)
	})

	t.Run("returns error when identifier is used with --last", func(t *testing.T) {
		identifier, err := resolveTaskRunIdentifierForRun(nil, "task-id", true, false)
		require.Error(t, err)
		assert.Equal(t, "", identifier)
	})

	t.Run("returns identifier when explicit identifier is provided", func(t *testing.T) {
		identifier, err := resolveTaskRunIdentifierForRun(nil, " task-id ", false, false)
		require.NoError(t, err)
		assert.Equal(t, "task-id", identifier)
	})

	t.Run("returns error when identifier is empty without --last/--next", func(t *testing.T) {
		identifier, err := resolveTaskRunIdentifierForRun(nil, " ", false, false)
		require.Error(t, err)
		assert.Equal(t, "", identifier)
	})
}

func TestResolveRunTaskDirPath(t *testing.T) {
	t.Run("uses task-dir flag value when present", func(t *testing.T) {
		workDir := t.TempDir()
		cmd := &cobra.Command{Use: "run"}
		cmd.Flags().String("task-dir", "", "")
		require.NoError(t, cmd.Flags().Set("task-dir", "custom/tasks"))

		resolvedTaskDir, err := resolveRunTaskDirPath(cmd, workDir, "", "", "")
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean(filepath.Join(workDir, "custom/tasks")), resolvedTaskDir)
	})

	t.Run("falls back to default task directory when flag is empty", func(t *testing.T) {
		workDir := t.TempDir()
		projectConfigDir := filepath.Join(workDir, "project-config")
		require.NoError(t, os.MkdirAll(projectConfigDir, 0o755))

		cmd := &cobra.Command{Use: "run"}
		cmd.Flags().String("task-dir", "", "")

		resolvedTaskDir, err := resolveRunTaskDirPath(cmd, workDir, "", projectConfigDir, "")
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean(filepath.Join(workDir, ".cswdata/tasks")), resolvedTaskDir)
	})

	t.Run("uses defaults task dir from shadow config", func(t *testing.T) {
		workDir := t.TempDir()
		shadowDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(shadowDir, ".csw", "config"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(shadowDir, ".csw", "config", "global.json"), []byte(`{
			"defaults": {
				"task-dir": ".cswdata/tasks-shadow"
			}
		}`), 0o644))

		cmd := &cobra.Command{Use: "run"}
		cmd.Flags().String("task-dir", "", "")

		resolvedTaskDir, err := resolveRunTaskDirPath(cmd, workDir, shadowDir, "", "")
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean(filepath.Join(workDir, ".cswdata/tasks-shadow")), resolvedTaskDir)
	})
}
