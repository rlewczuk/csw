package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// PermissionQueryWidget is a bubbletea widget that displays a permission query dialog.
type PermissionQueryWidget struct {
	query     *ui.PermissionQueryUI
	callback  func(string)
	cursor    int
	textInput textinput.Model
	visible   bool
	closed    bool
	width     int
	height    int

	// Styles
	containerStyle lipgloss.Style
	titleStyle     lipgloss.Style
	detailsStyle   lipgloss.Style
	itemStyle      lipgloss.Style
	selectedStyle  lipgloss.Style
	borderStyle    lipgloss.Style
}

// NewPermissionQueryWidget creates a new permission query widget.
func NewPermissionQueryWidget(query *ui.PermissionQueryUI, callback func(string)) *PermissionQueryWidget {
	ti := textinput.New()
	ti.Placeholder = query.AllowCustomResponse
	ti.CharLimit = 256
	ti.Width = 50

	w := &PermissionQueryWidget{
		query:     query,
		callback:  callback,
		cursor:    0,
		textInput: ti,
		visible:   false,
		closed:    false,
		width:     60,
		height:    10,
	}

	w.initStyles()
	return w
}

// initStyles initializes the lipgloss styles for the widget.
func (w *PermissionQueryWidget) initStyles() {
	w.containerStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69"))

	w.titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true).
		MarginBottom(1)

	w.detailsStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		MarginBottom(1)

	w.itemStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("251")).
		Width(w.width - 4)

	w.selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("213")).
		Bold(true).
		Width(w.width - 4)

	w.borderStyle = lipgloss.NewStyle().
		BorderForeground(lipgloss.Color("69"))
}

// Show makes the widget visible.
func (w *PermissionQueryWidget) Show() {
	w.visible = true
}

// Hide makes the widget invisible.
func (w *PermissionQueryWidget) Hide() {
	w.visible = false
}

// IsVisible returns whether the widget is currently visible.
func (w *PermissionQueryWidget) IsVisible() bool {
	return w.visible
}

// IsClosed returns whether the widget has been closed.
func (w *PermissionQueryWidget) IsClosed() bool {
	return w.closed
}

// Init initializes the widget.
func (w *PermissionQueryWidget) Init() tea.Cmd {
	return nil
}

// Update handles updates to the widget.
func (w *PermissionQueryWidget) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !w.visible || w.closed {
		return w, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Check if we're in custom input mode
		if w.isInCustomInputMode() {
			switch msg.String() {
			case "esc":
				// Cancel and close
				w.closed = true
				w.visible = false
				if w.callback != nil {
					w.callback("")
				}
				return w, nil
			case "enter":
				// Submit custom response
				response := w.textInput.Value()
				w.closed = true
				w.visible = false
				if w.callback != nil {
					w.callback(response)
				}
				return w, nil
			default:
				// Update text input
				var cmd tea.Cmd
				w.textInput, cmd = w.textInput.Update(msg)
				return w, cmd
			}
		}

		// Handle navigation in option list
		switch msg.String() {
		case "up":
			if w.cursor > 0 {
				w.cursor--
			}
		case "down":
			maxCursor := len(w.query.Options) - 1
			if w.query.AllowCustomResponse != "" {
				maxCursor++ // Add one for custom input option
			}
			if w.cursor < maxCursor {
				w.cursor++
			}
		case "enter":
			// Check if custom input option is selected
			if w.query.AllowCustomResponse != "" && w.cursor == len(w.query.Options) {
				// Switch to custom input mode
				w.textInput.Focus()
				return w, textinput.Blink
			}
			// Otherwise, select the current option
			w.selectOption()
		case "esc":
			// Cancel
			w.closed = true
			w.visible = false
			if w.callback != nil {
				w.callback("")
			}
		}
	}

	return w, nil
}

