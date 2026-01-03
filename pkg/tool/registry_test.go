package tool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// MockTool implements Tool interface for testing.
type MockTool struct {
	name string
}

func (m *MockTool) Info() ToolInfo {
	schema := NewToolSchema()
	schema.AddProperty("arg1", PropertySchema{
		Type:        SchemaTypeString,
		Description: "A test argument.",
	}, false)

	return ToolInfo{
		Name:        m.name,
		Description: "A mock tool for testing.",
		Schema:      schema,
	}
}

func (m *MockTool) Execute(args ToolCall) ToolResponse {
	result := ToolResult{}
	result.Set("executed", m.name)
	return ToolResponse{
		ID:     args.ID,
		Error:  nil,
		Result: result,
		Done:   true,
	}
}

func TestToolRegistry_Register(t *testing.T) {
	registry := NewToolRegistry()
	tool := &MockTool{name: "test"}

	registry.Register("test", tool)

	retrievedTool, err := registry.Get("test")
	require.NoError(t, err)
	assert.Equal(t, tool, retrievedTool)
}

func TestToolRegistry_RegisterMultipleNames(t *testing.T) {
	registry := NewToolRegistry()
	tool := &MockTool{name: "multi"}

	registry.Register("name1", tool)
	registry.Register("name2", tool)

	retrievedTool1, err := registry.Get("name1")
	require.NoError(t, err)
	assert.Equal(t, tool, retrievedTool1)

	retrievedTool2, err := registry.Get("name2")
	require.NoError(t, err)
	assert.Equal(t, tool, retrievedTool2)
}

func TestToolRegistry_GetNotFound(t *testing.T) {
	registry := NewToolRegistry()

	tool, err := registry.Get("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, tool)
	assert.Contains(t, err.Error(), "tool not found: nonexistent")
}

func TestToolRegistry_List(t *testing.T) {
	registry := NewToolRegistry()
	tool1 := &MockTool{name: "tool1"}
	tool2 := &MockTool{name: "tool2"}

	registry.Register("name1", tool1)
	registry.Register("name2", tool2)

	names := registry.List()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "name1")
	assert.Contains(t, names, "name2")
}

func TestToolRegistry_Execute(t *testing.T) {
	registry := NewToolRegistry()
	tool := &MockTool{name: "test"}

	registry.Register("test", tool)

	args := ToolCall{
		ID:       "test-id",
		Function: "test",
		Arguments: NewToolArgs(map[string]any{
			"arg1": "value1",
		}),
	}

	response := registry.Execute(args)
	assert.Equal(t, "test-id", response.ID)
	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, "test", response.Result.Get("executed").String())
}

func TestToolRegistry_ExecuteNotFound(t *testing.T) {
	registry := NewToolRegistry()

	args := ToolCall{
		ID:        "test-id",
		Function:  "nonexistent",
		Arguments: NewToolArgs(nil),
	}

	response := registry.Execute(args)
	assert.Equal(t, "test-id", response.ID)
	assert.Error(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, 0, response.Result.Len())
	assert.Contains(t, response.Error.Error(), "tool not found: nonexistent")
}

func TestToolRegistry_Info(t *testing.T) {
	registry := NewToolRegistry()
	info := registry.Info()
	assert.Equal(t, "registry", info.Name)
	assert.NotEmpty(t, info.Description)
}

func TestToolRegistry_ListInfo(t *testing.T) {
	registry := NewToolRegistry()
	tool1 := &MockTool{name: "tool1"}
	tool2 := &MockTool{name: "tool2"}

	registry.Register("tool1", tool1)
	registry.Register("tool2", tool2)

	infos := registry.ListInfo()
	assert.Len(t, infos, 2)

	// Collect names from infos
	names := make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.Name
	}
	assert.Contains(t, names, "tool1")
	assert.Contains(t, names, "tool2")

	// Verify each info has a schema
	for _, info := range infos {
		assert.Equal(t, SchemaTypeObject, info.Schema.Type)
		assert.NotEmpty(t, info.Description)
	}
}

func TestRegisterVFSTools(t *testing.T) {
	registry := NewToolRegistry()
	mockVFS := vfs.NewMockVFS()

	RegisterVFSTools(registry, mockVFS)

	// Test that all VFS tools are registered
	vfsTools := []string{"vfs.read", "vfs.write", "vfs.delete", "vfs.list", "vfs.move"}

	for _, toolName := range vfsTools {
		tool, err := registry.Get(toolName)
		require.NoError(t, err, "Tool %s should be registered", toolName)
		assert.NotNil(t, tool, "Tool %s should not be nil", toolName)
	}

	// Test that the tools have the correct names
	readTool, err := registry.Get("vfs.read")
	require.NoError(t, err)
	assert.Equal(t, "vfs.read", readTool.Info().Name)

	writeTool, err := registry.Get("vfs.write")
	require.NoError(t, err)
	assert.Equal(t, "vfs.write", writeTool.Info().Name)

	deleteTool, err := registry.Get("vfs.delete")
	require.NoError(t, err)
	assert.Equal(t, "vfs.delete", deleteTool.Info().Name)

	listTool, err := registry.Get("vfs.list")
	require.NoError(t, err)
	assert.Equal(t, "vfs.list", listTool.Info().Name)

	moveTool, err := registry.Get("vfs.move")
	require.NoError(t, err)
	assert.Equal(t, "vfs.move", moveTool.Info().Name)
}

func TestToolRegistry_VFSIntegration(t *testing.T) {
	registry := NewToolRegistry()
	mockVFS := vfs.NewMockVFS()

	RegisterVFSTools(registry, mockVFS)

	// Test writing a file
	writeArgs := ToolCall{
		ID:       "write-id",
		Function: "vfs.write",
		Arguments: NewToolArgs(map[string]any{
			"path":    "test.txt",
			"content": "Hello, World!",
		}),
	}

	writeResponse := registry.Execute(writeArgs)
	assert.Equal(t, "write-id", writeResponse.ID)
	assert.NoError(t, writeResponse.Error)
	assert.True(t, writeResponse.Done)

	// Test reading the file
	readArgs := ToolCall{
		ID:       "read-id",
		Function: "vfs.read",
		Arguments: NewToolArgs(map[string]any{
			"path": "test.txt",
		}),
	}

	readResponse := registry.Execute(readArgs)
	assert.Equal(t, "read-id", readResponse.ID)
	assert.NoError(t, readResponse.Error)
	assert.True(t, readResponse.Done)
	assert.Equal(t, "Hello, World!", readResponse.Result.Get("content").String())

	// Test listing files
	listArgs := ToolCall{
		ID:       "list-id",
		Function: "vfs.list",
		Arguments: NewToolArgs(map[string]any{
			"path": ".",
		}),
	}

	listResponse := registry.Execute(listArgs)
	assert.Equal(t, "list-id", listResponse.ID)
	assert.NoError(t, listResponse.Error)
	assert.True(t, listResponse.Done)

	filesArr := listResponse.Result.Get("files").Array()
	require.Len(t, filesArr, 1)
	assert.Equal(t, "test.txt", filesArr[0].String())
}
