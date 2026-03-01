package tool

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVFSReadTool(t *testing.T) {
	t.Run("should read file successfully", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world"))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "hello world", response.Result.Get("content").AsString())
	})

	t.Run("should return error for missing path argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "vfsRead",
			Arguments: NewToolValue(nil),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "nonexistent.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should apply limit parameter", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		content := "line1\nline2\nline3\nline4\nline5"
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path":  "test.txt",
				"limit": 3,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "line1\nline2\nline3\n", response.Result.Get("content").AsString())
	})

	t.Run("should apply offset parameter", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		content := "line1\nline2\nline3\nline4\nline5"
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path":   "test.txt",
				"offset": 2,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "line3\nline4\nline5", response.Result.Get("content").AsString())
	})

	t.Run("should apply both offset and limit parameters", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		content := "line1\nline2\nline3\nline4\nline5\nline6"
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path":   "test.txt",
				"offset": 1,
				"limit":  3,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "line2\nline3\nline4\n", response.Result.Get("content").AsString())
	})

	t.Run("should return empty string when offset exceeds line count", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		content := "line1\nline2\nline3"
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path":   "test.txt",
				"offset": 10,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "", response.Result.Get("content").AsString())
	})

	t.Run("should handle file without trailing newline", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		content := "line1\nline2\nline3"
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path":  "test.txt",
				"limit": 2,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "line1\nline2\n", response.Result.Get("content").AsString())
	})

	t.Run("should use default limit of 2000 when not provided", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		// Create content with more than 2000 lines
		var lines []string
		for i := 1; i <= 2500; i++ {
			lines = append(lines, fmt.Sprintf("line%d", i))
		}
		content := ""
		for _, line := range lines {
			content += line + "\n"
		}
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Count lines in result
		resultContent := response.Result.Get("content").AsString()
		resultLines := 0
		for i := 0; i < len(resultContent); i++ {
			if resultContent[i] == '\n' {
				resultLines++
			}
		}
		assert.Equal(t, 2000, resultLines)
	})

	t.Run("should handle empty file", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte(""))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path":   "test.txt",
				"offset": 0,
				"limit":  10,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "", response.Result.Get("content").AsString())
	})

	t.Run("should format content with line numbers when enabled", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		content := "line1\nline2\nline3"
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, true)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Expected format: "    1  line1\n    2  line2\n    3  line3"
		expected := "    1  line1\n    2  line2\n    3  line3"
		assert.Equal(t, expected, response.Result.Get("content").AsString())
	})

	t.Run("should format content with line numbers and offset", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		content := "line1\nline2\nline3\nline4\nline5"
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, true)

		// Execute with offset
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path":   "test.txt",
				"offset": 2,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// With offset=2, line numbers should start at 3 (offset + 1)
		expected := "    3  line3\n    4  line4\n    5  line5"
		assert.Equal(t, expected, response.Result.Get("content").AsString())
	})

	t.Run("should not format content with line numbers when disabled", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		content := "line1\nline2\nline3"
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Without line numbers, content should be unchanged
		assert.Equal(t, content, response.Result.Get("content").AsString())
	})
}

func TestVFSReadToolPermissionQuery(t *testing.T) {
	t.Run("should return permission query when access is ask", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		privileges := map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAsk},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSReadTool(accessVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)

		// Check that error is ToolPermissionsQuery
		query, ok := response.Error.(*ToolPermissionsQuery)
		require.True(t, ok, "Error should be ToolPermissionsQuery")
		assert.NotEmpty(t, query.Id)
		assert.Equal(t, "vfsRead", query.Tool.Function)
		assert.Equal(t, "Permission Required", query.Title)
		assert.Contains(t, query.Details, "test.txt")
		assert.True(t, query.AllowCustomResponse)
		assert.Contains(t, query.Options, "Allow")
		assert.Contains(t, query.Options, "Deny")
	})

	t.Run("should succeed when access is allow", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAllow},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSReadTool(accessVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "hello", response.Result.Get("content").AsString())
	})

	t.Run("should fail when access is deny", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		privileges := map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessDeny},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSReadTool(accessVFS, false)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, vfs.ErrPermissionDenied)
		assert.True(t, response.Done)
	})
}

