package main

import (
	"bytes"
	"os"
	"path/filepath"
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
// These tests verify that LogmdChatPresenter logs method calls to markdown files.

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

	// Wrap presenter with logging wrapper using buffer
	mu := &sync.Mutex{}
	logPresenter := logmd.NewLogmdChatPresenter(basePresenter, &buf, mu)

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

	contentStr := buf.String()

	// Verify the buffer contains presenter logs
	assert.Contains(t, contentStr, "Hello, this is a test prompt", "buffer should contain the user prompt")
	assert.Contains(t, contentStr, "## System", "buffer should contain system message header")
	assert.Contains(t, contentStr, "SendUserMessage", "buffer should contain SendUserMessage method call")
}
