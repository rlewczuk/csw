package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
)

func TestTuiChatView(t *testing.T) {
	t.Run("NewTuiChatView creates view with presenter", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)
		assert.NotNil(t, view)
		assert.NotNil(t, view.Model())
	})

	t.Run("Init initializes view with session messages", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		session := &ui.ChatSessionUI{
			Id:      "test-session",
			Model:   "test-model",
			Role:    "assistant",
			WorkDir: "/test/dir",
			Messages: []*ui.ChatMessageUI{
				{
					Id:   "msg-1",
					Role: ui.ChatRoleUser,
					Text: "Hello",
				},
				{
					Id:   "msg-2",
					Role: ui.ChatRoleAssistant,
					Text: "Hi there!",
				},
			},
		}

		err = view.Init(session)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(view.model.messages))
	})

	t.Run("Init clears existing messages", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "First",
		}
		err = view.AddMessage(msg)
		assert.NoError(t, err)

		session := &ui.ChatSessionUI{
			Messages: []*ui.ChatMessageUI{
				{
					Id:   "msg-2",
					Role: ui.ChatRoleUser,
					Text: "Second",
				},
			},
		}

		err = view.Init(session)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(view.model.messages))
		assert.Equal(t, "Second", view.model.messages[0].content)
	})

	t.Run("AddMessage adds new message to view", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Test message",
		}

		err = view.AddMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(view.model.messages))
		assert.Equal(t, "Test message", view.model.messages[0].content)
		assert.Equal(t, string(ui.ChatRoleUser), view.model.messages[0].role)
	})

	t.Run("AddMessage adds assistant message with tools", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "Assistant response",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool-1",
					Status: ui.ToolStatusStarted,
					Name:   "test_tool",
				},
			},
		}

		err = view.AddMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(view.model.messages))
		assert.Equal(t, 1, len(view.model.messages[0].toolCalls))
		assert.Equal(t, "test_tool", view.model.messages[0].toolCalls[0].tool.Name)
	})

	t.Run("AddMessage adds multiple messages", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		messages := []*ui.ChatMessageUI{
			{Id: "1", Role: ui.ChatRoleUser, Text: "First"},
			{Id: "2", Role: ui.ChatRoleAssistant, Text: "Second"},
			{Id: "3", Role: ui.ChatRoleUser, Text: "Third"},
		}

		for _, msg := range messages {
			err = view.AddMessage(msg)
			assert.NoError(t, err)
		}

		assert.Equal(t, 3, len(view.model.messages))
	})

	t.Run("UpdateMessage updates existing message", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		partialMsg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "",
		}

		err = view.AddMessage(partialMsg)
		assert.NoError(t, err)

		updatedMsg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "Updated content",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool-1",
					Status: ui.ToolStatusStarted,
					Name:   "test_tool",
					Props:  [][]string{{"arg1", "value1"}},
				},
			},
		}

		err = view.UpdateMessage(updatedMsg)
		assert.NoError(t, err)
		assert.Equal(t, "Updated content", view.model.messages[0].content)
		assert.Equal(t, 1, len(view.model.messages[0].toolCalls))
	})

	t.Run("UpdateMessage updates last message with matching role", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		msg1 := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "First assistant",
		}

		err = view.AddMessage(msg1)
		assert.NoError(t, err)

		partialMsg := &ui.ChatMessageUI{
			Id:   "msg-2",
			Role: ui.ChatRoleAssistant,
			Text: "",
		}

		err = view.AddMessage(partialMsg)
		assert.NoError(t, err)

		updatedMsg := &ui.ChatMessageUI{
			Id:   "msg-2",
			Role: ui.ChatRoleAssistant,
			Text: "Updated second message",
		}

		err = view.UpdateMessage(updatedMsg)
		assert.NoError(t, err)
		assert.Equal(t, "First assistant", view.model.messages[0].content)
		assert.Equal(t, "Updated second message", view.model.messages[1].content)
	})

	t.Run("UpdateTool updates tool state", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "Message with tool",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool-1",
					Status: ui.ToolStatusStarted,
					Name:   "test_tool",
				},
			},
		}

		err = view.AddMessage(msg)
		assert.NoError(t, err)

		updatedTool := &ui.ToolUI{
			Id:     "tool-1",
			Status: ui.ToolStatusSucceeded,
			Name:   "test_tool",
		}

		err = view.UpdateTool(updatedTool)
		assert.NoError(t, err)
		assert.Equal(t, ui.ToolStatusSucceeded, view.model.messages[0].toolCalls[0].tool.Status)
	})

	t.Run("UpdateTool handles non-existent tool", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "Message",
		}

		err = view.AddMessage(msg)
		assert.NoError(t, err)

		updatedTool := &ui.ToolUI{
			Id:     "non-existent",
			Status: ui.ToolStatusSucceeded,
			Name:   "test_tool",
		}

		err = view.UpdateTool(updatedTool)
		assert.NoError(t, err)
	})

	t.Run("UpdateTool updates tool in message with multiple tools", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "Message with tools",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool-1",
					Status: ui.ToolStatusStarted,
					Name:   "tool_one",
				},
				{
					Id:     "tool-2",
					Status: ui.ToolStatusStarted,
					Name:   "tool_two",
				},
			},
		}

		err = view.AddMessage(msg)
		assert.NoError(t, err)

		updatedTool := &ui.ToolUI{
			Id:     "tool-2",
			Status: ui.ToolStatusSucceeded,
			Name:   "tool_two",
		}

		err = view.UpdateTool(updatedTool)
		assert.NoError(t, err)
		assert.Equal(t, ui.ToolStatusStarted, view.model.messages[0].toolCalls[0].tool.Status)
		assert.Equal(t, ui.ToolStatusSucceeded, view.model.messages[0].toolCalls[1].tool.Status)
	})

	t.Run("MoveToBottom scrolls viewport to bottom", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		err = view.MoveToBottom()
		assert.NoError(t, err)
	})
}

