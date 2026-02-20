package tool

import (
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	t.Run("should use default recursive=true when not provided", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("content1"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("dir/file2.txt", []byte("content2"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("dir/subdir/file3.txt", []byte("content3"))
		require.NoError(t, err)

		tool := NewVFSFindTool(mockVFS)

		// Execute without recursive parameter
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

		// Verify all files are found (recursive by default)
		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 3)
		files := []string{filesArr[0].AsString(), filesArr[1].AsString(), filesArr[2].AsString()}
		assert.Contains(t, files, "file1.txt")
		assert.Contains(t, files, filepath.Join("dir", "file2.txt"))
		assert.Contains(t, files, filepath.Join("dir", "subdir", "file3.txt"))
	})

	t.Run("should return all files when query is empty", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("content1"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("file2.go", []byte("content2"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("dir/file3.md", []byte("content3"))
		require.NoError(t, err)

		tool := NewVFSFindTool(mockVFS)

		// Execute with empty query
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query":     "",
				"recursive": true,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.True(t, response.Result.Has("files"))

		// Verify all files and directories are returned
		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 4)
		files := []string{filesArr[0].AsString(), filesArr[1].AsString(), filesArr[2].AsString(), filesArr[3].AsString()}
		assert.Contains(t, files, "file1.txt")
		assert.Contains(t, files, "file2.go")
		assert.Contains(t, files, "dir")
		assert.Contains(t, files, filepath.Join("dir", "file3.md"))
	})

	t.Run("should return all files when query is not provided", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("content1"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("dir/file2.go", []byte("content2"))
		require.NoError(t, err)

		tool := NewVFSFindTool(mockVFS)

		// Execute without query parameter
		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "vfsFind",
			Arguments: NewToolValue(map[string]any{}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.True(t, response.Result.Has("files"))

		// Verify all files and directories are returned (recursive by default, empty query)
		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 3)
		files := []string{filesArr[0].AsString(), filesArr[1].AsString(), filesArr[2].AsString()}
		assert.Contains(t, files, "file1.txt")
		assert.Contains(t, files, "dir")
		assert.Contains(t, files, filepath.Join("dir", "file2.go"))
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
