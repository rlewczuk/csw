package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLISummaryIncludesDetailedSessionInfo verifies extended session info in summary.md.
func TestCLISummaryIncludesDetailedSessionInfo(t *testing.T) {
	tmpProjectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpProjectDir, ".csw", "config", "models"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpProjectDir, "test.txt"), []byte("old\n"), 0644))
	require.NoError(t, runGit(tmpProjectDir, "init"))
	require.NoError(t, runGit(tmpProjectDir, "config", "user.name", "Test User"))
	require.NoError(t, runGit(tmpProjectDir, "config", "user.email", "test@example.com"))
	require.NoError(t, runGit(tmpProjectDir, "add", "test.txt"))
	require.NoError(t, runGit(tmpProjectDir, "commit", "-m", "initial"))

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

	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"vfsWrite","arguments":{"path":"test.txt","content":"new\nline\n"}}}]},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)
 
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"Done."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	err := runCLI(&CLIParams{
		Prompt:         "Edit test.txt",
		ModelName:      "ollama/test-model",
		RoleName:       "developer",
		WorkDir:        tmpProjectDir,
		AllowAllPerms:  true,
		ConfigPath:     filepath.Join(tmpProjectDir, ".csw", "config"),
		Thinking:       "high",
		LSPServer:      "/not/used/lsp",
		ContainerImage: "alpine:3.20",
	})
	require.NoError(t, err)

	sessionsDir := filepath.Join(tmpProjectDir, ".cswdata", "logs", "sessions")
	sessionID, summaryPath, err := findLatestSummaryFile(sessionsDir)
	require.NoError(t, err)
	summaryBytes, err := os.ReadFile(summaryPath)
	require.NoError(t, err)
	summary := string(summaryBytes)

	assert.Contains(t, summary, "Model: ollama/test-model")
	assert.Contains(t, summary, "Thinking: high")
	assert.Contains(t, summary, "LSP server: /not/used/lsp")
	assert.Contains(t, summary, "Container image: -")
	assert.Contains(t, summary, "Roles used: developer")
	assert.Contains(t, summary, "Tools used: vfsWrite")
	assert.Contains(t, summary, "Edited files:")
	assert.Contains(t, summary, "- test.txt (+2/-1)")
	assert.Contains(t, summary, "Session ID: "+sessionID)
}

func runGit(workDir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("runGit() [cli_summary_details_integ_test.go]: git %v failed: %w: %s", args, err, string(output))
	}

	return nil
}