func TestTuiChatViewBubbletea(t *testing.T) {
	t.Run("Model returns bubbletea model", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		model := view.Model()
		assert.NotNil(t, model)

		cmd := model.Init()
		assert.NotNil(t, cmd)
	})

	t.Run("Model handles WindowSizeMsg", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		model := view.Model()

		model.Update(tea.WindowSizeMsg{
			Width:  100,
			Height: 50,
		})

		assert.Equal(t, 100, view.model.width)
		assert.Equal(t, 50, view.model.height)
	})

	t.Run("Model handles quit keys", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		model := view.Model()

		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		assert.NotNil(t, cmd)

		_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Nil(t, cmd)
	})

	t.Run("QueryPermission shows permission widget", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		query := &ui.PermissionQueryUI{
			Id:      "test-query",
			Title:   "Test Permission",
			Details: "Do you want to proceed?",
			Options: []string{"Yes", "No"},
		}

		err = view.QueryPermission(query)
		assert.NoError(t, err)
		assert.True(t, view.model.showingPermission)
		assert.NotNil(t, view.model.permissionWidget)
	})

	t.Run("QueryPermission replaces input box in view", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		query := &ui.PermissionQueryUI{
			Id:      "test-query",
			Title:   "Test Permission",
			Details: "Do you want to proceed?",
			Options: []string{"Yes", "No"},
		}

		err = view.QueryPermission(query)
		assert.NoError(t, err)

		viewStr := view.model.View()
		assert.Contains(t, viewStr, "Test Permission")
	})

	t.Run("QueryPermission callback restores input box", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		query := &ui.PermissionQueryUI{
			Id:      "test-query",
			Title:   "Test Permission",
			Details: "Do you want to proceed?",
			Options: []string{"Yes", "No"},
		}

		err = view.QueryPermission(query)
		assert.NoError(t, err)

		// Trigger the callback by selecting an option
		// Simulate pressing enter to select first option
		view.model.Update(tea.KeyMsg{Type: tea.KeyEnter})

		assert.False(t, view.model.showingPermission)
		assert.Nil(t, view.model.permissionWidget)
	})

	t.Run("QueryPermission callback calls PermissionResponse", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		query := &ui.PermissionQueryUI{
			Id:      "test-query",
			Title:   "Test Permission",
			Details: "Do you want to proceed?",
			Options: []string{"Yes", "No"},
		}

		err = view.QueryPermission(query)
		assert.NoError(t, err)

		// Simulate pressing enter to select first option (Yes)
		view.model.Update(tea.KeyMsg{Type: tea.KeyEnter})

		// Check that PermissionResponse was called
		assert.Equal(t, 1, len(presenter.PermissionResponseCalls))
		assert.Equal(t, "Yes", presenter.PermissionResponseCalls[0])
	})

	t.Run("QueryPermission routes keys to widget when showing", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		query := &ui.PermissionQueryUI{
			Id:      "test-query",
			Title:   "Test Permission",
			Details: "Do you want to proceed?",
			Options: []string{"Yes", "No"},
		}

		err = view.QueryPermission(query)
		assert.NoError(t, err)

		// Widget should start with cursor at 0
		assert.Equal(t, 0, view.model.permissionWidget.cursor)

		// Simulate pressing down key
		view.model.Update(tea.KeyMsg{Type: tea.KeyDown})

		// Widget cursor should have moved
		assert.Equal(t, 1, view.model.permissionWidget.cursor)
	})

	t.Run("QueryPermission overlays on bottom keeping chat visible", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view, err := NewTuiChatView(presenter)
		assert.NoError(t, err)

		// Set window size
		view.model.Update(tea.WindowSizeMsg{
			Width:  80,
			Height: 24,
		})

		// Record initial viewport height (before permission query)
		initialViewportHeight := view.model.viewport.Height

		// Add some chat messages so there's content to verify visibility
		err = view.AddMessage(&ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "User message that should remain visible",
		})
		assert.NoError(t, err)

		err = view.AddMessage(&ui.ChatMessageUI{
			Id:   "msg-2",
			Role: ui.ChatRoleAssistant,
			Text: "Assistant response that should remain visible",
		})
		assert.NoError(t, err)

		// Query permission
		query := &ui.PermissionQueryUI{
			Id:      "test-query",
			Title:   "Test Permission",
			Details: "Do you want to proceed?",
			Options: []string{"Yes", "No"},
		}

		err = view.QueryPermission(query)
		assert.NoError(t, err)

		// Get the rendered view AFTER permission query
		viewAfterPermission := view.model.View()

		// Verify chat messages are still visible in the view
		assert.Contains(t, viewAfterPermission, "User message that should remain visible", "Chat messages should be visible above permission query")
		assert.Contains(t, viewAfterPermission, "Assistant response that should remain visible", "Chat messages should be visible above permission query")

		// Verify permission query is shown
		assert.Contains(t, viewAfterPermission, "Test Permission", "Permission query should be visible")
		assert.Contains(t, viewAfterPermission, "Do you want to proceed?", "Permission query details should be visible")

		// Verify input box is NOT shown (should be replaced by permission widget)
		assert.NotContains(t, viewAfterPermission, "Type your message here...", "Input box placeholder should not be visible when permission query is shown")

		// Verify viewport content is present (chat messages)
		assert.True(t, len(view.model.viewport.View()) > 0, "Viewport should contain chat messages")

		// Verify the permission widget appears AFTER the viewport (at bottom)
		// by checking the position of chat content vs permission content in the view string
		chatPos := strings.Index(viewAfterPermission, "User message that should remain visible")
		permissionPos := strings.Index(viewAfterPermission, "Test Permission")
		assert.Less(t, chatPos, permissionPos, "Chat messages should appear before permission query in the view (permission at bottom)")

		// CRITICAL: Verify viewport height was adjusted to account for permission widget being larger than input box
		// The viewport should be reduced in height to make room for the permission widget
		// This ensures chat content doesn't get scrolled up and hidden
		viewportHeightAfterPermission := view.model.viewport.Height

		// Permission widget is taller than the textarea (3 lines), so viewport should be smaller
		// to maintain the same total view height
		assert.Less(t, viewportHeightAfterPermission, initialViewportHeight,
			"Viewport height should be reduced when permission query is shown to prevent chat content from scrolling off screen")
	})
}
