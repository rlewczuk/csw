package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		assert.Equal(t, "provider/model", params.ModelName)
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

func TestTaskNewCommandPromptFlagIsOptional(t *testing.T) {
	command := taskNewCommand()
	promptFlag := command.Flags().Lookup("prompt")
	require.NotNil(t, promptFlag)
	_, required := promptFlag.Annotations[cobra.BashCompOneRequiredFlag]
	assert.False(t, required)
	assert.Nil(t, command.Flags().Lookup("run"))
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
		{name: "archive rejects more than one argument", command: taskArchiveCommand(), args: []string{"task-1", "task-2"}, expectError: true},
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

func TestTaskUpdateCommandIncludesExpectedFlags(t *testing.T) {
	command := taskUpdateCommand()
	assert.NotNil(t, command.Flags().Lookup("last"))
	assert.NotNil(t, command.Flags().Lookup("next"))
	assert.NotNil(t, command.Flags().Lookup("status"))
	assert.NotNil(t, command.Flags().Lookup("edit"))
	assert.NotNil(t, command.Flags().Lookup("editor"))
	assert.NotNil(t, command.Flags().Lookup("regen"))
	assert.NotNil(t, command.Flags().Lookup("regen-branch"))
	assert.NotNil(t, command.Flags().Lookup("regen-name"))
	assert.NotNil(t, command.Flags().Lookup("regen-description"))
	assert.Nil(t, command.Flags().Lookup("run"))
}

func TestTaskEditCommandDefaultsToEditMode(t *testing.T) {
	command := taskEditCommand()
	assert.Equal(t, "edit", command.Name())
	editFlag := command.Flags().Lookup("edit")
	require.NotNil(t, editFlag)
	assert.Equal(t, "true", editFlag.DefValue)
}

func TestTaskUpdateCommandArgsValidationWithLastFlag(t *testing.T) {
	command := taskUpdateCommand()

	argsErr := command.Args(command, []string{})
	assert.Error(t, argsErr)

	require.NoError(t, command.Flags().Set("last", "true"))

	assert.NoError(t, command.Args(command, []string{}))

	require.NoError(t, command.Flags().Set("next", "true"))
	lastNextConflictErr := command.Args(command, []string{})
	require.Error(t, lastNextConflictErr)
	assert.Contains(t, lastNextConflictErr.Error(), "--last and --next cannot be used together")

	require.NoError(t, command.Flags().Set("next", "false"))
	conflictErr := command.Args(command, []string{"task-1"})
	require.Error(t, conflictErr)
	assert.Contains(t, conflictErr.Error(), "cannot be used with --last or --next")
}

func TestReadTaskPromptFileReturnsEmptyWhenFileIsMissing(t *testing.T) {
	taskDir := t.TempDir()
	prompt, err := readTaskPromptFile(taskDir)
	require.NoError(t, err)
	assert.Equal(t, "", prompt)
}

func TestEditTaskPromptDetectsPromptChange(t *testing.T) {
	originalEditorRunner := runTaskEditorFunc
	t.Cleanup(func() {
		runTaskEditorFunc = originalEditorRunner
	})

	runTaskEditorFunc = func(ctx context.Context, editorCommand string, promptFilePath string) error {
		_ = ctx
		_ = editorCommand
		return os.WriteFile(promptFilePath, []byte("new prompt\n"), 0o644)
	}

	prompt, changed, err := editTaskPrompt(context.Background(), "editor", "old prompt")
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "new prompt", prompt)
}

func TestEditTaskPromptDetectsNoPromptChange(t *testing.T) {
	originalEditorRunner := runTaskEditorFunc
	t.Cleanup(func() {
		runTaskEditorFunc = originalEditorRunner
	})

	runTaskEditorFunc = func(ctx context.Context, editorCommand string, promptFilePath string) error {
		_ = ctx
		_ = editorCommand
		return nil
	}

	prompt, changed, err := editTaskPrompt(context.Background(), "editor", "same prompt")
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, "same prompt", prompt)
}

func TestTaskListCommandIncludesExpectedFlags(t *testing.T) {
	command := taskListCommand()
	assert.NotNil(t, command.Flags().Lookup("recursive"))
	assert.NotNil(t, command.Flags().Lookup("archived"))
	assert.NotNil(t, command.Flags().Lookup("status"))
	assert.Nil(t, command.Flags().Lookup("verbose"))
}

