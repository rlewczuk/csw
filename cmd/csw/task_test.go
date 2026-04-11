package main

import (
	"bytes"
	"context"
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
	"gopkg.in/yaml.v3"
)

func TestResolveTaskEditorCommandPrefersFlagValue(t *testing.T) {
	command, err := resolveTaskEditorCommand(taskNewPromptParams{Editor: "custom-editor --flag"})
	require.NoError(t, err)
	assert.Equal(t, "custom-editor --flag", command)
}

func TestResolveTaskEditorCommandUsesEditorEnvironment(t *testing.T) {
	t.Setenv("EDITOR", "env-editor")

	command, err := resolveTaskEditorCommand(taskNewPromptParams{})
	require.NoError(t, err)
	assert.Equal(t, "env-editor", command)
}

func TestResolveTaskEditorCommandUsesConfiguredEditorsFallback(t *testing.T) {
	originalResolver := resolveTaskRunDefaultsFunc
	originalLookPath := taskEditorLookPathFunc
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalResolver
		taskEditorLookPathFunc = originalLookPath
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{Editors: []string{"missing-editor", "vim"}}, nil
	}

	taskEditorLookPathFunc = func(file string) (string, error) {
		if file == "vim" {
			return "/usr/bin/vim", nil
		}
		return "", os.ErrNotExist
	}

	command, err := resolveTaskEditorCommand(taskNewPromptParams{})
	require.NoError(t, err)
	assert.Equal(t, "vim", command)
}

func TestResolveTaskNewPromptSkipsTaskCreationWhenEditedPromptIsEmpty(t *testing.T) {
	originalEditorRunner := runTaskEditorFunc
	t.Cleanup(func() {
		runTaskEditorFunc = originalEditorRunner
	})

	runTaskEditorFunc = func(ctx context.Context, editorCommand string, promptFilePath string) error {
		_ = ctx
		_ = editorCommand
		return os.WriteFile(promptFilePath, []byte(" \n\t "), 0o644)
	}

	prompt, shouldCreate, err := resolveTaskNewPrompt(context.Background(), taskNewPromptParams{Editor: "editor"})
	require.NoError(t, err)
	assert.False(t, shouldCreate)
	assert.Equal(t, "", prompt)
}

func TestResolveTaskCreateParamsGeneratesBranchAndDescription(t *testing.T) {
	originalDefaults := resolveTaskRunDefaultsFunc
	originalBranchResolver := resolveTaskWorktreeBranchNameFunc
	originalDescriptionGenerator := generateTaskDescriptionFunc
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalDefaults
		resolveTaskWorktreeBranchNameFunc = originalBranchResolver
		generateTaskDescriptionFunc = originalDescriptionGenerator
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{Model: "provider/model", Worktree: "feature/%"}, nil
	}

	resolveTaskWorktreeBranchNameFunc = func(ctx context.Context, params system.ResolveWorktreeBranchNameParams) (string, error) {
		_ = ctx
		assert.Equal(t, "do this task", params.Prompt)
		assert.Equal(t, "provider/model", params.ModelName)
		assert.Equal(t, "feature/%", params.WorktreeBranch)
		return "feature/generated", nil
	}

	generateTaskDescriptionFunc = func(ctx context.Context, params taskCreateResolveParams) (string, error) {
		_ = ctx
		assert.Equal(t, "do this task", params.Prompt)
		assert.Equal(t, "feature/generated", params.Branch)
		return "generated description", nil
	}

	resolved, err := resolveTaskCreateParams(context.Background(), taskCreateResolveParams{Prompt: "do this task"})
	require.NoError(t, err)
	assert.Equal(t, "feature/generated", resolved.FeatureBranch)
	assert.Equal(t, "feature/generated", resolved.Name)
	assert.Equal(t, "generated description", resolved.Description)
	assert.Equal(t, "do this task", resolved.Prompt)
}

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
	require.Len(t, subcommands, 7)

	names := make([]string, 0, len(subcommands))
	for _, subcommand := range subcommands {
		names = append(names, subcommand.Name())
	}

	assert.ElementsMatch(t, []string{"new", "update", "get", "run", "list", "merge", "archive"}, names)
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
		{name: "archive accepts no argument", command: taskArchiveCommand(), args: []string{}, expectError: false},
		{name: "archive accepts one argument", command: taskArchiveCommand(), args: []string{"task-1"}, expectError: false},
		{name: "archive rejects more than one argument", command: taskArchiveCommand(), args: []string{"task-1", "task-2"}, expectError: true},
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

