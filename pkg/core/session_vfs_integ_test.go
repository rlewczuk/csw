// session_vfs_integ_test.go contains integration tests for VFS tools (grep, edit, move)
// that verify the interaction between session and virtual filesystem operations.
package core

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionGrepToolIntegration(t *testing.T) {
	fixture := newSweSystemFixture(t, "You are skilled software developer.")
	system := fixture.system
	vfsInstance := fixture.vfs

	// Setup test files in VFS
	err := vfsInstance.WriteFile("src/main.go", []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"))
	require.NoError(t, err)
	err = vfsInstance.WriteFile("src/utils.go", []byte("package main\n\nfunc helper() {\n\tfmt.Println(\"help\")\n}"))
	require.NoError(t, err)
	err = vfsInstance.WriteFile("README.md", []byte("# Project\n\nThis is a test project with main content."))
	require.NoError(t, err)

	t.Run("grep tool finds matches across files", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Verify grep tool is registered
		grepTool, err := session.Tools.Get("vfsGrep")
		require.NoError(t, err)
		require.NotNil(t, grepTool)

		// Execute grep tool to find "main"
		response := grepTool.Execute(&tool.ToolCall{
			ID:       "test-grep-1",
			Function: "vfsGrep",
			Arguments: tool.NewToolValue(map[string]any{
				"pattern": "main",
			}),
		})

		// Verify response
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "src/main.go:1")
		assert.Contains(t, content, "src/main.go:3")
		assert.Contains(t, content, "src/utils.go:1")
		assert.Contains(t, content, "README.md:3")
	})

	t.Run("grep tool filters by include pattern", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		grepTool, err := session.Tools.Get("vfsGrep")
		require.NoError(t, err)

		// Execute grep tool with include filter for .go files only
		response := grepTool.Execute(&tool.ToolCall{
			ID:       "test-grep-2",
			Function: "vfsGrep",
			Arguments: tool.NewToolValue(map[string]any{
				"pattern": "main",
				"include": "*.go",
			}),
		})

		// Verify response
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "src/main.go:1")
		assert.Contains(t, content, "src/main.go:3")
		assert.Contains(t, content, "src/utils.go:1")
		assert.NotContains(t, content, "README.md")
	})

	t.Run("grep tool filters by path", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		grepTool, err := session.Tools.Get("vfsGrep")
		require.NoError(t, err)

		// Execute grep tool with path filter for src/ directory
		response := grepTool.Execute(&tool.ToolCall{
			ID:       "test-grep-3",
			Function: "vfsGrep",
			Arguments: tool.NewToolValue(map[string]any{
				"pattern": "main",
				"path":    "src",
			}),
		})

		// Verify response
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "src/main.go:1")
		assert.Contains(t, content, "src/main.go:3")
		assert.Contains(t, content, "src/utils.go:1")
		assert.NotContains(t, content, "README.md")
	})

	t.Run("grep tool returns no files found when no matches", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		grepTool, err := session.Tools.Get("vfsGrep")
		require.NoError(t, err)

		// Execute grep tool with pattern that doesn't match
		response := grepTool.Execute(&tool.ToolCall{
			ID:       "test-grep-4",
			Function: "vfsGrep",
			Arguments: tool.NewToolValue(map[string]any{
				"pattern": "nonexistent_pattern_xyz",
			}),
		})

		// Verify response
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		assert.Equal(t, "No files found", content)
	})

	t.Run("grep tool respects limit parameter", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		grepTool, err := session.Tools.Get("vfsGrep")
		require.NoError(t, err)

		// Execute grep tool with low limit
		response := grepTool.Execute(&tool.ToolCall{
			ID:       "test-grep-5",
			Function: "vfsGrep",
			Arguments: tool.NewToolValue(map[string]any{
				"pattern": "main",
				"limit":   2,
			}),
		})

		// Verify response
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		content := response.Result.Get("content").AsString()
		// Should contain truncation message
		assert.Contains(t, content, "(Results are truncated. Consider using a more specific path or pattern.)")
	})
}

