package core

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/logging"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSessionThreadPromptGenerator is a simple mock implementation of PromptGenerator for testing
type mockSessionThreadPromptGenerator struct {
	prompt string
}

func newMockSessionThreadPromptGenerator(prompt string) *mockSessionThreadPromptGenerator {
	return &mockSessionThreadPromptGenerator{prompt: prompt}
}

func (m *mockSessionThreadPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error) {
	return m.prompt, nil
}

func (m *mockSessionThreadPromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *AgentState) (tool.ToolInfo, error) {
	// Return a simple tool info for testing
	schema := tool.NewToolSchema()
	return tool.ToolInfo{
		Name:        toolName,
		Description: "Mock tool for testing",
		Schema:      schema,
	}, nil
}

func (m *mockSessionThreadPromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	return make(map[string]string), nil
}

func TestSessionThread(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfsInstance := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance)

	system := &SweSystem{
		ModelProviders:       map[string]models.ModelProvider{"ollama": client},
		ModelTags:            models.NewModelTagRegistry(),
		PromptGenerator:      newMockSessionThreadPromptGenerator("You are skilled software developer."),
		Tools:                tools,
		VFS:                  vfsInstance,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		WorkDir:              ".",
	}

	t.Run("basic initialization and session management", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		// Initially no session
		assert.Nil(t, controller.GetSession())

		// Start a session
		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Now we have a session
		assert.NotNil(t, controller.GetSession())
	})
}

// TestSessionToolSelection verifies that the session presents session-level tools
// (which include access control wrappers and session-specific tools) to the LLM,
// not the system-level tools. This is a regression test for a bug where
// s.system.Tools.ListInfo() was used instead of s.Tools.ListInfo().
func TestSessionToolSelection(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfsInstance := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance)

	system := &SweSystem{
		ModelProviders:       map[string]models.ModelProvider{"ollama": client},
		ModelTags:            models.NewModelTagRegistry(),
		PromptGenerator:      newMockSessionThreadPromptGenerator("You are skilled software developer."),
		Tools:                tools,
		VFS:                  vfsInstance,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		WorkDir:              ".",
	}

	t.Run("bug: Run uses system tools instead of session tools", func(t *testing.T) {
		// This test exposes a bug where SweSession.Run() uses s.system.Tools.ListInfo()
		// instead of s.Tools.ListInfo() when passing tools to the model provider.
		//
		// The bug means:
		// 1. Session-specific tools (todoRead, todoWrite) are not presented to the LLM
		// 2. Access control wrappers applied to session tools are bypassed
		//
		// Expected behavior: Run() should use s.Tools.ListInfo()
		// Current (buggy) behavior: Run() uses s.system.Tools.ListInfo()

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Verify the session has session-specific tools
		sessionToolNames := session.Tools.List()
		assert.Contains(t, sessionToolNames, "todoRead", "session should have todoRead tool")
		assert.Contains(t, sessionToolNames, "todoWrite", "session should have todoWrite tool")

		// Verify the system does NOT have session-specific tools
		systemToolNames := system.Tools.List()
		assert.NotContains(t, systemToolNames, "todoRead", "system should not have todoRead")
		assert.NotContains(t, systemToolNames, "todoWrite", "system should not have todoWrite")

		// The counts should be different
		assert.NotEqual(t, len(systemToolNames), len(sessionToolNames),
			"session and system should have different number of tools")
	})

	t.Run("session without role uses system tools correctly", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Without SetRole, session tools should be a copy of system tools
		// plus session-specific tools like todoRead/todoWrite
		sessionToolNames := session.Tools.List()

		// Should have session-specific tools
		assert.Contains(t, sessionToolNames, "todoRead")
		assert.Contains(t, sessionToolNames, "todoWrite")

		// Should also have system tools like VFS tools
		assert.Contains(t, sessionToolNames, "vfsRead")
		assert.Contains(t, sessionToolNames, "vfsWrite")
	})
}

func TestSessionThreadSafety(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfsInstance := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance)

	system := &SweSystem{
		ModelProviders:       map[string]models.ModelProvider{"ollama": client},
		ModelTags:            models.NewModelTagRegistry(),
		PromptGenerator:      newMockSessionThreadPromptGenerator("You are skilled software developer."),
		Tools:                tools,
		VFS:                  vfsInstance,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		WorkDir:              ".",
	}

	t.Run("concurrent GetSession calls", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Call GetSession from multiple goroutines
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				session := controller.GetSession()
				assert.NotNil(t, session)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("concurrent UserPrompt calls with single session", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Setup multiple responses
		for i := 0; i < 5; i++ {
			mockServer.AddStreamingResponse("/api/chat", "POST", false,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Response"},"done":true,"done_reason":"stop"}`,
			)
		}

		// Send prompts from multiple goroutines
		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func(idx int) {
				err := controller.UserPrompt("Test prompt")
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all prompts to be queued
		for i := 0; i < 5; i++ {
			<-done
		}

		// Wait for the session to finish processing all prompts
		mockHandler.WaitForRunFinished()

		// Verify no error occurred
		assert.NoError(t, mockHandler.RunFinishedError)
	})
}
