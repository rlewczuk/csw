package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCliChatView_NewCliChatView(t *testing.T) {
	t.Run("creates view with interactive=true", func(t *testing.T) {
		output := &bytes.Buffer{}
		input := strings.NewReader("")
		presenter := mock.NewMockChatPresenter()

		view := NewCliChatView(presenter, output, input, true, false, false)

		assert.NotNil(t, view)
		assert.True(t, view.interactive)
		assert.False(t, view.acceptAllPermissions)
		assert.False(t, view.verbose)
		assert.NotNil(t, view.scanner)
	})

	t.Run("creates view with interactive=false", func(t *testing.T) {
		output := &bytes.Buffer{}
		input := strings.NewReader("")
		presenter := mock.NewMockChatPresenter()

		view := NewCliChatView(presenter, output, input, false, false, false)

		assert.NotNil(t, view)
		assert.False(t, view.interactive)
		assert.False(t, view.acceptAllPermissions)
		assert.False(t, view.verbose)
		assert.Nil(t, view.scanner)
	})

	t.Run("acceptAllPermissions implies interactive=false", func(t *testing.T) {
		output := &bytes.Buffer{}
		input := strings.NewReader("")
		presenter := mock.NewMockChatPresenter()

		view := NewCliChatView(presenter, output, input, true, true, false)

		assert.NotNil(t, view)
		assert.False(t, view.interactive)
		assert.True(t, view.acceptAllPermissions)
		assert.False(t, view.verbose)
		assert.Nil(t, view.scanner)
	})

	t.Run("creates view with verbose=true", func(t *testing.T) {
		output := &bytes.Buffer{}
		input := strings.NewReader("")
		presenter := mock.NewMockChatPresenter()

		view := NewCliChatView(presenter, output, input, false, false, true)

		assert.NotNil(t, view)
		assert.False(t, view.interactive)
		assert.False(t, view.acceptAllPermissions)
		assert.True(t, view.verbose)
		assert.Nil(t, view.scanner)
	})

	t.Run("uses provided slug prefix", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, "feature/test", false, false, false)

		err := view.AddMessage(&ui.ChatMessageUI{Id: "msg1", Role: ui.ChatRoleUser, Text: "Hello"})
		require.NoError(t, err)

		assert.Contains(t, output.String(), "\x1b[90m[feature/test]\x1b[0m You: Hello")
	})

	t.Run("falls back to main slug when empty", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, "", false, false, false)

		err := view.AddMessage(&ui.ChatMessageUI{Id: "msg1", Role: ui.ChatRoleUser, Text: "Hello"})
		require.NoError(t, err)

		assert.Contains(t, output.String(), "\x1b[90m[main]\x1b[0m You: Hello")
	})
}

func TestCliChatView_Init(t *testing.T) {
	t.Run("initializes with empty session", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

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
		view := NewCliChatView(presenter, output, nil, false, false, false)

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

	t.Run("initializes with tool calls in final status", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

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
							Summary: "vfsRead",
						},
					},
				},
			},
		}

		err := view.Init(session)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "Assistant: Running tool")
		assert.Contains(t, outputStr, "✅ vfsRead")
	})

	t.Run("does not display tools in started or executing status", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

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
							Id:     "tool1",
							Name:   "vfsRead",
							Status: ui.ToolStatusStarted,
							Props:  [][]string{{"path", "/test/file.txt"}},
						},
						{
							Id:     "tool2",
							Name:   "vfsWrite",
							Status: ui.ToolStatusExecuting,
							Props:  [][]string{{"path", "/test.txt"}},
						},
					},
				},
			},
		}

		err := view.Init(session)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "Assistant: Running tool")
		assert.NotContains(t, outputStr, "TOOL: vfsRead")
		assert.NotContains(t, outputStr, "TOOL: vfsWrite")
	})
}

func TestCliChatView_AddMessage(t *testing.T) {
	t.Run("adds user message", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

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

	t.Run("does not display tools in executing status", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

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
		assert.NotContains(t, outputStr, "TOOL:")
	})

	t.Run("displays tools in succeeded status with Display field", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Done",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsWrite",
					Status:  ui.ToolStatusSucceeded,
					Props:   [][]string{{"path", "/test.txt"}},
					Message: "written",
					Summary: "vfsWrite",
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)
		assert.Len(t, view.messages, 1)

		outputStr := output.String()
		assert.Contains(t, outputStr, "Assistant: Done")
		assert.Contains(t, outputStr, "✅ vfsWrite")
	})

	t.Run("does not render tool when summary is empty", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Done",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool1",
					Name:   "customTool",
					Status: ui.ToolStatusSucceeded,
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.NotContains(t, outputStr, "TOOL:")
	})
}