// View renders the widget.
func (w *PermissionQueryWidget) View() string {
	if !w.visible || w.closed {
		return ""
	}

	var content strings.Builder

	// Add title
	if w.query.Title != "" {
		content.WriteString(w.titleStyle.Render(w.query.Title))
		content.WriteString("\n")
	}

	// Add details
	if w.query.Details != "" {
		content.WriteString(w.detailsStyle.Render(w.query.Details))
		content.WriteString("\n")
	}

	// Add options
	for i, option := range w.query.Options {
		if i == w.cursor && !w.isInCustomInputMode() {
			content.WriteString(w.selectedStyle.Render("▶ " + option))
		} else {
			content.WriteString(w.itemStyle.Render("  " + option))
		}
		content.WriteString("\n")
	}

	// Add custom input option if allowed
	if w.query.AllowCustomResponse != "" {
		customIdx := len(w.query.Options)
		if w.isInCustomInputMode() {
			// Show active text input
			content.WriteString(w.selectedStyle.Render("▶ "))
			content.WriteString(w.textInput.View())
		} else if customIdx == w.cursor {
			// Show selected but not active
			content.WriteString(w.selectedStyle.Render("▶ " + w.query.AllowCustomResponse))
		} else {
			content.WriteString(w.itemStyle.Render("  " + w.query.AllowCustomResponse))
		}
	}

	// Apply container style and center it
	rendered := w.containerStyle.Render(content.String())
	return w.centerInScreen(rendered)
}

// ViewAtBottom renders the widget at the bottom of the screen, replacing the input box.
// This is used for overlay mode where the widget should appear at the bottom while
// keeping chat content visible above.
func (w *PermissionQueryWidget) ViewAtBottom(screenWidth int) string {
	if !w.visible || w.closed {
		return ""
	}

	var content strings.Builder

	// Add title
	if w.query.Title != "" {
		content.WriteString(w.titleStyle.Render(w.query.Title))
		content.WriteString("\n")
	}

	// Add details
	if w.query.Details != "" {
		content.WriteString(w.detailsStyle.Render(w.query.Details))
		content.WriteString("\n")
	}

	// Add options
	for i, option := range w.query.Options {
		if i == w.cursor && !w.isInCustomInputMode() {
			content.WriteString(w.selectedStyle.Render("▶ " + option))
		} else {
			content.WriteString(w.itemStyle.Render("  " + option))
		}
		content.WriteString("\n")
	}

	// Add custom input option if allowed
	if w.query.AllowCustomResponse != "" {
		customIdx := len(w.query.Options)
		if w.isInCustomInputMode() {
			// Show active text input
			content.WriteString(w.selectedStyle.Render("▶ "))
			content.WriteString(w.textInput.View())
		} else if customIdx == w.cursor {
			// Show selected but not active
			content.WriteString(w.selectedStyle.Render("▶ " + w.query.AllowCustomResponse))
		} else {
			content.WriteString(w.itemStyle.Render("  " + w.query.AllowCustomResponse))
		}
	}

	// Apply container style without centering - just render at bottom
	return w.containerStyle.Render(content.String())
}

// selectOption executes the callback with the selected option.
func (w *PermissionQueryWidget) selectOption() {
	if w.cursor >= 0 && w.cursor < len(w.query.Options) {
		response := w.query.Options[w.cursor]
		w.closed = true
		w.visible = false
		if w.callback != nil {
			w.callback(response)
		}
	}
}

// isInCustomInputMode returns true if the custom input is currently active.
func (w *PermissionQueryWidget) isInCustomInputMode() bool {
	return w.textInput.Focused()
}

// GetHeight returns the rendered height of the permission widget (in lines).
// This includes title, details, options, and the border/padding from the container style.
func (w *PermissionQueryWidget) GetHeight() int {
	if !w.visible || w.closed {
		return 0
	}

	height := 0

	// Title + newline
	if w.query.Title != "" {
		height += 2 // Title line + margin bottom (1 line)
	}

	// Details + newline
	if w.query.Details != "" {
		height += 2 // Details line + margin bottom (1 line)
	}

	// Options (one line each)
	height += len(w.query.Options)

	// Custom input option if allowed
	if w.query.AllowCustomResponse != "" {
		height += 1
	}

	// Container style padding (top and bottom) + border (top and bottom)
	// From containerStyle: Padding(1, 2) = 1 line top + 1 line bottom
	// Border adds 2 more lines (top and bottom)
	height += 4

	return height
}

// centerInScreen centers the widget content in the terminal.
func (w *PermissionQueryWidget) centerInScreen(content string) string {
	lines := strings.Split(content, "\n")

	// Calculate padding to center the widget
	verticalPadding := (24 - len(lines)) / 2 // Assuming 24 rows default
	if verticalPadding < 0 {
		verticalPadding = 0
	}

	horizontalPadding := (80 - w.width) / 2 // Assuming 80 cols default
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
