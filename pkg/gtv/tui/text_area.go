package tui

import (
	"strings"
	"unicode"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
)

// ITextArea is an interface for text area widgets that allow multi-line text input.
// It extends IFocusable with text area-specific methods.
type ITextArea interface {
	IFocusable

	// GetText returns the current text in the text area.
	GetText() string

	// SetText sets the text in the text area.
	SetText(text string)

	// GetAttrs returns the text attributes for normal state.
	GetAttrs() gtv.CellAttributes

	// SetAttrs sets the text attributes for normal state.
	SetAttrs(attrs gtv.CellAttributes)

	// GetFocusedAttrs returns the text attributes for focused state.
	GetFocusedAttrs() gtv.CellAttributes

	// SetFocusedAttrs sets the text attributes for focused state.
	SetFocusedAttrs(attrs gtv.CellAttributes)

	// Focus sets focus to the text area.
	Focus()

	// Blur removes focus from the text area.
	Blur()

	// IsFocused returns true if the text area is focused.
	IsFocused() bool

	// SetCursorPos sets the cursor position.
	SetCursorPos(line, col int)

	// GetCursorPos returns the cursor position.
	GetCursorPos() (line, col int)

	// GetSelection returns the selection start and end positions.
	// If there is no selection, both values are equal to cursor position.
	GetSelection() (startLine, startCol, endLine, endCol int)

	// SetSelection sets the selection start and end positions.
	SetSelection(startLine, startCol, endLine, endCol int)

	// ClearSelection clears the selection.
	ClearSelection()

	// SetKeyHandler sets a custom key handler that is called before default handling.
	// The handler should return true if the event was handled and should not be processed further.
	SetKeyHandler(handler func(event *gtv.InputEvent) bool)
}

// TTextArea is a widget that allows multi-line text input with cursor, selection, and editing capabilities.
// It extends TFocusable and implements ITextArea interface.
//
// Features:
// - Multi-line text input
// - Keyboard input when focused
// - Cursor navigation with arrow keys (Up, Down, Left, Right, Home, End)
// - Text selection with Shift+arrow keys
// - Mouse selection support
// - Text deletion with backspace
// - Text replacement when typing over selection
// - Focus/blur handling
// - Vertical and horizontal scrolling
type TTextArea struct {
	TFocusable

	// Lines of text content
	lines []string

	// Cursor position (line and column, 0-based)
	cursorLine int
	cursorCol  int

	// Selection start and end positions (line and column indices)
	// If selection start == selection end, there is no selection
	selectionStartLine int
	selectionStartCol  int
	selectionEndLine   int
	selectionEndCol    int

	// Scroll offsets for viewing
	scrollOffsetX int
	scrollOffsetY int

	// Track if we're in mouse drag mode
	isDragging    bool
	dragStartLine int
	dragStartCol  int

	// Custom key handler (optional) - called before default key handling
	// Return true if the event was handled and should not be processed further
	keyHandler func(event *gtv.InputEvent) bool
}

// WithTextAreaText sets the initial text for the text area.
func WithTextAreaText(text string) gtv.Option {
	return func(w any) {
		if w, ok := w.(*TTextArea); ok {
			w.SetText(text)
		}
	}
}

// NewTextArea creates a new text area widget with the specified parent and options.
// The parent parameter is optional (can be nil for root widgets).
// Options can be used to configure text, position, attributes, and other properties.
//
// Default values:
// - Text: ""
// - Position: gtv.TRect{X: 0, Y: 0, W: 0, H: 0}
// - Attrs: gtv.CellTag("textarea")
// - FocusedAttrs: gtv.CellTag("textarea-focused")
// - CursorPos: (0, 0)
// - Selection: no selection
// - ScrollOffset: (0, 0)
//
// Available options:
// - WithTextAreaText(text) - sets initial text
// - WithRectangle(X, Y, W, H) - sets position and size
// - WithPosition(X, Y) - sets position only
// - WithAttrs(attrs) - sets normal state attributes (use gtv.CellTag("textarea"))
// - WithFocusedAttrs(attrs) - sets focused state attributes (use gtv.CellTag("textarea-focused"))
// - WithFlags(flags) - sets widget flags
// - WithChild(child) - adds a child widget
func NewTextArea(parent IWidget, opts ...gtv.Option) *TTextArea {
	textArea := &TTextArea{
		TFocusable:         *newFocusableBase(parent, opts...),
		lines:              []string{""},
		cursorLine:         0,
		cursorCol:          0,
		selectionStartLine: 0,
		selectionStartCol:  0,
		selectionEndLine:   0,
		selectionEndCol:    0,
		scrollOffsetX:      0,
		scrollOffsetY:      0,
		isDragging:         false,
	}

	// Apply options to TTextArea
	for _, opt := range opts {
		opt(textArea)
	}

	// Apply default theme tags if no theme tag or colors are specified
	if textArea.TWidget.cellAttrs.ThemeTag == "" && textArea.TWidget.cellAttrs.TextColor == 0 && textArea.TWidget.cellAttrs.BackColor == 0 {
		textArea.TWidget.cellAttrs = gtv.CellTag("textarea")
	}
	if textArea.focusedAttrs.ThemeTag == "" && textArea.focusedAttrs.TextColor == 0 && textArea.focusedAttrs.BackColor == 0 {
		textArea.focusedAttrs = gtv.CellTag("textarea-focused")
	}

	// Update scroll offset to show cursor
	textArea.updateScrollOffset()

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(textArea)
	}

	return textArea
}

