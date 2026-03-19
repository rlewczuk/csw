package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildQualifiedToolName(t *testing.T) {
	assert.Equal(t, "mcp.server.read", BuildQualifiedToolName("server", "read"))
}

func TestCompileToolMatchers(t *testing.T) {
	matchers, err := compileToolMatchers([]string{"^read", "write$"})
	require.NoError(t, err)
	require.Len(t, matchers, 2)
	assert.True(t, isToolEnabled("read_file", []string{"^read", "write$"}, matchers))
	assert.True(t, isToolEnabled("mywrite", []string{"^read", "write$"}, matchers))
	assert.False(t, isToolEnabled("list", []string{"^read", "write$"}, matchers))

	_, err = compileToolMatchers([]string{"["})
	require.Error(t, err)
}

func TestConvertMCPToolInfo(t *testing.T) {
	info, err := convertMCPToolInfo("mcp.srv.echo", RemoteTool{
		Name:        "echo",
		Description: "Echo tool",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string", "description": "text"},
			},
			"required": []any{"text"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "mcp.srv.echo", info.Name)
	assert.Equal(t, "Echo tool", info.Description)
	assert.Contains(t, info.Schema.Properties, "text")
}

func TestPromptGeneratorOverride(t *testing.T) {
	base := testPromptGenerator{toolInfos: map[string]tool.ToolInfo{
		"base": {Name: "base", Description: "base", Schema: tool.NewToolSchema()},
	}}
	manager := NewMockManager()
	manager.ToolInfosMap["mcp.srv.echo"] = tool.ToolInfo{Name: "mcp.srv.echo", Description: "mcp", Schema: tool.NewToolSchema()}

	pg := NewPromptGenerator(&base, manager)
	info, err := pg.GetToolInfo(nil, "mcp.srv.echo", &conf.AgentRoleConfig{Name: "dev"}, &core.AgentState{})
	require.NoError(t, err)
	assert.Equal(t, "mcp", info.Description)

	info, err = pg.GetToolInfo(nil, "base", &conf.AgentRoleConfig{Name: "dev"}, &core.AgentState{})
	require.NoError(t, err)
	assert.Equal(t, "base", info.Description)
}

func TestRegisterToolsAndForwarding(t *testing.T) {
	registry := tool.NewToolRegistry()
	manager := NewMockManager()
	manager.ToolInfosMap["mcp.srv.echo"] = tool.ToolInfo{Name: "mcp.srv.echo", Description: "echo", Schema: tool.NewToolSchema()}
	manager.Responses["mcp.srv.echo"] = &tool.ToolResponse{Result: tool.NewToolValue(map[string]any{"ok": true}), Done: true}

	err := RegisterTools(registry, manager)
	require.NoError(t, err)

	toolImpl, err := registry.Get("mcp.srv.echo")
	require.NoError(t, err)
	resp := toolImpl.Execute(&tool.ToolCall{Function: "mcp.srv.echo", Arguments: tool.NewToolValue(map[string]any{"a": 1})})
	require.NotNil(t, resp)
	assert.True(t, resp.Done)
	require.Len(t, manager.Calls, 1)
	assert.Equal(t, "mcp.srv.echo", manager.Calls[0].Function)
}

type testPromptGenerator struct {
	toolInfos map[string]tool.ToolInfo
}

func (g *testPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *core.AgentState) (string, error) {
	return "prompt", nil
}

func (g *testPromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *core.AgentState) (tool.ToolInfo, error) {
	if info, ok := g.toolInfos[toolName]; ok {
		return info, nil
	}
	return tool.ToolInfo{Name: toolName, Description: "missing", Schema: tool.NewToolSchema()}, nil
}

func (g *testPromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	return map[string]string{}, nil
}

func TestReadWriteMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	req := JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: 1, Method: "ping", Params: map[string]any{"a": 1}}
	require.NoError(t, WriteMessage(buf, req))

	content, err := ReadMessage(bufio.NewReader(bytes.NewReader(buf.Bytes())))
	require.NoError(t, err)

	var decoded JSONRPCRequest
	require.NoError(t, json.Unmarshal(content, &decoded))
	assert.Equal(t, int64(1), decoded.ID)
	assert.Equal(t, "ping", decoded.Method)
}

func TestNewManagerWithDisabledServerSkipsStart(t *testing.T) {
	store := confimpl.NewMockConfigStore()
	store.SetMCPServerConfigs(map[string]*conf.MCPServerConfig{
		"disabled": {Enabled: false, Cmd: "echo"},
	})

	manager, err := NewManager(store)
	require.NoError(t, err)
	require.NotNil(t, manager)
	assert.Empty(t, manager.ToolInfos())
}

