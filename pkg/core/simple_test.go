package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/models/ollama"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chatResponseJSON(response ollama.ChatResponse) string {
	data, _ := json.Marshal(response)
	return string(data)
}

func TestAgentCoreInitializationAndSimpleProgramGen(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := ollama.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
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
			chatResponseJSON(ollama.ChatResponse{
				Model:     "devstral-small-2:latest",
				CreatedAt: "2024-01-01T00:00:00Z",
				Message: ollama.Message{
					Role: "assistant",
					ToolCalls: []ollama.ToolCall{
						{
							Function: ollama.ToolCallFunction{
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
			chatResponseJSON(ollama.ChatResponse{
				Model:     "devstral-small-2:latest",
				CreatedAt: "2024-01-01T00:00:01Z",
				Message: ollama.Message{
					Role: "assistant",
				},
				Done:       true,
				DoneReason: "stop",
			}),
		)

		// Second response: after tool execution, assistant confirms completion
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			chatResponseJSON(ollama.ChatResponse{
				Model:     "devstral-small-2:latest",
				CreatedAt: "2024-01-01T00:00:02Z",
				Message: ollama.Message{
					Role:    "assistant",
					Content: "I've created the Hello World program in Python.",
				},
				Done: false,
			}),
			chatResponseJSON(ollama.ChatResponse{
				Model:     "devstral-small-2:latest",
				CreatedAt: "2024-01-01T00:00:03Z",
				Message: ollama.Message{
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
}
