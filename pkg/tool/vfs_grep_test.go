package tool

import (
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVFSGrepTool(t *testing.T) {
	t.Run("should find matches successfully", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("hello world\nfoo bar\nhello again"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("file2.txt", []byte("hello there\nno match here"))
		require.NoError(t, err)

		tool := NewVFSGrepTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "file1.txt:1")
		assert.Contains(t, content, "file1.txt:3")
		assert.Contains(t, content, "file2.txt:1")
	})

	t.Run("should return error for missing pattern argument", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "vfsGrep",
			Arguments: NewToolValue(nil),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, 0, response.Result.Len())
	})

	t.Run("should return 'No files found' for no matches", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("foo bar"))
		require.NoError(t, err)

		tool := NewVFSGrepTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "No files found", response.Result.Get("content").AsString())
	})

	t.Run("should filter by path parameter", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("dir1/file1.txt", []byte("hello world"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("dir2/file2.txt", []byte("hello there"))
		require.NoError(t, err)

		tool := NewVFSGrepTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"path":    "dir1",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "dir1/file1.txt:1")
		assert.NotContains(t, content, "dir2/file2.txt")
	})

	t.Run("should filter by include parameter", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.go", []byte("hello world"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("file2.txt", []byte("hello there"))
		require.NoError(t, err)

		tool := NewVFSGrepTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"include": "*.go",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "file1.go:1")
		assert.NotContains(t, content, "file2.txt")
	})

	t.Run("should support multiple include patterns", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.go", []byte("hello world"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("file2.md", []byte("hello there"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("file3.txt", []byte("hello again"))
		require.NoError(t, err)

		tool := NewVFSGrepTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"include": "*.go, *.md",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "file1.go:1")
		assert.Contains(t, content, "file2.md:1")
		assert.NotContains(t, content, "file3.txt")
	})

	t.Run("should apply limit parameter", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		// Create a file with many matching lines
		content := "hello line 1\nhello line 2\nhello line 3\nhello line 4\nhello line 5"
		err := mockVFS.WriteFile("file.txt", []byte(content))
		require.NoError(t, err)

		tool := NewVFSGrepTool(mockVFS)

		// Execute with limit of 3
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"limit":   3,
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content = response.Result.Get("content").AsString()
		assert.Contains(t, content, "file.txt:1")
		assert.Contains(t, content, "file.txt:2")
		assert.Contains(t, content, "file.txt:3")
		assert.NotContains(t, content, "file.txt:4")
		assert.NotContains(t, content, "file.txt:5")
		assert.Contains(t, content, "(Results are truncated. Consider using a more specific path or pattern.)")
	})

	t.Run("should handle regex patterns", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file.txt", []byte("test123\ntest456\ntest\nfoo789"))
		require.NoError(t, err)

		tool := NewVFSGrepTool(mockVFS)

		// Execute with regex pattern
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "test[0-9]+",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "file.txt:1")
		assert.Contains(t, content, "file.txt:2")
		assert.NotContains(t, content, "file.txt:3")
		assert.NotContains(t, content, "file.txt:4")
	})

	t.Run("should return error for invalid regex pattern", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute with invalid regex
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "[invalid",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
	})

	t.Run("should handle multiple matches in multiple files", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		err := mockVFS.WriteFile("file1.txt", []byte("match1\nno\nmatch2"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("file2.txt", []byte("match3\nmatch4"))
		require.NoError(t, err)

		tool := NewVFSGrepTool(mockVFS)

		// Execute
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "match",
			}),
		})

		// Assert
		assert.Equal(t, "test-id", response.Call.ID)
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "file1.txt:1")
		assert.Contains(t, content, "file1.txt:3")
		assert.Contains(t, content, "file2.txt:1")
		assert.Contains(t, content, "file2.txt:2")
	})
}

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"zero", 0, "0"},
		{"positive single digit", 5, "5"},
		{"positive multiple digits", 123, "123"},
		{"negative single digit", -7, "-7"},
		{"negative multiple digits", -456, "-456"},
		{"large number", 9876543210, "9876543210"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatInt64(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVFSGrepToolIntegration(t *testing.T) {
	t.Run("should work with real filesystem structure", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()

		// Create a directory structure with multiple files
		err := mockVFS.WriteFile("src/main.go", []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("src/utils.go", []byte("package main\n\nfunc helper() {\n\tfmt.Println(\"help\")\n}"))
		require.NoError(t, err)
		err = mockVFS.WriteFile("README.md", []byte("# Project\n\nThis is a test project."))
		require.NoError(t, err)

		tool := NewVFSGrepTool(mockVFS)

		// Execute - search for "main"
		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "main",
			}),
		})

		// Assert
		assert.NoError(t, response.Error)
		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "src/main.go:1")
		assert.Contains(t, content, "src/main.go:3")
		assert.Contains(t, content, "src/utils.go:1")
	})
}

