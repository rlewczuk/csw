package tool

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
)

func TestVFSGrepToolRender(t *testing.T) {
	t.Run("should display relative path when absolute path is provided", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - absolute path under worktree root
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"path":    "/path/to/worktree/src",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - path should be relative to worktree root
		assert.Contains(t, oneLiner, "grep hello in src")
		assert.Contains(t, full, "grep hello in src")
		assert.NotContains(t, oneLiner, "/path/to/worktree")
	})

	t.Run("should keep relative path as is", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - relative path
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"path":    "src/components",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - path should remain unchanged
		assert.Contains(t, oneLiner, "grep hello in src/components")
		assert.Contains(t, full, "grep hello in src/components")
	})

	t.Run("should handle empty path", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - no path
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - should not include "in" phrase
		assert.Equal(t, "grep hello", oneLiner)
		assert.Equal(t, "grep hello\n\n", full)
	})

	t.Run("should handle absolute path outside worktree", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - absolute path outside worktree (cannot be made relative)
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"path":    "/some/other/path",
			}),
		}

		oneLiner, _, _, _ := tool.Render(call)

		// Assert - should keep original path since it cannot be made relative
		// filepath.Rel will return an error for paths on different drives/volumes
		// or when the path is not under the worktree root
		// In such cases, the original path is kept
		assert.Contains(t, oneLiner, "grep hello in")
	})

	t.Run("should handle pattern without path", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - only pattern
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "func main",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert
		assert.Equal(t, "grep func main", oneLiner)
		assert.Equal(t, "grep func main\n\n", full)
	})

	t.Run("should truncate long patterns", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - very long pattern
		longPattern := "this is a very long pattern that should be truncated because it exceeds the limit"
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": longPattern,
			}),
		}

		oneLiner, _, _, _ := tool.Render(call)

		// Assert - should be truncated to 128 chars
		assert.LessOrEqual(t, len(oneLiner), 128)
		assert.Contains(t, oneLiner, "grep")
	})

	t.Run("should include error in output when error is present", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - call with error
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"error":   "VFSGrepTool.Execute() [grep.go]: invalid regex pattern",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - oneliner should contain error on second line
		assert.Contains(t, oneLiner, "grep hello")
		assert.Contains(t, oneLiner, "invalid regex pattern")
		// Assert - full should contain ERROR: prefix
		assert.Contains(t, full, "grep hello")
		assert.Contains(t, full, "ERROR: VFSGrepTool.Execute() [grep.go]: invalid regex pattern")
	})

	t.Run("should convert multiline error to single line in oneliner", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - call with multiline error
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "test",
				"error":   "error on line 1\nerror on line 2\nerror on line 3",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - oneliner should have error on single line (newlines converted to spaces)
		assert.Contains(t, oneLiner, "grep test")
		// Check that oneliner does not contain literal newlines in error portion
		lines := splitLines(oneLiner)
		assert.Equal(t, 2, len(lines), "oneliner should have exactly 2 lines")
		assert.NotContains(t, lines[1], "\n")
		// Assert - full should contain full error with ERROR: prefix
		assert.Contains(t, full, "ERROR: error on line 1\nerror on line 2\nerror on line 3")
	})

	t.Run("should include result count in oneliner and full output", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - call with content
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"content": "file1.txt:1\nfile1.txt:3\nfile2.txt:1",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - should include result count
		assert.Equal(t, "grep hello (3 results)", oneLiner)
		assert.Contains(t, full, "grep hello (3 results)\n\n")
		assert.Contains(t, full, "file1.txt:1")
	})

	t.Run("should show single result with singular form", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - call with single result
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"content": "file1.txt:5",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - should use singular form "1 result"
		assert.Equal(t, "grep hello (1 result)", oneLiner)
		assert.Contains(t, full, "grep hello (1 result)\n\n")
	})

	t.Run("should not show result count when no results", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - call with no results
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"content": "No files found",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - should not include result count
		assert.Equal(t, "grep hello", oneLiner)
		assert.Equal(t, "grep hello\n\nNo files found", full)
	})

	t.Run("should include result count with path in oneliner", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - call with path and content
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"path":    "src",
				"content": "src/file1.txt:1\nsrc/file2.txt:2",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - should include result count with path
		assert.Equal(t, "grep hello in src (2 results)", oneLiner)
		assert.Contains(t, full, "grep hello in src (2 results)\n\n")
	})

	t.Run("should count only grep rows when too many-results suffix is present", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"content": "a.txt:1\na.txt:2\n...\n(too many results, please narrow search query and try again)",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		assert.Equal(t, "grep hello (2 results)", oneLiner)
		assert.Contains(t, full, "(too many results, please narrow search query and try again)")
	})
}
