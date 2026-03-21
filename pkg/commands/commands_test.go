package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInvocation(t *testing.T) {
	tests := []struct {
		name         string
		prompt       string
		extra        []string
		expectedName string
		expectedArgs []string
		expectedHit  bool
		expectError  bool
	}{
		{name: "not command", prompt: "hello", expectedHit: false},
		{name: "simple command", prompt: "/test", expectedHit: true, expectedName: "test", expectedArgs: []string{}},
		{name: "quoted args", prompt: `/test one "two three"`, expectedHit: true, expectedName: "test", expectedArgs: []string{"one", "two three"}},
		{name: "extra args appended", prompt: "/test one", extra: []string{"two", "three"}, expectedHit: true, expectedName: "test", expectedArgs: []string{"one", "two", "three"}},
		{name: "invalid quote", prompt: `/test "abc`, expectedHit: true, expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invocation, hit, err := ParseInvocation(tt.prompt, tt.extra)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedHit, hit)
			if !tt.expectedHit {
				assert.Nil(t, invocation)
				return
			}
			require.NotNil(t, invocation)
			assert.Equal(t, tt.expectedName, invocation.Name)
			assert.Equal(t, tt.expectedArgs, invocation.Arguments)
		})
	}
}

func TestLoadFromDir(t *testing.T) {
	root := t.TempDir()
	commandsDir := filepath.Join(root, ".agents", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("---\ndescription: desc\nagent: reviewer\nmodel: provider/model\n---\nrun $ARGUMENTS"), 0644))

	command, err := LoadFromDir(commandsDir, "test")
	require.NoError(t, err)
	assert.Equal(t, "test", command.Name)
	assert.Equal(t, "desc", command.Metadata.Description)
	assert.Equal(t, "reviewer", command.Metadata.Agent)
	assert.Equal(t, "provider/model", command.Metadata.Model)
	assert.Equal(t, "run $ARGUMENTS", command.Template)
}

func TestApplyArguments(t *testing.T) {
	rendered := ApplyArguments("$ARGUMENTS :: $1 :: $2 :: $4", []string{"alpha", "beta", "gamma"})
	assert.Equal(t, "alpha beta gamma :: alpha :: beta :: ", rendered)
}

func TestExpandPrompt(t *testing.T) {
	workDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "sample.txt"), []byte("file-content"), 0644))

	defaultRunner := runner.NewMockRunner()
	hostRunner := runner.NewMockRunner()
	defaultRunner.SetResponse("echo value", "shell-output\n", 0, nil)

	rendered, err := ExpandPrompt("Shell: !`echo value`\nFile: @sample.txt", workDir, defaultRunner, hostRunner)
	require.NoError(t, err)
	assert.Equal(t, "Shell: shell-output\nFile: file-content", rendered)

	executions := defaultRunner.GetExecutions()
	require.Len(t, executions, 1)
	assert.Equal(t, workDir, executions[0].Workdir)
}

func TestExpandPromptShellFailure(t *testing.T) {
	defaultRunner := runner.NewMockRunner()
	hostRunner := runner.NewMockRunner()
	defaultRunner.SetResponse("false", "boom", 1, nil)

	_, err := ExpandPrompt("!`false`", t.TempDir(), defaultRunner, hostRunner)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit code 1")
}

func TestExpandPromptScriptPrefixes(t *testing.T) {
	workDir := t.TempDir()
	defaultRunner := runner.NewMockRunner()
	hostRunner := runner.NewMockRunner()
	defaultRunner.SetResponse("bash 'scripts/default.sh'", "default-output\n", 0, nil)
	hostRunner.SetResponse("bash 'scripts/host.sh'", "host-output\n", 0, nil)

	rendered, err := ExpandPrompt("A !scripts/default.sh B !!scripts/host.sh", workDir, defaultRunner, hostRunner)
	require.NoError(t, err)
	assert.Equal(t, "A default-output B host-output", rendered)

	defaultExecutions := defaultRunner.GetExecutions()
	hostExecutions := hostRunner.GetExecutions()
	require.Len(t, defaultExecutions, 1)
	require.Len(t, hostExecutions, 1)
	assert.Equal(t, "bash 'scripts/default.sh'", defaultExecutions[0].Command)
	assert.Equal(t, "bash 'scripts/host.sh'", hostExecutions[0].Command)
	assert.Equal(t, workDir, defaultExecutions[0].Workdir)
	assert.Equal(t, workDir, hostExecutions[0].Workdir)
}

func TestHasDefaultRuntimeShellExpansion(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected bool
	}{
		{name: "inline shell command", prompt: "!`echo test`", expected: true},
		{name: "default runtime script", prompt: "run !scripts/check.sh", expected: true},
		{name: "host-only script", prompt: "run !!scripts/check.sh", expected: false},
		{name: "no shell expansion", prompt: "plain prompt", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasDefaultRuntimeShellExpansion(tt.prompt))
		})
	}
}
