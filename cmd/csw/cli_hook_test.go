package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCliHookFlagPropagation(t *testing.T) {
	originalRun := runCLIFunc
	t.Cleanup(func() {
		runCLIFunc = originalRun
	})

	captured := ""
	runCLIFunc = func(params *CLIParams) error {
		captured = fmt.Sprintf("hooks=%v", params.HookOverrides)
		return nil
	}

	cmd := CliCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--hook=commit", "--hook=merge:disable", "prompt"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, captured, "hooks=[commit merge:disable]")
}
