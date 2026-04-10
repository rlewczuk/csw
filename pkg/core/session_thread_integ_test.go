// session_thread_integ_test.go contains integration tests for session thread management
// including initialization, tool selection, and thread safety.
package core

import (
	"sync"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type interruptMessageRecord struct {
	message     string
	messageType string
}

type interruptOutputDouble struct {
	mu               sync.Mutex
	messages         []interruptMessageRecord
	runFinishedCalls int
	runFinishedErr   error
}

func (d *interruptOutputDouble) ShowMessage(message string, messageType string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.messages = append(d.messages, interruptMessageRecord{message: message, messageType: messageType})
}

func (d *interruptOutputDouble) AddUserMessage(text string) {
	_ = text
}

func (d *interruptOutputDouble) AddAssistantMessage(text string, thinking string) {
	_ = text
	_ = thinking
}

func (d *interruptOutputDouble) AddToolCall(call *tool.ToolCall) {
	_ = call
}

func (d *interruptOutputDouble) AddToolCallResult(result *tool.ToolResponse) {
	_ = result
}

func (d *interruptOutputDouble) RunFinished(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.runFinishedCalls++
	d.runFinishedErr = err
}

func (d *interruptOutputDouble) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	_ = query
}

func (d *interruptOutputDouble) OnRateLimitError(retryAfterSeconds int) {
	_ = retryAfterSeconds
}

func (d *interruptOutputDouble) ShouldRetryAfterFailure(message string) bool {
	_ = message
	return false
}

func (d *interruptOutputDouble) snapshot() ([]interruptMessageRecord, int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	messagesCopy := append([]interruptMessageRecord(nil), d.messages...)
	return messagesCopy, d.runFinishedCalls, d.runFinishedErr
}

var _ SessionThreadOutput = (*interruptOutputDouble)(nil)

func TestSessionThread(t *testing.T) {
	fixture := newSweSystemFixture(t, "You are skilled software developer.")
	system := fixture.system

	t.Run("basic initialization and session management", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		// Initially no session
		assert.Nil(t, controller.GetSession())

		// Start a session
		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Now we have a session
		assert.NotNil(t, controller.GetSession())
	})
}

// TestSessionToolSelection verifies that the session presents session-level tools
// (which include access control wrappers and session-specific tools) to the LLM,
// not the system-level tools. This is a regression test for a bug where
// s.system.Tools.ListInfo() was used instead of s.Tools.ListInfo().
func TestSessionToolSelection(t *testing.T) {
	fixture := newSweSystemFixture(t, "You are skilled software developer.")
	system := fixture.system

	t.Run("bug: Run uses system tools instead of session tools", func(t *testing.T) {
		// This test exposes a bug where SweSession.Run() uses s.system.Tools.ListInfo()
		// instead of s.Tools.ListInfo() when passing tools to the model provider.
		//
		// The bug means:
		// 1. Session-specific tools (todoRead, todoWrite) are not presented to the LLM
		// 2. Access control wrappers applied to session tools are bypassed
		//
		// Expected behavior: Run() should use s.Tools.ListInfo()
		// Current (buggy) behavior: Run() uses s.system.Tools.ListInfo()

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Verify the session has session-specific tools
		sessionToolNames := session.Tools.List()
		assert.Contains(t, sessionToolNames, "todoRead", "session should have todoRead tool")
		assert.Contains(t, sessionToolNames, "todoWrite", "session should have todoWrite tool")

		// Verify the system does NOT have session-specific tools
		systemToolNames := system.Tools.List()
		assert.NotContains(t, systemToolNames, "todoRead", "system should not have todoRead")
		assert.NotContains(t, systemToolNames, "todoWrite", "system should not have todoWrite")

		// The counts should be different
		assert.NotEqual(t, len(systemToolNames), len(sessionToolNames),
			"session and system should have different number of tools")
	})

	t.Run("session without role uses system tools correctly", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()

		// Without SetRole, session tools should be a copy of system tools
		// plus session-specific tools like todoRead/todoWrite
		sessionToolNames := session.Tools.List()

		// Should have session-specific tools
		assert.Contains(t, sessionToolNames, "todoRead")
		assert.Contains(t, sessionToolNames, "todoWrite")

		// Should also have system tools like VFS tools
		assert.Contains(t, sessionToolNames, "vfsRead")
		assert.Contains(t, sessionToolNames, "vfsWrite")
	})

	t.Run("model tags apply tool selection for session tools", func(t *testing.T) {
		system.ModelTags = models.NewModelTagRegistry()
		err := system.ModelTags.SetGlobalMappings([]conf.ModelTagMapping{{Model: "devstral-.*", Tag: "limited"}})
		require.NoError(t, err)
		system.ToolSelection = conf.ToolSelectionConfig{
			Default: map[string]bool{"runBash": true, "vfsRead": true},
			Tags: map[string]map[string]bool{
				"limited": {"runBash": false},
			},
		}

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err = controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		session := controller.GetSession()
		require.NotNil(t, session)

		names := session.Tools.List()
		assert.Contains(t, names, "vfsRead")
		assert.NotContains(t, names, "runBash")
		assert.Contains(t, names, "todoRead")
	})
}

