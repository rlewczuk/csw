package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/codesnort/codesnort-swe/pkg/tool"
)

// ToolCallStatus represents the current status of a tool call.
type ToolCallStatus int

const (
	ToolCallStatusStarted ToolCallStatus = iota
	ToolCallStatusExecuting
	ToolCallStatusSucceeded
	ToolCallStatusFailed
)

// ToolCallWidget represents a widget for rendering a single tool call.
type ToolCallWidget struct {
	call    *tool.ToolCall
	result  *tool.ToolResponse
	status  ToolCallStatus
	spinner spinner.Model
}

// NewToolCallWidget creates a new tool call widget.
func NewToolCallWidget(call *tool.ToolCall) *ToolCallWidget {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &ToolCallWidget{
		call:    call,
		status:  ToolCallStatusStarted,
		spinner: s,
	}
}

// Update handles updates to the tool call widget.
func (w *ToolCallWidget) Update(msg tea.Msg) (*ToolCallWidget, tea.Cmd) {
	if w.status == ToolCallStatusStarted || w.status == ToolCallStatusExecuting {
		var cmd tea.Cmd
		w.spinner, cmd = w.spinner.Update(msg)
		return w, cmd
	}
	return w, nil
}

// UpdateDetails updates the tool call details.
func (w *ToolCallWidget) UpdateDetails(call *tool.ToolCall) {
	w.call = call
	if w.status == ToolCallStatusStarted {
		w.status = ToolCallStatusExecuting
	}
}

// SetResult sets the result of the tool call.
func (w *ToolCallWidget) SetResult(result *tool.ToolResponse) {
	w.result = result
	if result.Error != nil {
		w.status = ToolCallStatusFailed
	} else {
		w.status = ToolCallStatusSucceeded
	}
}

// View renders the tool call widget.
func (w *ToolCallWidget) View() string {
	var statusIcon string
	var statusStyle lipgloss.Style

	switch w.status {
	case ToolCallStatusStarted:
		statusIcon = w.spinner.View()
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	case ToolCallStatusExecuting:
		statusIcon = w.spinner.View()
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	case ToolCallStatusSucceeded:
		statusIcon = "✓"
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	case ToolCallStatusFailed:
		statusIcon = "✗"
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	}

	// Build the card content
	var content strings.Builder

	// Tool name and status
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	content.WriteString(statusStyle.Render(statusIcon))
	content.WriteString(" ")
	content.WriteString(titleStyle.Render(w.call.Function))
	content.WriteString("\n")

	// Arguments (if available and not too large)
	if w.call.Arguments.Raw() != nil {
		argsJSON, err := json.MarshalIndent(w.call.Arguments.Raw(), "", "  ")
		if err == nil && len(argsJSON) < 500 {
			argStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
			content.WriteString(argStyle.Render(string(argsJSON)))
			content.WriteString("\n")
		}
	}

	// Result or error
	if w.result != nil {
		if w.result.Error != nil {
			errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			content.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", w.result.Error)))
		} else if w.result.Result.Raw() != nil {
			resultJSON, err := json.MarshalIndent(w.result.Result.Raw(), "", "  ")
			if err == nil && len(resultJSON) < 300 {
				resultStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
				content.WriteString(resultStyle.Render("Result:"))
				content.WriteString("\n")
				content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(string(resultJSON)))
			}
		}
	}

	// Create a bordered card
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	return cardStyle.Render(content.String())
}
