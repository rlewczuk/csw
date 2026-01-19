package tui

import (
	"github.com/codesnort/codesnort-swe/pkg/gophertv"
	"github.com/codesnort/codesnort-swe/pkg/gophertv/util"
)

// ILabel is an interface for label widgets that display text on the screen.
// It extends IWidget with label-specific methods.
type ILabel interface {
	IWidget

	// GetText returns the current text of the label.
	GetText() string

	// SetText sets the text of the label.
	SetText(text string)

	// GetAttrs returns the text attributes of the label.
	GetAttrs() gophertv.CellAttributes

	// SetAttrs sets the text attributes of the label.
	SetAttrs(attrs gophertv.CellAttributes)
}

// TLabel is a struct that implements ILabel interface and extends TWidget.
// It displays text on the screen with formatting support.
type TLabel struct {
	TWidget

	// Text to display
	text string

	// Base attributes (colors and base attribute mask)
	attrs gophertv.CellAttributes

	// Formatted cells cache
	formattedCells []gophertv.Cell

	// Whether the formatted cells cache is valid
	cacheValid bool
}

// NewLabel creates a new label widget with the specified text, position, and attributes.
// If rect width and height are 0, the label is auto-sized to fit the text.
// The parent parameter is optional (can be nil for root widgets).
func NewLabel(parent IWidget, text string, rect gophertv.TRect, attrs gophertv.CellAttributes) *TLabel {
	label := &TLabel{
		TWidget: TWidget{
			Position: rect,
			Parent:   parent,
		},
		text:       text,
		attrs:      attrs,
		cacheValid: false,
	}

	// Auto-size if dimensions are 0
	if rect.W == 0 || rect.H == 0 {
		label.autoSize()
	}

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(label)
	}

	return label
}

// autoSize adjusts the label's dimensions to fit the text
func (l *TLabel) autoSize() {
	cells := l.getFormattedCells()
	if len(cells) > 0 {
		// Label width is the text length (single line)
		l.Position.W = uint16(len(cells))
		// Label height is always 1 for single line text
		l.Position.H = 1
	} else {
		l.Position.W = 0
		l.Position.H = 1
	}
}

// getFormattedCells returns the formatted cells, using cache if valid
func (l *TLabel) getFormattedCells() []gophertv.Cell {
	if l.cacheValid {
		return l.formattedCells
	}

	// Format text using TextToCells
	cells := util.TextToCells(l.text)

	// Combine attributes: base attributes provide colors and base mask,
	// formatted text attributes provide additional attributes (bold, italic, etc.)
	for i := range cells {
		// Start with base attributes from constructor
		combinedAttrs := l.attrs

		// OR the formatted text attributes with base attributes
		combinedAttrs.Attributes |= cells[i].Attrs.Attributes

		// Use colors from base attributes if they are set, otherwise use formatted colors
		if combinedAttrs.TextColor == 0 {
			combinedAttrs.TextColor = cells[i].Attrs.TextColor
		}
		if combinedAttrs.BackColor == 0 {
			combinedAttrs.BackColor = cells[i].Attrs.BackColor
		}
		if combinedAttrs.StrikeColor == 0 {
			combinedAttrs.StrikeColor = cells[i].Attrs.StrikeColor
		}
		if combinedAttrs.ThemeTag == 0 {
			combinedAttrs.ThemeTag = cells[i].Attrs.ThemeTag
		}

		cells[i].Attrs = combinedAttrs
	}

	l.formattedCells = cells
	l.cacheValid = true

	return l.formattedCells
}

// invalidateCache marks the cached formatted cells as invalid
func (l *TLabel) invalidateCache() {
	l.cacheValid = false
}

// GetText returns the current text of the label.
func (l *TLabel) GetText() string {
	return l.text
}

// SetText sets the text of the label and invalidates the cache.
func (l *TLabel) SetText(text string) {
	if l.text != text {
		l.text = text
		l.invalidateCache()

		// Auto-size if dimensions were 0
		if l.Position.W == 0 || l.Position.H == 0 {
			l.autoSize()
		}
	}
}

// GetAttrs returns the text attributes of the label.
func (l *TLabel) GetAttrs() gophertv.CellAttributes {
	return l.attrs
}

// SetAttrs sets the text attributes of the label and invalidates the cache.
func (l *TLabel) SetAttrs(attrs gophertv.CellAttributes) {
	l.attrs = attrs
	l.invalidateCache()
}

// Draw draws the label on the screen.
func (l *TLabel) Draw(screen gophertv.IScreenOutput) {
	// Don't draw if hidden
	if l.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Get absolute position
	absPos := l.GetAbsolutePos()

	// Get formatted cells
	cells := l.getFormattedCells()

	// Draw the formatted text
	if len(cells) > 0 {
		screen.PutContent(absPos, cells)
	}

	// Draw children (if any)
	l.TWidget.Draw(screen)
}

// HandleEvent handles events for the label.
// Labels are passive widgets and only handle position events.
func (l *TLabel) HandleEvent(event *TEvent) {
	// Handle position events directly
	if event.Type == TEventTypePosition {
		l.Position.X = event.Rect.X
		l.Position.Y = event.Rect.Y
		l.Position.W = event.Rect.W
		l.Position.H = event.Rect.H
		return
	}

	// Delegate other events to base widget
	l.TWidget.HandleEvent(event)
}
