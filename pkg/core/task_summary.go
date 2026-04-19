package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// TaskOutputMetadata stores metadata section for task output file.
type TaskOutputMetadata struct {
	TaskID        string `json:"task_id" yaml:"task_id"`
	TaskName      string `json:"task_name,omitempty" yaml:"task_name,omitempty"`
	Status        string `json:"status" yaml:"status"`
	UpdatedAt     string `json:"updated_at" yaml:"updated_at"`
	LastSessionID string `json:"last_session_id,omitempty" yaml:"last_session_id,omitempty"`
}

// writeSessionSummary persists summary metadata and markdown for a single task session.
func (m *TaskManager) writeSessionSummary(taskDir string, meta *TaskSessionSummary, summaryText string) error {
	if meta == nil {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task_summary.go]: summary metadata is nil")
	}
	if strings.TrimSpace(meta.SessionID) == "" {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task_summary.go]: session id is empty")
	}

	sessionDir := filepath.Join(taskDir, "ses-"+meta.SessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task_summary.go]: failed to create session directory: %w", err)
	}

	metaBytes, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task_summary.go]: failed to marshal summary metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "summary.yml"), metaBytes, 0644); err != nil {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task_summary.go]: failed to write summary metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "summary.md"), []byte(strings.TrimSpace(summaryText)+"\n"), 0644); err != nil {
		return fmt.Errorf("TaskManager.writeSessionSummary() [task_summary.go]: failed to write summary text: %w", err)
	}

	return nil
}

// writeTaskOutput persists task output markdown with metadata frontmatter.
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
		return fmt.Errorf("TaskManager.writeTaskOutput() [task_summary.go]: failed to marshal task output metadata: %w", err)
	}

	content := strings.Builder{}
	content.WriteString("---\n")
	content.Write(metaBytes)
	content.WriteString("---\n\n")
	content.WriteString(strings.TrimSpace(summaryText))
	content.WriteString("\n")

	if err := os.WriteFile(filepath.Join(taskDir, "output.md"), []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("TaskManager.writeTaskOutput() [task_summary.go]: failed to write task output: %w", err)
	}

	return nil
}

// readLastSessionSummary returns metadata and markdown from the latest task session summary.
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
			return nil, "", fmt.Errorf("TaskManager.readLastSessionSummary() [task_summary.go]: failed to unmarshal summary metadata: %w", err)
		}
	}
	text := ""
	if textErr == nil {
		text = strings.TrimSpace(string(textBytes))
	}

	return meta, text, nil
}
