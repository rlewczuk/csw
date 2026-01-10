package tui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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

		// Create mock terminal
		term := NewTerminal()

		term.Run(widget)

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
		session := controller.GetSession()
		messages := session.ChatMessages()
		// Should have: system prompt + user message + assistant response
		assert.GreaterOrEqual(t, len(messages), 3)

		// Cleanup
		term.SendKey("esc")
		term.Close()
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

		// Create mock terminal
		term := NewTerminal()

		term.Run(widget)

		// Wait for initial render
		term.WaitForText("Welcome!", 2*time.Second)

		// Set user input
		userMessage := "Test message for controller"
		term.SendString(userMessage)

		// Send Alt+Enter to submit
		term.SendKey("alt+enter")

		// Wait for assistant response to appear
		assert.True(t, term.WaitForText("I received your message!", 5*time.Second), "Should show assistant response")

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

		// Create mock terminal
		term := NewTerminal()

		// Run the widget in the terminal
		term.Run(widget)

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
}

// chatResponseJSON converts a chat response to JSON string.
func chatResponseJSON(resp models.OllamaChatResponse) string {
	data, _ := json.Marshal(resp)
	return string(data)
}