func TestRunTaskListListsCurrentTasksSortedByTaskYMLModTime(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	first, err := manager.CreateTask(core.TaskCreateParams{Name: "first", Prompt: "prompt", Description: "first desc", FeatureBranch: "feature/first"})
	require.NoError(t, err)
	second, err := manager.CreateTask(core.TaskCreateParams{Name: "second", Prompt: "prompt", Description: "second desc", FeatureBranch: "feature/second"})
	require.NoError(t, err)

	newer := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)
	older := time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC)
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", first.UUID, "task.yml"), newer)
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", second.UUID, "task.yml"), older)

	buffer := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, false, "", buffer)
	require.NoError(t, err)

	lines := nonEmptyLines(buffer.String())
	require.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[0], second.UUID+"\t"))
	assert.True(t, strings.HasPrefix(lines[1], first.UUID+"\t"))

	columns := strings.Split(lines[0], "\t")
	require.Len(t, columns, 3)
	assert.Equal(t, second.UUID, columns[0])
	assert.Equal(t, second.Status, columns[1])
	assert.Equal(t, second.Description, columns[2])
}

func TestRunTaskListIncludesArchivedWhenRequested(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	activeTask, err := manager.CreateTask(core.TaskCreateParams{Name: "active", Prompt: "prompt"})
	require.NoError(t, err)
	archivedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "archived", Prompt: "prompt"})
	require.NoError(t, err)
	_, err = manager.ArchiveTask(core.TaskLookup{Identifier: archivedTask.UUID})
	require.NoError(t, err)

	withoutArchived := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, false, "", withoutArchived)
	require.NoError(t, err)
	assert.Contains(t, withoutArchived.String(), activeTask.UUID)
	assert.NotContains(t, withoutArchived.String(), archivedTask.UUID)

	withArchived := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, true, "", withArchived)
	require.NoError(t, err)
	assert.Contains(t, withArchived.String(), activeTask.UUID)
	assert.Contains(t, withArchived.String(), archivedTask.UUID)
}

func TestRunTaskListIgnoresArchiveContainerDirectory(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	activeTask, err := manager.CreateTask(core.TaskCreateParams{Name: "active", Prompt: "prompt"})
	require.NoError(t, err)
	archivedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "archived", Prompt: "prompt"})
	require.NoError(t, err)
	_, err = manager.ArchiveTask(core.TaskLookup{Identifier: archivedTask.UUID})
	require.NoError(t, err)

	buffer := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, false, "", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), activeTask.UUID)
	assert.NotContains(t, buffer.String(), archivedTask.UUID)
}

func TestRunTaskListStatusFiltering(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	openTask, err := manager.CreateTask(core.TaskCreateParams{Name: "open", Prompt: "prompt"})
	require.NoError(t, err)
	mergedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "merged", Prompt: "prompt"})
	require.NoError(t, err)
	createdTask, err := manager.CreateTask(core.TaskCreateParams{Name: "created", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", openTask.UUID, "task.yml"), core.TaskStatusOpen)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", mergedTask.UUID, "task.yml"), core.TaskStatusMerged)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", createdTask.UUID, "task.yml"), core.TaskStatusCreated)

	included := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, false, "open,merged", included)
	require.NoError(t, err)
	assert.Contains(t, included.String(), openTask.UUID)
	assert.Contains(t, included.String(), mergedTask.UUID)
	assert.NotContains(t, included.String(), createdTask.UUID)

	excluded := &bytes.Buffer{}
	err = runTaskList(manager, nil, false, false, "!merged", excluded)
	require.NoError(t, err)
	assert.Contains(t, excluded.String(), openTask.UUID)
	assert.Contains(t, excluded.String(), createdTask.UUID)
	assert.NotContains(t, excluded.String(), mergedTask.UUID)
}

func TestTaskArchiveCommandIncludesStatusFlag(t *testing.T) {
	command := taskArchiveCommand()
	assert.NotNil(t, command.Flags().Lookup("status"))
}

func TestRunTaskArchiveConflictingArguments(t *testing.T) {
	manager, err := core.NewTaskManager(t.TempDir(), nil)
	require.NoError(t, err)

	err = runTaskArchive(manager, []string{"task-id"}, "merged", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task identifier and --status cannot be used together")
}

func TestRunTaskArchiveByIdentifier(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(core.TaskCreateParams{Name: "archive-me", Prompt: "prompt"})
	require.NoError(t, err)

	buffer := &bytes.Buffer{}
	err = runTaskArchive(manager, []string{created.UUID}, "", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), "Task archived: "+created.UUID)

	archivedPath := filepath.Join(baseDir, ".cswdata", "tasks", "archive", created.UUID, "task.yml")
	_, statErr := os.Stat(archivedPath)
	require.NoError(t, statErr)
}