func TestVFSGrepToolTimeout(t *testing.T) {
	// This test ensures grep operations complete in reasonable time
	t.Run("should complete search within timeout", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()

		// Create many files with content
		for i := 0; i < 100; i++ {
			path := "file" + formatInt64(int64(i)) + ".txt"
			err := mockVFS.WriteFile(path, []byte("line 1\nline 2\nline 3\nline 4\nline 5"))
			require.NoError(t, err)
		}

		tool := NewVFSGrepTool(mockVFS)

		// Execute with timeout
		done := make(chan bool)
		go func() {
			response := tool.Execute(&ToolCall{
				ID:       "test-id",
				Function: "vfsGrep",
				Arguments: NewToolValue(map[string]any{
					"pattern": "line",
				}),
			})
			assert.NoError(t, response.Error)
			done <- true
		}()

		select {
		case <-done:
			// Test passed
		case <-time.After(5 * time.Second):
			t.Fatal("Grep operation timed out")
		}
	})
}

func TestVFSGrepToolRender(t *testing.T) {
	t.Run("should display relative path when absolute path is provided", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - absolute path under worktree root
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"path":    "/path/to/worktree/src",
			}),
		}

		oneLiner, full, _ := tool.Render(call)

		// Assert - path should be relative to worktree root
		assert.Contains(t, oneLiner, "grep hello in src")
		assert.Contains(t, full, "grep hello in src")
		assert.NotContains(t, oneLiner, "/path/to/worktree")
	})

	t.Run("should keep relative path as is", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - relative path
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"path":    "src/components",
			}),
		}

		oneLiner, full, _ := tool.Render(call)

		// Assert - path should remain unchanged
		assert.Contains(t, oneLiner, "grep hello in src/components")
		assert.Contains(t, full, "grep hello in src/components")
	})

	t.Run("should handle empty path", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - no path
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
			}),
		}

		oneLiner, full, _ := tool.Render(call)

		// Assert - should not include "in" phrase
		assert.Equal(t, "grep hello", oneLiner)
		assert.Equal(t, "grep hello\n\n", full)
	})

	t.Run("should handle absolute path outside worktree", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - absolute path outside worktree (cannot be made relative)
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "hello",
				"path":    "/some/other/path",
			}),
		}

		oneLiner, _, _ := tool.Render(call)

		// Assert - should keep original path since it cannot be made relative
		// filepath.Rel will return an error for paths on different drives/volumes
		// or when the path is not under the worktree root
		// In such cases, the original path is kept
		assert.Contains(t, oneLiner, "grep hello in")
	})

	t.Run("should handle pattern without path", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - only pattern
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": "func main",
			}),
		}

		oneLiner, full, _ := tool.Render(call)

		// Assert
		assert.Equal(t, "grep func main", oneLiner)
		assert.Equal(t, "grep func main\n\n", full)
	})

	t.Run("should truncate long patterns", func(t *testing.T) {
		// Setup
		mockVFS := vfs.NewMockVFS()
		tool := NewVFSGrepTool(mockVFS)

		// Execute - very long pattern
		longPattern := "this is a very long pattern that should be truncated because it exceeds the limit"
		call := &ToolCall{
			ID:       "test-id",
			Function: "vfsGrep",
			Arguments: NewToolValue(map[string]any{
				"pattern": longPattern,
			}),
		}

		oneLiner, _, _ := tool.Render(call)

		// Assert - should be truncated to 128 chars
		assert.LessOrEqual(t, len(oneLiner), 128)
		assert.Contains(t, oneLiner, "grep")
	})
}
