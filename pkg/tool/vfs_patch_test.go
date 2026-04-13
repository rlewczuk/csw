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

func TestVFSPatchTool(t *testing.T) {
	t.Run("should return error for missing patchText argument", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "vfsPatch",
			Arguments: NewToolValue(nil),
		})

		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "missing required argument: patchText")
		assert.True(t, response.Done)
	})

	t.Run("should reject empty patch", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "*** Begin Patch\n*** End Patch",
			}),
		})

		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.Equal(t, "patch rejected: empty patch", response.Error.Error())
		assert.True(t, response.Done)
	})

	t.Run("should return verification error for invalid patch", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "not a patch",
			}),
		})

		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "apply_patch verification failed")
		assert.True(t, response.Done)
	})

	t.Run("should apply add update move and delete operations", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		require.NoError(t, mockVFS.WriteFile("existing.txt", []byte("alpha\nbeta\ngamma\n")))
		require.NoError(t, mockVFS.WriteFile("old.txt", []byte("remove me\n")))
		require.NoError(t, mockVFS.WriteFile("rename.txt", []byte("before\n")))

		tool := NewVFSPatchTool(mockVFS, nil)
		patchText := `*** Begin Patch
*** Add File: new.txt
+hello
*** Update File: existing.txt
@@
-beta
+BETA
*** Update File: rename.txt
*** Move to: moved.txt
@@
-before
+after
*** Delete File: old.txt
*** End Patch`

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": patchText,
			}),
		})

		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "Success. Updated the following files:")
		assert.Contains(t, content, "A new.txt")
		assert.Contains(t, content, "M existing.txt")
		assert.Contains(t, content, "M moved.txt")
		assert.Contains(t, content, "D old.txt")

		newContent, err := mockVFS.ReadFile("new.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello\n", string(newContent))

		existing, err := mockVFS.ReadFile("existing.txt")
		require.NoError(t, err)
		assert.Equal(t, "alpha\nBETA\ngamma\n", string(existing))

		moved, err := mockVFS.ReadFile("moved.txt")
		require.NoError(t, err)
		assert.Equal(t, "after\n", string(moved))

		_, err = mockVFS.ReadFile("rename.txt")
		assert.ErrorIs(t, err, apis.ErrFileNotFound)
		_, err = mockVFS.ReadFile("old.txt")
		assert.ErrorIs(t, err, apis.ErrFileNotFound)
	})

	t.Run("should include lsp diagnostics for changed files", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		require.NoError(t, mockVFS.WriteFile("test.go", []byte("package main\n\nfunc main() {}\n")))

		mockLSP, err := lsp.NewMockLSP("/tmp/test")
		require.NoError(t, err)
		require.NoError(t, mockLSP.Init(true))

		absPath, err := filepath.Abs("test.go")
		require.NoError(t, err)
		uri := "file://" + filepath.ToSlash(absPath)
		mockLSP.SetDiagnostics(uri, []lsp.Diagnostic{
			{
				Range: lsp.Range{
					Start: lsp.Position{Line: 2, Character: 16},
					End:   lsp.Position{Line: 2, Character: 20},
				},
				Severity: lsp.SeverityError,
				Message:  "undefined: bad",
			},
		})

		tool := NewVFSPatchTool(mockVFS, mockLSP)
		patchText := `*** Begin Patch
*** Update File: test.go
@@
-func main() {}
+func main() { bad() }
*** End Patch`
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": patchText,
			}),
		})

		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		msg := response.Result.Get("content").AsString()
		assert.Contains(t, msg, "LSP errors detected in test.go, please fix:")
		assert.Contains(t, msg, "<diagnostics file=\"test.go\">")
		assert.Contains(t, msg, "Error[3:17] undefined: bad")
	})
}

