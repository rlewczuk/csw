package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"gopkg.in/yaml.v3"
)

const (
	// TaskStatusDraft indicates task is prepared but not selectable for execution.
	TaskStatusDraft = "draft"
	// TaskStatusCreated indicates task is created and not yet started.
	TaskStatusCreated = "created"
	// TaskStatusOpen indicates task can be worked on.
	TaskStatusOpen = "open"
	// TaskStatusRunning indicates task is currently being executed.
	TaskStatusRunning = "running"
	// TaskStatusSubtasks indicates task has subtasks and is waiting for them to complete.
	TaskStatusSubtasks = "subtasks"
	// TaskStatusMerged indicates task result was merged to parent branch.
	TaskStatusMerged = "merged"
	// TaskStatusCompleted indicates task completed successfully.
	TaskStatusCompleted = "completed"
	// TaskStatusFailed indicates task execution failed.
	TaskStatusFailed = "failed"
)

// Task stores persistent task metadata.
type Task struct {
	UUID          string   `json:"uuid" yaml:"uuid"`
	Name          string   `json:"name,omitempty" yaml:"name,omitempty"`
	Description   string   `json:"description,omitempty" yaml:"description,omitempty"`
	TaskDir       string   `json:"-" yaml:"-"`
	Status        string   `json:"status" yaml:"status"`
	FeatureBranch string   `json:"feature_branch" yaml:"feature_branch"`
	ParentBranch  string   `json:"parent_branch" yaml:"parent_branch"`
	Role          string   `json:"role,omitempty" yaml:"role,omitempty"`
	Deps          []string `json:"deps,omitempty" yaml:"deps,omitempty"`
	SessionIDs    []string `json:"session_ids,omitempty" yaml:"session_ids,omitempty"`
	SubtaskIDs    []string `json:"subtask_ids,omitempty" yaml:"subtask_ids,omitempty"`
	ParentTaskID  string   `json:"parent_task_id,omitempty" yaml:"parent_task_id,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// TaskSessionSummary stores persisted session summary metadata.
type TaskSessionSummary struct {
	SessionID   string `json:"session_id" yaml:"session_id"`
	Status      string `json:"status" yaml:"status"`
	StartedAt   string `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
	TaskID      string `json:"task_id" yaml:"task_id"`
}

// TaskOutputMetadata stores metadata section for task output file.
type TaskOutputMetadata struct {
	TaskID        string `json:"task_id" yaml:"task_id"`
	TaskName      string `json:"task_name,omitempty" yaml:"task_name,omitempty"`
	Status        string `json:"status" yaml:"status"`
	UpdatedAt     string `json:"updated_at" yaml:"updated_at"`
	LastSessionID string `json:"last_session_id,omitempty" yaml:"last_session_id,omitempty"`
}

// TaskCreateParams defines parameters for creating a task.
type TaskCreateParams struct {
	ParentTaskID  string
	Name          string
	Description   string
	FeatureBranch string
	ParentBranch  string
	Role          string
	Deps          []string
	Prompt        string
}

// TaskUpdateParams defines parameters for updating a task.
type TaskUpdateParams struct {
	Identifier    string
	Name          *string
	Description   *string
	Status        *string
	FeatureBranch *string
	ParentBranch  *string
	Role          *string
	Deps          *[]string
	Prompt        *string
}

// TaskRunParams defines parameters for running a task.
type TaskRunParams struct {
	Identifier     string
	Merge          bool
	Reset          bool
	PromptOverride string
	PromptArgs     []string
	RunOptions     TaskSessionRunOptions
}

// TaskSessionRunOptions stores session run CLI options used for task execution.
type TaskSessionRunOptions struct {
	Model             string
	Role              string
	WorkDir           string
	ShadowDir         string
	ContainerImage    string
	ContainerEnabled  bool
	ContainerDisabled bool
	ContainerMounts   []string
	ContainerEnv      []string
	AllowAllPerms     bool
	Interactive       bool
	ConfigPath        string
	ProjectConfig     string
	SaveSessionTo     string
	SaveSession       bool
	LogLLMRequests    bool
	LogLLMRequestsRaw bool
	NoRefresh         bool
	LSPServer         string
	Thinking          string
	BashRunTimeout    string
	MaxThreads        int
	OutputFormat      string
	VFSAllow          []string
	MCPEnable         []string
	MCPDisable        []string
	HookOverrides     []string
	ContextEntries    []string
	GitUserName       string
	GitUserEmail      string
}

// TaskRunResult stores run outcome information.
type TaskRunResult struct {
	Task           *Task
	SessionID      string
	SummaryMeta    *TaskSessionSummary
	SummaryText    string
	Merged         bool
	TaskBranchName string
}

// TaskLookup identifies a task by name, UUID, or fallback current task.
type TaskLookup struct {
	Identifier     string
	FallbackTaskID string
}

// TaskSessionRunRequest defines task session execution input.
type TaskSessionRunRequest struct {
	TaskID        string
	TaskName      string
	Task          *Task
	TaskDir       string
	TaskBranch    string
	FeatureBranch string
	ParentBranch  string
	Role          string
	Prompt        string
	RunOptions    TaskSessionRunOptions
	VCS           apis.VCS
}

