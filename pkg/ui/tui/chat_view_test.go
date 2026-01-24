package tui

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatView_Creation(t *testing.T) {
	t.Run("NewChatView creates view with presenter", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

		require.NotNil(t, view)
		assert.NotNil(t, view.layout)
		assert.NotNil(t, view.markdownView)
		assert.NotNil(t, view.textArea)
		assert.Equal(t, presenter, view.presenter)
	})
}

func TestChatView_Init(t *testing.T) {
	t.Run("Init initializes view with session messages", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

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

		err := view.Init(session)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(view.messages))
	})

	t.Run("Init clears existing messages", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "First",
		}
		err := view.AddMessage(msg)
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
		assert.Equal(t, 1, len(view.messages))
		assert.Equal(t, "Second", view.messages[0].content)
	})
}

func TestChatView_AddMessage(t *testing.T) {
	t.Run("AddMessage adds new message to view", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Test message",
		}

		err := view.AddMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(view.messages))
		assert.Equal(t, "Test message", view.messages[0].content)
		assert.Equal(t, string(ui.ChatRoleUser), view.messages[0].role)
	})

	t.Run("AddMessage adds assistant message with tools", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

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

		err := view.AddMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(view.messages))
		assert.Equal(t, 1, len(view.messages[0].toolCalls))
		assert.Equal(t, "test_tool", view.messages[0].toolCalls[0].tool.Name)
	})

	t.Run("AddMessage adds multiple messages", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

		messages := []*ui.ChatMessageUI{
			{Id: "1", Role: ui.ChatRoleUser, Text: "First"},
			{Id: "2", Role: ui.ChatRoleAssistant, Text: "Second"},
			{Id: "3", Role: ui.ChatRoleUser, Text: "Third"},
		}

		for _, msg := range messages {
			err := view.AddMessage(msg)
			assert.NoError(t, err)
		}

		assert.Equal(t, 3, len(view.messages))
	})
}

func TestChatView_UpdateMessage(t *testing.T) {
	t.Run("UpdateMessage updates existing message", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

		partialMsg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "",
		}

		err := view.AddMessage(partialMsg)
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
		assert.Equal(t, "Updated content", view.messages[0].content)
		assert.Equal(t, 1, len(view.messages[0].toolCalls))
	})

	t.Run("UpdateMessage updates last message with matching role", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

		msg1 := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "First assistant",
		}

		err := view.AddMessage(msg1)
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
		assert.Equal(t, "First assistant", view.messages[0].content)
		assert.Equal(t, "Updated second message", view.messages[1].content)
	})
}

func TestChatView_UpdateTool(t *testing.T) {
	t.Run("UpdateTool updates tool state", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

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

		err := view.AddMessage(msg)
		assert.NoError(t, err)

		updatedTool := &ui.ToolUI{
			Id:     "tool-1",
			Status: ui.ToolStatusSucceeded,
			Name:   "test_tool",
		}

		err = view.UpdateTool(updatedTool)
		assert.NoError(t, err)
		assert.Equal(t, ui.ToolStatusSucceeded, view.messages[0].toolCalls[0].tool.Status)
	})

	t.Run("UpdateTool handles non-existent tool", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "Message",
		}

		err := view.AddMessage(msg)
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
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

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

		err := view.AddMessage(msg)
		assert.NoError(t, err)

		updatedTool := &ui.ToolUI{
			Id:     "tool-2",
			Status: ui.ToolStatusSucceeded,
			Name:   "tool_two",
		}

		err = view.UpdateTool(updatedTool)
		assert.NoError(t, err)
		assert.Equal(t, ui.ToolStatusStarted, view.messages[0].toolCalls[0].tool.Status)
		assert.Equal(t, ui.ToolStatusSucceeded, view.messages[0].toolCalls[1].tool.Status)
	})
}

func TestChatView_MoveToBottom(t *testing.T) {
	t.Run("MoveToBottom scrolls viewport to bottom", func(t *testing.T) {
		presenter := mock.NewMockChatPresenter()
		view := NewChatView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, presenter)

		err := view.MoveToBottom()
		assert.NoError(t, err)
	})
}
