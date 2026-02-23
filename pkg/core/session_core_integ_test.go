// session_core_integ_test.go contains core session functionality integration tests
// including streaming mode, system prompts, LLM logging, reasoning content, and rate limiting.
package core

import (
	"context"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionSystemPrompt tests that system prompt is correctly set when creating
// a session with default role and when changing roles.
func TestSessionSystemPrompt(t *testing.T) {
	// Create mock config store with roles
	configStore := impl.NewMockConfigStore()
	developerRole := &conf.AgentRoleConfig{
		Name:        "developer",
		Description: "Software developer role",
	}
	testerRole := &conf.AgentRoleConfig{
		Name:        "tester",
		Description: "QA tester role",
	}
	configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
		"developer": developerRole,
		"tester":    testerRole,
	})

	roleRegistry := NewAgentRoleRegistry(configStore)
	fixture := newSweSystemFixture(t, "You are a skilled software developer.", withConfigStore(configStore), withRoles(roleRegistry))
	system := fixture.system

	t.Run("system prompt is set when creating session with default role", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Verify role was set
		assert.NotNil(t, session.role)
		assert.Equal(t, "developer", session.role.Name)

		// Verify system prompt was added to messages
		require.Greater(t, len(session.messages), 0, "session should have at least one message (system prompt)")
		assert.Equal(t, models.ChatRoleSystem, session.messages[0].Role, "first message should be system prompt")

		// Verify the system prompt content
		systemPrompt := session.messages[0].GetText()
		assert.Contains(t, systemPrompt, "You are a skilled software developer.", "system prompt should contain the role description")
	})

	t.Run("system prompt is updated when changing role", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)

		// Verify initial role and system prompt
		assert.Equal(t, "developer", session.role.Name)
		require.Greater(t, len(session.messages), 0)
		assert.Equal(t, models.ChatRoleSystem, session.messages[0].Role)
		initialPrompt := session.messages[0].GetText()

		// Add a user message to simulate conversation
		session.UserPrompt("Hello")

		// Change role to tester
		err = session.SetRole("tester")
		require.NoError(t, err)

		// Verify role was changed
		assert.Equal(t, "tester", session.role.Name)

		// Verify system prompt is still the first message
		require.Greater(t, len(session.messages), 0)
		assert.Equal(t, models.ChatRoleSystem, session.messages[0].Role, "first message should still be system prompt after role change")

		// Verify system prompt was updated (should be the same since our mock returns same prompt)
		newPrompt := session.messages[0].GetText()
		assert.Equal(t, initialPrompt, newPrompt, "system prompt should be maintained when changing role")

		// Verify user message is still there
		require.Greater(t, len(session.messages), 1, "user message should still exist after role change")
		assert.Equal(t, models.ChatRoleUser, session.messages[1].Role)
	})

	t.Run("system prompt persists when setting same role twice", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/devstral-small-2:latest", mockHandler)
		require.NoError(t, err)

		// Verify initial role and system prompt
		assert.Equal(t, "developer", session.role.Name)
		require.Greater(t, len(session.messages), 0)
		assert.Equal(t, models.ChatRoleSystem, session.messages[0].Role)
		initialPrompt := session.messages[0].GetText()

		// Set the same role again (this happens in CLI)
		err = session.SetRole("developer")
		require.NoError(t, err)

		// Verify role is still set
		assert.Equal(t, "developer", session.role.Name)

		// Verify system prompt is still there
		require.Greater(t, len(session.messages), 0, "system prompt should still exist")
		assert.Equal(t, models.ChatRoleSystem, session.messages[0].Role, "first message should still be system prompt")

		// Verify system prompt content hasn't changed
		newPrompt := session.messages[0].GetText()
		assert.Equal(t, initialPrompt, newPrompt, "system prompt should be the same")
	})
}

