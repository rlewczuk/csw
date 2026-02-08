package tool

import (
	"strings"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
)

// TestVFSReadTool_Render tests the Render method for VFSReadTool
func TestVFSReadTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSReadTool(mockVFS, true)

	tests := []struct {
		name          string
		args          *ToolCall
		expectedShort string
		expectedFull  string
		maxLen        int
	}{
		{
			name: "basic read",
			args: &ToolCall{
				Function:  "vfsRead",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go"}),
			},
			expectedShort: "vfsRead",
			expectedFull:  "vfsRead",
			maxLen:        150,
		},
		{
			name: "read with long path",
			args: &ToolCall{
				Function:  "vfsRead",
				Arguments: NewToolValue(map[string]any{"path": "/very/long/path/to/the/file/that/might/exceed/the/limit/if/we/make/it/long/enough/and/add/some/more/characters/to/be/sure/it/is/really/long/file.go"}),
			},
			expectedShort: "vfsRead",
			expectedFull:  "vfsRead",
			maxLen:        150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render()
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.Equal(t, tt.expectedShort, summary, "Summary should match expected")
			assert.Equal(t, tt.expectedFull, details, "Details should match expected")
			assert.True(t, strings.HasPrefix(summary, tt.expectedShort), "Summary should start with tool name")
			assert.LessOrEqual(t, len(summary), tt.maxLen, "Summary should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSWriteTool_Render tests the Render method for VFSWriteTool
func TestVFSWriteTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSWriteTool(mockVFS, nil)

	tests := []struct {
		name          string
		args          *ToolCall
		expectedShort string
		expectedFull  string
		maxLen        int
	}{
		{
			name: "basic write",
			args: &ToolCall{
				Function:  "vfsWrite",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go", "content": "hello world"}),
			},
			expectedShort: "vfsWrite",
			expectedFull:  "vfsWrite",
			maxLen:        150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render()
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.Equal(t, tt.expectedShort, summary, "Summary should match expected")
			assert.Equal(t, tt.expectedFull, details, "Details should match expected")
			assert.True(t, strings.HasPrefix(summary, tt.expectedShort), "Summary should start with tool name")
			assert.LessOrEqual(t, len(summary), tt.maxLen, "Summary should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSDeleteTool_Render tests the Render method for VFSDeleteTool
func TestVFSDeleteTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSDeleteTool(mockVFS)

	tests := []struct {
		name          string
		args          *ToolCall
		expectedShort string
		expectedFull  string
		maxLen        int
	}{
		{
			name: "basic delete",
			args: &ToolCall{
				Function:  "vfsDelete",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go"}),
			},
			expectedShort: "vfsDelete",
			expectedFull:  "vfsDelete",
			maxLen:        150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render()
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.Equal(t, tt.expectedShort, summary, "Summary should match expected")
			assert.Equal(t, tt.expectedFull, details, "Details should match expected")
			assert.True(t, strings.HasPrefix(summary, tt.expectedShort), "Summary should start with tool name")
			assert.LessOrEqual(t, len(summary), tt.maxLen, "Summary should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSListTool_Render tests the Render method for VFSListTool
func TestVFSListTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSListTool(mockVFS)

	tests := []struct {
		name          string
		args          *ToolCall
		expectedShort string
		expectedFull  string
		maxLen        int
	}{
		{
			name: "basic list",
			args: &ToolCall{
				Function:  "vfsList",
				Arguments: NewToolValue(map[string]any{"path": "/test"}),
			},
			expectedShort: "vfsList",
			expectedFull:  "vfsList",
			maxLen:        150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render()
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.Equal(t, tt.expectedShort, summary, "Summary should match expected")
			assert.Equal(t, tt.expectedFull, details, "Details should match expected")
			assert.True(t, strings.HasPrefix(summary, tt.expectedShort), "Summary should start with tool name")
			assert.LessOrEqual(t, len(summary), tt.maxLen, "Summary should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSMoveTool_Render tests the Render method for VFSMoveTool
func TestVFSMoveTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSMoveTool(mockVFS)

	tests := []struct {
		name          string
		args          *ToolCall
		expectedShort string
		expectedFull  string
		maxLen        int
	}{
		{
			name: "basic move",
			args: &ToolCall{
				Function:  "vfsMove",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go", "destination": "/test/newfile.go"}),
			},
			expectedShort: "vfsMove",
			expectedFull:  "vfsMove",
			maxLen:        150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render()
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.Equal(t, tt.expectedShort, summary, "Summary should match expected")
			assert.Equal(t, tt.expectedFull, details, "Details should match expected")
			assert.True(t, strings.HasPrefix(summary, tt.expectedShort), "Summary should start with tool name")
			assert.LessOrEqual(t, len(summary), tt.maxLen, "Summary should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSFindTool_Render tests the Render method for VFSFindTool
func TestVFSFindTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSFindTool(mockVFS)

	tests := []struct {
		name          string
		args          *ToolCall
		expectedShort string
		expectedFull  string
		maxLen        int
	}{
		{
			name: "basic find",
			args: &ToolCall{
				Function:  "vfsFind",
				Arguments: NewToolValue(map[string]any{"query": "*.go"}),
			},
			expectedShort: "vfsFind",
			expectedFull:  "vfsFind",
			maxLen:        150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render()
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.Equal(t, tt.expectedShort, summary, "Summary should match expected")
			assert.Equal(t, tt.expectedFull, details, "Details should match expected")
			assert.True(t, strings.HasPrefix(summary, tt.expectedShort), "Summary should start with tool name")
			assert.LessOrEqual(t, len(summary), tt.maxLen, "Summary should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSEditTool_Render tests the Render method for VFSEditTool
func TestVFSEditTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSEditTool(mockVFS, nil)

	tests := []struct {
		name          string
		args          *ToolCall
		expectedShort string
		expectedFull  string
		maxLen        int
	}{
		{
			name: "basic edit",
			args: &ToolCall{
				Function:  "vfsEdit",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go", "oldString": "foo", "newString": "bar"}),
			},
			expectedShort: "vfsEdit",
			expectedFull:  "vfsEdit",
			maxLen:        150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render()
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.Equal(t, tt.expectedShort, summary, "Summary should match expected")
			assert.Equal(t, tt.expectedFull, details, "Details should match expected")
			assert.True(t, strings.HasPrefix(summary, tt.expectedShort), "Summary should start with tool name")
			assert.LessOrEqual(t, len(summary), tt.maxLen, "Summary should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSGrepTool_Render tests the Render method for VFSGrepTool
func TestVFSGrepTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSGrepTool(mockVFS)

	tests := []struct {
		name          string
		args          *ToolCall
		expectedShort string
		expectedFull  string
		maxLen        int
	}{
		{
			name: "basic grep",
			args: &ToolCall{
				Function:  "vfsGrep",
				Arguments: NewToolValue(map[string]any{"pattern": "func.*"}),
			},
			expectedShort: "vfsGrep",
			expectedFull:  "vfsGrep",
			maxLen:        150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render()
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.Equal(t, tt.expectedShort, summary, "Summary should match expected")
			assert.Equal(t, tt.expectedFull, details, "Details should match expected")
			assert.True(t, strings.HasPrefix(summary, tt.expectedShort), "Summary should start with tool name")
			assert.LessOrEqual(t, len(summary), tt.maxLen, "Summary should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestAccessControlTool_Render tests the Render method for AccessControlTool
func TestAccessControlTool_Render(t *testing.T) {
	mockTool := &mockTool{name: "testTool"}
	acTool := NewAccessControlTool(mockTool, map[string]conf.AccessFlag{})

	summary, details, meta := acTool.Render()
	assert.NotEmpty(t, summary, "Summary should not be empty")
	assert.Equal(t, "AccessControl", summary, "Summary should be 'AccessControl'")
	assert.Equal(t, "AccessControl", details, "Details should be 'AccessControl'")
	assert.LessOrEqual(t, len(summary), 150, "Summary should not exceed 150 characters")
	assert.NotNil(t, meta, "Meta should not be nil")
}

// TestToolRegistry_Render tests the Render method for ToolRegistry
func TestToolRegistry_Render(t *testing.T) {
	registry := NewToolRegistry()

	summary, details, meta := registry.Render()
	assert.NotEmpty(t, summary, "Summary should not be empty")
	assert.Equal(t, "ToolRegistry", summary, "Summary should be 'ToolRegistry'")
	assert.Equal(t, "ToolRegistry", details, "Details should be 'ToolRegistry'")
	assert.LessOrEqual(t, len(summary), 150, "Summary should not exceed 150 characters")
	assert.NotNil(t, meta, "Meta should not be nil")
}

// TestTruncateString tests the truncateString helper function
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "no truncation needed",
			input:    "short string",
			maxLen:   50,
			expected: "short string",
		},
		{
			name:     "exactly at limit",
			input:    "1234567890",
			maxLen:   10,
			expected: "1234567890",
		},
		{
			name:     "needs truncation",
			input:    "this is a very long string that needs to be truncated",
			maxLen:   20,
			expected: "this is a very lo...",
		},
		{
			name:     "maxLen less than ellipsis",
			input:    "12345",
			maxLen:   2,
			expected: "12",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}
