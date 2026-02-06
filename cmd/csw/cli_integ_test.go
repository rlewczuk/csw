package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/presenter"
	"github.com/codesnort/codesnort-swe/pkg/runner"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/ui/logmd"
	"github.com/codesnort/codesnort-swe/pkg/ui/mock"
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

// TestRunBashToolIntegration tests that runBash tool is properly registered and presented to LLM.
func TestRunBashToolIntegration(t *testing.T) {
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

	// Register runBash tool with mock runner
	mockRunner := runner.NewMockRunner()
	mockRunner.SetDefaultResponse("test output", 0, nil)
	tool.RegisterRunBashTool(tools, mockRunner, map[string]conf.AccessFlag{
		"echo .*": conf.AccessAllow,
	})

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

	// Set up mock streaming response with tool call
	// First response: assistant makes a tool call to run bash command (closeAfter=false to continue processing)
	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"runBash","arguments":{"command":"echo hello"}}}]},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Second response: after tool execution, assistant confirms completion
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"Command executed successfully."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Create thread
	thread := core.NewSessionThread(system, nil)

	// Start session
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	// Create presenter and view
	basePresenter := presenter.NewChatPresenter(system, thread)
	baseView := newMockChatView()

	err = basePresenter.SetView(baseView)
	require.NoError(t, err)

	// Set output handler
	thread.SetOutputHandler(basePresenter)

	// Send user message
	userMsg := &ui.ChatMessageUI{
		Role: ui.ChatRoleUser,
		Text: "Run echo hello",
	}
	err = basePresenter.SendUserMessage(userMsg)
	require.NoError(t, err)

	// Wait for completion
	done := make(chan struct{})
	go func() {
		for {
			if !thread.IsRunning() {
				close(done)
				return
			}
		}
	}()
	<-done

	// Verify that runBash tool was registered
	toolNames := tools.List()
	assert.Contains(t, toolNames, "runBash", "runBash tool should be registered")

	// Verify that the mock runner was called (tool was executed)
	executions := mockRunner.GetExecutions()
	assert.GreaterOrEqual(t, len(executions), 1, "runBash tool should have been executed")

	// Verify the command was executed
	if len(executions) > 0 {
		lastExec := mockRunner.GetLastExecution()
		require.NotNil(t, lastExec)
		assert.Equal(t, "echo hello", lastExec.Command)
		assert.Equal(t, 0, lastExec.ExitCode)
		assert.Equal(t, "test output", lastExec.Output)
	}

	// Verify that captured requests contain runBash tool in the tools list
	requests := mockServer.GetRequests()
	require.GreaterOrEqual(t, len(requests), 1, "should have captured at least one request")

	// Find the chat request and verify it contains runBash tool
	var foundToolList bool
	for _, req := range requests {
		if req.Path == "/api/chat" && req.Method == "POST" {
			bodyStr := string(req.Body)
			// Verify runBash is in the tools list sent to LLM
			assert.Contains(t, bodyStr, "runBash", "LLM request should contain runBash tool")
			foundToolList = true
			break
		}
	}
	assert.True(t, foundToolList, "should have found a chat request with runBash tool")
}

