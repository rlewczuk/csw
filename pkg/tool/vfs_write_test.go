package tool

import (
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
