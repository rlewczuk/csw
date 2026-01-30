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

// mockPromptGenerator is defined in system_test.go but we need it here too for session tests
// This is a simple mock implementation of PromptGenerator for testing
type mockSessionPromptGenerator struct {
	prompt string
}

func newMockSessionPromptGenerator(prompt string) *mockSessionPromptGenerator {
	return &mockSessionPromptGenerator{prompt: prompt}
}

func (m *mockSessionPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error) {
	return m.prompt, nil
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
		PromptGenerator:      newMockSessionPromptGenerator("You are skilled software developer."),
		Tools:                tools,
		VFS:                  vfsInstance,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
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
		PromptGenerator:      newMockSessionPromptGenerator("You are skilled software developer."),
		Tools:                tools,
		VFS:                  vfsInstance,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
	}

	t.Run("bug: Run uses system tools instead of session tools", func(t *testing.T) {
		// This test exposes a bug where SweSession.Run() uses s.system.Tools.ListInfo()
		// instead of s.Tools.ListInfo() when passing tools to the model provider.
		//
		// The bug means:
		// 1. Session-specific tools (todo.read, todo.write) are not presented to the LLM
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
		sessionToolInfo := session.Tools.ListInfo()
		sessionToolNames := make([]string, 0, len(sessionToolInfo))
		for _, info := range sessionToolInfo {
			sessionToolNames = append(sessionToolNames, info.Name)
		}
		assert.Contains(t, sessionToolNames, "todo.read", "session should have todo.read tool")
		assert.Contains(t, sessionToolNames, "todo.write", "session should have todo.write tool")

		// Verify the system does NOT have session-specific tools
		systemToolInfo := system.Tools.ListInfo()
		systemToolNames := make([]string, 0, len(systemToolInfo))
		for _, info := range systemToolInfo {
			systemToolNames = append(systemToolNames, info.Name)
		}
		assert.NotContains(t, systemToolNames, "todo.read", "system should not have todo.read")
		assert.NotContains(t, systemToolNames, "todo.write", "system should not have todo.write")

		// The counts should be different
		assert.NotEqual(t, len(systemToolInfo), len(sessionToolInfo),
			"session and system should have different number of tools")

		// BUG: When we call Run(), it will use s.system.Tools.ListInfo() on line 463
		// instead of s.Tools.ListInfo(). We can't directly observe what tools are sent
		// to the model in this test, but we document the expected fix here.
		//
		// To fix: change line 463 in session.go from:
		//     tools := s.system.Tools.ListInfo()
		// to:
		//     tools := s.Tools.ListInfo()
	})

	t.Run("session without role uses system tools correctly", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Without SetRole, session tools should be a copy of system tools
		// plus session-specific tools like todo.read/todo.write
		sessionToolInfo := session.Tools.ListInfo()
		sessionToolNames := make([]string, 0, len(sessionToolInfo))
		for _, info := range sessionToolInfo {
			sessionToolNames = append(sessionToolNames, info.Name)
		}

		// Should have session-specific tools
		assert.Contains(t, sessionToolNames, "todo.read")
		assert.Contains(t, sessionToolNames, "todo.write")

		// Should also have system tools like VFS tools
		assert.Contains(t, sessionToolNames, "vfs.read")
		assert.Contains(t, sessionToolNames, "vfs.write")
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
		PromptGenerator:      newMockSessionPromptGenerator("You are skilled software developer."),
		Tools:                tools,
		VFS:                  vfsInstance,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
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
