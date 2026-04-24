package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/commands"
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

func TestResolveRunCommandInvocation_TaskModeUsesCommands(t *testing.T) {
	t.Run("loads local nested command for explicit task run variant", func(t *testing.T) {
		workDir := t.TempDir()
		commandPath := filepath.Join(workDir, ".agents", "commands", "my", "command.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(commandPath), 0o755))
		require.NoError(t, os.WriteFile(commandPath, []byte("Local command template"), 0o644))

		invocation := &commands.Invocation{Name: "my/command", Arguments: []string{"arg-one", "arg-two"}}
		resolved, err := resolveRunCommandInvocation(invocation, workDir, "", true)
		require.NoError(t, err)

		require.NotNil(t, resolved)
		assert.Equal(t, "my/command", resolved.CommandName)
		assert.Equal(t, "Local command template", resolved.CommandTemplate)
		assert.Equal(t, []string{"arg-one", "arg-two"}, resolved.CommandArgs)
		assert.Equal(t, "/my/command", resolved.Prompt)
		assert.Equal(t, []string{"arg-one", "arg-two"}, resolved.ExtraPositionalArgs)
	})

	t.Run("loads embedded command for --next/--last task run variant", func(t *testing.T) {
		invocation := &commands.Invocation{Name: "csw/task-critic", Arguments: nil}
		resolved, err := resolveRunCommandInvocation(invocation, t.TempDir(), "", true)
		require.NoError(t, err)

		require.NotNil(t, resolved)
		assert.Equal(t, "csw/task-critic", resolved.CommandName)
		assert.Contains(t, resolved.CommandTemplate, "Analyze and edit task")
		assert.Equal(t, "/csw/task-critic", resolved.Prompt)
		assert.Empty(t, resolved.ExtraPositionalArgs)
	})
}
