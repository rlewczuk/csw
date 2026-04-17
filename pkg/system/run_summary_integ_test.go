package system

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
	t.Setenv("HOME", tmpProjectDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpProjectDir, ".config"))
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

	err := runCommand(&RunParams{
		Prompt:        "Do the task",
		ModelName:     "ollama/test-model",
		RoleName:      "developer",
		WorkDir:       tmpProjectDir,
		AllowAllPerms: true,
		ConfigPath:    filepath.Join(tmpProjectDir, ".csw", "config"),
	})
	require.NoError(t, err)

	sessionsDir := filepath.Join(tmpProjectDir, ".cswdata", "logs", "sessions")
	sessionID, summaryPath, err := findLatestSummaryFile(sessionsDir)
	require.NoError(t, err)
	summaryBytes, err := os.ReadFile(summaryPath)
	require.NoError(t, err)
	summary := string(summaryBytes)

	assert.Contains(t, summary, "# Summary\n\nTask completed successfully.")
	assert.Contains(t, summary, "# Session Info")
	assert.Contains(t, summary, "Session completed in ")
	assert.Contains(t, summary, "Model: ollama/test-model")
	assert.Contains(t, summary, "Thinking: -")
	assert.Contains(t, summary, "LSP server: -")
	assert.Contains(t, summary, "Container image: -")
	assert.Contains(t, summary, "Roles used: developer")
	assert.Contains(t, summary, "Tools used: -")
	assert.Contains(t, summary, "Edited files:\n-")
	assert.Contains(t, summary, "Session ID: "+sessionID)
}
