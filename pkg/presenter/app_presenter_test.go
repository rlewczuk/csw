package presenter

import (
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppPresenter_SetView(t *testing.T) {
	system, _, _ := setupTestSystem(t)
	presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")

	mockView := mock.NewMockAppView()
	err := presenter.SetView(mockView)
	assert.NoError(t, err)

	// Verify view is set
	presenter.mu.Lock()
	assert.Equal(t, mockView, presenter.view)
	presenter.mu.Unlock()
}

func TestAppPresenter_NewSession(t *testing.T) {
	system, _, _ := setupTestSystem(t)

	t.Run("creates new session without view", func(t *testing.T) {
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")

		err := presenter.NewSession()
		assert.NoError(t, err)

		// Verify session was created
		sessions := system.ListSessions()
		assert.Len(t, sessions, 1)
	})

	t.Run("creates new session and shows chat view", func(t *testing.T) {
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")
		mockView := mock.NewMockAppView()
		err := presenter.SetView(mockView)
		require.NoError(t, err)

		err = presenter.NewSession()
		assert.NoError(t, err)

		// Verify ShowChat was called
		require.Len(t, mockView.ShowChatCalls, 1)
		chatPresenter := mockView.ShowChatCalls[0]
		assert.NotNil(t, chatPresenter)

		// Verify session was created
		sessions := system.ListSessions()
		assert.GreaterOrEqual(t, len(sessions), 1)
	})

	t.Run("creates session with correct model", func(t *testing.T) {
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")

		err := presenter.NewSession()
		require.NoError(t, err)

		sessions := system.ListSessions()
		require.NotEmpty(t, sessions)

		// Find the newly created session
		found := false
		for _, session := range sessions {
			if session.Model() == "devstral-small-2:latest" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have created session with correct model")
	})

	t.Run("returns error for invalid model", func(t *testing.T) {
		presenter := NewAppPresenter(system, "invalid/model:latest", "")

		err := presenter.NewSession()
		assert.Error(t, err)
	})

	t.Run("creates session with default role", func(t *testing.T) {
		// Create a fresh system for this test
		roleSystem, _, _ := setupTestSystem(t)

		// Setup role in system
		roleName := "test_role"
		testRole := conf.AgentRoleConfig{
			Name: roleName,
		}
		roleSystem.Roles = core.NewAgentRoleRegistry()
		roleSystem.Roles.Register(testRole)

		presenter := NewAppPresenter(roleSystem, "ollama/devstral-small-2:latest", roleName)

		err := presenter.NewSession()
		require.NoError(t, err)

		// Verify session was created with the role
		sessions := roleSystem.ListSessions()
		require.NotEmpty(t, sessions)

		// The session should have the role set
		foundSession := sessions[0]
		require.NotNil(t, foundSession, "should have created session")

		// Verify role was set
		require.NotNil(t, foundSession.Role(), "session role should not be nil")
		assert.Equal(t, roleName, foundSession.Role().Name)
	})

	t.Run("creates session without role when empty string", func(t *testing.T) {
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")

		err := presenter.NewSession()
		require.NoError(t, err)

		sessions := system.ListSessions()
		require.NotEmpty(t, sessions)

		// The last session should have no role set
		lastSession := sessions[len(sessions)-1]
		assert.Nil(t, lastSession.Role())
	})
}

func TestAppPresenter_OpenSession(t *testing.T) {
	system, _, _ := setupTestSystem(t)

	t.Run("reopens existing session", func(t *testing.T) {
		// Create a session first
		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)
		sessionID := session.ID()

		// Create presenter and open the session
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")
		mockView := mock.NewMockAppView()
		err = presenter.SetView(mockView)
		require.NoError(t, err)

		err = presenter.OpenSession(sessionID)
		assert.NoError(t, err)

		// Verify ShowChat was called
		require.Len(t, mockView.ShowChatCalls, 1)
		chatPresenter := mockView.ShowChatCalls[0]
		assert.NotNil(t, chatPresenter)
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")

		err := presenter.OpenSession("non-existent-session-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("reopens session without view", func(t *testing.T) {
		// Create a session first
		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)
		sessionID := session.ID()

		// Create presenter without setting view
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")

		err = presenter.OpenSession(sessionID)
		assert.NoError(t, err)
	})
}

func TestAppPresenter_Exit(t *testing.T) {
	system, _, _ := setupTestSystem(t)

	t.Run("calls shutdown on system", func(t *testing.T) {
		// Create some sessions
		mockHandler := testutil.NewMockSessionOutputHandler()
		_, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)
		_, err = system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)

		// Verify sessions exist
		sessions := system.ListSessions()
		assert.GreaterOrEqual(t, len(sessions), 2)

		// Exit
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")
		err = presenter.Exit()
		assert.NoError(t, err)

		// Verify all sessions are deleted
		sessions = system.ListSessions()
		assert.Empty(t, sessions)
	})
}

