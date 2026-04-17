package system

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
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
