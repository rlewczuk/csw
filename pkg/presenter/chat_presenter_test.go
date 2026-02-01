package presenter

import (
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/logging"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/ui/mock"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock prompt generator for tests
type mockPromptGen struct{}

func (m *mockPromptGen) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *core.AgentState) (string, error) {
	return "You are skilled software developer.", nil
}

func (m *mockPromptGen) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *core.AgentState) (tool.ToolInfo, error) {
	schema := tool.NewToolSchema()
	return tool.ToolInfo{
		Name:        toolName,
		Description: "Mock tool for testing",
		Schema:      schema,
	}, nil
}

func (m *mockPromptGen) GetAgentFiles(dir string) (map[string]string, error) {
	return make(map[string]string), nil
}

func setupTestSystem(t *testing.T) (*core.SweSystem, *testutil.MockHTTPServer, vfs.VFS) {
	mockServer := testutil.NewMockHTTPServer()
	t.Cleanup(func() { mockServer.Close() })

	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)

	vfsInstance := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance)

	system := &core.SweSystem{
		ModelProviders:       map[string]models.ModelProvider{"ollama": client},
		ModelTags:            models.NewModelTagRegistry(),
		PromptGenerator:      &mockPromptGen{},
		Tools:                tools,
		VFS:                  vfsInstance,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
	}

	return system, mockServer, vfsInstance
}

func TestChatPresenter_SetView(t *testing.T) {
	system, _, _ := setupTestSystem(t)

	t.Run("set view without session", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)
		presenter := NewChatPresenter(system, thread)

		mockView := mock.NewMockChatView()
		err := presenter.SetView(mockView)
		assert.NoError(t, err)

		// Init should not be called without a session
		assert.Empty(t, mockView.InitCalls)
	})

	t.Run("set view with existing session", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)

		err := thread.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		presenter := NewChatPresenter(system, thread)

		mockView := mock.NewMockChatView()
		err = presenter.SetView(mockView)
		assert.NoError(t, err)

		// Init should be called with session data
		require.Len(t, mockView.InitCalls, 1)
		assert.NotEmpty(t, mockView.InitCalls[0].Id)
		assert.Equal(t, "devstral-small-2:latest", mockView.InitCalls[0].Model)
	})
}

func TestChatPresenter_SendUserMessage(t *testing.T) {
	system, mockServer, _ := setupTestSystem(t)

	t.Run("send message and receive response", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)

		err := thread.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		presenter := NewChatPresenter(system, thread)

		mockView := mock.NewMockChatView()
		err = presenter.SetView(mockView)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello! How can I help you?"},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","done":true,"done_reason":"stop"}`,
		)

		// Send user message
		userMsg := &ui.ChatMessageUI{
			Id:   "user-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}
		err = presenter.SendUserMessage(userMsg)
		assert.NoError(t, err)

		// Wait for processing to complete
		time.Sleep(5 * time.Millisecond)

		// Verify user message was added to view
		assert.GreaterOrEqual(t, len(mockView.AddMessageCalls), 1)
		assert.Equal(t, "Hello", mockView.AddMessageCalls[0].Text)
		assert.Equal(t, ui.ChatRoleUser, mockView.AddMessageCalls[0].Role)

		// Verify assistant message was added
		require.GreaterOrEqual(t, len(mockView.AddMessageCalls), 2)
		found := false
		for _, msg := range mockView.AddMessageCalls {
			if msg.Role == ui.ChatRoleAssistant {
				found = true
				break
			}
		}
		assert.True(t, found, "should have added assistant message")
	})
}

func TestChatPresenter_SaveUserMessage(t *testing.T) {
	system, _, _ := setupTestSystem(t)

	t.Run("save message without processing", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)

		err := thread.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		presenter := NewChatPresenter(system, thread)

		mockView := mock.NewMockChatView()
		err = presenter.SetView(mockView)
		require.NoError(t, err)

		// Save user message (should not start processing)
		userMsg := &ui.ChatMessageUI{
			Id:   "user-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}
		err = presenter.SaveUserMessage(userMsg)
		assert.NoError(t, err)

		// Wait a bit to ensure no processing started
		time.Sleep(5 * time.Millisecond)

		// Verify user message was added to view
		require.Len(t, mockView.AddMessageCalls, 1)
		assert.Equal(t, "Hello", mockView.AddMessageCalls[0].Text)

		// Verify no assistant message (processing didn't start)
		assert.False(t, thread.IsRunning())
	})
}

func TestChatPresenter_PauseResume(t *testing.T) {
	system, mockServer, _ := setupTestSystem(t)

	t.Run("pause and resume processing", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)

		err := thread.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		presenter := NewChatPresenter(system, thread)

		mockView := mock.NewMockChatView()
		err = presenter.SetView(mockView)
		require.NoError(t, err)

		// Save message without processing
		userMsg := &ui.ChatMessageUI{
			Id:   "user-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}
		err = presenter.SaveUserMessage(userMsg)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello! How can I help you?"},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","done":true,"done_reason":"stop"}`,
		)

		// Verify not running initially
		assert.False(t, thread.IsRunning())

		// Resume to start processing
		err = presenter.Resume()
		assert.NoError(t, err)

		// Wait for processing to complete
		time.Sleep(10 * time.Millisecond)

		// Verify assistant message was added
		found := false
		for _, msg := range mockView.AddMessageCalls {
			if msg.Role == ui.ChatRoleAssistant {
				found = true
				break
			}
		}
		assert.True(t, found, "should have added assistant message after resume")
	})
}

