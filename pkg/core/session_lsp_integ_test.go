// session_lsp_integ_test.go contains integration tests for LSP (Language Server Protocol)
// functionality within sessions, including tool validation and diagnostics.
package core

import (
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		session.roles = roleRegistry

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
		session.roles = roleRegistry
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
		assert.Contains(t, content, "LSP errors detected in this file, please fix:")
		assert.Contains(t, content, "<diagnostics file=\"")
		assert.Contains(t, content, "Error[4:2] undefined: fmt")
		assert.Contains(t, content, "</diagnostics>")
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
		session.roles = roleRegistry
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

	t.Run("VFS patch tool uses LSP for validation", func(t *testing.T) {
		testPath := "patch.go"
		err := vfsInstance.WriteFile(testPath, []byte("package main\n\nfunc main() {}\n"))
		require.NoError(t, err)

		absPath, err := filepath.Abs(testPath)
		require.NoError(t, err)
		uri := "file://" + filepath.ToSlash(absPath)
		mockLSP.SetDiagnostics(uri, []lsp.Diagnostic{
			{
				Range: lsp.Range{
					Start: lsp.Position{Line: 2, Character: 16},
					End:   lsp.Position{Line: 2, Character: 19},
				},
				Severity: lsp.SeverityError,
				Message:  "undefined: bad",
			},
		})

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err = controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		testRoleConfig := conf.AgentRoleConfig{
			Name:        "test",
			Description: "Test role",
		}
		configStore := impl.NewMockConfigStore()
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"test": &testRoleConfig,
		})
		roleRegistry := NewAgentRoleRegistry(configStore)
		session.roles = roleRegistry
		err = session.SetRole("test")
		require.NoError(t, err)

		patchTool, err := session.Tools.Get("vfsPatch")
		require.NoError(t, err)

		response := patchTool.Execute(&tool.ToolCall{
			ID:       "test-patch-lsp",
			Function: "vfsPatch",
			Arguments: tool.NewToolValue(map[string]any{
				"patchText": "*** Begin Patch\n*** Update File: patch.go\n@@\n-func main() {}\n+func main() { bad() }\n*** End Patch",
			}),
		})

		assert.NoError(t, response.Error)
		assert.True(t, response.Done)
		content := response.Result.Get("content").AsString()
		assert.Contains(t, content, "Success. Updated the following files:")
		assert.Contains(t, content, "M patch.go")
		assert.Contains(t, content, "LSP errors detected in patch.go, please fix:")
		assert.Contains(t, content, "<diagnostics file=\"patch.go\">")
		assert.Contains(t, content, "Error[3:17] undefined: bad")
		assert.Contains(t, content, "</diagnostics>")
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
		session.roles = roleRegistry
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
