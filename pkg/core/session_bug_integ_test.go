// session_bug_integ_test.go contains regression and bug-specific integration tests
// that verify fixes for previously identified issues.
package core

import (
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermissionLoopBug tests that when a tool permission is automatically allowed,
// the session does not fall into an infinite loop of asking for permission.
// This is a regression test for a bug where granting permission would cause
// the same tool call to be executed repeatedly.
func TestPermissionLoopBug(t *testing.T) {
	// Create a local VFS
	localVFS, err := vfs.NewLocalVFS(t.TempDir(), nil)
	require.NoError(t, err)

	// Wrap with access control that asks for all permissions
	accessConfig := map[string]conf.FileAccess{
		"*": {
			Read:  conf.AccessAsk,
			Write: conf.AccessAsk,
		},
	}
	restrictedVFS := vfs.NewAccessControlVFS(localVFS, accessConfig)

	// Create a role with tool access set to "ask"
	configStore := impl.NewMockConfigStore()
	testRole := &conf.AgentRoleConfig{
		Name:        "test",
		Description: "Test role with ask permissions",
		ToolsAccess: map[string]conf.AccessFlag{
			"**": conf.AccessAsk, // Ask for all tools
		},
	}
	configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
		"test": testRole,
	})
	roleRegistry := NewAgentRoleRegistry(configStore)
	fixture := newSweSystemFixture(t, "You are a helpful assistant.", withVFS(restrictedVFS), withRoles(roleRegistry), withConfigStore(configStore), withWorkDir(t.TempDir()))
	system := fixture.system
	mockServer := fixture.server

	t.Run("permission allow should not cause infinite loop", func(t *testing.T) {
		// Set up mock streaming response with tool call
		// First response: assistant makes a tool call to vfsRead
		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"vfsRead","arguments":{"path":"test.txt"}}}]},"done":false}`,
			`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Second response: after permission is granted and tool executes, assistant responds
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"I have read the file."},"done":false}`,
			`{"model":"test-model","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create a mock handler that automatically allows permissions
		mockHandler := &autoAllowPermissionHandler{
			MockSessionOutputHandler: testutil.NewMockSessionOutputHandler(),
		}

		// Create thread
		thread := NewSessionThread(system, mockHandler)
		mockHandler.SetThread(thread)

		// Start session
		err := thread.StartSession("ollama/test-model")
		require.NoError(t, err)

		session := thread.GetSession()
		require.NotNil(t, session)

		// Set the role to enable access control
		err = session.SetRole("test")
		require.NoError(t, err)

		// Send user prompt through the thread (this starts processing)
		err = thread.UserPrompt("Read the file test.txt")
		require.NoError(t, err)

		// Wait for the thread to finish processing with timeout
		done := make(chan struct{})
		go func() {
			for {
				if !thread.IsRunning() {
					close(done)
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()

		select {
		case <-done:
			// Success - thread completed
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out - session fell into infinite permission loop")
		}

		// Verify that permission was queried exactly twice (once for tool access, once for VFS access)
		// The bug was that it would loop infinitely, so we expect exactly 2, not more
		assert.Equal(t, 2, mockHandler.PermissionQueryCount, "Permission should be queried exactly twice (tool access + VFS access), not in an infinite loop")

		// Verify that the tool was actually executed (tool result should be present)
		assert.GreaterOrEqual(t, len(mockHandler.ToolCallResults), 1, "Tool should have been executed and produced a result")
	})
}

// autoAllowPermissionHandler is a mock handler that automatically allows permissions
type autoAllowPermissionHandler struct {
	*testutil.MockSessionOutputHandler
	PermissionQueryCount int
	thread               *SessionThread
}

func (h *autoAllowPermissionHandler) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	h.PermissionQueryCount++
	h.MockSessionOutputHandler.OnPermissionQuery(query)

	// Automatically allow the permission
	if h.thread != nil {
		h.thread.PermissionResponse("Allow")
	}
}

func (h *autoAllowPermissionHandler) SetThread(thread *SessionThread) {
	h.thread = thread
}