func TestNewManagerStartsEnabledServerAndRunsTool(t *testing.T) {
	original := newClientFunc
	t.Cleanup(func() { newClientFunc = original })

	mockClient := &stubClient{
		tools: []RemoteTool{{
			Name:        "echo",
			Description: "echo",
			InputSchema: map[string]any{"type": "object"},
		}},
		callResult: &CallToolResult{Content: []MCPContentBlock{{Type: "text", Text: "ok"}}},
	}
	newClientFunc = func(name string, cfg *conf.MCPServerConfig) (client, error) {
		return mockClient, nil
	}

	store := confimpl.NewMockConfigStore()
	store.SetMCPServerConfigs(map[string]*conf.MCPServerConfig{
		"srv": {Enabled: true, Transport: conf.MCPTransportTypeStdio, Cmd: "dummy", Tools: nil},
	})

	manager, err := NewManager(store)
	require.NoError(t, err)
	require.NotNil(t, manager)
	require.Len(t, manager.ToolInfos(), 1)

	resp := manager.ExecuteTool(&tool.ToolCall{Function: "mcp.srv.echo", Arguments: tool.NewToolValue(map[string]any{"v": 1})})
	require.NotNil(t, resp)
	assert.NoError(t, resp.Error)
	assert.Equal(t, "ok", resp.Result.Get("text").AsString())

	require.NoError(t, manager.Close())
	assert.True(t, mockClient.started)
	assert.True(t, mockClient.closed)
}

func TestNewManagerFiltersToolsByRegex(t *testing.T) {
	original := newClientFunc
	t.Cleanup(func() { newClientFunc = original })

	mockClient := &stubClient{tools: []RemoteTool{
		{Name: "read_file", InputSchema: map[string]any{"type": "object"}},
		{Name: "write_file", InputSchema: map[string]any{"type": "object"}},
	}}
	newClientFunc = func(name string, cfg *conf.MCPServerConfig) (client, error) {
		return mockClient, nil
	}

	store := confimpl.NewMockConfigStore()
	store.SetMCPServerConfigs(map[string]*conf.MCPServerConfig{
		"srv": {Enabled: true, Transport: conf.MCPTransportTypeStdio, Cmd: "dummy", Tools: []string{"^read_"}},
	})

	manager, err := NewManager(store)
	require.NoError(t, err)
	infos := manager.ToolInfos()
	require.Len(t, infos, 1)
	_, hasRead := infos["mcp.srv.read_file"]
	_, hasWrite := infos["mcp.srv.write_file"]
	assert.True(t, hasRead)
	assert.False(t, hasWrite)
}

func TestNewClient_SelectsTransport(t *testing.T) {
	t.Run("defaults to stdio when transport not provided", func(t *testing.T) {
		c, err := NewClient("srv", &conf.MCPServerConfig{Cmd: "dummy"})
		require.NoError(t, err)
		require.NotNil(t, c)
		_, ok := c.(*Client)
		assert.True(t, ok)
	})

	t.Run("creates http client for http transport", func(t *testing.T) {
		c, err := NewClient("srv", &conf.MCPServerConfig{Transport: conf.MCPTransportTypeHTTP, URL: "http://example.com/mcp"})
		require.NoError(t, err)
		require.NotNil(t, c)
		_, ok := c.(*HTTPClient)
		assert.True(t, ok)
	})

	t.Run("creates http client for https transport", func(t *testing.T) {
		c, err := NewClient("srv", &conf.MCPServerConfig{Transport: conf.MCPTransportTypeHTTPS, URL: "https://example.com/mcp"})
		require.NoError(t, err)
		require.NotNil(t, c)
		_, ok := c.(*HTTPClient)
		assert.True(t, ok)
	})
}

func TestNewHTTPClientValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *conf.MCPServerConfig
		hasErr  bool
		errPart string
	}{
		{name: "nil config", cfg: nil, hasErr: true, errPart: "config cannot be nil"},
		{name: "empty url", cfg: &conf.MCPServerConfig{Transport: conf.MCPTransportTypeHTTP}, hasErr: true, errPart: "url cannot be empty"},
		{name: "invalid url", cfg: &conf.MCPServerConfig{Transport: conf.MCPTransportTypeHTTP, URL: "://bad"}, hasErr: true, errPart: "invalid url"},
		{name: "http transport with https url", cfg: &conf.MCPServerConfig{Transport: conf.MCPTransportTypeHTTP, URL: "https://example.com/mcp"}, hasErr: true, errPart: "http transport requires http url"},
		{name: "https transport with http url", cfg: &conf.MCPServerConfig{Transport: conf.MCPTransportTypeHTTPS, URL: "http://example.com/mcp"}, hasErr: true, errPart: "https transport requires https url"},
		{name: "valid http", cfg: &conf.MCPServerConfig{Transport: conf.MCPTransportTypeHTTP, URL: "http://example.com/mcp"}, hasErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewHTTPClient("srv", tt.cfg)
			if tt.hasErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errPart)
				assert.Nil(t, client)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}
}

