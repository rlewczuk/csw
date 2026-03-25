package system_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweSystem_SubAgentDuplicateSlugAndErrorSummary(t *testing.T) {
	fixture := coretestfixture.NewSweSystemFixture(t)
	system := fixture.System

	parentHandler := testutil.NewMockSessionOutputHandler()
	parent, err := system.NewSession("ollama/test-model:latest", parentHandler)
	require.NoError(t, err)

	tmpLogs := t.TempDir()
	system.LogBaseDir = tmpLogs

	_, err = system.ExecuteSubAgentTask(parent, tool.SubAgentTaskRequest{
		Slug:   "dup-slug",
		Title:  "first",
		Prompt: "first",
	})
	require.NoError(t, err)

	result, err := system.ExecuteSubAgentTask(parent, tool.SubAgentTaskRequest{
		Slug:   "dup-slug",
		Title:  "second",
		Prompt: "second",
	})
	require.NoError(t, err)
	assert.Equal(t, "error", result.Status)

	childSessions := system.ListSessions()
	childrenBySlug := make(map[string]struct{})
	for _, session := range childSessions {
		if session.ParentID() != parent.ID() {
			continue
		}
		childrenBySlug[session.Slug()] = struct{}{}
	}
	assert.Contains(t, childrenBySlug, "dup-slug")
	assert.Contains(t, childrenBySlug, "dup-slug-2")
}

func TestSweSystem_SubAgentWritesErrorSummaryWithParentSession(t *testing.T) {
	fixture := coretestfixture.NewSweSystemFixture(t)
	system := fixture.System

	parentHandler := testutil.NewMockSessionOutputHandler()
	parent, err := system.NewSession("ollama/test-model:latest", parentHandler)
	require.NoError(t, err)

	tmpLogs := t.TempDir()
	system.LogBaseDir = tmpLogs

	result, err := system.ExecuteSubAgentTask(parent, tool.SubAgentTaskRequest{
		Slug:   "failing-child",
		Title:  "failing child",
		Prompt: "trigger failure due to missing mock response",
	})
	require.NoError(t, err)
	assert.Equal(t, "error", result.Status)
	assert.NotContains(t, result.Summary, "session")

	childSessions := system.ListSessions()
	var childSessionID string
	for _, session := range childSessions {
		if session.ID() == parent.ID() {
			continue
		}
		if session.ParentID() == parent.ID() {
			childSessionID = session.ID()
			break
		}
	}
	require.NotEmpty(t, childSessionID)

	summaryPath := filepath.Join(tmpLogs, "sessions", childSessionID, "summary.json")
	summaryBytes, readErr := os.ReadFile(summaryPath)
	require.NoError(t, readErr)

	var summary map[string]any
	require.NoError(t, json.Unmarshal(summaryBytes, &summary))
	assert.Equal(t, "error", summary["status"])
	assert.Equal(t, parent.ID(), summary["parent_session_id"])
}
