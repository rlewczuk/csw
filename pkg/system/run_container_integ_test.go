package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/testutil/cfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLIContainerModeRunBashBusybox verifies runBash execution in container mode.
func TestCLIContainerModeRunBashBusybox(t *testing.T) {
	if !cfg.TestEnabled("runc") {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

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

	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"runBash","arguments":{"command":"bash -lc 'echo inside'"}}}]},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"done"},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	err := RunCommand(&RunParams{
		Prompt:           "Run a shell command",
		ModelName:        "ollama/test-model",
		RoleName:         "developer",
		WorkDir:          tmpProjectDir,
		AllowAllPerms:    true,
		ContainerEnabled: true,
		ContainerImage:   "busybox:latest",
		ConfigPath:       filepath.Join(tmpProjectDir, ".csw", "config"),
	})
	require.NoError(t, err)

	requests := mockServer.GetRequests()
	require.GreaterOrEqual(t, len(requests), 2)

	toolResultPayload := ""
	for _, req := range requests {
		body := string(req.Body)
		if strings.Contains(body, `"tool_name":"runBash"`) {
			toolResultPayload = body
			break
		}
	}

	require.NotEmpty(t, toolResultPayload, "expected request payload containing runBash tool result")
	assert.Contains(t, toolResultPayload, `exit_code`)
	assert.Contains(t, toolResultPayload, `127`)
	assert.True(t,
		strings.Contains(strings.ToLower(toolResultPayload), "not found") || strings.Contains(strings.ToLower(toolResultPayload), "bash"),
		"expected busybox container to report missing bash",
	)
}

func TestCLIContainerModeFromGlobalConfig(t *testing.T) {
	if !cfg.TestEnabled("runc") {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

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

	globalConfig := `{
  "container": {
    "enabled": true,
    "image": "busybox:latest"
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpProjectDir, ".csw", "config", "global.json"), []byte(globalConfig), 0644))

	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"runBash","arguments":{"command":"bash -lc 'echo inside'"}}}]},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"done"},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	err := RunCommand(&RunParams{
		Prompt:        "Run a shell command",
		ModelName:     "ollama/test-model",
		RoleName:      "developer",
		WorkDir:       tmpProjectDir,
		AllowAllPerms: true,
		ConfigPath:    filepath.Join(tmpProjectDir, ".csw", "config"),
	})
	require.NoError(t, err)

	requests := mockServer.GetRequests()
	require.GreaterOrEqual(t, len(requests), 2)

	toolResultPayload := ""
	for _, req := range requests {
		body := string(req.Body)
		if strings.Contains(body, `"tool_name":"runBash"`) {
			toolResultPayload = body
			break
		}
	}

	require.NotEmpty(t, toolResultPayload)
	assert.Contains(t, toolResultPayload, `exit_code`)
	assert.Contains(t, toolResultPayload, `127`)
}

func TestCLIContainerModeWithAdditionalMountAndEnv(t *testing.T) {
	if !cfg.TestEnabled("runc") {
		t.Skip("Skipping container integration test (runc.enabled or all.enabled not set to 'yes')")
	}

	tmpProjectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpProjectDir, ".csw", "config", "models"), 0755))

	extraMountDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(extraMountDir, "flag.txt"), []byte("from_mount"), 0644))

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
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"runBash","arguments":{"command":"sh -c 'cat /mnt/extra/flag.txt && echo $EXTRA_ENV'"}}}]},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"done"},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	err := RunCommand(&RunParams{
		Prompt:           "Run a shell command",
		ModelName:        "ollama/test-model",
		RoleName:         "developer",
		WorkDir:          tmpProjectDir,
		AllowAllPerms:    true,
		ContainerEnabled: true,
		ContainerImage:   "busybox:latest",
		ContainerMounts:  []string{extraMountDir + ":/mnt/extra"},
		ContainerEnv:     []string{"EXTRA_ENV=from_env"},
		ConfigPath:       filepath.Join(tmpProjectDir, ".csw", "config"),
	})
	require.NoError(t, err)

	requests := mockServer.GetRequests()
	require.GreaterOrEqual(t, len(requests), 2)

	toolResultPayload := ""
	for _, req := range requests {
		body := string(req.Body)
		if strings.Contains(body, `"tool_name":"runBash"`) {
			toolResultPayload = body
			break
		}
	}

	require.NotEmpty(t, toolResultPayload)
	assert.Contains(t, toolResultPayload, `exit_code`)
	assert.Contains(t, toolResultPayload, `"exit_code":0`)
	assert.Contains(t, toolResultPayload, "from_mount")
	assert.Contains(t, toolResultPayload, "from_env")
}
