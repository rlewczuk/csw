package system

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunNoRefreshFlagPropagation(t *testing.T) {
	originalRun := runCommandFunc
	t.Cleanup(func() {
		runCommandFunc = originalRun
	})

	var captured bool
	runCommandFunc = func(params *RunParams) error {
		captured = params.NoRefresh
		return nil
	}

	cmd := RunCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--no-refresh", "prompt"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.True(t, captured)
}
