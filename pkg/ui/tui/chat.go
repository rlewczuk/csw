package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/tool"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
)

// ChatMessage represents a single message in the chat.
type ChatMessage struct {
	// Role is the role of the message sender (user, assistant, system).
	Role string
	// Content is the text content of the message.
	Content string
	// ToolCalls are the tool calls associated with this message.
	ToolCalls []*ToolCallWidget
}

// ChatWidget is a TUI widget for chat conversation with SweSession.
type ChatWidget struct {
	controller *core.SessionController
	viewport   viewport.Model
	textarea   textarea.Model
	messages   []*ChatMessage
	width      int
	height     int
	err        error

	// Markdown renderer
	renderer *glamour.TermRenderer

	// Current message being built from chunks
	currentMessage *ChatMessage

	// Map of tool call ID to widget for quick lookup
	toolCallWidgets map[string]*ToolCallWidget

	// Mutex for thread-safe updates from SessionOutputHandler
	mu sync.Mutex
}

// contentUpdateMsg is a message sent when content is updated from SessionOutputHandler
type contentUpdateMsg struct{}

// NewChatWidget creates a new chat widget with the given session controller.
func NewChatWidget(controller *core.SessionController) (*ChatWidget, error) {
	// Create markdown renderer
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(defaultWidth-4),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown renderer: %w", err)
	}

	// Create viewport
	vp := viewport.New(defaultWidth, defaultHeight-5)
	vp.SetContent("Welcome! Type a message and press Ctrl+Enter to send.\n")

	// Create textarea
	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 0 // No limit
	ta.SetWidth(defaultWidth)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	// Enable Ctrl+Enter to submit, Enter to add newline
	ta.KeyMap.InsertNewline.SetEnabled(true)

	widget := &ChatWidget{
		controller:      controller,
		viewport:        vp,
		textarea:        ta,
		messages:        make([]*ChatMessage, 0),
		width:           defaultWidth,
		height:          defaultHeight,
		renderer:        renderer,
		toolCallWidgets: make(map[string]*ToolCallWidget),
	}

	return widget, nil
}

// Init initializes the chat widget.
func (w *ChatWidget) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles updates to the chat widget.
func (w *ChatWidget) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case contentUpdateMsg:
		// Content was updated from SessionOutputHandler, re-render viewport
		w.updateViewportContent()
		w.viewport.GotoBottom()
		return w, nil

	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height

		// Update viewport size
		w.viewport.Width = msg.Width
		w.viewport.Height = msg.Height - w.textarea.Height() - 3

		// Update textarea width
		w.textarea.SetWidth(msg.Width)

		// Re-render content with new width
		w.updateViewportContent()
		w.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return w, tea.Quit

		case "alt+enter":
			// Send message
			input := w.textarea.Value()
			if input != "" {
				// Add user message
				w.addUserMessage(input)
				w.textarea.Reset()
				w.updateViewportContent()
				w.viewport.GotoBottom()

				// Send to controller
				if err := w.controller.UserPrompt(input); err != nil {
					w.err = err
				}
			}
			return w, nil
		}
	}

	// Update textarea
	var taCmd tea.Cmd
	w.textarea, taCmd = w.textarea.Update(msg)
	cmds = append(cmds, taCmd)

	// Update viewport
	var vpCmd tea.Cmd
	w.viewport, vpCmd = w.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	// Update tool call widgets
	w.mu.Lock()
	for _, message := range w.messages {
		for _, toolCallWidget := range message.ToolCalls {
			var tcCmd tea.Cmd
			_, tcCmd = toolCallWidget.Update(msg)
			cmds = append(cmds, tcCmd)
		}
	}
	w.mu.Unlock()

	// Re-render if we have active tool calls
	w.updateViewportContent()

	return w, tea.Batch(cmds...)
}