// TaskSessionRunResult defines task session execution output.
type TaskSessionRunResult struct {
	SessionID   string
	SummaryText string
	StartedAt   time.Time
	CompletedAt time.Time
}

var taskDirUUIDPattern = regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

// TaskManager manages persistent hierarchical tasks.
type TaskManager struct {
	baseDir     string
	tasksDir    string
	configStore conf.ConfigStore
	uuidFn      func() string
	nowFn       func() time.Time
}

// TaskBackendAdapter exposes TaskManager through tool.TaskBackend interface.
type TaskBackendAdapter struct {
	manager *TaskManager
	vcsRepo apis.VCS
	logger  *slog.Logger
}

// NewTaskManager creates a new TaskManager.
func NewTaskManager(baseDir string, configStore conf.ConfigStore) (*TaskManager, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, fmt.Errorf("NewTaskManager() [task.go]: baseDir cannot be empty")
	}

	return NewTaskManagerWithTasksDir(baseDir, ".cswdata/tasks", configStore)
}

// NewTaskManagerWithTasksDir creates a new TaskManager with custom tasks directory.
func NewTaskManagerWithTasksDir(baseDir string, tasksDir string, configStore conf.ConfigStore) (*TaskManager, error) {
	trimmedBaseDir := strings.TrimSpace(baseDir)
	if trimmedBaseDir == "" {
		return nil, fmt.Errorf("NewTaskManagerWithTasksDir() [task.go]: baseDir cannot be empty")
	}
	trimmedTasksDir := strings.TrimSpace(tasksDir)
	if trimmedTasksDir == "" {
		return nil, fmt.Errorf("NewTaskManagerWithTasksDir() [task.go]: tasksDir cannot be empty")
	}

	return &TaskManager{
		baseDir:     trimmedBaseDir,
		tasksDir:    trimmedTasksDir,
		configStore: configStore,
		uuidFn:      shared.GenerateUUIDv7,
		nowFn:       time.Now,
	}, nil
}

// NewTaskBackendAdapter creates task backend adapter for tools.
func NewTaskBackendAdapter(manager *TaskManager, vcsRepo apis.VCS, logger *slog.Logger) (*TaskBackendAdapter, error) {
	if manager == nil {
		return nil, fmt.Errorf("NewTaskBackendAdapter() [task.go]: manager is nil")
	}

	return &TaskBackendAdapter{manager: manager, vcsRepo: vcsRepo, logger: logger}, nil
}

// CreateTask creates task through backend interface.
func (a *TaskBackendAdapter) CreateTask(ctx context.Context, params tool.TaskRecord, prompt string, parentTaskID string) (tool.TaskRecord, error) {
	_ = ctx
	if a == nil || a.manager == nil {
		return tool.TaskRecord{}, fmt.Errorf("TaskBackendAdapter.CreateTask() [task.go]: adapter is not configured")
	}

	created, err := a.manager.CreateTask(TaskCreateParams{
		ParentTaskID:  strings.TrimSpace(parentTaskID),
		Name:          strings.TrimSpace(params.Name),
		Description:   strings.TrimSpace(params.Description),
		FeatureBranch: strings.TrimSpace(params.FeatureBranch),
		ParentBranch:  strings.TrimSpace(params.ParentBranch),
		Role:          strings.TrimSpace(params.Role),
		Deps:          append([]string(nil), params.Deps...),
		Prompt:        strings.TrimSpace(prompt),
	})
	if err != nil {
		return tool.TaskRecord{}, err
	}

	return toToolTaskRecord(created), nil
}

// UpdateTask updates task through backend interface.
func (a *TaskBackendAdapter) UpdateTask(ctx context.Context, identifier string, params tool.TaskRecord, prompt *string) (tool.TaskRecord, error) {
	_ = ctx
	if a == nil || a.manager == nil {
		return tool.TaskRecord{}, fmt.Errorf("TaskBackendAdapter.UpdateTask() [task.go]: adapter is not configured")
	}

	update := TaskUpdateParams{Identifier: strings.TrimSpace(identifier)}
	if strings.TrimSpace(params.Name) != "" {
		value := strings.TrimSpace(params.Name)
		update.Name = &value
	}
	if params.Description != "" {
		value := strings.TrimSpace(params.Description)
		update.Description = &value
	}
	if params.Status != "" {
		value := strings.TrimSpace(params.Status)
		update.Status = &value
	}
	if strings.TrimSpace(params.FeatureBranch) != "" {
		value := strings.TrimSpace(params.FeatureBranch)
		update.FeatureBranch = &value
	}
	if strings.TrimSpace(params.ParentBranch) != "" {
		value := strings.TrimSpace(params.ParentBranch)
		update.ParentBranch = &value
	}
	if params.Role != "" {
		value := strings.TrimSpace(params.Role)
		update.Role = &value
	}
	if params.Deps != nil {
		value := append([]string(nil), params.Deps...)
		update.Deps = &value
	}
	if prompt != nil {
		trimmedPrompt := strings.TrimSpace(*prompt)
		update.Prompt = &trimmedPrompt
	}

	updated, err := a.manager.UpdateTask(update)
	if err != nil {
		return tool.TaskRecord{}, err
	}

	return toToolTaskRecord(updated), nil
}

