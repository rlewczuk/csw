package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

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
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalResolver
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{}, nil
	}

	command := TaskCommand()
	rootDir := filepath.Join("/tmp", "project")
	resolved, err := resolveTaskDirPath(command, rootDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(rootDir, ".cswdata", "tasks"), resolved)
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

func TestTaskNewCommandPromptFlagIsOptional(t *testing.T) {
	command := taskNewCommand()
	promptFlag := command.Flags().Lookup("prompt")
	require.NotNil(t, promptFlag)
	_, required := promptFlag.Annotations[cobra.BashCompOneRequiredFlag]
	assert.False(t, required)
}

func TestTaskCommandContainsExpectedSubcommands(t *testing.T) {
	command := TaskCommand()
	subcommands := command.Commands()
	require.Len(t, subcommands, 6)

	names := make([]string, 0, len(subcommands))
	for _, subcommand := range subcommands {
		names = append(names, subcommand.Name())
	}

	assert.ElementsMatch(t, []string{"new", "update", "get", "run", "list", "merge"}, names)
}

func TestTaskCommandArgValidators(t *testing.T) {
	tests := []struct {
		name        string
		command     *cobra.Command
		args        []string
		expectError bool
	}{
		{name: "update requires one argument", command: taskUpdateCommand(), args: []string{}, expectError: true},
		{name: "update accepts one argument", command: taskUpdateCommand(), args: []string{"task-1"}, expectError: false},
		{name: "get requires one argument", command: taskGetCommand(), args: []string{}, expectError: true},
		{name: "get accepts one argument", command: taskGetCommand(), args: []string{"task-1"}, expectError: false},
		{name: "run requires one argument", command: taskRunCommand(), args: []string{}, expectError: true},
		{name: "run accepts one argument", command: taskRunCommand(), args: []string{"task-1"}, expectError: false},
		{name: "list accepts no argument", command: taskListCommand(), args: []string{}, expectError: false},
		{name: "list accepts one argument", command: taskListCommand(), args: []string{"task-1"}, expectError: false},
		{name: "list rejects more than one argument", command: taskListCommand(), args: []string{"task-1", "task-2"}, expectError: true},
		{name: "merge requires one argument", command: taskMergeCommand(), args: []string{}, expectError: true},
		{name: "merge accepts one argument", command: taskMergeCommand(), args: []string{"task-1"}, expectError: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
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

func TestPrintTaskHuman(t *testing.T) {
	taskData := &core.Task{
		UUID:          "task-uuid",
		Name:          "task-name",
		Description:   "task-description",
		Status:        core.TaskStatusOpen,
		State:         core.TaskStateCompleted,
		FeatureBranch: "feature/task",
		ParentBranch:  "main",
		Role:          "developer",
		ParentTaskID:  "parent-uuid",
		Deps:          []string{"dep-a", "dep-b"},
		SessionIDs:    []string{"ses-1"},
		SubtaskIDs:    []string{"sub-1"},
		CreatedAt:     "2026-01-01T10:00:00Z",
		UpdatedAt:     "2026-01-01T10:01:00Z",
	}
	meta := &core.TaskSessionSummary{SessionID: "ses-1", Status: core.TaskStateCompleted}

	output := captureStdout(t, func() {
		printTaskHuman(taskData, meta, " latest summary ")
	})

	assert.Contains(t, output, "UUID: task-uuid")
	assert.Contains(t, output, "Name: task-name")
	assert.Contains(t, output, "Feature branch: feature/task")
	assert.Contains(t, output, "Last summary session: ses-1 (completed)")
	assert.Contains(t, output, "latest summary")
}

func TestPrintTaskHumanNilTaskDoesNothing(t *testing.T) {
	output := captureStdout(t, func() {
		printTaskHuman(nil, nil, "")
	})

	assert.Empty(t, output)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer

	fn()

	require.NoError(t, writer.Close())
	os.Stdout = oldStdout

	var buffer bytes.Buffer
	_, readErr := buffer.ReadFrom(reader)
	require.NoError(t, readErr)
	require.NoError(t, reader.Close())

	return buffer.String()
}
