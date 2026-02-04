package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPromptGenerator is a simple mock implementation of PromptGenerator for testing
type mockPromptGenerator struct {
	prompt string
}

func newMockPromptGenerator(prompt string) *mockPromptGenerator {
	return &mockPromptGenerator{prompt: prompt}
}

func (m *mockPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *core.AgentState) (string, error) {
	return m.prompt, nil
}

func (m *mockPromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *core.AgentState) (tool.ToolInfo, error) {
	schema := tool.NewToolSchema()
	return tool.ToolInfo{
		Name:        toolName,
		Description: "Mock tool for testing",
		Schema:      schema,
	}, nil
}

func (m *mockPromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	return make(map[string]string), nil
}

func TestSaveSessionFlagWritesConversation(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()

	// Create provider config pointing to mock server
	providerConfig := &conf.ModelProviderConfig{
		Type: "ollama",
		Name: "ollama",
		URL:  mockServer.URL(),
	}

	client, err := models.NewOllamaClient(providerConfig)
	require.NoError(t, err)

	vfsInstance := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance)

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	system := &core.SweSystem{
		ModelProviders:  map[string]models.ModelProvider{"ollama": client},
		ModelTags:       models.NewModelTagRegistry(),
		PromptGenerator: newMockPromptGenerator("You are a helpful assistant."),
		Tools:           tools,
		VFS:             vfsInstance,
		WorkDir:         tmpDir,
		LogBaseDir:      logsDir,
	}

	// Set up mock streaming response
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello! I received your test prompt."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Simulate the bug: create a regular thread first (without writer)
	// Then create a new thread with writer (simulating --save-session behavior)
	thread := core.NewSessionThread(system, nil)

	// Start session
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	// Add a user prompt (this would be the initial prompt)
	err = thread.UserPrompt("Hello, this is a test prompt")
	require.NoError(t, err)

	// Wait for completion
	handler := testutil.NewMockSessionOutputHandler()
	thread.SetOutputHandler(handler)
	handler.WaitForRunFinished()

	// Now simulate what happens with --save-session flag:
	// A new thread is created with writer, but messages are not transferred
	sessionLogDir := filepath.Join(logsDir, "sessions", thread.GetSession().ID())
	sessionFilePath := filepath.Join(sessionLogDir, "session.md")

	// Create new thread with writer (this is what the buggy code does)
	writerThread, err := core.NewSessionThreadWithWriter(system, nil, sessionFilePath)
	require.NoError(t, err)
	defer writerThread.Close()

	// Start new session (bug: this creates a NEW empty session)
	err = writerThread.StartSession("ollama/test-model")
	require.NoError(t, err)

	// The bug: the new session has NO messages from the old session
	// So when we check the session file, it should be empty or missing the conversation
	content, err := os.ReadFile(sessionFilePath)
	if err == nil {
		// If file exists, it should be empty or just have headers without the conversation
		contentStr := string(content)
		// This assertion demonstrates the bug - the conversation is missing
		assert.NotContains(t, contentStr, "Hello, this is a test prompt",
			"BUG: With current implementation, the session file should NOT contain the user prompt because the session was recreated")
	}
}

func TestSaveSessionWithTransferredMessages(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()

	// Create provider config pointing to mock server
	providerConfig := &conf.ModelProviderConfig{
		Type: "ollama",
		Name: "ollama",
		URL:  mockServer.URL(),
	}

	client, err := models.NewOllamaClient(providerConfig)
	require.NoError(t, err)

	vfsInstance := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance)

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	system := &core.SweSystem{
		ModelProviders:  map[string]models.ModelProvider{"ollama": client},
		ModelTags:       models.NewModelTagRegistry(),
		PromptGenerator: newMockPromptGenerator("You are a helpful assistant."),
		Tools:           tools,
		VFS:             vfsInstance,
		WorkDir:         tmpDir,
		LogBaseDir:      logsDir,
	}

	// Set up mock streaming response
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello! I received your test prompt."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Create a thread with writer from the start (correct behavior)
	sessionLogDir := filepath.Join(logsDir, "sessions", "test-session")
	sessionFilePath := filepath.Join(sessionLogDir, "session.md")

	// Create output handler to track completion
	handler := testutil.NewMockSessionOutputHandler()

	writerThread, err := core.NewSessionThreadWithWriter(system, handler, sessionFilePath)
	require.NoError(t, err)
	defer writerThread.Close()

	// Start session
	err = writerThread.StartSession("ollama/test-model")
	require.NoError(t, err)

	// Add a user prompt
	err = writerThread.UserPrompt("Hello, this is a test prompt")
	require.NoError(t, err)

	// Wait for completion
	handler.WaitForRunFinished()

	// Verify the session file contains the conversation
	content, err := os.ReadFile(sessionFilePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# User", "session file should contain user message header")
	assert.Contains(t, contentStr, "Hello, this is a test prompt", "session file should contain the user prompt")
	assert.Contains(t, contentStr, "# Assistant", "session file should contain assistant message header")
	assert.NotEmpty(t, strings.TrimSpace(contentStr), "session file should not be empty")
}

