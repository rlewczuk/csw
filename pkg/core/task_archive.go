package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ArchiveTask moves selected task directory to archive root and returns archived task metadata.
func (m *TaskManager) ArchiveTask(lookup TaskLookup) (*Task, error) {
	taskDir, task, err := m.ResolveTask(lookup)
	if err != nil {
		return nil, err
	}

	archivedRoot := filepath.Join(m.TasksRoot(), "archive")
	relativeDir, err := filepath.Rel(m.TasksRoot(), taskDir)
	if err != nil {
		return nil, fmt.Errorf("TaskManager.ArchiveTask() [task_archive.go]: failed to calculate archive path: %w", err)
	}
	if strings.HasPrefix(relativeDir, "..") {
		return nil, fmt.Errorf("TaskManager.ArchiveTask() [task_archive.go]: task path %q is outside of tasks root %q", taskDir, m.TasksRoot())
	}
	destinationDir := filepath.Join(archivedRoot, relativeDir)
	if err := os.MkdirAll(filepath.Dir(destinationDir), 0755); err != nil {
		return nil, fmt.Errorf("TaskManager.ArchiveTask() [task_archive.go]: failed to create archive parent directory: %w", err)
	}
	if err := os.Rename(taskDir, destinationDir); err != nil {
		return nil, fmt.Errorf("TaskManager.ArchiveTask() [task_archive.go]: failed to move task to archive: %w", err)
	}

	return cloneTask(task), nil
}

// ArchiveTasksByStatus moves all tasks with provided status to archive root.
func (m *TaskManager) ArchiveTasksByStatus(status string) ([]*Task, error) {
	trimmedStatus := strings.TrimSpace(status)
	if trimmedStatus == "" {
		return nil, fmt.Errorf("TaskManager.ArchiveTasksByStatus() [task_archive.go]: status cannot be empty")
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

	archivedRoot := filepath.Join(m.TasksRoot(), "archive")
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
			return nil, fmt.Errorf("TaskManager.ArchiveTasksByStatus() [task_archive.go]: failed to calculate archive path: %w", relErr)
		}
		if strings.HasPrefix(relativeDir, "..") {
			return nil, fmt.Errorf("TaskManager.ArchiveTasksByStatus() [task_archive.go]: task path %q is outside of tasks root %q", item.dir, m.TasksRoot())
		}
		destinationDir := filepath.Join(archivedRoot, relativeDir)
		if err := os.MkdirAll(filepath.Dir(destinationDir), 0755); err != nil {
			return nil, fmt.Errorf("TaskManager.ArchiveTasksByStatus() [task_archive.go]: failed to create archive parent directory: %w", err)
		}
		if err := os.Rename(item.dir, destinationDir); err != nil {
			return nil, fmt.Errorf("TaskManager.ArchiveTasksByStatus() [task_archive.go]: failed to move task to archive: %w", err)
		}

		archivedPaths = append(archivedPaths, item.dir)
		archivedTasks = append(archivedTasks, cloneTask(item.task))
	}

	return archivedTasks, nil
}

// isTaskPathNestedUnder reports whether path equals or is nested under parent.
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