// GetText returns the current text in the text area.
func (t *TTextArea) GetText() string {
	return strings.Join(t.lines, "\n")
}

// SetText sets the text in the text area and moves cursor to end.
func (t *TTextArea) SetText(text string) {
	// Split text into lines
	if text == "" {
		t.lines = []string{""}
	} else {
		t.lines = strings.Split(text, "\n")
	}

	// Reset cursor to end and clear selection
	t.cursorLine = len(t.lines) - 1
	lastLine := []rune(t.lines[t.cursorLine])
	t.cursorCol = len(lastLine)
	t.ClearSelection()
	t.updateScrollOffset()
}

// Blur removes focus from the text area and clears selection.
// This overrides the TFocusable.Blur() method to add selection clearing.
func (t *TTextArea) Blur() {
	t.TFocusable.Blur()
	t.ClearSelection()
}

// SetCursorPos sets the cursor position.
func (t *TTextArea) SetCursorPos(line, col int) {
	if line < 0 {
		line = 0
	}
	if line >= len(t.lines) {
		line = len(t.lines) - 1
	}

	lineRunes := []rune(t.lines[line])
	if col < 0 {
		col = 0
	}
	if col > len(lineRunes) {
		col = len(lineRunes)
	}

	t.cursorLine = line
	t.cursorCol = col
	t.updateScrollOffset()
}

// GetCursorPos returns the cursor position.
func (t *TTextArea) GetCursorPos() (line, col int) {
	return t.cursorLine, t.cursorCol
}

// GetSelection returns the selection start and end positions.
func (t *TTextArea) GetSelection() (startLine, startCol, endLine, endCol int) {
	// Normalize selection so start is always before end
	if t.selectionStartLine < t.selectionEndLine ||
		(t.selectionStartLine == t.selectionEndLine && t.selectionStartCol < t.selectionEndCol) {
		return t.selectionStartLine, t.selectionStartCol, t.selectionEndLine, t.selectionEndCol
	}
	return t.selectionEndLine, t.selectionEndCol, t.selectionStartLine, t.selectionStartCol
}

// SetSelection sets the selection start and end positions.
func (t *TTextArea) SetSelection(startLine, startCol, endLine, endCol int) {
	// Clamp to valid ranges
	if startLine < 0 {
		startLine = 0
	}
	if startLine >= len(t.lines) {
		startLine = len(t.lines) - 1
	}
	if endLine < 0 {
		endLine = 0
	}
	if endLine >= len(t.lines) {
		endLine = len(t.lines) - 1
	}

	startLineRunes := []rune(t.lines[startLine])
	if startCol < 0 {
		startCol = 0
	}
	if startCol > len(startLineRunes) {
		startCol = len(startLineRunes)
	}

	endLineRunes := []rune(t.lines[endLine])
	if endCol < 0 {
		endCol = 0
	}
	if endCol > len(endLineRunes) {
		endCol = len(endLineRunes)
	}

	t.selectionStartLine = startLine
	t.selectionStartCol = startCol
	t.selectionEndLine = endLine
	t.selectionEndCol = endCol
}

// ClearSelection clears the selection.
func (t *TTextArea) ClearSelection() {
	t.selectionStartLine = t.cursorLine
	t.selectionStartCol = t.cursorCol
	t.selectionEndLine = t.cursorLine
	t.selectionEndCol = t.cursorCol
}