// GetTask gets task through backend interface.
func (a *TaskBackendAdapter) GetTask(ctx context.Context, identifier string, fallbackTaskID string, includeSummary bool) (tool.TaskRecord, *tool.TaskSessionSummary, string, error) {
	_ = ctx
	if a == nil || a.manager == nil {
		return tool.TaskRecord{}, nil, "", fmt.Errorf("TaskBackendAdapter.GetTask() [task.go]: adapter is not configured")
	}

	taskData, summaryMeta, summaryText, err := a.manager.GetTask(TaskLookup{Identifier: strings.TrimSpace(identifier), FallbackTaskID: strings.TrimSpace(fallbackTaskID)}, includeSummary)
	if err != nil {
		return tool.TaskRecord{}, nil, "", err
	}

	var summary *tool.TaskSessionSummary
	if summaryMeta != nil {
		summary = &tool.TaskSessionSummary{
			SessionID:   summaryMeta.SessionID,
			Status:      summaryMeta.Status,
			StartedAt:   summaryMeta.StartedAt,
			CompletedAt: summaryMeta.CompletedAt,
			TaskID:      summaryMeta.TaskID,
		}
	}

	return toToolTaskRecord(taskData), summary, summaryText, nil
}

// ListTasks lists tasks through backend interface.
func (a *TaskBackendAdapter) ListTasks(ctx context.Context, identifier string, fallbackTaskID string, recursive bool) ([]tool.TaskRecord, error) {
	_ = ctx
	if a == nil || a.manager == nil {
		return nil, fmt.Errorf("TaskBackendAdapter.ListTasks() [task.go]: adapter is not configured")
	}

	tasks, err := a.manager.ListTasks(TaskLookup{Identifier: strings.TrimSpace(identifier), FallbackTaskID: strings.TrimSpace(fallbackTaskID)}, recursive)
	if err != nil {
		return nil, err
	}

	result := make([]tool.TaskRecord, 0, len(tasks))
	for _, item := range tasks {
		result = append(result, toToolTaskRecord(item))
	}

	return result, nil
}

// MergeTask merges task through backend interface.
func (a *TaskBackendAdapter) MergeTask(ctx context.Context, identifier string, fallbackTaskID string) (tool.TaskRecord, error) {
	_ = ctx
	if a == nil || a.manager == nil {
		return tool.TaskRecord{}, fmt.Errorf("TaskBackendAdapter.MergeTask() [task.go]: adapter is not configured")
	}
	if a.vcsRepo == nil {
		return tool.TaskRecord{}, fmt.Errorf("TaskBackendAdapter.MergeTask() [task.go]: vcs repository is not configured")
	}

	merged, err := a.manager.MergeTask(TaskLookup{Identifier: strings.TrimSpace(identifier), FallbackTaskID: strings.TrimSpace(fallbackTaskID)}, a.vcsRepo)
	if err != nil {
		return tool.TaskRecord{}, err
	}

	return toToolTaskRecord(merged), nil
}

func toToolTaskRecord(taskData *Task) tool.TaskRecord {
	if taskData == nil {
		return tool.TaskRecord{}
	}

	return tool.TaskRecord{
		UUID:          taskData.UUID,
		Name:          taskData.Name,
		Description:   taskData.Description,
		Status:        taskData.Status,
		FeatureBranch: taskData.FeatureBranch,
		ParentBranch:  taskData.ParentBranch,
		Role:          taskData.Role,
		Deps:          append([]string(nil), taskData.Deps...),
		SessionIDs:    append([]string(nil), taskData.SessionIDs...),
		SubtaskIDs:    append([]string(nil), taskData.SubtaskIDs...),
		ParentTaskID:  taskData.ParentTaskID,
		CreatedAt:     taskData.CreatedAt,
		UpdatedAt:     taskData.UpdatedAt,
	}
}

// TasksRoot returns root directory for task persistence.
func (m *TaskManager) TasksRoot() string {
	if filepath.IsAbs(m.tasksDir) {
		return filepath.Clean(m.tasksDir)
	}

	return filepath.Join(m.baseDir, m.tasksDir)
}

