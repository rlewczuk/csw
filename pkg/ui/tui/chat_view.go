package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/mdv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"
	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// IChatView is the interface for TChatView
type IChatView interface {
	tui.IWidget
	ui.IChatView
}

// chatMessage represents a chat message in the view
type chatMessage struct {
	id        string
	role      string
	content   string
	toolCalls []*toolCallWidget
}

// toolCallWidget represents a tool call visualization
type toolCallWidget struct {
	tool *ui.ToolUI
}

func newToolCallWidget(tool *ui.ToolUI) *toolCallWidget {
	return &toolCallWidget{tool: tool}
}

// formatToolCall formats a tool call as markdown
func (w *toolCallWidget) toMarkdown() string {
	var sb strings.Builder

	// Status icon based on tool status
	var statusIcon string
	switch w.tool.Status {
	case ui.ToolStatusStarted:
		statusIcon = "○"
	case ui.ToolStatusExecuting:
		statusIcon = "◉"
	case ui.ToolStatusSucceeded:
		statusIcon = "✓"
	case ui.ToolStatusFailed:
		statusIcon = "✗"
	}

	sb.WriteString(fmt.Sprintf("\n**%s %s**\n", statusIcon, w.tool.Name))

	if w.tool.Message != "" {
		sb.WriteString(fmt.Sprintf("_%s_\n", w.tool.Message))
	}

	if len(w.tool.Props) > 0 {
		for _, prop := range w.tool.Props {
			if len(prop) == 2 {
				sb.WriteString(fmt.Sprintf("  - %s: %s\n", prop[0], prop[1]))
			}
		}
	}

	return sb.String()
}

// TChatView is a widget that renders chat conversation with input box
type TChatView struct {
	tui.TWidget

	// Layout container
	layout *tui.TFlexLayout

	// Markdown view for chat messages
	markdownView *mdv.TMarkdownView

	// Text area for user input
	textArea *tui.TTextArea

	// Menu widget for permission queries (replaces text area temporarily)
	permissionMenu *tui.TMenuWidget

	// Current permission query (nil if none active)
	currentPermissionQuery *ui.PermissionQueryUI

	// Presenter for handling user input
	presenter ui.IChatPresenter

	// Messages
	messages []*chatMessage
	mu       sync.Mutex

	// Dimensions
	width  int
	height int
}

// NewChatView creates a new chat view widget
func NewChatView(parent tui.IWidget, rect gtv.TRect, presenter ui.IChatPresenter) *TChatView {
	view := &TChatView{
		TWidget: tui.TWidget{
			Position: rect,
			Parent:   parent,
		},
		presenter: presenter,
		messages:  make([]*chatMessage, 0),
		width:     int(rect.W),
		height:    int(rect.H),
	}

	// Create flex layout (vertical) to hold markdown view and text area
	view.layout = tui.NewFlexLayout(
		view,
		gtv.TRect{X: 0, Y: 0, W: rect.W, H: rect.H},
		tui.FlexDirectionColumn,
	)

	// Calculate heights: chat gets all height except last 5 rows
	chatHeight := rect.H
	if chatHeight > 5 {
		chatHeight = chatHeight - 5
	}
	inputHeight := uint16(5)

	// Create markdown view for chat messages
	view.markdownView = mdv.NewMarkdownView(
		view.layout,
		gtv.TRect{X: 0, Y: 0, W: rect.W, H: chatHeight},
		"Welcome! Type a message and press Alt+Enter to send.\n",
	)

	// Create text area for user input
	view.textArea = tui.NewTextArea(
		view.layout,
		tui.WithRectangle(0, 0, int(rect.W), int(inputHeight)),
		tui.WithTextAreaText(""),
	)

	// Set up flex properties
	// Markdown view grows to fill available space
	view.layout.SetItemProperties(view.markdownView, tui.FlexItemProperties{
		FlexGrow:    1.0,
		FlexShrink:  1.0,
		FixedHeight: 0,
		MinSize:     1,
	})

	// Text area has fixed height of 5
	view.layout.SetItemProperties(view.textArea, tui.FlexItemProperties{
		FlexGrow:    0.0,
		FlexShrink:  0.0,
		FixedHeight: inputHeight,
	})

	// Set up Alt+Enter handler for text area
	view.textArea.SetKeyHandler(func(event *gtv.InputEvent) bool {
		// Check for Alt+Enter
		if (event.Key == '\r' || event.Key == '\n') && event.Modifiers&gtv.ModAlt != 0 {
			// Submit user input
			text := view.textArea.GetText()
			if text != "" {
				view.textArea.SetText("") // Clear input
				view.textArea.Focus()     // Keep focus

				// Send message to presenter
				message := &ui.ChatMessageUI{
					Id:   "",
					Role: ui.ChatRoleUser,
					Text: text,
				}
				if view.presenter != nil {
					if err := view.presenter.SendUserMessage(message); err != nil {
						// TODO: Show error to user
						_ = err
					}
				}
			}
			return true // Event handled
		}
		return false // Not handled
	})

	// Focus the text area by default
	view.textArea.Focus()
	view.layout.ActiveChild = view.textArea

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(view)
	}

	return view
}

