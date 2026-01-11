package core

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionThread(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfsInstance := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance)

	system := &SweSystem{
		ModelProviders: map[string]models.ModelProvider{"ollama": client},
		SystemPrompt:   "You are skilled software developer.",
		Tools:          tools,
		VFS:            vfsInstance,
	}

	t.Run("basic initialization and session management", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		// Initially no session
		assert.Nil(t, controller.GetSession())

		// Start a session
		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Now we have a session
		assert.NotNil(t, controller.GetSession())
	})

	t.Run("cannot start session twice", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Try to start another session while session already exists
		err = controller.StartSession("ollama/devstral-small-2:latest")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session already exists")
	})

	t.Run("full conversation with tool calls", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Populate mock server with LLM responses
		// First response: assistant makes a tool call to write the file
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "devstral-small-2:latest",
				CreatedAt: "2024-01-01T00:00:00Z",
				Message: models.OllamaMessage{
					Role: "assistant",
					ToolCalls: []models.OllamaToolCall{
						{
							Function: models.OllamaToolCallFunction{
								Name: "vfs.write",
								Arguments: map[string]interface{}{
									"path":    "hello_world.py",
									"content": "print(\"Hello World\")\n",
								},
							},
						},
					},
				},
				Done: false,
			}),
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "devstral-small-2:latest",
				CreatedAt: "2024-01-01T00:00:01Z",
				Message: models.OllamaMessage{
					Role: "assistant",
				},
				Done:       true,
				DoneReason: "stop",
			}),
		)

		// Second response: after tool execution, assistant confirms completion
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "devstral-small-2:latest",
				CreatedAt: "2024-01-01T00:00:02Z",
				Message: models.OllamaMessage{
					Role:    "assistant",
					Content: "I've created the Hello World program in Python.",
				},
				Done: false,
			}),
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "devstral-small-2:latest",
				CreatedAt: "2024-01-01T00:00:03Z",
				Message: models.OllamaMessage{
					Role: "assistant",
				},
				Done:       true,
				DoneReason: "stop",
			}),
		)

		// Send user prompt (non-blocking)
		err = controller.UserPrompt("Implement Hello World program in Python")
		assert.NoError(t, err)

		// Wait for the session to finish
		mockHandler.WaitForRunFinished()

		// Verify no error occurred
		assert.NoError(t, mockHandler.RunFinishedError)

		// Verify file was created
		bytes, err := vfsInstance.ReadFile("hello_world.py")
		assert.NoError(t, err)
		assert.Contains(t, string(bytes), "print(\"Hello World\")")

		// Verify UI handler captured the events
		assert.NotEmpty(t, mockHandler.ToolCallStarts, "should have captured tool call start")
		assert.NotEmpty(t, mockHandler.ToolCallDetails, "should have captured tool call details")
		assert.NotEmpty(t, mockHandler.ToolCallResults, "should have captured tool call result")
		assert.Equal(t, "vfs.write", mockHandler.ToolCallResults[0].Call.Function)
		assert.NotEmpty(t, mockHandler.MarkdownChunks, "should have captured markdown chunks")
		assert.Contains(t, mockHandler.MarkdownChunks, "I've created the Hello World program in Python.")
	})

	t.Run("multiple prompts in queue", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Setup responses for first prompt
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "devstral-small-2:latest",
				CreatedAt: "2024-01-01T00:00:00Z",
				Message: models.OllamaMessage{
					Role:    "assistant",
					Content: "First response",
				},
				Done:       true,
				DoneReason: "stop",
			}),
		)

		// Setup responses for second prompt
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			chatResponseJSON(models.OllamaChatResponse{
				Model:     "devstral-small-2:latest",
				CreatedAt: "2024-01-01T00:00:01Z",
				Message: models.OllamaMessage{
					Role:    "assistant",
					Content: "Second response",
				},
				Done:       true,
				DoneReason: "stop",
			}),
		)

		// Send two prompts quickly
		err = controller.UserPrompt("First prompt")
		assert.NoError(t, err)

		err = controller.UserPrompt("Second prompt")
		assert.NoError(t, err)

		// Wait for the session to finish
		mockHandler.WaitForRunFinished()

		// Verify no error occurred
		assert.NoError(t, mockHandler.RunFinishedError)

		// Verify both responses were received
		assert.Contains(t, mockHandler.MarkdownChunks, "First response")
		assert.Contains(t, mockHandler.MarkdownChunks, "Second response")

		// Verify session processed both messages
		session := controller.GetSession()
		messages := session.ChatMessages()
		// Should have: system prompt + user1 + assistant1 + user2 + assistant2
		assert.GreaterOrEqual(t, len(messages), 5)
	})

	t.Run("error when prompting without session", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		// Try to send prompt without starting session
		err := controller.UserPrompt("Test prompt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not initialized")
	})

	t.Run("error when interrupting without running session", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		// Try to interrupt without a running session
		err := controller.Interrupt()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no session running")
	})
}

func TestSessionThreadSafety(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfsInstance := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance)

	system := &SweSystem{
		ModelProviders: map[string]models.ModelProvider{"ollama": client},
		SystemPrompt:   "You are skilled software developer.",
		Tools:          tools,
		VFS:            vfsInstance,
	}

	t.Run("concurrent GetSession calls", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Call GetSession from multiple goroutines
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				session := controller.GetSession()
				assert.NotNil(t, session)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("concurrent UserPrompt calls with single session", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Setup multiple responses
		for i := 0; i < 5; i++ {
			mockServer.AddStreamingResponse("/api/chat", "POST", false,
				chatResponseJSON(models.OllamaChatResponse{
					Model:     "devstral-small-2:latest",
					CreatedAt: "2024-01-01T00:00:00Z",
					Message: models.OllamaMessage{
						Role:    "assistant",
						Content: "Response",
					},
					Done:       true,
					DoneReason: "stop",
				}),
			)
		}

		// Send prompts from multiple goroutines
		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func(idx int) {
				err := controller.UserPrompt("Test prompt")
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all prompts to be queued
		for i := 0; i < 5; i++ {
			<-done
		}

		// Wait for the session to finish processing all prompts
		mockHandler.WaitForRunFinished()

		// Verify no error occurred
		assert.NoError(t, mockHandler.RunFinishedError)
	})
}
