package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweSessionGetStateTaskNilWhenMissing(t *testing.T) {
	session := NewSweSession(&SweSessionParams{})

	state := session.GetState()
	assert.Nil(t, state.Task)
}

func TestSweSessionGetStateTaskIncludesTaskMetadata(t *testing.T) {
	session := NewSweSession(&SweSessionParams{Task: &Task{UUID: "task-uuid", Name: "task-name", TaskDir: ".cswdata/tasks/task-uuid", Deps: []string{"dep-1"}}})

	state := session.GetState()
	require.NotNil(t, state.Task)
	assert.Equal(t, "task-uuid", state.Task.UUID)
	assert.Equal(t, "task-name", state.Task.Name)
	assert.Equal(t, ".cswdata/tasks/task-uuid", state.Task.TaskDir)

	state.Task.Name = "changed"
	state.Task.Deps[0] = "dep-2"
	state.Task.TaskDir = "other"

	nextState := session.GetState()
	require.NotNil(t, nextState.Task)
	assert.Equal(t, "task-name", nextState.Task.Name)
	assert.Equal(t, []string{"dep-1"}, nextState.Task.Deps)
	assert.Equal(t, ".cswdata/tasks/task-uuid", nextState.Task.TaskDir)
}
