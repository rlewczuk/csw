package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskListCommandIncludesExpectedFlags(t *testing.T) {
	command := taskListCommand()
	assert.NotNil(t, command.Flags().Lookup("recursive"))
	assert.NotNil(t, command.Flags().Lookup("archived"))
	assert.NotNil(t, command.Flags().Lookup("status"))
	assert.Nil(t, command.Flags().Lookup("verbose"))
}

func TestRunTaskListListsCurrentTasksSortedByTaskYMLModTime(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	first, err := manager.CreateTask(core.TaskCreateParams{Name: "first", Prompt: "prompt", Description: "first desc", FeatureBranch: "feature/first"})
	require.NoError(t, err)
	second, err := manager.CreateTask(core.TaskCreateParams{Name: "second", Prompt: "prompt", Description: "second desc", FeatureBranch: "feature/second"})
	require.NoError(t, err)

	newer := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)
	older := time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC)
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", first.UUID, "task.yml"), newer)
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", second.UUID, "task.yml"), older)

	buffer := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, false, "", buffer)
	require.NoError(t, err)

	lines := nonEmptyLines(buffer.String())
	require.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[0], second.UUID+"\t"))
	assert.True(t, strings.HasPrefix(lines[1], first.UUID+"\t"))

	columns := strings.Split(lines[0], "\t")
	require.Len(t, columns, 3)
	assert.Equal(t, second.UUID, columns[0])
	assert.Equal(t, second.Status, columns[1])
	assert.Equal(t, second.Description, columns[2])
}

func TestRunTaskListIncludesArchivedWhenRequested(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	activeTask, err := manager.CreateTask(core.TaskCreateParams{Name: "active", Prompt: "prompt"})
	require.NoError(t, err)
	archivedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "archived", Prompt: "prompt"})
	require.NoError(t, err)
	_, err = manager.ArchiveTask(core.TaskLookup{Identifier: archivedTask.UUID})
	require.NoError(t, err)

	withoutArchived := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, false, "", withoutArchived)
	require.NoError(t, err)
	assert.Contains(t, withoutArchived.String(), activeTask.UUID)
	assert.NotContains(t, withoutArchived.String(), archivedTask.UUID)

	withArchived := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, true, "", withArchived)
	require.NoError(t, err)
	assert.Contains(t, withArchived.String(), activeTask.UUID)
	assert.Contains(t, withArchived.String(), archivedTask.UUID)
}

func TestRunTaskListIgnoresArchiveContainerDirectory(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	activeTask, err := manager.CreateTask(core.TaskCreateParams{Name: "active", Prompt: "prompt"})
	require.NoError(t, err)
	archivedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "archived", Prompt: "prompt"})
	require.NoError(t, err)
	_, err = manager.ArchiveTask(core.TaskLookup{Identifier: archivedTask.UUID})
	require.NoError(t, err)

	buffer := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, false, "", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), activeTask.UUID)
	assert.NotContains(t, buffer.String(), archivedTask.UUID)
}

func TestRunTaskListStatusFiltering(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	openTask, err := manager.CreateTask(core.TaskCreateParams{Name: "open", Prompt: "prompt"})
	require.NoError(t, err)
	mergedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "merged", Prompt: "prompt"})
	require.NoError(t, err)
	createdTask, err := manager.CreateTask(core.TaskCreateParams{Name: "created", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", openTask.UUID, "task.yml"), core.TaskStatusOpen)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", mergedTask.UUID, "task.yml"), core.TaskStatusMerged)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", createdTask.UUID, "task.yml"), core.TaskStatusCreated)

	included := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, false, "open,merged", included)
	require.NoError(t, err)
	assert.Contains(t, included.String(), openTask.UUID)
	assert.Contains(t, included.String(), mergedTask.UUID)
	assert.NotContains(t, included.String(), createdTask.UUID)

	excluded := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, false, "!merged", excluded)
	require.NoError(t, err)
	assert.Contains(t, excluded.String(), openTask.UUID)
	assert.Contains(t, excluded.String(), createdTask.UUID)
	assert.NotContains(t, excluded.String(), mergedTask.UUID)
}
