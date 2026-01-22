package tui

import (
	"unicode"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
)

// IInputBox is an interface for input box widgets that allow text input.
// It extends IWidget with input box-specific methods.
type IInputBox interface {
	IFocusable

	// GetText returns the current text in the input box.
	GetText() string

	// SetText sets the text in the input box.
	SetText(text string)

	// GetAttrs returns the text attributes for normal state.
	GetAttrs() gtv.CellAttributes

	// SetAttrs sets the text attributes for normal state.
	SetAttrs(attrs gtv.CellAttributes)

	// GetFocusedAttrs returns the text attributes for focused state.
	GetFocusedAttrs() gtv.CellAttributes

	// SetFocusedAttrs sets the text attributes for focused state.
	SetFocusedAttrs(attrs gtv.CellAttributes)

	// Focus sets focus to the input box.
	Focus()

	// Blur removes focus from the input box.
	Blur()

	// IsFocused returns true if the input box is focused.
	IsFocused() bool

	// SetCursorPos sets the cursor position.
	SetCursorPos(pos int)

	// GetCursorPos returns the cursor position.
	GetCursorPos() int

	// GetSelection returns the selection start and end positions.
	// If there is no selection, both values are equal to cursor position.
	GetSelection() (start, end int)

	// SetSelection sets the selection start and end positions.
	SetSelection(start, end int)

	// ClearSelection clears the selection.
	ClearSelection()
}

// TInputBox is a widget that allows single-line text input with cursor, selection, and editing capabilities.
// It extends TFocusable and implements IInputBox interface.
//
// Features:
// - Single line text input
// - Keyboard input when focused
// - Cursor navigation with arrow keys (Left, Right, Home, End)
// - Text selection with Shift+arrow keys
// - Mouse selection support
// - Text deletion with backspace (Delete key not currently supported due to key mapping collision)
// - Text replacement when typing over selection
// - Focus/blur handling
type TInputBox struct {
	TFocusable

	// Text content
	text string

	// Cursor position (character index, 0-based)
	cursorPos int

	// Selection start and end positions (character indices)
	// If selectionStart == selectionEnd, there is no selection
	selectionStart int
	selectionEnd   int

	// Horizontal scroll offset for long text
	scrollOffset int

	// Track if we're in mouse drag mode
	isDragging   bool
	dragStartPos int
}

// WithText sets the initial text for the input box.
func WithText(text string) gtv.Option {
	return func(w any) {
		if w, ok := w.(*TInputBox); ok {
			w.text = text
			w.cursorPos = len([]rune(text)) // Start at end of text
		}
	}
}

// NewInputBox creates a new input box widget with the specified parent and options.
// The parent parameter is optional (can be nil for root widgets).
// Options can be used to configure text, position, attributes, and other properties.
//
// Default values:
// - Text: ""
// - Position: gtv.TRect{X: 0, Y: 0, W: 0, H: 0}
// - Attrs: gtv.CellTag("input")
// - FocusedAttrs: gtv.CellTag("input-focused")
// - CursorPos: 0
// - SelectionStart: 0
// - SelectionEnd: 0
// - ScrollOffset: 0
//
// Available options:
// - WithText(text) - sets initial text
// - WithRectangle(X, Y, W, H) - sets position and size
// - WithPosition(X, Y) - sets position only
// - WithAttrs(attrs) - sets normal state attributes (use gtv.CellTag("input"))
// - WithFocusedAttrs(attrs) - sets focused state attributes (use gtv.CellTag("input-focused"))
// - WithFlags(flags) - sets widget flags
// - WithChild(child) - adds a child widget
func NewInputBox(parent IWidget, opts ...gtv.Option) *TInputBox {
	inputBox := &TInputBox{
		TFocusable:     *newFocusableBase(parent, opts...),
		text:           "",
		cursorPos:      0,
		selectionStart: 0,
		selectionEnd:   0,
		scrollOffset:   0,
		isDragging:     false,
	}

	// Apply options to TInputBox
	for _, opt := range opts {
		opt(inputBox)
	}

	// Apply default theme tags if no theme tag or colors are specified
	// Note: Check for 0 (zero-value) instead of NoColor because zero-value CellAttributes has 0, not NoColor
	if inputBox.TWidget.cellAttrs.ThemeTag == "" && inputBox.TWidget.cellAttrs.TextColor == 0 && inputBox.TWidget.cellAttrs.BackColor == 0 {
		inputBox.TWidget.cellAttrs = gtv.CellTag("input")
	}
	if inputBox.focusedAttrs.ThemeTag == "" && inputBox.focusedAttrs.TextColor == 0 && inputBox.focusedAttrs.BackColor == 0 {
		inputBox.focusedAttrs = gtv.CellTag("input-focused")
	}

	// Update scroll offset to show cursor
	inputBox.updateScrollOffset()

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(inputBox)
	}

	return inputBox
}