func TestVFSReadToolRender(t *testing.T) {
	t.Run("should render with relative path for absolute path within worktree", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS, false)

		// Execute - use an absolute path that's within the mock worktree (/path/to/worktree)
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "/path/to/worktree/cmd/csw/main.go",
			}),
		}
		oneLiner, full, _ := tool.Render(call)

		// Assert - path should be relative to worktree with line count
		assert.Equal(t, "read cmd/csw/main.go (0 lines)", oneLiner)
		assert.Contains(t, full, "read cmd/csw/main.go (0 lines)")
	})

	t.Run("should render with original path for relative path", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS, false)

		// Execute
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "cmd/csw/main.go",
			}),
		}
		oneLiner, _, _ := tool.Render(call)

		// Assert
		assert.Equal(t, "read cmd/csw/main.go (0 lines)", oneLiner)
	})

	t.Run("should render with original path for path outside worktree", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS, false)

		// Execute - use a path outside the worktree
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "/other/path/file.go",
			}),
		}
		oneLiner, _, _ := tool.Render(call)

		// Assert - path should remain as-is since it's outside worktree with line count
		assert.Equal(t, "read /other/path/file.go (0 lines)", oneLiner)
	})

	t.Run("should render error in oneLiner and full when error is present", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS, false)

		// Execute - simulate error by including error in arguments
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path":  "cmd/csw/main.go",
				"error": "failed to read file: permission denied",
			}),
		}
		oneLiner, full, _ := tool.Render(call)

		// Assert - oneLiner should have error as second line
		assert.Contains(t, oneLiner, "read cmd/csw/main.go")
		assert.Contains(t, oneLiner, "failed to read file: permission denied")
		// Assert - full should have ERROR: prefix
		assert.Contains(t, full, "ERROR: failed to read file: permission denied")
	})

	t.Run("should convert multiline error to single line in oneLiner", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS, false)

		// Execute - simulate multiline error
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path":  "cmd/csw/main.go",
				"error": "error line 1\nerror line 2\nerror line 3",
			}),
		}
		oneLiner, full, _ := tool.Render(call)

		// Assert - oneLiner should have error on single line
		lines := strings.Split(oneLiner, "\n")
		assert.Len(t, lines, 2) // First line is operation, second is error
		assert.Contains(t, lines[1], "error line 1")
		assert.Contains(t, lines[1], "error line 2")
		assert.Contains(t, lines[1], "error line 3")
		// Assert - full should preserve original multiline with ERROR: prefix
		assert.Contains(t, full, "ERROR: error line 1")
	})

	t.Run("should show correct line count when content is provided in result", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS, false)

		// First execute the tool to get actual content
		err := mockVFS.WriteFile("test.txt", []byte("line1\nline2\nline3"))
		require.NoError(t, err)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})
		require.NoError(t, response.Error)

		// Simulate what presenter should do: merge result into call arguments for rendering
		// The bug is that the presenter doesn't do this, so Render shows "0 lines"
		callWithResult := &ToolCall{
			ID:       response.Call.ID,
			Function: response.Call.Function,
			Arguments: NewToolValue(map[string]any{
				"path":    "test.txt",
				"content": response.Result.Get("content").AsString(),
			}),
		}
		oneLiner, full, _ := tool.Render(callWithResult)

		// Assert - should show correct line count (3 lines)
		assert.Contains(t, oneLiner, "read test.txt (3 lines)")
		assert.Contains(t, full, "line1")
		assert.Contains(t, full, "line2")
		assert.Contains(t, full, "line3")
	})

	t.Run("should show 0 lines when content is not provided (bug case)", func(t *testing.T) {
		// This test exposes the bug: when content is not in arguments,
		// Render shows 0 lines even though Execute returned content.
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS, false)

		// First execute the tool to get actual content
		err := mockVFS.WriteFile("test.txt", []byte("line1\nline2\nline3"))
		require.NoError(t, err)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})
		require.NoError(t, response.Error)

		// Call Render with original call (without result merged in)
		// This simulates what currently happens in the presenter
		oneLiner, _, _ := tool.Render(response.Call)

		// BUG: This shows "0 lines" because content is not in Arguments
		// After fix, this should still show "0 lines" since we're testing
		// the current broken behavior to document the issue
		assert.Contains(t, oneLiner, "read test.txt (0 lines)")
	})
}
