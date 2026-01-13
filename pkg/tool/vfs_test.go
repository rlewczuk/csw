package tool

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/shared"
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

	t.Run("should have correct tool info", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS)
		info := tool.Info()
		assert.Equal(t, "vfs.read", info.Name)
		assert.NotEmpty(t, info.Description)
		assert.Equal(t, SchemaTypeObject, info.Schema.Type)
		assert.Contains(t, info.Schema.Properties, "path")
		assert.Equal(t, []string{"path"}, info.Schema.Required)
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

	t.Run("should have correct tool info", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSWriteTool(mockVFS)
		info := tool.Info()
		assert.Equal(t, "vfs.write", info.Name)
		assert.NotEmpty(t, info.Description)
		assert.Equal(t, SchemaTypeObject, info.Schema.Type)
		assert.Contains(t, info.Schema.Properties, "path")
		assert.Contains(t, info.Schema.Properties, "content")
		assert.Contains(t, info.Schema.Required, "path")
		assert.Contains(t, info.Schema.Required, "content")
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
		assert.Equal(t, vfs.ErrFileNotFound, err)
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

	t.Run("should have correct tool info", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSDeleteTool(mockVFS)
		info := tool.Info()
		assert.Equal(t, "vfs.delete", info.Name)
		assert.NotEmpty(t, info.Description)
		assert.Equal(t, SchemaTypeObject, info.Schema.Type)
		assert.Contains(t, info.Schema.Properties, "path")
		assert.Equal(t, []string{"path"}, info.Schema.Required)
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

	t.Run("should have correct tool info", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSListTool(mockVFS)
		info := tool.Info()
		assert.Equal(t, "vfs.list", info.Name)
		assert.NotEmpty(t, info.Description)
		assert.Equal(t, SchemaTypeObject, info.Schema.Type)
		assert.Contains(t, info.Schema.Properties, "path")
		assert.Equal(t, []string{"path"}, info.Schema.Required)
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
		assert.Equal(t, vfs.ErrFileNotFound, err)

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

	t.Run("should have correct tool info", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSMoveTool(mockVFS)
		info := tool.Info()
		assert.Equal(t, "vfs.move", info.Name)
		assert.NotEmpty(t, info.Description)
		assert.Equal(t, SchemaTypeObject, info.Schema.Type)
		assert.Contains(t, info.Schema.Properties, "path")
		assert.Contains(t, info.Schema.Properties, "destination")
		assert.Contains(t, info.Schema.Required, "path")
		assert.Contains(t, info.Schema.Required, "destination")
	})
}

func TestVFSReadToolPermissionQuery(t *testing.T) {
	t.Run("should return permission query when access is ask", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		privileges := map[string]vfs.FileAccess{
			"*.txt": {Read: shared.AccessAsk},
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

		privileges := map[string]vfs.FileAccess{
			"*.txt": {Read: shared.AccessAllow},
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
		privileges := map[string]vfs.FileAccess{
			"*.txt": {Read: shared.AccessDeny},
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
		privileges := map[string]vfs.FileAccess{
			"*.txt": {Write: shared.AccessAsk},
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
		privileges := map[string]vfs.FileAccess{
			"*.txt": {Write: shared.AccessAllow},
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
		privileges := map[string]vfs.FileAccess{
			"*.txt": {Write: shared.AccessDeny},
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

		privileges := map[string]vfs.FileAccess{
			"*.txt": {Delete: shared.AccessAsk},
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

		privileges := map[string]vfs.FileAccess{
			"*.txt": {Delete: shared.AccessAllow},
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

		privileges := map[string]vfs.FileAccess{
			"*.txt": {Delete: shared.AccessDeny},
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

		privileges := map[string]vfs.FileAccess{
			"*": {List: shared.AccessAsk},
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

		privileges := map[string]vfs.FileAccess{
			"*": {List: shared.AccessAllow},
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
		privileges := map[string]vfs.FileAccess{
			"*": {List: shared.AccessDeny},
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

		privileges := map[string]vfs.FileAccess{
			"*.txt": {Move: shared.AccessAsk, Write: shared.AccessAllow},
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

		privileges := map[string]vfs.FileAccess{
			"*.txt": {Move: shared.AccessAllow, Write: shared.AccessAllow},
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

		privileges := map[string]vfs.FileAccess{
			"*.txt": {Move: shared.AccessDeny, Write: shared.AccessAllow},
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
