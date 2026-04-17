package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderCommandPromptFileReference(t *testing.T) {
	workDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "file.txt"), []byte("abc"), 0644))

	params := &RunParams{CommandName: "review", CommandTemplate: "Read @file.txt", CommandArgs: []string{}}
	err := renderCommandPrompt(params, workDir, runner.NewMockRunner(), runner.NewMockRunner())
	require.NoError(t, err)
	assert.Equal(t, "Read abc", params.Prompt)
}
