package core

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/codesnort/codesnort-swe/pkg/lsp"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
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

		// Verify diff was returned
		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "```diff")
		assert.Contains(t, content, "-hello world")
		assert.Contains(t, content, "+hi world")

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

		// Verify diff was returned
		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "```diff")

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

		// Verify diff was returned
		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "```diff")

		// Verify file was modified
		fileContent, err := vfsInstance.ReadFile("test5.go")
		require.NoError(t, err)
		assert.Equal(t, "func main() {\n\tfmt.Println(\"hi\")\n}", string(fileContent))
	})
}

func TestSessionLSPIntegration(t *testing.T) {
	// Create mock LSP
	mockLSP, err := lsp.NewMockLSP(".")
	require.NoError(t, err)
	err = mockLSP.Init(true)
	require.NoError(t, err)
	fixture := newSweSystemFixture(t, "You are skilled software developer.", withLSP(mockLSP))
	system := fixture.system
	vfsInstance := fixture.vfs

	t.Run("session receives LSP from system", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()
		assert.NotNil(t, session.LSP, "session should receive LSP from system")
		assert.Equal(t, mockLSP, session.LSP, "session LSP should be the same as system LSP")
	})

	t.Run("session passes LSP to VFS tools when setting role", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Create a test role registry with a minimal role
		testRoleConfig := conf.AgentRoleConfig{
			Name:        "test",
			Description: "Test role",
		}
		configStore := impl.NewMockConfigStore()
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"test": &testRoleConfig,
		})
		roleRegistry := NewAgentRoleRegistry(configStore)
		system.Roles = roleRegistry

		// Set role to trigger tool re-registration
		err = session.SetRole("test")
		require.NoError(t, err)

		// Verify VFS tools are registered
		editTool, err := session.Tools.Get("vfsEdit")
		require.NoError(t, err)
		assert.NotNil(t, editTool)

		writeTool, err := session.Tools.Get("vfsWrite")
		require.NoError(t, err)
		assert.NotNil(t, writeTool)
	})

	t.Run("VFS edit tool uses LSP for validation", func(t *testing.T) {
		// Setup test file in VFS
		testPath := "test.go"
		err := vfsInstance.WriteFile(testPath, []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"))
		require.NoError(t, err)

		// Setup mock LSP to return diagnostics using the same path format that pathToURI will use
		// MockVFS uses current directory as working directory, so we need to get absolute path
		absPath, err := filepath.Abs(testPath)
		require.NoError(t, err)
		uri := "file://" + filepath.ToSlash(absPath)
		mockLSP.SetDiagnostics(uri, []lsp.Diagnostic{
			{
				Range: lsp.Range{
					Start: lsp.Position{Line: 3, Character: 1},
					End:   lsp.Position{Line: 3, Character: 4},
				},
				Severity: lsp.SeverityError,
				Message:  "undefined: fmt",
			},
		})

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err = controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Create a minimal role to trigger tool registration with LSP
		testRoleConfig := conf.AgentRoleConfig{
			Name:        "test",
			Description: "Test role",
		}
		configStore := impl.NewMockConfigStore()
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"test": &testRoleConfig,
		})
		roleRegistry := NewAgentRoleRegistry(configStore)
		system.Roles = roleRegistry
		err = session.SetRole("test")
		require.NoError(t, err)

		// Get edit tool
		editTool, err := session.Tools.Get("vfsEdit")
		require.NoError(t, err)

		// Execute edit
		response := editTool.Execute(&tool.ToolCall{
			ID:       "test-edit-lsp",
			Function: "vfsEdit",
			Arguments: tool.NewToolValue(map[string]any{
				"path":      "test.go",
				"oldString": "\"hello\"",
				"newString": "\"hi\"",
			}),
		})

		// Verify response
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify LSP diagnostics are included in the result
		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "LSP validation found issues")
		assert.Contains(t, content, "Error [4:2] undefined: fmt")
	})

	t.Run("VFS write tool uses LSP for validation", func(t *testing.T) {
		// Setup mock LSP to return diagnostics for new file
		testPath := "new.go"
		absPath2, err2 := filepath.Abs(testPath)
		require.NoError(t, err2)
		uri := "file://" + filepath.ToSlash(absPath2)
		mockLSP.SetDiagnostics(uri, []lsp.Diagnostic{
			{
				Range: lsp.Range{
					Start: lsp.Position{Line: 0, Character: 0},
					End:   lsp.Position{Line: 0, Character: 7},
				},
				Severity: lsp.SeverityError,
				Message:  "expected 'package', found 'EOF'",
			},
		})

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Create a minimal role to trigger tool registration with LSP
		testRoleConfig := conf.AgentRoleConfig{
			Name:        "test",
			Description: "Test role",
		}
		configStore := impl.NewMockConfigStore()
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"test": &testRoleConfig,
		})
		roleRegistry := NewAgentRoleRegistry(configStore)
		system.Roles = roleRegistry
		err = session.SetRole("test")
		require.NoError(t, err)

		// Get write tool
		writeTool, err := session.Tools.Get("vfsWrite")
		require.NoError(t, err)

		// Execute write with invalid content
		response := writeTool.Execute(&tool.ToolCall{
			ID:       "test-write-lsp",
			Function: "vfsWrite",
			Arguments: tool.NewToolValue(map[string]any{
				"path":    "new.go",
				"content": "// incomplete file",
			}),
		})

		// Verify response
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)

		// Verify LSP diagnostics are included in the result
		validation := response.Result.Get("validation").AsString()
		assert.Contains(t, validation, "LSP validation found issues")
		assert.Contains(t, validation, "Error [1:1] expected 'package', found 'EOF'")
	})

	t.Run("session works without LSP when LSP is nil", func(t *testing.T) {
		fixtureNoLSP := newSweSystemFixture(t, "You are skilled software developer.")
		systemNoLSP := fixtureNoLSP.system
		vfsInstance := fixtureNoLSP.vfs

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(systemNoLSP, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()
		assert.Nil(t, session.LSP, "session LSP should be nil when system LSP is nil")

		// Create a minimal role to trigger tool registration
		testRoleConfig := conf.AgentRoleConfig{
			Name:        "test",
			Description: "Test role",
		}
		configStore := impl.NewMockConfigStore()
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"test": &testRoleConfig,
		})
		roleRegistry := NewAgentRoleRegistry(configStore)
		systemNoLSP.Roles = roleRegistry
		err = session.SetRole("test")
		require.NoError(t, err)

		// Write a test file
		err = vfsInstance.WriteFile("no-lsp.go", []byte("package main\n\nfunc main() {}"))
		require.NoError(t, err)

		// Execute edit without LSP
		editTool, err := session.Tools.Get("vfsEdit")
		require.NoError(t, err)

		response := editTool.Execute(&tool.ToolCall{
			ID:       "test-edit-no-lsp",
			Function: "vfsEdit",
			Arguments: tool.NewToolValue(map[string]any{
				"path":      "no-lsp.go",
				"oldString": "func main() {}",
				"newString": "func main() { fmt.Println(\"test\") }",
			}),
		})

		// Should succeed without LSP validation
		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		content := response.Result.Get("content").AsString()
		// Should not contain LSP validation messages
		assert.NotContains(t, content, "LSP validation")
	})
}

