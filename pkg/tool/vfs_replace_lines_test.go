package tool

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVFSReplaceLinesTool(t *testing.T) {
	t.Run("should replace inclusive line range", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\nthree\nfour\n"))
		require.NoError(t, err)

		tool := NewVFSReplaceLinesTool(mockVFS, nil)
		response := tool.Execute(newReplaceLinesCall(map[string]any{
			"path":        "test.txt",
			"start_line":  int64(2),
			"end_line":    int64(3),
			"replacement": "TWO\nTHREE\n",
		}))

		require.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "Lines replaced successfully", response.Result.Get("content").AsString())
		assert.NotEmpty(t, response.Result.Get("sha256").AsString())

		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\nTWO\nTHREE\nfour\n", string(content))
	})

	t.Run("should replace first line in file without trailing newline", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\nthree"))
		require.NoError(t, err)

		tool := NewVFSReplaceLinesTool(mockVFS, nil)
		response := tool.Execute(newReplaceLinesCall(map[string]any{
			"path":        "test.txt",
			"start_line":  int64(1),
			"end_line":    int64(1),
			"replacement": "ONE\n",
		}))

		require.NoError(t, response.Error)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "ONE\ntwo\nthree", string(content))
	})

	t.Run("should replace last line", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\nthree"))
		require.NoError(t, err)

		tool := NewVFSReplaceLinesTool(mockVFS, nil)
		response := tool.Execute(newReplaceLinesCall(map[string]any{
			"path":        "test.txt",
			"start_line":  int64(3),
			"end_line":    int64(3),
			"replacement": "THREE",
		}))

		require.NoError(t, response.Error)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\ntwo\nTHREE", string(content))
	})

	t.Run("should delete selected line when replacement is empty", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\nthree\n"))
		require.NoError(t, err)

		tool := NewVFSReplaceLinesTool(mockVFS, nil)
		response := tool.Execute(newReplaceLinesCall(map[string]any{
			"path":        "test.txt",
			"start_line":  int64(2),
			"end_line":    int64(2),
			"replacement": "",
		}))

		require.NoError(t, response.Error)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\nthree\n", string(content))
	})

	t.Run("should verify expected sha256 before editing", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		original := []byte("one\ntwo\n")
		err := mockVFS.WriteFile("test.txt", original)
		require.NoError(t, err)

		tool := NewVFSReplaceLinesTool(mockVFS, nil)
		response := tool.Execute(newReplaceLinesCall(map[string]any{
			"path":            "test.txt",
			"start_line":      int64(2),
			"end_line":        int64(2),
			"replacement":     "TWO\n",
			"expected_sha256": sha256Hex(original),
		}))

		require.NoError(t, response.Error)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\nTWO\n", string(content))
	})

	t.Run("should reject sha256 mismatch without modifying file", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\n"))
		require.NoError(t, err)

		tool := NewVFSReplaceLinesTool(mockVFS, nil)
		response := tool.Execute(newReplaceLinesCall(map[string]any{
			"path":            "test.txt",
			"start_line":      int64(2),
			"end_line":        int64(2),
			"replacement":     "TWO\n",
			"expected_sha256": "deadbeef",
		}))

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "expected_sha256 mismatch")
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\ntwo\n", string(content))
	})

	t.Run("should return error for invalid line ranges", func(t *testing.T) {
		tests := []struct {
			name      string
			startLine int64
			endLine   int64
			wantError string
		}{
			{name: "start less than one", startLine: 0, endLine: 1, wantError: "start_line must be greater than 0"},
			{name: "end before start", startLine: 2, endLine: 1, wantError: "end_line must be greater than or equal to start_line"},
			{name: "end outside file", startLine: 1, endLine: 3, wantError: "outside file with 2 lines"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockVFS := vfs.NewMockVFS()
				err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\n"))
				require.NoError(t, err)

				tool := NewVFSReplaceLinesTool(mockVFS, nil)
				response := tool.Execute(newReplaceLinesCall(map[string]any{
					"path":        "test.txt",
					"start_line":  tt.startLine,
					"end_line":    tt.endLine,
					"replacement": "changed\n",
				}))

				require.Error(t, response.Error)
				assert.Contains(t, response.Error.Error(), tt.wantError)
				content, err := mockVFS.ReadFile("test.txt")
				require.NoError(t, err)
				assert.Equal(t, "one\ntwo\n", string(content))
			})
		}
	})

	t.Run("should return error for empty file", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte(""))
		require.NoError(t, err)

		tool := NewVFSReplaceLinesTool(mockVFS, nil)
		response := tool.Execute(newReplaceLinesCall(map[string]any{
			"path":        "test.txt",
			"start_line":  int64(1),
			"end_line":    int64(1),
			"replacement": "content\n",
		}))

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "outside empty file")
	})

	t.Run("should return error for missing arguments", func(t *testing.T) {
		tests := []struct {
			name      string
			arguments map[string]any
			wantError string
		}{
			{name: "path", arguments: map[string]any{"start_line": int64(1), "end_line": int64(1), "replacement": "x"}, wantError: "missing required argument: path"},
			{name: "start_line", arguments: map[string]any{"path": "test.txt", "end_line": int64(1), "replacement": "x"}, wantError: "missing required argument: start_line"},
			{name: "end_line", arguments: map[string]any{"path": "test.txt", "start_line": int64(1), "replacement": "x"}, wantError: "missing required argument: end_line"},
			{name: "replacement", arguments: map[string]any{"path": "test.txt", "start_line": int64(1), "end_line": int64(1)}, wantError: "missing required argument: replacement"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockVFS := vfs.NewMockVFS()
				tool := NewVFSReplaceLinesTool(mockVFS, nil)

				response := tool.Execute(newReplaceLinesCall(tt.arguments))

				require.Error(t, response.Error)
				assert.Contains(t, response.Error.Error(), tt.wantError)
				assert.True(t, response.Done)
			})
		}
	})
}