// SetKeyHandler sets a custom key handler that is called before default handling.
func (t *TTextArea) SetKeyHandler(handler func(event *gtv.InputEvent) bool) {
	t.keyHandler = handler
}

// hasSelection returns true if there is an active selection.
func (t *TTextArea) hasSelection() bool {
	return t.selectionStartLine != t.selectionEndLine ||
		t.selectionStartCol != t.selectionEndCol
}

// deleteSelection deletes the selected text and returns true if text was deleted.
func (t *TTextArea) deleteSelection() bool {
	if !t.hasSelection() {
		return false
	}

	startLine, startCol, endLine, endCol := t.GetSelection()

	if startLine == endLine {
		// Selection within single line
		lineRunes := []rune(t.lines[startLine])
		t.lines[startLine] = string(lineRunes[:startCol]) + string(lineRunes[endCol:])
	} else {
		// Selection spans multiple lines
		startLineRunes := []rune(t.lines[startLine])
		endLineRunes := []rune(t.lines[endLine])

		// Merge start and end lines
		t.lines[startLine] = string(startLineRunes[:startCol]) + string(endLineRunes[endCol:])

		// Remove lines in between
		t.lines = append(t.lines[:startLine+1], t.lines[endLine+1:]...)
	}

	t.cursorLine = startLine
	t.cursorCol = startCol
	t.ClearSelection()
	t.updateScrollOffset()
	return true
}

// updateScrollOffset updates the scroll offset to ensure cursor is visible.
func (t *TTextArea) updateScrollOffset() {
	width := int(t.Position.W)
	height := int(t.Position.H)

	if width <= 0 || height <= 0 {
		return
	}

	// Vertical scrolling
	if t.cursorLine < t.scrollOffsetY {
		t.scrollOffsetY = t.cursorLine
	}
	if t.cursorLine >= t.scrollOffsetY+height {
		t.scrollOffsetY = t.cursorLine - height + 1
	}

	// Horizontal scrolling
	if t.cursorCol < t.scrollOffsetX {
		t.scrollOffsetX = t.cursorCol
	}
	if t.cursorCol >= t.scrollOffsetX+width {
		t.scrollOffsetX = t.cursorCol - width + 1
	}

	// Ensure scroll offsets are not negative
	if t.scrollOffsetX < 0 {
		t.scrollOffsetX = 0
	}
	if t.scrollOffsetY < 0 {
		t.scrollOffsetY = 0
	}
}

// Draw draws the text area on the screen.
func (t *TTextArea) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if t.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Get absolute position
	absPos := t.GetAbsolutePos()

	// Determine which attributes to use
	currentAttrs := t.GetAttrs()
	if t.IsFocused() {
		currentAttrs = t.GetFocusedAttrs()
	}

	// Get selection range
	selStartLine, selStartCol, selEndLine, selEndCol := t.GetSelection()

	// Draw visible lines
	for row := 0; row < int(absPos.H); row++ {
		lineIdx := t.scrollOffsetY + row
		if lineIdx >= len(t.lines) {
			// Fill empty lines with spaces
			cells := make([]gtv.Cell, int(absPos.W))
			for i := range cells {
				cells[i] = gtv.Cell{Rune: ' ', Attrs: currentAttrs}
			}
			screen.PutContent(gtv.TRect{X: absPos.X, Y: absPos.Y + uint16(row), W: absPos.W, H: 1}, cells)
			continue
		}

		// Get line runes
		lineRunes := []rune(t.lines[lineIdx])

		// Build visible cells for this line
		cells := make([]gtv.Cell, 0, int(absPos.W))
		for col := 0; col < int(absPos.W); col++ {
			runeIdx := t.scrollOffsetX + col
			if runeIdx >= len(lineRunes) {
				// Fill with spaces
				cells = append(cells, gtv.Cell{Rune: ' ', Attrs: currentAttrs})
				continue
			}

			// Determine if this character is selected
			isSelected := t.hasSelection() && t.isPositionInSelection(lineIdx, runeIdx, selStartLine, selStartCol, selEndLine, selEndCol)

			cellAttrs := currentAttrs
			if isSelected {
				// Invert colors for selection
				cellAttrs.Attributes |= gtv.AttrReverse
			}

			cells = append(cells, gtv.Cell{
				Rune:  lineRunes[runeIdx],
				Attrs: cellAttrs,
			})
		}

		screen.PutContent(gtv.TRect{X: absPos.X, Y: absPos.Y + uint16(row), W: absPos.W, H: 1}, cells)
	}

	// Update cursor position and style if focused
	if t.IsFocused() {
		visibleCursorY := t.cursorLine - t.scrollOffsetY
		visibleCursorX := t.cursorCol - t.scrollOffsetX
		if visibleCursorY >= 0 && visibleCursorY < int(absPos.H) &&
			visibleCursorX >= 0 && visibleCursorX <= int(absPos.W) {
			screen.MoveCursor(int(absPos.X)+visibleCursorX, int(absPos.Y)+visibleCursorY)
			screen.SetCursorStyle(gtv.CursorStyleBar | gtv.CursorStyleBlinking)
		}
	}

	// Draw children (if any)
	t.TFocusable.Draw(screen)
}

