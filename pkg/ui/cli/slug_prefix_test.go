package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddCLISlugPrefix(t *testing.T) {
	t.Run("uses session slug for regular lines", func(t *testing.T) {
		result := addCLISlugPrefix("session-branch-name", "Assistant: hello")

		assert.Equal(t, "\x1b[90m[session-branch-name]\x1b[0m Assistant: hello", result)
	})

	t.Run("subagent slug replaces session slug", func(t *testing.T) {
		result := addCLISlugPrefix("session-branch-name", "*child-agent* Assistant: hello")

		assert.Equal(t, "\x1b[90m[child-agent]\x1b[0m Assistant: hello", result)
	})

	t.Run("keeps mixed multiline output correct", func(t *testing.T) {
		message := "line one\n*sub-agent* line two\n\nline three"
		result := addCLISlugPrefix("session-branch-name", message)

		expected := "\x1b[90m[session-branch-name]\x1b[0m line one\n\x1b[90m[sub-agent]\x1b[0m line two\n\n\x1b[90m[session-branch-name]\x1b[0m line three"
		assert.Equal(t, expected, result)
	})
}
