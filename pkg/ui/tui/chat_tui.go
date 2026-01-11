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
	"github.com/codesnort/codesnort-swe/pkg/ui"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
)

type tuiChatMessage struct {
	id        string
	role      string
	content   string
	toolCalls []*tuiToolCallWidget
}

type tuiToolCallWidget struct {
	tool *ui.ToolUI
}

func newTuiToolCallWidget(tool *ui.ToolUI) *tuiToolCallWidget {
	return &tuiToolCallWidget{
		tool: tool,
	}
}

func (w *tuiToolCallWidget) View() string {
	var statusIcon string
	var statusStyle lipgloss.Style

	switch w.tool.Status {
	case ui.ToolStatusStarted:
		statusIcon = "○"
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	case ui.ToolStatusExecuting:
		statusIcon = "◉"
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	case ui.ToolStatusSucceeded:
		statusIcon = "✓"
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	case ui.ToolStatusFailed:
		statusIcon = "✗"
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	}

	var content strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	content.WriteString(statusStyle.Render(statusIcon))
	content.WriteString(" ")
	content.WriteString(titleStyle.Render(w.tool.Name))
	content.WriteString("\n")

	if w.tool.Message != "" {
		messageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
		content.WriteString(messageStyle.Render(w.tool.Message))
		content.WriteString("\n")
	}

	if len(w.tool.Props) > 0 {
		propStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
		for _, prop := range w.tool.Props {
			if len(prop) == 2 {
				content.WriteString(propStyle.Render(fmt.Sprintf("  %s: %s", prop[0], prop[1])))
				content.WriteString("\n")
			}
		}
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	return cardStyle.Render(content.String())
}

type TuiChatView struct {
	model  *tuiChatViewModel
	parent ui.CompositeWidget
}

// Notify implements ui.CompositeWidget.
func (v *TuiChatView) Notify(msg ui.CompositeNotification) {
	// Pass through to parent if needed, or handle self-refresh
	// For now, TuiChatView is a leaf, but it could have children tool widgets
}

// SetParent implements ui.CompositeWidget.
func (v *TuiChatView) SetParent(parent ui.CompositeWidget) {
	v.parent = parent
}

type tuiChatViewModel struct {
	presenter ui.IChatPresenter
	viewport  viewport.Model
	textarea  textarea.Model
	messages  []*tuiChatMessage
	width     int
	height    int
	err       error

	renderer *glamour.TermRenderer

	mu sync.Mutex
}

func NewTuiChatView(presenter ui.IChatPresenter) (*TuiChatView, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(defaultWidth-4),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown renderer: %w", err)
	}

	vp := viewport.New(defaultWidth, defaultHeight-5)
	vp.SetContent("Welcome! Type a message and press Ctrl+Enter to send.\n")

	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 0
	ta.SetWidth(defaultWidth)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	ta.KeyMap.InsertNewline.SetEnabled(true)

	model := &tuiChatViewModel{
		presenter: presenter,
		viewport:  vp,
		textarea:  ta,
		messages:  make([]*tuiChatMessage, 0),
		width:     defaultWidth,
		height:    defaultHeight,
		renderer:  renderer,
	}

	return &TuiChatView{model: model}, nil
}

func (v *TuiChatView) Model() tea.Model {
	return v.model
}

func (m *tuiChatViewModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *tuiChatViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - m.textarea.Height() - 3

		m.textarea.SetWidth(msg.Width)

		m.updateViewportContent()
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "alt+enter":
			input := m.textarea.Value()
			if input != "" {
				m.textarea.Reset()

				message := &ui.ChatMessageUI{
					Id:   "",
					Role: ui.ChatRoleUser,
					Text: input,
				}
				if err := m.presenter.SendUserMessage(message); err != nil {
					m.err = err
				}
			}
			return m, nil
		}
	}

	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	cmds = append(cmds, taCmd)

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	m.updateViewportContent()

	return m, tea.Batch(cmds...)
}

func (m *tuiChatViewModel) View() string {
	var view strings.Builder

	view.WriteString(m.viewport.View())
	view.WriteString("\n\n")
	view.WriteString(m.textarea.View())

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		view.WriteString("\n")
		view.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return view.String()
}