// View renders the chat widget.
func (w *ChatWidget) View() string {
	var view strings.Builder

	// Viewport
	view.WriteString(w.viewport.View())
	view.WriteString("\n\n")

	// Textarea
	view.WriteString(w.textarea.View())

	// Error message (if any)
	if w.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		view.WriteString("\n")
		view.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", w.err)))
	}

	return view.String()
}

// addUserMessage adds a user message to the chat.
func (w *ChatWidget) addUserMessage(content string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	msg := &ChatMessage{
		Role:      "user",
		Content:   content,
		ToolCalls: make([]*ToolCallWidget, 0),
	}
	w.messages = append(w.messages, msg)
}

// updateViewportContent re-renders all messages and updates the viewport content.
func (w *ChatWidget) updateViewportContent() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// If no messages, show welcome message
	if len(w.messages) == 0 {
		w.viewport.SetContent("Welcome! Type a message and press Alt+Enter to send.\n")
		return
	}

	var content strings.Builder

	userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	assistantStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)

	for _, msg := range w.messages {
		switch msg.Role {
		case "user":
			content.WriteString(userStyle.Render("You: "))
			content.WriteString(msg.Content)
			content.WriteString("\n\n")

		case "assistant":
			content.WriteString(assistantStyle.Render("Assistant: "))
			if msg.Content != "" {
				// Render markdown
				rendered, err := w.renderer.Render(msg.Content)
				if err != nil {
					content.WriteString(msg.Content)
				} else {
					content.WriteString(rendered)
				}
			}
			content.WriteString("\n")

			// Render tool calls
			for _, toolCall := range msg.ToolCalls {
				content.WriteString(toolCall.View())
				content.WriteString("\n")
			}

			content.WriteString("\n")

		case "system":
			systemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
			content.WriteString(systemStyle.Render("System: "))
			content.WriteString(systemStyle.Render(msg.Content))
			content.WriteString("\n\n")
		}
	}

	w.viewport.SetContent(content.String())
}

// SessionOutputHandler implementation

// AddMarkdownChunk is called when new text chunk is generated by LLM.
func (w *ChatWidget) AddMarkdownChunk(markdown string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// If no current message, create a new assistant message
	if w.currentMessage == nil {
		w.currentMessage = &ChatMessage{
			Role:      "assistant",
			Content:   "",
			ToolCalls: make([]*ToolCallWidget, 0),
		}
		w.messages = append(w.messages, w.currentMessage)
	}

	// Append chunk to current message
	w.currentMessage.Content += markdown
}

// AddToolCallStart is called when tool call is started but not fully parsed yet.
func (w *ChatWidget) AddToolCallStart(call *tool.ToolCall) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// If no current message, create a new assistant message
	if w.currentMessage == nil {
		w.currentMessage = &ChatMessage{
			Role:      "assistant",
			Content:   "",
			ToolCalls: make([]*ToolCallWidget, 0),
		}
		w.messages = append(w.messages, w.currentMessage)
	}

	// Create a new tool call widget
	widget := NewToolCallWidget(call)
	w.currentMessage.ToolCalls = append(w.currentMessage.ToolCalls, widget)
	w.toolCallWidgets[call.ID] = widget
}

// AddToolCallDetails is called when tool call is fully or partially parsed.
func (w *ChatWidget) AddToolCallDetails(call *tool.ToolCall) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Find the widget and update it
	if widget, ok := w.toolCallWidgets[call.ID]; ok {
		widget.UpdateDetails(call)
	}
}

// AddToolCallResult is called when tool call is completed.
func (w *ChatWidget) AddToolCallResult(result *tool.ToolResponse) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Find the widget and set the result
	if widget, ok := w.toolCallWidgets[result.Call.ID]; ok {
		widget.SetResult(result)
	}
}

// RunFinished is called when the session Run() loop is finished.
func (w *ChatWidget) RunFinished(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Finalize current message
	w.currentMessage = nil

	// Set error if any
	if err != nil {
		w.err = err
	}
}
