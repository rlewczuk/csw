package tool

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVFSListTool(t *testing.T) {
	t.Run("should list files non-recursively", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		require.NoError(t, mockVFS.WriteFile("a.txt", []byte("a")))
		require.NoError(t, mockVFS.WriteFile("dir/b.txt", []byte("b")))

		listTool := NewVFSListTool(mockVFS)
		response := listTool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsList",
			Arguments: NewToolValue(map[string]any{
				"path":      ".",
				"recursive": false,
			}),
		})

		require.NoError(t, response.Error)
		files := response.Result.Get("files").Array()
		assert.Len(t, files, 2)
		assert.Contains(t, []string{files[0].AsString(), files[1].AsString()}, "a.txt")
		assert.Contains(t, []string{files[0].AsString(), files[1].AsString()}, "dir")
	})

	t.Run("should list files recursively", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		require.NoError(t, mockVFS.WriteFile("a.txt", []byte("a")))
		require.NoError(t, mockVFS.WriteFile("dir/b.txt", []byte("b")))

		listTool := NewVFSListTool(mockVFS)
		response := listTool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsList",
			Arguments: NewToolValue(map[string]any{
				"path":      ".",
				"recursive": true,
			}),
		})

		require.NoError(t, response.Error)
		files := response.Result.Get("files").Array()
		values := make([]string, 0, len(files))
		for _, file := range files {
			values = append(values, file.AsString())
		}
		assert.Contains(t, values, "a.txt")
		assert.Contains(t, values, filepath.Join("dir", "b.txt"))
	})

	t.Run("should filter with glob pattern", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		require.NoError(t, mockVFS.WriteFile("a.go", []byte("a")))
		require.NoError(t, mockVFS.WriteFile("b.txt", []byte("b")))

		listTool := NewVFSListTool(mockVFS)
		response := listTool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsList",
			Arguments: NewToolValue(map[string]any{
				"path":      ".",
				"recursive": true,
				"pattern":   "*.go",
			}),
		})

		require.NoError(t, response.Error)
		files := response.Result.Get("files").Array()
		require.Len(t, files, 1)
		assert.Equal(t, "a.go", files[0].AsString())
	})

	t.Run("should apply limit and set truncated", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		require.NoError(t, mockVFS.WriteFile("a.txt", []byte("a")))
		require.NoError(t, mockVFS.WriteFile("b.txt", []byte("b")))
		require.NoError(t, mockVFS.WriteFile("c.txt", []byte("c")))

		listTool := NewVFSListTool(mockVFS)
		response := listTool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsList",
			Arguments: NewToolValue(map[string]any{
				"path":      ".",
				"recursive": true,
				"pattern":   "*.txt",
				"limit":     2,
			}),
		})

		require.NoError(t, response.Error)
		assert.True(t, response.Result.Bool("truncated"))
		assert.Len(t, response.Result.Get("files").Array(), 2)
	})

	t.Run("should return error when limit is negative", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		listTool := NewVFSListTool(mockVFS)

		response := listTool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsList",
			Arguments: NewToolValue(map[string]any{
				"path":  ".",
				"limit": -1,
			}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "limit must be >= 0")
	})

	t.Run("should default to current directory when path is omitted", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		require.NoError(t, mockVFS.WriteFile("x.txt", []byte("x")))

		listTool := NewVFSListTool(mockVFS)
		response := listTool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "vfsList",
			Arguments: NewToolValue(map[string]any{}),
		})

		require.NoError(t, response.Error)
		assert.NotEmpty(t, response.Result.Get("files").Array())
	})
}

func TestVFSListToolAbsolutePathAllowed(t *testing.T) {
	rootDir := t.TempDir()
	allowedDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(allowedDir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(allowedDir, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(allowedDir, "sub", "b.go"), []byte("b"), 0644))

	localVFS, err := vfs.NewLocalVFS(rootDir, nil, []string{allowedDir})
	require.NoError(t, err)

	listTool := NewVFSListTool(localVFS)
	response := listTool.Execute(&ToolCall{
		ID:       "test-id",
		Function: "vfsList",
		Arguments: NewToolValue(map[string]any{
			"path":      allowedDir,
			"recursive": true,
			"pattern":   "*.go",
		}),
	})

	require.NoError(t, response.Error)
	files := response.Result.Get("files").Array()
	require.Len(t, files, 1)
	assert.Equal(t, filepath.Join(allowedDir, "sub", "b.go"), files[0].AsString())
}

func TestVFSListToolAbsolutePathDenied(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()

	localVFS, err := vfs.NewLocalVFS(rootDir, nil, nil)
	require.NoError(t, err)

	listTool := NewVFSListTool(localVFS)
	response := listTool.Execute(&ToolCall{
		ID:       "test-id",
		Function: "vfsList",
		Arguments: NewToolValue(map[string]any{
			"path": outsideDir,
		}),
	})

	require.Error(t, response.Error)
	assert.ErrorIs(t, response.Error, vfs.ErrPermissionDenied)
}

func TestVFSListToolPermissionQuery(t *testing.T) {
	t.Run("should return permission query when list access is ask", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		require.NoError(t, mockVFS.WriteFile("a.txt", []byte("a")))

		accessVFS := vfs.NewAccessControlVFS(mockVFS, map[string]conf.FileAccess{
			"*": {List: conf.AccessAsk},
		})

		listTool := NewVFSListTool(accessVFS)
		response := listTool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsList",
			Arguments: NewToolValue(map[string]any{
				"path": ".",
			}),
		})

		require.Error(t, response.Error)
		query, ok := response.Error.(*ToolPermissionsQuery)
		require.True(t, ok)
		assert.Equal(t, "vfsList", query.Tool.Function)
	})

	t.Run("should fail when list access is deny", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		accessVFS := vfs.NewAccessControlVFS(mockVFS, map[string]conf.FileAccess{
			"*": {List: conf.AccessDeny},
		})

		listTool := NewVFSListTool(accessVFS)
		response := listTool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsList",
			Arguments: NewToolValue(map[string]any{
				"path": ".",
			}),
		})

		require.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, vfs.ErrPermissionDenied)
	})
}

func TestVFSListToolRender(t *testing.T) {
	listTool := NewVFSListTool(vfs.NewMockVFS())
	call := &ToolCall{
		ID:       "test-id",
		Function: "vfsList",
		Arguments: NewToolValue(map[string]any{
			"path":      ".",
			"pattern":   "*.go",
			"files":     []any{"a.go", "sub/b.go"},
			"truncated": true,
		}),
	}

	oneLiner, full, _ := listTool.Render(call)
	assert.Contains(t, oneLiner, "list . matching *.go")
	assert.Contains(t, oneLiner, "(2 results)")
	assert.Contains(t, full, "a.go")
	assert.Contains(t, full, "sub/b.go")
	assert.Contains(t, full, "Results are truncated")
}
