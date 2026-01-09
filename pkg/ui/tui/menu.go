package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MenuAction represents a function to be called when a menu item is selected.
type MenuAction func()

// MenuItem represents a single menu item with a label and action.
type MenuItem struct {
	Label  string
	Action MenuAction
}

// MenuItems represents a slice of menu items in a stable order.
type MenuItems []MenuItem

// MenuColors defines the color scheme for the menu widget.
type MenuColors struct {
	Background     lipgloss.Color
	Border         lipgloss.Color
	Title          lipgloss.Color
	SelectedItem   lipgloss.Color
	UnselectedItem lipgloss.Color
}

// DefaultMenuColors returns the default color scheme for the menu.
func DefaultMenuColors() MenuColors {
	return MenuColors{
		Background:     lipgloss.Color("235"),
		Border:         lipgloss.Color("69"),
		Title:          lipgloss.Color("214"),
		SelectedItem:   lipgloss.Color("213"),
		UnselectedItem: lipgloss.Color("251"),
	}
}

// MenuWidget is a bubbletea widget that displays a popup menu.
type MenuWidget struct {
	title   string
	items   MenuItems
	colors  MenuColors
	cursor  int
	width   int
	height  int
	visible bool
	closed  bool

	// Styles
	borderStyle    lipgloss.Style
	titleStyle     lipgloss.Style
	itemStyle      lipgloss.Style
	selectedStyle  lipgloss.Style
	containerStyle lipgloss.Style
}

// NewMenuWidget creates a new menu widget with the given title, items, and colors.
func NewMenuWidget(title string, items MenuItems, colors MenuColors) *MenuWidget {
	if colors.Background == "" {
		colors = DefaultMenuColors()
	}

	m := &MenuWidget{
		title:  title,
		items:  items,
		colors: colors,
		cursor: 0,
		width:  40,
		height: len(items) + 4, // Title + border + padding
	}

	m.initStyles()
	return m
}

// initStyles initializes the lipgloss styles for the menu.
func (m *MenuWidget) initStyles() {
	m.containerStyle = lipgloss.NewStyle().
		Background(m.colors.Background).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.colors.Border)

	m.titleStyle = lipgloss.NewStyle().
		Foreground(m.colors.Title).
		Bold(true).
		MarginBottom(1)

	m.itemStyle = lipgloss.NewStyle().
		Foreground(m.colors.UnselectedItem).
		Width(m.width - 4)

	m.selectedStyle = lipgloss.NewStyle().
		Foreground(m.colors.SelectedItem).
		Bold(true).
		Width(m.width - 4)
}

// Show makes the menu visible.
func (m *MenuWidget) Show() {
	m.visible = true
}

// Hide makes the menu invisible.
func (m *MenuWidget) Hide() {
	m.visible = false
}

// IsVisible returns whether the menu is currently visible.
func (m *MenuWidget) IsVisible() bool {
	return m.visible
}

// IsClosed returns whether the menu has been closed.
func (m *MenuWidget) IsClosed() bool {
	return m.closed
}

// Init initializes the menu widget.
func (m *MenuWidget) Init() tea.Cmd {
	return nil
}

// Update handles updates to the menu widget.
func (m *MenuWidget) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.visible || m.closed {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter", "right":
			m.selectItem()
		case "esc", "ctrl+c":
			m.closed = true
			m.visible = false
		}
	}

	return m, nil
}

// View renders the menu widget.
func (m *MenuWidget) View() string {
	if !m.visible || m.closed {
		return ""
	}

	var content strings.Builder

	// Add title
	if m.title != "" {
		content.WriteString(m.titleStyle.Render(m.title))
		content.WriteString("\n")
	}

	// Add menu items
	for i, item := range m.items {
		if i == m.cursor {
			content.WriteString(m.selectedStyle.Render("▶ " + item.Label))
		} else {
			content.WriteString(m.itemStyle.Render("  " + item.Label))
		}
		if i < len(m.items)-1 {
			content.WriteString("\n")
		}
	}

	// Apply container style and center it
	rendered := m.containerStyle.Render(content.String())
	return m.centerInScreen(rendered)
}

// selectItem executes the action for the currently selected item.
func (m *MenuWidget) selectItem() {
	if m.cursor >= 0 && m.cursor < len(m.items) {
		m.items[m.cursor].Action()
		m.closed = true
		m.visible = false
	}
}

// centerInScreen centers the menu content in the terminal.
func (m *MenuWidget) centerInScreen(content string) string {
	lines := strings.Split(content, "\n")

	// Calculate padding to center the menu
	verticalPadding := (24 - len(lines)) / 2 // Assuming 24 rows default
	if verticalPadding < 0 {
		verticalPadding = 0
	}

	horizontalPadding := (80 - m.width) / 2 // Assuming 80 cols default
	if horizontalPadding < 0 {
		horizontalPadding = 0
	}

	var result strings.Builder

	// Add vertical padding
	for i := 0; i < verticalPadding; i++ {
		result.WriteString("\n")
	}

	// Add horizontal padding to each line
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result.WriteString(strings.Repeat(" ", horizontalPadding))
			result.WriteString(line)
		}
		result.WriteString("\n")
	}

	return result.String()
}

// SetColors updates the menu colors and reinitializes styles.
func (m *MenuWidget) SetColors(colors MenuColors) {
	m.colors = colors
	m.initStyles()
}

// SetTitle updates the menu title.
func (m *MenuWidget) SetTitle(title string) {
	m.title = title
}

// SetItems updates the menu items and resets cursor.
func (m *MenuWidget) SetItems(items MenuItems) {
	m.items = items
	m.cursor = 0
	m.height = len(items) + 4
}
