package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// CliChatView is a simple text-based chat view that prints to stdout and reads from stdin.
type CliChatView struct {
	presenter ui.IChatPresenter
	output    io.Writer
	input     io.Reader

	// interactive controls whether to read user input
	interactive bool

	// acceptAllPermissions controls whether to automatically accept all permissions
	acceptAllPermissions bool

	// messages stores all messages for rendering
	messages []*ui.ChatMessageUI

	// mu protects messages slice
	mu sync.Mutex

	// scanner for reading input
	scanner *bufio.Scanner

	// stopCh signals the input reading goroutine to stop
	stopCh chan struct{}

	// renderedText tracks the text that has been rendered for each message
	// Key is message ID, value is the text that was printed
	renderedText map[string]string

	// renderedTools tracks the last rendered output for each tool
	// Key is tool ID, value is the output line that was last printed
	renderedTools map[string]string
}

// NewCliChatView creates a new CLI chat view.
// interactive controls whether to read user input.
// acceptAllPermissions controls whether to automatically accept all permissions.
func NewCliChatView(presenter ui.IChatPresenter, output io.Writer, input io.Reader, interactive bool, acceptAllPermissions bool) *CliChatView {
	// acceptAllPermissions implies interactive=false
	if acceptAllPermissions {
		interactive = false
	}

	view := &CliChatView{
		presenter:            presenter,
		output:               output,
		input:                input,
		interactive:          interactive,
		acceptAllPermissions: acceptAllPermissions,
		messages:             make([]*ui.ChatMessageUI, 0),
		stopCh:               make(chan struct{}),
		renderedText:         make(map[string]string),
		renderedTools:        make(map[string]string),
	}

	// Setup scanner only for interactive mode
	if interactive && input != nil {
		view.scanner = bufio.NewScanner(input)
	}

	// Set this view as the presenter's view
	if presenter != nil {
		presenter.SetView(view)
	}

	return view
}

// Init initializes the view with all messages from the session.
func (v *CliChatView) Init(session *ui.ChatSessionUI) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.messages = make([]*ui.ChatMessageUI, len(session.Messages))
	copy(v.messages, session.Messages)

	// Render all messages
	for _, msg := range v.messages {
		v.renderMessage(msg)
	}

	return nil
}

// AddMessage adds a new message to the view.
func (v *CliChatView) AddMessage(msg *ui.ChatMessageUI) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.messages = append(v.messages, msg)
	v.renderMessage(msg)

	return nil
}

// UpdateMessage updates an existing message in the view.
func (v *CliChatView) UpdateMessage(msg *ui.ChatMessageUI) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Find and update the message
	for i, m := range v.messages {
		if m.Id == msg.Id {
			v.messages[i] = msg
			// Re-render the message
			v.renderMessage(msg)
			return nil
		}
	}

	// If not found by ID, try to find by role (for backwards compatibility)
	for i, m := range v.messages {
		if msg.Id == "" && m.Role == msg.Role && m.Text == "" {
			v.messages[i] = msg
			v.renderMessage(msg)
			return nil
		}
	}

	return nil
}

// UpdateTool updates an existing tool in the view.
func (v *CliChatView) UpdateTool(tool *ui.ToolUI) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Find the message containing this tool
	for _, msg := range v.messages {
		for _, t := range msg.Tools {
			if t.Id == tool.Id {
				// Update the tool
				*t = *tool
				// Render the tool status
				v.renderTool(tool)
				return nil
			}
		}
	}

	return nil
}

// MoveToBottom scrolls the view to the bottom (no-op for CLI).
func (v *CliChatView) MoveToBottom() error {
	// No-op for CLI view
	return nil
}

// QueryPermission queries user for permission to use a tool.
func (v *CliChatView) QueryPermission(query *ui.PermissionQueryUI) error {
	// If acceptAllPermissions is true, automatically accept
	if v.acceptAllPermissions {
		// Automatically select the first option
		if len(query.Options) > 0 {
			if v.presenter != nil {
				return v.presenter.PermissionResponse(query.Options[0])
			}
		}
		return nil
	}

	// If not interactive, deny all permissions by default
	if !v.interactive {
		if v.presenter != nil && len(query.Options) > 0 {
			// Find and select the "Deny" option, or use the last option as fallback
			denyOption := query.Options[len(query.Options)-1]
			for _, opt := range query.Options {
				if opt == "Deny" {
					denyOption = opt
					break
				}
			}
			return v.presenter.PermissionResponse(denyOption)
		}
		return nil
	}

	// Print the permission query
	fmt.Fprintf(v.output, "\n=== Permission Required ===\n")
	fmt.Fprintf(v.output, "%s\n", query.Title)
	if query.Details != "" {
		fmt.Fprintf(v.output, "%s\n", query.Details)
	}

	// Print options
	for i, option := range query.Options {
		fmt.Fprintf(v.output, "  %d. %s\n", i+1, option)
	}

	if query.AllowCustomResponse != "" {
		fmt.Fprintf(v.output, "  0. %s\n", query.AllowCustomResponse)
	}

	fmt.Fprintf(v.output, "Enter your choice: ")

	// Read user input
	if v.scanner == nil || !v.scanner.Scan() {
		return nil
	}

	input := strings.TrimSpace(v.scanner.Text())

	// Parse the input
	var response string
	if input == "0" && query.AllowCustomResponse != "" {
		// Custom response
		fmt.Fprintf(v.output, "%s: ", query.AllowCustomResponse)
		if v.scanner.Scan() {
			response = strings.TrimSpace(v.scanner.Text())
		}
	} else {
		// Try to parse as option number
		var optionIndex int
		if _, err := fmt.Sscanf(input, "%d", &optionIndex); err == nil {
			if optionIndex >= 1 && optionIndex <= len(query.Options) {
				response = query.Options[optionIndex-1]
			}
		} else {
			// Treat as direct response
			response = input
		}
	}

	// Send response to presenter
	if v.presenter != nil && response != "" {
		return v.presenter.PermissionResponse(response)
	}

	return nil
}