func TestChatPresenter_ToolCallHandling(t *testing.T) {
	system, mockServer, vfsInstance := setupTestSystem(t)

	t.Run("tool call updates are propagated to view", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)

		err := thread.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		presenter := NewChatPresenter(system, thread)

		mockView := mock.NewMockChatView()
		err = presenter.SetView(mockView)
		require.NoError(t, err)

		// Setup LLM responses with tool call
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"vfs.write","arguments":{"path":"test.txt","content":"Hello World"}}}]},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","done":true,"done_reason":"stop"}`,
		)

		// Second response after tool execution
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"File created."},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:03Z","done":true,"done_reason":"stop"}`,
		)

		// Send user message
		userMsg := &ui.ChatMessageUI{
			Id:   "user-1",
			Role: ui.ChatRoleUser,
			Text: "Create a test file",
		}
		err = presenter.SendUserMessage(userMsg)
		assert.NoError(t, err)

		// Wait for processing to complete
		time.Sleep(15 * time.Millisecond)

		// Verify tool updates were sent to view
		assert.NotEmpty(t, mockView.UpdateToolCalls, "should have tool update calls")

		// Verify file was created
		content, err := vfsInstance.ReadFile("test.txt")
		assert.NoError(t, err)
		assert.Contains(t, string(content), "Hello World")
	})
}

func TestChatPresenter_SessionPersistence(t *testing.T) {
	system, mockServer, _ := setupTestSystem(t)

	t.Run("session state persists across presenter instances", func(t *testing.T) {
		// Create first presenter and send a message
		mockHandler1 := testutil.NewMockSessionOutputHandler()
		thread1 := core.NewSessionThread(system, mockHandler1)

		err := thread1.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		presenter1 := NewChatPresenter(system, thread1)
		mockView1 := mock.NewMockChatView()
		err = presenter1.SetView(mockView1)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"First response"},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","done":true,"done_reason":"stop"}`,
		)

		// Send message via first presenter
		userMsg := &ui.ChatMessageUI{
			Id:   "user-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}
		err = presenter1.SendUserMessage(userMsg)
		require.NoError(t, err)

		// Wait for processing
		time.Sleep(10 * time.Millisecond)

		// Get the session
		session := thread1.GetSession()
		require.NotNil(t, session)

		// Create second presenter with same session
		mockHandler2 := testutil.NewMockSessionOutputHandler()
		thread2 := core.NewSessionThreadWithSession(system, session, mockHandler2)
		presenter2 := NewChatPresenter(system, thread2)

		mockView2 := mock.NewMockChatView()
		err = presenter2.SetView(mockView2)
		require.NoError(t, err)

		// Verify the second view received the existing messages
		require.Len(t, mockView2.InitCalls, 1)
		chatSession := mockView2.InitCalls[0]

		// Should have user message and assistant message (system prompt is filtered)
		assert.GreaterOrEqual(t, len(chatSession.Messages), 2)

		// Find user message
		userFound := false
		assistantFound := false
		for _, msg := range chatSession.Messages {
			if msg.Role == ui.ChatRoleUser && msg.Text == "Hello" {
				userFound = true
			}
			if msg.Role == ui.ChatRoleAssistant && msg.Text == "First response" {
				assistantFound = true
			}
		}
		assert.True(t, userFound, "should have user message in session")
		assert.True(t, assistantFound, "should have assistant message in session")
	})
}

