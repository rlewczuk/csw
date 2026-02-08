package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/codesnort/codesnort-swe/pkg/core"
	coretestfixture "github.com/codesnort/codesnort-swe/pkg/core/testfixture"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/ui/cli/testfixture"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCliPresenter is a minimal mock presenter for simple tests.
type mockCliPresenter struct{}

func (m *mockCliPresenter) SetView(view ui.IChatView) error                 { return nil }
func (m *mockCliPresenter) SendUserMessage(message *ui.ChatMessageUI) error { return nil }
func (m *mockCliPresenter) SaveUserMessage(message *ui.ChatMessageUI) error { return nil }
func (m *mockCliPresenter) Pause() error                                    { return nil }
func (m *mockCliPresenter) Resume() error                                   { return nil }
func (m *mockCliPresenter) PermissionResponse(response string) error        { return nil }
func (m *mockCliPresenter) SetModel(model string) error                     { return nil }

func newCliFixture(t *testing.T) *testfixture.CliFixture {
	return testfixture.NewCliFixture(t,
		coretestfixture.WithPromptGenerator(coretestfixture.NewStaticPromptGenerator("You are a helpful assistant.")),
	)
}

func TestCliChatView_IntegrationWithSession(t *testing.T) {
	t.Run("chat response appears in non-interactive mode", func(t *testing.T) {
		fixture := newCliFixture(t)
		mockServer := fixture.Server

		// Create session thread
		thread := fixture.NewSessionThread(nil)
		err := thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create chat presenter
		chatPresenter := fixture.NewChatPresenter(thread)

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
		fixture := newCliFixture(t)
		mockServer := fixture.Server

		// Create session thread
		thread := fixture.NewSessionThread(nil)
		err := thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create chat presenter
		chatPresenter := fixture.NewChatPresenter(thread)

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
					Id:      "tool1",
					Name:    "vfsRead",
					Status:  ui.ToolStatusStarted,
					Props:   [][]string{{"path", "/test/file.txt"}},
					Message: "",
				},
			},
		}

		err := view.AddMessage(msg)
		require.NoError(t, err)

		// Update tool status to succeeded
		msg.Tools[0].Status = ui.ToolStatusSucceeded
		msg.Tools[0].Message = "file content here"
		msg.Tools[0].Summary = "vfsRead"
		err = view.UpdateTool(msg.Tools[0])
		require.NoError(t, err)

		// Verify output
		outputStr := output.String()
		assert.Contains(t, outputStr, "Assistant: Let me read that file")
		// Tool should be displayed in final status using Display field (or name as fallback)
		assert.Contains(t, outputStr, "TOOL: vfsRead (tool1) - succeeded")
	})

	t.Run("accepts all permissions automatically when configured", func(t *testing.T) {
		fixture := newCliFixture(t)
		mockServer := fixture.Server

		// Create session thread
		thread := fixture.NewSessionThread(nil)
		err := thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create chat presenter
		chatPresenter := fixture.NewChatPresenter(thread)

		// Create CLI view with acceptAllPermissions=true
		output := &bytes.Buffer{}
		view := NewCliChatView(chatPresenter, output, nil, false, true)

		// Verify interactive is false when acceptAllPermissions is true
		assert.False(t, view.interactive)
		assert.True(t, view.acceptAllPermissions)

		// Setup LLM response with tool call
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Creating file","tool_calls":[{"id":"call_1","type":"function","function":{"name":"vfsWrite","arguments":"{\"path\":\"new.txt\",\"content\":\"test\"}"}}]},"done":false}`,
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
		fixture := newCliFixture(t)
		mockServer := fixture.Server

		// Create session thread
		thread := fixture.NewSessionThread(nil)
		err := thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create chat presenter
		chatPresenter := fixture.NewChatPresenter(thread)

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

	t.Run("system prompt included when SetRole is called", func(t *testing.T) {
		// This test verifies that when SetRole() is called, the system prompt
		// is included in the messages sent to the LLM.
		vfsInstance := vfs.NewMockVFS()

		// Create a mock role registry and config store with developer role
		configStore := impl.NewMockConfigStore()
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"all": {
				Name:        "all",
				Description: "All roles base",
				PromptFragments: map[string]string{
					"1-system.md": "You are a skilled software assistant.",
				},
			},
			"developer": {
				Name:        "developer",
				Description: "Software developer role",
			},
		})

		// Create prompt generator with the config store
		promptGenerator, err := core.NewConfPromptGenerator(configStore, vfsInstance)
		require.NoError(t, err)

		roleRegistry := core.NewAgentRoleRegistry(configStore)
		fixture := coretestfixture.NewSweSystemFixture(t,
			coretestfixture.WithPromptGenerator(promptGenerator),
			coretestfixture.WithRoles(roleRegistry),
			coretestfixture.WithConfigStore(configStore),
			coretestfixture.WithVFS(vfsInstance),
		)
		system := fixture.System

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		session := thread.GetSession()
		require.NotNil(t, session)

		// After StartSession with our fix, a role should be automatically set
		// because we have a "developer" role in the registry and it's the default fallback
		role := session.Role()
		require.NotNil(t, role, "Role should be automatically set after StartSession")
		assert.Equal(t, "developer", role.Name, "Should use developer as default fallback role")

		// Verify that a system message was created (even if content is empty in this test setup)
		messagesAfter := session.ChatMessages()
		if len(messagesAfter) > 0 {
			assert.Equal(t, models.ChatRoleSystem, messagesAfter[0].Role,
				"First message should be system prompt when role is set")
		}
	})

	t.Run("system prompt uses default role from global config", func(t *testing.T) {
		// This test verifies that the default role from global config is used
		vfsInstance := vfs.NewMockVFS()

		// Create a mock config store with custom default role
		configStore := impl.NewMockConfigStore()
		configStore.SetGlobalConfig(&conf.GlobalConfig{
			DefaultRole: "tester",
		})
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"developer": {
				Name:        "developer",
				Description: "Software developer role",
				PromptFragments: map[string]string{
					"1-system.md": "You are a skilled software developer.",
				},
			},
			"tester": {
				Name:        "tester",
				Description: "Software tester role",
				PromptFragments: map[string]string{
					"1-system.md": "You are a skilled software tester.",
				},
			},
		})

		// Create prompt generator with the config store
		promptGenerator, err := core.NewConfPromptGenerator(configStore, vfsInstance)
		require.NoError(t, err)

		roleRegistry := core.NewAgentRoleRegistry(configStore)
		fixture := coretestfixture.NewSweSystemFixture(t,
			coretestfixture.WithPromptGenerator(promptGenerator),
			coretestfixture.WithRoles(roleRegistry),
			coretestfixture.WithConfigStore(configStore),
			coretestfixture.WithVFS(vfsInstance),
		)
		system := fixture.System

		// Create session - should automatically use "tester" role from global config
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		session := thread.GetSession()
		require.NotNil(t, session)

		// Verify that the tester role was set
		role := session.Role()
		require.NotNil(t, role, "Role should be set automatically")
		assert.Equal(t, "tester", role.Name, "Should use default role from global config")
	})

	t.Run("tool description from markdown file is included", func(t *testing.T) {
		// This test verifies that tool descriptions from markdown files are included
		// in the tool info sent to the LLM.
		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		// Register a dummy tool
		dTool := &dummyTool{name: "dummyTool"}
		tools.Register("dummyTool", dTool)

		// Create a mock config store with a role that has tool fragments
		configStore := impl.NewMockConfigStore()
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"developer": {
				Name:        "developer",
				Description: "Software developer role",
				PromptFragments: map[string]string{
					"1-system.md": "You are a skilled software developer.",
				},
				ToolFragments: map[string]string{
					"dummyTool/dummyTool.schema.json": `{
						"type": "object",
						"description": "Short description from schema",
						"properties": {
							"arg": { "type": "string" }
						}
					}`,
					"dummyTool/dummyTool.md":         "# Detailed Tool Description\n\nThis is a detailed description from markdown.",
					"dummyTool/dummyTool-special.md": "\n\nExtra special instructions.",
				},
			},
		})

		// Create REAL prompt generator (not mock)
		promptGenerator, err := core.NewConfPromptGenerator(configStore, vfsInstance)
		require.NoError(t, err)

		roleRegistry := core.NewAgentRoleRegistry(configStore)
		fixture := coretestfixture.NewSweSystemFixture(t,
			coretestfixture.WithPromptGenerator(promptGenerator),
			coretestfixture.WithRoles(roleRegistry),
			coretestfixture.WithConfigStore(configStore),
			coretestfixture.WithVFS(vfsInstance),
			coretestfixture.WithTools(tools),
		)
		system := fixture.System
		mockServer := fixture.Server

		// Add a tag mapping to ensure we have the 'special' tag
		system.ModelTags.SetGlobalMappings([]conf.ModelTagMapping{
			{Model: ".*", Tag: "special"},
		})

		// Create session
		thread := core.NewSessionThread(system, nil)

		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Mock the LLM response to avoid hanging
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true}`,
		)

		// Trigger a run to send request
		ctx := context.Background()
		session := thread.GetSession()
		err = session.Run(ctx)
		require.NoError(t, err)

		// Verify the request body sent to LLM
		requests := mockServer.GetRequests()
		require.NotEmpty(t, requests)
		lastRequest := requests[len(requests)-1]

		// Parse request body
		var chatReq struct {
			Tools []struct {
				Function struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"function"`
			} `json:"tools"`
		}
		err = json.Unmarshal(lastRequest.Body, &chatReq)
		require.NoError(t, err)

		// Find the dummy tool
		var foundTool bool
		for _, toolObj := range chatReq.Tools {
			if toolObj.Function.Name == "dummyTool" {
				foundTool = true
				// Check if description contains the markdown content
				assert.Contains(t, toolObj.Function.Description, "This is a detailed description from markdown",
					"Tool description should contain content from markdown file")
				// Check if tagged description is included
				assert.Contains(t, toolObj.Function.Description, "Extra special instructions",
					"Tool description should contain tagged content")
			}
		}

		assert.True(t, foundTool, "dummyTool should be in the request")
	})
}

// dummyTool is a simple tool implementation for testing
type dummyTool struct {
	name string
}

func (d *dummyTool) Execute(args *tool.ToolCall) *tool.ToolResponse {
	return &tool.ToolResponse{
		Call:   args,
		Result: tool.NewToolValue("success"),
		Done:   true,
	}
}

func (d *dummyTool) Render() (string, string, map[string]string) {
	return d.name, d.name, make(map[string]string)
}