func TestSessionThreadSafety(t *testing.T) {
	fixture := newSweSystemFixture(t, "You are skilled software developer.")
	system := fixture.system
	mockServer := fixture.server

	t.Run("concurrent GetSession calls", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Call GetSession from multiple goroutines
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				session := controller.GetSession()
				assert.NotNil(t, session)
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("concurrent UserPrompt calls with single session", func(t *testing.T) {
		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		// Setup multiple responses
		for i := 0; i < 5; i++ {
			mockServer.AddStreamingResponse("/api/chat", "POST", false,
				`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Response"},"done":true,"done_reason":"stop"}`,
			)
		}

		// Send prompts from multiple goroutines
		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func(idx int) {
				err := controller.UserPrompt("Test prompt")
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all prompts to be queued
		for i := 0; i < 5; i++ {
			<-done
		}

		// Wait for the session to finish processing all prompts
		mockHandler.WaitForRunFinished()

		// Verify no error occurred
		assert.NoError(t, mockHandler.RunFinishedError)
	})
}

func TestSessionThreadInterruptFlow(t *testing.T) {
	t.Run("first interrupt suspends and clears queue, second interrupt terminates", func(t *testing.T) {
		output := &interruptOutputDouble{}
		cancelled := false
		controller := &SessionThread{
			outputHandler: output,
			sessionRunning: true,
			inputQueue: []string{"queued-1", "queued-2"},
			cancelFunc: func() {
				cancelled = true
			},
		}

		err := controller.Interrupt()
		require.NoError(t, err)
		assert.True(t, cancelled)

		controller.mu.Lock()
		assert.True(t, controller.paused)
		assert.True(t, controller.suspended)
		assert.Len(t, controller.inputQueue, 0)
		controller.mu.Unlock()

		messages, runFinishedCalls, runFinishedErr := output.snapshot()
		require.Len(t, messages, 2)
		assert.Equal(t, "Session suspended.", messages[0].message)
		assert.Equal(t, "info", messages[0].messageType)
		assert.Equal(t, "Removed 2 queued user prompt(s).", messages[1].message)
		assert.Equal(t, "warning", messages[1].messageType)
		assert.Equal(t, 0, runFinishedCalls)
		assert.NoError(t, runFinishedErr)

		err = controller.Interrupt()
		require.NoError(t, err)

		controller.mu.Lock()
		assert.False(t, controller.suspended)
		controller.mu.Unlock()

		messages, runFinishedCalls, runFinishedErr = output.snapshot()
		require.Len(t, messages, 3)
		assert.Equal(t, "Session terminated.", messages[2].message)
		assert.Equal(t, "info", messages[2].messageType)
		assert.Equal(t, 1, runFinishedCalls)
		require.Error(t, runFinishedErr)
		assert.Contains(t, runFinishedErr.Error(), "terminated by interrupt while suspended")
	})

	t.Run("user prompt resumes suspended session", func(t *testing.T) {
		fixture := newSweSystemFixture(t, "You are skilled software developer.")
		system := fixture.system
		mockServer := fixture.server

		mockServer.AddStreamingResponse("/api/chat", "POST", false,
			`{"model":"devstral-small-2:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Resumed response"},"done":true,"done_reason":"stop"}`,
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		controller := NewSessionThread(system, mockHandler)

		err := controller.StartSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)

		controller.mu.Lock()
		controller.paused = true
		controller.suspended = true
		controller.mu.Unlock()

		err = controller.UserPrompt("continue after suspend")
		require.NoError(t, err)

		mockHandler.WaitForRunFinished()
		assert.NoError(t, mockHandler.RunFinishedError)

		controller.mu.Lock()
		assert.False(t, controller.paused)
		assert.False(t, controller.suspended)
		controller.mu.Unlock()

		require.Eventually(t, func() bool {
			return len(mockHandler.AssistantMessages) > 0
		}, 2*time.Second, 20*time.Millisecond)
	})
}
