package logmd

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogmdChatView(t *testing.T) {
	t.Run("creates new LogmdChatView with correct dependencies", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}

		view := NewLogmdChatView(wrapped, &buf, mu)

		assert.NotNil(t, view)
		assert.Implements(t, (*ui.IChatView)(nil), view)
	})
}

func TestLogmdChatView_Init(t *testing.T) {
	t.Run("delegates to wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		session := &ui.ChatSessionUI{
			Id:      "session-1",
			Model:   "gpt-4",
			Role:    "assistant",
			WorkDir: "/tmp/test",
		}

		err := view.Init(session)

		require.NoError(t, err)
		assert.Len(t, wrapped.InitCalls, 1)
		assert.Equal(t, session, wrapped.InitCalls[0])
	})

	t.Run("returns error from wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		wrapped.InitErr = errors.New("init error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		session := &ui.ChatSessionUI{
			Id:      "session-1",
			Model:   "gpt-4",
			Role:    "assistant",
			WorkDir: "/tmp/test",
		}

		err := view.Init(session)

		assert.Error(t, err)
		assert.Equal(t, "init error", err.Error())
	})

	t.Run("writes session header to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		session := &ui.ChatSessionUI{
			Id:      "session-1",
			Model:   "gpt-4",
			Role:    "assistant",
			WorkDir: "/tmp/test",
		}

		err := view.Init(session)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "# Chat Session")
		assert.Contains(t, output, "**Session ID:** session-1")
		assert.Contains(t, output, "**Model:** gpt-4")
		assert.Contains(t, output, "**Role:** assistant")
		assert.Contains(t, output, "**Working Directory:** /tmp/test")
	})

	t.Run("writes existing messages to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		session := &ui.ChatSessionUI{
			Id:      "session-1",
			Model:   "gpt-4",
			Role:    "assistant",
			WorkDir: "/tmp/test",
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

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## User")
		assert.Contains(t, output, "Hello")
		assert.Contains(t, output, "## Assistant")
		assert.Contains(t, output, "Hi there!")
	})
}

func TestLogmdChatView_AddMessage(t *testing.T) {
	t.Run("delegates to wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}

		err := view.AddMessage(msg)

		require.NoError(t, err)
		assert.Len(t, wrapped.AddMessageCalls, 1)
		assert.Equal(t, msg, wrapped.AddMessageCalls[0])
	})

	t.Run("returns error from wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		wrapped.AddMessageErr = errors.New("add error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}

		err := view.AddMessage(msg)

		assert.Error(t, err)
		assert.Equal(t, "add error", err.Error())
	})

	t.Run("writes message to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Hello, world!",
		}

		err := view.AddMessage(msg)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## User")
		assert.Contains(t, output, "Hello, world!")
	})

	t.Run("writes message with tools to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "I'll help you with that",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool-1",
					Name:    "read_file",
					Status:  ui.ToolStatusStarted,
					Message: "Reading file",
				},
				{
					Id:      "tool-2",
					Name:    "write_file",
					Status:  ui.ToolStatusSucceeded,
					Message: "File written",
					Props:   [][]string{{"path", "/tmp/test.txt"}},
				},
			},
		}

		err := view.AddMessage(msg)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## Assistant")
		assert.Contains(t, output, "I'll help you with that")
		assert.Contains(t, output, "**Tools:**")
		assert.Contains(t, output, "**read_file** (started): Reading file")
		assert.Contains(t, output, "**write_file** (succeeded): File written")
	})

	t.Run("writes assistant message to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleAssistant,
			Text: "This is the assistant response",
		}

		err := view.AddMessage(msg)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## Assistant")
		assert.Contains(t, output, "This is the assistant response")
	})
}

func TestLogmdChatView_UpdateMessage(t *testing.T) {
	t.Run("delegates to wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Updated text",
		}

		err := view.UpdateMessage(msg)

		require.NoError(t, err)
		assert.Len(t, wrapped.UpdateMessageCalls, 1)
		assert.Equal(t, msg, wrapped.UpdateMessageCalls[0])
	})

	t.Run("returns error from wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		wrapped.UpdateMessageErr = errors.New("update error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Updated text",
		}

		err := view.UpdateMessage(msg)

		assert.Error(t, err)
		assert.Equal(t, "update error", err.Error())
	})

	t.Run("does not write to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Updated text",
		}

		err := view.UpdateMessage(msg)

		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})
}

func TestLogmdChatView_UpdateTool(t *testing.T) {
	t.Run("delegates to wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		tool := &ui.ToolUI{
			Id:      "tool-1",
			Name:    "read_file",
			Status:  ui.ToolStatusSucceeded,
			Message: "Done",
		}

		err := view.UpdateTool(tool)

		require.NoError(t, err)
		assert.Len(t, wrapped.UpdateToolCalls, 1)
		assert.Equal(t, tool, wrapped.UpdateToolCalls[0])
	})

	t.Run("returns error from wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		wrapped.UpdateToolErr = errors.New("update tool error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		tool := &ui.ToolUI{
			Id:      "tool-1",
			Name:    "read_file",
			Status:  ui.ToolStatusSucceeded,
			Message: "Done",
		}

		err := view.UpdateTool(tool)

		assert.Error(t, err)
		assert.Equal(t, "update tool error", err.Error())
	})

	t.Run("does not write to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		tool := &ui.ToolUI{
			Id:      "tool-1",
			Name:    "read_file",
			Status:  ui.ToolStatusSucceeded,
			Message: "Done",
		}

		err := view.UpdateTool(tool)

		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})
}

