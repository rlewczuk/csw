package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/logging"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/presenter"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cliViewMockPromptGen is a mock prompt generator for CLI view tests.
type cliViewMockPromptGen struct{}

// mockCliPresenter is a minimal mock presenter for simple tests.
type mockCliPresenter struct{}

func (m *mockCliPresenter) SetView(view ui.IChatView) error                 { return nil }
func (m *mockCliPresenter) SendUserMessage(message *ui.ChatMessageUI) error { return nil }
func (m *mockCliPresenter) SaveUserMessage(message *ui.ChatMessageUI) error { return nil }
func (m *mockCliPresenter) Pause() error                                    { return nil }
func (m *mockCliPresenter) Resume() error                                   { return nil }
func (m *mockCliPresenter) PermissionResponse(response string) error        { return nil }
func (m *mockCliPresenter) SetModel(model string) error                     { return nil }

func (m *cliViewMockPromptGen) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *core.AgentState) (string, error) {
	return "You are a helpful assistant.", nil
}

func (m *cliViewMockPromptGen) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *core.AgentState) (tool.ToolInfo, error) {
	schema := tool.NewToolSchema()
	return tool.ToolInfo{
		Name:        toolName,
		Description: "Mock tool for testing",
		Schema:      schema,
	}, nil
}

func TestCliChatView_IntegrationWithSession(t *testing.T) {
	t.Run("chat response appears in non-interactive mode", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      &cliViewMockPromptGen{},
			Tools:                tools,
			VFS:                  vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create chat presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create CLI view in non-interactive mode
		output := &bytes.Buffer{}
		_ = NewCliChatView(chatPresenter, output, nil, false, false)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello from LLM!"},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Send user message through presenter
		err = chatPresenter.SendUserMessage(&ui.ChatMessageUI{
			Role: ui.ChatRoleUser,
			Text: "Hello, assistant!",
		})
		require.NoError(t, err)

		// Wait for response
		time.Sleep(2 * time.Millisecond)

		// Verify output contains both user message and assistant response
		outputStr := output.String()
		assert.Contains(t, outputStr, "You: Hello, assistant!")
		assert.Contains(t, outputStr, "Assistant: Hello from LLM!")
	})

	t.Run("streaming response updates appear in output", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      &cliViewMockPromptGen{},
			Tools:                tools,
			VFS:                  vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create chat presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create CLI view in non-interactive mode
		output := &bytes.Buffer{}
		view := NewCliChatView(chatPresenter, output, nil, false, false)
		_ = view

		// Setup LLM response with multiple chunks
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"First chunk "},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"second chunk "},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"third chunk."},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Send user message through presenter
		err = chatPresenter.SendUserMessage(&ui.ChatMessageUI{
			Role: ui.ChatRoleUser,
			Text: "Tell me something",
		})
		require.NoError(t, err)

		// Wait for complete response
		time.Sleep(2 * time.Millisecond)

		// Verify output contains complete streamed response
		outputStr := output.String()
		assert.Contains(t, outputStr, "First chunk second chunk third chunk.")
	})

	t.Run("tool call appears in output", func(t *testing.T) {
		// This test verifies that tool calls are rendered in the CLI view.
		// We don't need a full integration with LLM - just test that the view
		// correctly renders tool calls when they're added via AddMessage.

		output := &bytes.Buffer{}
		presenter := &mockCliPresenter{}
		view := NewCliChatView(presenter, output, nil, false, false)

		// Add a message with a tool call directly
		msg := &ui.ChatMessageUI{
			Id:   "msg1",
			Role: ui.ChatRoleAssistant,
			Text: "Let me read that file",
			Tools: []*ui.ToolUI{
				{
					Id:     "tool1",
					Name:   "vfs.read",
					Status: ui.ToolStatusStarted,
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		// Update tool status to succeeded
		msg.Tools[0].Status = ui.ToolStatusSucceeded
		err = view.UpdateTool(msg.Tools[0])
		require.NoError(t, err)

		// Verify output
		outputStr := output.String()
		assert.Contains(t, outputStr, "Assistant: Let me read that file")
		assert.Contains(t, outputStr, "TOOL: vfs.read (started)")
		assert.Contains(t, outputStr, "TOOL: vfs.read (succeeded)")
	})

	t.Run("accepts all permissions automatically when configured", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      &cliViewMockPromptGen{},
			Tools:                tools,
			VFS:                  vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create chat presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create CLI view with acceptAllPermissions=true
		output := &bytes.Buffer{}
		view := NewCliChatView(chatPresenter, output, nil, false, true)

		// Verify interactive is false when acceptAllPermissions is true
		assert.False(t, view.interactive)
		assert.True(t, view.acceptAllPermissions)

		// Setup LLM response with tool call
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Creating file","tool_calls":[{"id":"call_1","type":"function","function":{"name":"vfs.write","arguments":"{\"path\":\"new.txt\",\"content\":\"test\"}"}}]},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Send user message through presenter
		err = chatPresenter.SendUserMessage(&ui.ChatMessageUI{
			Role: ui.ChatRoleUser,
			Text: "Create new.txt",
		})
		require.NoError(t, err)

		// Wait for response
		time.Sleep(5 * time.Millisecond)

		// Verify output does not contain permission prompt
		outputStr := output.String()
		assert.NotContains(t, outputStr, "=== Permission Required ===")
	})

	t.Run("multiple messages in session", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"ollama": client},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      &cliViewMockPromptGen{},
			Tools:                tools,
			VFS:                  vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create chat presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create CLI view
		output := &bytes.Buffer{}
		view := NewCliChatView(chatPresenter, output, nil, false, false)
		_ = view

		// Setup first LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"First response"},"done":true,"done_reason":"stop"}`,
		)

		// Send first message through presenter
		err = chatPresenter.SendUserMessage(&ui.ChatMessageUI{
			Role: ui.ChatRoleUser,
			Text: "First message",
		})
		require.NoError(t, err)

		time.Sleep(2 * time.Millisecond)

		// Setup second LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Second response"},"done":true,"done_reason":"stop"}`,
		)

		// Send second message through presenter
		err = chatPresenter.SendUserMessage(&ui.ChatMessageUI{
			Role: ui.ChatRoleUser,
			Text: "Second message",
		})
		require.NoError(t, err)

		time.Sleep(2 * time.Millisecond)

		// Verify both conversations appear in output
		outputStr := output.String()
		assert.Contains(t, outputStr, "You: First message")
		assert.Contains(t, outputStr, "Assistant: First response")
		assert.Contains(t, outputStr, "You: Second message")
		assert.Contains(t, outputStr, "Assistant: Second response")

		// Verify messages appear in order
		firstIdx := strings.Index(outputStr, "First message")
		secondIdx := strings.Index(outputStr, "Second message")
		assert.Less(t, firstIdx, secondIdx, "Messages should appear in order")
	})
}