// Init initializes the view with all messages from the session
func (v *TChatView) Init(session *ui.ChatSessionUI) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.messages = make([]*chatMessage, 0)
	for _, msg := range session.Messages {
		v.addMessageUnsafe(msg)
	}

	v.updateMarkdownContentUnsafe()
	v.scrollToBottom()

	return nil
}

// AddMessage adds a new message to the view
func (v *TChatView) AddMessage(msg *ui.ChatMessageUI) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.addMessageUnsafe(msg)
	v.updateMarkdownContentUnsafe()
	v.scrollToBottom()

	return nil
}

// UpdateMessage updates an existing message in the view
func (v *TChatView) UpdateMessage(msg *ui.ChatMessageUI) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Find message by ID or by role if ID is empty (backwards compatibility)
	for _, m := range v.messages {
		if (msg.Id != "" && m.id == msg.Id) || (msg.Id == "" && m.role == string(msg.Role) && len(m.content) == 0) {
			// Update content (replace, not append - presenter sends accumulated text)
			m.content = msg.Text

			// Update tool calls
			m.toolCalls = make([]*toolCallWidget, 0, len(msg.Tools))
			for _, tool := range msg.Tools {
				m.toolCalls = append(m.toolCalls, newToolCallWidget(tool))
			}
			break
		}
	}

	v.updateMarkdownContentUnsafe()
	v.scrollToBottom()

	return nil
}

// UpdateTool updates an existing tool in the view
func (v *TChatView) UpdateTool(tool *ui.ToolUI) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, msg := range v.messages {
		for _, tc := range msg.toolCalls {
			if tc.tool.Id == tool.Id {
				tc.tool = tool
			}
		}
	}

	v.updateMarkdownContentUnsafe()

	return nil
}

// MoveToBottom scrolls the view to the bottom
func (v *TChatView) MoveToBottom() error {
	v.scrollToBottom()
	return nil
}

// QueryPermission queries user for permission to use a tool
func (v *TChatView) QueryPermission(query *ui.PermissionQueryUI) error {
	// Try to get the application from the widget tree
	app := v.TWidget.GetApplication()

	// If app is available, use ExecuteOnUiThread to ensure proper redraw
	if app != nil {
		app.ExecuteOnUiThread(func() {
			v.queryPermissionUnsafe(query)
		})
		return nil
	}

	// Fallback: execute directly (for tests or if app not set)
	v.mu.Lock()
	defer v.mu.Unlock()
	v.queryPermissionUnsafe(query)
	return nil
}

// queryPermissionUnsafe queries user for permission (caller must hold lock or be on UI thread)
func (v *TChatView) queryPermissionUnsafe(query *ui.PermissionQueryUI) {
	// Store the current permission query
	v.currentPermissionQuery = query

	// Calculate menu height based on number of options
	menuHeight := uint16(len(query.Options) + 3) // +3 for border and title
	if query.AllowCustomResponse != "" {
		menuHeight++ // Extra line for custom input option
	}
	if menuHeight > 15 {
		menuHeight = 15 // Cap at reasonable height
	}

	// Create menu widget with nil parent (we'll set it manually)
	v.permissionMenu = tui.NewMenuWidget(
		nil,
		tui.WithRectangle(0, 0, int(v.layout.Position.W), int(menuHeight)),
	)

	// Set menu title and items
	v.permissionMenu.SetTitle(query.Title)

	// Clear existing items
	v.permissionMenu.SetItems(nil)

	// Add menu items for each option
	for _, option := range query.Options {
		optionText := option // Capture for closure
		v.permissionMenu.AddItem(optionText, func(text string) {
			v.handlePermissionResponse(optionText)
		})
	}

	// Enable custom input if allowed
	if query.AllowCustomResponse != "" {
		v.permissionMenu.EnableCustomInput(true, query.AllowCustomResponse)
		v.permissionMenu.SetOnCustomInput(func(text string) {
			v.handlePermissionResponse(text)
		})
	} else {
		v.permissionMenu.EnableCustomInput(false, "")
	}

	// Set cancel handler (ESC key)
	v.permissionMenu.SetOnCancel(func() {
		v.hidePermissionMenu()
	})

	// First, remove the text area from the layout
	v.layout.RemoveChild(v.textArea)

	// Set parent for the menu widget before adding it
	v.permissionMenu.Parent = v.layout

	// Then, add the menu to the layout
	v.layout.AddChild(v.permissionMenu)

	// Update menu size to match fixed height
	v.layout.SetItemProperties(v.permissionMenu, tui.FlexItemProperties{
		FlexGrow:    0.0,
		FlexShrink:  0.0,
		FixedHeight: menuHeight,
	})

	// Focus the menu
	v.permissionMenu.Focus()
	v.layout.ActiveChild = v.permissionMenu
}

