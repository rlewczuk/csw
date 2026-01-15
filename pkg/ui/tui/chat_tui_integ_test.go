package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/presenter"
	"github.com/codesnort/codesnort-swe/pkg/shared"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// Mock prompt generator for chat TUI tests
type chatTuiMockPromptGen struct{}

func (m *chatTuiMockPromptGen) GetPrompt(tags []string, role *core.AgentRole, state *core.AgentState) (string, error) {
	return "You are a helpful assistant.", nil
}

func TestTuiChatViewWithPresenter(t *testing.T) {
	t.Run("basic chat interaction", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &chatTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello! How can I help you today?"},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()

		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Type a message via mock terminal
		term.SendString("Hello, assistant!")

		// Send Alt+Enter to submit
		term.SendKey("alt+enter")

		// Wait for user message to appear in view
		assert.True(t, term.WaitForText("Hello, assistant!", 2*time.Second), "Should show user message in view")

		// Wait for assistant response to appear
		assert.True(t, term.WaitForText("Hello! How can I help you today?", 5*time.Second), "Should show assistant response in view")

		// Verify session has the messages
		session := thread.GetSession()
		messages := session.ChatMessages()
		// Should have: user message + assistant response (no system prompt without SetRole)
		assert.GreaterOrEqual(t, len(messages), 2)

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("verify user input sent to presenter", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &chatTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"I received your message!"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()

		term.Run(tuiView.Model())

		// Wait for initial render
		term.WaitForText("Welcome!", 2*time.Second)

		// Set user input
		userMessage := "Test message for presenter"
		term.SendString(userMessage)

		// Send Alt+Enter to submit
		term.SendKey("alt+enter")

		// Wait for assistant response to appear
		assert.True(t, term.WaitForText("I received your message!", 5*time.Second), "Should show assistant response")

		// Verify the session has the user message
		session := thread.GetSession()
		messages := session.ChatMessages()

		// Should have user message + assistant response (no system prompt without SetRole)
		assert.GreaterOrEqual(t, len(messages), 2)

		// Find the user message
		var foundUserMessage bool
		for _, msg := range messages {
			if msg.Role == models.ChatRoleUser && strings.Contains(msg.GetText(), userMessage) {
				foundUserMessage = true
				break
			}
		}
		assert.True(t, foundUserMessage, "User message should be in session history")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("verify multiple messages", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &chatTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup first LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"First response"},"done":true,"done_reason":"stop"}`,
		)

		// Setup second LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"Second response"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()

		// Run the widget in the terminal
		term.Run(tuiView.Model())

		// Wait for initial render
		term.WaitForText("Welcome!", 2*time.Second)

		// Send first message
		term.SendString("First message")
		term.SendKey("alt+enter")

		// Wait for first response
		assert.True(t, term.WaitForText("First response", 5*time.Second), "Should show first assistant response")

		// Send second message
		term.SendString("Second message")
		term.SendKey("alt+enter")

		// Wait for second response
		assert.True(t, term.WaitForText("Second response", 5*time.Second), "Should show second assistant response")

		// Verify both messages appear in view
		assert.True(t, term.ContainsText("First message"), "First user message should appear")
		assert.True(t, term.ContainsText("First response"), "First assistant response should appear")
		assert.True(t, term.ContainsText("Second message"), "Second user message should appear")
		assert.True(t, term.ContainsText("Second response"), "Second assistant response should appear")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("multi-chunk streaming response renders completely", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &chatTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response with multiple chunks
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"This is "},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"chunk two "},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"and chunk three."},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()

		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Type a message via mock terminal
		term.SendString("Tell me a story")

		// Send Alt+Enter to submit
		term.SendKey("alt+enter")

		// Wait for user message to appear in view
		assert.True(t, term.WaitForText("Tell me a story", 2*time.Second), "Should show user message in view")

		// Wait for complete assistant response to appear
		// The complete response should be: "This is chunk two and chunk three."
		assert.True(t, term.WaitForText("This is chunk two and chunk three.", 5*time.Second), "Should show complete assistant response with all chunks")

		// Also verify each individual chunk text is present
		assert.True(t, term.ContainsText("This is "), "Should contain first chunk")
		assert.True(t, term.ContainsText("chunk two "), "Should contain second chunk")
		assert.True(t, term.ContainsText("chunk three"), "Should contain third chunk")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("user message should not be duplicated", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &chatTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Response received"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()

		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Type a unique message via mock terminal
		uniqueUserMessage := "This is my unique test message"
		term.SendString(uniqueUserMessage)

		// Send Alt+Enter to submit
		term.SendKey("alt+enter")

		// Wait for user message to appear in view
		assert.True(t, term.WaitForText(uniqueUserMessage, 2*time.Second), "Should show user message in view")

		// Wait for assistant response to appear (ensures processing is complete)
		assert.True(t, term.WaitForText("Response received", 5*time.Second), "Should show assistant response")

		// Give it a moment to ensure all rendering is complete
		time.Sleep(100 * time.Millisecond)

		// Get the output and count occurrences of the user message
		output := term.GetOutput()
		count := strings.Count(output, uniqueUserMessage)

		// The message should appear exactly once in the view
		assert.Equal(t, 1, count, "User message should appear exactly once, but appeared %d times", count)

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})
}

func TestTuiChatViewWithRunBashTool(t *testing.T) {
	t.Run("run bash command with permission allowed", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		// Create mock runner with predefined response
		mockRunner := runner.NewMockRunner()
		mockRunner.SetResponse("echo 'test output'", "test output\n", 0, nil)

		// Register run.bash tool with Allow privilege for echo command
		privileges := map[string]shared.AccessFlag{
			"echo.*": shared.AccessAllow,
		}
		tool.RegisterRunBashTool(tools, mockRunner, privileges)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &chatTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response with tool call (Ollama format uses object for arguments)
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"run.bash","arguments":{"command":"echo 'test output'"}}}]},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Setup second LLM response after tool execution
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"Command executed successfully!"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()
		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Send user message
		term.SendString("Run echo command")
		term.SendKey("alt+enter")

		// Wait for the tool to execute and response to appear
		assert.True(t, term.WaitForText("Command executed successfully!", 10*time.Second), "Should show assistant response after tool execution")

		// Verify the command was executed
		executions := mockRunner.GetExecutions()
		assert.Equal(t, 1, len(executions), "Should have executed one command")
		assert.Equal(t, "echo 'test output'", executions[0].Command)

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("run bash command with permission denied", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		// Create mock runner
		mockRunner := runner.NewMockRunner()

		// Register run.bash tool with Deny privilege for rm command
		privileges := map[string]shared.AccessFlag{
			"rm.*": shared.AccessDeny,
		}
		tool.RegisterRunBashTool(tools, mockRunner, privileges)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &chatTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response with tool call (Ollama format uses object for arguments)
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"run.bash","arguments":{"command":"rm -rf /"}}}]},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Setup second LLM response after tool execution fails
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"Command was blocked."},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()
		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Send user message
		term.SendString("Delete everything")
		term.SendKey("alt+enter")

		// Wait for the response (command should be denied)
		assert.True(t, term.WaitForText("Command was blocked.", 10*time.Second), "Should show assistant response after command blocked")

		// Verify the command was NOT executed
		executions := mockRunner.GetExecutions()
		assert.Equal(t, 0, len(executions), "Should not have executed any commands")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("run bash command with Ask permission - user allows", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		// Create mock runner with predefined response
		mockRunner := runner.NewMockRunner()
		mockRunner.SetResponse("ls -la", "file1.txt\nfile2.txt\n", 0, nil)

		// Register run.bash tool with Ask privilege (default)
		tool.RegisterRunBashTool(tools, mockRunner, nil)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &chatTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response with tool call (Ollama format uses object for arguments)
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"run.bash","arguments":{"command":"ls -la"}}}]},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()
		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Send user message
		term.SendString("List files")
		term.SendKey("alt+enter")

		// Wait for permission dialog to appear
		assert.True(t, term.WaitForText("Permission Required", 5*time.Second), "Should show permission dialog")
		assert.True(t, term.ContainsText("Allow running command: ls -la"), "Should show command in permission dialog")

		// Select "Allow" option (should be first option)
		// Note: In real implementation, this would simulate user selecting Allow
		// For now, we assume permission is granted automatically in test
		// This test validates that the permission query is shown

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("run bash command with Ask permission - user denies", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		// Create mock runner
		mockRunner := runner.NewMockRunner()

		// Register run.bash tool with Ask privilege (default)
		tool.RegisterRunBashTool(tools, mockRunner, nil)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &chatTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response with tool call (Ollama format uses object for arguments)
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"run.bash","arguments":{"command":"cat /etc/passwd"}}}]},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()
		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Send user message
		term.SendString("Show passwd file")
		term.SendKey("alt+enter")

		// Wait for permission dialog to appear
		assert.True(t, term.WaitForText("Permission Required", 5*time.Second), "Should show permission dialog")
		assert.True(t, term.ContainsText("Allow running command: cat /etc/passwd"), "Should show command in permission dialog")

		// Verify the command was NOT executed yet (waiting for user permission)
		executions := mockRunner.GetExecutions()
		assert.Equal(t, 0, len(executions), "Should not have executed command yet")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})
}