// StartReadingInput starts reading user input in a goroutine.
// This should be called after Init to start accepting user messages.
func (v *CliChatView) StartReadingInput() {
	if !v.interactive || v.scanner == nil {
		return
	}

	go func() {
		fmt.Fprintf(v.output, "\nType your message and press Enter (Ctrl+D to exit):\n")

		for {
			select {
			case <-v.stopCh:
				return
			default:
				fmt.Fprintf(v.output, "> ")
				if !v.scanner.Scan() {
					return
				}

				input := strings.TrimSpace(v.scanner.Text())
				if input == "" {
					continue
				}

				// Send message to presenter
				if v.presenter != nil {
					msg := &ui.ChatMessageUI{
						Role: ui.ChatRoleUser,
						Text: input,
					}
					if err := v.presenter.SendUserMessage(msg); err != nil {
						fmt.Fprintf(v.output, "Error sending message: %v\n", err)
					}
				}
			}
		}
	}()
}

// Stop stops the input reading goroutine.
func (v *CliChatView) Stop() {
	close(v.stopCh)
}

// renderMessage renders a single message to output.
// Must be called with mu locked.
func (v *CliChatView) renderMessage(msg *ui.ChatMessageUI) {
	switch msg.Role {
	case ui.ChatRoleUser:
		if msg.Text != "" {
			fmt.Fprintf(v.output, "\nYou: %s\n", msg.Text)
		}
	case ui.ChatRoleAssistant:
		v.renderAssistantMessage(msg)
	}
}

// renderAssistantMessage renders an assistant message, handling streaming updates.
// Must be called with mu locked.
func (v *CliChatView) renderAssistantMessage(msg *ui.ChatMessageUI) {
	// Get the previously rendered text for this message
	prevRendered := v.renderedText[msg.Id]

	// Check if this is a streaming update (new text starts with previously rendered text)
	// or a full update (text is completely different)
	isStreamingUpdate := false
	if len(prevRendered) > 0 && len(msg.Text) >= len(prevRendered) {
		// Check if the new text starts with what we already rendered
		isStreamingUpdate = msg.Text[:len(prevRendered)] == prevRendered
	}

	if isStreamingUpdate {
		// Streaming update: only print the delta
		if len(msg.Text) > len(prevRendered) {
			newContent := msg.Text[len(prevRendered):]
			fmt.Fprint(v.output, newContent)
			v.renderedText[msg.Id] = msg.Text
		}
	} else {
		// Full update or new message: print the full message
		if msg.Text != "" {
			fmt.Fprintf(v.output, "\nAssistant: %s", msg.Text)
			v.renderedText[msg.Id] = msg.Text
		}
	}

	// Render tool calls
	for _, tool := range msg.Tools {
		v.renderTool(tool)
	}

	// Ensure assistant message ends with a newline (only for non-streaming updates of existing messages)
	if !isStreamingUpdate && msg.Text != "" && !strings.HasSuffix(msg.Text, "\n") && prevRendered != "" {
		fmt.Fprint(v.output, "\n")
	}
}

// renderTool renders a tool call status.
// Must be called with mu locked.
// Only displays tool calls in final status (succeeded or failed).
func (v *CliChatView) renderTool(tool *ui.ToolUI) {
	// Only render in final status (succeeded or failed)
	if tool.Status != ui.ToolStatusSucceeded && tool.Status != ui.ToolStatusFailed {
		return
	}

	// Use the Summary field from ToolUI if available, otherwise fall back to Details
	displayStr := tool.Summary
	if displayStr == "" {
		displayStr = tool.Details
	}
	if displayStr == "" {
		return
	}

	outputLine := fmt.Sprintf("%s: %s\n", displayStr, tool.Status)

	// Check if this tool has already been rendered with the same output
	if lastOutput, ok := v.renderedTools[tool.Id]; ok && lastOutput == outputLine {
		return
	}

	// Render the tool call result
	fmt.Fprint(v.output, outputLine)

	// Mark this tool as rendered with current output
	v.renderedTools[tool.Id] = outputLine
}

var _ ui.IChatView = (*CliChatView)(nil)