// isPositionInSelection returns true if the given position is within the selection.
func (t *TTextArea) isPositionInSelection(line, col, selStartLine, selStartCol, selEndLine, selEndCol int) bool {
	if line < selStartLine || line > selEndLine {
		return false
	}
	if line == selStartLine && line == selEndLine {
		return col >= selStartCol && col < selEndCol
	}
	if line == selStartLine {
		return col >= selStartCol
	}
	if line == selEndLine {
		return col < selEndCol
	}
	return true
}

// HandleEvent handles events for the text area.
func (t *TTextArea) HandleEvent(event *TEvent) {
	// Handle input events only if focused
	if event.Type == TEventTypeInput && event.InputEvent != nil {
		inputEvent := event.InputEvent

		// Handle mouse events for focus and selection
		if inputEvent.Type == gtv.InputEventMouse {
			t.handleMouseEvent(inputEvent)
			return
		}

		// Handle keyboard events only when focused
		if t.IsFocused() && inputEvent.Type == gtv.InputEventKey {
			t.handleKeyEvent(inputEvent)
			return
		}
	}

	// Delegate other events to base widget
	t.TFocusable.HandleEvent(event)
}

// handleMouseEvent handles mouse events for focus and text selection.
func (t *TTextArea) handleMouseEvent(event *gtv.InputEvent) {
	absPos := t.GetAbsolutePos()

	// Check if click is within text area
	if !absPos.Contains(event.X, event.Y) {
		// Click outside - lose focus if focused
		if t.IsFocused() {
			t.Blur()
		}
		t.isDragging = false
		return
	}

	// Calculate character position from mouse coordinates
	relativeX := int(event.X - absPos.X)
	relativeY := int(event.Y - absPos.Y)

	line := t.scrollOffsetY + relativeY
	col := t.scrollOffsetX + relativeX

	// Clamp to valid ranges
	if line >= len(t.lines) {
		line = len(t.lines) - 1
	}
	if line < 0 {
		line = 0
	}

	lineRunes := []rune(t.lines[line])
	if col > len(lineRunes) {
		col = len(lineRunes)
	}
	if col < 0 {
		col = 0
	}

	// Handle mouse press - start selection or move cursor
	if event.Modifiers&gtv.ModPress != 0 {
		t.isDragging = true
		t.dragStartLine = line
		t.dragStartCol = col
		t.cursorLine = line
		t.cursorCol = col
		t.ClearSelection()
		t.updateScrollOffset()
		return
	}

	// Handle mouse drag - extend selection
	if t.isDragging && event.Modifiers&gtv.ModDrag != 0 {
		t.cursorLine = line
		t.cursorCol = col
		t.selectionStartLine = t.dragStartLine
		t.selectionStartCol = t.dragStartCol
		t.selectionEndLine = line
		t.selectionEndCol = col
		t.updateScrollOffset()
		return
	}

	// Handle mouse release - end dragging
	if event.Modifiers&gtv.ModRelease != 0 {
		if t.isDragging {
			t.cursorLine = line
			t.cursorCol = col
			t.selectionStartLine = t.dragStartLine
			t.selectionStartCol = t.dragStartCol
			t.selectionEndLine = line
			t.selectionEndCol = col
			t.updateScrollOffset()
		}
		t.isDragging = false
		return
	}
}

