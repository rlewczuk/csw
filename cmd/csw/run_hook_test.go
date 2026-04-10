package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHookFlagPropagation(t *testing.T) {
	originalRun := runFunc
	t.Cleanup(func() {
		runFunc = originalRun
	})

	captured := ""
	runFunc = func(params *RunParams) error {
		captured = fmt.Sprintf("hooks=%v", params.HookOverrides)
		return nil
	}

	cmd := RunCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--hook=commit", "--hook=merge:disable", "prompt"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, captured, "hooks=[commit merge:disable]")
}

func TestRunNoRefreshFlagPropagation(t *testing.T) {
	originalRun := runFunc
	t.Cleanup(func() {
		runFunc = originalRun
	})

	var captured bool
	runFunc = func(params *RunParams) error {
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
