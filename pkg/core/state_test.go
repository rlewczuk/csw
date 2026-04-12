package core

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentStateCloneDeepCopiesRoleAndHookContext(t *testing.T) {
	original := AgentState{
		Role: &conf.AgentRoleConfig{Name: "developer", Aliases: []string{"dev"}},
		Task: &Task{UUID: "task-1", Name: "task-name", TaskDir: ".cswdata/tasks/task-1", Deps: []string{"dep-a"}},
		HookContext: HookContext{
			"status": "running",
		},
	}

	cloned := original.Clone()
	require.NotNil(t, cloned.Role)
	require.NotNil(t, cloned.Task)
	require.NotNil(t, cloned.HookContext)

	cloned.Role.Name = "reviewer"
	cloned.Task.Name = "changed"
	cloned.Task.Deps[0] = "dep-b"
	cloned.Task.TaskDir = "other"
	cloned.HookContext["status"] = "success"

	assert.Equal(t, "developer", original.Role.Name)
	assert.Equal(t, "task-name", original.Task.Name)
	assert.Equal(t, []string{"dep-a"}, original.Task.Deps)
	assert.Equal(t, ".cswdata/tasks/task-1", original.Task.TaskDir)
	assert.Equal(t, "running", original.HookContext["status"])
}

func TestAgentStateHookContextHelpers(t *testing.T) {
	state := AgentState{}

	state.SetHookContextValue("alpha", "one")
	state.SetHookContextValue("", "ignored")
	state.MergeHookContext(map[string]string{"beta": "two", "  ": "ignored"})

	copyData := state.HookContextData()
	copyData["alpha"] = "changed"

	assert.Equal(t, "one", state.HookContext["alpha"])
	assert.Equal(t, "two", state.HookContext["beta"])
	_, exists := state.HookContext["  "]
	assert.False(t, exists)
}
