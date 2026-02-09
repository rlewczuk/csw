package tool

import (
	"fmt"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
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
			Function: "vfsList",
			Arguments: NewToolValue(map[string]any{
				"path": ".",
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