// CreateTask creates a new persistent task and prompt file.
func (m *TaskManager) CreateTask(params TaskCreateParams) (*Task, error) {
	now := m.nowFn().UTC().Format(time.RFC3339Nano)
	taskID := m.uuidFn()
	name := strings.TrimSpace(params.Name)
	if name == "" {
		name = taskID
	}
	featureBranch := strings.TrimSpace(params.FeatureBranch)
	if featureBranch == "" {
		featureBranch = name
	}
	parentBranch := strings.TrimSpace(params.ParentBranch)
	if parentBranch == "" {
		parentBranch = "main"
	}
	taskDir := filepath.Join(m.TasksRoot(), taskID)
	parentTaskID := strings.TrimSpace(params.ParentTaskID)
	if parentTaskID != "" {
		parentDir, _, err := m.findTaskByUUID(parentTaskID)
		if err != nil {
			return nil, fmt.Errorf("TaskManager.CreateTask() [task.go]: failed to resolve parent task: %w", err)
		}
		taskDir = filepath.Join(parentDir, taskID)
	}

	task := &Task{
		UUID:          taskID,
		Name:          name,
		Description:   strings.TrimSpace(params.Description),
		TaskDir:       taskDir,
		Status:        TaskStatusCreated,
		FeatureBranch: featureBranch,
		ParentBranch:  parentBranch,
		Role:          strings.TrimSpace(params.Role),
		Deps:          normalizeTaskDeps(params.Deps),
		SessionIDs:    []string{},
		SubtaskIDs:    []string{},
		ParentTaskID:  parentTaskID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return nil, fmt.Errorf("TaskManager.CreateTask() [task.go]: failed to create task directory: %w", err)
	}

	if err := m.writeTaskFile(taskDir, task); err != nil {
		return nil, err
	}
	promptContent := strings.TrimSpace(params.Prompt)
	promptBytes := []byte{}
	if promptContent != "" {
		promptBytes = []byte(promptContent + "\n")
	}
	if err := os.WriteFile(filepath.Join(taskDir, "task.md"), promptBytes, 0644); err != nil {
		return nil, fmt.Errorf("TaskManager.CreateTask() [task.go]: failed to write task prompt: %w", err)
	}

	if strings.TrimSpace(task.ParentTaskID) != "" {
		if err := m.appendSubtask(task.ParentTaskID, task.UUID); err != nil {
			return nil, err
		}
	}

	return task, nil
}

// UpdateTask updates selected task metadata and prompt.
func (m *TaskManager) UpdateTask(params TaskUpdateParams) (*Task, error) {
	taskDir, task, err := m.ResolveTask(TaskLookup{Identifier: params.Identifier})
	if err != nil {
		return nil, err
	}

	taskChanged := false

	if params.Name != nil {
		trimmed := strings.TrimSpace(*params.Name)
		if trimmed != "" && trimmed != strings.TrimSpace(task.Name) {
			task.Name = trimmed
			taskChanged = true
		}
	}
	if params.Description != nil {
		trimmed := strings.TrimSpace(*params.Description)
		if trimmed != strings.TrimSpace(task.Description) {
			task.Description = trimmed
			taskChanged = true
		}
	}
	if params.Status != nil {
		trimmed := strings.TrimSpace(*params.Status)
		if trimmed == "" {
			return nil, fmt.Errorf("TaskManager.UpdateTask() [task.go]: status cannot be empty")
		}
		if !isTaskStatusSupported(trimmed) {
			return nil, fmt.Errorf("TaskManager.UpdateTask() [task.go]: unsupported task status %q", trimmed)
		}
		if trimmed != strings.TrimSpace(task.Status) {
			task.Status = trimmed
			taskChanged = true
		}
	}
	if params.FeatureBranch != nil {
		trimmed := strings.TrimSpace(*params.FeatureBranch)
		if trimmed != "" && trimmed != strings.TrimSpace(task.FeatureBranch) {
			task.FeatureBranch = trimmed
			taskChanged = true
		}
	}
	if params.ParentBranch != nil {
		trimmed := strings.TrimSpace(*params.ParentBranch)
		if trimmed != "" && trimmed != strings.TrimSpace(task.ParentBranch) {
			task.ParentBranch = trimmed
			taskChanged = true
		}
	}
	if params.Role != nil {
		trimmed := strings.TrimSpace(*params.Role)
		if trimmed != strings.TrimSpace(task.Role) {
			task.Role = trimmed
			taskChanged = true
		}
	}
	if params.Deps != nil {
		normalizedDeps := normalizeTaskDeps(*params.Deps)
		if strings.Join(normalizedDeps, "\x00") != strings.Join(task.Deps, "\x00") {
			task.Deps = normalizedDeps
			taskChanged = true
		}
	}
	if params.Prompt != nil {
		resolvedPrompt := strings.TrimSpace(*params.Prompt)
		if resolvedPrompt == "" {
			return nil, fmt.Errorf("TaskManager.UpdateTask() [task.go]: prompt cannot be empty")
		}
		promptPath := filepath.Join(taskDir, "task.md")
		existingPrompt := ""
		existingPromptBytes, readErr := os.ReadFile(promptPath)
		if readErr != nil {
			if !errors.Is(readErr, os.ErrNotExist) {
				return nil, fmt.Errorf("TaskManager.UpdateTask() [task.go]: failed to read existing task prompt: %w", readErr)
			}
		} else {
			existingPrompt = strings.TrimSpace(string(existingPromptBytes))
		}

		if resolvedPrompt != existingPrompt {
			if err := os.WriteFile(promptPath, []byte(resolvedPrompt+"\n"), 0644); err != nil {
				return nil, fmt.Errorf("TaskManager.UpdateTask() [task.go]: failed to update task prompt: %w", err)
			}
			taskChanged = true
		}
	}

	if !taskChanged {
		return task, nil
	}

	task.UpdatedAt = m.nowFn().UTC().Format(time.RFC3339Nano)
	if err := m.writeTaskFile(taskDir, task); err != nil {
		return nil, err
	}

	return task, nil
}

