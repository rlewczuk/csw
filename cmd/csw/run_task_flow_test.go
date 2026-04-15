package main

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommandTaskModeUsesUnifiedRunFlowAndTaskPrompt(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)
	createdTask, err := manager.CreateTask(core.TaskCreateParams{
		Name:          "task-a",
		Prompt:        "prompt from task file",
		FeatureBranch: "feature/task-a",
		ParentBranch:  "main",
	})
	require.NoError(t, err)

	originalRun := runFunc
	originalDefaults := resolveRunDefaultsFunc
	originalTaskBackendLoader := loadTaskBackendFunc
	t.Cleanup(func() {
		runFunc = originalRun
		resolveRunDefaultsFunc = originalDefaults
		loadTaskBackendFunc = originalTaskBackendLoader
	})

	resolveRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{Model: "provider/default"}, nil
	}
	loadTaskBackendFunc = func(cmd *cobra.Command) (*core.TaskManager, *core.TaskBackendAdapter, error) {
		_ = cmd
		return manager, nil, nil
	}

	var captured *RunParams
	runFunc = func(params *RunParams) error {
		captured = params
		_, taskData, resolveErr := manager.ResolveTask(core.TaskLookup{Identifier: createdTask.UUID})
		require.NoError(t, resolveErr)
		assert.Equal(t, core.TaskStatusRunning, taskData.Status)
		return nil
	}

	cmd := RunCommand()
	cmd.SetArgs([]string{"--task", createdTask.UUID})

	err = cmd.Execute()
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, "prompt from task file", captured.Prompt)
	require.NotNil(t, captured.Task)
	assert.Equal(t, createdTask.UUID, captured.Task.UUID)
	assert.Equal(t, core.TaskStatusCreated, captured.InitialTask.Status)

	_, finalTask, err := manager.ResolveTask(core.TaskLookup{Identifier: createdTask.UUID})
	require.NoError(t, err)
	assert.Equal(t, core.TaskStatusCompleted, finalTask.Status)
}

func TestRunCommandTaskModeSetsMergedStatusWhenMergeRequested(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)
	createdTask, err := manager.CreateTask(core.TaskCreateParams{
		Name:          "task-merge",
		Prompt:        "merge prompt",
		FeatureBranch: "feature/task-merge",
		ParentBranch:  "main",
	})
	require.NoError(t, err)

	originalRun := runFunc
	originalDefaults := resolveRunDefaultsFunc
	originalTaskBackendLoader := loadTaskBackendFunc
	t.Cleanup(func() {
		runFunc = originalRun
		resolveRunDefaultsFunc = originalDefaults
		loadTaskBackendFunc = originalTaskBackendLoader
	})

	resolveRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{Model: "provider/default"}, nil
	}
	loadTaskBackendFunc = func(cmd *cobra.Command) (*core.TaskManager, *core.TaskBackendAdapter, error) {
		_ = cmd
		return manager, nil, nil
	}

	var captured *RunParams
	runFunc = func(params *RunParams) error {
		captured = params
		return nil
	}

	cmd := RunCommand()
	cmd.SetArgs([]string{"--task", createdTask.UUID, "--merge"})

	err = cmd.Execute()
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.True(t, captured.Merge)
	assert.Equal(t, createdTask.FeatureBranch, captured.WorktreeBranch)

	_, finalTask, err := manager.ResolveTask(core.TaskLookup{Identifier: createdTask.UUID})
	require.NoError(t, err)
	assert.Equal(t, core.TaskStatusMerged, finalTask.Status)
}
