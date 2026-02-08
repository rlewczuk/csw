package tool

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/lsp"
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

func TestVFSWriteTool(t *testing.T) {
	t.Run("should write file successfully", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSWriteTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsWrite",
			Arguments: NewToolValue(map[string]any{
				"path":    "test.txt",
				"content": "hello world",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify file was written
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("should return error for missing path argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSWriteTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsWrite",
			Arguments: NewToolValue(map[string]any{
				"content": "hello",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should return error for missing content argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSWriteTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsWrite",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})
}

func TestVFSDeleteTool(t *testing.T) {
	t.Run("should delete file successfully", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world"))
		require.NoError(t, err)

		tool := NewVFSDeleteTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsDelete",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify file was deleted
		_, err = mockVFS.ReadFile("test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, vfs.ErrFileNotFound)
	})

	t.Run("should return error for missing path argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSDeleteTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "vfsDelete",
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
		tool := NewVFSDeleteTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsDelete",
			Arguments: NewToolValue(map[string]any{
				"path": "non-existent.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})
}

func TestVFSListTool(t *testing.T) {
	t.Run("should list files successfully", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("content1"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("file2.txt", []byte("content2"))
		require.NoError(t, err)

		tool := NewVFSListTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfs.list",
			Arguments: NewToolValue(map[string]any{
				"path": ".",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.True(t, response.Result.Has("files"))

		// Verify files list - now stored as array
		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 2)
		files := []string{filesArr[0].AsString(), filesArr[1].AsString()}
		assert.Contains(t, files, "file1.txt")
		assert.Contains(t, files, "file2.txt")
	})

	t.Run("should return empty list for empty directory", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSListTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfs.list",
			Arguments: NewToolValue(map[string]any{
				"path": ".",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.True(t, response.Result.Has("files"))

		// Verify empty files list
		filesArr := response.Result.Get("files").Array()
		assert.Len(t, filesArr, 0)
	})

	t.Run("should return error for missing path argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSListTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "vfsList",
			Arguments: NewToolValue(nil),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should return error for non-existent directory", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSListTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsList",
			Arguments: NewToolValue(map[string]any{
				"path": "non-existent",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})
}

func TestVFSMoveTool(t *testing.T) {
	t.Run("should move file successfully", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("source.txt", []byte("hello world"))
		require.NoError(t, err)

		tool := NewVFSMoveTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsMove",
			Arguments: NewToolValue(map[string]any{
				"path":        "source.txt",
				"destination": "dest.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify file was moved
		_, err = mockVFS.ReadFile("source.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, vfs.ErrFileNotFound)

		content, err := mockVFS.ReadFile("dest.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("should return error for missing path argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSMoveTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsMove",
			Arguments: NewToolValue(map[string]any{
				"destination": "dest.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should return error for missing destination argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSMoveTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsMove",
			Arguments: NewToolValue(map[string]any{
				"path": "source.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should return error for non-existent source file", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSMoveTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsMove",
			Arguments: NewToolValue(map[string]any{
				"path":        "non-existent.txt",
				"destination": "dest.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
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

func TestVFSWriteToolPermissionQuery(t *testing.T) {
	t.Run("should return permission query when access is ask", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		privileges := map[string]conf.FileAccess{
			"*.txt": {Write: conf.AccessAsk},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSWriteTool(accessVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsWrite",
			Arguments: NewToolValue(map[string]any{
				"path":    "test.txt",
				"content": "hello",
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
		assert.Equal(t, "vfsWrite", query.Tool.Function)
		assert.Equal(t, "Permission Required", query.Title)
		assert.Contains(t, query.Details, "test.txt")
		assert.True(t, query.AllowCustomResponse)
		assert.Contains(t, query.Options, "Allow")
		assert.Contains(t, query.Options, "Deny")
	})

	t.Run("should succeed when access is allow", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		privileges := map[string]conf.FileAccess{
			"*.txt": {Write: conf.AccessAllow},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSWriteTool(accessVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsWrite",
			Arguments: NewToolValue(map[string]any{
				"path":    "test.txt",
				"content": "hello",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify file was written
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello", string(content))
	})

	t.Run("should fail when access is deny", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		privileges := map[string]conf.FileAccess{
			"*.txt": {Write: conf.AccessDeny},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSWriteTool(accessVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsWrite",
			Arguments: NewToolValue(map[string]any{
				"path":    "test.txt",
				"content": "hello",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, vfs.ErrPermissionDenied)
		assert.True(t, response.Done)
	})
}

func TestVFSDeleteToolPermissionQuery(t *testing.T) {
	t.Run("should return permission query when access is ask", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Delete: conf.AccessAsk},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSDeleteTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsDelete",
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
		assert.Equal(t, "vfsDelete", query.Tool.Function)
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
			"*.txt": {Delete: conf.AccessAllow},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSDeleteTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsDelete",
			Arguments: NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify file was deleted
		_, err = mockVFS.ReadFile("test.txt")
		assert.ErrorIs(t, err, vfs.ErrFileNotFound)
	})

	t.Run("should fail when access is deny", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Delete: conf.AccessDeny},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSDeleteTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsDelete",
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

func TestVFSListToolPermissionQuery(t *testing.T) {
	t.Run("should return permission query when access is ask", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("dir/test.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*": {List: conf.AccessAsk},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSListTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsList",
			Arguments: NewToolValue(map[string]any{
				"path": "dir",
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
		assert.Equal(t, "vfsList", query.Tool.Function)
		assert.Equal(t, "Permission Required", query.Title)
		assert.Contains(t, query.Details, "dir")
		assert.True(t, query.AllowCustomResponse)
		assert.Contains(t, query.Options, "Allow")
		assert.Contains(t, query.Options, "Deny")
	})

	t.Run("should succeed when access is allow", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("hello"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("file2.txt", []byte("world"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*": {List: conf.AccessAllow},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSListTool(accessVFS)

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
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 2)
	})

	t.Run("should fail when access is deny", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		privileges := map[string]conf.FileAccess{
			"*": {List: conf.AccessDeny},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSListTool(accessVFS)

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
		assert.ErrorIs(t, response.Error, vfs.ErrPermissionDenied)
		assert.True(t, response.Done)
	})
}

func TestVFSMoveToolPermissionQuery(t *testing.T) {
	t.Run("should return permission query when access is ask", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("source.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Move: conf.AccessAsk, Write: conf.AccessAllow},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSMoveTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsMove",
			Arguments: NewToolValue(map[string]any{
				"path":        "source.txt",
				"destination": "dest.txt",
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
		assert.Equal(t, "vfsMove", query.Tool.Function)
		assert.Equal(t, "Permission Required", query.Title)
		assert.Contains(t, query.Details, "source.txt")
		assert.True(t, query.AllowCustomResponse)
		assert.Contains(t, query.Options, "Allow")
		assert.Contains(t, query.Options, "Deny")
	})

	t.Run("should request write permission for destination", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("source.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"source.txt": {Move: conf.AccessAllow},
			"dest.txt":   {Write: conf.AccessAsk},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSMoveTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsMove",
			Arguments: NewToolValue(map[string]any{
				"path":        "source.txt",
				"destination": "dest.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)

		query, ok := response.Error.(*ToolPermissionsQuery)
		require.True(t, ok, "Error should be ToolPermissionsQuery")
		assert.Equal(t, "dest.txt", query.Meta["path"])
		assert.Equal(t, "write", query.Meta["operation"])
		assert.Contains(t, query.Details, "writing to file")
		assert.Contains(t, query.Details, "dest.txt")
	})

	t.Run("should succeed when access is allow", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("source.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Move: conf.AccessAllow, Write: conf.AccessAllow},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSMoveTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsMove",
			Arguments: NewToolValue(map[string]any{
				"path":        "source.txt",
				"destination": "dest.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify file was moved
		_, err = mockVFS.ReadFile("source.txt")
		assert.ErrorIs(t, err, vfs.ErrFileNotFound)
		content, err := mockVFS.ReadFile("dest.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello", string(content))
	})

	t.Run("should fail when access is deny", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("source.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Move: conf.AccessDeny, Write: conf.AccessAllow},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSMoveTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsMove",
			Arguments: NewToolValue(map[string]any{
				"path":        "source.txt",
				"destination": "dest.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, vfs.ErrPermissionDenied)
		assert.True(t, response.Done)
	})
}

func TestVFSFindTool(t *testing.T) {
	t.Run("should find files matching pattern non-recursively", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("content1"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("file2.txt", []byte("content2"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("other.go", []byte("content3"))
		require.NoError(t, err)

		tool := NewVFSFindTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query":     "*.txt",
				"recursive": false,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.True(t, response.Result.Has("files"))

		// Verify files list
		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 2)
		files := []string{filesArr[0].AsString(), filesArr[1].AsString()}
		assert.Contains(t, files, "file1.txt")
		assert.Contains(t, files, "file2.txt")
	})

	t.Run("should find files recursively", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("content1"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("dir/file2.txt", []byte("content2"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("dir/subdir/file3.txt", []byte("content3"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("other.go", []byte("content4"))
		require.NoError(t, err)

		tool := NewVFSFindTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query":     "*.txt",
				"recursive": true,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.True(t, response.Result.Has("files"))

		// Verify files list
		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 3)
		files := []string{filesArr[0].AsString(), filesArr[1].AsString(), filesArr[2].AsString()}
		assert.Contains(t, files, "file1.txt")
		assert.Contains(t, files, filepath.Join("dir", "file2.txt"))
		assert.Contains(t, files, filepath.Join("dir", "subdir", "file3.txt"))
	})

	t.Run("should return empty list when no files match", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("content1"))
		require.NoError(t, err)

		tool := NewVFSFindTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query":     "*.go",
				"recursive": false,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.True(t, response.Result.Has("files"))

		// Verify empty files list
		filesArr := response.Result.Get("files").Array()
		assert.Len(t, filesArr, 0)
	})

	t.Run("should use default recursive=false when not provided", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("content1"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("dir/file2.txt", []byte("content2"))
		require.NoError(t, err)

		tool := NewVFSFindTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query": "*.txt",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.True(t, response.Result.Has("files"))

		// Verify only root level files are found
		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 1)
		assert.Equal(t, "file1.txt", filesArr[0].AsString())
	})

	t.Run("should return error for missing query argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSFindTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "vfsFind",
			Arguments: NewToolValue(nil),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should find directories matching pattern", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test_dir/file.txt", []byte("content"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("test_other/file.txt", []byte("content"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("other/file.txt", []byte("content"))
		require.NoError(t, err)

		tool := NewVFSFindTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query":     "test_*",
				"recursive": false,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.True(t, response.Result.Has("files"))

		// Verify directories list
		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 2)
		files := []string{filesArr[0].AsString(), filesArr[1].AsString()}
		assert.Contains(t, files, "test_dir")
		assert.Contains(t, files, "test_other")
	})
}

func TestVFSFindToolPermissionQuery(t *testing.T) {
	t.Run("should return permission query when access is ask", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*": {Find: conf.AccessAsk},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSFindTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query":     "*.txt",
				"recursive": false,
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
		assert.Equal(t, "vfsFind", query.Tool.Function)
		assert.Equal(t, "Permission Required", query.Title)
		assert.Contains(t, query.Details, "*.txt")
		assert.True(t, query.AllowCustomResponse)
		assert.Contains(t, query.Options, "Allow")
		assert.Contains(t, query.Options, "Deny")
	})

	t.Run("should succeed when access is allow", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("hello"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("file2.txt", []byte("world"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*": {Find: conf.AccessAllow},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSFindTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query":     "*.txt",
				"recursive": false,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 2)
	})

	t.Run("should fail when access is deny", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		privileges := map[string]conf.FileAccess{
			"*": {Find: conf.AccessDeny},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSFindTool(accessVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query":     "*.txt",
				"recursive": false,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, vfs.ErrPermissionDenied)
		assert.True(t, response.Done)
	})
}

func TestVFSEditTool(t *testing.T) {
	t.Run("should replace first occurrence by default and return error for multiple matches", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world hello"))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS, nil)

		// Execute - should fail because multiple occurrences without replaceAll
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Assert - should return error for multiple matches
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "oldString found multiple times")
	})

	t.Run("should replace all occurrences when replaceAll is true", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world hello"))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":       "test.txt",
				"oldString":  "hello",
				"newString":  "hi",
				"replaceAll": true,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify all occurrences were replaced
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "hi world hi", string(content))

		// Verify diff was returned
		diff := response.Result.Get("content").AsString()
		assert.Contains(t, diff, "```diff")
		assert.Contains(t, diff, "-hello world hello")
		assert.Contains(t, diff, "+hi world hi")
	})

	t.Run("should return error for missing path argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSEditTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "missing required argument: path")
	})

	t.Run("should return error for missing oldString argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSEditTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "missing required argument: oldString")
	})

	t.Run("should return error for missing newString argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSEditTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "hello",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "missing required argument: newString")
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSEditTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "non-existent.txt",
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
	})

	t.Run("should return error when oldString not found", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world"))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "goodbye",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "oldString not found")
	})

	t.Run("should replace unique occurrence with empty string", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world"))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "hello ",
				"newString": "",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify replacement
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "world", string(content))

		// Verify diff was returned
		diff := response.Result.Get("content").AsString()
		assert.Contains(t, diff, "```diff")
	})

	t.Run("should handle multiline content with replaceAll", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		content := "line1\nline2\nline3\nline1\nline4"
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":       "test.txt",
				"oldString":  "line1",
				"newString":  "first",
				"replaceAll": true,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify all occurrences replaced
		result, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "first\nline2\nline3\nfirst\nline4", string(result))

		// Verify diff was returned
		diff := response.Result.Get("content").AsString()
		assert.Contains(t, diff, "```diff")
	})
}

func TestVFSEditToolPermissionQuery(t *testing.T) {
	t.Run("should return permission query when read access is ask", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAsk},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSEditTool(accessVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "hello",
				"newString": "hi",
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
		assert.Equal(t, "vfsEdit", query.Tool.Function)
		assert.Equal(t, "Permission Required", query.Title)
		assert.Contains(t, query.Details, "test.txt")
		assert.True(t, query.AllowCustomResponse)
		assert.Contains(t, query.Options, "Allow")
		assert.Contains(t, query.Options, "Deny")
	})

	t.Run("should return permission query when write access is ask", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAllow, Write: conf.AccessAsk},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSEditTool(accessVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "hello",
				"newString": "hi",
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
		assert.Equal(t, "vfsEdit", query.Tool.Function)
		assert.Equal(t, "Permission Required", query.Title)
		assert.Contains(t, query.Details, "test.txt")
		assert.True(t, query.AllowCustomResponse)
		assert.Contains(t, query.Options, "Allow")
		assert.Contains(t, query.Options, "Deny")
	})

	t.Run("should succeed when read and write access are allow", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAllow, Write: conf.AccessAllow},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSEditTool(accessVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify file was modified
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "hi world", string(content))

		// Verify diff was returned
		diff := response.Result.Get("content").AsString()
		assert.Contains(t, diff, "```diff")
	})

	t.Run("should fail when read access is deny", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		privileges := map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessDeny},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSEditTool(accessVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, vfs.ErrPermissionDenied)
		assert.True(t, response.Done)
	})

	t.Run("should fail when write access is deny", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAllow, Write: conf.AccessDeny},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSEditTool(accessVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, vfs.ErrPermissionDenied)
		assert.True(t, response.Done)
	})
}

func TestToolPermissionsQueryError(t *testing.T) {
	t.Run("should return correct error message", func(t *testing.T) {
		query := &ToolPermissionsQuery{
			Id: "test-id-123",
			Tool: &ToolCall{
				Function: "vfsRead",
			},
		}
		errMsg := query.Error()
		assert.Contains(t, errMsg, "vfsRead")
		assert.Contains(t, errMsg, "test-id-123")
		assert.Contains(t, errMsg, "permission query")
	})

	t.Run("should handle nil tool", func(t *testing.T) {
		query := &ToolPermissionsQuery{
			Id:   "test-id-456",
			Tool: nil,
		}
		errMsg := query.Error()
		assert.Contains(t, errMsg, "test-id-456")
		assert.Contains(t, errMsg, "permission query")
	})
}

func TestVFSWriteToolWithLSP(t *testing.T) {
	t.Run("should write file and validate with LSP successfully", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		mockLSP, err := lsp.NewMockLSP("/tmp/test")
		require.NoError(t, err)
		err = mockLSP.Init(true)
		require.NoError(t, err)

		tool := NewVFSWriteTool(mockVFS, mockLSP)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsWrite",
			Arguments: NewToolValue(map[string]any{
				"path":    "test.go",
				"content": "package main\n\nfunc main() {}\n",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify file was written
		content, err := mockVFS.ReadFile("test.go")
		require.NoError(t, err)
		assert.Equal(t, "package main\n\nfunc main() {}\n", string(content))
	})

	t.Run("should report LSP diagnostics errors after write", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		mockLSP, err := lsp.NewMockLSP("/tmp/test")
		require.NoError(t, err)
		err = mockLSP.Init(true)
		require.NoError(t, err)

		// Setup mock diagnostics
		absPath, _ := filepath.Abs("test.go")
		uri := "file://" + filepath.ToSlash(absPath)
		mockLSP.SetDiagnostics(uri, []lsp.Diagnostic{
			{
				Range: lsp.Range{
					Start: lsp.Position{Line: 2, Character: 0},
					End:   lsp.Position{Line: 2, Character: 5},
				},
				Severity: lsp.SeverityError,
				Message:  "undefined: invalid",
			},
		})

		tool := NewVFSWriteTool(mockVFS, mockLSP)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsWrite",
			Arguments: NewToolValue(map[string]any{
				"path":    "test.go",
				"content": "package main\n\nfunc main() { invalid }\n",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify validation message contains error
		validationMsg := response.Result.Get("validation").AsString()
		assert.Contains(t, validationMsg, "LSP validation found issues")
		assert.Contains(t, validationMsg, "Error [3:1]")
		assert.Contains(t, validationMsg, "undefined: invalid")
	})

	t.Run("should work without LSP when nil", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSWriteTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsWrite",
			Arguments: NewToolValue(map[string]any{
				"path":    "test.txt",
				"content": "hello world",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify file was written
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})
}

func TestVFSEditToolWithLSP(t *testing.T) {
	t.Run("should edit file and validate with LSP successfully", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.go", []byte("package main\n\nfunc main() {}\n"))
		require.NoError(t, err)

		mockLSP, err := lsp.NewMockLSP("/tmp/test")
		require.NoError(t, err)
		err = mockLSP.Init(true)
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS, mockLSP)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.go",
				"oldString": "func main() {}",
				"newString": "func main() {\n\tfmt.Println(\"hello\")\n}",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify diff was returned
		diff := response.Result.Get("content").AsString()
		assert.Contains(t, diff, "```diff")
	})

	t.Run("should report LSP diagnostics errors after edit", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.go", []byte("package main\n\nfunc main() {}\n"))
		require.NoError(t, err)

		mockLSP, err := lsp.NewMockLSP("/tmp/test")
		require.NoError(t, err)
		err = mockLSP.Init(true)
		require.NoError(t, err)

		// Setup mock diagnostics
		absPath, _ := filepath.Abs("test.go")
		uri := "file://" + filepath.ToSlash(absPath)
		mockLSP.SetDiagnostics(uri, []lsp.Diagnostic{
			{
				Range: lsp.Range{
					Start: lsp.Position{Line: 2, Character: 16},
					End:   lsp.Position{Line: 2, Character: 23},
				},
				Severity: lsp.SeverityError,
				Message:  "undefined: invalid",
			},
		})

		tool := NewVFSEditTool(mockVFS, mockLSP)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.go",
				"oldString": "func main() {}",
				"newString": "func main() { invalid }",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify validation message contains error
		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "LSP validation found issues")
		assert.Contains(t, content, "Error [3:17]")
		assert.Contains(t, content, "undefined: invalid")
	})

	t.Run("should work without LSP when nil", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world"))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS, nil)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify edit was applied
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "hi world", string(content))
	})
}
