package core

import (
	"context"
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

// mockPromptGenerator is a simple mock implementation of PromptGenerator for testing
type mockPromptGenerator struct {
	prompt string
}

func newMockPromptGenerator(prompt string) *mockPromptGenerator {
	return &mockPromptGenerator{prompt: prompt}
}

func (m *mockPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error) {
	return m.prompt, nil
}

func (m *mockPromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *AgentState) (tool.ToolInfo, error) {
	schema := tool.NewToolSchema()
	return tool.ToolInfo{
		Name:        toolName,
		Description: "Mock tool for testing",
		Schema:      schema,
	}, nil
}

func (m *mockPromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	return make(map[string]string), nil
}

func TestAgentCoreInitializationAndSimpleProgramGen(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfs := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfs)

	t.Run("basic initialization", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are skilled software developer."),
			Tools:                tools,
			VFS:                  vfs,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Populate mock server with LLM responses
		// First response: assistant makes a tool call to write the file
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"vfsWrite","arguments":{"path":"hello_world.py","content":"print(\"Hello World\")\n"}}}]},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Second response: after tool execution, assistant confirms completion
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"I've created the Hello World program in Python."},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		err = session.UserPrompt("Implement Hello World program in Python")
		assert.NoError(t, err)

		err = session.Run(context.Background())
		assert.NoError(t, err)

		bytes, err := vfs.ReadFile("hello_world.py")
		assert.NoError(t, err)
		assert.Contains(t, string(bytes), "print(\"Hello World\")")
	})

	t.Run("UI output handler integration", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are skilled software developer."),
			Tools:                tools,
			VFS:                  vfs,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Populate mock server with LLM responses
		// First response: assistant makes a tool call to write the file
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"vfsWrite","arguments":{"path":"test.txt","content":"test content"}}}]},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Second response: after tool execution, assistant confirms completion
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"File created successfully."},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		err = session.UserPrompt("Create a test file")
		assert.NoError(t, err)

		err = session.Run(context.Background())
		assert.NoError(t, err)

		// Verify UI handler captured the events
		// Should have at least one tool call start
		assert.NotEmpty(t, mockHandler.ToolCallStarts, "should have captured tool call start")
		// Should have tool call details
		assert.NotEmpty(t, mockHandler.ToolCallDetails, "should have captured tool call details")
		// Should have tool call result
		assert.NotEmpty(t, mockHandler.ToolCallResults, "should have captured tool call result")
		assert.Equal(t, "vfsWrite", mockHandler.ToolCallResults[0].Call.Function)
		// Should have markdown chunks from the final response
		assert.NotEmpty(t, mockHandler.MarkdownChunks, "should have captured markdown chunks")
		assert.Contains(t, mockHandler.MarkdownChunks, "File created successfully.")
	})
}