func TestSessionEditToolIntegration(t *testing.T) {
	fixture := newSweSystemFixture(t, "You are skilled software developer.")
	system := fixture.system
	vfsInstance := fixture.vfs

	// Setup test file in VFS
	err := vfsInstance.WriteFile("test.txt", []byte("hello world\ngoodbye world"))
	require.NoError(t, err)

	t.Run("edit tool replaces unique occurrence and returns diff", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Verify edit tool is registered
		editTool, err := session.Tools.Get("vfsEdit")
		require.NoError(t, err)
		require.NotNil(t, editTool)

		// Execute edit tool to replace "hello"
		response := editTool.Execute(&tool.ToolCall{
			ID:       "test-edit-1",
			Function: "vfsEdit",
			Arguments: tool.NewToolValue(map[string]any{
				"path":      "test.txt",
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Verify response
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify success message was returned
		content := response.Result.Get("content").AsString()
		assert.Equal(t, "Edit applied successfully", content)

		// Verify file was modified
		fileContent, err := vfsInstance.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "hi world\ngoodbye world", string(fileContent))
	})

	t.Run("edit tool replaces all occurrences when replaceAll is true", func(t *testing.T) {
		// Reset the file
		err := vfsInstance.WriteFile("test2.txt", []byte("foo bar foo baz"))
		require.NoError(t, err)

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err = controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Verify edit tool is registered
		editTool, err := session.Tools.Get("vfsEdit")
		require.NoError(t, err)
		require.NotNil(t, editTool)

		// Execute edit tool to replace all "foo"
		response := editTool.Execute(&tool.ToolCall{
			ID:       "test-edit-2",
			Function: "vfsEdit",
			Arguments: tool.NewToolValue(map[string]any{
				"path":       "test2.txt",
				"oldString":  "foo",
				"newString":  "qux",
				"replaceAll": true,
			}),
		})

		// Verify response
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify success message was returned
		content := response.Result.Get("content").AsString()
		assert.Equal(t, "Edit applied successfully", content)

		// Verify file was modified
		fileContent, err := vfsInstance.ReadFile("test2.txt")
		require.NoError(t, err)
		assert.Equal(t, "qux bar qux baz", string(fileContent))
	})

	t.Run("edit tool returns error when oldString not found", func(t *testing.T) {
		// Reset the file
		err := vfsInstance.WriteFile("test3.txt", []byte("hello world"))
		require.NoError(t, err)

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err = controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Execute edit tool with non-existent string
		editTool, err := session.Tools.Get("vfsEdit")
		require.NoError(t, err)

		response := editTool.Execute(&tool.ToolCall{
			ID:       "test-edit-3",
			Function: "vfsEdit",
			Arguments: tool.NewToolValue(map[string]any{
				"path":      "test3.txt",
				"oldString": "goodbye",
				"newString": "hi",
			}),
		})

		// Verify error
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "oldString not found")
	})

	t.Run("edit tool returns error when multiple matches without replaceAll", func(t *testing.T) {
		// Reset the file
		err := vfsInstance.WriteFile("test4.txt", []byte("hello world\nhello again"))
		require.NoError(t, err)

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err = controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Execute edit tool without replaceAll
		editTool, err := session.Tools.Get("vfsEdit")
		require.NoError(t, err)

		response := editTool.Execute(&tool.ToolCall{
			ID:       "test-edit-4",
			Function: "vfsEdit",
			Arguments: tool.NewToolValue(map[string]any{
				"path":      "test4.txt",
				"oldString": "hello",
				"newString": "hi",
			}),
		})

		// Verify error
		assert.Error(t, response.Error)
		assert.True(t, response.Done)
		assert.Contains(t, response.Error.Error(), "oldString found multiple times")
	})

	t.Run("edit tool handles multiline content correctly", func(t *testing.T) {
		// Setup multiline file
		multilineContent := "func main() {\n\tfmt.Println(\"hello\")\n}"
		err := vfsInstance.WriteFile("test5.go", []byte(multilineContent))
		require.NoError(t, err)

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err = controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Execute edit tool with multiline replacement
		editTool, err := session.Tools.Get("vfsEdit")
		require.NoError(t, err)

		response := editTool.Execute(&tool.ToolCall{
			ID:       "test-edit-5",
			Function: "vfsEdit",
			Arguments: tool.NewToolValue(map[string]any{
				"path":      "test5.go",
				"oldString": "func main() {\n\tfmt.Println(\"hello\")\n}",
				"newString": "func main() {\n\tfmt.Println(\"hi\")\n}",
			}),
		})

		// Verify response
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify success message was returned
		content := response.Result.Get("content").AsString()
		assert.Equal(t, "Edit applied successfully", content)

		// Verify file was modified
		fileContent, err := vfsInstance.ReadFile("test5.go")
		require.NoError(t, err)
		assert.Equal(t, "func main() {\n\tfmt.Println(\"hi\")\n}", string(fileContent))
	})
}