func TestVFSReplaceLinesToolPermissions(t *testing.T) {
	t.Run("should request read permission before editing", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\n"))
		require.NoError(t, err)

		accessVFS := vfs.NewAccessControlVFS(mockVFS, map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAsk, Write: conf.AccessAllow},
		})
		tool := NewVFSReplaceLinesTool(accessVFS, nil)

		response := tool.Execute(newReplaceLinesCall(map[string]any{
			"path":        "test.txt",
			"start_line":  int64(1),
			"end_line":    int64(1),
			"replacement": "ONE\n",
		}))

		require.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, apis.ErrPermissionDenied)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\ntwo\n", string(content))
	})

	t.Run("should request write permission after reading", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\n"))
		require.NoError(t, err)

		accessVFS := vfs.NewAccessControlVFS(mockVFS, map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAllow, Write: conf.AccessAsk},
		})
		tool := NewVFSReplaceLinesTool(accessVFS, nil)

		response := tool.Execute(newReplaceLinesCall(map[string]any{
			"path":        "test.txt",
			"start_line":  int64(1),
			"end_line":    int64(1),
			"replacement": "ONE\n",
		}))

		require.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, apis.ErrPermissionDenied)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\ntwo\n", string(content))
	})
}

func TestVFSReplaceLinesToolLSP(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	err := mockVFS.WriteFile("test.go", []byte("package main\n"))
	require.NoError(t, err)

	mockLSP, err := lsp.NewMockLSP("/path/to/worktree")
	require.NoError(t, err)
	require.NoError(t, mockLSP.Init(true))
	mockLSP.SetDiagnostics(pathToURI("test.go"), []lsp.Diagnostic{
		{
			Range:    lsp.Range{Start: lsp.Position{Line: 0, Character: 8}},
			Severity: lsp.SeverityError,
			Message:  "expected package name",
		},
	})

	tool := NewVFSReplaceLinesTool(mockVFS, mockLSP)
	response := tool.Execute(newReplaceLinesCall(map[string]any{
		"path":        "test.go",
		"start_line":  int64(1),
		"end_line":    int64(1),
		"replacement": "package\n",
	}))

	require.NoError(t, response.Error)
	content := response.Result.Get("content").AsString()
	assert.Contains(t, content, "LSP errors detected in this file")
	assert.Contains(t, content, "Error[1:9] expected package name")
	assert.NotEmpty(t, response.Result.Get("sha256").AsString())
}

func TestVFSReplaceLinesToolRender(t *testing.T) {
	tool := NewVFSReplaceLinesTool(vfs.NewMockVFS(), nil)
	call := newReplaceLinesCall(map[string]any{
		"path":        "test.txt",
		"start_line":  int64(2),
		"end_line":    int64(3),
		"replacement": "TWO\nTHREE\n",
	})

	oneLiner, full, jsonl, attachments := tool.Render(call)

	assert.Contains(t, oneLiner, "replace lines test.txt:2-3")
	assert.Contains(t, oneLiner, "+3/-2")
	assert.Contains(t, full, "--- test.txt:2-3")
	assert.Contains(t, full, "TWO\nTHREE\n")
	assert.Contains(t, jsonl, "vfsReplaceLines")
	assert.Empty(t, attachments)
}

func TestRegisterVFSToolsIncludesReplaceLines(t *testing.T) {
	registry := NewToolRegistry()
	RegisterVFSTools(registry, vfs.NewMockVFS(), nil, nil)

	registeredTool, err := registry.Get("vfsReplaceLines")
	require.NoError(t, err)
	assert.IsType(t, &VFSReplaceLinesTool{}, registeredTool)
}

func newReplaceLinesCall(arguments map[string]any) *ToolCall {
	return &ToolCall{
		ID:        "test-id",
		Function:  "vfsReplaceLines",
		Arguments: NewToolValue(arguments),
	}
}
