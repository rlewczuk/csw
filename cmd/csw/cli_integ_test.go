package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/presenter"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/ui/logmd"
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

// mockChatView is a mock implementation of ui.IChatView for testing
type mockChatView struct {
	mu         sync.Mutex
	messages   []*ui.ChatMessageUI
	initCalled bool
}

func newMockChatView() *mockChatView {
	return &mockChatView{
		messages: make([]*ui.ChatMessageUI, 0),
	}
}

func (m *mockChatView) Init(session *ui.ChatSessionUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initCalled = true
	m.messages = append(m.messages, session.Messages...)
	return nil
}

func (m *mockChatView) AddMessage(msg *ui.ChatMessageUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockChatView) UpdateMessage(msg *ui.ChatMessageUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, existing := range m.messages {
		if existing.Id == msg.Id {
			m.messages[i] = msg
			return nil
		}
	}
	return nil
}

func (m *mockChatView) UpdateTool(tool *ui.ToolUI) error {
	return nil
}

func (m *mockChatView) MoveToBottom() error {
	return nil
}

func (m *mockChatView) QueryPermission(query *ui.PermissionQueryUI) error {
	return nil
}

func (m *mockChatView) GetMessages() []*ui.ChatMessageUI {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*ui.ChatMessageUI, len(m.messages))
	copy(result, m.messages)
	return result
}

// TestLogmdChatViewLogsSession tests that LogmdChatView properly logs session activity to markdown.
func TestLogmdChatViewLogsSession(t *testing.T) {
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

	// Create session file
	sessionFilePath := filepath.Join(tmpDir, "session.md")
	sessionFile, err := os.Create(sessionFilePath)
	require.NoError(t, err)
	defer sessionFile.Close()

	// Create thread
	thread := core.NewSessionThread(system, nil)

	// Start session
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	// Create base presenter and view
	basePresenter := presenter.NewChatPresenter(system, thread)
	baseView := newMockChatView()

	// Wrap with LogmdChatView
	mu := &sync.Mutex{}
	logView := logmd.NewLogmdChatView(baseView, sessionFile, mu)

	// Set view on presenter
	err = basePresenter.SetView(logView)
	require.NoError(t, err)

	// Set output handler to the presenter (so it receives assistant messages)
	thread.SetOutputHandler(basePresenter)

	// Send user message through the presenter
	userMsg := &ui.ChatMessageUI{
		Role: ui.ChatRoleUser,
		Text: "Hello, this is a test prompt",
	}
	err = basePresenter.SendUserMessage(userMsg)
	require.NoError(t, err)

	// Wait for completion using a done channel
	done := make(chan struct{})
	go func() {
		// Poll for completion
		for {
			if !thread.IsRunning() {
				close(done)
				return
			}
		}
	}()
	<-done

	// Read session file content
	content, err := os.ReadFile(sessionFilePath)
	require.NoError(t, err)

	contentStr := string(content)

	// Verify the session file contains the conversation
	assert.Contains(t, contentStr, "# Chat Session", "session file should contain header")
	assert.Contains(t, contentStr, "## User", "session file should contain user message header")
	assert.Contains(t, contentStr, "Hello, this is a test prompt", "session file should contain the user prompt")
	assert.Contains(t, contentStr, "## Assistant", "session file should contain assistant message header")
	assert.NotEmpty(t, strings.TrimSpace(contentStr), "session file should not be empty")
}

// TestLogmdChatPresenterLogsCalls tests that LogmdChatPresenter properly logs method calls to markdown.
func TestLogmdChatPresenterLogsCalls(t *testing.T) {
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

	// Create session file
	sessionFilePath := filepath.Join(tmpDir, "session.md")
	sessionFile, err := os.Create(sessionFilePath)
	require.NoError(t, err)
	defer sessionFile.Close()

	// Create thread
	thread := core.NewSessionThread(system, nil)

	// Start session
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	// Create base presenter and view
	basePresenter := presenter.NewChatPresenter(system, thread)
	baseView := newMockChatView()

	// Wrap presenter with LogmdChatPresenter
	mu := &sync.Mutex{}
	logPresenter := logmd.NewLogmdChatPresenter(basePresenter, sessionFile, mu)

	// Set view on wrapped presenter
	err = logPresenter.SetView(baseView)
	require.NoError(t, err)

	// Set output handler on thread (use base presenter for output handling)
	thread.SetOutputHandler(basePresenter)

	// Send user message through the wrapped presenter
	userMsg := &ui.ChatMessageUI{
		Role: ui.ChatRoleUser,
		Text: "Hello, this is a test prompt",
	}
	err = logPresenter.SendUserMessage(userMsg)
	require.NoError(t, err)

	// Wait for completion using a done channel
	done := make(chan struct{})
	go func() {
		// Poll for completion
		for {
			if !thread.IsRunning() {
				close(done)
				return
			}
		}
	}()
	<-done

	// Read session file content
	content, err := os.ReadFile(sessionFilePath)
	require.NoError(t, err)

	contentStr := string(content)

	// Verify the session file contains the logged method calls
	assert.Contains(t, contentStr, "## System", "session file should contain system message header")
	assert.Contains(t, contentStr, "SendUserMessage", "session file should contain SendUserMessage method call")
	assert.Contains(t, contentStr, "Hello, this is a test prompt", "session file should contain the user prompt text")
}

