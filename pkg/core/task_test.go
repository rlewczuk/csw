package core

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	manager, err := NewTaskManager("   ", nil)
	require.Error(t, err)
	assert.Nil(t, manager)
	assert.Contains(t, err.Error(), "baseDir cannot be empty")
}

func TestTaskManagerCreateTaskUsesAbsoluteTasksDirWithoutPrefixingBaseDir(t *testing.T) {
	baseDir := t.TempDir()
	tasksDir := filepath.Join(baseDir, ".cswdata", "tasks")

	manager, err := NewTaskManagerWithTasksDir(baseDir, tasksDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task-name", Prompt: "prompt"})
	require.NoError(t, err)

	expectedTaskPath := filepath.Join(tasksDir, created.UUID, "task.yml")
	_, expectedTaskPathErr := os.Stat(expectedTaskPath)
	require.NoError(t, expectedTaskPathErr)

	incorrectTaskPath := filepath.Join(baseDir, tasksDir, created.UUID, "task.yml")
	_, incorrectTaskPathErr := os.Stat(incorrectTaskPath)
	assert.ErrorIs(t, incorrectTaskPathErr, os.ErrNotExist)
}

func TestTaskManagerCreateTaskDefaultsAndNormalizeDeps(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
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
	manager, err := NewTaskManager(baseDir, nil)
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
	manager, err := NewTaskManager(baseDir, nil)
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
	manager, err := NewTaskManager(baseDir, nil)
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
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "old", Prompt: "old prompt"})
	require.NoError(t, err)

	name := "new-name"
	description := "new description"
	status := TaskStatusDraft
	feature := "feature/new"
	parentBranch := "develop"
	role := "reviewer"
	deps := []string{"dep-2", "dep-1", "dep-1"}
	prompt := "new prompt"

	updated, err := manager.UpdateTask(TaskUpdateParams{
		Identifier:    created.UUID,
		Name:          &name,
		Description:   &description,
		Status:        &status,
		FeatureBranch: &feature,
		ParentBranch:  &parentBranch,
		Role:          &role,
		Deps:          &deps,
		Prompt:        &prompt,
	})
	require.NoError(t, err)

	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, "new description", updated.Description)
	assert.Equal(t, TaskStatusDraft, updated.Status)
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
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task", Prompt: "prompt"})
	require.NoError(t, err)

	empty := "  "
	updated, err := manager.UpdateTask(TaskUpdateParams{Identifier: created.UUID, Prompt: &empty})
	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Contains(t, err.Error(), "prompt cannot be empty")
}

func TestTaskManagerUpdateTaskSupportsNameIdentifier(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task-by-name", Prompt: "prompt"})
	require.NoError(t, err)

	status := TaskStatusOpen
	updated, err := manager.UpdateTask(TaskUpdateParams{Identifier: created.Name, Status: &status})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.UUID, updated.UUID)
	assert.Equal(t, TaskStatusOpen, updated.Status)
}

func TestTaskManagerUpdateTaskRejectsEmptyStatus(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task", Prompt: "prompt"})
	require.NoError(t, err)

	empty := "  "
	updated, err := manager.UpdateTask(TaskUpdateParams{Identifier: created.UUID, Status: &empty})
	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Contains(t, err.Error(), "status cannot be empty")
}

func TestTaskManagerUpdateTaskRejectsUnsupportedStatus(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task", Prompt: "prompt"})
	require.NoError(t, err)

	unsupported := "archived"
	updated, err := manager.UpdateTask(TaskUpdateParams{Identifier: created.UUID, Status: &unsupported})
	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Contains(t, err.Error(), "unsupported task status")
}

func TestTaskManagerUpdateTaskKeepsUpdatedAtWhenNothingChanges(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task", Prompt: "same prompt"})
	require.NoError(t, err)

	name := "task"
	prompt := "same prompt"
	updated, err := manager.UpdateTask(TaskUpdateParams{Identifier: created.UUID, Name: &name, Prompt: &prompt})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.UpdatedAt, updated.UpdatedAt)
}

func TestTaskManagerUpdateTaskChangesUpdatedAtWhenPromptChanges(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task", Prompt: "old prompt"})
	require.NoError(t, err)

	time.Sleep(2 * time.Millisecond)
	prompt := "new prompt"
	updated, err := manager.UpdateTask(TaskUpdateParams{Identifier: created.UUID, Prompt: &prompt})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.NotEqual(t, created.UpdatedAt, updated.UpdatedAt)
}

