package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskArchiveCommandIncludesStatusFlag(t *testing.T) {
	command := taskArchiveCommand()
	assert.NotNil(t, command.Flags().Lookup("status"))
}

func TestRunTaskArchiveConflictingArguments(t *testing.T) {
	manager, err := core.NewTaskManager(t.TempDir(), nil)
	require.NoError(t, err)

	err = runTaskArchive(manager, []string{"task-id"}, "merged", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task identifier and --status cannot be used together")
}

func TestRunTaskArchiveByIdentifier(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(core.TaskCreateParams{Name: "archive-me", Prompt: "prompt"})
	require.NoError(t, err)

	buffer := &bytes.Buffer{}
	err = runTaskArchive(manager, []string{created.UUID}, "", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), "Task archived: "+created.UUID)

	archivedPath := filepath.Join(baseDir, ".cswdata", "tasks", "archive", created.UUID, "task.yml")
	_, statErr := os.Stat(archivedPath)
	require.NoError(t, statErr)
}

func TestRunTaskArchiveDefaultStatusMerged(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	mergedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "merged", Prompt: "prompt"})
	require.NoError(t, err)
	openTask, err := manager.CreateTask(core.TaskCreateParams{Name: "open", Prompt: "prompt"})
	require.NoError(t, err)
	draftTask, err := manager.CreateTask(core.TaskCreateParams{Name: "draft", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", mergedTask.UUID, "task.yml"), core.TaskStatusMerged)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", openTask.UUID, "task.yml"), core.TaskStatusOpen)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", draftTask.UUID, "task.yml"), core.TaskStatusDraft)

	buffer := &bytes.Buffer{}
	err = runTaskArchive(manager, nil, "", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), mergedTask.UUID)
	assert.NotContains(t, buffer.String(), openTask.UUID)
	assert.NotContains(t, buffer.String(), draftTask.UUID)

	_, mergedErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", "archive", mergedTask.UUID, "task.yml"))
	require.NoError(t, mergedErr)
	_, openErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", openTask.UUID, "task.yml"))
	require.NoError(t, openErr)
	_, draftErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", draftTask.UUID, "task.yml"))
	require.NoError(t, draftErr)
}

func TestRunTaskArchiveByStatusFailed(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	failedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "failed", Prompt: "prompt"})
	require.NoError(t, err)
	otherTask, err := manager.CreateTask(core.TaskCreateParams{Name: "other", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", failedTask.UUID, "task.yml"), "failed")
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", otherTask.UUID, "task.yml"), core.TaskStatusOpen)

	buffer := &bytes.Buffer{}
	err = runTaskArchive(manager, nil, "failed", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), failedTask.UUID)
	assert.NotContains(t, buffer.String(), otherTask.UUID)

	_, failedErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", "archive", failedTask.UUID, "task.yml"))
	require.NoError(t, failedErr)
	_, otherErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", otherTask.UUID, "task.yml"))
	require.NoError(t, otherErr)
}
