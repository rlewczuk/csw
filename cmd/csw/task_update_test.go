package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskUpdateCommandIncludesExpectedFlags(t *testing.T) {
	command := taskUpdateCommand()
	assert.NotNil(t, command.Flags().Lookup("last"))
	assert.NotNil(t, command.Flags().Lookup("next"))
	assert.NotNil(t, command.Flags().Lookup("status"))
	assert.NotNil(t, command.Flags().Lookup("edit"))
	assert.NotNil(t, command.Flags().Lookup("editor"))
	assert.NotNil(t, command.Flags().Lookup("regen"))
	assert.NotNil(t, command.Flags().Lookup("regen-branch"))
	assert.NotNil(t, command.Flags().Lookup("regen-name"))
	assert.NotNil(t, command.Flags().Lookup("regen-description"))
	assert.Nil(t, command.Flags().Lookup("run"))
}

func TestTaskEditCommandDefaultsToEditMode(t *testing.T) {
	command := taskEditCommand()
	assert.Equal(t, "edit", command.Name())
	editFlag := command.Flags().Lookup("edit")
	require.NotNil(t, editFlag)
	assert.Equal(t, "true", editFlag.DefValue)
}

func TestTaskUpdateCommandArgsValidationWithLastFlag(t *testing.T) {
	command := taskUpdateCommand()

	argsErr := command.Args(command, []string{})
	assert.Error(t, argsErr)

	require.NoError(t, command.Flags().Set("last", "true"))

	assert.NoError(t, command.Args(command, []string{}))

	require.NoError(t, command.Flags().Set("next", "true"))
	lastNextConflictErr := command.Args(command, []string{})
	require.Error(t, lastNextConflictErr)
	assert.Contains(t, lastNextConflictErr.Error(), "--last and --next cannot be used together")

	require.NoError(t, command.Flags().Set("next", "false"))
	conflictErr := command.Args(command, []string{"task-1"})
	require.Error(t, conflictErr)
	assert.Contains(t, conflictErr.Error(), "cannot be used with --last or --next")
}

func TestReadTaskPromptFileReturnsEmptyWhenFileIsMissing(t *testing.T) {
	taskDir := t.TempDir()
	prompt, err := readTaskPromptFile(taskDir)
	require.NoError(t, err)
	assert.Equal(t, "", prompt)
}

func TestEditTaskPromptDetectsPromptChange(t *testing.T) {
	t.Setenv("EDITOR", `sh -c 'printf "new prompt\n" > "$1"' sh`)

	prompt, changed, err := editTaskPrompt(context.Background(), os.Getenv("EDITOR"), "old prompt")
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "new prompt", prompt)
}

func TestEditTaskPromptDetectsNoPromptChange(t *testing.T) {
	t.Setenv("EDITOR", `sh -c 'cat "$1" > /dev/null' sh`)

	prompt, changed, err := editTaskPrompt(context.Background(), os.Getenv("EDITOR"), "same prompt")
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, "same prompt", prompt)
}

func TestResolveTaskRunIdentifierReturnsProvidedIdentifierWhenNotUsingLastFlag(t *testing.T) {
	resolved, err := resolveTaskRunIdentifier(nil, " task-123 ", false, false)
	require.NoError(t, err)
	assert.Equal(t, "task-123", resolved)
}

func TestResolveTaskRunIdentifierReturnsNameLastWhenLastFlagIsNotUsed(t *testing.T) {
	resolved, err := resolveTaskRunIdentifier(nil, " last ", false, false)
	require.NoError(t, err)
	assert.Equal(t, "last", resolved)
}