// GetText returns the current text in the input box.
func (i *TInputBox) GetText() string {
	return i.text
}

// SetText sets the text in the input box and moves cursor to end.
func (i *TInputBox) SetText(text string) {
	i.text = text
	// Reset cursor to end and clear selection
	textRunes := []rune(text)
	i.cursorPos = len(textRunes)
	i.ClearSelection()
	i.updateScrollOffset()
}

// Blur removes focus from the input box and clears selection.
// This overrides the TFocusable.Blur() method to add selection clearing.
func (i *TInputBox) Blur() {
	i.TFocusable.Blur()
	i.ClearSelection()
}

// SetCursorPos sets the cursor position.
func (i *TInputBox) SetCursorPos(pos int) {
	textRunes := []rune(i.text)
	if pos < 0 {
		pos = 0
	}
	if pos > len(textRunes) {
		pos = len(textRunes)
	}
	i.cursorPos = pos
	i.updateScrollOffset()
}

// GetCursorPos returns the cursor position.
func (i *TInputBox) GetCursorPos() int {
	return i.cursorPos
}

// GetSelection returns the selection start and end positions.
func (i *TInputBox) GetSelection() (start, end int) {
	if i.selectionStart < i.selectionEnd {
		return i.selectionStart, i.selectionEnd
	}
	return i.selectionEnd, i.selectionStart
}

// SetSelection sets the selection start and end positions.
func (i *TInputBox) SetSelection(start, end int) {
	textRunes := []rune(i.text)
	if start < 0 {
		start = 0
	}
	if start > len(textRunes) {
		start = len(textRunes)
	}
	if end < 0 {
		end = 0
	}
	if end > len(textRunes) {
		end = len(textRunes)
	}
	i.selectionStart = start
	i.selectionEnd = end
}

// ClearSelection clears the selection.
func (i *TInputBox) ClearSelection() {
	i.selectionStart = i.cursorPos
	i.selectionEnd = i.cursorPos
}

// hasSelection returns true if there is an active selection.
func (i *TInputBox) hasSelection() bool {
	return i.selectionStart != i.selectionEnd
}

// deleteSelection deletes the selected text and returns true if text was deleted.
func (i *TInputBox) deleteSelection() bool {
	if !i.hasSelection() {
		return false
	}

	start, end := i.GetSelection()
	textRunes := []rune(i.text)
	i.text = string(textRunes[:start]) + string(textRunes[end:])
	i.cursorPos = start
	i.ClearSelection()
	i.updateScrollOffset()
	return true
}

// updateScrollOffset updates the horizontal scroll offset to ensure cursor is visible.
func (i *TInputBox) updateScrollOffset() {
	width := int(i.Position.W)
	if width <= 0 {
		return
	}

	// Calculate visible cursor position
	visibleCursorPos := i.cursorPos - i.scrollOffset

	// If cursor is before visible area, scroll left
	if visibleCursorPos < 0 {
		i.scrollOffset = i.cursorPos
	}

	// If cursor is after visible area, scroll right
	// We want the cursor to be visible at the right edge
	if visibleCursorPos > width {
		i.scrollOffset = i.cursorPos - width
	}

	// Ensure scroll offset is not negative
	if i.scrollOffset < 0 {
		i.scrollOffset = 0
	}
}

