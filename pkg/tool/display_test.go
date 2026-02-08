package tool

import (
	"strings"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
)

// TestVFSReadTool_Display tests the Display method for VFSReadTool
func TestVFSReadTool_Display(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSReadTool(mockVFS, true)

	tests := []struct {
		name     string
		args     *ToolCall
		expected string
		maxLen   int
	}{
		{
			name: "basic read",
			args: &ToolCall{
				Function:  "vfsRead",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go"}),
			},
			expected: "vfsRead",
			maxLen:   150,
		},
		{
			name: "read with long path",
			args: &ToolCall{
				Function:  "vfsRead",
				Arguments: NewToolValue(map[string]any{"path": "/very/long/path/to/the/file/that/might/exceed/the/limit/if/we/make/it/long/enough/and/add/some/more/characters/to/be/sure/it/is/really/long/file.go"}),
			},
			expected: "vfsRead",
			maxLen:   150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display, meta := tool.Display(DisplayModeOneliner, false)
			assert.NotEmpty(t, display, "Display should not be empty")
			assert.True(t, strings.HasPrefix(display, tt.expected), "Display should start with tool name")
			assert.LessOrEqual(t, len(display), tt.maxLen, "Display should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSWriteTool_Display tests the Display method for VFSWriteTool
func TestVFSWriteTool_Display(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSWriteTool(mockVFS, nil)

	tests := []struct {
		name     string
		args     *ToolCall
		expected string
		maxLen   int
	}{
		{
			name: "basic write",
			args: &ToolCall{
				Function:  "vfsWrite",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go", "content": "hello world"}),
			},
			expected: "vfsWrite",
			maxLen:   150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display, meta := tool.Display(DisplayModeOneliner, false)
			assert.NotEmpty(t, display, "Display should not be empty")
			assert.True(t, strings.HasPrefix(display, tt.expected), "Display should start with tool name")
			assert.LessOrEqual(t, len(display), tt.maxLen, "Display should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSDeleteTool_Display tests the Display method for VFSDeleteTool
func TestVFSDeleteTool_Display(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSDeleteTool(mockVFS)

	tests := []struct {
		name     string
		args     *ToolCall
		expected string
		maxLen   int
	}{
		{
			name: "basic delete",
			args: &ToolCall{
				Function:  "vfsDelete",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go"}),
			},
			expected: "vfsDelete",
			maxLen:   150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display, meta := tool.Display(DisplayModeOneliner, false)
			assert.NotEmpty(t, display, "Display should not be empty")
			assert.True(t, strings.HasPrefix(display, tt.expected), "Display should start with tool name")
			assert.LessOrEqual(t, len(display), tt.maxLen, "Display should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSListTool_Display tests the Display method for VFSListTool
func TestVFSListTool_Display(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSListTool(mockVFS)

	tests := []struct {
		name     string
		args     *ToolCall
		expected string
		maxLen   int
	}{
		{
			name: "basic list",
			args: &ToolCall{
				Function:  "vfsList",
				Arguments: NewToolValue(map[string]any{"path": "/test"}),
			},
			expected: "vfsList",
			maxLen:   150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display, meta := tool.Display(DisplayModeOneliner, false)
			assert.NotEmpty(t, display, "Display should not be empty")
			assert.True(t, strings.HasPrefix(display, tt.expected), "Display should start with tool name")
			assert.LessOrEqual(t, len(display), tt.maxLen, "Display should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSMoveTool_Display tests the Display method for VFSMoveTool
func TestVFSMoveTool_Display(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSMoveTool(mockVFS)

	tests := []struct {
		name     string
		args     *ToolCall
		expected string
		maxLen   int
	}{
		{
			name: "basic move",
			args: &ToolCall{
				Function:  "vfsMove",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go", "destination": "/test/newfile.go"}),
			},
			expected: "vfsMove",
			maxLen:   150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display, meta := tool.Display(DisplayModeOneliner, false)
			assert.NotEmpty(t, display, "Display should not be empty")
			assert.True(t, strings.HasPrefix(display, tt.expected), "Display should start with tool name")
			assert.LessOrEqual(t, len(display), tt.maxLen, "Display should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSFindTool_Display tests the Display method for VFSFindTool
func TestVFSFindTool_Display(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSFindTool(mockVFS)

	tests := []struct {
		name     string
		args     *ToolCall
		expected string
		maxLen   int
	}{
		{
			name: "basic find",
			args: &ToolCall{
				Function:  "vfsFind",
				Arguments: NewToolValue(map[string]any{"query": "*.go"}),
			},
			expected: "vfsFind",
			maxLen:   150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display, meta := tool.Display(DisplayModeOneliner, false)
			assert.NotEmpty(t, display, "Display should not be empty")
			assert.True(t, strings.HasPrefix(display, tt.expected), "Display should start with tool name")
			assert.LessOrEqual(t, len(display), tt.maxLen, "Display should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSEditTool_Display tests the Display method for VFSEditTool
func TestVFSEditTool_Display(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSEditTool(mockVFS, nil)

	tests := []struct {
		name     string
		args     *ToolCall
		expected string
		maxLen   int
	}{
		{
			name: "basic edit",
			args: &ToolCall{
				Function:  "vfsEdit",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go", "oldString": "foo", "newString": "bar"}),
			},
			expected: "vfsEdit",
			maxLen:   150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display, meta := tool.Display(DisplayModeOneliner, false)
			assert.NotEmpty(t, display, "Display should not be empty")
			assert.True(t, strings.HasPrefix(display, tt.expected), "Display should start with tool name")
			assert.LessOrEqual(t, len(display), tt.maxLen, "Display should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestVFSGrepTool_Display tests the Display method for VFSGrepTool
func TestVFSGrepTool_Display(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSGrepTool(mockVFS)

	tests := []struct {
		name     string
		args     *ToolCall
		expected string
		maxLen   int
	}{
		{
			name: "basic grep",
			args: &ToolCall{
				Function:  "vfsGrep",
				Arguments: NewToolValue(map[string]any{"pattern": "func.*"}),
			},
			expected: "vfsGrep",
			maxLen:   150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display, meta := tool.Display(DisplayModeOneliner, false)
			assert.NotEmpty(t, display, "Display should not be empty")
			assert.True(t, strings.HasPrefix(display, tt.expected), "Display should start with tool name")
			assert.LessOrEqual(t, len(display), tt.maxLen, "Display should not exceed %d characters", tt.maxLen)
			assert.NotNil(t, meta, "Meta should not be nil")
		})
	}
}

// TestAccessControlTool_Display tests the Display method for AccessControlTool
func TestAccessControlTool_Display(t *testing.T) {
	mockTool := &mockTool{name: "testTool"}
	acTool := NewAccessControlTool(mockTool, map[string]conf.AccessFlag{})

	display, meta := acTool.Display(DisplayModeOneliner, false)
	assert.NotEmpty(t, display, "Display should not be empty")
	assert.Equal(t, "AccessControl", display, "Display should be 'AccessControl'")
	assert.LessOrEqual(t, len(display), 150, "Display should not exceed 150 characters")
	assert.NotNil(t, meta, "Meta should not be nil")
}

// TestToolRegistry_Display tests the Display method for ToolRegistry
func TestToolRegistry_Display(t *testing.T) {
	registry := NewToolRegistry()

	display, meta := registry.Display(DisplayModeOneliner, false)
	assert.NotEmpty(t, display, "Display should not be empty")
	assert.Equal(t, "ToolRegistry", display, "Display should be 'ToolRegistry'")
	assert.LessOrEqual(t, len(display), 150, "Display should not exceed 150 characters")
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
