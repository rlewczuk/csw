package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCliChatView_NewCliChatView(t *testing.T) {
	t.Run("creates view with interactive=true", func(t *testing.T) {
		output := &bytes.Buffer{}
		input := strings.NewReader("")
		presenter := mock.NewMockChatPresenter()

		view := NewCliChatView(presenter, output, input, true, false)

		assert.NotNil(t, view)
		assert.True(t, view.interactive)
		assert.False(t, view.acceptAllPermissions)
		assert.NotNil(t, view.scanner)
		assert.Len(t, presenter.SetViewCalls, 1)
	})

	t.Run("creates view with interactive=false", func(t *testing.T) {
		output := &bytes.Buffer{}
		input := strings.NewReader("")
		presenter := mock.NewMockChatPresenter()

		view := NewCliChatView(presenter, output, input, false, false)

		assert.NotNil(t, view)
		assert.False(t, view.interactive)
		assert.False(t, view.acceptAllPermissions)
		assert.Nil(t, view.scanner)
		assert.Len(t, presenter.SetViewCalls, 1)
	})

	t.Run("acceptAllPermissions implies interactive=false", func(t *testing.T) {
		output := &bytes.Buffer{}
		input := strings.NewReader("")
		presenter := mock.NewMockChatPresenter()

		view := NewCliChatView(presenter, output, input, true, true)

		assert.NotNil(t, view)
		assert.False(t, view.interactive)
		assert.True(t, view.acceptAllPermissions)
		assert.Nil(t, view.scanner)
	})
}

func TestCliChatView_Init(t *testing.T) {
	t.Run("initializes with empty session", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		session := &ui.ChatSessionUI{
			Id:       "test-session",
			Model:    "test-model",
			Messages: []*ui.ChatMessageUI{},
		}

		err := view.Init(session)
		require.NoError(t, err)
		assert.Len(t, view.messages, 0)
		assert.Equal(t, "", output.String())
	})

	t.Run("initializes with user and assistant messages", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		session := &ui.ChatSessionUI{
			Id:    "test-session",
			Model: "test-model",
			Messages: []*ui.ChatMessageUI{
				{
					Id:   "msg1",
					Role: ui.ChatRoleUser,
					Text: "Hello",
				},
				{
					Id:   "msg2",
					Role: ui.ChatRoleAssistant,
					Text: "Hi there!",
				},
			},
		}

		err := view.Init(session)
		require.NoError(t, err)
		assert.Len(t, view.messages, 2)

		outputStr := output.String()
		assert.Contains(t, outputStr, "You: Hello")
		assert.Contains(t, outputStr, "Assistant: Hi there!")
	})

	t.Run("initializes with tool calls", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		session := &ui.ChatSessionUI{
			Id:    "test-session",
			Model: "test-model",
			Messages: []*ui.ChatMessageUI{
				{
					Id:   "msg1",
					Role: ui.ChatRoleAssistant,
					Text: "Running tool",
					Tools: []*ui.ToolUI{
						{
							Id:      "tool1",
							Name:    "vfsRead",
							Status:  ui.ToolStatusSucceeded,
							Props:   [][]string{{"path", "/test/file.txt"}},
							Message: "file content here",
						},
					},
				},
			},
		}

		err := view.Init(session)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "Assistant: Running tool")
		assert.Contains(t, outputStr, "TOOL: vfsRead (tool1) - path: /test/file.txt")
		assert.Contains(t, outputStr, "TOOL: vfsRead (tool1) - (succeeded) result: file content here")
	})
}

func TestCliChatView_AddMessage(t *testing.T) {
	t.Run("adds user message", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleUser,
			Text: "Test message",
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)
		assert.Len(t, view.messages, 1)
		assert.Contains(t, output.String(), "You: Test message")
	})

	t.Run("adds assistant message with tools", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Executing",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsWrite",
					Status:  ui.ToolStatusExecuting,
					Props:   [][]string{{"path", "/test.txt"}, {"content", "hello"}},
					Message: "written",
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)
		assert.Len(t, view.messages, 1)

		outputStr := output.String()
		assert.Contains(t, outputStr, "Assistant: Executing")
		assert.Contains(t, outputStr, "TOOL: vfsWrite (tool1) - path: /test.txt, content: hello")
		assert.Contains(t, outputStr, "TOOL: vfsWrite (tool1) - (executing) result: written")
	})
}

func TestCliChatView_UpdateMessage(t *testing.T) {
	t.Run("updates message by ID", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		// Add initial message
		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Initial",
		}
		view.AddMessage(msg)

		// Clear output
		output.Reset()

		// Update message
		updatedMsg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Updated",
		}

		err := view.UpdateMessage(updatedMsg)
		require.NoError(t, err)
		assert.Len(t, view.messages, 1)
		assert.Equal(t, "Updated", view.messages[0].Text)
		assert.Contains(t, output.String(), "Assistant: Updated")
	})

	t.Run("updates message by role when ID is empty", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		// Add initial message with empty text
		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "",
		}
		view.AddMessage(msg)

		// Clear output
		output.Reset()

		// Update message by role
		updatedMsg := &ui.ChatMessageUI{
			Id:   "",
			Role: ui.ChatRoleAssistant,
			Text: "New content",
		}

		err := view.UpdateMessage(updatedMsg)
		require.NoError(t, err)
		assert.Equal(t, "New content", view.messages[0].Text)
	})
}