func TestTaskArchiveCommandIncludesStatusFlag(t *testing.T) {
	command := taskArchiveCommand()
	assert.NotNil(t, command.Flags().Lookup("status"))
}

func TestRunTaskArchiveConflictingArguments(t *testing.T) {
	manager, err := core.NewTaskManager(t.TempDir(), nil, nil)
	require.NoError(t, err)

	err = runTaskArchive(manager, []string{"task-id"}, "merged", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task identifier and --status cannot be used together")
}

func TestRunTaskArchiveByIdentifier(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(core.TaskCreateParams{Name: "archive-me", Prompt: "prompt"})
	require.NoError(t, err)

	buffer := &bytes.Buffer{}
	err = runTaskArchive(manager, []string{created.UUID}, "", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), "Task archived: "+created.UUID)

	archivedPath := filepath.Join(baseDir, ".cswdata", "tasks-archived", created.UUID, "task.yml")
	_, statErr := os.Stat(archivedPath)
	require.NoError(t, statErr)
}

func TestRunTaskArchiveDefaultStatusMerged(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil, nil)
	require.NoError(t, err)

	mergedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "merged", Prompt: "prompt"})
	require.NoError(t, err)
	openTask, err := manager.CreateTask(core.TaskCreateParams{Name: "open", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", mergedTask.UUID, "task.yml"), core.TaskStatusMerged, core.TaskStateCompleted)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", openTask.UUID, "task.yml"), core.TaskStatusOpen, core.TaskStateCreated)

	buffer := &bytes.Buffer{}
	err = runTaskArchive(manager, nil, "", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), mergedTask.UUID)
	assert.NotContains(t, buffer.String(), openTask.UUID)

	_, mergedErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks-archived", mergedTask.UUID, "task.yml"))
	require.NoError(t, mergedErr)
	_, openErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", openTask.UUID, "task.yml"))
	require.NoError(t, openErr)
}

func TestRunTaskArchiveByStatusFailed(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil, nil)
	require.NoError(t, err)

	failedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "failed", Prompt: "prompt"})
	require.NoError(t, err)
	otherTask, err := manager.CreateTask(core.TaskCreateParams{Name: "other", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", failedTask.UUID, "task.yml"), core.TaskStatusOpen, core.TaskStateFailed)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", otherTask.UUID, "task.yml"), core.TaskStatusOpen, core.TaskStateCompleted)

	buffer := &bytes.Buffer{}
	err = runTaskArchive(manager, nil, "failed", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), failedTask.UUID)
	assert.NotContains(t, buffer.String(), otherTask.UUID)

	_, failedErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks-archived", failedTask.UUID, "task.yml"))
	require.NoError(t, failedErr)
	_, otherErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", otherTask.UUID, "task.yml"))
	require.NoError(t, otherErr)
}

func setTaskStatusForTest(t *testing.T, taskPath string, status string, state string) {
	t.Helper()

	contents, err := os.ReadFile(taskPath)
	require.NoError(t, err)

	taskData := &core.Task{}
	require.NoError(t, yaml.Unmarshal(contents, taskData))
	taskData.Status = status
	taskData.State = state

	updatedContents, err := yaml.Marshal(taskData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(taskPath, updatedContents, 0o644))
}

func TestTaskRunCommandIncludesRunSessionFlags(t *testing.T) {
	command := taskRunCommand()

	flagNames := []string{
		"allow-all-permissions",
		"bash-run-timeout",
		"config-path",
		"container-disabled",
		"container-enabled",
		"container-env",
		"container-image",
		"container-mount",
		"context",
		"force-compact",
		"git-email",
		"git-user",
		"interactive",
		"log-llm-requests",
		"log-llm-requests-raw",
		"lsp-server",
		"max-threads",
		"mcp-disable",
		"mcp-enable",
		"merge",
		"model",
		"no-refresh",
		"output-format",
		"project-config",
		"role",
		"save-session",
		"save-session-to",
		"shadow-dir",
		"thinking",
		"vfs-allow",
		"workdir",
		"worktree",
		"hook",
	}

	for _, flagName := range flagNames {
		t.Run(flagName, func(t *testing.T) {
			assert.NotNil(t, command.Flags().Lookup(flagName))
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
