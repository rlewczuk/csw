package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookCommand_RunLLMWithRunIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := t.TempDir()

	oldHome := os.Getenv("HOME")
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Chdir(oldDir)
	})
	require.NoError(t, os.Setenv("HOME", tmpHome))
	require.NoError(t, os.Chdir(tmpDir))

	mockServer := testutil.NewMockHTTPServer()
	t.Cleanup(func() {
		mockServer.Close()
	})

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".csw", "config", "models"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".csw", "config", "hooks", "llm_real"), 0o755))

	providerConfig := fmt.Sprintf(`{
  "type": "ollama",
  "name": "ollama",
  "url": %q
}`+"\n", mockServer.URL())
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".csw", "config", "models", "ollama.json"), []byte(providerConfig), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".csw", "config", "hooks", "llm_real", "llm_real.yml"),
		[]byte("name: llm_real\ndescription: real llm test\nhook: pre_run\ntype: llm\nmodel: ollama/test-model\nprompt: summarize {{.ticket}}\noutput_to: llm_out\n"),
		0o644,
	))

	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"summary ready"},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	cmd := HookCommand()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"run", "llm_real", "--context", "ticket=CSW-321", "--run"})

	err = cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "Hook: llm_real")
	assert.Contains(t, out, "rendered_prompt_or_command:")
	assert.Contains(t, out, "summarize CSW-321")
	assert.Contains(t, out, "stdout:")
	assert.Contains(t, out, "summary ready")
	assert.Contains(t, out, "session_context")
	assert.Contains(t, out, "\"llm_out\": \"summary ready\"")
}