func TestSessionLLMLoggerUsage(t *testing.T) {
	t.Run("runNonStreamingChat uses llmLogger when enabled", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a test assistant.", withLogLLMRequests(true))
		system := fixture.system
		mockServer := fixture.server

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Verify llmLogger is set
		assert.NotNil(t, session.llmLogger, "llmLogger should be set")

		// Setup non-streaming response
		mockServer.AddRestResponse("/api/chat", "POST", `{"model":"test-model:latest","message":{"role":"assistant","content":"Hello!"},"done":true}`)

		// Set session to non-streaming mode

		// Add a user message
		session.UserPrompt("Hi there")

		// Run the session - this should use llmLogger in runNonStreamingChat
		err = session.Run(t.Context())
		require.NoError(t, err)

		// Verify the response was processed
		assert.NotEmpty(t, mockHandler.AssistantMessages)
		assert.Contains(t, mockHandler.AssistantMessages[len(mockHandler.AssistantMessages)-1].Text, "Hello!")
	})

	t.Run("runStreamingChat uses llmLogger when enabled", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a test assistant.", withLogLLMRequests(true))
		system := fixture.system
		mockServer := fixture.server

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Verify llmLogger is set
		assert.NotNil(t, session.llmLogger, "llmLogger should be set")

		// Setup streaming response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"!"},"done":true,"done_reason":"stop"}`,
		)

		// Add a user message
		session.UserPrompt("Hi there")

		// Run the session - this should use llmLogger in runStreamingChat
		err = session.Run(t.Context())
		require.NoError(t, err)

		// Verify the response was processed
		assert.NotEmpty(t, mockHandler.AssistantMessages)
		assert.Contains(t, mockHandler.AssistantMessages[len(mockHandler.AssistantMessages)-1].Text, "Hello!")
	})

	t.Run("runNonStreamingChat works without llmLogger", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a test assistant.", withLogLLMRequests(false))
		system := fixture.system
		mockServer := fixture.server

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Verify llmLogger is nil
		assert.Nil(t, session.llmLogger, "llmLogger should be nil")

		// Setup non-streaming response
		mockServer.AddRestResponse("/api/chat", "POST", `{"model":"test-model:latest","message":{"role":"assistant","content":"Hello!"},"done":true}`)

		// Set session to non-streaming mode

		// Add a user message
		session.UserPrompt("Hi there")

		// Run the session - this should work without llmLogger
		err = session.Run(t.Context())
		require.NoError(t, err)

		// Verify the response was processed
		assert.NotEmpty(t, mockHandler.AssistantMessages)
		assert.Contains(t, mockHandler.AssistantMessages[len(mockHandler.AssistantMessages)-1].Text, "Hello!")
	})

	t.Run("runStreamingChat works without llmLogger", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are a test assistant.", withLogLLMRequests(false))
		system := fixture.system
		mockServer := fixture.server

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", mockHandler)
		require.NoError(t, err)

		// Verify llmLogger is nil
		assert.Nil(t, session.llmLogger, "llmLogger should be nil")

		// Setup streaming response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":"!"},"done":true,"done_reason":"stop"}`,
		)

		// Add a user message
		session.UserPrompt("Hi there")

		// Run the session - this should work without llmLogger
		err = session.Run(t.Context())
		require.NoError(t, err)

		// Verify the response was processed
		assert.NotEmpty(t, mockHandler.AssistantMessages)
		assert.Contains(t, mockHandler.AssistantMessages[len(mockHandler.AssistantMessages)-1].Text, "Hello!")
	})
}

// TestSessionReasoningContent tests that reasoning content from previous messages
// is properly included in subsequent LLM calls.

// TestSessionRun_EmitsSingleMarkdownChunkForFragmentedResponse verifies that session
// emits one concatenated markdown chunk in non-streaming mode even when provider Chat()
// returns multiple text parts.
func TestSessionRun_EmitsSingleMarkdownChunkForFragmentedResponse(t *testing.T) {
	fixture := newSweSystemFixture(t, "You are a helpful assistant.")
	system := fixture.system

	mockProvider := models.NewMockProvider([]models.ModelInfo{
		{Name: "test-model", Model: "test-model"},
	})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: &models.ChatMessage{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{
				{Text: "clean"},
				{Text: ".sh"},
				{Text: " &&"},
				{Text: " go"},
				{Text: " test"},
			},
		},
	})
	system.ModelProviders = map[string]models.ModelProvider{"mock": mockProvider}

	mockHandler := testutil.NewMockSessionOutputHandler()
	session, err := system.NewSession("mock/test-model", mockHandler)
	require.NoError(t, err)

	err = session.UserPrompt("run tests")
	require.NoError(t, err)

	err = session.Run(context.Background())
	require.NoError(t, err)

	require.Len(t, mockHandler.AssistantMessages, 1)
	assert.Equal(t, "clean.sh && go test", mockHandler.AssistantMessages[0].Text)
}