func TestAppPresenter_Integration(t *testing.T) {
	system, mockServer, vfsInstance := setupTestSystem(t)

	t.Run("create session and send message", func(t *testing.T) {
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")
		mockView := mock.NewMockAppView()
		err := presenter.SetView(mockView)
		require.NoError(t, err)

		// Create new session
		err = presenter.NewSession()
		require.NoError(t, err)

		// Get the chat presenter
		require.Len(t, mockView.ShowChatCalls, 1)
		chatPresenter := mockView.ShowChatCalls[0]

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello! I'm ready to help."},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","done":true,"done_reason":"stop"}`,
		)

		// Send message
		userMsg := &ui.ChatMessageUI{
			Id:   "user-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}
		err = chatPresenter.SendUserMessage(userMsg)
		require.NoError(t, err)

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		// Verify messages were added to view
		chatView := mockView.ShowChatCalls[0].(*ChatPresenter).view
		if mockChatView, ok := chatView.(*mock.MockChatView); ok {
			assert.GreaterOrEqual(t, len(mockChatView.AddMessageCalls), 2)
		}
	})

	t.Run("create session with tool call", func(t *testing.T) {
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")
		mockView := mock.NewMockAppView()
		err := presenter.SetView(mockView)
		require.NoError(t, err)

		// Create new session
		err = presenter.NewSession()
		require.NoError(t, err)

		// Get the chat presenter
		require.Len(t, mockView.ShowChatCalls, 1)
		chatPresenter := mockView.ShowChatCalls[0]

		// Setup LLM responses with tool call
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"vfs.write","arguments":{"path":"test.txt","content":"Test content"}}}]},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","done":true,"done_reason":"stop"}`,
		)

		// Second response after tool execution
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"File created."},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:03Z","done":true,"done_reason":"stop"}`,
		)

		// Send message
		userMsg := &ui.ChatMessageUI{
			Id:   "user-1",
			Role: ui.ChatRoleUser,
			Text: "Create a test file",
		}
		err = chatPresenter.SendUserMessage(userMsg)
		require.NoError(t, err)

		// Wait for processing
		time.Sleep(150 * time.Millisecond)

		// Verify file was created
		content, err := vfsInstance.ReadFile("test.txt")
		assert.NoError(t, err)
		assert.Contains(t, string(content), "Test content")
	})

	t.Run("reopen session and continue conversation", func(t *testing.T) {
		presenter := NewAppPresenter(system, "ollama/devstral-small-2:latest", "")
		mockView := mock.NewMockAppView()
		err := presenter.SetView(mockView)
		require.NoError(t, err)

		// Create new session and send first message
		err = presenter.NewSession()
		require.NoError(t, err)

		chatPresenter1 := mockView.ShowChatCalls[0]

		// Setup first LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"First response"},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","done":true,"done_reason":"stop"}`,
		)

		userMsg1 := &ui.ChatMessageUI{
			Id:   "user-1",
			Role: ui.ChatRoleUser,
			Text: "First message",
		}
		err = chatPresenter1.SendUserMessage(userMsg1)
		require.NoError(t, err)

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		// Get session ID
		sessions := system.ListSessions()
		require.NotEmpty(t, sessions)
		sessionID := sessions[len(sessions)-1].ID()

		// Reset mock view
		mockView.Reset()

		// Reopen session
		err = presenter.OpenSession(sessionID)
		require.NoError(t, err)

		// Verify chat was shown
		require.Len(t, mockView.ShowChatCalls, 1)
		chatPresenter2 := mockView.ShowChatCalls[0]

		// Setup second LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"Second response"},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:03Z","done":true,"done_reason":"stop"}`,
		)

		// Send second message
		userMsg2 := &ui.ChatMessageUI{
			Id:   "user-2",
			Role: ui.ChatRoleUser,
			Text: "Second message",
		}
		err = chatPresenter2.SendUserMessage(userMsg2)
		require.NoError(t, err)

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		// Verify the session has both messages in history
		session, err := system.GetSession(sessionID)
		require.NoError(t, err)
		messages := session.ChatMessages()

		// Should have: system prompt, first user msg, first assistant msg, second user msg, second assistant msg
		assert.GreaterOrEqual(t, len(messages), 4)
	})
}
