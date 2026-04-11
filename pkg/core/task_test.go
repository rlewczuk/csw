package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/apis"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/vcs"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// taskTestRunner is a test double for TaskSessionRunner.
type taskTestRunner struct {
	calls       int
	result      TaskSessionRunResult
	err         error
	lastRequest TaskSessionRunRequest
}

// RunTaskSession records invocation count and returns configured result.
func (r *taskTestRunner) RunTaskSession(ctx context.Context, request TaskSessionRunRequest) (TaskSessionRunResult, error) {
	_ = ctx
	r.calls++
	r.lastRequest = request
	if r.result.SessionID == "" && r.result.StartedAt.IsZero() && r.result.CompletedAt.IsZero() && strings.TrimSpace(r.result.SummaryText) == "" {
		now := time.Now().UTC()
		r.result = TaskSessionRunResult{
			SessionID:   "ses-1",
			SummaryText: "summary",
			StartedAt:   now,
			CompletedAt: now,
		}
	}

	return r.result, r.err
}

type fakeVCS struct {
	branches          []string
	newBranchCalls    [][2]string
	deleteBranchCalls []string
	dropWorktreeCalls []string
	mergeCalls        [][2]string

	listBranchesErr error
	newBranchErr    error
	mergeErr        error
}

func (f *fakeVCS) GetWorktree(branch string) (apis.VFS, error) {
	_ = branch
	return nil, nil
}

func (f *fakeVCS) DropWorktree(branch string) error {
	f.dropWorktreeCalls = append(f.dropWorktreeCalls, branch)
	return nil
}

func (f *fakeVCS) CommitWorktree(branch string, message string) error {
	_ = branch
	_ = message
	return nil
}

func (f *fakeVCS) NewBranch(name string, from string) error {
	f.newBranchCalls = append(f.newBranchCalls, [2]string{name, from})
	if f.newBranchErr != nil {
		return f.newBranchErr
	}
	f.branches = append(f.branches, name)
	return nil
}

func (f *fakeVCS) DeleteBranch(name string) error {
	f.deleteBranchCalls = append(f.deleteBranchCalls, name)
	return nil
}

func (f *fakeVCS) ListBranches(prefix string) ([]string, error) {
	_ = prefix
	if f.listBranchesErr != nil {
		return nil, f.listBranchesErr
	}
	return append([]string(nil), f.branches...), nil
}

func (f *fakeVCS) ListWorktrees() ([]string, error) {
	return []string{}, nil
}

func (f *fakeVCS) MergeBranches(into string, from string) error {
	f.mergeCalls = append(f.mergeCalls, [2]string{into, from})
	if f.mergeErr != nil {
		return f.mergeErr
	}
	return nil
}

func TestNewTaskManagerFailsOnEmptyBaseDir(t *testing.T) {
	manager, err := NewTaskManager("   ", nil, &taskTestRunner{})
	require.Error(t, err)
	assert.Nil(t, manager)
	assert.Contains(t, err.Error(), "baseDir cannot be empty")
}

func TestTaskManagerCreateTaskDefaultsAndNormalizeDeps(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil, &taskTestRunner{})
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Deps: []string{"dep-b", "", "dep-a", "dep-b", " dep-a "}})
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.Equal(t, created.UUID, created.Name)
	assert.Equal(t, created.Name, created.FeatureBranch)
	assert.Equal(t, "main", created.ParentBranch)
	assert.Equal(t, []string{"dep-a", "dep-b"}, created.Deps)
}

func TestTaskManagerCreateTaskAllowsEmptyPrompt(t *testing.T) {
	baseDir := t.TempDir()
	runner := &taskTestRunner{}
	manager, err := NewTaskManager(baseDir, nil, runner)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task-name"})
	require.NoError(t, err)
	require.NotNil(t, created)

	taskPromptPath := filepath.Join(baseDir, ".cswdata", "tasks", created.UUID, "task.md")
	contents, err := os.ReadFile(taskPromptPath)
	require.NoError(t, err)
	assert.Empty(t, string(contents))
}

