package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/ui"
)

// CliChatView is a simple text-based chat view that prints to stdout and reads from stdin.
type CliChatView struct {
	presenter ui.IChatPresenter
	output    io.Writer
	input     io.Reader
	slug      string

	// interactive controls whether to read user input
	interactive bool

	// acceptAllPermissions controls whether to automatically accept all permissions
	acceptAllPermissions bool

	// verbose controls whether to display full tool output instead of one-liners
	verbose bool

	// outputFormat controls tool rendering output format: short, full, jsonl
	outputFormat string

	// messages stores all messages for rendering
	messages []*ui.ChatMessageUI

	// mu protects messages slice
	mu sync.Mutex

	// scanner for reading input
	scanner *bufio.Scanner

	// stopCh signals the input reading goroutine to stop
	stopCh chan struct{}

	// renderedTools tracks the last rendered output for each tool
	// Key is tool ID, value is the output line that was last printed
	renderedTools map[string]string
}

// NewCliChatView creates a new CLI chat view.
// interactive controls whether to read user input.
// acceptAllPermissions controls whether to automatically accept all permissions.
// verbose controls whether to display full tool output instead of one-liners.
func NewCliChatView(presenter ui.IChatPresenter, output io.Writer, input io.Reader, options ...any) *CliChatView {
	slug := defaultCLISlug
	interactive := false
	acceptAllPermissions := false
	verbose := false
	outputFormat := "short"

	if len(options) == 3 {
		interactive, _ = options[0].(bool)
		acceptAllPermissions, _ = options[1].(bool)
		switch formatValue := options[2].(type) {
		case string:
			outputFormat = strings.TrimSpace(formatValue)
		case bool:
			verbose = formatValue
			if verbose {
				outputFormat = "full"
			}
		}
	}

	if len(options) >= 4 {
		slug, _ = options[0].(string)
		interactive, _ = options[1].(bool)
		acceptAllPermissions, _ = options[2].(bool)
		switch formatValue := options[3].(type) {
		case string:
			outputFormat = strings.TrimSpace(formatValue)
		case bool:
			verbose = formatValue
			if verbose {
				outputFormat = "full"
			}
		}
	}

	if outputFormat == "" {
		if verbose {
			outputFormat = "full"
		} else {
			outputFormat = "short"
		}
	}
	if outputFormat != "short" && outputFormat != "full" && outputFormat != "jsonl" {
		outputFormat = "short"
	}
	verbose = outputFormat == "full"

	// acceptAllPermissions implies interactive=false
	if acceptAllPermissions {
		interactive = false
	}

	view := &CliChatView{
		presenter:            presenter,
		output:               output,
		input:                input,
		slug:                 normalizeCLISlug(slug),
		interactive:          interactive,
		acceptAllPermissions: acceptAllPermissions,
		verbose:              verbose,
		outputFormat:         outputFormat,
		messages:             make([]*ui.ChatMessageUI, 0),
		stopCh:               make(chan struct{}),
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
			prev := m
			v.messages[i] = msg
			v.renderUpdatedMessage(prev, msg)
			return nil
		}
	}

	// If not found by ID, try to find by role (for backwards compatibility)
	if msg.Id == "" {
		for i := len(v.messages) - 1; i >= 0; i-- {
			m := v.messages[i]
			if m.Role == msg.Role {
				prev := m
				if msg.Id == "" && m.Id != "" {
					msg.Id = m.Id
				}
				v.messages[i] = msg
				v.renderUpdatedMessage(prev, msg)
				return nil
			}
		}
	}

	return nil
}

