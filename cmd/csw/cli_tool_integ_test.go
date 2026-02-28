package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/presenter"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cli_tool_integ_test.go contains integration tests for tool functionality.
// These tests verify that tools like runBash and VFS tools are properly
// registered, presented to the LLM, and log their operations correctly.

// TestRunBashToolIntegration tests that runBash tool is properly registered and presented to LLM.
func TestRunBashToolIntegration(t *testing.T) {
	var err error
	vfsInstance := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()

	// Register runBash tool with mock runner
	mockRunner := runner.NewMockRunner()
	mockRunner.SetDefaultResponse("test output", 0, nil)
	tool.RegisterRunBashTool(tools, mockRunner, map[string]conf.AccessFlag{
		"echo .*": conf.AccessAllow,
	}, 120*time.Second)

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)
	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithVFS(vfsInstance),
		coretestfixture.WithTools(tools),
		coretestfixture.WithWorkDir(tmpDir),
		coretestfixture.WithLogBaseDir(logsDir),
	)
	system := fixture.System
	mockServer := fixture.Server

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

// TestWebFetchToolIntegration tests that webFetch tool is properly registered and presented to LLM.
func TestWebFetchToolIntegration(t *testing.T) {
	vfsInstance := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()

	tool.RegisterWebFetchTool(tools, nil)

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err := os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)
	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithVFS(vfsInstance),
		coretestfixture.WithTools(tools),
		coretestfixture.WithWorkDir(tmpDir),
		coretestfixture.WithLogBaseDir(logsDir),
	)
	system := fixture.System
	mockServer := fixture.Server

	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"webFetch is available."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	thread := core.NewSessionThread(system, nil)
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	basePresenter := presenter.NewChatPresenter(system, thread)
	baseView := newMockChatView()
	err = basePresenter.SetView(baseView)
	require.NoError(t, err)

	thread.SetOutputHandler(basePresenter)

	userMsg := &ui.ChatMessageUI{Role: ui.ChatRoleUser, Text: "Can you fetch a page?"}
	err = basePresenter.SendUserMessage(userMsg)
	require.NoError(t, err)

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

	toolNames := tools.List()
	assert.Contains(t, toolNames, "webFetch", "webFetch tool should be registered")

	requests := mockServer.GetRequests()
	require.GreaterOrEqual(t, len(requests), 1, "should have captured at least one request")

	var foundToolList bool
	for _, req := range requests {
		if req.Path == "/api/chat" && req.Method == "POST" {
			bodyStr := string(req.Body)
			assert.Contains(t, bodyStr, "webFetch", "LLM request should contain webFetch tool")
			foundToolList = true
			break
		}
	}
	assert.True(t, foundToolList, "should have found a chat request with webFetch tool")
}

// TestCLIVFSToolLogging verifies that VFS write/edit tools log to the session logger.
func TestCLIVFSToolLogging(t *testing.T) {
	vfsInstance := vfs.NewMockVFS()

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err := os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithVFS(vfsInstance),
		coretestfixture.WithWorkDir(tmpDir),
		coretestfixture.WithLogBaseDir(logsDir),
	)
	system := fixture.System
	mockServer := fixture.Server

	// First response: assistant makes a tool call to vfsWrite
	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"vfsWrite","arguments":{"path":"notes.txt","content":"hello"}}}]},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Second response: assistant makes a tool call to vfsEdit
	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"vfsEdit","arguments":{"path":"notes.txt","oldString":"hello","newString":"hello world","replaceAll":true}}}]},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	// Third response: assistant confirms completion
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:04Z","message":{"role":"assistant","content":"Updated the file."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:05Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
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
		Text: "Create a file and update it",
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
		// Session completed
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - session did not complete")
	}

	session := thread.GetSession()
	require.NotNil(t, session)
	logBuffer := logging.GetTestSessionBuffer(session.ID())
	require.NotNil(t, logBuffer)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "vfsWrite_start", "vfsWrite_start should be logged")
	assert.Contains(t, logOutput, "vfsEdit_start", "vfsEdit_start should be logged")
}
