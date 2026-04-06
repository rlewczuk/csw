package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"gopkg.in/yaml.v3"
)

const (
	// TaskStatusCreated indicates task is created and not yet started.
	TaskStatusCreated = "created"
	// TaskStatusOpen indicates task can be worked on.
	TaskStatusOpen = "open"
	// TaskStatusRunning indicates task is currently being executed.
	TaskStatusRunning = "running"
	// TaskStatusMerged indicates task result was merged to parent branch.
	TaskStatusMerged = "merged"

	// TaskStateCreated indicates task has not run yet.
	TaskStateCreated = "created"
	// TaskStateRunning indicates task run is in progress.
	TaskStateRunning = "running"
	// TaskStateCompleted indicates task completed successfully.
	TaskStateCompleted = "completed"
	// TaskStateFailed indicates task execution failed.
	TaskStateFailed = "failed"
)

// Task stores persistent task metadata.
type Task struct {
	UUID          string   `json:"uuid" yaml:"uuid"`
	Name          string   `json:"name,omitempty" yaml:"name,omitempty"`
	Description   string   `json:"description,omitempty" yaml:"description,omitempty"`
	Status        string   `json:"status" yaml:"status"`
	FeatureBranch string   `json:"feature_branch" yaml:"feature_branch"`
	ParentBranch  string   `json:"parent_branch" yaml:"parent_branch"`
	Role          string   `json:"role,omitempty" yaml:"role,omitempty"`
	State         string   `json:"state" yaml:"state"`
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
	State         string `json:"state" yaml:"state"`
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
	FeatureBranch *string
	ParentBranch  *string
	Role          *string
	Deps          *[]string
	Prompt        *string
}

