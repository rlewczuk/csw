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
		name     string
		args     *ToolCall
		wantPath string
	}{
		{
			name: "basic read",
			args: &ToolCall{
				Function:  "vfsRead",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go"}),
			},
			wantPath: "/test/file.go",
		},
		{
			name: "read with long path",
			args: &ToolCall{
				Function:  "vfsRead",
				Arguments: NewToolValue(map[string]any{"path": "/very/long/path/to/the/file/that/might/exceed/the/limit/if/we/make/it/long/enough/and/add/some/more/characters/to/be/sure/it/is/really/long/file.go"}),
			},
			wantPath: "...", // Path will be truncated in summary
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.True(t, strings.HasPrefix(summary, "VFS: read "), "Summary should start with 'VFS: read '")
			assert.Contains(t, summary, tt.wantPath, "Summary should contain path")
			assert.LessOrEqual(t, len(summary), 128, "Summary should not exceed 128 characters")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Contains(t, details, summary, "Details should start with summary")
		})
	}
}

// TestVFSWriteTool_Render tests the Render method for VFSWriteTool
func TestVFSWriteTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSWriteTool(mockVFS, nil)

	tests := []struct {
		name     string
		args     *ToolCall
		wantPath string
		wantContent string
	}{
		{
			name: "basic write",
			args: &ToolCall{
				Function:  "vfsWrite",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go", "content": "hello world"}),
			},
			wantPath:    "/test/file.go",
			wantContent: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.True(t, strings.HasPrefix(summary, "VFS: write "), "Summary should start with 'VFS: write '")
			assert.Contains(t, summary, tt.wantPath, "Summary should contain path")
			assert.LessOrEqual(t, len(summary), 128, "Summary should not exceed 128 characters")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Contains(t, details, summary, "Details should start with summary")
			assert.Contains(t, details, tt.wantContent, "Details should contain content")
		})
	}
}

// TestVFSDeleteTool_Render tests the Render method for VFSDeleteTool
func TestVFSDeleteTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSDeleteTool(mockVFS)

	tests := []struct {
		name     string
		args     *ToolCall
		wantPath string
	}{
		{
			name: "basic delete",
			args: &ToolCall{
				Function:  "vfsDelete",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go"}),
			},
			wantPath: "/test/file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.True(t, strings.HasPrefix(summary, "VFS: delete "), "Summary should start with 'VFS: delete '")
			assert.Contains(t, summary, tt.wantPath, "Summary should contain path")
			assert.LessOrEqual(t, len(summary), 128, "Summary should not exceed 128 characters")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Equal(t, summary, details, "Details should be same as summary for delete")
		})
	}
}

// TestVFSListTool_Render tests the Render method for VFSListTool
func TestVFSListTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSListTool(mockVFS)

	tests := []struct {
		name     string
		args     *ToolCall
		wantPath string
	}{
		{
			name: "basic list",
			args: &ToolCall{
				Function:  "vfsList",
				Arguments: NewToolValue(map[string]any{"path": "/test"}),
			},
			wantPath: "/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.True(t, strings.HasPrefix(summary, "VFS: list "), "Summary should start with 'VFS: list '")
			assert.Contains(t, summary, tt.wantPath, "Summary should contain path")
			assert.LessOrEqual(t, len(summary), 128, "Summary should not exceed 128 characters")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Contains(t, details, summary, "Details should start with summary")
		})
	}
}

// TestVFSMoveTool_Render tests the Render method for VFSMoveTool
func TestVFSMoveTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSMoveTool(mockVFS)

	tests := []struct {
		name            string
		args            *ToolCall
		wantPath        string
		wantDestination string
	}{
		{
			name: "basic move",
			args: &ToolCall{
				Function:  "vfsMove",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go", "destination": "/test/newfile.go"}),
			},
			wantPath:        "/test/file.go",
			wantDestination: "/test/newfile.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.True(t, strings.HasPrefix(summary, "VFS: move "), "Summary should start with 'VFS: move '")
			assert.Contains(t, summary, tt.wantPath, "Summary should contain path")
			assert.Contains(t, summary, tt.wantDestination, "Summary should contain destination")
			assert.LessOrEqual(t, len(summary), 128, "Summary should not exceed 128 characters")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Equal(t, summary, details, "Details should be same as summary for move")
		})
	}
}