func TestCliChatView_ShowMessage(t *testing.T) {
	tests := []struct {
		name        string
		messageType shared.MessageType
		expected    string
	}{
		{name: "info", messageType: shared.MessageTypeInfo, expected: "\x1b[90m[main]\x1b[0m [INFO] hello\n"},
		{name: "warning", messageType: shared.MessageTypeWarning, expected: "\x1b[90m[main]\x1b[0m [WARNING] hello\n"},
		{name: "error", messageType: shared.MessageTypeError, expected: "\x1b[90m[main]\x1b[0m [ERROR] hello\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			view := NewCliChatView(nil, output, nil, false, false, false)

			view.ShowMessage("hello", tt.messageType)

			assert.Equal(t, tt.expected, output.String())
		})
	}
}

func TestCliChatView_UpdateMessage(t *testing.T) {
	t.Run("updates message by ID", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

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
		view := NewCliChatView(presenter, output, nil, false, false, false)

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

	t.Run("assistant updates are rendered as full messages", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Hello ",
		}
		view.AddMessage(msg)

		updatedMsg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Hello world",
		}
		err := view.UpdateMessage(updatedMsg)
		require.NoError(t, err)

		updatedMsg = &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Hello world!",
		}
		err = view.UpdateMessage(updatedMsg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "Assistant: Hello ")
		assert.Contains(t, outputStr, "Assistant: Hello world")
		assert.Contains(t, outputStr, "Assistant: Hello world!")
	})
}

func TestCliChatView_UpdateTool(t *testing.T) {
	t.Run("updates tool status and displays when in final status", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		// Add message with tool in started state (should not be displayed)
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

		// Update tool to succeeded status
		updatedTool := &ui.ToolUI{
			Id:      "tool1",
			Name:    "vfsRead",
			Status:  ui.ToolStatusSucceeded,
			Summary: "vfsRead",
		}

		err := view.UpdateTool(updatedTool)
		require.NoError(t, err)
		assert.Equal(t, ui.ToolStatusSucceeded, view.messages[0].Tools[0].Status)
		assert.Contains(t, output.String(), "✅ vfsRead")
	})

	t.Run("does not display tool when status is started or executing", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

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

		// Update tool to executing status
		updatedTool := &ui.ToolUI{
			Id:     "tool1",
			Name:   "vfsRead",
			Status: ui.ToolStatusExecuting,
		}

		err := view.UpdateTool(updatedTool)
		require.NoError(t, err)
		assert.Equal(t, ui.ToolStatusExecuting, view.messages[0].Tools[0].Status)
		assert.NotContains(t, output.String(), "TOOL:")
	})
}

func TestCliChatView_ToolRenderOutputOnUpdate(t *testing.T) {
	t.Run("renders summary once it becomes available", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Done",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool1",
					Name:   "vfsRead",
					Status: ui.ToolStatusSucceeded,
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)
		assert.NotContains(t, output.String(), "TOOL:")

		updatedTool := &ui.ToolUI{
			Id:      "tool1",
			Name:    "vfsRead",
			Status:  ui.ToolStatusSucceeded,
			Summary: "Read file: /test.txt",
		}

		err = view.UpdateTool(updatedTool)
		require.NoError(t, err)
		assert.Contains(t, output.String(), "✅ Read file: /test.txt")
	})
}

func TestCliChatView_MoveToBottom(t *testing.T) {
	t.Run("is a no-op", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		err := view.MoveToBottom()
		require.NoError(t, err)
	})
}

func TestCliChatView_QueryPermission(t *testing.T) {
	t.Run("automatically accepts when acceptAllPermissions=true", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, true, false)

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
		view := NewCliChatView(presenter, output, nil, false, false, false)

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
		view := NewCliChatView(presenter, output, input, true, false, false)

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
		view := NewCliChatView(presenter, output, input, true, false, false)

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
		name           string
		status         ui.ToolStatusUI
		shouldDisplay  bool
		expectedOutput string
	}{
		{"succeeded", ui.ToolStatusSucceeded, true, "✅ test.tool"},
		{"failed", ui.ToolStatusFailed, true, "❌ test.tool"},
		{"started", ui.ToolStatusStarted, false, ""},
		{"executing", ui.ToolStatusExecuting, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			presenter := mock.NewMockChatPresenter()
			view := NewCliChatView(presenter, output, nil, false, false, false)

			tool := &ui.ToolUI{
				Id:      "tool1",
				Name:    "test.tool",
				Summary: "test.tool",
				Status:  tt.status,
			}

			msg := &ui.ChatMessageUI{
				Id:    "msg1",
				Role:  ui.ChatRoleAssistant,
				Text:  "Running",
				Tools: []*ui.ToolUI{tool},
			}

			view.AddMessage(msg)

			outputStr := output.String()
			if tt.shouldDisplay {
				assert.Contains(t, outputStr, tt.expectedOutput)
			} else {
				assert.NotContains(t, outputStr, "TOOL:")
			}
		})
	}
}

