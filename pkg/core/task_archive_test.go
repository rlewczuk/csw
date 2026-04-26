package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskManagerArchiveTaskMovesTaskDirectory(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "archive-me", Prompt: "prompt"})
	require.NoError(t, err)

	archived, err := manager.ArchiveTask(TaskLookup{Identifier: created.UUID})
	require.NoError(t, err)
	require.NotNil(t, archived)
	assert.Equal(t, created.UUID, archived.UUID)

	_, _, resolveErr := manager.ResolveTask(TaskLookup{Identifier: created.UUID})
	require.Error(t, resolveErr)
	assert.Contains(t, resolveErr.Error(), "not found")

	archivedTaskPath := filepath.Join(baseDir, ".cswdata", "tasks", "archive", created.UUID, "task.yml")
	_, statErr := os.Stat(archivedTaskPath)
	require.NoError(t, statErr)
}

func TestTaskManagerArchiveTaskPreservesNestedPath(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	parent, err := manager.CreateTask(TaskCreateParams{Name: "parent", Prompt: "parent"})
	require.NoError(t, err)
	child, err := manager.CreateTask(TaskCreateParams{Name: "child", ParentTaskID: parent.UUID, Prompt: "child"})
	require.NoError(t, err)

	_, err = manager.ArchiveTask(TaskLookup{Identifier: child.Name})
	require.NoError(t, err)

	archivedChildPath := filepath.Join(baseDir, ".cswdata", "tasks", "archive", parent.UUID, child.UUID, "task.yml")
	_, statErr := os.Stat(archivedChildPath)
	require.NoError(t, statErr)
}

func TestTaskManagerListTasksSkipsArchiveContainerDirectory(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	activeTask, err := manager.CreateTask(TaskCreateParams{Name: "active", Prompt: "prompt"})
	require.NoError(t, err)
	archivedTask, err := manager.CreateTask(TaskCreateParams{Name: "archived", Prompt: "prompt"})
	require.NoError(t, err)
	_, err = manager.ArchiveTask(TaskLookup{Identifier: archivedTask.UUID})
	require.NoError(t, err)

	tasks, err := manager.ListTasks(TaskLookup{}, false)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, activeTask.UUID, tasks[0].UUID)
}

func TestTaskManagerArchiveTasksByStatusArchivesMatchingTasks(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	mergedTask, err := manager.CreateTask(TaskCreateParams{Name: "merged-task", FeatureBranch: "feat/merged", ParentBranch: "main", Prompt: "prompt"})
	require.NoError(t, err)
	openTask, err := manager.CreateTask(TaskCreateParams{Name: "open-task", Prompt: "prompt"})
	require.NoError(t, err)

	fake := &fakeVCS{branches: []string{"main", "feat/merged"}}
	_, err = manager.MergeTask(TaskLookup{Identifier: mergedTask.UUID}, fake)
	require.NoError(t, err)

	archivedTasks, err := manager.ArchiveTasksByStatus(TaskStatusMerged)
	require.NoError(t, err)
	require.Len(t, archivedTasks, 1)
	assert.Equal(t, mergedTask.UUID, archivedTasks[0].UUID)

	mergedArchivedPath := filepath.Join(baseDir, ".cswdata", "tasks", "archive", mergedTask.UUID, "task.yml")
	_, mergedArchivedErr := os.Stat(mergedArchivedPath)
	require.NoError(t, mergedArchivedErr)

	openTaskPath := filepath.Join(baseDir, ".cswdata", "tasks", openTask.UUID, "task.yml")
	_, openTaskErr := os.Stat(openTaskPath)
	require.NoError(t, openTaskErr)
}

func TestIsTaskPathNestedUnder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		parent string
		want   bool
	}{
		{name: "equal", path: "/tmp/a", parent: "/tmp/a", want: true},
		{name: "nested", path: "/tmp/a/b", parent: "/tmp/a", want: true},
		{name: "outside", path: "/tmp/ab", parent: "/tmp/a", want: false},
		{name: "dot path", path: ".", parent: "/tmp/a", want: false},
		{name: "dot parent", path: "/tmp/a", parent: ".", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isTaskPathNestedUnder(tc.path, tc.parent))
		})
	}
}
