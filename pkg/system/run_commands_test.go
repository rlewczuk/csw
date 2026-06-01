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

	params := &runExecution{CommandName: "review", CommandTemplate: "Read @file.txt", CommandArgs: []string{}}
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

func TestApplyCommandRunDefaults(t *testing.T) {
	intPtr := func(value int) *int { return &value }

	t.Run("loads yaml command defaults into run defaults config", func(t *testing.T) {
		workDir := t.TempDir()
		commandPath := filepath.Join(workDir, ".agents", "commands", "defaults.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(commandPath), 0o755))
		require.NoError(t, os.WriteFile(commandPath, []byte(`---
csw:
  defaults:
    default-provider: ollama
    default-role: developer
    model: qwen
    container:
      image: golang:latest
      enabled: true
---
Run command
`), 0o644))

		resolved, err := resolveRunCommandInvocation(&commands.Invocation{Name: "defaults"}, workDir, "", false)
		require.NoError(t, err)

		require.NotNil(t, resolved.CommandRunDefaults)
		assert.Equal(t, "ollama", resolved.CommandRunDefaults.DefaultProvider)
		assert.Equal(t, "developer", resolved.CommandRunDefaults.DefaultRole)
		assert.Equal(t, "qwen", resolved.CommandRunDefaults.Model)
		require.NotNil(t, resolved.CommandRunDefaults.Container)
		assert.Equal(t, "golang:latest", resolved.CommandRunDefaults.Container.Image)
		assert.True(t, resolved.CommandRunDefaults.Container.Enabled)
	})

	t.Run("applies command defaults without cobra flags", func(t *testing.T) {
		defaults := &conf.RunDefaultsConfig{Container: &conf.ContainerConfig{}}
		containerOn := false
		containerOff := true

		commandContainerEnabled, err := applyCommandRunDefaults(
			&conf.RunDefaultsConfig{
				DefaultProvider:     " ollama ",
				Model:               " qwen ",
				DefaultRole:         " developer ",
				Workdir:             " . ",
				Worktree:            " feature-branch ",
				Merge:               true,
				LogLLMRequests:      true,
				Thinking:            " high ",
				LSPServer:           " gopls ",
				GitUserName:         " User ",
				GitUserEmail:        " user@example.com ",
				MaxThreads:          3,
				TaskDir:             " .tasks ",
				ShadowDir:           " .shadow ",
				AllowAllPermissions: true,
				VFSAllow:            []string{"/one", "/two"},
				RunBashMax:          intPtr(2048),
				VfsReadLimit:        intPtr(512),
				Container: &conf.ContainerConfig{
					Image:   " golang:latest ",
					Mounts:  []string{"src:/src"},
					Env:     []string{"GOFLAGS=-mod=mod"},
					Enabled: true,
				},
			},
			defaults, &containerOn, &containerOff,
		)

		require.NoError(t, err)
		require.NotNil(t, commandContainerEnabled)
		assert.True(t, *commandContainerEnabled)
		assert.Equal(t, "ollama", defaults.DefaultProvider)
		assert.Equal(t, "qwen", defaults.Model)
		assert.Equal(t, "developer", defaults.Role)
		assert.Equal(t, ".", defaults.Workdir)
		assert.Equal(t, "feature-branch", defaults.Worktree)
		assert.True(t, defaults.Merge)
		assert.True(t, defaults.LogLLMRequests)
		assert.Equal(t, "high", defaults.Thinking)
		assert.Equal(t, "gopls", defaults.LSPServer)
		assert.Equal(t, "User", defaults.GitUserName)
		assert.Equal(t, "user@example.com", defaults.GitUserEmail)
		assert.Equal(t, 3, defaults.MaxThreads)
		assert.Equal(t, ".tasks", defaults.TaskDir)
		assert.Equal(t, ".shadow", defaults.ShadowDir)
		assert.True(t, defaults.AllowAllPermissions)
		assert.Equal(t, []string{"/one", "/two"}, defaults.VFSAllow)
		require.NotNil(t, defaults.RunBashMax)
		assert.Equal(t, 2048, *defaults.RunBashMax)
		require.NotNil(t, defaults.VfsReadLimit)
		assert.Equal(t, 512, *defaults.VfsReadLimit)
		assert.True(t, containerOn)
		assert.False(t, containerOff)
		assert.Equal(t, "golang:latest", defaults.Container.Image)
		assert.Equal(t, []string{"src:/src"}, defaults.Container.Mounts)
		assert.Equal(t, []string{"GOFLAGS=-mod=mod"}, defaults.Container.Env)
	})

	t.Run("no commit disables worktree and merge", func(t *testing.T) {
		defaults := &conf.RunDefaultsConfig{Worktree: "feature", Merge: true, Container: &conf.ContainerConfig{}}
		containerOn := false
		containerOff := false

		_, err := applyCommandRunDefaults(
			&conf.RunDefaultsConfig{NoCommit: true, Worktree: "command-feature", Merge: true},
			defaults, &containerOn, &containerOff,
		)

		require.NoError(t, err)
		assert.True(t, defaults.NoCommit)
		assert.Empty(t, defaults.Worktree)
		assert.False(t, defaults.Merge)
	})
}

func TestBuildRunAgentStartupInfoMessages(t *testing.T) {
	t.Run("builds startup lines without command", func(t *testing.T) {
		messages := BuildRunAgentStartupInfoMessages(&runExecution{Thinking: "high", RoleName: "developer"}, BuildSystemResult{ModelName: "ollama/qwen3", RoleConfig: conf.AgentRoleConfig{Name: "developer"}})

		require.Len(t, messages, 3)
		assert.Equal(t, "[INFO] Model: ollama/qwen3", messages[0])
		assert.Equal(t, "[INFO] Thinking: high", messages[1])
		assert.Equal(t, "[INFO] Role: developer", messages[2])
	})

	t.Run("includes command with embedded source", func(t *testing.T) {
		messages := BuildRunAgentStartupInfoMessages(
			&runExecution{Thinking: "", RoleName: "", CommandName: "csw/task-critic", CommandPath: "embedded:data/csw/task-critic.md"},
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