func TestTaskManagerCreateTaskWithParentAddsSubtaskAndNestedPath(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil, &taskTestRunner{})
	require.NoError(t, err)

	parent, err := manager.CreateTask(TaskCreateParams{Name: "parent", Prompt: "parent prompt"})
	require.NoError(t, err)

	child, err := manager.CreateTask(TaskCreateParams{Name: "child", ParentTaskID: parent.UUID, Prompt: "child prompt"})
	require.NoError(t, err)

	parentDir := filepath.Join(baseDir, ".cswdata", "tasks", parent.UUID)
	childTaskFile := filepath.Join(parentDir, child.UUID, "task.yml")
	_, statErr := os.Stat(childTaskFile)
	require.NoError(t, statErr)

	_, parentTask, err := manager.ResolveTask(TaskLookup{Identifier: parent.UUID})
	require.NoError(t, err)
	assert.Contains(t, parentTask.SubtaskIDs, child.UUID)
}

func TestTaskManagerResolveTaskByNameAndFallback(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil, &taskTestRunner{})
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "my-task", Prompt: "do it"})
	require.NoError(t, err)

	_, byName, err := manager.ResolveTask(TaskLookup{Identifier: "my-task"})
	require.NoError(t, err)
	assert.Equal(t, created.UUID, byName.UUID)

	_, byFallback, err := manager.ResolveTask(TaskLookup{FallbackTaskID: created.UUID})
	require.NoError(t, err)
	assert.Equal(t, created.UUID, byFallback.UUID)
}

func TestTaskManagerUpdateTaskUpdatesFieldsAndPrompt(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil, &taskTestRunner{})
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "old", Prompt: "old prompt"})
	require.NoError(t, err)

	name := "new-name"
	description := "new description"
	feature := "feature/new"
	parentBranch := "develop"
	role := "reviewer"
	deps := []string{"dep-2", "dep-1", "dep-1"}
	prompt := "new prompt"

	updated, err := manager.UpdateTask(TaskUpdateParams{
		Identifier:    created.UUID,
		Name:          &name,
		Description:   &description,
		FeatureBranch: &feature,
		ParentBranch:  &parentBranch,
		Role:          &role,
		Deps:          &deps,
		Prompt:        &prompt,
	})
	require.NoError(t, err)

	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, "new description", updated.Description)
	assert.Equal(t, "feature/new", updated.FeatureBranch)
	assert.Equal(t, "develop", updated.ParentBranch)
	assert.Equal(t, "reviewer", updated.Role)
	assert.Equal(t, []string{"dep-1", "dep-2"}, updated.Deps)

	promptPath := filepath.Join(baseDir, ".cswdata", "tasks", created.UUID, "task.md")
	promptBytes, readErr := os.ReadFile(promptPath)
	require.NoError(t, readErr)
	assert.Equal(t, "new prompt\n", string(promptBytes))
}

func TestTaskManagerUpdateTaskRejectsEmptyPrompt(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil, &taskTestRunner{})
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task", Prompt: "prompt"})
	require.NoError(t, err)

	empty := "  "
	updated, err := manager.UpdateTask(TaskUpdateParams{Identifier: created.UUID, Prompt: &empty})
	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Contains(t, err.Error(), "prompt cannot be empty")
}

func TestTaskManagerListTasksTopLevelAndRecursiveChildren(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil, &taskTestRunner{})
	require.NoError(t, err)

	rootA, err := manager.CreateTask(TaskCreateParams{Name: "root-b", Prompt: "a"})
	require.NoError(t, err)
	_, err = manager.CreateTask(TaskCreateParams{Name: "root-a", Prompt: "b"})
	require.NoError(t, err)
	child, err := manager.CreateTask(TaskCreateParams{Name: "child", ParentTaskID: rootA.UUID, Prompt: "c"})
	require.NoError(t, err)
	_, err = manager.CreateTask(TaskCreateParams{Name: "grand", ParentTaskID: child.UUID, Prompt: "d"})
	require.NoError(t, err)

	top, err := manager.ListTasks(TaskLookup{}, false)
	require.NoError(t, err)
	require.Len(t, top, 2)
	assert.Equal(t, []string{"root-a", "root-b"}, []string{top[0].Name, top[1].Name})

	nonRecursiveChildren, err := manager.ListTasks(TaskLookup{Identifier: rootA.UUID}, false)
	require.NoError(t, err)
	require.Len(t, nonRecursiveChildren, 1)
	assert.Equal(t, "child", nonRecursiveChildren[0].Name)

	recursiveChildren, err := manager.ListTasks(TaskLookup{Identifier: rootA.UUID}, true)
	require.NoError(t, err)
	require.Len(t, recursiveChildren, 2)
	names := []string{recursiveChildren[0].Name, recursiveChildren[1].Name}
	sort.Strings(names)
	assert.Equal(t, []string{"child", "grand"}, names)
}

