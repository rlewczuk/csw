package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCliWorktreeAndCommitMessageFlagsDefinition(t *testing.T) {
	cmd := CliCommand()

	worktreeFlag := cmd.Flags().Lookup("worktree")
	require.NotNil(t, worktreeFlag)
	assert.Equal(t, "", worktreeFlag.DefValue)

	commitMessageFlag := cmd.Flags().Lookup("commit-message")
	require.NotNil(t, commitMessageFlag)
	assert.Equal(t, "", commitMessageFlag.DefValue)
	assert.Equal(t, "string", commitMessageFlag.Value.Type())

	mergeFlag := cmd.Flags().Lookup("merge")
	require.NotNil(t, mergeFlag)
	assert.Equal(t, "false", mergeFlag.DefValue)
	assert.Equal(t, "bool", mergeFlag.Value.Type())
}

func TestCLICommandInvocation(t *testing.T) {
	tests := []struct {
		name                    string
		args                    []string
		commandContent          string
		shellResponse           map[string]string
		expectedPrompt          string
		expectedRole            string
		expectedModel           string
		expectedContainerEnable bool
		expectError             bool
		errorContains           string
	}{
		{
			name:           "single argument contains command and args",
			args:           []string{`/review alpha "beta gamma"`},
			commandContent: "---\ndescription: Review\nagent: reviewer\nmodel: provider/review\n---\nTask: $ARGUMENTS; one=$1; two=$2",
			expectedPrompt: "Task: alpha beta gamma; one=alpha; two=beta gamma",
			expectedRole:   "reviewer",
			expectedModel:  "provider/review",
		},
		{
			name:           "extra positional arguments are command arguments",
			args:           []string{"/review", "alpha", "beta"},
			commandContent: "---\nagent: reviewer\nmodel: provider/review\n---\nTask $ARGUMENTS",
			expectedPrompt: "Task alpha beta",
			expectedRole:   "reviewer",
			expectedModel:  "provider/review",
		},
		{
			name:           "cli overrides metadata model and role",
			args:           []string{"--role=developer", "--model=provider/cli", "/review"},
			commandContent: "---\nagent: reviewer\nmodel: provider/review\n---\nTask",
			expectedPrompt: "Task",
			expectedRole:   "developer",
			expectedModel:  "provider/cli",
		},
		{
			name:           "shell command enables container by default",
			args:           []string{"/review"},
			commandContent: "---\n---\nBefore !`echo hi`",
			shellResponse: map[string]string{
				"echo hi": "hello",
			},
			expectedPrompt:          "Before hello",
			expectedRole:            "developer",
			expectedModel:           "provider/default",
			expectedContainerEnable: true,
		},
		{
			name:           "shell command can be disabled by cli flag",
			args:           []string{"--container-disabled", "/review"},
			commandContent: "---\n---\nBefore !`echo hi`",
			shellResponse: map[string]string{
				"echo hi": "hello",
			},
			expectedPrompt:          "Before hello",
			expectedRole:            "developer",
			expectedModel:           "provider/default",
			expectedContainerEnable: false,
		},
		{
			name:           "host script does not enable container by default",
			args:           []string{"/review"},
			commandContent: "---\n---\nBefore !!scripts/host.sh",
			shellResponse: map[string]string{
				"bash 'scripts/host.sh'": "hello",
			},
			expectedPrompt:          "Before hello",
			expectedRole:            "developer",
			expectedModel:           "provider/default",
			expectedContainerEnable: false,
		},
		{
			name:          "not command with extra args fails",
			args:          []string{"plain", "extra"},
			expectError:   true,
			errorContains: "single argument unless using /command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workDir := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(workDir, ".agents", "commands"), 0755))
			if tt.commandContent != "" {
				require.NoError(t, os.WriteFile(filepath.Join(workDir, ".agents", "commands", "review.md"), []byte(tt.commandContent), 0644))
			}

			originalRun := runCLIFunc
			originalDefaults := resolveCLIDefaultsFunc
			t.Cleanup(func() {
				runCLIFunc = originalRun
				resolveCLIDefaultsFunc = originalDefaults
			})

			resolveCLIDefaultsFunc = func(params system.ResolveCLIDefaultsParams) (conf.CLIDefaultsConfig, error) {
				_ = params
				return conf.CLIDefaultsConfig{Model: "provider/default"}, nil
			}

			captured := ""
			runCLIFunc = func(params *CLIParams) error {
				mockRunner := runner.NewMockRunner()
				hostRunner := runner.NewMockRunner()
				for command, output := range tt.shellResponse {
					mockRunner.SetResponse(command, output, 0, nil)
					hostRunner.SetResponse(command, output, 0, nil)
				}
				if err := renderCommandPrompt(params, workDir, mockRunner, hostRunner); err != nil {
					return err
				}
				captured = fmt.Sprintf("prompt=%s,role=%s,model=%s,container=%t", params.Prompt, params.RoleName, params.ModelName, params.ContainerEnabled)
				return nil
			}

			cmd := CliCommand()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			fullArgs := append([]string{"--workdir", workDir}, tt.args...)
			cmd.SetArgs(fullArgs)

			err := cmd.Execute()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				return
			}

			require.NoError(t, err)
			assert.Contains(t, captured, "prompt="+tt.expectedPrompt)
			assert.Contains(t, captured, "role="+tt.expectedRole)
			assert.Contains(t, captured, "model="+tt.expectedModel)
			assert.Contains(t, captured, fmt.Sprintf("container=%t", tt.expectedContainerEnable))
		})
	}
}

func TestRenderCommandPromptFileReference(t *testing.T) {
	workDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "file.txt"), []byte("abc"), 0644))

	params := &CLIParams{CommandName: "review", CommandTemplate: "Read @file.txt", CommandArgs: []string{}}
	err := renderCommandPrompt(params, workDir, runner.NewMockRunner(), runner.NewMockRunner())
	require.NoError(t, err)
	assert.Equal(t, "Read abc", params.Prompt)
}
