package tool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/vfs"
)

// MockTool implements Tool interface for testing.
type MockTool struct {
	name string
}

func (m *MockTool) Execute(args *ToolCall) *ToolResponse {
	var result ToolValue
	result.Set("executed", m.name)
	return &ToolResponse{
		Call:   args,
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

	args := &ToolCall{
		ID:       "test-id",
		Function: "test",
		Arguments: NewToolValue(map[string]any{
			"arg1": "value1",
		}),
	}

	response := registry.Execute(args)
	assert.Equal(t, "test-id", response.Call.ID)
	assert.NoError(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, "test", response.Result.Get("executed").AsString())
}

func TestToolRegistry_ExecuteNotFound(t *testing.T) {
	registry := NewToolRegistry()

	args := &ToolCall{
		ID:        "test-id",
		Function:  "nonexistent",
		Arguments: NewToolValue(nil),
	}

	response := registry.Execute(args)
	assert.Equal(t, "test-id", response.Call.ID)
	assert.Error(t, response.Error)
	assert.True(t, response.Done)
	assert.Equal(t, 0, response.Result.Len())
	assert.Contains(t, response.Error.Error(), "tool not found: nonexistent")
}

func TestToolRegistry_FilterByModelTags(t *testing.T) {
	tests := []struct {
		name      string
		tags      []string
		selection conf.ToolSelectionConfig
		expected  []string
	}{
		{
			name: "default rules without tags",
			tags: nil,
			selection: conf.ToolSelectionConfig{
				Default: map[string]bool{
					"vfsRead": true,
					"runBash": false,
				},
			},
			expected: []string{"vfsRead"},
		},
		{
			name: "tag enables disabled by default tool",
			tags: []string{"bash-enabled"},
			selection: conf.ToolSelectionConfig{
				Default: map[string]bool{
					"vfsRead": true,
					"runBash": false,
				},
				Tags: map[string]conf.ToolTagSelectionRule{
					"bash-enabled": {
						Enable: []string{"runBash"},
					},
				},
			},
			expected: []string{"runBash", "vfsRead"},
		},
		{
			name: "tag disables enabled by default tool",
			tags: []string{"safe"},
			selection: conf.ToolSelectionConfig{
				Default: map[string]bool{
					"vfsRead": true,
					"runBash": true,
				},
				Tags: map[string]conf.ToolTagSelectionRule{
					"safe": {
						Disable: []string{"runBash"},
					},
				},
			},
			expected: []string{"vfsRead"},
		},
		{
			name: "later tags override earlier tags",
			tags: []string{"enable-bash", "disable-bash"},
			selection: conf.ToolSelectionConfig{
				Default: map[string]bool{
					"runBash": false,
					"vfsRead": false,
				},
				Tags: map[string]conf.ToolTagSelectionRule{
					"enable-bash": {
						Enable: []string{"runBash"},
					},
					"disable-bash": {
						Disable: []string{"runBash"},
					},
				},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewToolRegistry()
			registry.Register("vfsRead", &MockTool{name: "read"})
			registry.Register("runBash", &MockTool{name: "bash"})

			filtered := registry.FilterByModelTags(tt.tags, tt.selection)
			require.NotNil(t, filtered)

			names := filtered.List()
			assert.ElementsMatch(t, tt.expected, names)
		})
	}
}

func TestToolRegistry_FilterByModelTags_NilRegistry(t *testing.T) {
	var registry *ToolRegistry

	filtered := registry.FilterByModelTags(nil, conf.ToolSelectionConfig{})
	assert.Nil(t, filtered)
}

func TestRegisterVFSTools(t *testing.T) {
	registry := NewToolRegistry()
	mockVFS := vfs.NewMockVFS()

	RegisterVFSTools(registry, mockVFS, nil, nil)

	// Test that all VFS tools are registered
	vfsTools := []string{"vfsRead", "vfsWrite", "vfsEdit", "vfsPatch", "vfsDelete", "vfsMove", "vfsFind", "vfsGrep"}

	for _, toolName := range vfsTools {
		tool, err := registry.Get(toolName)
		require.NoError(t, err, "Tool %s should be registered", toolName)
		assert.NotNil(t, tool, "Tool %s should not be nil", toolName)
	}
}

func TestToolRegistry_VFSIntegration(t *testing.T) {
	registry := NewToolRegistry()
	mockVFS := vfs.NewMockVFS()

	RegisterVFSTools(registry, mockVFS, nil, nil)

	// Test writing a file
	writeArgs := &ToolCall{
		ID:       "write-id",
		Function: "vfsWrite",
		Arguments: NewToolValue(map[string]any{
			"path":    "test.txt",
			"content": "Hello, World!",
		}),
	}

	writeResponse := registry.Execute(writeArgs)
	assert.Equal(t, "write-id", writeResponse.Call.ID)
	assert.NoError(t, writeResponse.Error)
	assert.True(t, writeResponse.Done)

	// Test reading the file
	readArgs := &ToolCall{
		ID:       "read-id",
		Function: "vfsRead",
		Arguments: NewToolValue(map[string]any{
			"path": "test.txt",
		}),
	}

	readResponse := registry.Execute(readArgs)
	assert.Equal(t, "read-id", readResponse.Call.ID)
	assert.NoError(t, readResponse.Error)
	assert.True(t, readResponse.Done)
	// Line numbers are enabled by default, so expect formatted output
	assert.Equal(t, "    1  Hello, World!", readResponse.Result.Get("content").AsString())

	// Test finding files
	findArgs := &ToolCall{
		ID:       "find-id",
		Function: "vfsFind",
		Arguments: NewToolValue(map[string]any{
			"query": "",
		}),
	}

	findResponse := registry.Execute(findArgs)
	assert.Equal(t, "find-id", findResponse.Call.ID)
	assert.NoError(t, findResponse.Error)
	assert.True(t, findResponse.Done)

	filesArr := findResponse.Result.Get("files").Array()
	require.Len(t, filesArr, 1)
	assert.Equal(t, "test.txt", filesArr[0].AsString())
}

func (m *MockTool) Render(call *ToolCall) (string, string, map[string]string) {
	return m.name, m.name, make(map[string]string)
}