func TestLogmdChatView_MoveToBottom(t *testing.T) {
	t.Run("delegates to wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		err := view.MoveToBottom()

		require.NoError(t, err)
		assert.Equal(t, 1, wrapped.MoveToBottomCalls)
	})

	t.Run("returns error from wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		wrapped.MoveToBottomErr = errors.New("move error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		err := view.MoveToBottom()

		assert.Error(t, err)
		assert.Equal(t, "move error", err.Error())
	})

	t.Run("does not write to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		err := view.MoveToBottom()

		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})
}

func TestLogmdChatView_QueryPermission(t *testing.T) {
	t.Run("delegates to wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		query := &ui.PermissionQueryUI{
			Id:      "query-1",
			Title:   "Allow tool?",
			Details: "This tool will modify files",
			Options: []string{"yes", "no"},
		}

		err := view.QueryPermission(query)

		require.NoError(t, err)
		assert.Len(t, wrapped.QueryPermissionCalls, 1)
		assert.Equal(t, query, wrapped.QueryPermissionCalls[0])
	})

	t.Run("returns error from wrapped view", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		wrapped.QueryPermissionErr = errors.New("permission error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		query := &ui.PermissionQueryUI{
			Id:      "query-1",
			Title:   "Allow tool?",
			Details: "This tool will modify files",
			Options: []string{"yes", "no"},
		}

		err := view.QueryPermission(query)

		assert.Error(t, err)
		assert.Equal(t, "permission error", err.Error())
	})

	t.Run("does not write to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		query := &ui.PermissionQueryUI{
			Id:      "query-1",
			Title:   "Allow tool?",
			Details: "This tool will modify files",
			Options: []string{"yes", "no"},
		}

		err := view.QueryPermission(query)

		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})
}

func TestLogmdChatView_Concurrency(t *testing.T) {
	t.Run("mutex protects concurrent writes", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				msg := &ui.ChatMessageUI{
					Id:   string(rune('a' + i%26)),
					Role: ui.ChatRoleUser,
					Text: "Message",
				}
				_ = view.AddMessage(msg)
			}(i)
		}
		wg.Wait()

		output := buf.String()
		count := strings.Count(output, "## User")
		assert.Equal(t, 100, count)
	})
}

func TestLogmdChatView_FullSession(t *testing.T) {
	t.Run("complete session lifecycle", func(t *testing.T) {
		wrapped := mock.NewMockChatView()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		view := NewLogmdChatView(wrapped, &buf, mu)

		// Initialize session
		session := &ui.ChatSessionUI{
			Id:      "session-123",
			Model:   "claude-3",
			Role:    "code-assistant",
			WorkDir: "/home/user/project",
			Messages: []*ui.ChatMessageUI{
				{
					Id:   "msg-1",
					Role: ui.ChatRoleUser,
					Text: "Can you help me refactor this code?",
				},
			},
		}

		err := view.Init(session)
		require.NoError(t, err)

		// Add a new message
		msg := &ui.ChatMessageUI{
			Id:   "msg-2",
			Role: ui.ChatRoleAssistant,
			Text: "Sure, I'll help you refactor the code.",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool-1",
					Name:    "analyze_code",
					Status:  ui.ToolStatusSucceeded,
					Message: "Code analyzed successfully",
				},
			},
		}
		err = view.AddMessage(msg)
		require.NoError(t, err)

		// Update message (should not log)
		msg.Text = "Sure, I'll help you refactor the code. Here's what I found:"
		err = view.UpdateMessage(msg)
		require.NoError(t, err)

		// Update tool (should not log)
		tool := &ui.ToolUI{
			Id:      "tool-1",
			Name:    "analyze_code",
			Status:  ui.ToolStatusSucceeded,
			Message: "Code analyzed with 3 suggestions",
		}
		err = view.UpdateTool(tool)
		require.NoError(t, err)

		// Move to bottom (should not log)
		err = view.MoveToBottom()
		require.NoError(t, err)

		// Query permission (should not log)
		query := &ui.PermissionQueryUI{
			Id:      "query-1",
			Title:   "Apply refactoring?",
			Details: "This will modify 3 files",
			Options: []string{"yes", "no", "ask"},
		}
		err = view.QueryPermission(query)
		require.NoError(t, err)

		// Verify output
		output := buf.String()

		// Should have header
		assert.Contains(t, output, "# Chat Session")
		assert.Contains(t, output, "session-123")
		assert.Contains(t, output, "claude-3")

		// Should have initial message
		assert.Contains(t, output, "Can you help me refactor this code?")

		// Should have added message
		assert.Contains(t, output, "Sure, I'll help you refactor the code.")
		assert.Contains(t, output, "analyze_code")

		// Should NOT have the updated text
		assert.NotContains(t, output, "Here's what I found:")

		// Should NOT have the updated tool message
		assert.NotContains(t, output, "3 suggestions")

		// Should NOT have permission query
		assert.NotContains(t, output, "Apply refactoring?")
	})
}