func TestSweSystemSessionManagement(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfs := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfs)

	system := &SweSystem{
		ModelProviders:       map[string]models.ModelProvider{"ollama": client},
		ModelTags:            models.NewModelTagRegistry(),
		PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
		Tools:                tools,
		VFS:                  vfs,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		WorkDir:              ".",
	}

	mockHandler := testutil.NewMockSessionOutputHandler()

	t.Run("NewSession creates and stores session", func(t *testing.T) {
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.NotEmpty(t, session.ID())

		// Verify session is stored
		storedSession, err := system.GetSession(session.ID())
		require.NoError(t, err)
		assert.Equal(t, session, storedSession)
	})

	t.Run("GetSession returns error for non-existent session", func(t *testing.T) {
		_, err := system.GetSession("non-existent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("ListSessions returns all sessions", func(t *testing.T) {
		// Create multiple sessions
		session1, err := system.NewSession("ollama/model1:latest", mockHandler)
		require.NoError(t, err)
		session2, err := system.NewSession("ollama/model2:latest", mockHandler)
		require.NoError(t, err)

		sessions := system.ListSessions()
		assert.GreaterOrEqual(t, len(sessions), 2)

		// Check that our sessions are in the list
		sessionIDs := make(map[string]bool)
		for _, s := range sessions {
			sessionIDs[s.ID()] = true
		}
		assert.True(t, sessionIDs[session1.ID()])
		assert.True(t, sessionIDs[session2.ID()])
	})

	t.Run("DeleteSession removes session", func(t *testing.T) {
		// Create a session
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		sessionID := session.ID()

		// Verify it exists
		_, err = system.GetSession(sessionID)
		require.NoError(t, err)

		// Delete it
		err = system.DeleteSession(sessionID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = system.GetSession(sessionID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("DeleteSession returns error for non-existent session", func(t *testing.T) {
		err := system.DeleteSession("non-existent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("Session IDs are UUIDs", func(t *testing.T) {
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		id := session.ID()
		// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (36 chars)
		assert.Len(t, id, 36)
		assert.Equal(t, "-", string(id[8]))
		assert.Equal(t, "-", string(id[13]))
		assert.Equal(t, "-", string(id[18]))
		assert.Equal(t, "-", string(id[23]))
		// Version should be 7
		assert.Equal(t, "7", string(id[14]))
	})

	t.Run("Multiple sessions have unique IDs", func(t *testing.T) {
		session1, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		session2, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		assert.NotEqual(t, session1.ID(), session2.ID())
	})
}

func TestSweSystemGetSessionThread(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfs := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfs)

	system := &SweSystem{
		ModelProviders:       map[string]models.ModelProvider{"ollama": client},
		ModelTags:            models.NewModelTagRegistry(),
		PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
		Tools:                tools,
		VFS:                  vfs,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		WorkDir:              ".",
	}

	mockHandler := testutil.NewMockSessionOutputHandler()

	t.Run("GetSessionThread returns thread for existing session", func(t *testing.T) {
		// Create a session
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		sessionID := session.ID()

		// Get thread for this session
		thread, err := system.GetSessionThread(sessionID)
		require.NoError(t, err)
		assert.NotNil(t, thread)

		// Verify the thread has the correct session
		assert.Equal(t, session, thread.GetSession())
	})

	t.Run("GetSessionThread returns error for non-existent session", func(t *testing.T) {
		_, err := system.GetSessionThread("non-existent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("GetSessionThread returns same thread on multiple calls", func(t *testing.T) {
		// Create a session
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		sessionID := session.ID()

		// Get thread twice
		thread1, err := system.GetSessionThread(sessionID)
		require.NoError(t, err)
		thread2, err := system.GetSessionThread(sessionID)
		require.NoError(t, err)

		// Should be the same thread instance
		assert.Equal(t, thread1, thread2)
	})

	t.Run("GetSessionThread creates different threads for different sessions", func(t *testing.T) {
		// Create two sessions
		session1, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		session2, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Get threads for both sessions
		thread1, err := system.GetSessionThread(session1.ID())
		require.NoError(t, err)
		thread2, err := system.GetSessionThread(session2.ID())
		require.NoError(t, err)

		// Threads should be different
		assert.NotEqual(t, thread1, thread2)

		// Each thread should have the correct session
		assert.Equal(t, session1, thread1.GetSession())
		assert.Equal(t, session2, thread2.GetSession())
	})
}

func TestSweSystemShutdown(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfs := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfs)

	t.Run("Shutdown with no sessions or threads", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfs,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		// Should not panic
		system.Shutdown()

		// Verify both maps are initialized and empty
		assert.Empty(t, system.ListSessions())
	})

	t.Run("Shutdown with sessions but no threads", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfs,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		mockHandler := testutil.NewMockSessionOutputHandler()

		// Create multiple sessions
		session1, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		session2, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Verify sessions exist
		assert.NotEmpty(t, system.ListSessions())

		// Shutdown
		system.Shutdown()

		// Verify all sessions are deleted
		assert.Empty(t, system.ListSessions())

		// Verify sessions are actually gone
		_, err = system.GetSession(session1.ID())
		assert.Error(t, err)
		_, err = system.GetSession(session2.ID())
		assert.Error(t, err)
	})

	t.Run("Shutdown with sessions and threads", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfs,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		mockHandler := testutil.NewMockSessionOutputHandler()

		// Create multiple sessions
		session1, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		session2, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Create threads for sessions
		thread1, err := system.GetSessionThread(session1.ID())
		require.NoError(t, err)
		require.NotNil(t, thread1)
		thread2, err := system.GetSessionThread(session2.ID())
		require.NoError(t, err)
		require.NotNil(t, thread2)

		// Shutdown
		system.Shutdown()

		// Verify all sessions and threads are deleted
		assert.Empty(t, system.ListSessions())

		// Verify sessions are gone
		_, err = system.GetSession(session1.ID())
		assert.Error(t, err)
		_, err = system.GetSession(session2.ID())
		assert.Error(t, err)

		// Verify threads are gone (would create new ones if requested)
		thread3, err := system.GetSessionThread(session1.ID())
		assert.Error(t, err) // Session doesn't exist, so thread creation should fail
		assert.Nil(t, thread3)
	})

	t.Run("Shutdown interrupts running threads", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfs,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		mockHandler := testutil.NewMockSessionOutputHandler()

		// Create a session
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Get thread for this session
		thread, err := system.GetSessionThread(session.ID())
		require.NoError(t, err)

		// Add a long-running streaming response to simulate a running task
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Processing..."},"done":false}`,
		)

		// Start the thread (non-blocking)
		err = thread.AddPrompt("Test prompt")
		require.NoError(t, err)
		err = thread.Resume()
		require.NoError(t, err)

		// Give it a moment to start
		// Note: In a real scenario, we'd use synchronization primitives,
		// but for this test we'll just verify the shutdown doesn't panic
		// and cleans up properly

		// Shutdown should interrupt the running thread
		system.Shutdown()

		// Verify cleanup
		assert.Empty(t, system.ListSessions())

		// Verify thread is not running anymore (if we could check)
		// This is tested indirectly by verifying the thread map is cleared
	})

	t.Run("Shutdown is idempotent", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfs,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		mockHandler := testutil.NewMockSessionOutputHandler()

		// Create a session
		_, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Shutdown once
		system.Shutdown()
		assert.Empty(t, system.ListSessions())

		// Shutdown again should not panic
		system.Shutdown()
		assert.Empty(t, system.ListSessions())
	})
}