func TestVFSPatchToolRender(t *testing.T) {
	t.Run("should return simple oneliner for empty patch", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		oneLiner, full, _, _ := tool.Render(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "",
			}),
		})

		assert.Equal(t, "apply patch", oneLiner)
		assert.Equal(t, "apply patch", full)
	})

	t.Run("should return simple oneliner for invalid patch", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		oneLiner, _, _, _ := tool.Render(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "not a valid patch",
			}),
		})

		assert.Equal(t, "apply patch", oneLiner)
	})

	t.Run("should show added file with line count", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		oneLiner, _, _, _ := tool.Render(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "*** Begin Patch\n*** Add File: /path/to/worktree/pkg/new.txt\n+line1\n+line2\n+line3\n*** End Patch",
			}),
		})

		assert.Equal(t, "apply patch: A:pkg/new.txt(+3)", oneLiner)
	})

	t.Run("should show deleted file", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		oneLiner, _, _, _ := tool.Render(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "*** Begin Patch\n*** Delete File: /path/to/worktree/pkg/old.txt\n*** End Patch",
			}),
		})

		assert.Equal(t, "apply patch: D:pkg/old.txt", oneLiner)
	})

	t.Run("should show updated file with added and removed lines", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		oneLiner, _, _, _ := tool.Render(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": `*** Begin Patch
*** Update File: /path/to/worktree/pkg/existing.txt
@@
-old line 1
-old line 2
+new line 1
+new line 2
+new line 3
*** End Patch`,
			}),
		})

		assert.Equal(t, "apply patch: U:pkg/existing.txt(+3/-2)", oneLiner)
	})

	t.Run("should show moved file with M prefix", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		oneLiner, _, _, _ := tool.Render(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": `*** Begin Patch
*** Update File: /path/to/worktree/pkg/old.txt
*** Move to: /path/to/worktree/pkg/new.txt
@@
-old content
+new content
*** End Patch`,
			}),
		})

		assert.Equal(t, "apply patch: M:pkg/new.txt(+1/-1)", oneLiner)
	})

	t.Run("should show multiple files space separated", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		oneLiner, _, _, _ := tool.Render(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": `*** Begin Patch
*** Add File: new.txt
+line1
*** Update File: existing.txt
@@
-old
+new
*** Delete File: remove.txt
*** End Patch`,
			}),
		})

		assert.Equal(t, "apply patch: A:new.txt(+1) U:existing.txt(+1/-1) D:remove.txt", oneLiner)
	})

	t.Run("should use project-relative path in output", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		oneLiner, _, _, _ := tool.Render(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "*** Begin Patch\n*** Add File: /path/to/worktree/very/long/path/to/the/file/name.txt\n+content\n*** End Patch",
			}),
		})

		assert.Equal(t, "apply patch: A:very/long/path/to/the/file/name.txt(+1)", oneLiner)
	})

	t.Run("should render error in oneLiner and full when error is present", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSPatchTool(mockVFS, nil)

		oneLiner, full, _, _ := tool.Render(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "*** Begin Patch\n*** Add File: test.txt\n+content\n*** End Patch",
				"error":     "failed to apply patch: file not found",
			}),
		})

		// Assert - oneLiner should have error as second line
		assert.Contains(t, oneLiner, "apply patch:")
		assert.Contains(t, oneLiner, "failed to apply patch: file not found")
		// Assert - full should have ERROR: prefix and not contain patch
		assert.Contains(t, full, "ERROR: failed to apply patch: file not found")
		assert.NotContains(t, full, "*** Begin Patch")
	})
}

func TestVFSPatchToolPermissionQuery(t *testing.T) {
	t.Run("should fail when read access is ask", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		require.NoError(t, mockVFS.WriteFile("test.txt", []byte("hello\n")))

		accessVFS := vfs.NewAccessControlVFS(mockVFS, map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAsk, Write: conf.AccessAllow, Delete: conf.AccessAllow},
		})
		tool := NewVFSPatchTool(accessVFS, nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "*** Begin Patch\n*** Update File: test.txt\n@@\n-hello\n+hi\n*** End Patch",
			}),
		})

		assert.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, apis.ErrPermissionDenied)
	})

	t.Run("should fail when write access is ask", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()

		accessVFS := vfs.NewAccessControlVFS(mockVFS, map[string]conf.FileAccess{
			"*.txt": {Write: conf.AccessAsk},
		})
		tool := NewVFSPatchTool(accessVFS, nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "*** Begin Patch\n*** Add File: created.txt\n+new\n*** End Patch",
			}),
		})

		assert.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, apis.ErrPermissionDenied)
	})

	t.Run("should fail when delete access is ask", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		require.NoError(t, mockVFS.WriteFile("delete.txt", []byte("x\n")))

		accessVFS := vfs.NewAccessControlVFS(mockVFS, map[string]conf.FileAccess{
			"delete.txt": {Read: conf.AccessAllow, Delete: conf.AccessAsk},
		})
		tool := NewVFSPatchTool(accessVFS, nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsPatch",
			Arguments: NewToolValue(map[string]any{
				"patchText": "*** Begin Patch\n*** Delete File: delete.txt\n*** End Patch",
			}),
		})

		assert.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, apis.ErrPermissionDenied)
	})
}
