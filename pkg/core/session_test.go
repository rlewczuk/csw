package core

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRunExecutionKeepsProvidedConfigReference(t *testing.T) {
	config := &conf.CswConfig{GlobalConfig: &conf.GlobalConfig{}}

	execution := NewRunExecution(config, nil, nil, nil)

	require.NotNil(t, execution)
	assert.Same(t, config, execution.Config)
}

func TestNewSweSessionKeepsProvidedRunExecutionReference(t *testing.T) {
	config := &conf.CswConfig{GlobalConfig: &conf.GlobalConfig{}}
	task := &Task{UUID: "task-id", Name: "task name"}
	execution := &RunExecution{Config: config, Task: task}

	session := NewSweSession(&SweSessionParams{Execution: execution})

	require.NotNil(t, session)
	assert.Same(t, execution, session.Execution)
	assert.Same(t, config, session.configValue())
	assert.Same(t, task, session.task())
	assert.Equal(t, task, session.GetState().Task)
}