func TestSessionReasoningContent(t *testing.T) {
	fixture := newSweSystemFixture(t, "You are a helpful assistant.")
	system := fixture.system
	vfsInstance := fixture.vfs

	err := vfsInstance.WriteFile("test.txt", []byte("file content"))
	require.NoError(t, err)

	t.Run("reasoning content is included in subsequent LLM call with tool result", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{
			{Name: "test-model", Model: "test-model"},
		})

		firstResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{ReasoningContent: "I need to read the file to help the user."},
					{ToolCall: &tool.ToolCall{
						ID:       "read-call-1",
						Function: "vfsRead",
						Arguments: tool.NewToolValue(map[string]any{
							"path": "test.txt",
						}),
					}},
				},
			},
		}

		secondResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{Text: "I have read the file."},
				},
			},
		}

		mockProvider.SetChatResponse("test-model", firstResponse)
		mockProvider.SetChatResponse("test-model", secondResponse)

		system.ModelProviders = map[string]models.ModelProvider{"mock": mockProvider}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)

		session.UserPrompt("Read the file test.txt")

		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()
		_ = session.Run(ctx)

		require.GreaterOrEqual(t, len(mockProvider.RecordedMessages), 2, "Should have at least 2 LLM calls")

		secondCallMessages := mockProvider.RecordedMessages[1]

		var foundAssistantWithReasoning bool
		for _, msg := range secondCallMessages {
			if msg.Role == models.ChatRoleAssistant {
				for _, part := range msg.Parts {
					if part.ReasoningContent == "I need to read the file to help the user." {
						foundAssistantWithReasoning = true
						break
					}
				}
			}
		}

		assert.True(t, foundAssistantWithReasoning, "Second LLM call should include reasoning content from previous assistant message")
	})

	t.Run("reasoning content is preserved in conversation history", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{
			{Name: "test-model", Model: "test-model"},
		})

		firstResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{ReasoningContent: "Thinking about the question..."},
					{Text: "Hello!"},
				},
			},
		}

		secondResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{Text: "Goodbye!"},
				},
			},
		}

		mockProvider.SetChatResponse("test-model", firstResponse)
		mockProvider.SetChatResponse("test-model", secondResponse)

		system.ModelProviders = map[string]models.ModelProvider{"mock": mockProvider}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)

		session.UserPrompt("Say hello")

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		_ = session.Run(ctx)
		cancel()

		session.UserPrompt("Say goodbye")

		ctx, cancel = context.WithTimeout(t.Context(), 100*time.Millisecond)
		_ = session.Run(ctx)
		cancel()

		require.GreaterOrEqual(t, len(mockProvider.RecordedMessages), 2, "Should have at least 2 LLM calls")

		secondCallMessages := mockProvider.RecordedMessages[1]

		var foundAssistantWithReasoning bool
		for _, msg := range secondCallMessages {
			if msg.Role == models.ChatRoleAssistant {
				for _, part := range msg.Parts {
					if part.ReasoningContent == "Thinking about the question..." {
						foundAssistantWithReasoning = true
						break
					}
				}
			}
		}

		assert.True(t, foundAssistantWithReasoning, "Second LLM call should include reasoning content from previous assistant message")
	})
}

