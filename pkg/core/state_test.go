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
	}

	cloned := original.Clone()
	require.NotNil(t, cloned.Role)
	require.NotNil(t, cloned.Task)

	cloned.Role.Name = "reviewer"
	cloned.Task.Name = "changed"
	cloned.Task.Deps[0] = "dep-b"
	cloned.Task.TaskDir = "other"

	assert.Equal(t, "developer", original.Role.Name)
	assert.Equal(t, "task-name", original.Task.Name)
	assert.Equal(t, []string{"dep-a"}, original.Task.Deps)
	assert.Equal(t, ".cswdata/tasks/task-1", original.Task.TaskDir)
}
