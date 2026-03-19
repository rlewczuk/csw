package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPCommand_HTTPIntegration(t *testing.T) {
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

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".csw", "config", "mcp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".csw", "config", "mcp", "remote.yml"), []byte(fmt.Sprintf("enabled: true\ndescription: Remote MCP\ntransport: http\nurl: %q\n", mockServer.URL()+"/mcp")), 0o644))

	queueMCPOperationCycle(mockServer)
	queueMCPOperationCycle(mockServer)
	queueMCPOperationCycle(mockServer)
	queueMCPResourceListCycle(mockServer)
	queueMCPResourceReadCycle(mockServer)

	t.Run("mcp list --status", func(t *testing.T) {
		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"list", "--status"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "remote")
		assert.Contains(t, output, "Remote MCP")
		assert.Contains(t, output, "http")
		assert.Contains(t, output, "available")
	})

	t.Run("mcp tool list", func(t *testing.T) {
		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tool", "list", "remote"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "read_file")
		assert.Contains(t, output, "Read file")
		assert.Contains(t, output, "yes")
	})

	t.Run("mcp tool info", func(t *testing.T) {
		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"tool", "info", "remote", "read_file"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "Server: remote")
		assert.Contains(t, output, "Tool: read_file")
		assert.Contains(t, output, "Description: Read file")
		assert.Contains(t, output, "path (type: string, required: yes): Path to read")
	})

	t.Run("mcp resource list", func(t *testing.T) {
		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"resource", "list", "remote"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "NAME")
		assert.Contains(t, output, "URI")
		assert.Contains(t, output, "README")
		assert.Contains(t, output, "file:///workspace/README.md")
	})

	t.Run("mcp resource read", func(t *testing.T) {
		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"resource", "read", "remote", "file:///workspace/README.md"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "Server: remote")
		assert.Contains(t, output, "Resource: file:///workspace/README.md")
		assert.Contains(t, output, "MIME Type: text/markdown")
		assert.Contains(t, output, "# Demo")
	})
}

func queueMCPOperationCycle(server *testutil.MockHTTPServer) {
	if server == nil {
		return
	}

	server.AddRestResponseWithStatusAndHeaders(
		"/mcp",
		http.MethodPost,
		`{"jsonrpc":"2.0","id":2,"result":{"protocolVersion":"2025-11-25","capabilities":{},"serverInfo":{"name":"mock","version":"1.0"}}}`,
		http.StatusOK,
		http.Header{"Mcp-Session-Id": []string{"session-123"}},
	)
	server.AddRestResponseWithStatus("/mcp", http.MethodPost, "", http.StatusAccepted)
	server.AddRestResponse(
		"/mcp",
		http.MethodPost,
		`{"jsonrpc":"2.0","id":3,"result":{"tools":[{"name":"read_file","description":"Read file","inputSchema":{"type":"object","properties":{"path":{"type":"string","description":"Path to read"}},"required":["path"]}}]}}`,
	)
}

func queueMCPResourceListCycle(server *testutil.MockHTTPServer) {
	if server == nil {
		return
	}

	server.AddRestResponseWithStatusAndHeaders(
		"/mcp",
		http.MethodPost,
		`{"jsonrpc":"2.0","id":2,"result":{"protocolVersion":"2025-11-25","capabilities":{},"serverInfo":{"name":"mock","version":"1.0"}}}`,
		http.StatusOK,
		http.Header{"Mcp-Session-Id": []string{"session-123"}},
	)
	server.AddRestResponseWithStatus("/mcp", http.MethodPost, "", http.StatusAccepted)
	server.AddRestResponse(
		"/mcp",
		http.MethodPost,
		`{"jsonrpc":"2.0","id":3,"result":{"resources":[{"uri":"file:///workspace/README.md","name":"README","mimeType":"text/markdown","description":"Project readme"}]}}`,
	)
}

func queueMCPResourceReadCycle(server *testutil.MockHTTPServer) {
	if server == nil {
		return
	}

	server.AddRestResponseWithStatusAndHeaders(
		"/mcp",
		http.MethodPost,
		`{"jsonrpc":"2.0","id":2,"result":{"protocolVersion":"2025-11-25","capabilities":{},"serverInfo":{"name":"mock","version":"1.0"}}}`,
		http.StatusOK,
		http.Header{"Mcp-Session-Id": []string{"session-123"}},
	)
	server.AddRestResponseWithStatus("/mcp", http.MethodPost, "", http.StatusAccepted)
	server.AddRestResponse(
		"/mcp",
		http.MethodPost,
		`{"jsonrpc":"2.0","id":3,"result":{"contents":[{"uri":"file:///workspace/README.md","mimeType":"text/markdown","text":"# Demo\n"}]}}`,
	)
}