func TestTaskManagerRunTaskFailsWhenTaskPromptIsEmpty(t *testing.T) {
	baseDir := t.TempDir()
	runner := &taskTestRunner{}
	manager, err := NewTaskManager(baseDir, nil, runner)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "empty-task"})
	require.NoError(t, err)

	nullVCS, err := vcs.NewNullVFS(vfs.NewMockVFS())
	require.NoError(t, err)

	outcome, err := manager.RunTask(context.Background(), TaskLookup{Identifier: created.UUID}, TaskRunParams{}, nullVCS)
	require.Error(t, err)
	assert.Nil(t, outcome)
	assert.Contains(t, err.Error(), "task is empty")
	assert.Contains(t, err.Error(), "task.md has no prompt")
	assert.Equal(t, 0, runner.calls)
}

func TestTaskManagerRunTaskSuccessWritesSummaryAndOutput(t *testing.T) {
	baseDir := t.TempDir()
	runner := &taskTestRunner{result: TaskSessionRunResult{
		SessionID:   "ses-123",
		SummaryText: "Task completed",
		StartedAt:   time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		CompletedAt: time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC),
	}}
	manager, err := NewTaskManager(baseDir, nil, runner)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "run-task", FeatureBranch: "feat/run", Prompt: "run prompt"})
	require.NoError(t, err)

	fake := &fakeVCS{branches: []string{"main"}}
	outcome, err := manager.RunTask(context.Background(), TaskLookup{Identifier: created.UUID}, TaskRunParams{}, fake)
	require.NoError(t, err)
	require.NotNil(t, outcome)

	assert.Equal(t, 1, runner.calls)
	assert.Equal(t, "developer", runner.lastRequest.Role)
	require.NotNil(t, runner.lastRequest.Task)
	assert.Equal(t, created.UUID, runner.lastRequest.Task.UUID)
	assert.Equal(t, created.Name, runner.lastRequest.Task.Name)
	assert.Equal(t, filepath.Join(baseDir, ".cswdata", "tasks", created.UUID), runner.lastRequest.TaskDir)
	assert.Equal(t, TaskStateCompleted, outcome.Task.State)
	assert.Equal(t, TaskStatusOpen, outcome.Task.Status)
	assert.Equal(t, "ses-123", outcome.SessionID)
	assert.Contains(t, outcome.TaskBranchName, "feat/run-task-")

	require.GreaterOrEqual(t, len(fake.mergeCalls), 1)
	assert.Equal(t, "feat/run", fake.mergeCalls[0][0])
	assert.Equal(t, outcome.TaskBranchName, fake.mergeCalls[0][1])

	taskDir := filepath.Join(baseDir, ".cswdata", "tasks", created.UUID)
	summaryMetaBytes, readMetaErr := os.ReadFile(filepath.Join(taskDir, "ses-ses-123", "summary.yml"))
	require.NoError(t, readMetaErr)
	var meta TaskSessionSummary
	require.NoError(t, yaml.Unmarshal(summaryMetaBytes, &meta))
	assert.Equal(t, TaskStateCompleted, meta.Status)

	summaryTextBytes, readSummaryErr := os.ReadFile(filepath.Join(taskDir, "ses-ses-123", "summary.md"))
	require.NoError(t, readSummaryErr)
	assert.Equal(t, "Task completed\n", string(summaryTextBytes))

	outputBytes, readOutputErr := os.ReadFile(filepath.Join(taskDir, "output.md"))
	require.NoError(t, readOutputErr)
	assert.Contains(t, string(outputBytes), "task_id: "+created.UUID)
	assert.Contains(t, string(outputBytes), "Task completed")
}

