package core

import (
	"context"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Populate mock server with LLM responses
		// First response: assistant makes a tool call to write the file
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"vfs.write","arguments":{"path":"hello_world.py","content":"print(\"Hello World\")\n"}}}]},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Second response: after tool execution, assistant confirms completion
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"I've created the Hello World program in Python."},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
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
		mockHandler := testutil.NewMockSessionOutputHandler()

		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are skilled software developer.",
			Tools:          tools,
			VFS:            vfs,
		}

		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Populate mock server with LLM responses
		// First response: assistant makes a tool call to write the file
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","tool_calls":[{"function":{"name":"vfs.write","arguments":{"path":"test.txt","content":"test content"}}}]},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Second response: after tool execution, assistant confirms completion
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"File created successfully."},"done":false}`,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		err = session.UserPrompt("Create a test file")
		assert.NoError(t, err)

		err = session.Run(context.Background())
		assert.NoError(t, err)

		// Verify UI handler captured the events
		// Should have at least one tool call start
		assert.NotEmpty(t, mockHandler.ToolCallStarts, "should have captured tool call start")
		// Should have tool call details
		assert.NotEmpty(t, mockHandler.ToolCallDetails, "should have captured tool call details")
		// Should have tool call result
		assert.NotEmpty(t, mockHandler.ToolCallResults, "should have captured tool call result")
		assert.Equal(t, "vfs.write", mockHandler.ToolCallResults[0].Call.Function)
		// Should have markdown chunks from the final response
		assert.NotEmpty(t, mockHandler.MarkdownChunks, "should have captured markdown chunks")
		assert.Contains(t, mockHandler.MarkdownChunks, "File created successfully.")
	})
}
