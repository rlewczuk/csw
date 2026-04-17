package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestApplyCommandTaskMetadataAppliesValuesWhenTaskUnchanged(t *testing.T) {
	taskDir := t.TempDir()
	initialTask := &core.Task{
		UUID:          "task-1",
		Status:        core.TaskStatusCreated,
		Role:          "developer",
		FeatureBranch: "feature/one",
	}
	bytesData, err := yaml.Marshal(initialTask)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(taskDir, "task.yml"), bytesData, 0644))

	params := &RunParams{
		Task:        &core.Task{TaskDir: taskDir},
		InitialTask: cloneRunTask(initialTask),
		CommandTaskMetadata: &commands.TaskMetadata{
			Status: statusPtr(core.TaskStatusMerged),
			Role:   statusPtr("reviewer"),
		},
	}

	require.NoError(t, applyCommandTaskMetadata(params))

	updatedBytes, err := os.ReadFile(filepath.Join(taskDir, "task.yml"))
	require.NoError(t, err)
	var updated core.Task
	require.NoError(t, yaml.Unmarshal(updatedBytes, &updated))
	assert.Equal(t, core.TaskStatusMerged, updated.Status)
	assert.Equal(t, "reviewer", updated.Role)
}

func TestApplyCommandTaskMetadataPreservesInSessionTaskUpdates(t *testing.T) {
	taskDir := t.TempDir()
	initialTask := &core.Task{
		UUID:          "task-1",
		Status:        core.TaskStatusCreated,
		Role:          "developer",
		FeatureBranch: "feature/one",
	}
	persistedTask := *initialTask
	persistedTask.Status = core.TaskStatusRunning

	bytesData, err := yaml.Marshal(&persistedTask)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(taskDir, "task.yml"), bytesData, 0644))

	params := &RunParams{
		Task:        &core.Task{TaskDir: taskDir},
		InitialTask: cloneRunTask(initialTask),
		CommandTaskMetadata: &commands.TaskMetadata{
			Status: statusPtr(core.TaskStatusMerged),
			Role:   statusPtr("reviewer"),
		},
	}

	require.NoError(t, applyCommandTaskMetadata(params))

	updatedBytes, err := os.ReadFile(filepath.Join(taskDir, "task.yml"))
	require.NoError(t, err)
	var updated core.Task
	require.NoError(t, yaml.Unmarshal(updatedBytes, &updated))
	assert.Equal(t, core.TaskStatusRunning, updated.Status)
	assert.Equal(t, "reviewer", updated.Role)
}

func statusPtr(value string) *string {
	return &value
}