// TestVFSFindTool_Render tests the Render method for VFSFindTool
func TestVFSFindTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSFindTool(mockVFS)

	tests := []struct {
		name      string
		args      *ToolCall
		wantQuery string
	}{
		{
			name: "basic find",
			args: &ToolCall{
				Function:  "vfsFind",
				Arguments: NewToolValue(map[string]any{"query": "*.go"}),
			},
			wantQuery: "*.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.True(t, strings.HasPrefix(summary, "VFS: find "), "Summary should start with 'VFS: find '")
			assert.Contains(t, summary, tt.wantQuery, "Summary should contain query")
			assert.LessOrEqual(t, len(summary), 128, "Summary should not exceed 128 characters")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Contains(t, details, summary, "Details should start with summary")
		})
	}
}

// TestVFSEditTool_Render tests the Render method for VFSEditTool
func TestVFSEditTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSEditTool(mockVFS, nil)

	tests := []struct {
		name        string
		args        *ToolCall
		wantPath    string
		wantOldStr  string
		wantNewStr  string
	}{
		{
			name: "basic edit",
			args: &ToolCall{
				Function:  "vfsEdit",
				Arguments: NewToolValue(map[string]any{"path": "/test/file.go", "oldString": "foo", "newString": "bar"}),
			},
			wantPath:   "/test/file.go",
			wantOldStr: "foo",
			wantNewStr: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.True(t, strings.HasPrefix(summary, "VFS: edit "), "Summary should start with 'VFS: edit '")
			assert.Contains(t, summary, tt.wantPath, "Summary should contain path")
			assert.LessOrEqual(t, len(summary), 128, "Summary should not exceed 128 characters")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Contains(t, details, summary, "Details should start with summary")
			// Check for unified diff format
			assert.Contains(t, details, "--- "+tt.wantPath, "Details should contain old file marker")
			assert.Contains(t, details, "+++ "+tt.wantPath, "Details should contain new file marker")
			assert.Contains(t, details, "-"+tt.wantOldStr, "Details should contain old string with -")
			assert.Contains(t, details, "+"+tt.wantNewStr, "Details should contain new string with +")
		})
	}
}

// TestVFSGrepTool_Render tests the Render method for VFSGrepTool
func TestVFSGrepTool_Render(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	tool := NewVFSGrepTool(mockVFS)

	tests := []struct {
		name         string
		args         *ToolCall
		wantPattern  string
		wantPath     string
	}{
		{
			name: "basic grep without path",
			args: &ToolCall{
				Function:  "vfsGrep",
				Arguments: NewToolValue(map[string]any{"pattern": "func.*"}),
			},
			wantPattern: "func.*",
			wantPath:    "",
		},
		{
			name: "grep with path",
			args: &ToolCall{
				Function:  "vfsGrep",
				Arguments: NewToolValue(map[string]any{"pattern": "hello", "path": "/test"}),
			},
			wantPattern: "hello",
			wantPath:    "/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, meta := tool.Render(tt.args)
			assert.NotEmpty(t, summary, "Summary should not be empty")
			assert.True(t, strings.HasPrefix(summary, "VFS: grep "), "Summary should start with 'VFS: grep '")
			assert.Contains(t, summary, tt.wantPattern, "Summary should contain pattern")
			if tt.wantPath != "" {
				assert.Contains(t, summary, tt.wantPath, "Summary should contain path")
			}
			assert.LessOrEqual(t, len(summary), 128, "Summary should not exceed 128 characters")
			assert.NotNil(t, meta, "Meta should not be nil")
			assert.Contains(t, details, summary, "Details should start with summary")
		})
	}
}

// TestAccessControlTool_Render tests the Render method for AccessControlTool
func TestAccessControlTool_Render(t *testing.T) {
	mockTool := &mockTool{name: "testTool"}
	acTool := NewAccessControlTool(mockTool, map[string]conf.AccessFlag{})

	summary, details, meta := acTool.Render(&ToolCall{})
	assert.NotEmpty(t, summary, "Summary should not be empty")
	assert.Equal(t, "AccessControl", summary, "Summary should be 'AccessControl'")
	assert.Equal(t, "AccessControl", details, "Details should be 'AccessControl'")
	assert.LessOrEqual(t, len(summary), 150, "Summary should not exceed 150 characters")
	assert.NotNil(t, meta, "Meta should not be nil")
}

// TestToolRegistry_Render tests the Render method for ToolRegistry
func TestToolRegistry_Render(t *testing.T) {
	registry := NewToolRegistry()

	summary, details, meta := registry.Render(&ToolCall{})
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