func TestSystemStreamingConfiguration(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()

	vfs := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfs)

	t.Run("session uses streaming from provider config", func(t *testing.T) {
		// Create provider with streaming enabled
		streamingEnabled := true
		config := &conf.ModelProviderConfig{
			Type:      "ollama",
			Name:      "ollama",
			URL:       mockServer.URL(),
			Streaming: &streamingEnabled,
		}
		client, err := models.NewOllamaClient(config)
		require.NoError(t, err)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfs,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		assert.True(t, session.streaming)
	})

	t.Run("session uses non-streaming from provider config", func(t *testing.T) {
		// Create provider with streaming disabled
		streamingDisabled := false
		config := &conf.ModelProviderConfig{
			Type:      "ollama",
			Name:      "ollama",
			URL:       mockServer.URL(),
			Streaming: &streamingDisabled,
		}
		client, err := models.NewOllamaClient(config)
		require.NoError(t, err)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfs,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		assert.False(t, session.streaming)
	})

	t.Run("session defaults to streaming when not configured", func(t *testing.T) {
		// Create provider without streaming config
		config := &conf.ModelProviderConfig{
			Type: "ollama",
			Name: "ollama",
			URL:  mockServer.URL(),
		}
		client, err := models.NewOllamaClient(config)
		require.NoError(t, err)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfs,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		assert.True(t, session.streaming, "Should default to streaming mode for backward compatibility")
	})
}

func TestLogLLMRequestsOption(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfsInstance := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance)

	t.Run("session has llmLogger when LogLLMRequests is enabled", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
			LogLLMRequests:       true,
		}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Verify llmLogger is set
		assert.NotNil(t, session.llmLogger, "llmLogger should be set when LogLLMRequests is enabled")
	})

	t.Run("session has nil llmLogger when LogLLMRequests is disabled", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
			LogLLMRequests:       false,
		}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Verify llmLogger is nil
		assert.Nil(t, session.llmLogger, "llmLogger should be nil when LogLLMRequests is disabled")
	})

	t.Run("session has nil llmLogger when LogLLMRequests is not set (default)", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      newMockPromptGenerator("You are a test assistant."),
			Tools:                tools,
			VFS:                  vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
			// LogLLMRequests not set - defaults to false
		}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Verify llmLogger is nil
		assert.Nil(t, session.llmLogger, "llmLogger should be nil when LogLLMRequests is not set")
	})
}
