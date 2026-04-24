package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/spf13/cobra"
)

func taskListCommand() *cobra.Command {
	var recursive bool
	var includeArchived bool
	var statusFilter string

	command := &cobra.Command{
		Use:   "list [name|uuid]",
		Short: "List tasks",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, _, err := loadTaskManager(cmd)
			if err != nil {
				return err
			}
			return runTaskList(manager, args, recursive, includeArchived, statusFilter, os.Stdout)
		},
	}

	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "List recursively")
	command.Flags().BoolVar(&includeArchived, "archived", false, "Include archived tasks")
	command.Flags().StringVar(&statusFilter, "status", "", "Filter by status list (e.g. open,merged) or exclude one/more statuses with !status")

	return command
}

func runTaskList(manager *core.TaskManager, args []string, recursive bool, includeArchived bool, statusFilter string, output io.Writer) error {
	if manager == nil {
		return fmt.Errorf("runTaskList() [task.go]: manager cannot be nil")
	}
	if output == nil {
		return fmt.Errorf("runTaskList() [task.go]: output cannot be nil")
	}

	lookup := core.TaskLookup{}
	if len(args) == 1 {
		lookup.Identifier = strings.TrimSpace(args[0])
	}

	statuses, exclude, err := parseTaskStatusFilter(statusFilter)
	if err != nil {
		return err
	}

	tasks, err := manager.ListTasks(lookup, recursive)
	if err != nil {
		if !(includeArchived && isTaskNotFoundError(err)) {
			return err
		}
		tasks = []*core.Task{}
	}

	modTimes := map[string]int64{}
	currentTimes, err := collectTaskYMLModTimes(manager.TasksRoot())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		for key, value := range currentTimes {
			modTimes[key] = value
		}
	}

	if includeArchived {
		archivedManager, createErr := core.NewTaskManagerWithTasksDir(manager.TasksRoot(), filepath.Join(manager.TasksRoot(), "archive"), nil)
		if createErr != nil {
			return fmt.Errorf("runTaskList() [task.go]: failed to prepare archived task manager: %w", createErr)
		}
		archivedTasks, archivedErr := archivedManager.ListTasks(lookup, recursive)
		if archivedErr != nil {
			if !isTaskNotFoundError(archivedErr) {
				return archivedErr
			}
		} else {
			tasks = append(tasks, archivedTasks...)
		}

		archivedTimes, archivedTimesErr := collectTaskYMLModTimes(filepath.Join(manager.TasksRoot(), "archive"))
		if archivedTimesErr != nil {
			if !errors.Is(archivedTimesErr, os.ErrNotExist) {
				return archivedTimesErr
			}
		} else {
			for key, value := range archivedTimes {
				modTimes[key] = value
			}
		}
	}

	filtered := make([]*core.Task, 0, len(tasks))
	for _, item := range tasks {
		if item == nil {
			continue
		}
		if !matchesStatusFilter(strings.TrimSpace(item.Status), statuses, exclude) {
			continue
		}
		filtered = append(filtered, item)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i]
		right := filtered[j]
		if left == nil || right == nil {
			return i < j
		}

		leftTime := modTimes[strings.TrimSpace(left.UUID)]
		rightTime := modTimes[strings.TrimSpace(right.UUID)]
		if leftTime == rightTime {
			return strings.TrimSpace(left.UUID) < strings.TrimSpace(right.UUID)
		}

		return leftTime < rightTime
	})

	for _, item := range filtered {
		if item == nil {
			continue
		}
		_, _ = fmt.Fprintf(output, "%s\t%s\t%s\n", item.UUID, item.Status, item.Description)
	}

	return nil
}

func collectTaskYMLModTimes(tasksRoot string) (map[string]int64, error) {
	result := map[string]int64{}
	trimmedRoot := strings.TrimSpace(tasksRoot)
	if trimmedRoot == "" {
		return result, fmt.Errorf("collectTaskYMLModTimes() [task.go]: tasks root cannot be empty")
	}

	if _, err := os.Stat(trimmedRoot); err != nil {
		return nil, fmt.Errorf("collectTaskYMLModTimes() [task.go]: failed to stat tasks root: %w", err)
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
		return nil, fmt.Errorf("collectTaskYMLModTimes() [task.go]: failed to collect task modification times: %w", walkErr)
	}

	return result, nil
}

func parseTaskStatusFilter(rawFilter string) (map[string]struct{}, bool, error) {
	trimmedFilter := strings.TrimSpace(rawFilter)
	if trimmedFilter == "" {
		return nil, false, nil
	}

	exclude := false
	if strings.HasPrefix(trimmedFilter, "!") {
		exclude = true
		trimmedFilter = strings.TrimSpace(strings.TrimPrefix(trimmedFilter, "!"))
	}
	if trimmedFilter == "" {
		return nil, false, fmt.Errorf("parseTaskStatusFilter() [task.go]: status filter cannot be empty")
	}

	parts := strings.Split(trimmedFilter, ",")
	statuses := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		status := strings.TrimSpace(part)
		if status == "" {
			return nil, false, fmt.Errorf("parseTaskStatusFilter() [task.go]: status filter contains empty value")
		}
		statuses[status] = struct{}{}
	}

	return statuses, exclude, nil
}

func matchesStatusFilter(taskStatus string, statuses map[string]struct{}, exclude bool) bool {
	if len(statuses) == 0 {
		return true
	}

	_, exists := statuses[strings.TrimSpace(taskStatus)]
	if exclude {
		return !exists
	}

	return exists
}

func isTaskNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
