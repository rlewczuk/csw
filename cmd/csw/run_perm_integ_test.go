package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	sessionio "github.com/rlewczuk/csw/pkg/io"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type autoPermissionOutputHandler struct {
	delegate core.SessionThreadOutput
	thread   *core.SessionThread
	response string

	mu sync.Mutex
}

func (h *autoPermissionOutputHandler) AddAssistantMessage(text string, thinking string) {
	if h.delegate != nil {
		h.delegate.AddAssistantMessage(text, thinking)
	}
}

func (h *autoPermissionOutputHandler) ShowMessage(message string, messageType string) {
	if h.delegate != nil {
		h.delegate.ShowMessage(message, messageType)
	}
}

func (h *autoPermissionOutputHandler) AddUserMessage(text string) {
	if h.delegate != nil {
		h.delegate.AddUserMessage(text)
	}
}

func (h *autoPermissionOutputHandler) AddToolCall(call *tool.ToolCall) {
	if h.delegate != nil {
		h.delegate.AddToolCall(call)
	}
}

func (h *autoPermissionOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	if h.delegate != nil {
		h.delegate.AddToolCallResult(result)
	}
}

func (h *autoPermissionOutputHandler) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	if h.delegate != nil {
		h.delegate.OnPermissionQuery(query)
	}

	h.mu.Lock()
	thread := h.thread
	response := h.response
	h.mu.Unlock()

	if thread != nil {
		_ = thread.PermissionResponse("", response)
	}
}

func (h *autoPermissionOutputHandler) OnRateLimitError(retryAfterSeconds int) {
	if h.delegate != nil {
		h.delegate.OnRateLimitError(retryAfterSeconds)
	}
}

func (h *autoPermissionOutputHandler) ShouldRetryAfterFailure(message string) bool {
	if h.delegate != nil {
		return h.delegate.ShouldRetryAfterFailure(message)
	}

	return false
}

func (h *autoPermissionOutputHandler) RunFinished(err error) {
	if h.delegate != nil {
		h.delegate.RunFinished(err)
	}
}

// cli_perm_integ_test.go contains integration tests for permission handling.
// These tests verify that CLI mode handles permission queries correctly,
// including auto-deny behavior in non-interactive mode and proper handling
// of allow-all-permissions mode.

// TestCLIPermissionQueryHandling tests that CLI mode handles permission queries correctly.
// When not in interactive mode and without --allow-all-permissions, permissions should be denied by default.
func TestCLIPermissionQueryHandling(t *testing.T) {
	// Create a VFS with access control that requires asking for permissions
	localVFS, err := vfs.NewLocalVFS(t.TempDir(), nil, nil)
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

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)
	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithVFS(restrictedVFS),
		coretestfixture.WithWorkDir(tmpDir),
		coretestfixture.WithLogBaseDir(logsDir),
	)
	system := fixture.System
	mockServer := fixture.Server

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

	handler := &autoPermissionOutputHandler{
		delegate: sessionio.NewTextSessionOutput(nil),
		response: "deny",
	}
	thread.SetOutputHandler(handler)
	handler.thread = thread

	err = thread.UserPrompt("Read the file test.txt")
	require.NoError(t, err)

	waitForThreadToFinish(t, thread)

	session := thread.GetSession()
	require.NotNil(t, session)
	messages := session.ChatMessages()
	require.NotEmpty(t, messages)
	last := messages[len(messages)-1]
	assert.Contains(t, last.GetText(), "cannot read that file without permission")
}

// TestCLIAllowAllPermissionsExecutesAllToolCalls verifies that allow-all-permissions
// mode continues executing subsequent tool calls after an auto-granted permission.
func TestCLIAllowAllPermissionsExecutesAllToolCalls(t *testing.T) {
	mockVFS := vfs.NewMockVFS()
	err := mockVFS.WriteFile("test.txt", []byte("hello"))
	require.NoError(t, err)

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
	restrictedVFS := vfs.NewAccessControlVFS(mockVFS, accessConfig)

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, restrictedVFS, nil, nil)

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)
	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithVFS(restrictedVFS),
		coretestfixture.WithTools(tools),
		coretestfixture.WithoutVFSTools(),
		coretestfixture.WithWorkDir(tmpDir),
		coretestfixture.WithLogBaseDir(logsDir),
	)
	system := fixture.System
	mockServer := fixture.Server

	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"id":"call-1","function":{"name":"vfsRead","arguments":{"path":"test.txt"}}},{"id":"call-2","function":{"name":"vfsFind","arguments":{"query":""}}}]},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"All done."},"done":false}`,
		`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	thread := core.NewSessionThread(system, nil)
	err = thread.StartSession("ollama/test-model")
	require.NoError(t, err)

	handler := &autoPermissionOutputHandler{
		delegate: sessionio.NewTextSessionOutput(nil),
		response: "allow",
	}
	thread.SetOutputHandler(handler)
	handler.thread = thread

	err = thread.UserPrompt("Read and list files")
	require.NoError(t, err)

	waitForThreadToFinish(t, thread)

	session := thread.GetSession()
	require.NotNil(t, session)

	responses := 0
	for _, msg := range session.ChatMessages() {
		responses += len(msg.GetToolResponses())
	}
	assert.Equal(t, 2, responses, "Should execute and complete both tool calls")
}

// TestCLIPermissionQueryWithResponse tests that CLI mode properly handles permission queries
// when the view calls PermissionResponse (simulating real CLI behavior).
// This test reproduces the bug where the session hangs after permission denial.
func TestCLIPermissionQueryWithResponse(t *testing.T) {
	// Create a VFS with access control that requires asking for permissions
	localVFS, err := vfs.NewLocalVFS(t.TempDir(), nil, nil)
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
	tool.RegisterVFSTools(tools, restrictedVFS, nil, nil)

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)
	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithVFS(restrictedVFS),
		coretestfixture.WithTools(tools),
		coretestfixture.WithoutVFSTools(),
		coretestfixture.WithWorkDir(tmpDir),
		coretestfixture.WithLogBaseDir(logsDir),
	)
	system := fixture.System
	mockServer := fixture.Server

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

	handler := &autoPermissionOutputHandler{
		delegate: sessionio.NewTextSessionOutput(nil),
		response: "deny",
	}
	thread.SetOutputHandler(handler)
	handler.thread = thread

	err = thread.UserPrompt("Read the file test.txt")
	require.NoError(t, err)

	waitForThreadToFinish(t, thread)

	session := thread.GetSession()
	require.NotNil(t, session)
	messages := session.ChatMessages()
	require.NotEmpty(t, messages)
	last := messages[len(messages)-1]
	assert.Contains(t, last.GetText(), "cannot read that file without permission")
}
