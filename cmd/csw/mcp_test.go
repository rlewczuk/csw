package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMCPClient struct {
	startErr      error
	listErr       error
	listResErr    error
	readResErr    error
	tools         []mcp.RemoteTool
	resources     []mcp.RemoteResource
	readResource  *mcp.ReadResourceResult

	started bool
	closed  bool
}

func (c *fakeMCPClient) Start() error {
	c.started = true
	return c.startErr
}

func (c *fakeMCPClient) Close() error {
	c.closed = true
	return nil
}

func (c *fakeMCPClient) ListTools() ([]mcp.RemoteTool, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	return append([]mcp.RemoteTool(nil), c.tools...), nil
}

func (c *fakeMCPClient) ListResources() ([]mcp.RemoteResource, error) {
	if c.listResErr != nil {
		return nil, c.listResErr
	}
	return append([]mcp.RemoteResource(nil), c.resources...), nil
}

func (c *fakeMCPClient) ReadResource(uri string) (*mcp.ReadResourceResult, error) {
	if c.readResErr != nil {
		return nil, c.readResErr
	}
	if c.readResource == nil {
		return nil, fmt.Errorf("fakeMCPClient.ReadResource() [mcp_test.go]: no read resource response for %s", uri)
	}
	return c.readResource, nil
}

func TestMCPCommand_List(t *testing.T) {
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

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".csw", "config", "mcp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".csw", "config", "mcp", "alpha.yml"), []byte("enabled: true\ndescription: Alpha server\ntransport: stdio\ncmd: alpha-server\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".csw", "config", "mcp", "beta.yml"), []byte("enabled: true\ndescription: Beta server\ntransport: http\nurl: http://127.0.0.1:65535/mcp\n"), 0o644))

	t.Run("without status", func(t *testing.T) {
		oldFactory := mcpNewClientFactory
		t.Cleanup(func() { mcpNewClientFactory = oldFactory })
		mcpNewClientFactory = func(name string, cfg *conf.MCPServerConfig) (mcpClient, error) {
			return nil, fmt.Errorf("unexpected client creation for %s", name)
		}

		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"list"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "NAME")
		assert.Contains(t, output, "DESCRIPTION")
		assert.Contains(t, output, "TRANSPORT")
		assert.NotContains(t, output, "STATUS")
		assert.Contains(t, output, "alpha")
		assert.Contains(t, output, "Alpha server")
		assert.Contains(t, output, "beta")
		assert.Contains(t, output, "Beta server")
	})

	t.Run("with status", func(t *testing.T) {
		oldFactory := mcpNewClientFactory
		t.Cleanup(func() { mcpNewClientFactory = oldFactory })

		clients := map[string]*fakeMCPClient{
			"alpha": {tools: []mcp.RemoteTool{{Name: "read_file"}}},
			"beta":  {startErr: fmt.Errorf("dial failed")},
		}
		mcpNewClientFactory = func(name string, cfg *conf.MCPServerConfig) (mcpClient, error) {
			return clients[name], nil
		}

		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"list", "--status"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "STATUS")
		assert.Contains(t, output, "alpha")
		assert.Contains(t, output, "available")
		assert.Contains(t, output, "beta")
		assert.Contains(t, output, "unavailable")
	})
}

func TestMCPCommand_ToolListAndInfo(t *testing.T) {
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

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".csw", "config", "mcp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".csw", "config", "mcp", "srv.yml"), []byte("enabled: true\ntransport: stdio\ncmd: srv-server\ntools:\n  - read_*\n  - exact_tool\n"), 0o644))

	remoteTools := []mcp.RemoteTool{
		{Name: "write_file", Description: "Write file", InputSchema: map[string]any{"type": "object"}},
		{Name: "exact_tool", Description: "Exact tool", InputSchema: map[string]any{"type": "object"}},
		{Name: "read_file", Description: "Read file", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "Path to read"}, "force": map[string]any{"type": "boolean", "description": "Force operation"}}, "required": []any{"path"}}},
	}

	oldFactory := mcpNewClientFactory
	t.Cleanup(func() { mcpNewClientFactory = oldFactory })
	mcpNewClientFactory = func(name string, cfg *conf.MCPServerConfig) (mcpClient, error) {
		return &fakeMCPClient{tools: remoteTools}, nil
	}

	t.Run("tool list shows availability based on filters", func(t *testing.T) {
		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"tool", "list", "srv"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "NAME")
		assert.Contains(t, output, "AVAILABLE")
		assert.Contains(t, output, "read_file")
		assert.Contains(t, output, "exact_tool")
		assert.Contains(t, output, "write_file")

		assert.True(t, strings.Contains(output, "read_file") && strings.Contains(output, "yes"))
		assert.True(t, strings.Contains(output, "exact_tool") && strings.Contains(output, "yes"))
		assert.True(t, strings.Contains(output, "write_file") && strings.Contains(output, "no"))
	})

	t.Run("tool info prints description and parameters", func(t *testing.T) {
		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"tool", "info", "srv", "read_file"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "Server: srv")
		assert.Contains(t, output, "Tool: read_file")
		assert.Contains(t, output, "Description: Read file")
		assert.Contains(t, output, "Parameters:")
		assert.Contains(t, output, "path (type: string, required: yes): Path to read")
		assert.Contains(t, output, "force (type: boolean, required: no): Force operation")
		assert.Contains(t, output, "Input Schema:")
	})

	t.Run("tool info fails for missing tool", func(t *testing.T) {
		cmd := McpCommand()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"tool", "info", "srv", "missing"})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tool \"missing\" not found")
	})
}

func TestMCPCommand_ResourceListAndRead(t *testing.T) {
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

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".csw", "config", "mcp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".csw", "config", "mcp", "srv.yml"), []byte("enabled: true\ntransport: stdio\ncmd: srv-server\n"), 0o644))

	oldFactory := mcpNewClientFactory
	t.Cleanup(func() { mcpNewClientFactory = oldFactory })
	mcpNewClientFactory = func(name string, cfg *conf.MCPServerConfig) (mcpClient, error) {
		return &fakeMCPClient{
			resources: []mcp.RemoteResource{
				{URI: "file:///tmp/a.txt", Name: "A", MimeType: "text/plain", Description: "alpha"},
				{URI: "file:///tmp/b.txt", Name: "B", MimeType: "text/plain", Description: "beta"},
			},
			readResource: &mcp.ReadResourceResult{
				Contents: []mcp.ResourceContent{{URI: "file:///tmp/a.txt", MimeType: "text/plain", Text: "hello from resource"}},
			},
		}, nil
	}

	t.Run("resource list", func(t *testing.T) {
		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"resource", "list", "srv"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "NAME")
		assert.Contains(t, output, "URI")
		assert.Contains(t, output, "MIME TYPE")
		assert.Contains(t, output, "DESCRIPTION")
		assert.Contains(t, output, "file:///tmp/a.txt")
		assert.Contains(t, output, "file:///tmp/b.txt")
	})

	t.Run("resource read", func(t *testing.T) {
		cmd := McpCommand()
		stdout := &bytes.Buffer{}
		cmd.SetOut(stdout)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"resource", "read", "srv", "file:///tmp/a.txt"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "Server: srv")
		assert.Contains(t, output, "Resource: file:///tmp/a.txt")
		assert.Contains(t, output, "MIME Type: text/plain")
		assert.Contains(t, output, "hello from resource")
	})
}