// handleKeyEvent handles keyboard events for text input and editing.
func (t *TTextArea) handleKeyEvent(event *gtv.InputEvent) {
	// Call custom key handler first if set
	if t.keyHandler != nil && t.keyHandler(event) {
		return
	}

	hasShift := event.Modifiers&gtv.ModShift != 0

	// Handle navigation keys
	if event.Modifiers&gtv.ModFn != 0 {
		switch event.Key {
		case 'A': // Up arrow
			if t.cursorLine > 0 {
				if hasShift {
					// Extend selection
					if !t.hasSelection() {
						t.selectionStartLine = t.cursorLine
						t.selectionStartCol = t.cursorCol
					}
					t.cursorLine--
					// Adjust cursor column to fit new line
					lineRunes := []rune(t.lines[t.cursorLine])
					if t.cursorCol > len(lineRunes) {
						t.cursorCol = len(lineRunes)
					}
					t.selectionEndLine = t.cursorLine
					t.selectionEndCol = t.cursorCol
				} else {
					// Move cursor
					t.ClearSelection()
					t.cursorLine--
					// Adjust cursor column to fit new line
					lineRunes := []rune(t.lines[t.cursorLine])
					if t.cursorCol > len(lineRunes) {
						t.cursorCol = len(lineRunes)
					}
				}
				t.updateScrollOffset()
			}
			return

		case 'B': // Down arrow
			if t.cursorLine < len(t.lines)-1 {
				if hasShift {
					// Extend selection
					if !t.hasSelection() {
						t.selectionStartLine = t.cursorLine
						t.selectionStartCol = t.cursorCol
					}
					t.cursorLine++
					// Adjust cursor column to fit new line
					lineRunes := []rune(t.lines[t.cursorLine])
					if t.cursorCol > len(lineRunes) {
						t.cursorCol = len(lineRunes)
					}
					t.selectionEndLine = t.cursorLine
					t.selectionEndCol = t.cursorCol
				} else {
					// Move cursor
					t.ClearSelection()
					t.cursorLine++
					// Adjust cursor column to fit new line
					lineRunes := []rune(t.lines[t.cursorLine])
					if t.cursorCol > len(lineRunes) {
						t.cursorCol = len(lineRunes)
					}
				}
				t.updateScrollOffset()
			}
			return

		case 'D': // Left arrow
			if t.cursorCol > 0 {
				if hasShift {
					// Extend selection
					if !t.hasSelection() {
						t.selectionStartLine = t.cursorLine
						t.selectionStartCol = t.cursorCol
					}
					t.cursorCol--
					t.selectionEndLine = t.cursorLine
					t.selectionEndCol = t.cursorCol
				} else {
					// Move cursor
					t.ClearSelection()
					t.cursorCol--
				}
				t.updateScrollOffset()
			} else if t.cursorLine > 0 {
				// Move to end of previous line
				if hasShift {
					if !t.hasSelection() {
						t.selectionStartLine = t.cursorLine
						t.selectionStartCol = t.cursorCol
					}
					t.cursorLine--
					prevLineRunes := []rune(t.lines[t.cursorLine])
					t.cursorCol = len(prevLineRunes)
					t.selectionEndLine = t.cursorLine
					t.selectionEndCol = t.cursorCol
				} else {
					t.ClearSelection()
					t.cursorLine--
					prevLineRunes := []rune(t.lines[t.cursorLine])
					t.cursorCol = len(prevLineRunes)
				}
				t.updateScrollOffset()
			}
			return

		case 'C': // Right arrow
			lineRunes := []rune(t.lines[t.cursorLine])
			if t.cursorCol < len(lineRunes) {
				if hasShift {
					// Extend selection
					if !t.hasSelection() {
						t.selectionStartLine = t.cursorLine
						t.selectionStartCol = t.cursorCol
					}
					t.cursorCol++
					t.selectionEndLine = t.cursorLine
					t.selectionEndCol = t.cursorCol
				} else {
					// Move cursor
					t.ClearSelection()
					t.cursorCol++
				}
				t.updateScrollOffset()
			} else if t.cursorLine < len(t.lines)-1 {
				// Move to beginning of next line
				if hasShift {
					if !t.hasSelection() {
						t.selectionStartLine = t.cursorLine
						t.selectionStartCol = t.cursorCol
					}
					t.cursorLine++
					t.cursorCol = 0
					t.selectionEndLine = t.cursorLine
					t.selectionEndCol = t.cursorCol
				} else {
					t.ClearSelection()
					t.cursorLine++
					t.cursorCol = 0
				}
				t.updateScrollOffset()
			}
			return

		case 'H': // Home
			if hasShift {
				// Select to beginning of line
				if !t.hasSelection() {
					t.selectionStartLine = t.cursorLine
					t.selectionStartCol = t.cursorCol
				}
				t.cursorCol = 0
				t.selectionEndLine = t.cursorLine
				t.selectionEndCol = t.cursorCol
			} else {
				t.ClearSelection()
				t.cursorCol = 0
			}
			t.updateScrollOffset()
			return

		case 'F': // End
			lineRunes := []rune(t.lines[t.cursorLine])
			if hasShift {
				// Select to end of line
				if !t.hasSelection() {
					t.selectionStartLine = t.cursorLine
					t.selectionStartCol = t.cursorCol
				}
				t.cursorCol = len(lineRunes)
				t.selectionEndLine = t.cursorLine
				t.selectionEndCol = t.cursorCol
			} else {
				t.ClearSelection()
				t.cursorCol = len(lineRunes)
			}
			t.updateScrollOffset()
			return
		}
	}

	// Handle Enter key
	if event.Key == '\r' || event.Key == '\n' {
		// If there's a selection, delete it first
		if t.hasSelection() {
			t.deleteSelection()
		}

		// Split current line at cursor position
		lineRunes := []rune(t.lines[t.cursorLine])
		beforeCursor := string(lineRunes[:t.cursorCol])
		afterCursor := string(lineRunes[t.cursorCol:])

		t.lines[t.cursorLine] = beforeCursor

		// Insert new line
		newLines := make([]string, len(t.lines)+1)
		copy(newLines, t.lines[:t.cursorLine+1])
		newLines[t.cursorLine+1] = afterCursor
		copy(newLines[t.cursorLine+2:], t.lines[t.cursorLine+1:])
		t.lines = newLines

		// Move cursor to beginning of new line
		t.cursorLine++
		t.cursorCol = 0
		t.updateScrollOffset()
		return
	}

	// Handle backspace
	if event.Key == 0x7F { // Backspace
		if t.hasSelection() {
			t.deleteSelection()
		} else if t.cursorCol > 0 {
			// Delete character before cursor
			lineRunes := []rune(t.lines[t.cursorLine])
			t.lines[t.cursorLine] = string(lineRunes[:t.cursorCol-1]) + string(lineRunes[t.cursorCol:])
			t.cursorCol--
			t.updateScrollOffset()
		} else if t.cursorLine > 0 {
			// Merge with previous line
			prevLineRunes := []rune(t.lines[t.cursorLine-1])
			t.cursorCol = len(prevLineRunes)
			t.lines[t.cursorLine-1] = t.lines[t.cursorLine-1] + t.lines[t.cursorLine]

			// Remove current line
			t.lines = append(t.lines[:t.cursorLine], t.lines[t.cursorLine+1:]...)
			t.cursorLine--
			t.updateScrollOffset()
		}
		return
	}

	// Handle Delete key
	if event.Modifiers&gtv.ModFn != 0 && event.Key == 'E' {
		if t.hasSelection() {
			t.deleteSelection()
		} else {
			lineRunes := []rune(t.lines[t.cursorLine])
			if t.cursorCol < len(lineRunes) {
				// Delete character at cursor position
				t.lines[t.cursorLine] = string(lineRunes[:t.cursorCol]) + string(lineRunes[t.cursorCol+1:])
				t.updateScrollOffset()
			} else if t.cursorLine < len(t.lines)-1 {
				// At end of line - merge with next line
				t.lines[t.cursorLine] = t.lines[t.cursorLine] + t.lines[t.cursorLine+1]
				// Remove next line
				t.lines = append(t.lines[:t.cursorLine+1], t.lines[t.cursorLine+2:]...)
				t.updateScrollOffset()
			}
		}
		return
	}

	// Handle printable characters
	if event.Key >= 32 && event.Key <= 126 || event.Key > 126 && unicode.IsPrint(event.Key) {
		// If there's a selection, delete it first
		if t.hasSelection() {
			t.deleteSelection()
		}

		// Insert character at cursor position
		lineRunes := []rune(t.lines[t.cursorLine])
		t.lines[t.cursorLine] = string(lineRunes[:t.cursorCol]) + string(event.Key) + string(lineRunes[t.cursorCol:])
		t.cursorCol++
		t.updateScrollOffset()
		return
	}
}