func TestSessionStreamingModeDefault(t *testing.T) {
	fixture := newSweSystemFixture(t, "You are skilled software developer.")
	system := fixture.system

	// Create provider config WITHOUT streaming field (should default to true)
	config := &conf.ModelProviderConfig{
		Type: "ollama",
		Name: "ollama",
		URL:  fixture.server.URL(),
	}

	client, err := models.NewOllamaClient(config)
	require.NoError(t, err)
	system.ModelProviders = map[string]models.ModelProvider{"ollama": client}

	t.Run("streaming defaults to true", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Verify streaming mode defaults to true
		assert.True(t, session.streaming)
	})
}

// TestSessionSystemPrompt tests that system prompt is correctly set when creating
// a session with default role and when changing roles.
func TestSessionSystemPrompt(t *testing.T) {
	// Create mock config store with roles
	configStore := impl.NewMockConfigStore()
	developerRole := &conf.AgentRoleConfig{
		Name:        "developer",
		Description: "Software developer role",
	}
	testerRole := &conf.AgentRoleConfig{
		Name:        "tester",
		Description: "QA tester role",
	}
	configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
		"developer": developerRole,
		"tester":    testerRole,
	})

	roleRegistry := NewAgentRoleRegistry(configStore)
	fixture := newSweSystemFixture(t, "You are a skilled software developer.", withConfigStore(configStore), withRoles(roleRegistry))
	system := fixture.system

	t.Run("system prompt is set when creating session with default role", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Verify role was set
		assert.NotNil(t, session.role)
		assert.Equal(t, "developer", session.role.Name)

		// Verify system prompt was added to messages
		require.Greater(t, len(session.messages), 0, "session should have at least one message (system prompt)")
		assert.Equal(t, models.ChatRoleSystem, session.messages[0].Role, "first message should be system prompt")

		// Verify the system prompt content
		systemPrompt := session.messages[0].GetText()
		assert.Contains(t, systemPrompt, "You are a skilled software developer.", "system prompt should contain the role description")
	})

	t.Run("system prompt is updated when changing role", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)

		// Verify initial role and system prompt
		assert.Equal(t, "developer", session.role.Name)
		require.Greater(t, len(session.messages), 0)
		assert.Equal(t, models.ChatRoleSystem, session.messages[0].Role)
		initialPrompt := session.messages[0].GetText()

		// Add a user message to simulate conversation
		session.UserPrompt("Hello")

		// Change role to tester
		err = session.SetRole("tester")
		require.NoError(t, err)

		// Verify role was changed
		assert.Equal(t, "tester", session.role.Name)

		// Verify system prompt is still the first message
		require.Greater(t, len(session.messages), 0)
		assert.Equal(t, models.ChatRoleSystem, session.messages[0].Role, "first message should still be system prompt after role change")

		// Verify system prompt was updated (should be the same since our mock returns same prompt)
		newPrompt := session.messages[0].GetText()
		assert.Equal(t, initialPrompt, newPrompt, "system prompt should be maintained when changing role")

		// Verify user message is still there
		require.Greater(t, len(session.messages), 1, "user message should still exist after role change")
		assert.Equal(t, models.ChatRoleUser, session.messages[1].Role)
	})

	t.Run("system prompt persists when setting same role twice", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)

		// Verify initial role and system prompt
		assert.Equal(t, "developer", session.role.Name)
		require.Greater(t, len(session.messages), 0)
		assert.Equal(t, models.ChatRoleSystem, session.messages[0].Role)
		initialPrompt := session.messages[0].GetText()

		// Set the same role again (this happens in CLI)
		err = session.SetRole("developer")
		require.NoError(t, err)

		// Verify role is still set
		assert.Equal(t, "developer", session.role.Name)

		// Verify system prompt is still there
		require.Greater(t, len(session.messages), 0, "system prompt should still exist")
		assert.Equal(t, models.ChatRoleSystem, session.messages[0].Role, "first message should still be system prompt")

		// Verify system prompt content hasn't changed
		newPrompt := session.messages[0].GetText()
		assert.Equal(t, initialPrompt, newPrompt, "system prompt should be the same")
	})
}