// TestCLIPermissionQueryHandling tests that CLI mode handles permission queries correctly.
// When not in interactive mode and without --allow-all-permissions, permissions should be denied by default.
func TestCLIPermissionQueryHandling(t *testing.T) {
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

	// Create a VFS with access control that requires asking for permissions
	localVFS, err := vfs.NewLocalVFS(t.TempDir(), nil)
	require.NoError(t, err)

	// Wrap with access control that asks for all permissions
	accessConfig := map[string]conf.FileAccess{
		"*": {
			Read:   conf.AccessAsk,
			Write:  conf.AccessAsk,
			Delete: conf.AccessAsk,
			List:   conf.AccessAsk,
			Find:   conf.AccessAsk,
			Move:   conf.AccessAsk,
		},
	}
	restrictedVFS := vfs.NewAccessControlVFS(localVFS, accessConfig)

	tools := tool.NewToolRegistry()
	// Register VFS tools with the restricted VFS
	tool.RegisterVFSTools(tools, restrictedVFS)

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	system := &core.SweSystem{
		ModelProviders:  map[string]models.ModelProvider{"ollama": client},
		ModelTags:       models.NewModelTagRegistry(),
		PromptGenerator: newMockPromptGenerator("You are a helpful assistant."),
		Tools:           tools,
		VFS:             restrictedVFS,
		WorkDir:         tmpDir,
		LogBaseDir:      logsDir,
	}

	// Set up mock streaming response with tool call that requires permission
	// First response: assistant makes a tool call to vfsRead (closeAfter=false to continue processing)
	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"vfsRead","arguments":{"path":"test.txt"}}}]},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Second response: after permission denial, assistant should handle the error
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"I apologize, but I cannot read that file without permission."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Create thread
	thread := core.NewSessionThread(system, nil)

	// Start session
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	// Create presenter and view - non-interactive mode, no allow-all-permissions
	basePresenter := presenter.NewChatPresenter(system, thread)

	// Track permission queries and responses
	var permissionQueryReceived bool
	var permissionResponse string
	mockView := &permissionTrackingMockView{
		onQueryPermission: func(query *ui.PermissionQueryUI) error {
			permissionQueryReceived = true
			// In non-interactive mode without --allow-all-permissions, the view should
			// automatically deny permissions. We track what response would be sent.
			if len(query.Options) > 0 {
				// Find the "Deny" option or use the last option
				response := query.Options[len(query.Options)-1]
				for _, opt := range query.Options {
					if opt == "Deny" {
						response = opt
						break
					}
				}
				permissionResponse = response
			}
			return nil
		},
	}

	err = basePresenter.SetView(mockView)
	require.NoError(t, err)

	// Set output handler
	thread.SetOutputHandler(basePresenter)

	// Send user message that will trigger a tool call requiring permission
	userMsg := &ui.ChatMessageUI{
		Role: ui.ChatRoleUser,
		Text: "Read the file test.txt",
	}
	err = basePresenter.SendUserMessage(userMsg)
	require.NoError(t, err)

	// Wait for completion with timeout
	done := make(chan struct{})
	go func() {
		for {
			if !thread.IsRunning() {
				close(done)
				return
			}
		}
	}()

	select {
	case <-done:
		// Success - session completed
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - session did not complete, likely due to hanging on permission query")
	}

	// Verify that permission query was received and denied
	assert.True(t, permissionQueryReceived, "Permission query should have been received")
	assert.Equal(t, "Deny", permissionResponse, "Permission should have been denied in non-interactive mode")
}

// permissionTrackingMockView is a mock view that tracks permission queries
type permissionTrackingMockView struct {
	onQueryPermission func(*ui.PermissionQueryUI) error
}

func (m *permissionTrackingMockView) Init(session *ui.ChatSessionUI) error      { return nil }
func (m *permissionTrackingMockView) AddMessage(msg *ui.ChatMessageUI) error    { return nil }
func (m *permissionTrackingMockView) UpdateMessage(msg *ui.ChatMessageUI) error { return nil }
func (m *permissionTrackingMockView) UpdateTool(tool *ui.ToolUI) error          { return nil }
func (m *permissionTrackingMockView) MoveToBottom() error                       { return nil }
func (m *permissionTrackingMockView) QueryPermission(query *ui.PermissionQueryUI) error {
	if m.onQueryPermission != nil {
		return m.onQueryPermission(query)
	}
	return nil
}

