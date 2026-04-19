package main

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/stretchr/testify/assert"
)

func TestPrintTaskHuman(t *testing.T) {
	taskData := &core.Task{
		UUID:          "task-uuid",
		Name:          "task-name",
		Description:   "task-description",
		Status:        core.TaskStatusOpen,
		FeatureBranch: "feature/task",
		ParentBranch:  "main",
		Role:          "developer",
		ParentTaskID:  "parent-uuid",
		Deps:          []string{"dep-a", "dep-b"},
		SessionIDs:    []string{"ses-1"},
		SubtaskIDs:    []string{"sub-1"},
		CreatedAt:     "2026-01-01T10:00:00Z",
		UpdatedAt:     "2026-01-01T10:01:00Z",
	}
	meta := &core.TaskSessionSummary{SessionID: "ses-1", Status: core.TaskStatusCompleted}

	output := captureStdout(t, func() {
		printTaskHuman(taskData, meta, " latest summary ")
	})

	assert.Contains(t, output, "UUID: task-uuid")
	assert.Contains(t, output, "Name: task-name")
	assert.Contains(t, output, "Feature branch: feature/task")
	assert.Contains(t, output, "Last summary session: ses-1 (completed)")
	assert.Contains(t, output, "latest summary")
}

func TestPrintTaskHumanNilTaskDoesNothing(t *testing.T) {
	output := captureStdout(t, func() {
		printTaskHuman(nil, nil, "")
	})

	assert.Empty(t, output)
}
