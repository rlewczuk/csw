package system

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func resolveTaskIdentifierFromPosition(manager *core.TaskManager, candidate string) (string, bool, error) {
	trimmedCandidate := strings.TrimSpace(candidate)
	if trimmedCandidate == "" {
		return "", false, nil
	}
	if shared.UUIDPattern.MatchString(trimmedCandidate) {
		return trimmedCandidate, true, nil
	}
	if manager == nil {
		return "", false, nil
	}

	tasks, err := listAllCurrentTasksForRun(manager)
	if err != nil {
		return "", false, err
	}

	matchedUUID := ""
	for _, taskData := range tasks {
		if taskData == nil || strings.TrimSpace(taskData.FeatureBranch) != trimmedCandidate {
			continue
		}
		if matchedUUID != "" && matchedUUID != strings.TrimSpace(taskData.UUID) {
			return "", false, fmt.Errorf("resolveTaskIdentifierFromPosition() [run_task.go]: multiple tasks match feature branch %q", trimmedCandidate)
		}
		matchedUUID = strings.TrimSpace(taskData.UUID)
	}

	if matchedUUID == "" {
		return "", false, nil
	}

	return matchedUUID, true, nil
}

func loadRunTaskManager(cmd *cobra.Command, workDir string, shadowDir string, projectConfig string, configPath string) (*core.TaskManager, error) {
	if cmd == nil {
		return nil, fmt.Errorf("loadRunTaskManager() [run_task.go]: command cannot be nil")
	}

	resolvedWorkDir, err := ResolveWorkDir(workDir)
	if err != nil {
		return nil, err
	}
	resolvedTaskDir, err := resolveRunTaskDirPath(cmd, resolvedWorkDir, shadowDir, projectConfig, configPath)
	if err != nil {
		return nil, err
	}

	resolvedConfigPath, err := BuildConfigPath(projectConfig, configPath)
	if err != nil {
		return nil, err
	}
	configStore, err := conf.CswConfigLoad(resolvedConfigPath)
	if err != nil {
		return nil, fmt.Errorf("loadRunTaskManager() [run_task.go]: failed to load config: %w", err)
	}

	taskManager, err := core.NewTaskManagerWithTasksDir(resolvedWorkDir, resolvedTaskDir, configStore)
	if err != nil {
		return nil, err
	}

	return taskManager, nil
}

func resolveRunTaskDirPath(cmd *cobra.Command, workDir string, shadowDir string, projectConfig string, configPath string) (string, error) {
	if cmd == nil {
		return "", fmt.Errorf("resolveRunTaskDirPath() [run_task.go]: command cannot be nil")
	}

	flagTaskDir := ""
	flag := cmd.Flag("task-dir")
	if flag != nil {
		flagTaskDir = strings.TrimSpace(flag.Value.String())
	}

	resolvedTaskDir := strings.TrimSpace(flagTaskDir)
	if resolvedTaskDir == "" {
		defaults, defaultsErr := ResolveRunDefaults(ResolveRunDefaultsParams{
			WorkDir:       workDir,
			ShadowDir:     shadowDir,
			ProjectConfig: projectConfig,
			ConfigPath:    configPath,
		})
		if defaultsErr != nil {
			return "", fmt.Errorf("resolveRunTaskDirPath() [run_task.go]: failed to resolve CLI defaults: %w", defaultsErr)
		}
		resolvedTaskDir = strings.TrimSpace(defaults.TaskDir)
	}

	if resolvedTaskDir == "" {
		resolvedTaskDir = ".cswdata/tasks"
	}
	if !filepath.IsAbs(resolvedTaskDir) {
		resolvedTaskDir = filepath.Join(workDir, resolvedTaskDir)
	}

	return filepath.Clean(resolvedTaskDir), nil
}

func resolveTaskRunIdentifierForRun(manager *core.TaskManager, identifier string, useLast bool, useNext bool) (string, error) {
	if useLast && useNext {
		return "", fmt.Errorf("resolveTaskRunIdentifierForRun() [run_task.go]: --last and --next cannot be used together")
	}
	if (useLast || useNext) && strings.TrimSpace(identifier) != "" {
		return "", fmt.Errorf("resolveTaskRunIdentifierForRun() [run_task.go]: task identifier cannot be used with --last or --next")
	}

	if useLast || useNext {
		if manager == nil {
			return "", fmt.Errorf("resolveTaskRunIdentifierForRun() [run_task.go]: manager cannot be nil")
		}

		taskData, err := findRunnableTaskByModTimeForRun(manager, useLast)
		if err != nil {
			return "", err
		}

		return strings.TrimSpace(taskData.UUID), nil
	}

	trimmedIdentifier := strings.TrimSpace(identifier)
	if trimmedIdentifier == "" {
		return "", fmt.Errorf("resolveTaskRunIdentifierForRun() [run_task.go]: task identifier cannot be empty")
	}

	return trimmedIdentifier, nil
}