func TestSessionLLMLoggerUsage(t *testing.T) {
	t.Run("runNonStreamingChat uses llmLogger when enabled", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a test assistant.", withLogLLMRequests(true))
		system := fixture.system
		mockServer := fixture.server

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Verify llmLogger is set
		assert.NotNil(t, session.llmLogger, "llmLogger should be set")

		// Setup non-streaming response
		mockServer.AddRestResponse("/api/chat", "POST", `{"model":"test-model:latest","message":{"role":"assistant","content":"Hello!"},"done":true}`)

		// Set session to non-streaming mode
		session.streaming = false

		// Add a user message
		session.UserPrompt("Hi there")

		// Run the session - this should use llmLogger in runNonStreamingChat
		err = session.Run(t.Context())
		require.NoError(t, err)

		// Verify the response was processed
		assert.Contains(t, mockHandler.MarkdownChunks, "Hello!")
	})

	t.Run("runStreamingChat uses llmLogger when enabled", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a test assistant.", withLogLLMRequests(true))
		system := fixture.system
		mockServer := fixture.server

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Verify llmLogger is set
		assert.NotNil(t, session.llmLogger, "llmLogger should be set")

		// Setup streaming response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"!"},"done":true,"done_reason":"stop"}`,
		)

		// Ensure session is in streaming mode
		session.streaming = true

		// Add a user message
		session.UserPrompt("Hi there")

		// Run the session - this should use llmLogger in runStreamingChat
		err = session.Run(t.Context())
		require.NoError(t, err)

		// Verify the response was processed
		assert.Contains(t, mockHandler.MarkdownChunks, "Hello")
		assert.Contains(t, mockHandler.MarkdownChunks, "!")
	})

	t.Run("runNonStreamingChat works without llmLogger", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a test assistant.", withLogLLMRequests(false))
		system := fixture.system
		mockServer := fixture.server

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Verify llmLogger is nil
		assert.Nil(t, session.llmLogger, "llmLogger should be nil")

		// Setup non-streaming response
		mockServer.AddRestResponse("/api/chat", "POST", `{"model":"test-model:latest","message":{"role":"assistant","content":"Hello!"},"done":true}`)

		// Set session to non-streaming mode
		session.streaming = false

		// Add a user message
		session.UserPrompt("Hi there")

		// Run the session - this should work without llmLogger
		err = session.Run(t.Context())
		require.NoError(t, err)

		// Verify the response was processed
		assert.Contains(t, mockHandler.MarkdownChunks, "Hello!")
	})

	t.Run("runStreamingChat works without llmLogger", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a test assistant.", withLogLLMRequests(false))
		system := fixture.system
		mockServer := fixture.server

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Verify llmLogger is nil
		assert.Nil(t, session.llmLogger, "llmLogger should be nil")

		// Setup streaming response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"!"},"done":true,"done_reason":"stop"}`,
		)

		// Ensure session is in streaming mode
		session.streaming = true

		// Add a user message
		session.UserPrompt("Hi there")

		// Run the session - this should work without llmLogger
		err = session.Run(t.Context())
		require.NoError(t, err)

		// Verify the response was processed
		assert.Contains(t, mockHandler.MarkdownChunks, "Hello")
		assert.Contains(t, mockHandler.MarkdownChunks, "!")
	})
}

