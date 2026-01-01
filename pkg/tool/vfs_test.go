package tool

import (
	"encoding/json"
	"testing"

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
			Arguments: map[string]string{
				"path": "test.txt",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "hello world", response.Result["content"])
	})

	t.Run("should return error for missing path argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:        "test-id",
			Function:  "vfs.read",
			Arguments: map[string]string{},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.read",
			Arguments: map[string]string{
				"path": "non-existent.txt",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should have correct tool name", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSReadTool(mockVFS)
		assert.Equal(t, "vfs.read", tool.Name())
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
			Arguments: map[string]string{
				"path":    "test.txt",
				"content": "hello world",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.NotNil(t, response.Result)

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
			Arguments: map[string]string{
				"content": "hello",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should return error for missing content argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSWriteTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.write",
			Arguments: map[string]string{
				"path": "test.txt",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should have correct tool name", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSWriteTool(mockVFS)
		assert.Equal(t, "vfs.write", tool.Name())
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
			Arguments: map[string]string{
				"path": "test.txt",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.NotNil(t, response.Result)

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
			Arguments: map[string]string{},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSDeleteTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.delete",
			Arguments: map[string]string{
				"path": "non-existent.txt",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should have correct tool name", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSDeleteTool(mockVFS)
		assert.Equal(t, "vfs.delete", tool.Name())
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
			Arguments: map[string]string{
				"path": ".",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.NotNil(t, response.Result)

		// Verify files list
		var files []string
		err = json.Unmarshal([]byte(response.Result["files"]), &files)
		require.NoError(t, err)
		assert.Len(t, files, 2)
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
			Arguments: map[string]string{
				"path": ".",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.NotNil(t, response.Result)

		// Verify empty files list
		var files []string
		err := json.Unmarshal([]byte(response.Result["files"]), &files)
		require.NoError(t, err)
		assert.Len(t, files, 0)
	})

	t.Run("should return error for missing path argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSListTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:        "test-id",
			Function:  "vfs.list",
			Arguments: map[string]string{},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should return error for non-existent directory", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSListTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.list",
			Arguments: map[string]string{
				"path": "non-existent",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should have correct tool name", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSListTool(mockVFS)
		assert.Equal(t, "vfs.list", tool.Name())
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
			Arguments: map[string]string{
				"path":        "source.txt",
				"destination": "dest.txt",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.NotNil(t, response.Result)

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
			Arguments: map[string]string{
				"destination": "dest.txt",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should return error for missing destination argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSMoveTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.move",
			Arguments: map[string]string{
				"path": "source.txt",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should return error for non-existent source file", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSMoveTool(mockVFS)

		// Execute
		response := tool.Execute(ToolCall{
			ID:       "test-id",
			Function: "vfs.move",
			Arguments: map[string]string{
				"path":        "non-existent.txt",
				"destination": "dest.txt",
			},
		})

		// Assert
		assert.Equal(t, "test-id", response.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Nil(t, response.Result)
	})

	t.Run("should have correct tool name", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSMoveTool(mockVFS)
		assert.Equal(t, "vfs.move", tool.Name())
	})
}
