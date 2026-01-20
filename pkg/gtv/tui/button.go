package tui

import (
	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/util"
)

// IButton is an interface for button widgets that can be clicked.
// It extends IFocusable with button-specific methods.
type IButton interface {
	IFocusable

	// GetText returns the current text of the button.
	GetText() string

	// SetText sets the text of the button.
	SetText(text string)

	// SetEnabled sets whether the button is enabled.
	SetEnabled(enabled bool)

	// IsEnabled returns true if the button is enabled.
	IsEnabled() bool

	// SetOnPress sets the callback that is called when the button is pressed.
	SetOnPress(callback func())

	// Press triggers the button press action.
	Press()
}

// TButton is a struct that implements IButton interface and extends TFocusable.
// It displays a clickable button with text, border, and background.
//
// Visual design:
// - Normal state: [ Button Text ] with border and background
// - Focused state: [ Button Text ] with different colors
// - Disabled state: [ Button Text ] grayed out
// - Pressed state: [ Button Text ] with inverted colors (brief visual feedback)
//
// The button responds to:
// - Enter key: triggers press
// - Space key: triggers press
type TButton struct {
	TFocusable

	// Text to display
	text string

	// Text attributes for disabled state
	disabledAttrs gtv.CellAttributes

	// Callback to call when the button is pressed
	onPress func()

	// Whether the button is currently pressed (for visual feedback)
	pressed bool

	// Formatted cells cache for normal state
	formattedCells []gtv.Cell

	// Formatted cells cache for focused state
	formattedCellsFocused []gtv.Cell

	// Formatted cells cache for disabled state
	formattedCellsDisabled []gtv.Cell

	// Formatted cells cache for pressed state
	formattedCellsPressed []gtv.Cell

	// Whether the formatted cells cache is valid
	cacheValid bool
}

// NewButton creates a new button widget with the specified text, position, and attributes.
// The parent parameter is optional (can be nil for root widgets).
// The rect parameter specifies the position and size of the button.
// If rect width is 0, the button is auto-sized to fit the text plus border.
// If rect height is 0, the button height is set to 1 (single line).
// The attrs parameter specifies text attributes for normal state.
// The focusedAttrs parameter specifies text attributes for focused state.
// The disabledAttrs parameter specifies text attributes for disabled state.
func NewButton(parent IWidget, text string, rect gtv.TRect, attrs, focusedAttrs, disabledAttrs gtv.CellAttributes) *TButton {
	button := &TButton{
		TFocusable:    *newFocusableBase(parent, rect, attrs, focusedAttrs),
		text:          text,
		disabledAttrs: disabledAttrs,
		cacheValid:    false,
	}

	// Auto-size if dimensions are 0
	if rect.W == 0 || rect.H == 0 {
		button.autoSize()
	}

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(button)
	}

	return button
}

// autoSize adjusts the button's dimensions to fit the text plus border
func (b *TButton) autoSize() {
	// Button format: [ Text ]
	// Width = 2 (for [ ]) + text length + 2 (for  ) + 1 (for ]) = text length + 4
	textLen := len([]rune(b.text))
	b.Position.W = uint16(textLen + 4)
	// Button height is always 1 for single line
	b.Position.H = 1
}

// invalidateCache marks the cached formatted cells as invalid
func (b *TButton) invalidateCache() {
	b.cacheValid = false
}

// getFormattedCells returns the formatted cells for the current button state
func (b *TButton) getFormattedCells() []gtv.Cell {
	if b.cacheValid {
		if b.pressed {
			return b.formattedCellsPressed
		}
		if b.Flags&WidgetFlagDisabled != 0 {
			return b.formattedCellsDisabled
		}
		if b.IsFocused() {
			return b.formattedCellsFocused
		}
		return b.formattedCells
	}

	// Determine which attributes to use
	var attrs gtv.CellAttributes
	if b.Flags&WidgetFlagDisabled != 0 {
		attrs = b.disabledAttrs
	} else if b.IsFocused() {
		attrs = b.GetFocusedAttrs()
	} else {
		attrs = b.GetAttrs()
	}

	// Build button text: [ Text ]
	buttonText := "[ " + b.text + " ]"

	// Format text using TextToCells
	cells := util.TextToCells(buttonText)

	// Apply attributes to all cells
	for i := range cells {
		cells[i].Attrs = attrs
	}

	// Cache the formatted cells for all states
	b.formattedCells = b.createCellsWithAttrs(buttonText, b.GetAttrs())
	b.formattedCellsFocused = b.createCellsWithAttrs(buttonText, b.GetFocusedAttrs())
	b.formattedCellsDisabled = b.createCellsWithAttrs(buttonText, b.disabledAttrs)

	// Pressed state: invert text and background colors
	pressedAttrs := b.GetFocusedAttrs()
	pressedAttrs.TextColor, pressedAttrs.BackColor = pressedAttrs.BackColor, pressedAttrs.TextColor
	b.formattedCellsPressed = b.createCellsWithAttrs(buttonText, pressedAttrs)

	b.cacheValid = true

	// Return the appropriate cached cells
	if b.pressed {
		return b.formattedCellsPressed
	}
	if b.Flags&WidgetFlagDisabled != 0 {
		return b.formattedCellsDisabled
	}
	if b.IsFocused() {
		return b.formattedCellsFocused
	}
	return b.formattedCells
}