// TestPermissionLoopBug tests that when a tool permission is automatically allowed,
// the session does not fall into an infinite loop of asking for permission.
// This is a regression test for a bug where granting permission would cause
// the same tool call to be executed repeatedly.
func TestPermissionLoopBug(t *testing.T) {
	// Create a local VFS
	localVFS, err := vfs.NewLocalVFS(t.TempDir(), nil)
	require.NoError(t, err)

	// Wrap with access control that asks for all permissions
	accessConfig := map[string]conf.FileAccess{
		"*": {
			Read:  conf.AccessAsk,
			Write: conf.AccessAsk,
		},
	}
	restrictedVFS := vfs.NewAccessControlVFS(localVFS, accessConfig)

	// Create a role with tool access set to "ask"
	configStore := impl.NewMockConfigStore()
	testRole := &conf.AgentRoleConfig{
		Name:        "test",
		Description: "Test role with ask permissions",
		ToolsAccess: map[string]conf.AccessFlag{
			"**": conf.AccessAsk, // Ask for all tools
		},
	}
	configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
		"test": testRole,
	})
	roleRegistry := NewAgentRoleRegistry(configStore)
	fixture := newSweSystemFixture(t, "You are a helpful assistant.", withVFS(restrictedVFS), withRoles(roleRegistry), withConfigStore(configStore), withWorkDir(t.TempDir()))
	system := fixture.system
	mockServer := fixture.server

	t.Run("permission allow should not cause infinite loop", func(t *testing.T) {
		// Set up mock streaming response with tool call
		// First response: assistant makes a tool call to vfsRead
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"vfsRead","arguments":{"path":"test.txt"}}}]},"done":false}`,
			`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Second response: after permission is granted and tool executes, assistant responds
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"I have read the file."},"done":false}`,
			`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create a mock handler that automatically allows permissions
		mockHandler := &autoAllowPermissionHandler{
			MockSessionOutputHandler: testutil.NewMockSessionOutputHandler(),
		}

		// Create thread
		thread := NewSessionThread(system, mockHandler)
		mockHandler.SetThread(thread)

		// Start session
		err := thread.StartSession("ollama/test-model")
		require.NoError(t, err)

		session := thread.GetSession()
		require.NotNil(t, session)

		// Set the role to enable access control
		err = session.SetRole("test")
		require.NoError(t, err)

		// Send user prompt through the thread (this starts processing)
		err = thread.UserPrompt("Read the file test.txt")
		require.NoError(t, err)

		// Wait for the thread to finish processing with timeout
		done := make(chan struct{})
		go func() {
			for {
				if !thread.IsRunning() {
					close(done)
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()

		select {
		case <-done:
			// Success - thread completed
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out - session fell into infinite permission loop")
		}

		// Verify that permission was queried exactly twice (once for tool access, once for VFS access)
		// The bug was that it would loop infinitely, so we expect exactly 2, not more
		assert.Equal(t, 2, mockHandler.PermissionQueryCount, "Permission should be queried exactly twice (tool access + VFS access), not in an infinite loop")

		// Verify that the tool was actually executed (tool result should be present)
		assert.GreaterOrEqual(t, len(mockHandler.ToolCallResults), 1, "Tool should have been executed and produced a result")
	})
}

// autoAllowPermissionHandler is a mock handler that automatically allows permissions
type autoAllowPermissionHandler struct {
	*testutil.MockSessionOutputHandler
	PermissionQueryCount int
	thread               *SessionThread
}

func (h *autoAllowPermissionHandler) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	h.PermissionQueryCount++
	h.MockSessionOutputHandler.OnPermissionQuery(query)

	// Automatically allow the permission
	if h.thread != nil {
		h.thread.PermissionResponse("Allow")
	}
}

func (h *autoAllowPermissionHandler) SetThread(thread *SessionThread) {
	h.thread = thread
}
