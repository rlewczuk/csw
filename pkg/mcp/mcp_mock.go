package mcp

import (
	"fmt"
	"sync"

	"github.com/rlewczuk/csw/pkg/tool"
)

// MockManager is reusable MCP manager mock for tests.
type MockManager struct {
	mu           sync.RWMutex
	ToolInfosMap map[string]tool.ToolInfo
	Responses    map[string]*tool.ToolResponse
	Calls        []*tool.ToolCall
	CloseErr     error
}

// NewMockManager creates mock manager with initialized maps.
func NewMockManager() *MockManager {
	return &MockManager{
		ToolInfosMap: make(map[string]tool.ToolInfo),
		Responses:    make(map[string]*tool.ToolResponse),
		Calls:        make([]*tool.ToolCall, 0),
	}
}

// ToolInfos returns configured tool infos.
func (m *MockManager) ToolInfos() map[string]tool.ToolInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]tool.ToolInfo, len(m.ToolInfosMap))
	for key, value := range m.ToolInfosMap {
		out[key] = value
	}
	return out
}

// GetToolInfo returns tool info by name.
func (m *MockManager) GetToolInfo(toolName string) (tool.ToolInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.ToolInfosMap[toolName]
	return info, ok
}

// ExecuteTool records call and returns configured response.
func (m *MockManager) ExecuteTool(call *tool.ToolCall) *tool.ToolResponse {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, call)
	if call == nil {
		return &tool.ToolResponse{Call: call, Error: fmt.Errorf("MockManager.ExecuteTool() [mcp_mock.go]: call cannot be nil"), Done: true}
	}
	if response, ok := m.Responses[call.Function]; ok {
		return response
	}
	return &tool.ToolResponse{Call: call, Error: fmt.Errorf("MockManager.ExecuteTool() [mcp_mock.go]: no response for %s", call.Function), Done: true}
}

// Close returns configured close error.
func (m *MockManager) Close() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.CloseErr
}