// createCellsWithAttrs creates formatted cells with the specified attributes
func (b *TButton) createCellsWithAttrs(text string, attrs gtv.CellAttributes) []gtv.Cell {
	cells := util.TextToCells(text)
	for i := range cells {
		cells[i].Attrs = attrs
	}
	return cells
}

// GetText returns the current text of the button.
func (b *TButton) GetText() string {
	return b.text
}

// SetText sets the text of the button and invalidates the cache.
func (b *TButton) SetText(text string) {
	if b.text != text {
		b.text = text
		b.invalidateCache()

		// Auto-size if dimensions were 0
		if b.Position.W == 0 || b.Position.H == 0 {
			b.autoSize()
		}
	}
}

// SetEnabled sets whether the button is enabled.
func (b *TButton) SetEnabled(enabled bool) {
	if enabled {
		b.Flags &= ^WidgetFlagDisabled
	} else {
		b.Flags |= WidgetFlagDisabled
	}
	b.invalidateCache()
}

// IsEnabled returns true if the button is enabled.
func (b *TButton) IsEnabled() bool {
	return b.Flags&WidgetFlagDisabled == 0
}

// SetOnPress sets the callback that is called when the button is pressed.
func (b *TButton) SetOnPress(callback func()) {
	b.onPress = callback
}

// Press triggers the button press action.
// This method is called when the button is activated (Enter/Space key).
// It calls the onPress callback if the button is enabled.
func (b *TButton) Press() {
	if b.IsEnabled() && b.onPress != nil {
		b.onPress()
	}
}

// Draw draws the button on the screen.
func (b *TButton) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if b.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Get absolute position
	absPos := b.GetAbsolutePos()

	// Get formatted cells
	cells := b.getFormattedCells()

	// Draw the formatted button
	if len(cells) > 0 {
		screen.PutContent(absPos, cells)
	}

	// Draw children (if any)
	b.TWidget.Draw(screen)
}

// HandleEvent handles events for the button.
func (b *TButton) HandleEvent(event *TEvent) {
	// Handle position events directly
	if event.Type == TEventTypeResize {
		b.Position.X = event.Rect.X
		b.Position.Y = event.Rect.Y
		b.Position.W = event.Rect.W
		b.Position.H = event.Rect.H
		return
	}

	// Handle input events
	if event.Type == TEventTypeInput && event.InputEvent != nil {
		inputEvent := event.InputEvent

		// Handle mouse events for button press
		if inputEvent.Type == gtv.InputEventMouse && b.IsEnabled() {
			absPos := b.GetAbsolutePos()

			// Check if click is within button bounds
			if absPos.Contains(inputEvent.X, inputEvent.Y) {
				// Handle mouse press - trigger button if focused
				if inputEvent.Modifiers&gtv.ModPress != 0 && b.IsFocused() {
					// Set pressed state for visual feedback
					b.pressed = true
					b.invalidateCache()

					// Trigger the press action
					b.Press()

					// Reset pressed state
					b.pressed = false
					b.invalidateCache()

					return
				}
			}
		}

		// Handle keyboard events when focused
		if inputEvent.Type == gtv.InputEventKey && b.IsEnabled() && b.IsFocused() {
			// Check for Enter or Space key
			if inputEvent.Key == '\r' || inputEvent.Key == '\n' || inputEvent.Key == ' ' {
				// Set pressed state for visual feedback
				b.pressed = true
				b.invalidateCache()

				// Trigger the press action
				b.Press()

				// Reset pressed state
				// In a real application, this would be done after a short delay
				// or on key release, but for simplicity we'll reset it immediately
				// after the callback completes
				b.pressed = false
				b.invalidateCache()

				return
			}
		}
	}

	// Delegate other events to base focusable widget
	b.TFocusable.HandleEvent(event)
}