// Draw draws the input box on the screen.
func (i *TInputBox) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if i.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Get absolute position
	absPos := i.GetAbsolutePos()

	// Determine which attributes to use
	currentAttrs := i.GetAttrs()
	if i.IsFocused() {
		currentAttrs = i.GetFocusedAttrs()
	}

	// Get text as runes for proper indexing
	textRunes := []rune(i.text)

	// Calculate visible portion of text
	visibleStart := i.scrollOffset
	visibleEnd := i.scrollOffset + int(absPos.W)
	if visibleEnd > len(textRunes) {
		visibleEnd = len(textRunes)
	}

	// Build visible text with selection highlighting
	cells := make([]gtv.Cell, 0, int(absPos.W))
	selStart, selEnd := i.GetSelection()

	for idx := visibleStart; idx < visibleEnd; idx++ {
		// Determine if this character is selected
		isSelected := i.hasSelection() && idx >= selStart && idx < selEnd

		cellAttrs := currentAttrs
		if isSelected {
			// Invert colors for selection
			cellAttrs.Attributes |= gtv.AttrReverse
		}

		cells = append(cells, gtv.Cell{
			Rune:  textRunes[idx],
			Attrs: cellAttrs,
		})
	}

	// Fill remaining width with spaces
	for len(cells) < int(absPos.W) {
		cells = append(cells, gtv.Cell{
			Rune:  ' ',
			Attrs: currentAttrs,
		})
	}

	// Draw the text
	screen.PutContent(absPos, cells)

	// Update cursor position and style if focused
	if i.IsFocused() {
		visibleCursorX := i.cursorPos - i.scrollOffset
		if visibleCursorX >= 0 && visibleCursorX <= int(absPos.W) {
			screen.MoveCursor(int(absPos.X)+visibleCursorX, int(absPos.Y))
			screen.SetCursorStyle(gtv.CursorStyleBar | gtv.CursorStyleBlinking)
		}
	}

	// Draw children (if any)
	i.TFocusable.Draw(screen)
}

// HandleEvent handles events for the input box.
func (i *TInputBox) HandleEvent(event *TEvent) {
	// Handle input events only if focused
	if event.Type == TEventTypeInput && event.InputEvent != nil {
		inputEvent := event.InputEvent

		// Handle mouse events for focus and selection
		if inputEvent.Type == gtv.InputEventMouse {
			i.handleMouseEvent(inputEvent)
			return
		}

		// Handle keyboard events only when focused
		if i.IsFocused() && inputEvent.Type == gtv.InputEventKey {
			i.handleKeyEvent(inputEvent)
			return
		}
	}

	// Delegate other events to base widget
	i.TFocusable.HandleEvent(event)
}

// handleMouseEvent handles mouse events for focus and text selection.
func (i *TInputBox) handleMouseEvent(event *gtv.InputEvent) {
	absPos := i.GetAbsolutePos()

	// Check if click is within input box
	if !absPos.Contains(event.X, event.Y) {
		// Click outside - lose focus if focused
		if i.IsFocused() {
			i.Blur()
		}
		i.isDragging = false
		return
	}

	// Calculate character position from mouse X coordinate
	relativeX := int(event.X - absPos.X)
	charPos := i.scrollOffset + relativeX

	textRunes := []rune(i.text)
	if charPos > len(textRunes) {
		charPos = len(textRunes)
	}
	if charPos < 0 {
		charPos = 0
	}

	// Handle mouse press - start selection or move cursor
	if event.Modifiers&gtv.ModPress != 0 {
		i.isDragging = true
		i.dragStartPos = charPos
		i.cursorPos = charPos
		i.ClearSelection()
		return
	}

	// Handle mouse drag - extend selection
	if i.isDragging && event.Modifiers&gtv.ModDrag != 0 {
		i.cursorPos = charPos
		i.selectionStart = i.dragStartPos
		i.selectionEnd = charPos
		i.updateScrollOffset()
		return
	}

	// Handle mouse release - end dragging
	if event.Modifiers&gtv.ModRelease != 0 {
		if i.isDragging {
			i.cursorPos = charPos
			i.selectionStart = i.dragStartPos
			i.selectionEnd = charPos
			i.updateScrollOffset()
		}
		i.isDragging = false
		return
	}
}