func TestCliChatView_ToolNotDuplicatedOnMessageUpdate(t *testing.T) {
	t.Run("tools are not printed multiple times when message is updated", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		// Add initial assistant message with a tool in succeeded status
		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Let me help",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsRead",
					Status:  ui.ToolStatusSucceeded,
					Summary: "vfsRead",
				},
			},
		}
		err := view.AddMessage(msg)
		require.NoError(t, err)

		// Clear output to track only updates
		output.Reset()

		// Update the message text (simulating streaming)
		updatedMsg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Let me help you",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsRead",
					Status:  ui.ToolStatusSucceeded,
					Summary: "vfsRead",
				},
			},
		}
		err = view.UpdateMessage(updatedMsg)
		require.NoError(t, err)

		// Count how many times the tool appears in output
		outputStr := output.String()
		toolCount := strings.Count(outputStr, "✅ vfsRead")

		// Tool should not appear again since it was already rendered with the same status
		assert.Equal(t, 0, toolCount, "Tool should not be printed again when status hasn't changed")
	})

	t.Run("tools are printed again only when status changes to final", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		// Add initial assistant message with a tool in started state (not displayed)
		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Running",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool1",
					Name:   "vfsRead",
					Status: ui.ToolStatusStarted,
					Props:  [][]string{{"path", "/test.txt"}},
				},
			},
		}
		err := view.AddMessage(msg)
		require.NoError(t, err)

		// Tool should not be printed since it's in started status
		outputStr := output.String()
		assert.NotContains(t, outputStr, "TOOL:")

		// Clear output
		output.Reset()

		// Update the message with same tool status (should not reprint tool)
		updatedMsg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Running tool",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool1",
					Name:   "vfsRead",
					Status: ui.ToolStatusStarted,
					Props:  [][]string{{"path", "/test.txt"}},
				},
			},
		}
		err = view.UpdateMessage(updatedMsg)
		require.NoError(t, err)

		// Tool should not be printed again since status hasn't changed
		outputStr = output.String()
		assert.NotContains(t, outputStr, "TOOL:")

		// Now update with a different status via UpdateTool
		output.Reset()
		updatedTool := &ui.ToolUI{
			Id:      "tool1",
			Name:    "vfsRead",
			Status:  ui.ToolStatusSucceeded,
			Summary: "vfsRead",
		}
		err = view.UpdateTool(updatedTool)
		require.NoError(t, err)

		// Tool should be printed now since status changed to final
		outputStr = output.String()
		assert.Contains(t, outputStr, "✅ vfsRead")
	})

	t.Run("multiple message updates do not duplicate tools", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		// Add initial assistant message
		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "",
		}
		err := view.AddMessage(msg)
		require.NoError(t, err)

		totalToolCount := 0

		// Simulate multiple streaming updates with tools in succeeded status
		for i := 0; i < 5; i++ {
			output.Reset()
			updatedMsg := &ui.ChatMessageUI{
				Id:   "msg1",
				Role: ui.ChatRoleAssistant,
				Text: strings.Repeat("x", i+1),
				Tools: []*ui.ToolUI{
					{
						Id:      "tool1",
						Name:    "vfsRead",
						Status:  ui.ToolStatusSucceeded,
						Props:   [][]string{{"path", "/test.txt"}},
						Summary: "vfsRead",
					},
				},
			}
			err = view.UpdateMessage(updatedMsg)
			require.NoError(t, err)

			// Count tool occurrences in this update
			outputStr := output.String()
			toolCount := strings.Count(outputStr, "✅ vfsRead")
			totalToolCount += toolCount

			// Tool should appear at most once (on first update when it's first seen)
			// and never again on subsequent updates
			if i == 0 {
				// First update: tool may be printed once
				assert.LessOrEqual(t, toolCount, 1, "Tool should be printed at most once on first update")
			} else {
				// Subsequent updates: tool should not be reprinted
				assert.Equal(t, 0, toolCount, "Tool should not be reprinted on streaming update %d", i)
			}
		}

		// Total tool occurrences across all updates should be at most 1
		assert.LessOrEqual(t, totalToolCount, 1, "Tool should not be duplicated across all streaming updates")
	})
}