func TestSweSession_RateLimitRetry(t *testing.T) {
	t.Run("retries on rate limit error with retry-after header", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})

		rateLimitErr := &models.RateLimitError{
			RetryAfterSeconds: 1,
			Message:           "rate limit exceeded",
		}

		mockProvider.RateLimitError = rateLimitErr
		rateLimitCount := 1
		mockProvider.RateLimitErrorCount = &rateLimitCount
		mockProvider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 5 * time.Millisecond}

		successResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{Text: "Success after retry!"},
				},
			},
		}
		mockProvider.SetChatResponse("test-model", successResponse)

		fixture := newSweSystemFixture(t, "You are a helpful assistant.",
			withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
			withGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 1}),
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := fixture.system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)

		session.UserPrompt("Hello")

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		err = session.Run(ctx)
		cancel()

		assert.NoError(t, err, "Session should complete successfully after retry")

		assert.Equal(t, 2, len(mockProvider.RecordedMessages), "Should have made 2 LLM calls (1 rate limit + 1 success)")

		if len(mockHandler.RateLimitErrors) > 0 {
			t.Logf("Rate limit errors received: %v", mockHandler.RateLimitErrors)
		}
	})

	t.Run("retries on rate limit error without retry-after using exponential backoff", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})

		rateLimitErr := &models.RateLimitError{
			RetryAfterSeconds: 0,
			Message:           "rate limit exceeded",
		}

		mockProvider.RateLimitError = rateLimitErr
		rateLimitCount := 1
		mockProvider.RateLimitErrorCount = &rateLimitCount
		mockProvider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 5 * time.Millisecond}

		successResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{Text: "Success after retry!"},
				},
			},
		}
		mockProvider.SetChatResponse("test-model", successResponse)

		fixture := newSweSystemFixture(t, "You are a helpful assistant.",
			withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
			withGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 1}),
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := fixture.system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)

		session.UserPrompt("Hello")

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		err = session.Run(ctx)
		cancel()

		assert.NoError(t, err, "Session should complete successfully after exponential backoff retry")
		assert.Equal(t, 2, len(mockProvider.RecordedMessages), "Should have made 2 LLM calls")
	})

	t.Run("fails after max retries exceeded", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})

		rateLimitErr := &models.RateLimitError{
			RetryAfterSeconds: 0,
			Message:           "rate limit exceeded",
		}

		mockProvider.RateLimitError = rateLimitErr
		rateLimitCount := 2
		mockProvider.RateLimitErrorCount = &rateLimitCount
		mockProvider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 5 * time.Millisecond}

		fixture := newSweSystemFixture(t, "You are a helpful assistant.",
			withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
			withGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 1}),
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := fixture.system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)

		session.UserPrompt("Hello")

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		err = session.Run(ctx)
		cancel()

		require.Error(t, err, "Session should fail after max retries")
		assert.Contains(t, err.Error(), "rate limit exceeded")
	})
}

func TestSweSession_NetworkErrorRetry(t *testing.T) {
	t.Run("retries on network error using exponential backoff", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})

		networkErr := &models.NetworkError{
			Message:     "connection refused",
			IsRetryable: true,
		}

		mockProvider.NetworkError = networkErr
		networkErrorCount := 1
		mockProvider.NetworkErrorCount = &networkErrorCount
		mockProvider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 5 * time.Millisecond}

		successResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{Text: "Success after retry!"},
				},
			},
		}
		mockProvider.SetChatResponse("test-model", successResponse)

		fixture := newSweSystemFixture(t, "You are a helpful assistant.",
			withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
			withGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 1}),
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := fixture.system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)

		session.UserPrompt("Hello")

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		err = session.Run(ctx)
		cancel()

		assert.NoError(t, err, "Session should complete successfully after retry")
		assert.Equal(t, 2, len(mockProvider.RecordedMessages), "Should have made 2 LLM calls (1 network error + 1 success)")

		if len(mockHandler.RateLimitErrors) > 0 {
			t.Logf("Network retry notifications received: %v", mockHandler.RateLimitErrors)
		}
	})

	t.Run("fails after max retries exceeded for network error", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})

		networkErr := &models.NetworkError{
			Message:     "connection refused",
			IsRetryable: true,
		}

		mockProvider.NetworkError = networkErr
		networkErrorCount := 2
		mockProvider.NetworkErrorCount = &networkErrorCount
		mockProvider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 5 * time.Millisecond}

		fixture := newSweSystemFixture(t, "You are a helpful assistant.",
			withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
			withGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 1}),
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := fixture.system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)

		session.UserPrompt("Hello")

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		err = session.Run(ctx)
		cancel()

		require.Error(t, err, "Session should fail after max retries")
		assert.Contains(t, err.Error(), "temporary LLM API failure")
	})
}