// ResolveTask resolves task by UUID or name.
func (m *TaskManager) ResolveTask(lookup TaskLookup) (string, *Task, error) {
	identifier := strings.TrimSpace(lookup.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(lookup.FallbackTaskID)
	}
	if identifier == "" {
		return "", nil, fmt.Errorf("TaskManager.ResolveTask() [task.go]: task identifier cannot be empty")
	}

	if dir, task, err := m.findTaskByUUID(identifier); err == nil {
		return dir, task, nil
	}

	tasks, err := m.loadAllTasks()
	if err != nil {
		return "", nil, err
	}
	for _, item := range tasks {
		if item.task != nil && strings.TrimSpace(item.task.Name) == identifier {
			return item.dir, item.task, nil
		}
	}

	return "", nil, fmt.Errorf("TaskManager.ResolveTask() [task.go]: task %q not found", identifier)
}

// GetTask returns task metadata with optional last summary.
func (m *TaskManager) GetTask(lookup TaskLookup, includeSummary bool) (*Task, *TaskSessionSummary, string, error) {
	taskDir, task, err := m.ResolveTask(lookup)
	if err != nil {
		return nil, nil, "", err
	}
	if !includeSummary {
		return task, nil, "", nil
	}
	summaryMeta, summaryText, _ := m.readLastSessionSummary(taskDir, task)
	return task, summaryMeta, summaryText, nil
}

// ListTasks lists tasks optionally under given parent and recursively.
func (m *TaskManager) ListTasks(lookup TaskLookup, recursive bool) ([]*Task, error) {
	if strings.TrimSpace(lookup.Identifier) == "" && strings.TrimSpace(lookup.FallbackTaskID) == "" {
		all, err := m.loadAllTasks()
		if err != nil {
			return nil, err
		}
		result := make([]*Task, 0, len(all))
		for _, item := range all {
			if item.task != nil && strings.TrimSpace(item.task.ParentTaskID) == "" {
				result = append(result, cloneTask(item.task))
			}
		}
		sortTasks(result)
		return result, nil
	}

	taskDir, _, err := m.ResolveTask(lookup)
	if err != nil {
		return nil, err
	}
	items, err := m.listChildTasks(taskDir, recursive)
	if err != nil {
		return nil, err
	}
	sortTasks(items)
	return items, nil
}

// MergeTask merges feature branch into parent branch and marks task merged.
func (m *TaskManager) MergeTask(lookup TaskLookup, vcsRepo apis.VCS) (*Task, error) {
	if vcsRepo == nil {
		return nil, fmt.Errorf("TaskManager.MergeTask() [task.go]: vcs repository is required")
	}
	taskDir, task, err := m.ResolveTask(lookup)
	if err != nil {
		return nil, err
	}

	if err := vcsRepo.MergeBranches(task.ParentBranch, task.FeatureBranch); err != nil {
		return nil, fmt.Errorf("TaskManager.MergeTask() [task.go]: failed to merge %q into %q: %w", task.FeatureBranch, task.ParentBranch, err)
	}

	task.Status = TaskStatusMerged
	task.UpdatedAt = m.nowFn().UTC().Format(time.RFC3339Nano)
	if err := m.writeTaskFile(taskDir, task); err != nil {
		return nil, err
	}

	return task, nil
}

// ArchiveTask moves selected task directory to archive root and returns archived task metadata.
func (m *TaskManager) ArchiveTask(lookup TaskLookup) (*Task, error) {
	taskDir, task, err := m.ResolveTask(lookup)
	if err != nil {
		return nil, err
	}

	archivedRoot := m.ArchivedTasksRoot()
	relativeDir, err := filepath.Rel(m.TasksRoot(), taskDir)
	if err != nil {
		return nil, fmt.Errorf("TaskManager.ArchiveTask() [task.go]: failed to calculate archive path: %w", err)
	}
	if strings.HasPrefix(relativeDir, "..") {
		return nil, fmt.Errorf("TaskManager.ArchiveTask() [task.go]: task path %q is outside of tasks root %q", taskDir, m.TasksRoot())
	}
	destinationDir := filepath.Join(archivedRoot, relativeDir)
	if err := os.MkdirAll(filepath.Dir(destinationDir), 0755); err != nil {
		return nil, fmt.Errorf("TaskManager.ArchiveTask() [task.go]: failed to create archive parent directory: %w", err)
	}
	if err := os.Rename(taskDir, destinationDir); err != nil {
		return nil, fmt.Errorf("TaskManager.ArchiveTask() [task.go]: failed to move task to archive: %w", err)
	}

	return cloneTask(task), nil
}

