package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/vcs"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// taskTestRunner is a test double for TaskSessionRunner.
type taskTestRunner struct {
	calls int
}

// RunTaskSession records invocation count and returns static result.
func (r *taskTestRunner) RunTaskSession(ctx context.Context, request TaskSessionRunRequest) (TaskSessionRunResult, error) {
	_ = ctx
	_ = request
	r.calls++
	now := time.Now().UTC()
	return TaskSessionRunResult{
		SessionID:   "ses-1",
		SummaryText: "summary",
		StartedAt:   now,
		CompletedAt: now,
	}, nil
}

func TestTaskManagerCreateTaskAllowsEmptyPrompt(t *testing.T) {
	baseDir := t.TempDir()
	runner := &taskTestRunner{}
	manager, err := NewTaskManager(baseDir, nil, runner)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task-name"})
	require.NoError(t, err)
	require.NotNil(t, created)

	taskPromptPath := filepath.Join(baseDir, ".csw", "tasks", created.UUID, "task.md")
	contents, err := os.ReadFile(taskPromptPath)
	require.NoError(t, err)
	assert.Empty(t, string(contents))
}

func TestTaskManagerRunTaskFailsWhenTaskPromptIsEmpty(t *testing.T) {
	baseDir := t.TempDir()
	runner := &taskTestRunner{}
	manager, err := NewTaskManager(baseDir, nil, runner)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "empty-task"})
	require.NoError(t, err)

	nullVCS, err := vcs.NewNullVFS(vfs.NewMockVFS())
	require.NoError(t, err)

	outcome, err := manager.RunTask(context.Background(), TaskLookup{Identifier: created.UUID}, TaskRunParams{}, nullVCS)
	require.Error(t, err)
	assert.Nil(t, outcome)
	assert.Contains(t, err.Error(), "task is empty")
	assert.Contains(t, err.Error(), "task.md has no prompt")
	assert.Equal(t, 0, runner.calls)
}