// renderUpdatedMessage renders only changed parts of an updated message.
// Must be called with mu locked.
func (v *CliChatView) renderUpdatedMessage(prev *ui.ChatMessageUI, msg *ui.ChatMessageUI) {
	if msg == nil {
		return
	}

	if prev == nil {
		v.renderMessage(msg)
		return
	}

	if msg.Role != ui.ChatRoleAssistant || prev.Role != ui.ChatRoleAssistant {
		v.renderMessage(msg)
		return
	}

	if msg.Thinking != "" && msg.Thinking != prev.Thinking {
		v.writef("\n*%s*\n", msg.Thinking)
	}

	if msg.Text != "" && msg.Text != prev.Text {
		v.writef("\nAssistant: %s\n", msg.Text)
	}

	for _, tool := range msg.Tools {
		v.renderTool(tool)
	}
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
	v.writef("\n=== Permission Required ===\n")
	v.writef("%s\n", query.Title)
	if query.Details != "" {
		v.writef("%s\n", query.Details)
	}

	// Print options
	for i, option := range query.Options {
		v.writef("  %d. %s\n", i+1, option)
	}

	if query.AllowCustomResponse != "" {
		v.writef("  0. %s\n", query.AllowCustomResponse)
	}

	v.writef("Enter your choice: ")

	// Read user input
	if v.scanner == nil || !v.scanner.Scan() {
		return nil
	}

	input := strings.TrimSpace(v.scanner.Text())

	// Parse the input
	var response string
	if input == "0" && query.AllowCustomResponse != "" {
		// Custom response
		v.writef("%s: ", query.AllowCustomResponse)
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
		v.writef("\nType your message and press Enter (Ctrl+D to exit):\n")

		for {
			select {
			case <-v.stopCh:
				return
			default:
				v.writef("> ")
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
						v.writef("Error sending message: %v\n", err)
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
			v.writef("\nYou: %s\n", msg.Text)
		}
	case ui.ChatRoleAssistant:
		v.renderAssistantMessage(msg)
	}
}

// renderAssistantMessage renders an assistant message.
// Must be called with mu locked.
func (v *CliChatView) renderAssistantMessage(msg *ui.ChatMessageUI) {
	if msg.Thinking != "" {
		v.writef("\n*%s*\n", msg.Thinking)
	}

	if msg.Text != "" {
		v.writef("\nAssistant: %s\n", msg.Text)
	}

	for _, tool := range msg.Tools {
		v.renderTool(tool)
	}
}

// renderTool renders a tool call status.
// Must be called with mu locked.
// Only displays tool calls in final status (succeeded or failed).
// Returns true if the tool was rendered (i.e., it was in final status and had display content).
func (v *CliChatView) renderTool(tool *ui.ToolUI) bool {
	// Only render in final status (succeeded or failed)
	if tool.Status != ui.ToolStatusSucceeded && tool.Status != ui.ToolStatusFailed {
		return false
	}

	// In selected mode, use JSONL/details/summary.
	displayStr := tool.Summary
	if v.outputFormat == "jsonl" {
		displayStr = tool.JSONL
	} else if v.verbose {
		displayStr = tool.Details
	}
	if displayStr == "" {
		displayStr = tool.Summary
	}
	if displayStr == "" {
		displayStr = tool.Details
	}
	if displayStr == "" {
		return false
	}

	// Choose icon based on status
	var icon string
	switch tool.Status {
	case ui.ToolStatusSucceeded:
		icon = "✅"
	case ui.ToolStatusFailed:
		icon = "❌"
	default:
		icon = "⌛"
	}

	outputLine := fmt.Sprintf("%s %s\n", icon, displayStr)
	if v.outputFormat == "jsonl" {
		outputLine = displayStr + "\n"
	}

	// Check if this tool has already been rendered with the same output
	if lastOutput, ok := v.renderedTools[tool.Id]; ok && lastOutput == outputLine {
		return false
	}

	// Render the tool call result
	v.write(outputLine)

	// Mark this tool as rendered with current output
	v.renderedTools[tool.Id] = outputLine

	return true
}

func (v *CliChatView) writef(format string, args ...any) {
	v.write(fmt.Sprintf(format, args...))
}

func (v *CliChatView) write(message string) {
	if v.outputFormat == "jsonl" {
		_, _ = fmt.Fprint(v.output, message)
		return
	}
	_, _ = fmt.Fprint(v.output, addCLISlugPrefix(v.slug, message))
}

var _ ui.IChatView = (*CliChatView)(nil)