func TestCliChatView_UpdateTool(t *testing.T) {
	t.Run("updates tool status", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		// Add message with tool
		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Running",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool1",
					Name:   "vfsRead",
					Status: ui.ToolStatusStarted,
				},
			},
		}
		view.AddMessage(msg)

		// Clear output
		output.Reset()

		// Update tool
		updatedTool := &ui.ToolUI{
			Id:     "tool1",
			Name:   "vfsRead",
			Status: ui.ToolStatusSucceeded,
		}

		err := view.UpdateTool(updatedTool)
		require.NoError(t, err)
		assert.Equal(t, ui.ToolStatusSucceeded, view.messages[0].Tools[0].Status)
		assert.Contains(t, output.String(), "TOOL: vfsRead (tool1) - (succeeded) result:")
	})
}

func TestCliChatView_MoveToBottom(t *testing.T) {
	t.Run("is a no-op", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		err := view.MoveToBottom()
		require.NoError(t, err)
	})
}

func TestCliChatView_QueryPermission(t *testing.T) {
	t.Run("automatically accepts when acceptAllPermissions=true", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, true)

		query := &ui.PermissionQueryUI{
			Id:      "perm1",
			Title:   "Allow access?",
			Options: []string{"Allow", "Deny"},
		}

		err := view.QueryPermission(query)
		require.NoError(t, err)

		// Should have called presenter with first option
		assert.Len(t, presenter.PermissionResponseCalls, 1)
		assert.Equal(t, "Allow", presenter.PermissionResponseCalls[0])
	})

	t.Run("automatically denies when interactive=false and acceptAllPermissions=false", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		query := &ui.PermissionQueryUI{
			Id:      "perm1",
			Title:   "Allow access?",
			Options: []string{"Allow", "Deny"},
		}

		err := view.QueryPermission(query)
		require.NoError(t, err)

		// Should have called presenter with "Deny" option
		assert.Len(t, presenter.PermissionResponseCalls, 1)
		assert.Equal(t, "Deny", presenter.PermissionResponseCalls[0])
		assert.Equal(t, "", output.String())
	})

	t.Run("prompts user when interactive=true", func(t *testing.T) {
		input := strings.NewReader("1\n")
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, input, true, false)

		query := &ui.PermissionQueryUI{
			Id:      "perm1",
			Title:   "Allow access?",
			Options: []string{"Allow", "Deny"},
		}

		err := view.QueryPermission(query)
		require.NoError(t, err)

		// Should have prompted user
		outputStr := output.String()
		assert.Contains(t, outputStr, "=== Permission Required ===")
		assert.Contains(t, outputStr, "Allow access?")
		assert.Contains(t, outputStr, "1. Allow")
		assert.Contains(t, outputStr, "2. Deny")

		// Should have sent response
		assert.Len(t, presenter.PermissionResponseCalls, 1)
		assert.Equal(t, "Allow", presenter.PermissionResponseCalls[0])
	})

	t.Run("handles custom response option", func(t *testing.T) {
		input := strings.NewReader("0\nCustom response\n")
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, input, true, false)

		query := &ui.PermissionQueryUI{
			Id:                  "perm1",
			Title:               "Allow access?",
			Options:             []string{"Allow", "Deny"},
			AllowCustomResponse: "Enter custom response",
		}

		err := view.QueryPermission(query)
		require.NoError(t, err)

		// Should have prompted for custom response
		outputStr := output.String()
		assert.Contains(t, outputStr, "0. Enter custom response")

		// Should have sent custom response
		assert.Len(t, presenter.PermissionResponseCalls, 1)
		assert.Equal(t, "Custom response", presenter.PermissionResponseCalls[0])
	})
}

func TestCliChatView_ToolStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     ui.ToolStatusUI
		wantInLine string
	}{
		{"succeeded", ui.ToolStatusSucceeded, "TOOL: test.tool (tool1) - (succeeded) result:"},
		{"failed", ui.ToolStatusFailed, "TOOL: test.tool (tool1) - (failed) result:"},
		{"started", ui.ToolStatusStarted, "TOOL: test.tool (tool1) - "},
		{"executing", ui.ToolStatusExecuting, "TOOL: test.tool (tool1) - "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			presenter := mock.NewMockChatPresenter()
			view := NewCliChatView(presenter, output, nil, false, false)

			tool := &ui.ToolUI{
				Id:     "tool1",
				Name:   "test.tool",
				Status: tt.status,
			}

			msg := &ui.ChatMessageUI{
				Id:    "msg1",
				Role:  ui.ChatRoleAssistant,
				Text:  "Running",
				Tools: []*ui.ToolUI{tool},
			}

			view.AddMessage(msg)

			outputStr := output.String()
			assert.Contains(t, outputStr, tt.wantInLine)
		})
	}
}

func TestCliChatView_TruncateString(t *testing.T) {
	t.Run("truncates long strings", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		longValue := strings.Repeat("a", 50)
		truncated := view.truncateString(longValue, 40)

		assert.Equal(t, 43, len(truncated))
		assert.True(t, strings.HasSuffix(truncated, "..."))
	})

	t.Run("does not truncate short strings", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		shortValue := "short string"
		result := view.truncateString(shortValue, 40)

		assert.Equal(t, shortValue, result)
	})

	t.Run("truncates long parameter values in tool output", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		longContent := strings.Repeat("x", 50)
		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Executing",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsWrite",
					Status:  ui.ToolStatusSucceeded,
					Props:   [][]string{{"path", "/test.txt"}, {"content", longContent}},
					Message: "done",
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		// Should contain truncated content with ellipsis
		assert.Contains(t, outputStr, "content: "+strings.Repeat("x", 40)+"...")
	})

	t.Run("truncates long result message in tool output", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false)

		longResult := strings.Repeat("y", 50)
		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Executing",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsRead",
					Status:  ui.ToolStatusSucceeded,
					Props:   [][]string{{"path", "/test.txt"}},
					Message: longResult,
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		// Should contain truncated result with ellipsis
		assert.Contains(t, outputStr, "result: "+strings.Repeat("y", 40)+"...")
	})
}