// ArchiveTasksByStatus moves all tasks with provided status to archive root.
func (m *TaskManager) ArchiveTasksByStatus(status string) ([]*Task, error) {
	trimmedStatus := strings.TrimSpace(status)
	if trimmedStatus == "" {
		return nil, fmt.Errorf("TaskManager.ArchiveTasksByStatus() [task.go]: status cannot be empty")
	}

	allTasks, err := m.loadAllTasks()
	if err != nil {
		return nil, err
	}

	candidates := make([]taskWithPath, 0, len(allTasks))
	for _, item := range allTasks {
		if item.task == nil {
			continue
		}
		if strings.TrimSpace(item.task.Status) == trimmedStatus {
			candidates = append(candidates, item)
		}
	}

	if len(candidates) == 0 {
		return []*Task{}, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return len(candidates[i].dir) < len(candidates[j].dir)
	})

	archivedRoot := m.ArchivedTasksRoot()
	archivedPaths := make([]string, 0, len(candidates))
	archivedTasks := make([]*Task, 0, len(candidates))
	for _, item := range candidates {
		skip := false
		for _, archivedPath := range archivedPaths {
			if isTaskPathNestedUnder(item.dir, archivedPath) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		relativeDir, relErr := filepath.Rel(m.TasksRoot(), item.dir)
		if relErr != nil {
			return nil, fmt.Errorf("TaskManager.ArchiveTasksByStatus() [task.go]: failed to calculate archive path: %w", relErr)
		}
		if strings.HasPrefix(relativeDir, "..") {
			return nil, fmt.Errorf("TaskManager.ArchiveTasksByStatus() [task.go]: task path %q is outside of tasks root %q", item.dir, m.TasksRoot())
		}
		destinationDir := filepath.Join(archivedRoot, relativeDir)
		if err := os.MkdirAll(filepath.Dir(destinationDir), 0755); err != nil {
			return nil, fmt.Errorf("TaskManager.ArchiveTasksByStatus() [task.go]: failed to create archive parent directory: %w", err)
		}
		if err := os.Rename(item.dir, destinationDir); err != nil {
			return nil, fmt.Errorf("TaskManager.ArchiveTasksByStatus() [task.go]: failed to move task to archive: %w", err)
		}

		archivedPaths = append(archivedPaths, item.dir)
		archivedTasks = append(archivedTasks, cloneTask(item.task))
	}

	return archivedTasks, nil
}

// ArchivedTasksRoot returns archive directory for task persistence.
func (m *TaskManager) ArchivedTasksRoot() string {
	return filepath.Join(m.TasksRoot(), "archive")
}

func (m *TaskManager) ensureBranches(task *Task, reset bool, vcsRepo apis.VCS) error {
	if vcsRepo == nil {
		return fmt.Errorf("TaskManager.ensureBranches() [task.go]: vcs repository is required")
	}
	if reset {
		_ = vcsRepo.DeleteBranch(task.FeatureBranch)
	}
	if err := ensureBranchFrom(vcsRepo, task.FeatureBranch, task.ParentBranch); err != nil {
		return fmt.Errorf("TaskManager.ensureBranches() [task.go]: %w", err)
	}
	return nil
}

func ensureBranchFrom(vcsRepo apis.VCS, branch string, from string) error {
	branches, err := vcsRepo.ListBranches("")
	if err != nil {
		return fmt.Errorf("ensureBranchFrom() [task.go]: failed to list branches: %w", err)
	}
	for _, existing := range branches {
		if strings.TrimSpace(existing) == strings.TrimSpace(branch) {
			return nil
		}
	}
	if err := vcsRepo.NewBranch(branch, from); err != nil {
		return fmt.Errorf("ensureBranchFrom() [task.go]: failed to create branch %q from %q: %w", branch, from, err)
	}
	return nil
}

func isTaskStatusSupported(status string) bool {
	trimmed := strings.TrimSpace(status)
	switch trimmed {
	case TaskStatusDraft, TaskStatusCreated, TaskStatusOpen, TaskStatusRunning, TaskStatusSubtasks, TaskStatusMerged, TaskStatusCompleted, TaskStatusFailed:
		return true
	default:
		return false
	}
}

func (m *TaskManager) appendSubtask(parentTaskID string, subtaskID string) error {
	parentDir, parentTask, err := m.findTaskByUUID(parentTaskID)
	if err != nil {
		return fmt.Errorf("TaskManager.appendSubtask() [task.go]: failed to resolve parent task: %w", err)
	}
	parentTask.SubtaskIDs = appendUniqueString(parentTask.SubtaskIDs, subtaskID)
	parentTask.UpdatedAt = m.nowFn().UTC().Format(time.RFC3339Nano)
	return m.writeTaskFile(parentDir, parentTask)
}

func (m *TaskManager) writeSessionSummary(taskDir string, meta *TaskSessionSummary, summaryText string) error {
	if meta == nil {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task.go]: summary metadata is nil")
	}
	if strings.TrimSpace(meta.SessionID) == "" {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task.go]: session id is empty")
	}

	sessionDir := filepath.Join(taskDir, "ses-"+meta.SessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task.go]: failed to create session directory: %w", err)
	}

	metaBytes, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task.go]: failed to marshal summary metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "summary.yml"), metaBytes, 0644); err != nil {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task.go]: failed to write summary metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "summary.md"), []byte(strings.TrimSpace(summaryText)+"\n"), 0644); err != nil {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task.go]: failed to write summary text: %w", err)
	}

	return nil
}

func (m *TaskManager) writeTaskOutput(taskDir string, task *Task, sessionID string, summaryText string) error {
	meta := TaskOutputMetadata{
		TaskID:        task.UUID,
		TaskName:      task.Name,
		Status:        task.Status,
		UpdatedAt:     m.nowFn().UTC().Format(time.RFC3339Nano),
		LastSessionID: strings.TrimSpace(sessionID),
	}
	metaBytes, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("TaskManager.writeTaskOutput() [task.go]: failed to marshal task output metadata: %w", err)
	}

	content := strings.Builder{}
	content.WriteString("---\n")
	content.Write(metaBytes)
	content.WriteString("---\n\n")
	content.WriteString(strings.TrimSpace(summaryText))
	content.WriteString("\n")

	if err := os.WriteFile(filepath.Join(taskDir, "output.md"), []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("TaskManager.writeTaskOutput() [task.go]: failed to write task output: %w", err)
	}

	return nil
}