// handlePermissionResponse handles the user's response to a permission query
func (v *TChatView) handlePermissionResponse(response string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Hide the permission menu
	v.hidePermissionMenuUnsafe()

	// Send response to presenter
	if v.presenter != nil && response != "" {
		// Run in goroutine to avoid blocking UI
		go func() {
			if err := v.presenter.PermissionResponse(response); err != nil {
				// TODO: Show error to user
				_ = err
			}
		}()
	}

	// Clear current permission query
	v.currentPermissionQuery = nil
}

// hidePermissionMenu hides the permission menu and restores the text area (with locking)
func (v *TChatView) hidePermissionMenu() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.hidePermissionMenuUnsafe()
}

// hidePermissionMenuUnsafe hides the permission menu and restores the text area (caller must lock)
func (v *TChatView) hidePermissionMenuUnsafe() {
	if v.permissionMenu == nil {
		return
	}

	// Remove menu from layout
	v.layout.RemoveChild(v.permissionMenu)

	// Ensure text area parent is set
	v.textArea.Parent = v.layout

	// Add text area back to layout
	v.layout.AddChild(v.textArea)

	// Restore text area properties
	inputHeight := uint16(5)
	v.layout.SetItemProperties(v.textArea, tui.FlexItemProperties{
		FlexGrow:    0.0,
		FlexShrink:  0.0,
		FixedHeight: inputHeight,
	})

	// Focus the text area
	v.textArea.Focus()
	v.layout.ActiveChild = v.textArea

	// Clear permission menu reference
	v.permissionMenu = nil

	// Clear current permission query
	v.currentPermissionQuery = nil
}

// addMessageUnsafe adds a message without locking (caller must lock)
func (v *TChatView) addMessageUnsafe(msg *ui.ChatMessageUI) {
	chatMsg := &chatMessage{
		id:        msg.Id,
		role:      string(msg.Role),
		content:   msg.Text,
		toolCalls: make([]*toolCallWidget, 0),
	}

	for _, tool := range msg.Tools {
		chatMsg.toolCalls = append(chatMsg.toolCalls, newToolCallWidget(tool))
	}

	v.messages = append(v.messages, chatMsg)
}

// updateMarkdownContentUnsafe updates the markdown view content (caller must lock)
func (v *TChatView) updateMarkdownContentUnsafe() {
	if len(v.messages) == 0 {
		v.markdownView.SetContent("Welcome! Type a message and press Alt+Enter to send.\n")
		return
	}

	var sb strings.Builder

	for _, msg := range v.messages {
		switch msg.role {
		case string(ui.ChatRoleUser):
			sb.WriteString("**You:** ")
			sb.WriteString(msg.content)
			sb.WriteString("\n\n")

		case string(ui.ChatRoleAssistant):
			sb.WriteString("**Assistant:** ")
			if msg.content != "" {
				sb.WriteString(msg.content)
			}
			sb.WriteString("\n")

			for _, toolCall := range msg.toolCalls {
				sb.WriteString(toolCall.toMarkdown())
			}

			sb.WriteString("\n")
		}
	}

	v.markdownView.SetContent(sb.String())
}

// scrollToBottom scrolls markdown view to bottom
func (v *TChatView) scrollToBottom() {
	// Set to a large value to ensure we're at the bottom
	// The markdown view will clamp this to the maximum valid offset
	v.markdownView.SetScrollOffset(999999)
}

// Draw draws the chat view on the screen
func (v *TChatView) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if v.Flags&tui.WidgetFlagHidden != 0 {
		return
	}

	// Draw the layout (which contains markdown view and text area)
	v.layout.Draw(screen)
}

// HandleEvent handles events for the chat view
func (v *TChatView) HandleEvent(event *tui.TEvent) {
	// Handle resize events
	if event.Type == tui.TEventTypeResize {
		v.Position = event.Rect
		v.width = int(event.Rect.W)
		v.height = int(event.Rect.H)

		// Resize layout to match
		layoutEvent := &tui.TEvent{
			Type: tui.TEventTypeResize,
			Rect: gtv.TRect{X: 0, Y: 0, W: event.Rect.W, H: event.Rect.H},
		}
		v.layout.HandleEvent(layoutEvent)
		return
	}

	// Delegate other events to layout
	v.layout.HandleEvent(event)
}

var _ ui.IChatView = (*TChatView)(nil)
var _ tui.IWidget = (*TChatView)(nil)
