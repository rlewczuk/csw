package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIContextPromptRenderingIntegration(t *testing.T) {
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
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"done"},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	cmd := RunCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"--workdir", tmpProjectDir,
		"--config-path", filepath.Join(tmpProjectDir, ".csw", "config"),
		"--model", "ollama/test-model",
		"--allow-all-permissions",
		"--context", "PROJECT=csw",
		"--context", "ENV=staging",
		"Deploy {{.PROJECT}} to {{.ENV}}",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	requests := mockServer.GetRequests()
	require.NotEmpty(t, requests)

	foundRendered := false
	foundRawTemplate := false
	for _, req := range requests {
		if req.Path != "/api/chat" || req.Method != "POST" {
			continue
		}
		body := string(req.Body)
		if strings.Contains(body, "Deploy csw to staging") {
			foundRendered = true
		}
		if strings.Contains(body, "Deploy {{.PROJECT}} to {{.ENV}}") {
			foundRawTemplate = true
		}
	}

	assert.True(t, foundRendered, "expected rendered prompt in model request")
	assert.False(t, foundRawTemplate, "expected template prompt to be rendered before model request")
}