func (m *TaskManager) readLastSessionSummary(taskDir string, task *Task) (*TaskSessionSummary, string, error) {
	if task == nil || len(task.SessionIDs) == 0 {
		return nil, "", nil
	}
	lastSessionID := task.SessionIDs[len(task.SessionIDs)-1]
	sessionDir := filepath.Join(taskDir, "ses-"+lastSessionID)
	metaBytes, metaErr := os.ReadFile(filepath.Join(sessionDir, "summary.yml"))
	textBytes, textErr := os.ReadFile(filepath.Join(sessionDir, "summary.md"))

	if metaErr != nil && textErr != nil {
		return nil, "", nil
	}

	meta := &TaskSessionSummary{}
	if metaErr == nil {
		if err := yaml.Unmarshal(metaBytes, meta); err != nil {
			return nil, "", fmt.Errorf("TaskManager.readLastSessionSummary() [task.go]: failed to unmarshal summary metadata: %w", err)
		}
	}
	text := ""
	if textErr == nil {
		text = strings.TrimSpace(string(textBytes))
	}

	return meta, text, nil
}

type taskWithPath struct {
	dir  string
	task *Task
}

func (m *TaskManager) loadAllTasks() ([]taskWithPath, error) {
	root := m.TasksRoot()
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []taskWithPath{}, nil
		}
		return nil, fmt.Errorf("TaskManager.loadAllTasks() [task.go]: failed to stat tasks root: %w", err)
	}

	items := []taskWithPath{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d != nil && d.IsDir() {
			if path != root && !taskDirUUIDPattern.MatchString(strings.TrimSpace(d.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if d == nil || filepath.Base(path) != "task.yml" {
			return nil
		}
		task, err := readTaskFile(path)
		if err != nil {
			return err
		}
		items = append(items, taskWithPath{dir: filepath.Dir(path), task: task})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("TaskManager.loadAllTasks() [task.go]: failed to walk tasks root: %w", err)
	}

	return items, nil
}

func (m *TaskManager) findTaskByUUID(taskID string) (string, *Task, error) {
	trimmedID := strings.TrimSpace(taskID)
	if trimmedID == "" {
		return "", nil, fmt.Errorf("TaskManager.findTaskByUUID() [task.go]: task id cannot be empty")
	}
	items, err := m.loadAllTasks()
	if err != nil {
		return "", nil, err
	}
	for _, item := range items {
		if item.task != nil && strings.TrimSpace(item.task.UUID) == trimmedID {
			return item.dir, item.task, nil
		}
	}
	return "", nil, fmt.Errorf("TaskManager.findTaskByUUID() [task.go]: task %q not found", taskID)
}

func (m *TaskManager) listChildTasks(taskDir string, recursive bool) ([]*Task, error) {
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return nil, fmt.Errorf("TaskManager.listChildTasks() [task.go]: failed to read task directory: %w", err)
	}
	items := []*Task{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !taskDirUUIDPattern.MatchString(strings.TrimSpace(entry.Name())) {
			continue
		}
		childDir := filepath.Join(taskDir, entry.Name())
		taskPath := filepath.Join(childDir, "task.yml")
		if _, statErr := os.Stat(taskPath); statErr != nil {
			continue
		}
		childTask, readErr := readTaskFile(taskPath)
		if readErr != nil {
			return nil, readErr
		}
		items = append(items, childTask)
		if recursive {
			nested, nestedErr := m.listChildTasks(childDir, true)
			if nestedErr != nil {
				return nil, nestedErr
			}
			items = append(items, nested...)
		}
	}
	return items, nil
}

func (m *TaskManager) writeTaskFile(taskDir string, task *Task) error {
	if task == nil {
		return fmt.Errorf("TaskManager.writeTaskFile() [task.go]: task is nil")
	}
	bytesData, err := yaml.Marshal(task)
	if err != nil {
		return fmt.Errorf("TaskManager.writeTaskFile() [task.go]: failed to marshal task metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "task.yml"), bytesData, 0644); err != nil {
		return fmt.Errorf("TaskManager.writeTaskFile() [task.go]: failed to write task metadata: %w", err)
	}
	return nil
}

func readTaskFile(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readTaskFile() [task.go]: failed to read task metadata: %w", err)
	}
	var task Task
	if err := yaml.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("readTaskFile() [task.go]: failed to unmarshal task metadata: %w", err)
	}
	task.TaskDir = filepath.Dir(path)
	return &task, nil
}

func normalizeTaskDeps(deps []string) []string {
	result := make([]string, 0, len(deps))
	seen := map[string]struct{}{}
	for _, dep := range deps {
		trimmed := strings.TrimSpace(dep)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}

func cloneTask(task *Task) *Task {
	if task == nil {
		return nil
	}
	cloned := *task
	cloned.Deps = append([]string(nil), task.Deps...)
	cloned.SessionIDs = append([]string(nil), task.SessionIDs...)
	cloned.SubtaskIDs = append([]string(nil), task.SubtaskIDs...)
	return &cloned
}

func sortTasks(tasks []*Task) {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i] == nil || tasks[j] == nil {
			return i < j
		}
		left := strings.TrimSpace(tasks[i].Name)
		right := strings.TrimSpace(tasks[j].Name)
		if left == right {
			return strings.TrimSpace(tasks[i].UUID) < strings.TrimSpace(tasks[j].UUID)
		}
		return left < right
	})
}