func TestHTTPClientStartListToolsAndCallTool(t *testing.T) {
	server := testutil.NewMockHTTPServer()
	defer server.Close()

	server.AddRestResponseWithStatusAndHeaders(
		"/mcp",
		http.MethodPost,
		`{"jsonrpc":"2.0","id":2,"result":{"protocolVersion":"2025-11-25","capabilities":{},"serverInfo":{"name":"mock","version":"1.0"}}}`,
		http.StatusOK,
		http.Header{mcpSessionIDHeader: []string{"session-123"}},
	)
	server.AddRestResponseWithStatus("/mcp", http.MethodPost, "", http.StatusAccepted)
	server.AddRestResponse("/mcp", http.MethodPost, `{"jsonrpc":"2.0","id":3,"result":{"tools":[{"name":"echo","description":"Echo","inputSchema":{"type":"object"}}]}}`)
	server.AddRestResponse("/mcp", http.MethodPost, `{"jsonrpc":"2.0","id":4,"result":{"content":[{"type":"text","text":"ok"}],"isError":false}}`)
	server.AddRestResponseWithStatus("/mcp", http.MethodDelete, "", http.StatusNoContent)

	client, err := NewHTTPClient("srv", &conf.MCPServerConfig{Transport: conf.MCPTransportTypeHTTP, URL: server.URL() + "/mcp", APIKey: "secret-token"})
	require.NoError(t, err)

	require.NoError(t, client.Start())

	tools, err := client.ListTools()
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "echo", tools[0].Name)

	result, err := client.CallTool("echo", map[string]any{"text": "hello"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "ok", result.Content[0].Text)

	require.NoError(t, client.Close())

	requests := server.GetRequests()
	require.Len(t, requests, 5)

	for i, req := range requests[:4] {
		assert.Equal(t, "Bearer secret-token", req.Header.Get("Authorization"))
		assert.Equal(t, "application/json, text/event-stream", req.Header.Get("Accept"))
		assert.Equal(t, LatestProtocolVersion, req.Header.Get(mcpProtocolVersionHeader))
		if i == 0 {
			assert.Empty(t, req.Header.Get(mcpSessionIDHeader))
		} else {
			assert.Equal(t, "session-123", req.Header.Get(mcpSessionIDHeader))
		}
	}

	assert.Equal(t, "session-123", requests[4].Header.Get(mcpSessionIDHeader))
	assert.Equal(t, LatestProtocolVersion, requests[4].Header.Get(mcpProtocolVersionHeader))
}

func TestHTTPClientHandlesSSEResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("data: {\"jsonrpc\":\"2.0\",\"id\":99,\"result\":{\"tools\":[]}}\n\n"))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewHTTPClient("srv", &conf.MCPServerConfig{Transport: conf.MCPTransportTypeHTTP, URL: server.URL})
	require.NoError(t, err)
	client.nextID.Store(98)

	tools, err := client.ListTools()
	require.NoError(t, err)
	assert.Empty(t, tools)
}

func TestHTTPClientNotificationRequiresAccepted(t *testing.T) {
	server := testutil.NewMockHTTPServer()
	defer server.Close()

	server.AddRestResponseWithStatus("/mcp", http.MethodPost, `{"jsonrpc":"2.0","id":2,"result":{"protocolVersion":"2025-11-25","capabilities":{},"serverInfo":{"name":"mock","version":"1.0"}}}`, http.StatusOK)
	server.AddRestResponseWithStatus("/mcp", http.MethodPost, "{}", http.StatusOK)

	client, err := NewHTTPClient("srv", &conf.MCPServerConfig{Transport: conf.MCPTransportTypeHTTP, URL: server.URL() + "/mcp"})
	require.NoError(t, err)

	err = client.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 202 Accepted")
}

func TestReadSSEJSONRPCResponse(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		expectedID int64
		hasErr     bool
	}{
		{
			name:       "returns matching event",
			payload:    "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"ok\":true}}\n\n",
			expectedID: 1,
			hasErr:     false,
		},
		{
			name:       "ignores non-matching and returns matching later",
			payload:    "data: {\"jsonrpc\":\"2.0\",\"id\":7,\"result\":{}}\n\ndata: {\"jsonrpc\":\"2.0\",\"id\":8,\"result\":{\"ok\":true}}\n\n",
			expectedID: 8,
			hasErr:     false,
		},
		{
			name:       "returns error when no matching event",
			payload:    "data: [DONE]\n\n",
			expectedID: 1,
			hasErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := readSSEJSONRPCResponse(bytes.NewBufferString(tt.payload), tt.expectedID)
			if tt.hasErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tt.expectedID, resp.ID)
		})
	}
}

type stubClient struct {
	started    bool
	closed     bool
	tools      []RemoteTool
	callResult *CallToolResult
}

func (s *stubClient) Start() error {
	s.started = true
	return nil
}

func (s *stubClient) Close() error {
	s.closed = true
	return nil
}

func (s *stubClient) ListTools() ([]RemoteTool, error) {
	return s.tools, nil
}

func (s *stubClient) CallTool(name string, arguments map[string]any) (*CallToolResult, error) {
	if s.callResult == nil {
		return nil, fmt.Errorf("stubClient.CallTool() [mcp_test.go]: no result")
	}
	return s.callResult, nil
}
