package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/presenter"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

func TestTuiAppViewWithChatIntegration(t *testing.T) {
	t.Run("chat response appears in app view without user interaction", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are a helpful assistant.",
			Tools:          tools,
			VFS:            vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI app view
		appView, err := NewTuiAppView(appPresenter)
		require.NoError(t, err)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello from LLM!"},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal and run app view
		term := NewTerminalMock()
		term.Run(appView.Model())

		// Wait for initial render
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Type a message and submit
		term.SendString("Hello, assistant!")
		term.SendKey("alt+enter")

		// Wait for user message to appear
		assert.True(t, term.WaitForText("Hello, assistant!", 2*time.Second), "Should show user message")

		// Wait for assistant response WITHOUT any user interaction
		// This is the key test - the response should appear automatically
		assert.True(t, term.WaitForText("Hello from LLM!", 5*time.Second), "Should show assistant response without requiring user interaction")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("streaming response updates appear automatically in app view", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are a helpful assistant.",
			Tools:          tools,
			VFS:            vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI app view
		appView, err := NewTuiAppView(appPresenter)
		require.NoError(t, err)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Setup LLM response with multiple chunks
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"First chunk "},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"second chunk "},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"third chunk."},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal and run app view
		term := NewTerminalMock()
		term.Run(appView.Model())

		// Wait for initial render
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Type a message and submit
		term.SendString("Tell me something")
		term.SendKey("alt+enter")

		// Wait for complete streaming response without user interaction
		assert.True(t, term.WaitForText("First chunk second chunk third chunk.", 5*time.Second), "Should show complete streamed response")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("multiple messages work correctly in app view", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are a helpful assistant.",
			Tools:          tools,
			VFS:            vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI app view
		appView, err := NewTuiAppView(appPresenter)
		require.NoError(t, err)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Setup first LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"First response"},"done":true,"done_reason":"stop"}`,
		)

		// Setup second LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"Second response"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal and run app view
		term := NewTerminalMock()
		term.Run(appView.Model())

		// Wait for initial render
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Send first message
		term.SendString("First message")
		term.SendKey("alt+enter")

		// Wait for first response (without user interaction)
		assert.True(t, term.WaitForText("First response", 5*time.Second), "Should show first assistant response")

		// Send second message
		term.SendString("Second message")
		term.SendKey("alt+enter")

		// Wait for second response (without user interaction)
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

	t.Run("user message should not be duplicated in app view", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are a helpful assistant.",
			Tools:          tools,
			VFS:            vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI app view
		appView, err := NewTuiAppView(appPresenter)
		require.NoError(t, err)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Response received"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal and run app view
		term := NewTerminalMock()
		term.Run(appView.Model())

		// Wait for initial render
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Type a unique message
		uniqueUserMessage := "This is my unique app view test message"
		term.SendString(uniqueUserMessage)

		// Submit
		term.SendKey("alt+enter")

		// Wait for user message to appear
		assert.True(t, term.WaitForText(uniqueUserMessage, 2*time.Second), "Should show user message")

		// Wait for assistant response
		assert.True(t, term.WaitForText("Response received", 5*time.Second), "Should show assistant response")

		// Give it a moment to ensure all rendering is complete
		time.Sleep(100 * time.Millisecond)

		// Get the output and count occurrences of the user message
		output := term.GetOutput()
		count := strings.Count(output, uniqueUserMessage)

		// The message should appear exactly once
		assert.Equal(t, 1, count, "User message should appear exactly once, but appeared %d times", count)

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("menu interaction does not break chat updates", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are a helpful assistant.",
			Tools:          tools,
			VFS:            vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI app view
		appView, err := NewTuiAppView(appPresenter)
		require.NoError(t, err)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Setup LLM responses
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Response before menu"},"done":true,"done_reason":"stop"}`,
		)
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"Response after menu"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal and run app view
		term := NewTerminalMock()
		term.Run(appView.Model())

		// Wait for initial render
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Send first message
		term.SendString("Message before menu")
		term.SendKey("alt+enter")

		// Wait for first response
		assert.True(t, term.WaitForText("Response before menu", 5*time.Second), "Should show first response")

		// Open and close menu
		term.SendKey("ctrl+p")
		assert.True(t, term.WaitForText("Main Menu", 1*time.Second), "Menu should appear")
		term.SendKey("esc")
		time.Sleep(50 * time.Millisecond)

		// Send second message
		term.SendString("Message after menu")
		term.SendKey("alt+enter")

		// Wait for second response (should work without issues after menu interaction)
		assert.True(t, term.WaitForText("Response after menu", 5*time.Second), "Should show second response after menu interaction")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})
}

// TestAppViewRefreshBug is a specific test to demonstrate the refresh bug.
// When chat view is embedded in app view, updates from presenter don't trigger re-render.
func TestAppViewRefreshBug(t *testing.T) {
	t.Run("refresh bug demonstration", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are a helpful assistant.",
			Tools:          tools,
			VFS:            vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI app view
		appView, err := NewTuiAppView(appPresenter)
		require.NoError(t, err)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"BUG_TEST_RESPONSE"},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal and run app view
		term := NewTerminalMock()
		term.Run(appView.Model())

		// Wait for initial render
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Type a message and submit
		term.SendString("Test")
		term.SendKey("alt+enter")

		// Wait a short time for LLM response to be processed (but NOT waiting for text)
		time.Sleep(500 * time.Millisecond)

		// Check if the internal chat view model has the response
		tuiChatView, ok := chatView.(*TuiChatView)
		require.True(t, ok, "chatView should be *TuiChatView")

		// Access the internal model to check if data is there
		tuiChatView.model.mu.Lock()
		messageCount := len(tuiChatView.model.messages)
		var foundResponse bool
		for _, msg := range tuiChatView.model.messages {
			if strings.Contains(msg.content, "BUG_TEST_RESPONSE") {
				foundResponse = true
				break
			}
		}
		tuiChatView.model.mu.Unlock()

		// The internal model should have the response
		assert.True(t, foundResponse, "Internal model should have the response (messages: %d)", messageCount)

		// But check if it appears in the terminal output WITHOUT any user interaction
		// This is the bug - the response might be in the model but not rendered to screen
		responseInOutput := term.ContainsText("BUG_TEST_RESPONSE")

		// This assertion exposes the bug: if the fix is not applied,
		// the response will be in the model but not in the output
		assert.True(t, responseInOutput, "Response should appear in terminal output without user interaction")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})
}

// Helper to ensure IChatView interface is correctly implemented
var _ ui.IChatView = (*TuiChatView)(nil)
