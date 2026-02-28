package godown

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCovertStr(t *testing.T) {
	t.Run("converts html to markdown string", func(t *testing.T) {
		markdown, err := CovertStr("<html><body><h1>Title</h1><p>Hello</p></body></html>", nil)
		require.NoError(t, err)
		assert.Contains(t, markdown, "Title")
		assert.Contains(t, markdown, "Hello")
	})

	t.Run("handles empty input", func(t *testing.T) {
		markdown, err := CovertStr("", nil)
		require.NoError(t, err)
		assert.NotEmpty(t, markdown)
	})
}