func (v *TuiChatView) Init(session *ui.ChatSessionUI) error {
	v.model.mu.Lock()
	defer v.model.mu.Unlock()

	v.model.messages = make([]*tuiChatMessage, 0)
	for _, msg := range session.Messages {
		v.model.addMessageUnsafe(msg)
	}

	v.model.updateViewportContentUnsafe()
	v.model.viewport.GotoBottom()

	if v.parent != nil {
		v.parent.Notify(ui.CompositeNotificationRefresh)
	}
	return nil
}

func (v *TuiChatView) AddMessage(msg *ui.ChatMessageUI) error {
	v.model.mu.Lock()
	defer v.model.mu.Unlock()

	v.model.addMessageUnsafe(msg)
	v.model.updateViewportContentUnsafe()
	v.model.viewport.GotoBottom()

	if v.parent != nil {
		v.parent.Notify(ui.CompositeNotificationRefresh)
	}
	return nil
}

func (v *TuiChatView) UpdateMessage(msg *ui.ChatMessageUI) error {
	v.model.mu.Lock()
	defer v.model.mu.Unlock()

	// Find message by ID or by role if ID is empty (backwards compatibility)
	for _, m := range v.model.messages {
		if (msg.Id != "" && m.id == msg.Id) || (msg.Id == "" && m.role == string(msg.Role) && len(m.content) == 0) {
			// Update content (replace, not append - presenter sends accumulated text)
			m.content = msg.Text

			// Update tool calls
			m.toolCalls = make([]*tuiToolCallWidget, 0, len(msg.Tools))
			for _, tool := range msg.Tools {
				m.toolCalls = append(m.toolCalls, newTuiToolCallWidget(tool))
			}
			break
		}
	}

	v.model.updateViewportContentUnsafe()
	v.model.viewport.GotoBottom()

	if v.parent != nil {
		v.parent.Notify(ui.CompositeNotificationRefresh)
	}
	return nil
}

func (v *TuiChatView) UpdateTool(tool *ui.ToolUI) error {
	v.model.mu.Lock()
	defer v.model.mu.Unlock()

	for _, msg := range v.model.messages {
		for _, tc := range msg.toolCalls {
			if tc.tool.Id == tool.Id {
				tc.tool = tool
			}
		}
	}

	v.model.updateViewportContentUnsafe()

	if v.parent != nil {
		v.parent.Notify(ui.CompositeNotificationRefresh)
	}
	return nil
}

func (v *TuiChatView) MoveToBottom() error {
	v.model.viewport.GotoBottom()
	return nil
}

func (m *tuiChatViewModel) addUserMessage(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := &tuiChatMessage{
		id:        "", // User messages from UI don't have IDs yet
		role:      string(ui.ChatRoleUser),
		content:   content,
		toolCalls: make([]*tuiToolCallWidget, 0),
	}
	m.messages = append(m.messages, msg)
}

func (m *tuiChatViewModel) addMessageUnsafe(msg *ui.ChatMessageUI) {
	chatMsg := &tuiChatMessage{
		id:        msg.Id,
		role:      string(msg.Role),
		content:   msg.Text,
		toolCalls: make([]*tuiToolCallWidget, 0),
	}

	for _, tool := range msg.Tools {
		chatMsg.toolCalls = append(chatMsg.toolCalls, newTuiToolCallWidget(tool))
	}

	m.messages = append(m.messages, chatMsg)
}

func (m *tuiChatViewModel) updateViewportContent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateViewportContentUnsafe()
}

func (m *tuiChatViewModel) updateViewportContentUnsafe() {
	if len(m.messages) == 0 {
		m.viewport.SetContent("Welcome! Type a message and press Alt+Enter to send.\n")
		return
	}

	var content strings.Builder

	userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	assistantStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)

	for _, msg := range m.messages {
		switch msg.role {
		case string(ui.ChatRoleUser):
			content.WriteString(userStyle.Render("You: "))
			content.WriteString(msg.content)
			content.WriteString("\n\n")

		case string(ui.ChatRoleAssistant):
			content.WriteString(assistantStyle.Render("Assistant: "))
			if msg.content != "" {
				rendered, err := m.renderer.Render(msg.content)
				if err != nil {
					content.WriteString(msg.content)
				} else {
					content.WriteString(rendered)
				}
			}
			content.WriteString("\n")

			for _, toolCall := range msg.toolCalls {
				content.WriteString(toolCall.View())
				content.WriteString("\n")
			}

			content.WriteString("\n")
		}
	}

	m.viewport.SetContent(content.String())
}

var _ ui.IChatView = (*TuiChatView)(nil)
var _ tea.Model = (*tuiChatViewModel)(nil)