func TestTaskManagerListTasksTopLevelAndRecursiveChildren(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
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

func TestTaskManagerResolveTaskSetsTaskDirFromFilesystemPath(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "task-with-dir", Prompt: "prompt"})
	require.NoError(t, err)

	taskDir, resolved, err := manager.ResolveTask(TaskLookup{Identifier: created.UUID})
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, taskDir, resolved.TaskDir)
	assert.Equal(t, filepath.Join(baseDir, ".cswdata", "tasks", created.UUID), resolved.TaskDir)

	taskFilePath := filepath.Join(taskDir, "task.yml")
	taskFileBytes, readErr := os.ReadFile(taskFilePath)
	require.NoError(t, readErr)
	assert.NotContains(t, string(taskFileBytes), "task_dir")
}

func TestTaskManagerMergeTaskUpdatesStatus(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
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
}

func TestTaskManagerArchiveTaskMovesTaskDirectory(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	created, err := manager.CreateTask(TaskCreateParams{Name: "archive-me", Prompt: "prompt"})
	require.NoError(t, err)

	archived, err := manager.ArchiveTask(TaskLookup{Identifier: created.UUID})
	require.NoError(t, err)
	require.NotNil(t, archived)
	assert.Equal(t, created.UUID, archived.UUID)

	_, _, resolveErr := manager.ResolveTask(TaskLookup{Identifier: created.UUID})
	require.Error(t, resolveErr)
	assert.Contains(t, resolveErr.Error(), "not found")

	archivedTaskPath := filepath.Join(baseDir, ".cswdata", "tasks", "archive", created.UUID, "task.yml")
	_, statErr := os.Stat(archivedTaskPath)
	require.NoError(t, statErr)
}

func TestTaskManagerArchiveTaskPreservesNestedPath(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	parent, err := manager.CreateTask(TaskCreateParams{Name: "parent", Prompt: "parent"})
	require.NoError(t, err)
	child, err := manager.CreateTask(TaskCreateParams{Name: "child", ParentTaskID: parent.UUID, Prompt: "child"})
	require.NoError(t, err)

	_, err = manager.ArchiveTask(TaskLookup{Identifier: child.Name})
	require.NoError(t, err)

	archivedChildPath := filepath.Join(baseDir, ".cswdata", "tasks", "archive", parent.UUID, child.UUID, "task.yml")
	_, statErr := os.Stat(archivedChildPath)
	require.NoError(t, statErr)
}

func TestTaskManagerListTasksSkipsArchiveContainerDirectory(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	activeTask, err := manager.CreateTask(TaskCreateParams{Name: "active", Prompt: "prompt"})
	require.NoError(t, err)
	archivedTask, err := manager.CreateTask(TaskCreateParams{Name: "archived", Prompt: "prompt"})
	require.NoError(t, err)
	_, err = manager.ArchiveTask(TaskLookup{Identifier: archivedTask.UUID})
	require.NoError(t, err)

	tasks, err := manager.ListTasks(TaskLookup{}, false)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, activeTask.UUID, tasks[0].UUID)
}

func TestTaskManagerArchiveTasksByStatusArchivesMatchingTasks(t *testing.T) {
	baseDir := t.TempDir()
	manager, err := NewTaskManager(baseDir, nil)
	require.NoError(t, err)

	mergedTask, err := manager.CreateTask(TaskCreateParams{Name: "merged-task", FeatureBranch: "feat/merged", ParentBranch: "main", Prompt: "prompt"})
	require.NoError(t, err)
	openTask, err := manager.CreateTask(TaskCreateParams{Name: "open-task", Prompt: "prompt"})
	require.NoError(t, err)

	fake := &fakeVCS{branches: []string{"main", "feat/merged"}}
	_, err = manager.MergeTask(TaskLookup{Identifier: mergedTask.UUID}, fake)
	require.NoError(t, err)

	archivedTasks, err := manager.ArchiveTasksByStatus(TaskStatusMerged)
	require.NoError(t, err)
	require.Len(t, archivedTasks, 1)
	assert.Equal(t, mergedTask.UUID, archivedTasks[0].UUID)

	mergedArchivedPath := filepath.Join(baseDir, ".cswdata", "tasks", "archive", mergedTask.UUID, "task.yml")
	_, mergedArchivedErr := os.Stat(mergedArchivedPath)
	require.NoError(t, mergedArchivedErr)

	openTaskPath := filepath.Join(baseDir, ".cswdata", "tasks", openTask.UUID, "task.yml")
	_, openTaskErr := os.Stat(openTaskPath)
	require.NoError(t, openTaskErr)
}
