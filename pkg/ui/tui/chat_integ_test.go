package tui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatWidgetWithFullController(t *testing.T) {
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
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are a helpful assistant.",
			Tools:          tools,
			VFS:            vfsInstance,
		}

		// Create chat widget
		controller := core.NewSessionController(system, nil)
		err = controller.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		widget, err := NewChatWidget(controller)
		require.NoError(t, err)

		// Set the controller's output handler to the widget
		controller.SetOutputHandler(widget)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "test-model:latest",
				CreatedAt: "2024-01-01T00:00:00Z",
				Message: models.OllamaMessage{
					Role:    "assistant",
					Content: "Hello! How can I help you today?",
				},
				Done: false,
			}),
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "test-model:latest",
				CreatedAt: "2024-01-01T00:00:01Z",
				Message: models.OllamaMessage{
					Role: "assistant",
				},
				Done:       true,
				DoneReason: "stop",
			}),
		)

		// Initialize the widget
		widget.Init()

		// Simulate user typing a message
		widget.textarea.SetValue("Hello, assistant!")

		// Check initial state - no messages yet
		output := widget.View()
		assert.Contains(t, output, "Welcome!", "Should show welcome message")

		// Simulate Alt+Enter key press to send message
		// Create a proper alt+enter key message
		updatedModel, _ := widget.Update(tea.KeyMsg{
			Type: tea.KeyEnter,
			Alt:  true,
		})
		widget = updatedModel.(*ChatWidget)

		// Verify textarea is cleared
		assert.Equal(t, "", widget.textarea.Value(), "Textarea should be cleared after sending")

		// Verify user message was added
		widget.mu.Lock()
		assert.GreaterOrEqual(t, len(widget.messages), 1, "Should have at least one message")
		assert.Equal(t, "user", widget.messages[0].Role)
		assert.Equal(t, "Hello, assistant!", widget.messages[0].Content)
		widget.mu.Unlock()

		// Wait for the response to be processed
		// Poll for the assistant message to appear
		var foundResponse bool
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			widget.mu.Lock()
			messageCount := len(widget.messages)
			hasContent := false
			if messageCount >= 2 && widget.messages[1].Content != "" {
				hasContent = true
			}
			widget.mu.Unlock()
			if hasContent {
				foundResponse = true
				break
			}
		}
		assert.True(t, foundResponse, "Should have received assistant response")

		// Update viewport content to reflect the changes
		widget.updateViewportContent()

		// Check that response appears in view
		output = widget.View()
		assert.Contains(t, output, "Hello, assistant!", "Should show user message in view")
		assert.Contains(t, output, "Hello! How can I help you today?", "Should show assistant response in view")

		// Verify session has the messages
		session := controller.GetSession()
		messages := session.ChatMessages()
		// Should have: system prompt + user message + assistant response
		assert.GreaterOrEqual(t, len(messages), 3)
	})

	t.Run("verify user input sent to controller", func(t *testing.T) {
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

		// Create chat widget
		controller := core.NewSessionController(system, nil)
		err = controller.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		widget, err := NewChatWidget(controller)
		require.NoError(t, err)

		// Set the controller's output handler to the widget
		controller.SetOutputHandler(widget)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "test-model:latest",
				CreatedAt: "2024-01-01T00:00:00Z",
				Message: models.OllamaMessage{
					Role:    "assistant",
					Content: "I received your message!",
				},
				Done:       true,
				DoneReason: "stop",
			}),
		)

		// Initialize the widget
		widget.Init()

		// Set user input
		userMessage := "Test message for controller"
		widget.textarea.SetValue(userMessage)

		// Simulate Alt+Enter key press to send message
		widget.Update(tea.KeyMsg{
			Type: tea.KeyEnter,
			Alt:  true,
		})

		// Wait for response
		var foundResponse bool
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			widget.mu.Lock()
			messageCount := len(widget.messages)
			hasContent := false
			if messageCount >= 2 && widget.messages[1].Content != "" {
				hasContent = true
			}
			widget.mu.Unlock()
			if hasContent {
				foundResponse = true
				break
			}
		}
		assert.True(t, foundResponse, "Should have received assistant response")

		// Update viewport content to reflect the changes
		widget.updateViewportContent()

		// Verify the session has the user message
		session := controller.GetSession()
		messages := session.ChatMessages()

		// Should have system prompt + user message + assistant response
		assert.GreaterOrEqual(t, len(messages), 3)

		// Find the user message
		var foundUserMessage bool
		for _, msg := range messages {
			if msg.Role == models.ChatRoleUser && strings.Contains(msg.GetText(), userMessage) {
				foundUserMessage = true
				break
			}
		}
		assert.True(t, foundUserMessage, "User message should be in session history")

		// Verify response appears in widget
		output := widget.View()
		assert.Contains(t, output, "I received your message!", "Should show assistant response")
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
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are a helpful assistant.",
			Tools:          tools,
			VFS:            vfsInstance,
		}

		// Create chat widget
		controller := core.NewSessionController(system, nil)
		err = controller.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		widget, err := NewChatWidget(controller)
		require.NoError(t, err)

		// Set the controller's output handler to the widget
		controller.SetOutputHandler(widget)

		// Setup first LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "test-model:latest",
				CreatedAt: "2024-01-01T00:00:00Z",
				Message: models.OllamaMessage{
					Role:    "assistant",
					Content: "First response",
				},
				Done:       true,
				DoneReason: "stop",
			}),
		)

		// Setup second LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "test-model:latest",
				CreatedAt: "2024-01-01T00:00:01Z",
				Message: models.OllamaMessage{
					Role:    "assistant",
					Content: "Second response",
				},
				Done:       true,
				DoneReason: "stop",
			}),
		)

		// Initialize the widget
		widget.Init()

		// Send first message
		widget.textarea.SetValue("First message")
		widget.Update(tea.KeyMsg{
			Type: tea.KeyEnter,
			Alt:  true,
		})

		// Wait for first response
		var foundFirstResponse bool
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			widget.mu.Lock()
			messageCount := len(widget.messages)
			hasContent := false
			if messageCount >= 2 && widget.messages[1].Content != "" {
				hasContent = true
			}
			widget.mu.Unlock()
			if hasContent {
				foundFirstResponse = true
				break
			}
		}
		assert.True(t, foundFirstResponse, "Should have received first assistant response")

		// Send second message
		widget.textarea.SetValue("Second message")
		widget.Update(tea.KeyMsg{
			Type: tea.KeyEnter,
			Alt:  true,
		})

		// Wait for second response
		var foundSecondResponse bool
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			widget.mu.Lock()
			messageCount := len(widget.messages)
			hasContent := false
			if messageCount >= 4 && widget.messages[3].Content != "" {
				hasContent = true
			}
			widget.mu.Unlock()
			if hasContent {
				foundSecondResponse = true
				break
			}
		}
		assert.True(t, foundSecondResponse, "Should have received second assistant response")

		// Update viewport content to reflect the changes
		widget.updateViewportContent()

		// Verify both messages appear in view
		output := widget.View()
		assert.Contains(t, output, "First message", "First user message should appear")
		assert.Contains(t, output, "First response", "First assistant response should appear")
		assert.Contains(t, output, "Second message", "Second user message should appear")
		assert.Contains(t, output, "Second response", "Second assistant response should appear")

		// Verify widget has both user messages
		widget.mu.Lock()
		messageCount := len(widget.messages)
		widget.mu.Unlock()
		assert.GreaterOrEqual(t, messageCount, 4, "Should have at least 4 messages (2 user + 2 assistant)")
	})
}

// chatResponseJSON converts a chat response to JSON string.
func chatResponseJSON(resp models.OllamaChatResponse) string {
	data, _ := json.Marshal(resp)
	return string(data)
}
