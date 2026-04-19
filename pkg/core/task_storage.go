package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var taskDirUUIDPattern = regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

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
		return nil, fmt.Errorf("TaskManager.loadAllTasks() [task_storage.go]: failed to stat tasks root: %w", err)
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
		return nil, fmt.Errorf("TaskManager.loadAllTasks() [task_storage.go]: failed to walk tasks root: %w", err)
	}

	return items, nil
}

func (m *TaskManager) findTaskByUUID(taskID string) (string, *Task, error) {
	trimmedID := strings.TrimSpace(taskID)
	if trimmedID == "" {
		return "", nil, fmt.Errorf("TaskManager.findTaskByUUID() [task_storage.go]: task id cannot be empty")
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
	return "", nil, fmt.Errorf("TaskManager.findTaskByUUID() [task_storage.go]: task %q not found", taskID)
}

func (m *TaskManager) listChildTasks(taskDir string, recursive bool) ([]*Task, error) {
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return nil, fmt.Errorf("TaskManager.listChildTasks() [task_storage.go]: failed to read task directory: %w", err)
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
		return fmt.Errorf("TaskManager.writeTaskFile() [task_storage.go]: task is nil")
	}
	bytesData, err := yaml.Marshal(task)
	if err != nil {
		return fmt.Errorf("TaskManager.writeTaskFile() [task_storage.go]: failed to marshal task metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "task.yml"), bytesData, 0644); err != nil {
		return fmt.Errorf("TaskManager.writeTaskFile() [task_storage.go]: failed to write task metadata: %w", err)
	}
	return nil
}

func readTaskFile(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readTaskFile() [task_storage.go]: failed to read task metadata: %w", err)
	}
	var task Task
	if err := yaml.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("readTaskFile() [task_storage.go]: failed to unmarshal task metadata: %w", err)
	}
	task.TaskDir = filepath.Dir(path)
	return &task, nil
}
