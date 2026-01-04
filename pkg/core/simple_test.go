package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chatResponseJSON(response models.OllamaChatResponse) string {
	data, _ := json.Marshal(response)
	return string(data)
}

func TestAgentCoreInitializationAndSimpleProgramGen(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfs := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfs)

	t.Run("basic initialization", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are skilled software developer.",
			Tools:          tools,
			VFS:            vfs,
		}

		session, err := system.NewSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)
		assert.NotNil(t, session)

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

		err = session.UserPrompt("Implement Hello World program in Python")
		assert.NoError(t, err)

		err = session.Run(context.Background())
		assert.NoError(t, err)

		bytes, err := vfs.ReadFile("hello_world.py")
		assert.NoError(t, err)
		assert.Contains(t, string(bytes), "print(\"Hello World\")")
	})

	t.Run("UI output handler integration", func(t *testing.T) {
		uiFactory := ui.NewMockUiFactory()

		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are skilled software developer.",
			Tools:          tools,
			VFS:            vfs,
			UiFactory:      uiFactory,
		}

		session, err := system.NewSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Verify that UI factory created a handler
		require.Len(t, uiFactory.Handlers, 1)
		handler := uiFactory.Handlers[0]

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
									"path":    "test.txt",
									"content": "test content",
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
					Content: "File created successfully.",
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

		err = session.UserPrompt("Create a test file")
		assert.NoError(t, err)

		err = session.Run(context.Background())
		assert.NoError(t, err)

		// Verify UI handler captured the events
		// Should have at least one tool call start
		assert.NotEmpty(t, handler.ToolCallStarts, "should have captured tool call start")
		// Should have tool call details
		assert.NotEmpty(t, handler.ToolCallDetails, "should have captured tool call details")
		// Should have tool call result
		assert.NotEmpty(t, handler.ToolCallResults, "should have captured tool call result")
		assert.Equal(t, "vfs.write", handler.ToolCallResults[0].Call.Function)
		// Should have markdown chunks from the final response
		assert.NotEmpty(t, handler.MarkdownChunks, "should have captured markdown chunks")
		assert.Contains(t, handler.MarkdownChunks, "File created successfully.")
	})
}
