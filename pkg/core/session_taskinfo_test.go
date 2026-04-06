package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweSessionGetStateTaskInfoNilWhenMissing(t *testing.T) {
	session := NewSweSession(&SweSessionParams{})

	state := session.GetState()
	assert.Nil(t, state.TaskInfo)
}

func TestSweSessionGetStateTaskInfoIncludesTaskMetadata(t *testing.T) {
	session := NewSweSession(&SweSessionParams{TaskInfo: &TaskInfo{
		Task:    &Task{UUID: "task-uuid", Name: "task-name", Deps: []string{"dep-1"}},
		TaskDir: ".csw/tasks/task-uuid",
	}})

	state := session.GetState()
	require.NotNil(t, state.TaskInfo)
	require.NotNil(t, state.TaskInfo.Task)
	assert.Equal(t, "task-uuid", state.TaskInfo.Task.UUID)
	assert.Equal(t, "task-name", state.TaskInfo.Task.Name)
	assert.Equal(t, ".csw/tasks/task-uuid", state.TaskInfo.TaskDir)

	state.TaskInfo.Task.Name = "changed"
	state.TaskInfo.Task.Deps[0] = "dep-2"
	state.TaskInfo.TaskDir = "other"

	nextState := session.GetState()
	require.NotNil(t, nextState.TaskInfo)
	require.NotNil(t, nextState.TaskInfo.Task)
	assert.Equal(t, "task-name", nextState.TaskInfo.Task.Name)
	assert.Equal(t, []string{"dep-1"}, nextState.TaskInfo.Task.Deps)
	assert.Equal(t, ".csw/tasks/task-uuid", nextState.TaskInfo.TaskDir)
}
