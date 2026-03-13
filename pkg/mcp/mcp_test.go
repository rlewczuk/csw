package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
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
		"srv": {Enabled: true, Cmd: "dummy", Tools: nil},
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
		"srv": {Enabled: true, Cmd: "dummy", Tools: []string{"^read_"}},
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
