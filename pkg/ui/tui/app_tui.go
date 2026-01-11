package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/codesnort/codesnort-swe/pkg/ui"
)

const (
	appName    = "Codesnort SWE"
	appVersion = "0.1.0"
)

// TuiAppView implements ui.IAppView for a terminal user interface.
type TuiAppView struct {
	presenter   ui.IAppPresenter
	chatView    *TuiChatView
	menu        *MenuWidget
	width       int
	height      int
	showingMenu bool
	refreshCh   chan struct{}
}

// NewTuiAppView creates a new TUI app view with the given presenter.
func NewTuiAppView(presenter ui.IAppPresenter) (*TuiAppView, error) {
	return &TuiAppView{
		presenter:   presenter,
		width:       defaultWidth,
		height:      defaultHeight,
		showingMenu: false,
		refreshCh:   make(chan struct{}, 1),
	}, nil
}

// refreshMsg is a message sent to trigger a refresh.
type refreshMsg struct{}

// Notify implements ui.CompositeWidget.
func (v *TuiAppView) Notify(msg ui.CompositeNotification) {
	if msg == ui.CompositeNotificationRefresh {
		select {
		case v.refreshCh <- struct{}{}:
		default:
			// Channel full, refresh already pending
		}
	}
}

// SetParent implements ui.CompositeWidget.
func (v *TuiAppView) SetParent(parent ui.CompositeWidget) {
	// AppView is the root widget, so this is a no-op
}

// waitForRefresh waits for a refresh signal.
func (v *TuiAppView) waitForRefresh() tea.Cmd {
	return func() tea.Msg {
		<-v.refreshCh
		return refreshMsg{}
	}
}

// ShowChat switches to the chat view with the given presenter.
func (v *TuiAppView) ShowChat(presenter ui.IChatPresenter) ui.IChatView {
	// Create or reuse chat view
	if v.chatView == nil {
		chatView, err := NewTuiChatView(presenter)
		if err != nil {
			// In case of error, return nil (this should be handled better in production)
			return nil
		}
		chatView.SetParent(v)
		v.chatView = chatView
	}
	return v.chatView
}

// ShowSettings shows the settings view (not yet implemented).
func (v *TuiAppView) ShowSettings() {
	// TODO: Implement settings view
}

// Model returns the bubbletea model for this app view.
func (v *TuiAppView) Model() tea.Model {
	return v
}

// Init initializes the app view.
func (v *TuiAppView) Init() tea.Cmd {
	var cmds []tea.Cmd

	if v.chatView != nil {
		cmds = append(cmds, v.chatView.model.Init())
	}

	cmds = append(cmds, v.waitForRefresh())

	return tea.Batch(cmds...)
}

// Update handles updates to the app view.
func (v *TuiAppView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case refreshMsg:
		// Just re-render and listen again
		cmds = append(cmds, v.waitForRefresh())

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

		// Update chat view size (minus status bar)
		if v.chatView != nil {
			chatMsg := tea.WindowSizeMsg{
				Width:  msg.Width,
				Height: msg.Height - 1, // Reserve space for status bar
			}
			_, cmd := v.chatView.model.Update(chatMsg)
			cmds = append(cmds, cmd)
		}

	case tea.KeyMsg:
		// If menu is showing, handle menu input
		if v.showingMenu && v.menu != nil {
			_, cmd := v.menu.Update(msg)
			cmds = append(cmds, cmd)

			// Check if menu was closed
			if v.menu.IsClosed() {
				v.showingMenu = false
				v.menu = nil
			}
			return v, tea.Batch(cmds...)
		}

		// Handle global hotkeys
		keyStr := msg.String()
		switch keyStr {
		case "ctrl+p", "\x10": // \x10 is the byte representation of Ctrl+P
			v.showMenu()
			return v, nil
		}

		// Pass through to chat view if not showing menu
		if v.chatView != nil {
			_, cmd := v.chatView.model.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update menu if visible
	if v.showingMenu && v.menu != nil && v.menu.IsVisible() {
		_, cmd := v.menu.Update(msg)
		cmds = append(cmds, cmd)
	}

	return v, tea.Batch(cmds...)
}

// View renders the app view.
func (v *TuiAppView) View() string {
	var view strings.Builder

	// Render chat view or empty space
	if v.chatView != nil {
		view.WriteString(v.chatView.model.View())
	} else {
		// Fill with empty space if no chat view (leave room for status bar)
		for i := 0; i < v.height-1; i++ {
			view.WriteString("\n")
		}
	}

	// Render status bar
	view.WriteString("\n")
	view.WriteString(v.renderStatusBar())

	// Overlay menu if showing
	if v.showingMenu && v.menu != nil && v.menu.IsVisible() {
		// Replace the view with menu overlay
		// For proper overlay, we need to render menu on top
		chatView := view.String()
		menuView := v.menu.View()

		// Simple overlay: just append menu view
		// In a more sophisticated implementation, we would overlay the menu on top of the chat
		return v.overlayMenu(chatView, menuView)
	}

	return view.String()
}

// renderStatusBar renders the bottom status bar.
func (v *TuiAppView) renderStatusBar() string {
	leftStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("235")).
		Padding(0, 1)

	rightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Align(lipgloss.Right)

	leftText := leftStyle.Render("Ctrl+P: Menu")
	rightText := rightStyle.Render(fmt.Sprintf("%s v%s", appName, appVersion))

	// Calculate spacing to fill the width
	leftWidth := lipgloss.Width(leftText)
	rightWidth := lipgloss.Width(rightText)
	spacingWidth := v.width - leftWidth - rightWidth

	if spacingWidth < 0 {
		spacingWidth = 0
	}

	spacing := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Width(spacingWidth).
		Render("")

	return leftText + spacing + rightText
}

// showMenu displays the main menu.
func (v *TuiAppView) showMenu() {
	items := MenuItems{
		{
			Label: "New Session",
			Action: func() {
				v.presenter.NewSession()
			},
		},
		{
			Label: "Exit",
			Action: func() {
				v.presenter.Exit()
			},
		},
	}

	v.menu = NewMenuWidget("Main Menu", items, DefaultMenuColors())
	v.menu.Show()
	v.showingMenu = true
}

// overlayMenu overlays the menu on top of the chat view.
func (v *TuiAppView) overlayMenu(chatView, menuView string) string {
	// Split both views into lines
	chatLines := strings.Split(chatView, "\n")
	menuLines := strings.Split(menuView, "\n")

	// Calculate menu position (centered)
	menuHeight := len(menuLines)
	menuStartLine := (v.height - menuHeight) / 2
	if menuStartLine < 0 {
		menuStartLine = 0
	}

	// Create result lines
	result := make([]string, len(chatLines))
	copy(result, chatLines)

	// Overlay menu lines
	for i, menuLine := range menuLines {
		targetLine := menuStartLine + i
		if targetLine >= 0 && targetLine < len(result) {
			result[targetLine] = menuLine
		}
	}

	return strings.Join(result, "\n")
}

// Ensure TuiAppView implements ui.IAppView
var _ ui.IAppView = (*TuiAppView)(nil)
var _ tea.Model = (*TuiAppView)(nil)