func TestRunTaskArchiveDefaultStatusMerged(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	mergedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "merged", Prompt: "prompt"})
	require.NoError(t, err)
	openTask, err := manager.CreateTask(core.TaskCreateParams{Name: "open", Prompt: "prompt"})
	require.NoError(t, err)
	draftTask, err := manager.CreateTask(core.TaskCreateParams{Name: "draft", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", mergedTask.UUID, "task.yml"), core.TaskStatusMerged)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", openTask.UUID, "task.yml"), core.TaskStatusOpen)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", draftTask.UUID, "task.yml"), core.TaskStatusDraft)

	buffer := &bytes.Buffer{}
	err = runTaskArchive(manager, nil, "", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), mergedTask.UUID)
	assert.NotContains(t, buffer.String(), openTask.UUID)
	assert.NotContains(t, buffer.String(), draftTask.UUID)

	_, mergedErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", "archive", mergedTask.UUID, "task.yml"))
	require.NoError(t, mergedErr)
	_, openErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", openTask.UUID, "task.yml"))
	require.NoError(t, openErr)
	_, draftErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", draftTask.UUID, "task.yml"))
	require.NoError(t, draftErr)
}

func TestRunTaskArchiveByStatusFailed(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	failedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "failed", Prompt: "prompt"})
	require.NoError(t, err)
	otherTask, err := manager.CreateTask(core.TaskCreateParams{Name: "other", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", failedTask.UUID, "task.yml"), "failed")
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", otherTask.UUID, "task.yml"), core.TaskStatusOpen)

	buffer := &bytes.Buffer{}
	err = runTaskArchive(manager, nil, "failed", buffer)
	require.NoError(t, err)
	assert.Contains(t, buffer.String(), failedTask.UUID)
	assert.NotContains(t, buffer.String(), otherTask.UUID)

	_, failedErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", "archive", failedTask.UUID, "task.yml"))
	require.NoError(t, failedErr)
	_, otherErr := os.Stat(filepath.Join(baseDir, ".cswdata", "tasks", otherTask.UUID, "task.yml"))
	require.NoError(t, otherErr)
}

func setTaskStatusForTest(t *testing.T, taskPath string, status string) {
	t.Helper()

	contents, err := os.ReadFile(taskPath)
	require.NoError(t, err)

	taskData := &core.Task{}
	require.NoError(t, yaml.Unmarshal(contents, taskData))
	taskData.Status = status

	updatedContents, err := yaml.Marshal(taskData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(taskPath, updatedContents, 0o644))
}

func setTaskYMLModTimeForTest(t *testing.T, taskPath string, modTime time.Time) {
	t.Helper()
	require.NoError(t, os.Chtimes(taskPath, modTime, modTime))
}

func nonEmptyLines(input string) []string {
	raw := strings.Split(strings.TrimSpace(input), "\n")
	result := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		result = append(result, line)
	}
	return result
}

func TestResolveTaskRunIdentifierReturnsProvidedIdentifierWhenNotUsingLastFlag(t *testing.T) {
	resolved, err := resolveTaskRunIdentifier(nil, " task-123 ", false, false)
	require.NoError(t, err)
	assert.Equal(t, "task-123", resolved)
}

func TestResolveTaskRunIdentifierReturnsNameLastWhenLastFlagIsNotUsed(t *testing.T) {
	resolved, err := resolveTaskRunIdentifier(nil, " last ", false, false)
	require.NoError(t, err)
	assert.Equal(t, "last", resolved)
}

