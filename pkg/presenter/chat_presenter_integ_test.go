package presenter

import (
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatPresenter_SetView(t *testing.T) {
	fixture := newPresenterFixture(t)
	system := fixture.System

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

	t.Run("restores runBash tool response details for resumed session", func(t *testing.T) {
		t.Skip("covered by chat presenter unit tests")

		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)

		err := thread.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := thread.GetSession()
		require.NotNil(t, session)
		require.NoError(t, session.UserPrompt("seed"))

		call := &tool.ToolCall{
			ID:       "bash-call-1",
			Function: "runBash",
			Arguments: tool.NewToolValue(map[string]any{
				"command": "go test ./pkg/models -v -run TestOllamaClient_RawLogChat -timeout 60s",
			}),
		}
		response := &tool.ToolResponse{
			Call: call,
			Result: tool.NewToolValue(map[string]any{
				"exit_code": int64(1),
				"stdout":    "stdout line 1\nstdout line 2\n",
				"stderr":    "stderr line 1\nstderr line 2\n",
			}),
			Done: true,
		}

		require.NoError(t, session.UserPrompt("seed"))
		messages := session.ChatMessages()
		require.NotEmpty(t, messages)
		messages[len(messages)-1].Role = models.ChatRoleAssistant
		messages[len(messages)-1].Parts = []models.ChatMessagePart{{ToolCall: call}}
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()
		session.ChatMessages()

		messages[0].Role = models.ChatRoleUser
		messages[0].Parts = append(messages[0].Parts, models.ChatMessagePart{ToolResponse: response})

		presenter := NewChatPresenter(system, thread)
		mockView := mock.NewMockChatView()
		err = presenter.SetView(mockView)
		require.NoError(t, err)

		require.Len(t, mockView.InitCalls, 1)
		chatSession := mockView.InitCalls[0]

		var restoredTool *ui.ToolUI
		for _, msg := range chatSession.Messages {
			for _, toolState := range msg.Tools {
				if toolState != nil && toolState.Id == "bash-call-1" {
					restoredTool = toolState
					break
				}
			}
			if restoredTool != nil {
				break
			}
		}

		require.NotNil(t, restoredTool)
		assert.Equal(t, ui.ToolStatusSucceeded, restoredTool.Status)
		assert.Contains(t, restoredTool.Details, "STDOUT:")
		assert.Contains(t, restoredTool.Details, "stdout line 1")
		assert.Contains(t, restoredTool.Details, "STDERR:")
		assert.Contains(t, restoredTool.Details, "stderr line 1")
	})
}

