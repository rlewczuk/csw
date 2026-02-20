// Package logmd provides a logging IChatPresenter implementation that writes session to markdown.
package logmd

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/ui"
)

// LogmdChatPresenter wraps an IChatPresenter and logs all method calls to a markdown file.
type LogmdChatPresenter struct {
	wrapped ui.IChatPresenter
	writer  io.Writer
	mu      *sync.Mutex
}

// NewLogmdChatPresenter creates a new LogmdChatPresenter that wraps the given IChatPresenter and writes to the provided io.Writer.
// The mutex is used to protect writes to the io.Writer.
func NewLogmdChatPresenter(wrapped ui.IChatPresenter, writer io.Writer, mu *sync.Mutex) ui.IChatPresenter {
	return &LogmdChatPresenter{
		wrapped: wrapped,
		writer:  writer,
		mu:      mu,
	}
}

// SetView sets the view to render the chat conversation.
// It delegates to the wrapped presenter and logs the call to markdown.
func (l *LogmdChatPresenter) SetView(view ui.IChatView) error {
	l.logMethodCall("SetView", fmt.Sprintf("view=%T", view))
	return l.wrapped.SetView(view)
}

// SendUserMessage sends a user message to the chat session and starts processing.
// It delegates to the wrapped presenter and logs the call to markdown.
func (l *LogmdChatPresenter) SendUserMessage(message *ui.ChatMessageUI) error {
	l.logMethodCall("SendUserMessage", fmt.Sprintf("id=%s, role=%s, text=%q", message.Id, message.Role, message.Text))
	return l.wrapped.SendUserMessage(message)
}

// SaveUserMessage saves a user message to the chat session but doesn't start processing.
// It delegates to the wrapped presenter and logs the call to markdown.
func (l *LogmdChatPresenter) SaveUserMessage(message *ui.ChatMessageUI) error {
	l.logMethodCall("SaveUserMessage", fmt.Sprintf("id=%s, role=%s, text=%q", message.Id, message.Role, message.Text))
	return l.wrapped.SaveUserMessage(message)
}

// Pause pauses the chat session (i.e. stops processing).
// It delegates to the wrapped presenter and logs the call to markdown.
func (l *LogmdChatPresenter) Pause() error {
	l.logMethodCall("Pause", "")
	return l.wrapped.Pause()
}

// Resume resumes the chat session (i.e. starts processing).
// It delegates to the wrapped presenter and logs the call to markdown.
func (l *LogmdChatPresenter) Resume() error {
	l.logMethodCall("Resume", "")
	return l.wrapped.Resume()
}

// PermissionResponse sends user response to permission query.
// It delegates to the wrapped presenter and logs the call to markdown.
func (l *LogmdChatPresenter) PermissionResponse(response string) error {
	l.logMethodCall("PermissionResponse", fmt.Sprintf("response=%q", response))
	return l.wrapped.PermissionResponse(response)
}

// SetModel sets the model used for the chat session.
// It delegates to the wrapped presenter and logs the call to markdown.
func (l *LogmdChatPresenter) SetModel(model string) error {
	l.logMethodCall("SetModel", fmt.Sprintf("model=%q", model))
	return l.wrapped.SetModel(model)
}

// logMethodCall writes a method call entry to the markdown file.
func (l *LogmdChatPresenter) logMethodCall(methodName, params string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	role := strings.Title("system")
	fmt.Fprintf(l.writer, "## %s\n\n", role)
	fmt.Fprintf(l.writer, "**Method:** `%s`\n\n", methodName)
	if params != "" {
		fmt.Fprintf(l.writer, "**Parameters:** %s\n\n", params)
	}
	fmt.Fprintf(l.writer, "---\n\n")
}