// TestCLIPermissionQueryWithResponse tests that CLI mode properly handles permission queries
// when the view calls PermissionResponse (simulating real CLI behavior).
// This test reproduces the bug where the session hangs after permission denial.
func TestCLIPermissionQueryWithResponse(t *testing.T) {
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

	// Create a VFS with access control that requires asking for permissions
	localVFS, err := vfs.NewLocalVFS(t.TempDir(), nil)
	require.NoError(t, err)

	// Wrap with access control that asks for all permissions
	accessConfig := map[string]conf.FileAccess{
		"*": {
			Read:   conf.AccessAsk,
			Write:  conf.AccessAsk,
			Delete: conf.AccessAsk,
			List:   conf.AccessAsk,
			Find:   conf.AccessAsk,
			Move:   conf.AccessAsk,
		},
	}
	restrictedVFS := vfs.NewAccessControlVFS(localVFS, accessConfig)

	tools := tool.NewToolRegistry()
	// Register VFS tools with the restricted VFS
	tool.RegisterVFSTools(tools, restrictedVFS)

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	system := &core.SweSystem{
		ModelProviders:  map[string]models.ModelProvider{"ollama": client},
		ModelTags:       models.NewModelTagRegistry(),
		PromptGenerator: newMockPromptGenerator("You are a helpful assistant."),
		Tools:           tools,
		VFS:             restrictedVFS,
		WorkDir:         tmpDir,
		LogBaseDir:      logsDir,
	}

	// Set up mock streaming response with tool call that requires permission
	// First response: assistant makes a tool call to vfsRead (closeAfter=false to continue processing)
	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"vfsRead","arguments":{"path":"test.txt"}}}]},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Second response: after permission denial, assistant should handle the error
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"I apologize, but I cannot read that file without permission."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Create thread
	thread := core.NewSessionThread(system, nil)

	// Start session
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	// Create presenter and view - non-interactive mode, no allow-all-permissions
	basePresenter := presenter.NewChatPresenter(system, thread)

	// Use a mock view that actually calls PermissionResponse (like real CLI view does)
	var permissionQueryReceived bool
	var permissionResponse string
	mockView := &autoDenyPermissionMockView{
		presenter: basePresenter,
		onQueryPermission: func(query *ui.PermissionQueryUI) {
			permissionQueryReceived = true
			// Find the "Deny" option or use the last option
			if len(query.Options) > 0 {
				response := query.Options[len(query.Options)-1]
				for _, opt := range query.Options {
					if opt == "Deny" {
						response = opt
						break
					}
				}
				permissionResponse = response
			}
		},
	}

	err = basePresenter.SetView(mockView)
	require.NoError(t, err)

	// Set output handler
	thread.SetOutputHandler(basePresenter)

	// Send user message that will trigger a tool call requiring permission
	userMsg := &ui.ChatMessageUI{
		Role: ui.ChatRoleUser,
		Text: "Read the file test.txt",
	}
	err = basePresenter.SendUserMessage(userMsg)
	require.NoError(t, err)

	// Wait for completion with timeout
	done := make(chan struct{})
	go func() {
		for {
			if !thread.IsRunning() {
				close(done)
				return
			}
		}
	}()

	select {
	case <-done:
		// Success - session completed
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - session did not complete, likely due to hanging on permission query")
	}

	// Verify that permission query was received and denied
	assert.True(t, permissionQueryReceived, "Permission query should have been received")
	assert.Equal(t, "Deny", permissionResponse, "Permission should have been denied in non-interactive mode")
}

// autoDenyPermissionMockView is a mock view that automatically denies permissions
// by calling PermissionResponse, simulating the real CLI view behavior
type autoDenyPermissionMockView struct {
	presenter         ui.IChatPresenter
	onQueryPermission func(*ui.PermissionQueryUI)
}