// TestSessionVFSMoveToolIntegration tests the vfsMove tool integration with the session.
// This test exposes a known bug where either tool call result or its ID are not
// propagated back to the LLM properly.
func TestSessionVFSMoveToolIntegration(t *testing.T) {
	fixture := newSweSystemFixture(t, "You are skilled software developer.")
	system := fixture.system
	vfsInstance := fixture.vfs

	// Setup test file in VFS
	err := vfsInstance.WriteFile("oldname.txt", []byte("hello world"))
	require.NoError(t, err)

	t.Run("vfsMove tool result is propagated to LLM with correct tool call ID", func(t *testing.T) {
		// Create mock provider that will record tool responses
		mockProvider := models.NewMockProvider([]models.ModelInfo{
			{Name: "test-model", Model: "test-model"},
		})

		// First response: assistant makes a tool call to vfsMove (using non-streaming response)
		firstResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{
						ToolCall: &tool.ToolCall{
							ID:       "move-call-123",
							Function: "vfsMove",
							Arguments: tool.NewToolValue(map[string]any{
								"path":        "oldname.txt",
								"destination": "newname.txt",
							}),
						},
					},
				},
			},
		}
		mockProvider.SetChatResponse("test-model", firstResponse)

		// Set up the system to use mock provider
		system.ModelProviders = map[string]models.ModelProvider{"mock": mockProvider}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)

		// Disable streaming for simpler test flow

		// Add a user message to trigger the conversation
		session.UserPrompt("Please rename oldname.txt to newname.txt")

		// Run the session - this should execute the vfsMove tool
		// Note: This will loop infinitely because the mock returns the same tool call
		// every time. We use a timeout context to stop after first iteration.
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		_ = session.Run(ctx)

		// Verify the file was moved in VFS (tool was executed)
		_, err = vfsInstance.ReadFile("oldname.txt")
		assert.Error(t, err, "oldname.txt should not exist after move")

		content, err := vfsInstance.ReadFile("newname.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))

		// Check that the tool response was recorded by the mock provider
		// This exposes the bug: either tool call result or its ID are not propagated back
		require.NotEmpty(t, mockProvider.RecordedToolResponses, "Tool responses should be recorded and sent back to LLM")

		// Verify the tool response contains the correct tool call ID
		foundMoveResponse := false
		for _, resp := range mockProvider.RecordedToolResponses {
			if resp.Call != nil && resp.Call.ID == "move-call-123" {
				foundMoveResponse = true
				require.True(t, resp.Done, "Tool response should be marked as done")
				require.NoError(t, resp.Error, "Tool response should not have an error")
				// Verify result contains expected fields
				resultMsg := resp.Result.Get("message").AsString()
				require.Equal(t, "File successfully moved", resultMsg)
				break
			}
		}
		assert.True(t, foundMoveResponse, "Tool response with matching tool call ID should be sent to LLM")
	})

	t.Run("vfsMove tool result contains proper result data", func(t *testing.T) {
		// Create mock provider that will record tool responses
		mockProvider := models.NewMockProvider([]models.ModelInfo{
			{Name: "test-model", Model: "test-model"},
		})

		// First response: assistant makes a tool call to vfsMove (using non-streaming response)
		firstResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{
						ToolCall: &tool.ToolCall{
							ID:       "move-call-456",
							Function: "vfsMove",
							Arguments: tool.NewToolValue(map[string]any{
								"path":        "oldname.txt",
								"destination": "anothername.txt",
							}),
						},
					},
				},
			},
		}
		mockProvider.SetChatResponse("test-model", firstResponse)

		// Reset VFS state
		err := vfsInstance.WriteFile("oldname.txt", []byte("hello world"))
		require.NoError(t, err)

		// Set up the system to use mock provider
		system.ModelProviders = map[string]models.ModelProvider{"mock": mockProvider}

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)

		// Disable streaming for simpler test flow

		// Add a user message to trigger the conversation
		session.UserPrompt("Please rename the file")

		// Run the session with timeout to prevent infinite loop
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		_ = session.Run(ctx)

		// Verify that tool responses were properly recorded
		require.NotEmpty(t, mockProvider.RecordedToolResponses, "Tool responses should be recorded")

		// Verify the response contains result with proper data
		for _, resp := range mockProvider.RecordedToolResponses {
			if resp.Call != nil && resp.Call.Function == "vfsMove" {
				// Check that result contains path and destination
				path := resp.Result.Get("path").AsString()
				dest := resp.Result.Get("destination").AsString()
				require.NotEmpty(t, path, "Result should contain path")
				require.NotEmpty(t, dest, "Result should contain destination")
			}
		}
	})
}
