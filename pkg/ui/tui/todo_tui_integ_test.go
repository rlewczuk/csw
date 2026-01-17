package tui

import (
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/presenter"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock prompt generator for todo TUI tests
type todoTuiMockPromptGen struct{}

func (m *todoTuiMockPromptGen) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *core.AgentState) (string, error) {
	return "You are a helpful assistant with access to todo list management tools.", nil
}

func TestTodoToolsWithTui(t *testing.T) {
	t.Run("should write and read todo list via LLM", func(t *testing.T) {
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
			PromptGenerator: &todoTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		session := thread.GetSession()
		require.NotNil(t, session)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response that uses todo.write tool (Ollama format)
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"I'll create a todo list for you.","tool_calls":[{"function":{"name":"todo.write","arguments":{"todos":[{"id":"todo-1","content":"Implement feature X","status":"pending","priority":"high"},{"id":"todo-2","content":"Write tests","status":"in_progress","priority":"medium"}]}}}]},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Setup second LLM response after tool execution
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"Todo list created!"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()
		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Send user message
		term.SendString("Create a todo list")
		term.SendKey("alt+enter")

		// Wait for user message to appear
		assert.True(t, term.WaitForText("Create a todo list", 2*time.Second), "Should show user message")

		// Wait for tool call to be executed
		time.Sleep(1 * time.Second)

		// Verify that the todo list was updated in the session
		todos := session.GetTodoList()
		require.Len(t, todos, 2)
		assert.Equal(t, "todo-1", todos[0].ID)
		assert.Equal(t, "Implement feature X", todos[0].Content)
		assert.Equal(t, "pending", todos[0].Status)
		assert.Equal(t, "high", todos[0].Priority)

		assert.Equal(t, "todo-2", todos[1].ID)
		assert.Equal(t, "Write tests", todos[1].Content)
		assert.Equal(t, "in_progress", todos[1].Status)
		assert.Equal(t, "medium", todos[1].Priority)

		// Verify pending count
		assert.Equal(t, 2, session.CountPendingTodos())

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("should read todo list via LLM", func(t *testing.T) {
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
			PromptGenerator: &todoTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		session := thread.GetSession()
		require.NotNil(t, session)

		// Pre-populate todo list
		session.SetTodoList([]tool.TodoItem{
			{
				ID:       "existing-1",
				Content:  "Review PR",
				Status:   "pending",
				Priority: "high",
			},
		})

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response that uses todo.read tool (Ollama format)
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Let me check the todo list.","tool_calls":[{"function":{"name":"todo.read","arguments":{}}}]},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Setup second LLM response after tool result
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"You have 1 pending task: Review PR"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()
		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Send user message
		term.SendString("Show me my todos")
		term.SendKey("alt+enter")

		// Wait for user message to appear
		assert.True(t, term.WaitForText("Show me my todos", 2*time.Second), "Should show user message")

		// Wait for response
		assert.True(t, term.WaitForText("You have 1 pending task", 5*time.Second), "Should show LLM response about todos")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("should update todo status via LLM", func(t *testing.T) {
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
			PromptGenerator: &todoTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		session := thread.GetSession()
		require.NotNil(t, session)

		// Pre-populate todo list
		session.SetTodoList([]tool.TodoItem{
			{
				ID:       "task-1",
				Content:  "Fix bug",
				Status:   "in_progress",
				Priority: "high",
			},
			{
				ID:       "task-2",
				Content:  "Update docs",
				Status:   "pending",
				Priority: "low",
			},
		})

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response that updates todo status to completed (Ollama format)
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"I'll mark the task as completed.","tool_calls":[{"function":{"name":"todo.write","arguments":{"todos":[{"id":"task-1","content":"Fix bug","status":"completed","priority":"high"},{"id":"task-2","content":"Update docs","status":"pending","priority":"low"}]}}}]},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Setup second LLM response after tool execution
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"Task marked as completed!"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()
		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Send user message
		term.SendString("Mark bug fix as completed")
		term.SendKey("alt+enter")

		// Wait for user message to appear
		assert.True(t, term.WaitForText("Mark bug fix as completed", 2*time.Second), "Should show user message")

		// Wait for tool call to be executed
		time.Sleep(1 * time.Second)

		// Verify that the todo list was updated
		todos := session.GetTodoList()
		require.Len(t, todos, 2)
		assert.Equal(t, "completed", todos[0].Status)
		assert.Equal(t, "pending", todos[1].Status)

		// Verify pending count decreased to 1
		assert.Equal(t, 1, session.CountPendingTodos())

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("should handle tool call result correctly", func(t *testing.T) {
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
			PromptGenerator: &todoTuiMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		session := thread.GetSession()
		require.NotNil(t, session)

		// Create presenter
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TUI chat view
		tuiView, err := NewTuiChatView(chatPresenter)
		require.NoError(t, err)

		// Connect presenter to view
		err = chatPresenter.SetView(tuiView)
		require.NoError(t, err)

		// Setup LLM response with todo.write tool call (Ollama format)
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Creating todo.","tool_calls":[{"function":{"name":"todo.write","arguments":{"todos":[{"id":"new-task","content":"New task","status":"pending","priority":"medium"}]}}}]},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Setup second LLM response after tool result
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"Done! I've created the task."},"done":true,"done_reason":"stop"}`,
		)

		// Create mock terminal
		term := NewTerminalMock()
		term.Run(tuiView.Model())

		// Wait for welcome message
		assert.True(t, term.WaitForText("Welcome!", 2*time.Second), "Should show welcome message")

		// Send user message
		term.SendString("Add a new task")
		term.SendKey("alt+enter")

		// Wait for response
		assert.True(t, term.WaitForText("Done! I've created the task", 5*time.Second), "Should show LLM confirmation")

		// Verify the messages in the session include the tool response
		messages := session.ChatMessages()
		var foundToolResponse bool
		for _, msg := range messages {
			// Tool responses are stored as user role messages with ToolResponse parts
			if msg.Role == models.ChatRoleUser {
				responses := msg.GetToolResponses()
				if len(responses) > 0 {
					foundToolResponse = true
					break
				}
			}
		}
		assert.True(t, foundToolResponse, "Tool response should be in session messages")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})
}