// TestLogmdWrappersIntegration tests the integration of LogmdChatView and LogmdChatPresenter.
func TestLogmdWrappersIntegration(t *testing.T) {
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

	// Create session file
	sessionFilePath := filepath.Join(tmpDir, "session.md")
	sessionFile, err := os.Create(sessionFilePath)
	require.NoError(t, err)
	defer sessionFile.Close()

	// Create thread
	thread := core.NewSessionThread(system, nil)

	// Start session
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	// Create base presenter and view
	basePresenter := presenter.NewChatPresenter(system, thread)
	baseView := newMockChatView()

	// Wrap both with logging wrappers
	mu := &sync.Mutex{}
	logView := logmd.NewLogmdChatView(baseView, sessionFile, mu)
	logPresenter := logmd.NewLogmdChatPresenter(basePresenter, sessionFile, mu)

	// Set wrapped view on wrapped presenter
	err = logPresenter.SetView(logView)
	require.NoError(t, err)

	// Set output handler on thread (use base presenter for output handling)
	thread.SetOutputHandler(basePresenter)

	// Send user message through the wrapped presenter
	userMsg := &ui.ChatMessageUI{
		Role: ui.ChatRoleUser,
		Text: "Hello, this is a test prompt",
	}
	err = logPresenter.SendUserMessage(userMsg)
	require.NoError(t, err)

	// Wait for completion using a done channel
	done := make(chan struct{})
	go func() {
		// Poll for completion
		for {
			if !thread.IsRunning() {
				close(done)
				return
			}
		}
	}()
	<-done

	// Read session file content
	content, err := os.ReadFile(sessionFilePath)
	require.NoError(t, err)

	contentStr := string(content)

	// Verify the session file contains both view and presenter logs
	assert.Contains(t, contentStr, "# Chat Session", "session file should contain chat session header")
	assert.Contains(t, contentStr, "## User", "session file should contain user message header")
	assert.Contains(t, contentStr, "Hello, this is a test prompt", "session file should contain the user prompt")
	assert.Contains(t, contentStr, "## Assistant", "session file should contain assistant message header")
	assert.Contains(t, contentStr, "## System", "session file should contain system message header (from presenter)")
	assert.Contains(t, contentStr, "SendUserMessage", "session file should contain SendUserMessage method call")
}

// TestSaveSessionWithWriterBuffer tests session saving using a buffer instead of file.
func TestSaveSessionWithWriterBuffer(t *testing.T) {
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

	// Use a buffer to capture the logged output
	var buf bytes.Buffer

	// Create thread
	thread := core.NewSessionThread(system, nil)

	// Start session
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	// Create base presenter and view
	basePresenter := presenter.NewChatPresenter(system, thread)
	baseView := newMockChatView()

	// Wrap both with logging wrappers using buffer
	mu := &sync.Mutex{}
	logView := logmd.NewLogmdChatView(baseView, &buf, mu)
	logPresenter := logmd.NewLogmdChatPresenter(basePresenter, &buf, mu)

	// Set wrapped view on wrapped presenter
	err = logPresenter.SetView(logView)
	require.NoError(t, err)

	// Set output handler on thread (use base presenter for output handling)
	thread.SetOutputHandler(basePresenter)

	// Send user message through the wrapped presenter
	userMsg := &ui.ChatMessageUI{
		Role: ui.ChatRoleUser,
		Text: "Hello, this is a test prompt",
	}
	err = logPresenter.SendUserMessage(userMsg)
	require.NoError(t, err)

	// Wait for completion using a done channel
	done := make(chan struct{})
	go func() {
		// Poll for completion
		for {
			if !thread.IsRunning() {
				close(done)
				return
			}
		}
	}()
	<-done

	contentStr := buf.String()

	// Verify the buffer contains both view and presenter logs
	assert.Contains(t, contentStr, "# Chat Session", "buffer should contain chat session header")
	assert.Contains(t, contentStr, "## User", "buffer should contain user message header")
	assert.Contains(t, contentStr, "Hello, this is a test prompt", "buffer should contain the user prompt")
	assert.Contains(t, contentStr, "## Assistant", "buffer should contain assistant message header")
	assert.Contains(t, contentStr, "## System", "buffer should contain system message header (from presenter)")
	assert.Contains(t, contentStr, "SendUserMessage", "buffer should contain SendUserMessage method call")
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
