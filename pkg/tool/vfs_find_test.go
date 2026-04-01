package tool

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
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

	t.Run("should return first 25 files with suffix when more than 255 results", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()

		for i := 0; i < 256; i++ {
			err := mockVFS.WriteFile("file"+formatInt64(int64(i))+".txt", []byte("content"))
			require.NoError(t, err)
		}

		tool := NewVFSFindTool(mockVFS)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query":     "*.txt",
				"recursive": false,
			}),
		})

		require.NoError(t, response.Error)
		filesArr := response.Result.Get("files").Array()
		require.Len(t, filesArr, 25)
		assert.Equal(t, tooManyResultsSuffix, response.Result.Get("suffix").AsString())
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
		assert.ErrorIs(t, response.Error, apis.ErrPermissionDenied)
		assert.True(t, response.Done)
	})
}

func TestVFSFindToolAbsolutePath(t *testing.T) {
	t.Run("should find files in allowed absolute path", func(t *testing.T) {
		rootDir := t.TempDir()
		allowedDir := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(allowedDir, "a.txt"), []byte("a"), 0644))
		require.NoError(t, os.MkdirAll(filepath.Join(allowedDir, "sub"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(allowedDir, "sub", "b.go"), []byte("b"), 0644))

		localVFS, err := vfs.NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		tool := NewVFSFindTool(localVFS)
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"path":      allowedDir,
				"query":     "*.go",
				"recursive": true,
			}),
		})

		require.NoError(t, response.Error)
		files := response.Result.Get("files").Array()
		require.Len(t, files, 1)
		assert.Equal(t, filepath.Join(allowedDir, "sub", "b.go"), files[0].AsString())
	})

	t.Run("should fail when absolute path is outside allowed paths", func(t *testing.T) {
		rootDir := t.TempDir()
		outsideDir := t.TempDir()

		localVFS, err := vfs.NewLocalVFS(rootDir, nil, nil)
		require.NoError(t, err)

		tool := NewVFSFindTool(localVFS)
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"path": outsideDir,
			}),
		})

		require.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, apis.ErrPermissionDenied)
	})
}

func TestVFSFindToolRender(t *testing.T) {
	t.Run("should render basic find query", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSFindTool(mockVFS)

		// Execute
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query": "*.txt",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert
		assert.Equal(t, "find *.txt", oneLiner)
		assert.Equal(t, "find *.txt\n\n", full)
	})

	t.Run("should render with files in output", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSFindTool(mockVFS)

		// Execute
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query": "*.txt",
				"files": []any{"file1.txt", "file2.txt", "dir/file3.txt"},
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert
		assert.Equal(t, "find *.txt (3 results)", oneLiner)
		assert.Contains(t, full, "find *.txt (3 results)")
		assert.Contains(t, full, "file1.txt")
		assert.Contains(t, full, "file2.txt")
		assert.Contains(t, full, "dir/file3.txt")
	})

	t.Run("should include error in output when error is present", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSFindTool(mockVFS)

		// Execute - call with error
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query": "*.txt",
				"error": "VFSFindTool.Execute() [vfs_find.go]: permission denied",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - oneliner should contain error on second line
		assert.Contains(t, oneLiner, "find *.txt")
		assert.Contains(t, oneLiner, "permission denied")
		// Assert - full should contain ERROR: prefix
		assert.Contains(t, full, "find *.txt")
		assert.Contains(t, full, "ERROR: VFSFindTool.Execute() [vfs_find.go]: permission denied")
	})

	t.Run("should convert multiline error to single line in oneliner", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSFindTool(mockVFS)

		// Execute - call with multiline error
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query": "*.go",
				"error": "error details:\nline 1\nline 2",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - oneliner should have error on single line (newlines converted to spaces)
		assert.Contains(t, oneLiner, "find *.go")
		// Check that oneliner does not contain literal newlines in error portion
		lines := splitLines(oneLiner)
		assert.Equal(t, 2, len(lines), "oneliner should have exactly 2 lines")
		// Assert - full should contain full error with ERROR: prefix
		assert.Contains(t, full, "ERROR: error details:\nline 1\nline 2")
	})

	t.Run("should show single result with singular form", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSFindTool(mockVFS)

		// Execute - call with single file
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query": "*.txt",
				"files": []any{"file1.txt"},
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - should use singular form "1 result"
		assert.Equal(t, "find *.txt (1 result)", oneLiner)
		assert.Contains(t, full, "find *.txt (1 result)\n\n")
		assert.Contains(t, full, "file1.txt")
	})

	t.Run("should not show result count when no files", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSFindTool(mockVFS)

		// Execute - call with empty files array
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query": "*.txt",
				"files": []any{},
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - should not include result count
		assert.Equal(t, "find *.txt", oneLiner)
		assert.Equal(t, "find *.txt\n\n", full)
	})

	t.Run("should not show result count when files not present", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSFindTool(mockVFS)

		// Execute - call without files
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query": "*.txt",
			}),
		}

		oneLiner, full, _, _ := tool.Render(call)

		// Assert - should not include result count
		assert.Equal(t, "find *.txt", oneLiner)
		assert.Equal(t, "find *.txt\n\n", full)
	})

	t.Run("should include suffix in full output", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSFindTool(mockVFS)

		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsFind",
			Arguments: NewToolValue(map[string]any{
				"query":  "*.txt",
				"files":  []any{"file1.txt"},
				"suffix": tooManyResultsSuffix,
			}),
		}

		_, full, _, _ := tool.Render(call)

		assert.Contains(t, full, "file1.txt")
		assert.Contains(t, full, tooManyResultsSuffix)
	})
}
