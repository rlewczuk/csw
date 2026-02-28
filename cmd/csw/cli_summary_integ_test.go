package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLISavesSummaryMarkdown verifies that CLI mode saves summary.md in the session log directory.
func TestCLISavesSummaryMarkdown(t *testing.T) {
	tmpProjectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpProjectDir, ".csw", "config", "models"), 0755))

	mockServer := testutil.NewMockHTTPServer()
	t.Cleanup(func() {
		mockServer.Close()
	})

	providerConfig := fmt.Sprintf(`{
  "type": "ollama",
  "name": "ollama",
  "url": %q
}`+"\n", mockServer.URL())
	require.NoError(t, os.WriteFile(filepath.Join(tmpProjectDir, ".csw", "config", "models", "ollama.json"), []byte(providerConfig), 0644))

	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Task completed successfully."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	err := runCLI(&CLIParams{
		Prompt:        "Do the task",
		ModelName:     "ollama/test-model",
		RoleName:      "developer",
		WorkDir:       tmpProjectDir,
		AllowAllPerms: true,
		ConfigPath:    filepath.Join(tmpProjectDir, ".csw", "config"),
	})
	require.NoError(t, err)

	sessionsDir := filepath.Join(tmpProjectDir, ".cswdata", "logs", "sessions")
	sessionEntries, err := os.ReadDir(sessionsDir)
	require.NoError(t, err)
	require.Len(t, sessionEntries, 1)

	sessionID := sessionEntries[0].Name()
	summaryPath := filepath.Join(sessionsDir, sessionID, "summary.md")
	summaryBytes, err := os.ReadFile(summaryPath)
	require.NoError(t, err)
	summary := string(summaryBytes)

	assert.Contains(t, summary, "# Summary\n\nTask completed successfully.")
	assert.Contains(t, summary, "# Session Info")
	assert.Contains(t, summary, "Session completed in ")
	assert.Contains(t, summary, "Session ID: "+sessionID)
}
