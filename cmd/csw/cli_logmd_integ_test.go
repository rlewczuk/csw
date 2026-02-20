package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/presenter"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/ui/logmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cli_logmd_integ_test.go contains integration tests for markdown logging functionality.
// These tests verify that LogmdChatView and LogmdChatPresenter properly log
// session activity and method calls to markdown files.

// TestLogmdChatViewLogsSession tests that LogmdChatView properly logs session activity to markdown.
func TestLogmdChatViewLogsSession(t *testing.T) {
	var err error
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)
	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithWorkDir(tmpDir),
		coretestfixture.WithLogBaseDir(logsDir),
	)
	system := fixture.System
	mockServer := fixture.Server

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
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err := os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)
	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithWorkDir(tmpDir),
		coretestfixture.WithLogBaseDir(logsDir),
	)
	system := fixture.System
	mockServer := fixture.Server

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
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err := os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)
	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithWorkDir(tmpDir),
		coretestfixture.WithLogBaseDir(logsDir),
	)
	system := fixture.System
	mockServer := fixture.Server

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
	var err error
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)
	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithWorkDir(tmpDir),
		coretestfixture.WithLogBaseDir(logsDir),
	)
	system := fixture.System
	mockServer := fixture.Server

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
