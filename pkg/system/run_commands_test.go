package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/conf"
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
		assert.Contains(t, resolved.CommandTemplate, "Analyze and edit current task description")
		assert.Equal(t, "/csw/task-critic", resolved.Prompt)
		assert.Empty(t, resolved.ExtraPositionalArgs)
	})
}

func TestBuildRunAgentStartupInfoMessages(t *testing.T) {
	t.Run("builds startup lines without command", func(t *testing.T) {
		messages := BuildRunAgentStartupInfoMessages(&RunParams{Thinking: "high", RoleName: "developer"}, BuildSystemResult{ModelName: "ollama/qwen3", RoleConfig: conf.AgentRoleConfig{Name: "developer"}})

		require.Len(t, messages, 3)
		assert.Equal(t, "[INFO] Model: ollama/qwen3", messages[0])
		assert.Equal(t, "[INFO] Thinking: high", messages[1])
		assert.Equal(t, "[INFO] Role: developer", messages[2])
	})

	t.Run("includes command with embedded source", func(t *testing.T) {
		messages := BuildRunAgentStartupInfoMessages(
			&RunParams{Thinking: "", RoleName: "", CommandName: "csw/task-critic", CommandPath: "embedded:data/csw/task-critic.md"},
			BuildSystemResult{ModelName: "ollama/qwen3", RoleConfig: conf.AgentRoleConfig{}},
		)

		require.Len(t, messages, 4)
		assert.Equal(t, "[INFO] Model: ollama/qwen3", messages[0])
		assert.Equal(t, "[INFO] Thinking: -", messages[1])
		assert.Equal(t, "[INFO] Role: -", messages[2])
		assert.Equal(t, "[INFO] Command: /csw/task-critic source=embedded", messages[3])
	})
}

func TestBuildRunCommandStartupInfoMessage(t *testing.T) {
	tests := []struct {
		name        string
		commandName string
		commandPath string
		expected    string
	}{
		{name: "empty command name", commandName: "", commandPath: "embedded:data/csw/task-critic.md", expected: ""},
		{name: "embedded command", commandName: "csw/task-critic", commandPath: "embedded:data/csw/task-critic.md", expected: "[INFO] Command: /csw/task-critic source=embedded"},
		{name: "local command", commandName: "my/command", commandPath: "/tmp/project/.agents/commands/my/command.md", expected: "[INFO] Command: /my/command source=.agents/commands"},
		{name: "custom command path", commandName: "my/command", commandPath: "/tmp/project/commands/my/command.md", expected: "[INFO] Command: /my/command source=custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := BuildRunCommandStartupInfoMessage(tt.commandName, tt.commandPath)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