func TestTaskManagerRunTaskFailurePersistsFailedSummary(t *testing.T) {
	baseDir := t.TempDir()
	runner := &taskTestRunner{
		result: TaskSessionRunResult{SessionID: "ses-fail", SummaryText: "partial summary"},
		err:    errors.New("runner boom"),
	}
	manager, err := NewTaskManager(baseDir, nil, runner)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "run-task", FeatureBranch: "feat/run", Prompt: "run prompt"})
	require.NoError(t, err)

	fake := &fakeVCS{branches: []string{"main"}}
	outcome, runErr := manager.RunTask(context.Background(), TaskLookup{Identifier: created.UUID}, TaskRunParams{}, fake)
	require.Error(t, runErr)
	require.NotNil(t, outcome)
	assert.Contains(t, runErr.Error(), "task run failed")
	assert.Equal(t, TaskStateFailed, outcome.Task.State)
	assert.Equal(t, TaskStatusOpen, outcome.Task.Status)
	assert.Equal(t, TaskStateFailed, outcome.SummaryMeta.Status)
	assert.Empty(t, fake.mergeCalls)
}

func TestTaskManagerRunTaskBlocksWhenDependencyNotCompleted(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil, &taskTestRunner{})
	require.NoError(t, err)

	dep, err := manager.CreateTask(TaskCreateParams{Name: "dep", Prompt: "dep prompt"})
	require.NoError(t, err)
	mainTask, err := manager.CreateTask(TaskCreateParams{Name: "main", Prompt: "main prompt", Deps: []string{dep.UUID}})
	require.NoError(t, err)

	fake := &fakeVCS{branches: []string{"main"}}
	outcome, runErr := manager.RunTask(context.Background(), TaskLookup{Identifier: mainTask.UUID}, TaskRunParams{}, fake)
	require.Error(t, runErr)
	assert.Nil(t, outcome)
	assert.Contains(t, runErr.Error(), "is not completed")
}

func TestTaskManagerRunTaskContinueRendersTemplate(t *testing.T) {
	baseDir := t.TempDir()
	runner := &taskTestRunner{result: TaskSessionRunResult{SessionID: "ses-continue", SummaryText: "ok"}}
	store := confimpl.NewMockConfigStore()
	store.SetAgentConfigFile("continue", "prompt.md", []byte("Task={{.TaskName}} Prompt={{.Prompt}}"))

	manager, err := NewTaskManager(baseDir, store, runner)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "cont", Prompt: "base prompt"})
	require.NoError(t, err)

	fake := &fakeVCS{branches: []string{"main"}}
	_, runErr := manager.RunTask(context.Background(), TaskLookup{Identifier: created.UUID}, TaskRunParams{Continue: true}, fake)
	require.NoError(t, runErr)
	assert.Contains(t, runner.lastRequest.Prompt, "Task=cont")
	assert.Contains(t, runner.lastRequest.Prompt, "Prompt=base prompt")
}

func TestTaskManagerMergeTaskUpdatesStatusAndState(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil, &taskTestRunner{})
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "merge-me", FeatureBranch: "feat/merge", ParentBranch: "main", Prompt: "prompt"})
	require.NoError(t, err)

	fake := &fakeVCS{branches: []string{"main", "feat/merge"}}
	merged, err := manager.MergeTask(TaskLookup{Identifier: created.UUID}, fake)
	require.NoError(t, err)
	require.NotNil(t, merged)

	require.Len(t, fake.mergeCalls, 1)
	assert.Equal(t, [2]string{"main", "feat/merge"}, fake.mergeCalls[0])
	assert.Equal(t, TaskStatusMerged, merged.Status)
	assert.Equal(t, TaskStateCompleted, merged.State)
}

