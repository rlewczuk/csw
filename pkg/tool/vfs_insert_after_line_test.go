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

func TestVFSInsertAfterLineTool(t *testing.T) {
	t.Run("should insert after middle line", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\nthree\n"))
		require.NoError(t, err)

		tool := NewVFSInsertAfterLineTool(mockVFS, nil)
		response := tool.Execute(newInsertAfterLineCall(map[string]any{
			"path":        "test.txt",
			"line_number": int64(2),
			"content":     "inserted\n",
		}))

		require.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "Line inserted successfully", response.Result.Get("content").AsString())
		assert.NotEmpty(t, response.Result.Get("sha256").AsString())

		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\ntwo\ninserted\nthree\n", string(content))
	})

	t.Run("should insert at beginning with line zero", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\n"))
		require.NoError(t, err)

		tool := NewVFSInsertAfterLineTool(mockVFS, nil)
		response := tool.Execute(newInsertAfterLineCall(map[string]any{
			"path":        "test.txt",
			"line_number": int64(0),
			"content":     "zero\n",
		}))

		require.NoError(t, response.Error)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "zero\none\ntwo\n", string(content))
	})

	t.Run("should insert after last line in file with trailing newline", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\n"))
		require.NoError(t, err)

		tool := NewVFSInsertAfterLineTool(mockVFS, nil)
		response := tool.Execute(newInsertAfterLineCall(map[string]any{
			"path":        "test.txt",
			"line_number": int64(2),
			"content":     "three\n",
		}))

		require.NoError(t, response.Error)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\ntwo\nthree\n", string(content))
	})

	t.Run("should insert after last line in file without trailing newline exactly as provided", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo"))
		require.NoError(t, err)

		tool := NewVFSInsertAfterLineTool(mockVFS, nil)
		response := tool.Execute(newInsertAfterLineCall(map[string]any{
			"path":        "test.txt",
			"line_number": int64(2),
			"content":     "three",
		}))

		require.NoError(t, response.Error)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\ntwothree", string(content))
	})

	t.Run("should insert into empty file only at beginning", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte(""))
		require.NoError(t, err)

		tool := NewVFSInsertAfterLineTool(mockVFS, nil)
		response := tool.Execute(newInsertAfterLineCall(map[string]any{
			"path":        "test.txt",
			"line_number": int64(0),
			"content":     "first\n",
		}))

		require.NoError(t, response.Error)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "first\n", string(content))
	})

	t.Run("should verify expected sha256 before editing", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		original := []byte("one\ntwo\n")
		err := mockVFS.WriteFile("test.txt", original)
		require.NoError(t, err)

		tool := NewVFSInsertAfterLineTool(mockVFS, nil)
		response := tool.Execute(newInsertAfterLineCall(map[string]any{
			"path":            "test.txt",
			"line_number":     int64(1),
			"content":         "inserted\n",
			"expected_sha256": sha256Hex(original),
		}))

		require.NoError(t, response.Error)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\ninserted\ntwo\n", string(content))
	})

	t.Run("should reject sha256 mismatch without modifying file", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\n"))
		require.NoError(t, err)

		tool := NewVFSInsertAfterLineTool(mockVFS, nil)
		response := tool.Execute(newInsertAfterLineCall(map[string]any{
			"path":            "test.txt",
			"line_number":     int64(1),
			"content":         "inserted\n",
			"expected_sha256": "deadbeef",
		}))

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "expected_sha256 mismatch")
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\ntwo\n", string(content))
	})

	t.Run("should return error for invalid line numbers", func(t *testing.T) {
		tests := []struct {
			name       string
			lineNumber int64
			wantError  string
		}{
			{name: "negative", lineNumber: -1, wantError: "line_number must be greater than or equal to 0"},
			{name: "outside file", lineNumber: 3, wantError: "outside file with 2 lines"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockVFS := vfs.NewMockVFS()
				err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\n"))
				require.NoError(t, err)

				tool := NewVFSInsertAfterLineTool(mockVFS, nil)
				response := tool.Execute(newInsertAfterLineCall(map[string]any{
					"path":        "test.txt",
					"line_number": tt.lineNumber,
					"content":     "inserted\n",
				}))

				require.Error(t, response.Error)
				assert.Contains(t, response.Error.Error(), tt.wantError)
				content, err := mockVFS.ReadFile("test.txt")
				require.NoError(t, err)
				assert.Equal(t, "one\ntwo\n", string(content))
			})
		}
	})

	t.Run("should return error for missing arguments", func(t *testing.T) {
		tests := []struct {
			name      string
			arguments map[string]any
			wantError string
		}{
			{name: "path", arguments: map[string]any{"line_number": int64(0), "content": "x"}, wantError: "missing required argument: path"},
			{name: "line_number", arguments: map[string]any{"path": "test.txt", "content": "x"}, wantError: "missing required argument: line_number"},
			{name: "content", arguments: map[string]any{"path": "test.txt", "line_number": int64(0)}, wantError: "missing required argument: content"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockVFS := vfs.NewMockVFS()
				tool := NewVFSInsertAfterLineTool(mockVFS, nil)

				response := tool.Execute(newInsertAfterLineCall(tt.arguments))

				require.Error(t, response.Error)
				assert.Contains(t, response.Error.Error(), tt.wantError)
				assert.True(t, response.Done)
			})
		}
	})
}