// TestSaveSessionWithExistingSession tests the fix for the --save-session bug.
// It simulates the scenario where a session is started first, then wrapped with
// a writer to preserve the conversation history.
func TestSaveSessionWithExistingSession(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()

	// Create provider config pointing to mock server
	providerConfig := &conf.ModelProviderConfig{
		Type: "ollama",
		Name: "ollama",
		URL:  mockServer.URL(),
	}

	client, err := models.NewOllamaClient(providerConfig)
	require.NoError(t, err)

	vfsInstance := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance)

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	system := &core.SweSystem{
		ModelProviders:  map[string]models.ModelProvider{"ollama": client},
		ModelTags:       models.NewModelTagRegistry(),
		PromptGenerator: newMockPromptGenerator("You are a helpful assistant."),
		Tools:           tools,
		VFS:             vfsInstance,
		WorkDir:         tmpDir,
		LogBaseDir:      logsDir,
	}

	// Set up mock streaming response
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello! I received your test prompt."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Step 1: Create a regular thread (simulating --save-session behavior before fix)
	handler := testutil.NewMockSessionOutputHandler()
	thread := core.NewSessionThread(system, handler)

	// Step 2: Start session and add user prompt
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	err = thread.UserPrompt("Hello, this is a test prompt")
	require.NoError(t, err)

	// Wait for completion
	handler.WaitForRunFinished()

	// Step 3: Now simulate what happens with --save-session flag (after fix)
	// Get the existing session and wrap it with a writer
	session := thread.GetSession()
	require.NotNil(t, session)

	sessionLogDir := filepath.Join(logsDir, "sessions", session.ID())
	sessionFilePath := filepath.Join(sessionLogDir, "session.md")

	// Create new handler for the writer thread
	writerHandler := testutil.NewMockSessionOutputHandler()

	// Use the new function to create a thread with writer that preserves the session
	writerThread, err := core.NewSessionThreadWithWriterAndSession(system, session, writerHandler, sessionFilePath)
	require.NoError(t, err)
	defer writerThread.Close()

	// The session should already have the conversation history
	messages := session.ChatMessages()
	require.Greater(t, len(messages), 0, "session should have messages from the conversation")

	// Verify the session file was created and contains the conversation
	// Note: The user message was already processed before the writer was attached,
	// so it won't be in the file. But we can verify the session still has the messages.
	// In real usage with --save-session, the conversation happens AFTER the writer is attached.

	// For a more realistic test, let's add another message after the writer is attached
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"I can help you with that!"},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	err = writerThread.UserPrompt("Can you help me?")
	require.NoError(t, err)

	writerHandler.WaitForRunFinished()

	// Verify the session file contains the new conversation
	content, err := os.ReadFile(sessionFilePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# User", "session file should contain user message header")
	assert.Contains(t, contentStr, "Can you help me?", "session file should contain the second user prompt")
	assert.Contains(t, contentStr, "# Assistant", "session file should contain assistant message header")
	assert.NotEmpty(t, strings.TrimSpace(contentStr), "session file should not be empty")
}

func TestKimiTagResolution(t *testing.T) {
	// Create embedded config store (which should have our modified global.json)
	store, err := impl.NewEmbeddedConfigStore()
	require.NoError(t, err)

	// Create empty provider registry
	providerRegistry := models.NewProviderRegistry(store)

	// Create model tag registry
	registry, err := CreateModelTagRegistry(store, providerRegistry)
	require.NoError(t, err)

	// Test cases
	tests := []struct {
		modelName     string
		expectedTag   string
		shouldContain bool
	}{
		{
			modelName:     "kimi/kimi-for-coding",
			expectedTag:   "kimi",
			shouldContain: true,
		},
		{
			modelName:     "kimi/moonshot-v1-8k",
			expectedTag:   "kimi",
			shouldContain: true,
		},
		{
			modelName:     "openai/gpt-4",
			expectedTag:   "openai",
			shouldContain: true,
		},
		{
			modelName:     "anthropic/claude-3-opus",
			expectedTag:   "anthropic",
			shouldContain: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.modelName, func(t *testing.T) {
			// Split provider/model
			var provider, model string
			parts := strings.Split(tc.modelName, "/")
			if len(parts) == 2 {
				provider = parts[0]
				model = parts[1]
			} else {
				model = tc.modelName
			}

			tags := registry.GetTagsForModel(provider, model)
			if tc.shouldContain {
				assert.Contains(t, tags, tc.expectedTag, "Model %s (provider=%s, model=%s) should have tag %s. Got: %v", tc.modelName, provider, model, tc.expectedTag, tags)
			} else {
				assert.NotContains(t, tags, tc.expectedTag, "Model %s should NOT have tag %s", tc.modelName, tc.expectedTag)
			}
		})
	}
}

func TestSystemPromptGenerationForKimi(t *testing.T) {
	// Setup config store with embedded config (including our fix)
	store, err := impl.NewEmbeddedConfigStore()
	require.NoError(t, err)

	// Setup VFS
	vfsInstance := vfs.NewMockVFS()

	// Setup prompt generator
	generator, err := core.NewConfPromptGenerator(store, vfsInstance)
	require.NoError(t, err)

	// Setup tags for kimi
	tags := []string{"kimi"}

	// Get developer role
	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	developerRole := roleConfigs["developer"]

	// Get state
	state := core.AgentState{
		Info: core.AgentStateCommonInfo{
			AgentName: "Kimi",
		},
	}

	// Generate prompt
	prompt, err := generator.GetPrompt(tags, developerRole, &state)
	require.NoError(t, err)

	// Verify it contains Kimi specific instructions
	// Content from 10-system-kimi.md
	assert.Contains(t, prompt, "interactive general AI agent")

	// Verify it does NOT contain generic instructions (50-system-generic.md is tagged 'generic')
	// The prompt generator excludes fragments with tags that are not in the provided tags list
	// 50-system-generic.md starts with "You are {{.Info.AgentName}}, an interactive CLI tool"
	assert.NotContains(t, prompt, "an interactive CLI tool")
}
