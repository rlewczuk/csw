package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskNewCommandPromptFlagIsOptional(t *testing.T) {
	command := taskNewCommand()
	promptFlag := command.Flags().Lookup("prompt")
	require.NotNil(t, promptFlag)
	_, required := promptFlag.Annotations[cobra.BashCompOneRequiredFlag]
	assert.False(t, required)
}