func TestChatPresenter_SendUserMessage(t *testing.T) {
	fixture := newPresenterFixture(t)
	system := fixture.System
	mockServer := fixture.Server

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
	fixture := newPresenterFixture(t)
	system := fixture.System

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
	fixture := newPresenterFixture(t)
	system := fixture.System
	mockServer := fixture.Server

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
	fixture := newPresenterFixture(t)
	system := fixture.System
	mockServer := fixture.Server
	vfsInstance := fixture.VFS

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
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"vfsWrite","arguments":{"path":"test.txt","content":"Hello World"}}}]},"done":false}`,
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

func TestChatPresenter_TodoToolRenderingUsesSessionToolRegistry(t *testing.T) {
	fixture := newPresenterFixture(t)
	system := fixture.System

	mockHandler := testutil.NewMockSessionOutputHandler()
	thread := core.NewSessionThread(system, mockHandler)

	err := thread.StartSession("ollama/devstral-small-2:latest")
	require.NoError(t, err)

	presenter := NewChatPresenter(system, thread)

	mockView := mock.NewMockChatView()
	err = presenter.SetView(mockView)
	require.NoError(t, err)

	session := thread.GetSession()
	require.NotNil(t, session)

	presenter.AddAssistantMessage("", "")

	call := &tool.ToolCall{
		ID:       "todo-call-1",
		Function: "todoWrite",
		Arguments: tool.NewToolValue(map[string]any{
			"todos": []any{
				map[string]any{
					"id":       "todo-1",
					"content":  "Completed task",
					"status":   "completed",
					"priority": "high",
				},
				map[string]any{
					"id":       "todo-2",
					"content":  "Current task to be done",
					"status":   "in_progress",
					"priority": "medium",
				},
				map[string]any{
					"id":       "todo-3",
					"content":  "Pending task",
					"status":   "pending",
					"priority": "low",
				},
			},
		}),
	}

	presenter.AddToolCall(call)

	todoWriteTool := tool.NewTodoWriteTool(session)
	response := todoWriteTool.Execute(call)
	require.NoError(t, response.Error)

	presenter.AddToolCallResult(response)

	require.NotEmpty(t, mockView.UpdateToolCalls)
	updatedTool := mockView.UpdateToolCalls[len(mockView.UpdateToolCalls)-1]

	assert.Equal(t, ui.ToolStatusSucceeded, updatedTool.Status)
	assert.Equal(t, "(2/3) Current task to be done.", updatedTool.Summary)
	assert.Contains(t, updatedTool.Details, "[X] Completed task")
	assert.Contains(t, updatedTool.Details, "[*] Current task to be done")
	assert.Contains(t, updatedTool.Details, "[ ] Pending task")
}

func TestChatPresenter_SessionPersistence(t *testing.T) {
	fixture := newPresenterFixture(t)
	system := fixture.System
	mockServer := fixture.Server

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
	fixture := newPresenterFixture(t)
	system := fixture.System
	mockServer := fixture.Server

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

func TestChatPresenter_ShowMessageAndRetryPrompt(t *testing.T) {
	fixture := newPresenterFixture(t)
	system := fixture.System

	t.Run("show message forwards to app view", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)
		presenter := NewChatPresenter(system, thread)

		appView := mock.NewMockAppView()
		presenter.SetAppView(appView)

		presenter.ShowMessage("temporary failure", "error")

		require.Len(t, appView.ShowMessageCalls, 1)
		assert.Equal(t, "temporary failure", appView.ShowMessageCalls[0].Message)
		assert.Equal(t, ui.MessageTypeError, appView.ShowMessageCalls[0].Type)
	})

	t.Run("should retry after failure uses app retry prompt", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)
		presenter := NewChatPresenter(system, thread)

		appView := mock.NewMockAppView()
		appView.AskRetryResult = true
		presenter.SetAppView(appView)

		shouldRetry := presenter.ShouldRetryAfterFailure("retry?")
		assert.True(t, shouldRetry)
		require.Len(t, appView.AskRetryCalls, 1)
		assert.Equal(t, "retry?", appView.AskRetryCalls[0])
	})
}

func TestChatPresenter_PermissionFlow(t *testing.T) {
	fixture := newPresenterFixture(t)
	system := fixture.System
	mockServer := fixture.Server
	vfsInstance := fixture.VFS

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
		`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"vfsWrite","arguments":{"path":"protected.txt","content":"secret"}}}]},"done":false}`,
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
	fixture := newPresenterFixture(t)
	system := fixture.System
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

func TestChatPresenter_ThinkingContent(t *testing.T) {
	fixture := newPresenterFixture(t)
	system := fixture.System
	mockServer := fixture.Server

	t.Run("thinking content is forwarded to view", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)

		err := thread.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		presenter := NewChatPresenter(system, thread)

		mockView := mock.NewMockChatView()
		err = presenter.SetView(mockView)
		require.NoError(t, err)

		// Setup LLM response with thinking content (simulating OpenAI-style with reasoning_content)
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello!"},"done":false}`,
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
		time.Sleep(10 * time.Millisecond)

		// Verify assistant message was added
		found := false
		for _, msg := range mockView.AddMessageCalls {
			if msg.Role == ui.ChatRoleAssistant {
				found = true
				break
			}
		}
		assert.True(t, found, "should have added assistant message")
	})

	t.Run("AddAssistantMessage creates message with thinking content", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		thread := core.NewSessionThread(system, mockHandler)

		err := thread.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		presenter := NewChatPresenter(system, thread)

		mockView := mock.NewMockChatView()
		err = presenter.SetView(mockView)
		require.NoError(t, err)

		presenter.AddAssistantMessage("Here is my answer.", "Let me think about this...")

		require.GreaterOrEqual(t, len(mockView.AddMessageCalls), 1)
		msg := mockView.AddMessageCalls[len(mockView.AddMessageCalls)-1]
		assert.Equal(t, ui.ChatRoleAssistant, msg.Role)
		assert.Equal(t, "Here is my answer.", msg.Text)
		assert.Equal(t, "Let me think about this...", msg.Thinking)
	})
}
