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
