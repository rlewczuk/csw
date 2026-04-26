package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskCommandHasTaskDirPersistentFlag(t *testing.T) {
	command := TaskCommand()
	flag := command.PersistentFlags().Lookup("task-dir")
	require.NotNil(t, flag)
}

func TestResolveTaskDirPathUsesDefaultWhenUnset(t *testing.T) {
	originalResolver := resolveTaskRunDefaultsFunc
	originalShadowDir := shadowDir
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalResolver
		shadowDir = originalShadowDir
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		assert.Equal(t, "shadow/project", params.ShadowDir)
		_ = params
		return conf.RunDefaultsConfig{}, nil
	}
	shadowDir = "shadow/project"

	command := TaskCommand()
	rootDir := filepath.Join("/tmp", "project")
	resolved, err := resolveTaskDirPath(command, rootDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(rootDir, ".cswdata", "tasks"), resolved)
}

func TestLoadTaskManagerPassesShadowDirectoryToPrepareSessionVFS(t *testing.T) {
	originalPrepare := prepareTaskSessionVFSFunc
	originalShadowDir := shadowDir
	originalProjectConfig := projectConfig
	originalConfigPath := configPath
	t.Cleanup(func() {
		prepareTaskSessionVFSFunc = originalPrepare
		shadowDir = originalShadowDir
		projectConfig = originalProjectConfig
		configPath = originalConfigPath
	})

	shadowDir = filepath.Join(".", "shadow")
	require.NoError(t, os.MkdirAll(shadowDir, 0o755))
	projectConfig = ""
	configPath = ""

	called := false
	prepareTaskSessionVFSFunc = func(workDir string, worktreesBaseDir string, worktreeBranch string, hidePatterns []string, gitUserName string, gitUserEmail string, allowedPaths []string) (apis.VCS, apis.VFS, error) {
		_ = worktreeBranch
		_ = hidePatterns
		_ = gitUserName
		_ = gitUserEmail
		called = true
		resolvedShadow, err := system.ResolveWorkDir(shadowDir)
		require.NoError(t, err)
		assert.Equal(t, resolvedShadow, worktreesBaseDir)
		assert.Contains(t, allowedPaths, resolvedShadow)
		return nil, nil, fmt.Errorf("test expected failure")
	}

	manager, vcsRepo, err := loadTaskManager(TaskCommand())
	assert.Nil(t, manager)
	assert.Nil(t, vcsRepo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test expected failure")
	assert.True(t, called)
}

func TestResolveTaskDirPathUsesConfigDefaultWhenUnset(t *testing.T) {
	originalResolver := resolveTaskRunDefaultsFunc
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalResolver
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{TaskDir: "custom/tasks"}, nil
	}

	command := TaskCommand()
	rootDir := filepath.Join("/tmp", "project")
	resolved, err := resolveTaskDirPath(command, rootDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(rootDir, "custom", "tasks"), resolved)
}

func TestResolveTaskDirPathPrefersFlagOverConfigAndMakesRelativeAbsolute(t *testing.T) {
	originalResolver := resolveTaskRunDefaultsFunc
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalResolver
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{TaskDir: "from-config"}, nil
	}

	command := TaskCommand()
	require.NoError(t, command.ParseFlags([]string{"--task-dir", "from-flag"}))

	rootDir := filepath.Join("/tmp", "project")
	resolved, err := resolveTaskDirPath(command, rootDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(rootDir, "from-flag"), resolved)
}

func TestResolveTaskDirPathReadsTaskDirFromSubcommandPersistentFlag(t *testing.T) {
	originalResolver := resolveTaskRunDefaultsFunc
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalResolver
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{TaskDir: "from-config"}, nil
	}

	command := TaskCommand()
	newCommand, _, err := command.Find([]string{"new"})
	require.NoError(t, err)
	require.NoError(t, newCommand.ParseFlags([]string{"--task-dir", "from-subcommand"}))

	rootDir := filepath.Join("/tmp", "project")
	resolved, err := resolveTaskDirPath(newCommand, rootDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(rootDir, "from-subcommand"), resolved)
}

func TestTaskCommandContainsExpectedSubcommands(t *testing.T) {
	command := TaskCommand()
	subcommands := command.Commands()
	require.Len(t, subcommands, 7)

	names := make([]string, 0, len(subcommands))
	for _, subcommand := range subcommands {
		names = append(names, subcommand.Name())
	}

	assert.ElementsMatch(t, []string{"new", "update", "edit", "get", "list", "merge", "archive"}, names)
}

func TestTaskCommandArgValidators(t *testing.T) {
	tests := []struct {
		name        string
		command     *cobra.Command
		args        []string
		expectError bool
		prepare     func(t *testing.T, command *cobra.Command)
	}{
		{name: "update requires one argument", command: taskUpdateCommand(), args: []string{}, expectError: true},
		{name: "update accepts one argument", command: taskUpdateCommand(), args: []string{"task-1"}, expectError: false},
		{name: "edit requires one argument", command: taskEditCommand(), args: []string{}, expectError: true},
		{name: "edit accepts one argument", command: taskEditCommand(), args: []string{"task-1"}, expectError: false},
		{name: "get requires one argument", command: taskGetCommand(), args: []string{}, expectError: true},
		{name: "get accepts one argument", command: taskGetCommand(), args: []string{"task-1"}, expectError: false},
		{name: "list accepts no argument", command: taskListCommand(), args: []string{}, expectError: false},
		{name: "list accepts one argument", command: taskListCommand(), args: []string{"task-1"}, expectError: false},
		{name: "list rejects more than one argument", command: taskListCommand(), args: []string{"task-1", "task-2"}, expectError: true},
		{name: "merge requires one argument", command: taskMergeCommand(), args: []string{}, expectError: true},
		{name: "merge accepts one argument", command: taskMergeCommand(), args: []string{"task-1"}, expectError: false},
		{name: "archive accepts no argument", command: taskArchiveCommand(), args: []string{}, expectError: false},
		{name: "archive accepts one argument", command: taskArchiveCommand(), args: []string{"task-1"}, expectError: false},
		{name: "archive accepts more than one argument", command: taskArchiveCommand(), args: []string{"task-1", "task-2"}, expectError: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.prepare != nil {
				testCase.prepare(t, testCase.command)
			}
			err := testCase.command.Args(testCase.command, testCase.args)
			if testCase.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestPrintTaskRunOutcome(t *testing.T) {
	outcome := tool.TaskRunOutcome{SessionID: " ses-123 ", TaskBranchName: " feature/task ", SummaryText: "  summary text  "}

	output := captureStdout(t, func() {
		printTaskRunOutcome(outcome)
	})

	assert.Contains(t, output, "Task run session: ses-123")
	assert.Contains(t, output, "Task branch: feature/task")
	assert.Contains(t, output, "summary text")
}

func TestPrintTaskCreated(t *testing.T) {
	taskData := &core.Task{UUID: " task-uuid ", Description: " generated description "}

	output := captureStdout(t, func() {
		printTaskCreated(taskData)
	})

	assert.Contains(t, output, "Task created: task-uuid")
	assert.Contains(t, output, "Description: generated description")
}