func findRunnableTaskByModTimeForRun(manager *core.TaskManager, newest bool) (*core.Task, error) {
	tasks, err := listAllCurrentTasksForRun(manager)
	if err != nil {
		return nil, err
	}

	modTimes, err := collectTaskYMLModTimesForRun(manager.TasksRoot())
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		modTimes = map[string]int64{}
	}

	var selected *core.Task
	selectedModTime := int64(0)
	for _, taskData := range tasks {
		if !isUnfinishedTaskForRun(taskData) {
			continue
		}

		taskID := strings.TrimSpace(taskData.UUID)
		currentModTime := modTimes[taskID]
		if selected == nil {
			selected = taskData
			selectedModTime = currentModTime
			continue
		}

		isBetter := false
		if newest {
			isBetter = currentModTime > selectedModTime || (currentModTime == selectedModTime && taskID > strings.TrimSpace(selected.UUID))
		} else {
			isBetter = currentModTime < selectedModTime || (currentModTime == selectedModTime && taskID < strings.TrimSpace(selected.UUID))
		}

		if isBetter {
			selected = taskData
			selectedModTime = currentModTime
		}
	}

	if selected == nil {
		return nil, fmt.Errorf("findRunnableTaskByModTimeForRun() [run_task.go]: no unfinished task found")
	}

	return selected, nil
}