func (m *autoDenyPermissionMockView) Init(session *ui.ChatSessionUI) error      { return nil }
func (m *autoDenyPermissionMockView) AddMessage(msg *ui.ChatMessageUI) error    { return nil }
func (m *autoDenyPermissionMockView) UpdateMessage(msg *ui.ChatMessageUI) error { return nil }
func (m *autoDenyPermissionMockView) UpdateTool(tool *ui.ToolUI) error          { return nil }
func (m *autoDenyPermissionMockView) MoveToBottom() error                       { return nil }
func (m *autoDenyPermissionMockView) QueryPermission(query *ui.PermissionQueryUI) error {
	if m.onQueryPermission != nil {
		m.onQueryPermission(query)
	}
	// Automatically deny the permission (like CLI view does in non-interactive mode)
	if m.presenter != nil && len(query.Options) > 0 {
		// Find the "Deny" option or use the last option
		response := query.Options[len(query.Options)-1]
		for _, opt := range query.Options {
			if opt == "Deny" {
				response = opt
				break
			}
		}
		return m.presenter.PermissionResponse(response)
	}
	return nil
}

// TestMockChatViewAutoPermissionResponse tests the MockChatView's automatic permission response functionality.
func TestMockChatViewAutoPermissionResponse(t *testing.T) {
	tests := []struct {
		name             string
		autoResponse     string
		options          []string
		expectedResponse string
	}{
		{
			name:             "Auto deny selects Deny option",
			autoResponse:     "Deny",
			options:          []string{"Allow", "Ask", "Deny"},
			expectedResponse: "Deny",
		},
		{
			name:             "Auto deny falls back to last option",
			autoResponse:     "Deny",
			options:          []string{"Allow", "Ask", "Reject"},
			expectedResponse: "Reject",
		},
		{
			name:             "Auto accept selects first option",
			autoResponse:     "Accept",
			options:          []string{"Allow", "Ask", "Deny"},
			expectedResponse: "Allow",
		},
		{
			name:             "Custom response",
			autoResponse:     "CustomAnswer",
			options:          []string{"Option1", "Option2"},
			expectedResponse: "CustomAnswer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock presenter to track responses
			mockPresenter := mock.NewMockChatPresenter()

			// Create mock view with automatic permission response
			mockView := mock.NewMockChatView()
			mockView.AutoPermissionResponse = tc.autoResponse
			mockView.Presenter = mockPresenter

			// Create permission query
			query := &ui.PermissionQueryUI{
				Id:      "test-query-1",
				Title:   "Test Permission",
				Details: "Test details",
				Options: tc.options,
			}

			// Call QueryPermission - should automatically respond
			err := mockView.QueryPermission(query)
			require.NoError(t, err)

			// Verify the query was recorded
			assert.Equal(t, 1, len(mockView.QueryPermissionCalls), "Query should have been recorded")

			// Verify the response was sent
			assert.Equal(t, 1, len(mockPresenter.PermissionResponseCalls), "Permission response should have been sent")
			assert.Equal(t, tc.expectedResponse, mockPresenter.PermissionResponseCalls[0], "Response should match expected")
		})
	}
}

// TestMockChatViewNoAutoResponse tests that MockChatView without auto-response just records the query.
func TestMockChatViewNoAutoResponse(t *testing.T) {
	// Create mock presenter
	mockPresenter := mock.NewMockChatPresenter()

	// Create mock view WITHOUT automatic permission response
	mockView := mock.NewMockChatView()
	// AutoPermissionResponse is empty by default
	mockView.Presenter = mockPresenter

	// Create permission query
	query := &ui.PermissionQueryUI{
		Id:      "test-query-1",
		Title:   "Test Permission",
		Details: "Test details",
		Options: []string{"Allow", "Deny"},
	}

	// Call QueryPermission - should just record without responding
	err := mockView.QueryPermission(query)
	require.NoError(t, err)

	// Verify the query was recorded
	assert.Equal(t, 1, len(mockView.QueryPermissionCalls), "Query should have been recorded")

	// Verify NO response was sent (backward compatibility)
	assert.Equal(t, 0, len(mockPresenter.PermissionResponseCalls), "No permission response should have been sent without auto-response")
}