func TestChatPresenter_MoveToBottom(t *testing.T) {
	system, mockServer, _ := setupTestSystem(t)

	t.Run("view scrolls to bottom on new content", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)

		err := thread.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		presenter := NewChatPresenter(system, thread)

		mockView := mock.NewMockChatView()
		err = presenter.SetView(mockView)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello! How can I help you?"},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","done":true,"done_reason":"stop"}`,
		)

		// Send message
		userMsg := &ui.ChatMessageUI{
			Id:   "user-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}
		err = presenter.SendUserMessage(userMsg)
		require.NoError(t, err)

		// Wait for processing
		time.Sleep(10 * time.Millisecond)

		// Verify MoveToBottom was called
		assert.Greater(t, mockView.MoveToBottomCalls, 0, "should have called MoveToBottom")
	})
}

func TestChatPresenter_PermissionFlow(t *testing.T) {
	system, mockServer, vfsInstance := setupTestSystem(t)

	// Define a role with VFS permission required
	roleName := "restricted_role"
	restrictedRole := &conf.AgentRoleConfig{
		Name: roleName,
		VFSPrivileges: map[string]conf.FileAccess{
			"**": {Read: conf.AccessAsk, Write: conf.AccessAsk},
		},
	}
	mockStore := impl.NewMockConfigStore()
	mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
		roleName: restrictedRole,
	})
	system.Roles = core.NewAgentRoleRegistry(mockStore)

	mockHandler := testutil.NewMockSessionOutputHandler()
	thread := core.NewSessionThread(system, mockHandler)

	err := thread.StartSession("ollama/devstral-small-2:latest")
	require.NoError(t, err)

	session := thread.GetSession()
	err = session.SetRole(roleName)
	require.NoError(t, err)

	presenter := NewChatPresenter(system, thread)
	mockView := mock.NewMockChatView()
	err = presenter.SetView(mockView)
	require.NoError(t, err)

	// Mock response: Assistant tries to write file
	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"vfs.write","arguments":{"path":"protected.txt","content":"secret"}}}]},"done":false}`,
		`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Send user message
	userMsg := &ui.ChatMessageUI{
		Id:   "user-1",
		Role: ui.ChatRoleUser,
		Text: "Write secret",
	}
	err = presenter.SendUserMessage(userMsg)
	assert.NoError(t, err)

	// Wait for permission query to appear in view
	require.Eventually(t, func() bool {
		return len(mockView.QueryPermissionCalls) > 0
	}, 2*time.Second, 5*time.Millisecond, "should receive permission query")

	// Verify query details
	query := mockView.QueryPermissionCalls[0]
	assert.Contains(t, query.Title, "Permission Required")
	assert.Contains(t, query.Details, "protected.txt")

	// Verify session is paused
	assert.True(t, thread.IsPaused())

	// Respond with Allow
	err = presenter.PermissionResponse("Allow")
	assert.NoError(t, err)

	// Wait for processing to complete
	require.Eventually(t, func() bool {
		return !thread.IsPaused() && !thread.IsRunning()
	}, 2*time.Second, 5*time.Millisecond, "session should resume and finish")

	// Verify file was created
	bytes, err := vfsInstance.ReadFile("protected.txt")
	assert.NoError(t, err)
	assert.Equal(t, "secret", string(bytes))
}

func TestChatPresenter_SetModel(t *testing.T) {
	system, _, _ := setupTestSystem(t)
	// Add another provider
	mockServer2 := testutil.NewMockHTTPServer()
	t.Cleanup(func() { mockServer2.Close() })

	client2, err := models.NewOllamaClientWithHTTPClient(mockServer2.URL(), mockServer2.Client())
	require.NoError(t, err)
	system.ModelProviders["other"] = client2

	mockHandler := testutil.NewMockSessionOutputHandler()
	thread := core.NewSessionThread(system, mockHandler)
	err = thread.StartSession("ollama/initial-model")
	require.NoError(t, err)

	presenter := NewChatPresenter(system, thread)

	t.Run("switch model successfully", func(t *testing.T) {
		err := presenter.SetModel("other/new-model")
		assert.NoError(t, err)

		session := thread.GetSession()
		assert.Equal(t, "new-model", session.Model())
	})

	t.Run("switch model invalid format", func(t *testing.T) {
		err := presenter.SetModel("invalid-format")
		assert.Error(t, err)
	})

	t.Run("switch model unknown provider", func(t *testing.T) {
		err := presenter.SetModel("unknown/model")
		assert.Error(t, err)
	})
}
