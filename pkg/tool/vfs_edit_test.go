package tool

import (
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

		// Verify success message was returned
		contentResult := response.Result.Get("content").AsString()
		assert.Equal(t, "Edit applied successfully", contentResult)
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

		// Verify success message was returned
		contentResult := response.Result.Get("content").AsString()
		assert.Equal(t, "Edit applied successfully", contentResult)
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

		// Verify success message was returned
		contentResult := response.Result.Get("content").AsString()
		assert.Equal(t, "Edit applied successfully", contentResult)
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

		// Verify success message was returned
		contentResult := response.Result.Get("content").AsString()
		assert.Equal(t, "Edit applied successfully", contentResult)
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
		assert.ErrorIs(t, response.Error, apis.ErrPermissionDenied)
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
		assert.ErrorIs(t, response.Error, apis.ErrPermissionDenied)
		assert.True(t, response.Done)
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

		// Verify success message was returned
		contentResult := response.Result.Get("content").AsString()
		assert.Equal(t, "Edit applied successfully", contentResult)
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

		// Verify validation message contains error in new format
		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "LSP errors detected in this file, please fix:")
		assert.Contains(t, content, "<diagnostics file=\"")
		assert.Contains(t, content, "Error[3:17]")
		assert.Contains(t, content, "undefined: invalid")
		assert.Contains(t, content, "</diagnostics>")
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

		// Verify success message was returned
		contentResult := response.Result.Get("content").AsString()
		assert.Equal(t, "Edit applied successfully", contentResult)
	})
}

func TestVFSEditToolRender(t *testing.T) {
	t.Run("should render with relative path for absolute path within worktree", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSEditTool(mockVFS, nil)

		// Execute - use an absolute path that's within the mock worktree (/path/to/worktree)
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "/path/to/worktree/cmd/csw/main.go",
				"oldString": "old",
				"newString": "new",
			}),
		}
		oneLiner, full, _, _ := tool.Render(call)

		// Assert - path should be relative to worktree with line stats (+1/-1)
		assert.Equal(t, "edit cmd/csw/main.go (+1/-1)", oneLiner)
		assert.Contains(t, full, "--- cmd/csw/main.go")
		assert.Contains(t, full, "+++ cmd/csw/main.go")
	})

	t.Run("should render with original path for relative path", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSEditTool(mockVFS, nil)

		// Execute
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "cmd/csw/main.go",
				"oldString": "old",
				"newString": "new",
			}),
		}
		oneLiner, _, _, _ := tool.Render(call)

		// Assert - path should be relative with line stats (+1/-1)
		assert.Equal(t, "edit cmd/csw/main.go (+1/-1)", oneLiner)
	})

	t.Run("should render error in oneLiner and full when error is present", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSEditTool(mockVFS, nil)

		// Execute - simulate error by including error in arguments
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsEdit",
			Arguments: NewToolValue(map[string]any{
				"path":      "cmd/csw/main.go",
				"oldString": "old",
				"newString": "new",
				"error":     "failed to edit file: oldString not found",
			}),
		}
		oneLiner, full, _, _ := tool.Render(call)

		// Assert - oneLiner should have error as second line
		assert.Contains(t, oneLiner, "edit cmd/csw/main.go (+1/-1)")
		assert.Contains(t, oneLiner, "failed to edit file: oldString not found")
		// Assert - full should have ERROR: prefix and not contain diff
		assert.Contains(t, full, "ERROR: failed to edit file: oldString not found")
		assert.NotContains(t, full, "--- cmd/csw/main.go")
	})
}
