package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/presenter"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cli_perm_integ_test.go contains integration tests for permission handling.
// These tests verify that CLI mode handles permission queries correctly,
// including auto-deny behavior in non-interactive mode and proper handling
// of allow-all-permissions mode.

type autoAllowPermissionTrackingView struct {
	presenter            ui.IChatPresenter
	permissionQueryCount int
	toolResultCount      int
	mu                   sync.Mutex
}

func (v *autoAllowPermissionTrackingView) Init(session *ui.ChatSessionUI) error {
	return nil
}

func (v *autoAllowPermissionTrackingView) AddMessage(msg *ui.ChatMessageUI) error {
	return nil
}

func (v *autoAllowPermissionTrackingView) UpdateMessage(msg *ui.ChatMessageUI) error {
	return nil
}

func (v *autoAllowPermissionTrackingView) UpdateTool(tool *ui.ToolUI) error {
	if tool.Status == ui.ToolStatusSucceeded || tool.Status == ui.ToolStatusFailed {
		v.mu.Lock()
		v.toolResultCount++
		v.mu.Unlock()
	}
	return nil
}

func (v *autoAllowPermissionTrackingView) MoveToBottom() error {
	return nil
}

func (v *autoAllowPermissionTrackingView) QueryPermission(query *ui.PermissionQueryUI) error {
	v.mu.Lock()
	v.permissionQueryCount++
	v.mu.Unlock()
	if v.presenter != nil && len(query.Options) > 0 {
		return v.presenter.PermissionResponse(query.Options[0])
	}
	return nil
}

func (v *autoAllowPermissionTrackingView) ShowMessage(message string, messageType shared.MessageType) {
	_ = message
	_ = messageType
}

func (v *autoAllowPermissionTrackingView) PermissionQueries() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.permissionQueryCount
}

func (v *autoAllowPermissionTrackingView) ToolResults() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.toolResultCount
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

func (m *permissionTrackingMockView) ShowMessage(message string, messageType shared.MessageType) {
	_ = message
	_ = messageType
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

func (m *autoDenyPermissionMockView) ShowMessage(message string, messageType shared.MessageType) {
	_ = message
	_ = messageType
}

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

	basePresenter := presenter.NewChatPresenter(system, thread)
	view := &autoAllowPermissionTrackingView{presenter: basePresenter}

	err = basePresenter.SetView(view)
	require.NoError(t, err)

	thread.SetOutputHandler(basePresenter)

	userMsg := &ui.ChatMessageUI{
		Role: ui.ChatRoleUser,
		Text: "Read and list files",
	}
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

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - session did not complete")
	}

	assert.Equal(t, 2, view.PermissionQueries(), "Should query permission for both tool calls")
	assert.Equal(t, 2, view.ToolResults(), "Should execute and complete both tool calls")
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