func TestResolveTaskRunIdentifierResolvesLastUnfinishedTaskByMostRecentModTime(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	oldCompleted, err := manager.CreateTask(core.TaskCreateParams{Name: "completed", Prompt: "prompt"})
	require.NoError(t, err)
	runnableOlder, err := manager.CreateTask(core.TaskCreateParams{Name: "runnable-older", Prompt: "prompt"})
	require.NoError(t, err)
	runnableNewest, err := manager.CreateTask(core.TaskCreateParams{Name: "runnable-newest", Prompt: "prompt"})
	require.NoError(t, err)
	draftNewest, err := manager.CreateTask(core.TaskCreateParams{Name: "draft-newest", Prompt: "prompt"})
	require.NoError(t, err)
	runningTask, err := manager.CreateTask(core.TaskCreateParams{Name: "running", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", oldCompleted.UUID, "task.yml"), core.TaskStatusMerged)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableOlder.UUID, "task.yml"), core.TaskStatusOpen)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableNewest.UUID, "task.yml"), core.TaskStatusCreated)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", draftNewest.UUID, "task.yml"), core.TaskStatusDraft)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runningTask.UUID, "task.yml"), core.TaskStatusRunning)

	baseTime := time.Date(2026, time.February, 1, 10, 0, 0, 0, time.UTC)
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", oldCompleted.UUID, "task.yml"), baseTime.Add(1*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableOlder.UUID, "task.yml"), baseTime.Add(2*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableNewest.UUID, "task.yml"), baseTime.Add(3*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", draftNewest.UUID, "task.yml"), baseTime.Add(5*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runningTask.UUID, "task.yml"), baseTime.Add(4*time.Minute))

	resolved, err := resolveTaskRunIdentifier(manager, "", true, false)
	require.NoError(t, err)
	assert.Equal(t, runnableNewest.UUID, resolved)
}

func TestResolveTaskRunIdentifierResolvesNextUnfinishedTaskByOldestModTime(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	runnableNewest, err := manager.CreateTask(core.TaskCreateParams{Name: "runnable-newest", Prompt: "prompt"})
	require.NoError(t, err)
	runnableOldest, err := manager.CreateTask(core.TaskCreateParams{Name: "runnable-oldest", Prompt: "prompt"})
	require.NoError(t, err)
	draftOldest, err := manager.CreateTask(core.TaskCreateParams{Name: "draft-oldest", Prompt: "prompt"})
	require.NoError(t, err)
	runningTask, err := manager.CreateTask(core.TaskCreateParams{Name: "running", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableNewest.UUID, "task.yml"), core.TaskStatusOpen)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableOldest.UUID, "task.yml"), core.TaskStatusCreated)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", draftOldest.UUID, "task.yml"), core.TaskStatusDraft)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runningTask.UUID, "task.yml"), core.TaskStatusRunning)

	baseTime := time.Date(2026, time.February, 1, 10, 0, 0, 0, time.UTC)
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableNewest.UUID, "task.yml"), baseTime.Add(3*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runnableOldest.UUID, "task.yml"), baseTime.Add(1*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", draftOldest.UUID, "task.yml"), baseTime.Add(0*time.Minute))
	setTaskYMLModTimeForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runningTask.UUID, "task.yml"), baseTime.Add(4*time.Minute))

	resolved, err := resolveTaskRunIdentifier(manager, "", false, true)
	require.NoError(t, err)
	assert.Equal(t, runnableOldest.UUID, resolved)
}

func TestResolveTaskRunIdentifierReturnsErrorWhenLastAndNextAreBothUsed(t *testing.T) {
	_, err := resolveTaskRunIdentifier(nil, "", true, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--last and --next cannot be used together")
}

func TestResolveTaskRunIdentifierReturnsErrorWhenNoUnfinishedTaskExists(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := core.NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	completedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "completed", Prompt: "prompt"})
	require.NoError(t, err)
	mergedTask, err := manager.CreateTask(core.TaskCreateParams{Name: "merged", Prompt: "prompt"})
	require.NoError(t, err)
	runningTask, err := manager.CreateTask(core.TaskCreateParams{Name: "running", Prompt: "prompt"})
	require.NoError(t, err)

	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", completedTask.UUID, "task.yml"), core.TaskStatusMerged)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", mergedTask.UUID, "task.yml"), core.TaskStatusMerged)
	setTaskStatusForTest(t, filepath.Join(baseDir, ".cswdata", "tasks", runningTask.UUID, "task.yml"), core.TaskStatusRunning)

	_, err = resolveTaskRunIdentifier(manager, "", true, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no unfinished task found")
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

func TestPrintTaskHuman(t *testing.T) {
	taskData := &core.Task{
		UUID:          "task-uuid",
		Name:          "task-name",
		Description:   "task-description",
		Status:        core.TaskStatusOpen,
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
	meta := &core.TaskSessionSummary{SessionID: "ses-1", Status: core.TaskStatusCompleted}

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