func firstNonEmptyTask(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func (m *TaskManager) resolveTaskPromptOverride(task *Task, params TaskRunParams, taskPrompt string) (string, error) {
	if task == nil {
		return "", fmt.Errorf("TaskManager.resolveTaskPromptOverride() [task.go]: task cannot be nil")
	}
	basePrompt := strings.TrimSpace(params.PromptOverride)
	if basePrompt == "" {
		return strings.TrimSpace(taskPrompt), nil
	}
	taskPromptValue := strings.TrimSpace(taskPrompt)

	contextData := map[string]any{
		"Task": map[string]any{
			"UUID":          strings.TrimSpace(task.UUID),
			"Name":          strings.TrimSpace(task.Name),
			"Description":   strings.TrimSpace(task.Description),
			"FeatureBranch": strings.TrimSpace(task.FeatureBranch),
			"ParentBranch":  strings.TrimSpace(task.ParentBranch),
			"Role":          strings.TrimSpace(task.Role),
			"Prompt":        taskPromptValue,
		},
	}

	renderedPrompt, err := renderTaskPromptTemplate(basePrompt, contextData)
	if err != nil {
		return "", fmt.Errorf("TaskManager.resolveTaskPromptOverride() [task.go]: failed to render prompt override template: %w", err)
	}
	renderedPrompt = strings.TrimSpace(renderedPrompt)

	invocation, isCommandInvocation, parseErr := commands.ParseInvocation(renderedPrompt, append([]string(nil), params.PromptArgs...))
	if parseErr != nil {
		return "", fmt.Errorf("TaskManager.resolveTaskPromptOverride() [task.go]: %w", parseErr)
	}
	if !isCommandInvocation {
		if len(params.PromptArgs) > 0 {
			return "", fmt.Errorf("TaskManager.resolveTaskPromptOverride() [task.go]: prompt override must be a single argument unless using /command invocation")
		}
		if strings.TrimSpace(renderedPrompt) == "" {
			return "", fmt.Errorf("TaskManager.resolveTaskPromptOverride() [task.go]: resolved prompt override is empty")
		}
		return renderedPrompt, nil
	}

	if m.configStore == nil {
		return "", fmt.Errorf("TaskManager.resolveTaskPromptOverride() [task.go]: config store is not available for command invocation")
	}
	commandsRoot := filepath.Join(strings.TrimSpace(m.baseDir), ".agents", "commands")
	loadedCommand, loadErr := commands.LoadFromDir(commandsRoot, invocation.Name)
	if loadErr != nil {
		return "", fmt.Errorf("TaskManager.resolveTaskPromptOverride() [task.go]: %w", loadErr)
	}

	commandTemplate, templateErr := renderTaskPromptTemplate(strings.TrimSpace(loadedCommand.Template), contextData)
	if templateErr != nil {
		return "", fmt.Errorf("TaskManager.resolveTaskPromptOverride() [task.go]: failed to render command template: %w", templateErr)
	}

	resolvedCommandPrompt := commands.ApplyArguments(commandTemplate, invocation.Arguments)
	resolvedCommandPrompt, expandErr := commands.ExpandPrompt(resolvedCommandPrompt, strings.TrimSpace(m.baseDir), nil, nil)
	if expandErr != nil {
		return "", fmt.Errorf("TaskManager.resolveTaskPromptOverride() [task.go]: failed to render command /%s: %w", strings.TrimSpace(loadedCommand.Name), expandErr)
	}
	resolvedCommandPrompt = strings.TrimSpace(resolvedCommandPrompt)
	if resolvedCommandPrompt == "" {
		return "", fmt.Errorf("TaskManager.resolveTaskPromptOverride() [task.go]: rendered command /%s prompt is empty", strings.TrimSpace(loadedCommand.Name))
	}

	return resolvedCommandPrompt, nil
}

func renderTaskPromptTemplate(input string, contextData map[string]any) (string, error) {
	templateText := strings.TrimSpace(input)
	if templateText == "" {
		return "", nil
	}
	tpl, err := template.New("task-prompt").Option("missingkey=error").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("renderTaskPromptTemplate() [task.go]: failed to parse prompt template: %w", err)
	}

	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, contextData); err != nil {
		return "", fmt.Errorf("renderTaskPromptTemplate() [task.go]: failed to render prompt template: %w", err)
	}

	return rendered.String(), nil
}

func isTaskPathNestedUnder(path string, parent string) bool {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	cleanParent := filepath.Clean(strings.TrimSpace(parent))
	if cleanPath == "." || cleanParent == "." {
		return false
	}
	if cleanPath == cleanParent {
		return true
	}
	relativePath, err := filepath.Rel(cleanParent, cleanPath)
	if err != nil {
		return false
	}
	if relativePath == "." {
		return true
	}
	if strings.HasPrefix(relativePath, "..") {
		return false
	}

	return true
}