func TestVFSInsertAfterLineToolPermissions(t *testing.T) {
	t.Run("should request read permission before editing", func(t *testing.T) {
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("test.txt", []byte("one\ntwo\n"))
		require.NoError(t, err)

		accessVFS := vfs.NewAccessControlVFS(mockVFS, map[string]conf.FileAccess{
			"*.txt": {Read: conf.AccessAsk, Write: conf.AccessAllow},
		})
		tool := NewVFSInsertAfterLineTool(accessVFS, nil)

		response := tool.Execute(newInsertAfterLineCall(map[string]any{
			"path":        "test.txt",
			"line_number": int64(1),
			"content":     "inserted\n",
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
		tool := NewVFSInsertAfterLineTool(accessVFS, nil)

		response := tool.Execute(newInsertAfterLineCall(map[string]any{
			"path":        "test.txt",
			"line_number": int64(1),
			"content":     "inserted\n",
		}))

		require.Error(t, response.Error)
		assert.ErrorIs(t, response.Error, apis.ErrPermissionDenied)
		content, err := mockVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "one\ntwo\n", string(content))
	})
}

func TestVFSInsertAfterLineToolLSP(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	err := mockVFS.WriteFile("test.go", []byte("package main\n"))
	require.NoError(t, err)

	mockLSP, err := lsp.NewMockLSP("/path/to/worktree")
	require.NoError(t, err)
	require.NoError(t, mockLSP.Init(true))
	mockLSP.SetDiagnostics(pathToURI("test.go"), []lsp.Diagnostic{
		{
			Range:    lsp.Range{Start: lsp.Position{Line: 1, Character: 0}},
			Severity: lsp.SeverityError,
			Message:  "expected declaration",
		},
	})

	tool := NewVFSInsertAfterLineTool(mockVFS, mockLSP)
	response := tool.Execute(newInsertAfterLineCall(map[string]any{
		"path":        "test.go",
		"line_number": int64(1),
		"content":     "bad\n",
	}))

	require.NoError(t, response.Error)
	content := response.Result.Get("content").AsString()
	assert.Contains(t, content, "LSP errors detected in this file")
	assert.Contains(t, content, "Error[2:1] expected declaration")
	assert.NotEmpty(t, response.Result.Get("sha256").AsString())
}

func TestVFSInsertAfterLineToolRender(t *testing.T) {
	tool := NewVFSInsertAfterLineTool(vfs.NewMockVFS(), nil)
	call := newInsertAfterLineCall(map[string]any{
		"path":        "test.txt",
		"line_number": int64(2),
		"content":     "inserted\nsecond\n",
	})

	oneLiner, full, jsonl, attachments := tool.Render(call)

	assert.Contains(t, oneLiner, "insert after line test.txt:2")
	assert.Contains(t, oneLiner, "+3")
	assert.Contains(t, full, "+++ test.txt:2")
	assert.Contains(t, full, "inserted\nsecond\n")
	assert.Contains(t, jsonl, "vfsInsertAfterLine")
	assert.Empty(t, attachments)
}

func TestRegisterVFSToolsIncludesInsertAfterLine(t *testing.T) {
	registry := NewToolRegistry()
	RegisterVFSTools(registry, vfs.NewMockVFS(), nil, nil)

	registeredTool, err := registry.Get("vfsInsertAfterLine")
	require.NoError(t, err)
	assert.IsType(t, &VFSInsertAfterLineTool{}, registeredTool)
}

func newInsertAfterLineCall(arguments map[string]any) *ToolCall {
	return &ToolCall{
		ID:        "test-id",
		Function:  "vfsInsertAfterLine",
		Arguments: NewToolValue(arguments),
	}
}