// TaskRunParams defines parameters for running a task.
type TaskRunParams struct {
	Identifier string
	Merge      bool
	Reset      bool
	Continue   bool
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

// TaskSessionRunner runs a single task session.
type TaskSessionRunner interface {
	RunTaskSession(ctx context.Context, request TaskSessionRunRequest) (TaskSessionRunResult, error)
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
	VCS           apis.VCS
}

// TaskSessionRunResult defines task session execution output.
type TaskSessionRunResult struct {
	SessionID   string
	SummaryText string
	StartedAt   time.Time
	CompletedAt time.Time
}

// CLITaskSessionRunner executes task sessions by spawning CLI process.
type CLITaskSessionRunner struct {
	BaseDir       string
	ModelName     string
	ConfigPath    string
	ProjectConfig string
	Thinking      string
}

// TaskManager manages persistent hierarchical tasks.
type TaskManager struct {
	baseDir     string
	configStore conf.ConfigStore
	runner      TaskSessionRunner
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
func NewTaskManager(baseDir string, configStore conf.ConfigStore, runner TaskSessionRunner) (*TaskManager, error) {
	trimmedBaseDir := strings.TrimSpace(baseDir)
	if trimmedBaseDir == "" {
		return nil, fmt.Errorf("NewTaskManager() [task.go]: baseDir cannot be empty")
	}

	return &TaskManager{
		baseDir:     trimmedBaseDir,
		configStore: configStore,
		runner:      runner,
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

// NewCLITaskSessionRunner creates a CLI-based task session runner.
func NewCLITaskSessionRunner(baseDir string, modelName string, configPath string, projectConfig string, thinking string) (*CLITaskSessionRunner, error) {
	trimmedBaseDir := strings.TrimSpace(baseDir)
	if trimmedBaseDir == "" {
		return nil, fmt.Errorf("NewCLITaskSessionRunner() [task.go]: baseDir cannot be empty")
	}

	return &CLITaskSessionRunner{
		BaseDir:       trimmedBaseDir,
		ModelName:     strings.TrimSpace(modelName),
		ConfigPath:    strings.TrimSpace(configPath),
		ProjectConfig: strings.TrimSpace(projectConfig),
		Thinking:      strings.TrimSpace(thinking),
	}, nil
}

// RunTaskSession executes one task session in dedicated worktree branch.
func (r *CLITaskSessionRunner) RunTaskSession(ctx context.Context, request TaskSessionRunRequest) (TaskSessionRunResult, error) {
	if r == nil {
		return TaskSessionRunResult{}, fmt.Errorf("CLITaskSessionRunner.RunTaskSession() [task.go]: runner is nil")
	}

	startTime := time.Now().UTC()
	args, err := r.buildCLIArgs(request)
	if err != nil {
		return TaskSessionRunResult{}, err
	}

	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = r.BaseDir
	output, err := command.CombinedOutput()

	completedAt := time.Now().UTC()
	sessionID := extractTaskSessionID(string(output))
	summaryText := strings.TrimSpace(string(output))
	if strings.TrimSpace(sessionID) != "" {
		if summary, readErr := readCLISessionSummary(r.BaseDir, sessionID); readErr == nil && strings.TrimSpace(summary) != "" {
			summaryText = summary
		}
	}

	result := TaskSessionRunResult{SessionID: sessionID, SummaryText: summaryText, StartedAt: startTime, CompletedAt: completedAt}
	if err != nil {
		return result, fmt.Errorf("CLITaskSessionRunner.RunTaskSession() [task.go]: cli task run failed: %w", err)
	}

	return result, nil
}

func (r *CLITaskSessionRunner) buildCLIArgs(request TaskSessionRunRequest) ([]string, error) {
	args := []string{"run", "./cmd/csw", "cli", "--workdir", r.BaseDir, "--worktree", strings.TrimSpace(request.TaskBranch), "--role", firstNonEmptyTask(request.Role, "developer")}
	if strings.TrimSpace(r.ModelName) != "" {
		args = append(args, "--model", r.ModelName)
	}
	if strings.TrimSpace(r.ConfigPath) != "" {
		args = append(args, "--config-path", r.ConfigPath)
	}
	if strings.TrimSpace(r.ProjectConfig) != "" {
		args = append(args, "--project-config", r.ProjectConfig)
	}
	if strings.TrimSpace(r.Thinking) != "" {
		args = append(args, "--thinking", r.Thinking)
	}
	if request.Task != nil {
		taskJSON, err := json.Marshal(request.Task)
		if err != nil {
			return nil, fmt.Errorf("CLITaskSessionRunner.buildCLIArgs() [task.go]: failed to marshal task metadata: %w", err)
		}
		args = append(args, "--task-json", string(taskJSON))
	}
	if strings.TrimSpace(request.TaskDir) != "" {
		args = append(args, "--task-dir", strings.TrimSpace(request.TaskDir))
	}

	args = append(args, "--output-format", "full", strings.TrimSpace(request.Prompt))

	return args, nil
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

// RunTask runs task through backend interface.
func (a *TaskBackendAdapter) RunTask(ctx context.Context, identifier string, fallbackTaskID string, merge bool, reset bool) (tool.TaskRunOutcome, error) {
	if a == nil || a.manager == nil {
		return tool.TaskRunOutcome{}, fmt.Errorf("TaskBackendAdapter.RunTask() [task.go]: adapter is not configured")
	}
	if a.vcsRepo == nil {
		return tool.TaskRunOutcome{}, fmt.Errorf("TaskBackendAdapter.RunTask() [task.go]: vcs repository is not configured")
	}

	outcome, err := a.manager.RunTask(ctx, TaskLookup{Identifier: strings.TrimSpace(identifier), FallbackTaskID: strings.TrimSpace(fallbackTaskID)}, TaskRunParams{Merge: merge, Reset: reset}, a.vcsRepo)
	if outcome == nil {
		if err != nil {
			return tool.TaskRunOutcome{}, err
		}
		return tool.TaskRunOutcome{}, fmt.Errorf("TaskBackendAdapter.RunTask() [task.go]: empty run outcome")
	}

	result := tool.TaskRunOutcome{
		Task:           toToolTaskRecord(outcome.Task),
		SessionID:      outcome.SessionID,
		SummaryText:    outcome.SummaryText,
		Merged:         outcome.Merged,
		TaskBranchName: outcome.TaskBranchName,
	}
	if outcome.SummaryMeta != nil {
		result.SummaryMeta = &tool.TaskSessionSummary{
			SessionID:   outcome.SummaryMeta.SessionID,
			Status:      outcome.SummaryMeta.Status,
			StartedAt:   outcome.SummaryMeta.StartedAt,
			CompletedAt: outcome.SummaryMeta.CompletedAt,
			TaskID:      outcome.SummaryMeta.TaskID,
		}
	}

	if err != nil {
		return result, err
	}

	return result, nil
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
		State:         taskData.State,
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
	return filepath.Join(m.baseDir, ".csw", "tasks")
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

	task := &Task{
		UUID:          taskID,
		Name:          name,
		Description:   strings.TrimSpace(params.Description),
		Status:        TaskStatusCreated,
		FeatureBranch: featureBranch,
		ParentBranch:  parentBranch,
		Role:          strings.TrimSpace(params.Role),
		State:         TaskStateCreated,
		Deps:          normalizeTaskDeps(params.Deps),
		SessionIDs:    []string{},
		SubtaskIDs:    []string{},
		ParentTaskID:  strings.TrimSpace(params.ParentTaskID),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	taskDir := filepath.Join(m.TasksRoot(), task.UUID)
	if strings.TrimSpace(task.ParentTaskID) != "" {
		parentDir, _, err := m.findTaskByUUID(task.ParentTaskID)
		if err != nil {
			return nil, fmt.Errorf("TaskManager.CreateTask() [task.go]: failed to resolve parent task: %w", err)
		}
		taskDir = filepath.Join(parentDir, task.UUID)
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

	if params.Name != nil {
		trimmed := strings.TrimSpace(*params.Name)
		if trimmed != "" {
			task.Name = trimmed
		}
	}
	if params.Description != nil {
		task.Description = strings.TrimSpace(*params.Description)
	}
	if params.FeatureBranch != nil {
		trimmed := strings.TrimSpace(*params.FeatureBranch)
		if trimmed != "" {
			task.FeatureBranch = trimmed
		}
	}
	if params.ParentBranch != nil {
		trimmed := strings.TrimSpace(*params.ParentBranch)
		if trimmed != "" {
			task.ParentBranch = trimmed
		}
	}
	if params.Role != nil {
		task.Role = strings.TrimSpace(*params.Role)
	}
	if params.Deps != nil {
		task.Deps = normalizeTaskDeps(*params.Deps)
	}
	if params.Prompt != nil {
		if strings.TrimSpace(*params.Prompt) == "" {
			return nil, fmt.Errorf("TaskManager.UpdateTask() [task.go]: prompt cannot be empty")
		}
		if err := os.WriteFile(filepath.Join(taskDir, "task.md"), []byte(strings.TrimSpace(*params.Prompt)+"\n"), 0644); err != nil {
			return nil, fmt.Errorf("TaskManager.UpdateTask() [task.go]: failed to update task prompt: %w", err)
		}
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

// RunTask runs one task session and persists summary/output.
func (m *TaskManager) RunTask(ctx context.Context, lookup TaskLookup, params TaskRunParams, vcsRepo apis.VCS) (*TaskRunResult, error) {
	if m.runner == nil {
		return nil, fmt.Errorf("TaskManager.RunTask() [task.go]: task session runner is not configured")
	}
	if vcsRepo == nil {
		return nil, fmt.Errorf("TaskManager.RunTask() [task.go]: vcs repository is required")
	}
	taskDir, task, err := m.ResolveTask(lookup)
	if err != nil {
		return nil, err
	}

	if err := m.validateDependencies(task); err != nil {
		return nil, err
	}

	if err := m.ensureBranches(task, params.Reset, vcsRepo); err != nil {
		return nil, err
	}

	promptBytes, err := os.ReadFile(filepath.Join(taskDir, "task.md"))
	if err != nil {
		return nil, fmt.Errorf("TaskManager.RunTask() [task.go]: failed to read task prompt: %w", err)
	}
	prompt := strings.TrimSpace(string(promptBytes))
	if prompt == "" {
		return nil, fmt.Errorf("TaskManager.RunTask() [task.go]: task is empty: task.md has no prompt")
	}
	if params.Continue {
		prompt, err = m.renderContinuePrompt(task, prompt)
		if err != nil {
			return nil, err
		}
	}

	sessionID := m.uuidFn()
	taskBranchName := fmt.Sprintf("%s-task-%s", task.FeatureBranch, strings.ReplaceAll(sessionID[:8], "-", ""))
	if params.Reset {
		_ = vcsRepo.DropWorktree(taskBranchName)
		_ = vcsRepo.DeleteBranch(taskBranchName)
	}
	if err := ensureBranchFrom(vcsRepo, taskBranchName, task.FeatureBranch); err != nil {
		return nil, fmt.Errorf("TaskManager.RunTask() [task.go]: failed to prepare task branch: %w", err)
	}

	task.Status = TaskStatusRunning
	task.State = TaskStateRunning
	task.UpdatedAt = m.nowFn().UTC().Format(time.RFC3339Nano)
	if err := m.writeTaskFile(taskDir, task); err != nil {
		return nil, err
	}

	runResult, runErr := m.runner.RunTaskSession(ctx, TaskSessionRunRequest{
		TaskID:        task.UUID,
		TaskName:      task.Name,
		Task:          cloneTask(task),
		TaskDir:       taskDir,
		TaskBranch:    taskBranchName,
		FeatureBranch: task.FeatureBranch,
		ParentBranch:  task.ParentBranch,
		Role:          firstNonEmptyTask(strings.TrimSpace(task.Role), "developer"),
		Prompt:        prompt,
		VCS:           vcsRepo,
	})

	meta := &TaskSessionSummary{
		SessionID:   firstNonEmptyTask(strings.TrimSpace(runResult.SessionID), sessionID),
		Status:      TaskStateCompleted,
		TaskID:      task.UUID,
		StartedAt:   runResult.StartedAt.UTC().Format(time.RFC3339Nano),
		CompletedAt: runResult.CompletedAt.UTC().Format(time.RFC3339Nano),
	}
	if runResult.StartedAt.IsZero() {
		meta.StartedAt = m.nowFn().UTC().Format(time.RFC3339Nano)
	}
	if runResult.CompletedAt.IsZero() {
		meta.CompletedAt = m.nowFn().UTC().Format(time.RFC3339Nano)
	}

	task.SessionIDs = appendUniqueString(task.SessionIDs, meta.SessionID)

	if runErr != nil {
		meta.Status = TaskStateFailed
		task.Status = TaskStatusOpen
		task.State = TaskStateFailed
	} else {
		if err := vcsRepo.MergeBranches(task.FeatureBranch, taskBranchName); err != nil {
			meta.Status = TaskStateFailed
			task.Status = TaskStatusOpen
			task.State = TaskStateFailed
			runErr = fmt.Errorf("TaskManager.RunTask() [task.go]: failed to merge task branch %q into feature branch %q: %w", taskBranchName, task.FeatureBranch, err)
		} else {
			_ = vcsRepo.DeleteBranch(taskBranchName)
		}
		task.Status = TaskStatusOpen
		if runErr == nil {
			task.State = TaskStateCompleted
		}
	}
	task.UpdatedAt = m.nowFn().UTC().Format(time.RFC3339Nano)
	if err := m.writeTaskFile(taskDir, task); err != nil {
		return nil, err
	}

	if writeErr := m.writeSessionSummary(taskDir, meta, runResult.SummaryText); writeErr != nil {
		return nil, writeErr
	}
	if writeErr := m.writeTaskOutput(taskDir, task, meta.SessionID, runResult.SummaryText); writeErr != nil {
		return nil, writeErr
	}

	merged := false
	if runErr == nil && params.Merge {
		if _, mergeErr := m.MergeTask(TaskLookup{Identifier: task.UUID}, vcsRepo); mergeErr != nil {
			return nil, mergeErr
		}
		merged = true
		taskDir, task, _ = m.ResolveTask(TaskLookup{Identifier: task.UUID})
		_ = taskDir
	}

	if runErr != nil {
		return &TaskRunResult{Task: cloneTask(task), SessionID: meta.SessionID, SummaryMeta: meta, SummaryText: runResult.SummaryText, Merged: merged, TaskBranchName: taskBranchName}, fmt.Errorf("TaskManager.RunTask() [task.go]: task run failed: %w", runErr)
	}

	return &TaskRunResult{Task: cloneTask(task), SessionID: meta.SessionID, SummaryMeta: meta, SummaryText: runResult.SummaryText, Merged: merged, TaskBranchName: taskBranchName}, nil
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
	task.State = TaskStateCompleted
	task.UpdatedAt = m.nowFn().UTC().Format(time.RFC3339Nano)
	if err := m.writeTaskFile(taskDir, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (m *TaskManager) validateDependencies(task *Task) error {
	for _, dep := range task.Deps {
		_, depTask, err := m.findTaskByUUID(dep)
		if err != nil {
			return fmt.Errorf("TaskManager.validateDependencies() [task.go]: dependency %q not found: %w", dep, err)
		}
		if depTask.State != TaskStateCompleted {
			return fmt.Errorf("TaskManager.validateDependencies() [task.go]: dependency %q is not completed (state=%s)", dep, depTask.State)
		}
	}
	return nil
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

func (m *TaskManager) appendSubtask(parentTaskID string, subtaskID string) error {
	parentDir, parentTask, err := m.findTaskByUUID(parentTaskID)
	if err != nil {
		return fmt.Errorf("TaskManager.appendSubtask() [task.go]: failed to resolve parent task: %w", err)
	}
	parentTask.SubtaskIDs = appendUniqueString(parentTask.SubtaskIDs, subtaskID)
	parentTask.UpdatedAt = m.nowFn().UTC().Format(time.RFC3339Nano)
	return m.writeTaskFile(parentDir, parentTask)
}

func (m *TaskManager) renderContinuePrompt(task *Task, prompt string) (string, error) {
	if m.configStore == nil {
		return "", fmt.Errorf("TaskManager.renderContinuePrompt() [task.go]: config store is not available")
	}
	tplBytes, err := m.configStore.GetAgentConfigFile("continue", "prompt.md")
	if err != nil {
		return "", fmt.Errorf("TaskManager.renderContinuePrompt() [task.go]: failed to load continue prompt template: %w", err)
	}
	tpl, err := template.New("task-continue-prompt").Parse(string(tplBytes))
	if err != nil {
		return "", fmt.Errorf("TaskManager.renderContinuePrompt() [task.go]: failed to parse continue prompt template: %w", err)
	}

	data := map[string]any{
		"TaskID":          task.UUID,
		"TaskName":        task.Name,
		"TaskDescription": task.Description,
		"FeatureBranch":   task.FeatureBranch,
		"ParentBranch":    task.ParentBranch,
		"TaskRole":        task.Role,
		"Prompt":          prompt,
	}

	var buffer bytes.Buffer
	if err := tpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("TaskManager.renderContinuePrompt() [task.go]: failed to render continue prompt template: %w", err)
	}
	return buffer.String(), nil
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
		State:         task.State,
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
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "task.yml" {
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

func extractTaskSessionID(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Session ID:") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "Session ID:"))
	}

	return ""
}

func readCLISessionSummary(baseDir string, sessionID string) (string, error) {
	if strings.TrimSpace(baseDir) == "" {
		return "", fmt.Errorf("readCLISessionSummary() [task.go]: baseDir is empty")
	}
	if strings.TrimSpace(sessionID) == "" {
		return "", fmt.Errorf("readCLISessionSummary() [task.go]: sessionID is empty")
	}

	path := filepath.Join(baseDir, ".cswdata", "logs", "sessions", strings.TrimSpace(sessionID), "summary.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("readCLISessionSummary() [task.go]: failed to read summary file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}
