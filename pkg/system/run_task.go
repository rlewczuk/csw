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

func loadRunTaskManager(taskDir string, workDir string, shadowDir string, projectConfig string, configPath string) (*core.TaskManager, error) {
	resolvedWorkDir, err := ResolveWorkDir(workDir)
	if err != nil {
		return nil, err
	}
	resolvedTaskDir, err := resolveRunTaskDirPath(taskDir, resolvedWorkDir, shadowDir, projectConfig, configPath)
	if err != nil {
		return nil, err
	}

	resolvedConfigPath, err := BuildConfigPath(projectConfig, configPath)
	if err != nil {
		return nil, err
	}
	resolvedConfigPath, err = ResolveConfigPathForProjectRoot(resolvedConfigPath, resolvedWorkDir)
	if err != nil {
		return nil, fmt.Errorf("loadRunTaskManager() [run_task.go]: failed to resolve config path for project root: %w", err)
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

func resolveRunTaskDirPath(taskDir string, workDir string, shadowDir string, projectConfig string, configPath string) (string, error) {
	resolvedTaskDir := strings.TrimSpace(taskDir)
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
		fallbackRootDir := strings.TrimSpace(workDir)
		trimmedShadowDir := strings.TrimSpace(shadowDir)
		if trimmedShadowDir != "" {
			fallbackRootDir = trimmedShadowDir
		}
		resolvedTaskDir = filepath.Join(fallbackRootDir, ".cswdata", "tasks")
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
	if status == core.TaskStatusMerged || status == core.TaskStatusCompleted || status == core.TaskStatusRunning || status == core.TaskStatusDraft {
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

func applyCommandTaskMetadata(params *RunExecution) error {
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
	if metadata.FieldsPresent&core.TaskFieldUUID != 0 {
		applyIfUnchanged("UUID", func() { persistedTask.UUID = strings.TrimSpace(metadata.UUID) })
	}
	if metadata.FieldsPresent&core.TaskFieldName != 0 {
		applyIfUnchanged("Name", func() { persistedTask.Name = strings.TrimSpace(metadata.Name) })
	}
	if metadata.FieldsPresent&core.TaskFieldDescription != 0 {
		applyIfUnchanged("Description", func() { persistedTask.Description = strings.TrimSpace(metadata.Description) })
	}
	if metadata.FieldsPresent&core.TaskFieldStatus != 0 {
		applyIfUnchanged("Status", func() { persistedTask.Status = strings.TrimSpace(metadata.Status) })
	}
	if metadata.FieldsPresent&core.TaskFieldFeatureBranch != 0 {
		applyIfUnchanged("FeatureBranch", func() { persistedTask.FeatureBranch = strings.TrimSpace(metadata.FeatureBranch) })
	}
	if metadata.FieldsPresent&core.TaskFieldNoCommit != 0 {
		applyIfUnchanged("NoCommit", func() { persistedTask.NoCommit = metadata.NoCommit })
	}
	if metadata.FieldsPresent&core.TaskFieldParentBranch != 0 {
		applyIfUnchanged("ParentBranch", func() { persistedTask.ParentBranch = strings.TrimSpace(metadata.ParentBranch) })
	}
	if metadata.FieldsPresent&core.TaskFieldRole != 0 {
		applyIfUnchanged("Role", func() { persistedTask.Role = strings.TrimSpace(metadata.Role) })
	}
	if metadata.FieldsPresent&core.TaskFieldDeps != 0 {
		applyIfUnchanged("Deps", func() { persistedTask.Deps = append([]string(nil), metadata.Deps...) })
	}
	if metadata.FieldsPresent&core.TaskFieldSessionIDs != 0 {
		applyIfUnchanged("SessionIDs", func() { persistedTask.SessionIDs = append([]string(nil), metadata.SessionIDs...) })
	}
	if metadata.FieldsPresent&core.TaskFieldSubtaskIDs != 0 {
		applyIfUnchanged("SubtaskIDs", func() { persistedTask.SubtaskIDs = append([]string(nil), metadata.SubtaskIDs...) })
	}
	if metadata.FieldsPresent&core.TaskFieldParentTaskID != 0 {
		applyIfUnchanged("ParentTaskID", func() { persistedTask.ParentTaskID = strings.TrimSpace(metadata.ParentTaskID) })
	}
	if metadata.FieldsPresent&core.TaskFieldCreatedAt != 0 {
		applyIfUnchanged("CreatedAt", func() { persistedTask.CreatedAt = strings.TrimSpace(metadata.CreatedAt) })
	}
	if metadata.FieldsPresent&core.TaskFieldUpdatedAt != 0 {
		applyIfUnchanged("UpdatedAt", func() { persistedTask.UpdatedAt = strings.TrimSpace(metadata.UpdatedAt) })
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