func collectTaskYMLModTimesForRun(tasksRoot string) (map[string]int64, error) {
	result := map[string]int64{}
	trimmedRoot := strings.TrimSpace(tasksRoot)
	if trimmedRoot == "" {
		return result, fmt.Errorf("collectTaskYMLModTimesForRun() [run_task.go]: tasks root cannot be empty")
	}

	if _, err := os.Stat(trimmedRoot); err != nil {
		return nil, fmt.Errorf("collectTaskYMLModTimesForRun() [run_task.go]: failed to stat tasks root: %w", err)
	}

	walkErr := filepath.WalkDir(trimmedRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry != nil && entry.IsDir() {
			if path != trimmedRoot && !shared.UUIDPattern.MatchString(strings.TrimSpace(entry.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry == nil || entry.IsDir() || strings.TrimSpace(entry.Name()) != "task.yml" {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}

		taskUUID := strings.TrimSpace(filepath.Base(filepath.Dir(path)))
		if taskUUID == "" {
			return nil
		}
		result[taskUUID] = info.ModTime().UnixNano()
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("collectTaskYMLModTimesForRun() [run_task.go]: failed to collect task modification times: %w", walkErr)
	}

	return result, nil
}

func isUnfinishedTaskForRun(taskData *core.Task) bool {
	if taskData == nil {
		return false
	}

	status := strings.TrimSpace(taskData.Status)
	if status == core.TaskStatusMerged || status == core.TaskStatusRunning || status == core.TaskStatusDraft {
		return false
	}

	return true
}

func listAllCurrentTasksForRun(manager *core.TaskManager) ([]*core.Task, error) {
	if manager == nil {
		return nil, fmt.Errorf("listAllCurrentTasksForRun() [run_task.go]: manager cannot be nil")
	}

	topLevelTasks, err := manager.ListTasks(core.TaskLookup{}, false)
	if err != nil {
		return nil, err
	}

	allTasks := make([]*core.Task, 0, len(topLevelTasks))
	for _, topLevelTask := range topLevelTasks {
		if topLevelTask == nil {
			continue
		}
		allTasks = append(allTasks, topLevelTask)

		children, childErr := manager.ListTasks(core.TaskLookup{Identifier: strings.TrimSpace(topLevelTask.UUID)}, true)
		if childErr != nil {
			return nil, childErr
		}
		allTasks = append(allTasks, children...)
	}

	return allTasks, nil
}

func appendTaskPromptVFSAllowPaths(values []string, taskDir string) []string {
	taskPromptPath := strings.TrimSpace(filepath.Join(strings.TrimSpace(taskDir), "task.md"))
	if taskPromptPath == "" {
		return values
	}
	for _, existingValue := range values {
		if strings.TrimSpace(existingValue) == taskPromptPath {
			return values
		}
	}
	return append(values, taskPromptPath)
}

func applyCommandTaskMetadata(params *RunParams) error {
	if params == nil || params.Task == nil || params.CommandTaskMetadata == nil {
		return nil
	}
	taskDir := strings.TrimSpace(params.Task.TaskDir)
	if taskDir == "" {
		return nil
	}
	taskFilePath := filepath.Join(taskDir, "task.yml")
	taskBytes, err := os.ReadFile(taskFilePath)
	if err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run_task.go]: failed to read task metadata: %w", err)
	}
	var persistedTask core.Task
	if err := yaml.Unmarshal(taskBytes, &persistedTask); err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run_task.go]: failed to parse task metadata: %w", err)
	}
	applyIfUnchanged := func(fieldName string, apply func()) {
		if params.InitialTask == nil {
			apply()
			return
		}
		currentField := reflect.ValueOf(persistedTask).FieldByName(fieldName)
		initialField := reflect.ValueOf(*params.InitialTask).FieldByName(fieldName)
		if !currentField.IsValid() || !initialField.IsValid() {
			return
		}
		if reflect.DeepEqual(currentField.Interface(), initialField.Interface()) {
			apply()
		}
	}
	metadata := params.CommandTaskMetadata
	if metadata.UUID != nil {
		applyIfUnchanged("UUID", func() { persistedTask.UUID = strings.TrimSpace(*metadata.UUID) })
	}
	if metadata.Name != nil {
		applyIfUnchanged("Name", func() { persistedTask.Name = strings.TrimSpace(*metadata.Name) })
	}
	if metadata.Description != nil {
		applyIfUnchanged("Description", func() { persistedTask.Description = strings.TrimSpace(*metadata.Description) })
	}
	if metadata.Status != nil {
		applyIfUnchanged("Status", func() { persistedTask.Status = strings.TrimSpace(*metadata.Status) })
	}
	if metadata.FeatureBranch != nil {
		applyIfUnchanged("FeatureBranch", func() { persistedTask.FeatureBranch = strings.TrimSpace(*metadata.FeatureBranch) })
	}
	if metadata.ParentBranch != nil {
		applyIfUnchanged("ParentBranch", func() { persistedTask.ParentBranch = strings.TrimSpace(*metadata.ParentBranch) })
	}
	if metadata.Role != nil {
		applyIfUnchanged("Role", func() { persistedTask.Role = strings.TrimSpace(*metadata.Role) })
	}
	if metadata.Deps != nil {
		applyIfUnchanged("Deps", func() { persistedTask.Deps = append([]string(nil), (*metadata.Deps)...) })
	}
	if metadata.SessionIDs != nil {
		applyIfUnchanged("SessionIDs", func() { persistedTask.SessionIDs = append([]string(nil), (*metadata.SessionIDs)...) })
	}
	if metadata.SubtaskIDs != nil {
		applyIfUnchanged("SubtaskIDs", func() { persistedTask.SubtaskIDs = append([]string(nil), (*metadata.SubtaskIDs)...) })
	}
	if metadata.ParentTaskID != nil {
		applyIfUnchanged("ParentTaskID", func() { persistedTask.ParentTaskID = strings.TrimSpace(*metadata.ParentTaskID) })
	}
	if metadata.CreatedAt != nil {
		applyIfUnchanged("CreatedAt", func() { persistedTask.CreatedAt = strings.TrimSpace(*metadata.CreatedAt) })
	}
	if metadata.UpdatedAt != nil {
		applyIfUnchanged("UpdatedAt", func() { persistedTask.UpdatedAt = strings.TrimSpace(*metadata.UpdatedAt) })
	}
	updatedBytes, err := yaml.Marshal(&persistedTask)
	if err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run_task.go]: failed to serialize task metadata: %w", err)
	}
	if err := os.WriteFile(taskFilePath, updatedBytes, 0o644); err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run_task.go]: failed to persist task metadata: %w", err)
	}
	return nil
}

func cloneRunTask(task *core.Task) *core.Task {
	if task == nil {
		return nil
	}
	cloned := *task
	cloned.Deps = append([]string(nil), task.Deps...)
	cloned.SessionIDs = append([]string(nil), task.SessionIDs...)
	cloned.SubtaskIDs = append([]string(nil), task.SubtaskIDs...)
	return &cloned
}
