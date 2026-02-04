// Package logmd provides a logging IChatView implementation that writes session to markdown.
package logmd

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// LogmdChatView wraps an IChatView and logs all session activity to a markdown file.
type LogmdChatView struct {
	wrapped ui.IChatView
	writer  io.Writer
	mu      *sync.Mutex
}

// NewLogmdChatView creates a new LogmdChatView that wraps the given IChatView and writes to the provided io.Writer.
// The mutex is used to protect writes to the io.Writer.
func NewLogmdChatView(wrapped ui.IChatView, writer io.Writer, mu *sync.Mutex) ui.IChatView {
	return &LogmdChatView{
		wrapped: wrapped,
		writer:  writer,
		mu:      mu,
	}
}

// Init initializes the view with all messages from the session.
// It delegates to the wrapped view and logs the session to markdown.
func (l *LogmdChatView) Init(session *ui.ChatSessionUI) error {
	l.writeHeader(session)
	for _, msg := range session.Messages {
		l.writeMessage(msg)
	}
	return l.wrapped.Init(session)
}

// AddMessage adds a new message to the view.
// It delegates to the wrapped view and logs the message to markdown.
func (l *LogmdChatView) AddMessage(msg *ui.ChatMessageUI) error {
	l.writeMessage(msg)
	return l.wrapped.AddMessage(msg)
}

// UpdateMessage updates an existing message in the view.
// It delegates to the wrapped view. Updates are not logged separately.
func (l *LogmdChatView) UpdateMessage(msg *ui.ChatMessageUI) error {
	return l.wrapped.UpdateMessage(msg)
}

// UpdateTool updates an existing tool in the view.
// It delegates to the wrapped view. Tool updates are not logged separately.
func (l *LogmdChatView) UpdateTool(tool *ui.ToolUI) error {
	return l.wrapped.UpdateTool(tool)
}

// MoveToBottom scrolls the view to the bottom.
// It delegates to the wrapped view. This operation is not logged.
func (l *LogmdChatView) MoveToBottom() error {
	return l.wrapped.MoveToBottom()
}

// QueryPermission queries user for permission to use a tool.
// It delegates to the wrapped view. Permission queries are not logged.
func (l *LogmdChatView) QueryPermission(query *ui.PermissionQueryUI) error {
	return l.wrapped.QueryPermission(query)
}

// writeHeader writes the session header to the markdown file.
func (l *LogmdChatView) writeHeader(session *ui.ChatSessionUI) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprintf(l.writer, "# Chat Session\n\n")
	fmt.Fprintf(l.writer, "**Session ID:** %s\n\n", session.Id)
	fmt.Fprintf(l.writer, "**Model:** %s\n\n", session.Model)
	fmt.Fprintf(l.writer, "**Role:** %s\n\n", session.Role)
	fmt.Fprintf(l.writer, "**Working Directory:** %s\n\n", session.WorkDir)
	fmt.Fprintf(l.writer, "---\n\n")
}

// writeMessage writes a chat message to the markdown file.
func (l *LogmdChatView) writeMessage(msg *ui.ChatMessageUI) {
	l.mu.Lock()
	defer l.mu.Unlock()

	role := strings.Title(string(msg.Role))
	fmt.Fprintf(l.writer, "## %s\n\n", role)
	fmt.Fprintf(l.writer, "%s\n\n", msg.Text)

	if len(msg.Tools) > 0 {
		fmt.Fprintf(l.writer, "**Tools:**\n\n")
		for _, tool := range msg.Tools {
			fmt.Fprintf(l.writer, "- **%s** (%s): %s\n", tool.Name, tool.Status, tool.Message)
		}
		fmt.Fprintf(l.writer, "\n")
	}

	fmt.Fprintf(l.writer, "---\n\n")
}