func TestCliChatView_ToolDisplayField(t *testing.T) {
	t.Run("uses Display field when available", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Done",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsRead",
					Status:  ui.ToolStatusSucceeded,
					Summary: "Read file: /test.txt",
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "✅ Read file: /test.txt")
		assert.NotContains(t, outputStr, "vfsRead")
	})

	t.Run("does not render when Display is empty", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Done",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool1",
					Name:   "vfsRead",
					Status: ui.ToolStatusSucceeded,
					// Display is empty
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.NotContains(t, outputStr, "TOOL:")
	})
}

func TestCliChatView_ThinkingContent(t *testing.T) {
	t.Run("displays thinking content as italic", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:       "msg1",
			Role:     ui.ChatRoleAssistant,
			Text:     "Hello!",
			Thinking: "Let me think about this...",
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "*Let me think about this...*")
		assert.Contains(t, outputStr, "Assistant: Hello!")
	})

	t.Run("thinking content updates are rendered as full messages", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		// Initial message with thinking
		msg := &ui.ChatMessageUI{
			Id:       "msg1",
			Role:     ui.ChatRoleAssistant,
			Text:     "",
			Thinking: "Thinking",
		}
		err := view.AddMessage(msg)
		require.NoError(t, err)

		output.Reset()

		// Update with more thinking content
		updatedMsg := &ui.ChatMessageUI{
			Id:       "msg1",
			Role:     ui.ChatRoleAssistant,
			Text:     "",
			Thinking: "Thinking about it",
		}
		err = view.UpdateMessage(updatedMsg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "*Thinking about it*")
	})

	t.Run("thinking content appears before text content", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:       "msg1",
			Role:     ui.ChatRoleAssistant,
			Thinking: "My reasoning",
			Text:     "My answer",
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		// Thinking should appear first
		thinkingIdx := strings.Index(outputStr, "*My reasoning*")
		textIdx := strings.Index(outputStr, "Assistant: My answer")
		assert.Less(t, thinkingIdx, textIdx, "Thinking should appear before assistant text")
	})

	t.Run("no thinking output when thinking is empty", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:       "msg1",
			Role:     ui.ChatRoleAssistant,
			Text:     "Hello!",
			Thinking: "",
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "Assistant: Hello!")
		assert.NotContains(t, outputStr, "**")
	})
}

func TestCliChatView_VerboseMode(t *testing.T) {
	t.Run("displays Details field when verbose=true", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, true)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Done",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsRead",
					Status:  ui.ToolStatusSucceeded,
					Summary: "Short summary",
					Details: "Full detailed output with multiple lines\nLine 2\nLine 3",
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "✅ Full detailed output with multiple lines")
		assert.Contains(t, outputStr, "Line 2")
		assert.Contains(t, outputStr, "Line 3")
		assert.NotContains(t, outputStr, "Short summary")
	})

	t.Run("displays Summary field when verbose=false", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Done",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsRead",
					Status:  ui.ToolStatusSucceeded,
					Summary: "Short summary",
					Details: "Full detailed output with multiple lines\nLine 2\nLine 3",
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "✅ Short summary")
		assert.NotContains(t, outputStr, "Line 2")
		assert.NotContains(t, outputStr, "Full detailed output")
	})

	t.Run("falls back to Summary when Details is empty in verbose mode", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, true)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Done",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsRead",
					Status:  ui.ToolStatusSucceeded,
					Summary: "Short summary",
					// Details is empty
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "✅ Short summary")
	})

	t.Run("falls back to Details when Summary is empty in non-verbose mode", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, false)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Done",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsRead",
					Status:  ui.ToolStatusSucceeded,
					Summary: "",
					Details: "Full detailed output",
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "✅ Full detailed output")
	})

	t.Run("verbose mode displays failed tool with Details", func(t *testing.T) {
		output := &bytes.Buffer{}
		presenter := mock.NewMockChatPresenter()
		view := NewCliChatView(presenter, output, nil, false, false, true)

		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Failed",
			Tools: []*ui.ToolUI{
				{
					Id:      "tool1",
					Name:    "vfsRead",
					Status:  ui.ToolStatusFailed,
					Summary: "Error: file not found",
					Details: "ERROR: file not found\nPath: /nonexistent/file.txt\nPlease check the path and try again.",
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "❌ ERROR: file not found")
		assert.Contains(t, outputStr, "Path: /nonexistent/file.txt")
	})
}