func TestEnsureBranchFromCreatesMissingBranch(t *testing.T) {
	fake := &fakeVCS{branches: []string{"main"}}
	err := ensureBranchFrom(fake, "feature/task", "main")
	require.NoError(t, err)
	require.Len(t, fake.newBranchCalls, 1)
	assert.Equal(t, [2]string{"feature/task", "main"}, fake.newBranchCalls[0])
}

func TestExtractTaskSessionID(t *testing.T) {
	output := "line1\nSession ID: ses-42\nline3"
	assert.Equal(t, "ses-42", extractTaskSessionID(output))
	assert.Equal(t, "", extractTaskSessionID("no session id here"))
}

func TestReadCLISessionSummary(t *testing.T) {
	baseDir := t.TempDir()
	sessionID := "ses-777"
	summaryDir := filepath.Join(baseDir, ".cswdata", "logs", "sessions", sessionID)
	require.NoError(t, os.MkdirAll(summaryDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(summaryDir, "summary.md"), []byte("\n  done \n"), 0644))

	summary, err := readCLISessionSummary(baseDir, sessionID)
	require.NoError(t, err)
	assert.Equal(t, "done", summary)
}

func TestCLITaskSessionRunnerIncludesTaskFlags(t *testing.T) {
	runner, err := NewCLITaskSessionRunner("/tmp/project", "model-name", "/tmp/conf", "/tmp/project/.csw/config", "high")
	require.NoError(t, err)

	request := TaskSessionRunRequest{
		TaskBranch: "feature/task",
		Role:       "developer",
		Prompt:     "do work",
		Task:       &Task{UUID: "task-uuid", Name: "task-name"},
		TaskDir:    ".cswdata/tasks/task-uuid",
		RunOptions: TaskSessionRunOptions{
			OutputFormat: "full",
		},
	}

	args, err := runner.buildCLIArgs(request)
	require.NoError(t, err)

	assert.Contains(t, args, "run")
	assert.Contains(t, args, "--task-json")
	assert.Contains(t, args, "--task-dir")
	assert.Contains(t, args, ".cswdata/tasks/task-uuid")
	assert.Equal(t, "do work", args[len(args)-1])
}

func TestCLITaskSessionRunnerIncludesRunOptionsFlags(t *testing.T) {
	runner, err := NewCLITaskSessionRunner("/tmp/project", "", "", "", "")
	require.NoError(t, err)

	request := TaskSessionRunRequest{
		TaskBranch: "feature/task",
		Prompt:     "do work",
		RunOptions: TaskSessionRunOptions{
			Model:             "provider/model",
			Role:              "reviewer",
			ShadowDir:         "/shadow",
			ContainerEnabled:  true,
			ContainerImage:    "img:latest",
			ContainerMounts:   []string{"/host:/container"},
			ContainerEnv:      []string{"A=B"},
			AllowAllPerms:     true,
			Interactive:       true,
			ConfigPath:        "/cfg",
			ProjectConfig:     "/project/.csw/config",
			SaveSessionTo:     "/tmp/ses.md",
			SaveSession:       true,
			LogLLMRequests:    true,
			LogLLMRequestsRaw: true,
			NoRefresh:         true,
			LSPServer:         "gopls",
			Thinking:          "high",
			ForceCompact:      true,
			BashRunTimeout:    "45s",
			MaxThreads:        3,
			OutputFormat:      "jsonl",
			VFSAllow:          []string{"/allow"},
			MCPEnable:         []string{"m1,m2"},
			MCPDisable:        []string{"m3"},
			HookOverrides:     []string{"h:disable"},
			ContextEntries:    []string{"k=v"},
			GitUserName:       "John",
			GitUserEmail:      "john@example.com",
		},
	}

	args, err := runner.buildCLIArgs(request)
	require.NoError(t, err)

	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "provider/model")
	assert.Contains(t, args, "--container-enabled")
	assert.Contains(t, args, "--container-image")
	assert.Contains(t, args, "img:latest")
	assert.Contains(t, args, "--max-threads")
	assert.Contains(t, args, "3")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "jsonl")
}