func TestResolveTaskRunIdentifierResolvesLastUnfinishedTaskByMostRecentModTime(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	oldCompleted, err := manager.CreateTask(core.TaskCreateParams{Name: "completed", Prompt: "prompt"})
	require.NoError(t, err)
	runnableOlder, err := manager.CreateTask(core.TaskCreateParams{Name: "runnable-older", Prompt: "prompt"})
	require.NoError(t, err)
	runnableNewest, err := manager.CreateTask(core.TaskCreateParams{Name: "runnable-newest", Prompt: "prompt"})
	require.NoError(t, err)
	draftNewest, err := manager.CreateTask(core.TaskCreateParams{Name: "draft-newest", Prompt: "prompt"})
	require.NoError(t, err)
	runningTask, err := manager.CreateTask(core.TaskCreateParams{Name: "running", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", oldCompleted.UUID, "task.yml"), core.TaskStatusMerged)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableOlder.UUID, "task.yml"), core.TaskStatusOpen)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableNewest.UUID, "task.yml"), core.TaskStatusCreated)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", draftNewest.UUID, "task.yml"), core.TaskStatusDraft)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runningTask.UUID, "task.yml"), core.TaskStatusRunning)

	baseTime := time.Date(2026, time.February, 1, 10, 0, 0, 0, time.UTC)
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", oldCompleted.UUID, "task.yml"), baseTime.Add(1*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableOlder.UUID, "task.yml"), baseTime.Add(2*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableNewest.UUID, "task.yml"), baseTime.Add(3*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", draftNewest.UUID, "task.yml"), baseTime.Add(5*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runningTask.UUID, "task.yml"), baseTime.Add(4*time.Minute))

	resolved, err := resolveTaskRunIdentifier(manager, "", true, false)
	require.NoError(t, err)
	assert.Equal(t, runnableNewest.UUID, resolved)
}

func TestResolveTaskRunIdentifierResolvesNextUnfinishedTaskByOldestModTime(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	runnableNewest, err := manager.CreateTask(core.TaskCreateParams{Name: "runnable-newest", Prompt: "prompt"})
	require.NoError(t, err)
	runnableOldest, err := manager.CreateTask(core.TaskCreateParams{Name: "runnable-oldest", Prompt: "prompt"})
	require.NoError(t, err)
	draftOldest, err := manager.CreateTask(core.TaskCreateParams{Name: "draft-oldest", Prompt: "prompt"})
	require.NoError(t, err)
	runningTask, err := manager.CreateTask(core.TaskCreateParams{Name: "running", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableNewest.UUID, "task.yml"), core.TaskStatusOpen)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableOldest.UUID, "task.yml"), core.TaskStatusCreated)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", draftOldest.UUID, "task.yml"), core.TaskStatusDraft)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runningTask.UUID, "task.yml"), core.TaskStatusRunning)

	baseTime := time.Date(2026, time.February, 1, 10, 0, 0, 0, time.UTC)
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableNewest.UUID, "task.yml"), baseTime.Add(3*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableOldest.UUID, "task.yml"), baseTime.Add(1*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", draftOldest.UUID, "task.yml"), baseTime.Add(0*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runningTask.UUID, "task.yml"), baseTime.Add(4*time.Minute))

	resolved, err := resolveTaskRunIdentifier(manager, "", false, true)
	require.NoError(t, err)
	assert.Equal(t, runnableOldest.UUID, resolved)
}

func TestResolveTaskRunIdentifierReturnsErrorWhenLastAndNextAreBothUsed(t *testing.T) {
	_, err := resolveTaskRunIdentifier(nil, "", true, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--last and --next cannot be used together")
}

func TestResolveTaskRunIdentifierReturnsErrorWhenNoUnfinishedTaskExists(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	completedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "completed", Prompt: "prompt"})
	require.NoError(t, err)
	mergedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "merged", Prompt: "prompt"})
	require.NoError(t, err)
	runningTask, err := manager.CreateTask(core.TaskCreateParams{Name: "running", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", completedTask.UUID, "task.yml"), core.TaskStatusMerged)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", mergedTask.UUID, "task.yml"), core.TaskStatusMerged)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runningTask.UUID, "task.yml"), core.TaskStatusRunning)

	_, err = resolveTaskRunIdentifier(manager, "", true, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no unfinished task found")
}
