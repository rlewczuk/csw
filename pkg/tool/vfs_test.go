package tool

import (
	"path/filepath"
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

		tool := NewVFSReadTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.read",
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
		tool := NewVFSReadTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:        "test-id",
			Function:  "vfs.read",
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
		tool := NewVFSReadTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.read",
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

func TestVFSWriteTool(t *testing.T) {
	t.Run("should write file successfully", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSWriteTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.write",
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
		tool := NewVFSWriteTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.write",
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
		tool := NewVFSWriteTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.write",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.delete",
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
		response := tool.Execute(ToolCall{
			ID:        "test-id",
			Function:  "vfs.delete",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.delete",
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
		response := tool.Execute(ToolCall{
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
		response := tool.Execute(ToolCall{
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
		response := tool.Execute(ToolCall{
			ID:        "test-id",
			Function:  "vfs.list",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.list",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.move",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.move",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.move",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.move",
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

		tool := NewVFSReadTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.read",
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
		assert.Equal(t, "vfs.read", query.Tool.Function)
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

		tool := NewVFSReadTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.read",
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

		tool := NewVFSReadTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.read",
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

		tool := NewVFSWriteTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.write",
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
		assert.Equal(t, "vfs.write", query.Tool.Function)
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

		tool := NewVFSWriteTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.write",
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

		tool := NewVFSWriteTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.write",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.delete",
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
		assert.Equal(t, "vfs.delete", query.Tool.Function)
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.delete",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.delete",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.list",
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
		assert.Equal(t, "vfs.list", query.Tool.Function)
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
		response := tool.Execute(ToolCall{
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.list",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.move",
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
		assert.Equal(t, "vfs.move", query.Tool.Function)
		assert.Equal(t, "Permission Required", query.Title)
		assert.Contains(t, query.Details, "source.txt")
		assert.True(t, query.AllowCustomResponse)
		assert.Contains(t, query.Options, "Allow")
		assert.Contains(t, query.Options, "Deny")
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.move",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.move",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.find",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.find",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.find",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.find",
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
		response := tool.Execute(ToolCall{
			ID:        "test-id",
			Function:  "vfs.find",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.find",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.find",
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
		assert.Equal(t, "vfs.find", query.Tool.Function)
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.find",
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
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.find",
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
	t.Run("should replace first occurrence by default", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world hello"))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "test.txt",
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify only first occurrence was replaced
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "hi world hello", string(content))
	})

	t.Run("should replace all occurrences when replaceAll is true", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world hello"))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":   "test.txt",
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
	})

	t.Run("should return error for missing filePath argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSEditTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should return error for missing oldString argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSEditTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "test.txt",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should return error for missing newString argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSEditTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "test.txt",
				"oldString": "hello",
			}),
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
		tool := NewVFSEditTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "non-existent.txt",
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should handle empty oldString", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world"))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "test.txt",
				"oldString": "",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify content unchanged
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("should handle oldString not found", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world"))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "test.txt",
				"oldString": "goodbye",
				"newString": "hi",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify content unchanged
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("should replace with empty string", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world"))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "test.txt",
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
	})

	t.Run("should handle multiline content", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		content := "line1\nline2\nline3\nline1\nline4"
		err := mockVFS.WriteFile("test.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSEditTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":   "test.txt",
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

		tool := NewVFSEditTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "test.txt",
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
		assert.Equal(t, "vfs.edit", query.Tool.Function)
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

		tool := NewVFSEditTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "test.txt",
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
		assert.Equal(t, "vfs.edit", query.Tool.Function)
		assert.Equal(t, "Permission Required", query.Title)
		assert.Contains(t, query.Details, "test.txt")
		assert.True(t, query.AllowCustomResponse)
		assert.Contains(t, query.Options, "Allow")
		assert.Contains(t, query.Options, "Deny")
	})

	t.Run("should succeed when read and write access are allow", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("hello world hello"))
		require.NoError(t, err)

		privileges := map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAllow, Write: conf.AccessAllow},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSEditTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":   "test.txt",
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
	})

	t.Run("should fail when read access is deny", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		privileges := map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessDeny},
		}
		accessVFS := vfs.NewAccessControlVFS(mockVFS, privileges)

		tool := NewVFSEditTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "test.txt",
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

		tool := NewVFSEditTool(accessVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.edit",
			Arguments: NewToolValue(map[string]any{
				"filePath":  "test.txt",
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
				Function: "vfs.read",
			},
		}
		errMsg := query.Error()
		assert.Contains(t, errMsg, "vfs.read")
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