// handleKeyEvent handles keyboard events for text input and editing.
func (i *TInputBox) handleKeyEvent(event *gtv.InputEvent) {
	textRunes := []rune(i.text)
	hasShift := event.Modifiers&gtv.ModShift != 0

	// Handle navigation keys
	if event.Modifiers&gtv.ModFn != 0 {
		switch event.Key {
		case 'D': // Left arrow or Delete key
			// Left arrow has just ModFn (and possibly ModShift)
			// Delete key would be distinguished by context or additional modifiers
			// For now, treat plain 'D' with ModFn as Left arrow
			// TODO: We need better differentiation for Delete vs Left
			// Based on the ParseKey code, both map to 'D', so we handle Left here
			if i.cursorPos > 0 {
				if hasShift {
					// Extend selection
					if !i.hasSelection() {
						i.selectionStart = i.cursorPos
					}
					i.cursorPos--
					i.selectionEnd = i.cursorPos
				} else {
					// Move cursor
					i.ClearSelection()
					i.cursorPos--
				}
				i.updateScrollOffset()
			}
			return

		case 'C': // Right arrow
			if i.cursorPos < len(textRunes) {
				if hasShift {
					// Extend selection
					if !i.hasSelection() {
						i.selectionStart = i.cursorPos
					}
					i.cursorPos++
					i.selectionEnd = i.cursorPos
				} else {
					// Move cursor
					i.ClearSelection()
					i.cursorPos++
				}
				i.updateScrollOffset()
			}
			return

		case 'H': // Home
			if hasShift {
				// Select to beginning
				if !i.hasSelection() {
					i.selectionStart = i.cursorPos
				}
				i.cursorPos = 0
				i.selectionEnd = i.cursorPos
			} else {
				i.ClearSelection()
				i.cursorPos = 0
			}
			i.updateScrollOffset()
			return

		case 'F': // End
			if hasShift {
				// Select to end
				if !i.hasSelection() {
					i.selectionStart = i.cursorPos
				}
				i.cursorPos = len(textRunes)
				i.selectionEnd = i.cursorPos
			} else {
				i.ClearSelection()
				i.cursorPos = len(textRunes)
			}
			i.updateScrollOffset()
			return
		}
	}

	// Handle backspace
	if event.Key == 0x7F { // Backspace
		if i.hasSelection() {
			i.deleteSelection()
		} else if i.cursorPos > 0 {
			// Delete character before cursor
			textRunes = []rune(i.text)
			i.text = string(textRunes[:i.cursorPos-1]) + string(textRunes[i.cursorPos:])
			i.cursorPos--
			i.updateScrollOffset()
		}
		return
	}

	// Handle printable characters
	if event.Key >= 32 && event.Key <= 126 || event.Key > 126 && unicode.IsPrint(event.Key) {
		// If there's a selection, delete it first
		if i.hasSelection() {
			i.deleteSelection()
			textRunes = []rune(i.text)
		}

		// Insert character at cursor position
		i.text = string(textRunes[:i.cursorPos]) + string(event.Key) + string(textRunes[i.cursorPos:])
		i.cursorPos++
		i.updateScrollOffset()
		return
	}
}
