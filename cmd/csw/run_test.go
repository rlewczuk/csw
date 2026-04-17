package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunCommandDoesNotExposeTaskFlag(t *testing.T) {
	command := RunCommand()
	assert.Nil(t, command.Flags().Lookup("task"))
}
