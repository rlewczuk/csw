package tui_test

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/rlewczuk/csw/pkg/gtv/tio"
	"github.com/rlewczuk/csw/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTextArea_Creation tests creating a text area widget
func TestTextArea_Creation(t *testing.T) {
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello\nWorld"),
		tui.WithRectangle(10, 10, 40, 10),
		tui.WithAttrs(normalAttrs),
		tui.WithFocusedAttrs(focusedAttrs),
	)

	require.NotNil(t, textArea)
	assert.Equal(t, "Hello\nWorld", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 1, line) // Last line
	assert.Equal(t, 5, col)  // End of "World"
	assert.False(t, textArea.IsFocused())
	assert.Len(t, layout.Children, 1)
}

// TestTextArea_EmptyText tests creating text area with empty text
func TestTextArea_EmptyText(t *testing.T) {
	textArea := tui.NewTextArea(nil, tui.WithRectangle(0, 0, 20, 5))

	require.NotNil(t, textArea)
	assert.Equal(t, "", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 0, col)
}

// TestTextArea_BasicRendering tests that the text area renders correctly
func TestTextArea_BasicRendering(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	_ = tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Line 1\nLine 2\nLine 3"),
		tui.WithRectangle(5, 5, 20, 5),
		tui.WithAttrs(normalAttrs),
		tui.WithFocusedAttrs(focusedAttrs),
	)

	// Draw the layout
	layout.Draw(screen)

	// Verify text is rendered at correct positions
	width, _, content := screen.GetContent()

	// Check "Line 1" at position (5, 5)
	expectedLine1 := "Line 1"
	for i, ch := range expectedLine1 {
		idx := 5*width + 5 + i
		assert.Equal(t, ch, content[idx].Rune, "Line 1 character at position %d", i)
	}

	// Check "Line 2" at position (5, 6)
	expectedLine2 := "Line 2"
	for i, ch := range expectedLine2 {
		idx := 6*width + 5 + i
		assert.Equal(t, ch, content[idx].Rune, "Line 2 character at position %d", i)
	}

	// Check "Line 3" at position (5, 7)
	expectedLine3 := "Line 3"
	for i, ch := range expectedLine3 {
		idx := 7*width + 5 + i
		assert.Equal(t, ch, content[idx].Rune, "Line 3 character at position %d", i)
	}
}

// TestTextArea_FocusBlur tests focus and blur functionality
func TestTextArea_FocusBlur(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	textArea := tui.NewTextArea(
		nil,
		tui.WithTextAreaText("Test"),
		tui.WithRectangle(0, 0, 20, 5),
		tui.WithAttrs(normalAttrs),
		tui.WithFocusedAttrs(focusedAttrs),
	)

	// Initially unfocused
	assert.False(t, textArea.IsFocused())

	// Focus
	textArea.Focus()
	assert.True(t, textArea.IsFocused())

	// Draw and check cursor is visible
	textArea.Draw(screen)
	cursorX, cursorY := screen.GetCursorPosition()
	assert.Equal(t, 4, cursorX) // Cursor at end of "Test"
	assert.Equal(t, 0, cursorY)
	assert.Equal(t, gtv.CursorStyleBar|gtv.CursorStyleBlinking, screen.GetCursorStyle())

	// Blur
	textArea.Blur()
	assert.False(t, textArea.IsFocused())
}

// TestTextArea_MouseClick tests mouse click for focus and cursor positioning
func TestTextArea_MouseClick(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello\nWorld"),
		tui.WithRectangle(10, 10, 20, 5),
		tui.WithAttrs(normalAttrs),
		tui.WithFocusedAttrs(focusedAttrs),
	)

	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Initially unfocused
	assert.False(t, textArea.IsFocused())

	// Click inside the text area at first line, position 2
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.MouseClick(12, 10, 0)

	// Should now be focused with cursor at click position
	assert.True(t, textArea.IsFocused())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 2, col)
}

// TestTextArea_TextInput tests typing text into the text area
func TestTextArea_TextInput(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText(""),
		tui.WithRectangle(10, 10, 20, 5),
		tui.WithAttrs(normalAttrs),
		tui.WithFocusedAttrs(focusedAttrs),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus the text area and set as active child to receive keyboard events
	textArea.Focus()
	layout.ActiveChild = textArea

	// Type some text
	mockInput.TypeKeys("Hello")

	// Verify text was entered
	assert.Equal(t, "Hello", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 5, col)
}

// TestTextArea_EnterKey tests inserting new lines with Enter key
func TestTextArea_EnterKey(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello"),
		tui.WithRectangle(10, 10, 20, 5),
		tui.WithAttrs(normalAttrs),
		tui.WithFocusedAttrs(focusedAttrs),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position cursor at end
	textArea.Focus()
	layout.ActiveChild = textArea

	// Press Enter
	mockInput.TypeKeysByName("Enter")

	// Verify new line was created
	assert.Equal(t, "Hello\n", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 1, line)
	assert.Equal(t, 0, col)

	// Type more text
	mockInput.TypeKeys("World")
	assert.Equal(t, "Hello\nWorld", textArea.GetText())
	line, col = textArea.GetCursorPos()
	assert.Equal(t, 1, line)
	assert.Equal(t, 5, col)
}

// TestTextArea_EnterKeyMidLine tests splitting a line with Enter key
func TestTextArea_EnterKeyMidLine(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("HelloWorld"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position cursor in middle
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(0, 5) // After "Hello"

	// Press Enter
	mockInput.TypeKeysByName("Enter")

	// Verify line was split
	assert.Equal(t, "Hello\nWorld", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 1, line)
	assert.Equal(t, 0, col)
}

// TestTextArea_Backspace tests backspace functionality
func TestTextArea_Backspace(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position cursor at end
	textArea.Focus()
	layout.ActiveChild = textArea

	// Press backspace
	mockInput.TypeKeysByName("Backspace")

	// Verify last character was deleted
	assert.Equal(t, "Hell", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 4, col)
}

// TestTextArea_BackspaceMultiLine tests backspace across lines
func TestTextArea_BackspaceMultiLine(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello\nWorld"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position cursor at beginning of second line
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(1, 0)

	// Press backspace - should merge lines
	mockInput.TypeKeysByName("Backspace")

	// Verify lines were merged
	assert.Equal(t, "HelloWorld", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 5, col)
}

// TestTextArea_ArrowKeys tests cursor navigation with arrow keys
func TestTextArea_ArrowKeys(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Line 1\nLine 2\nLine 3"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus
	textArea.Focus()
	layout.ActiveChild = textArea

	// Start at end (line 2, col 6)
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 2, line)
	assert.Equal(t, 6, col)

	// Press Up arrow
	mockInput.TypeKeysByName("Up")
	line, col = textArea.GetCursorPos()
	assert.Equal(t, 1, line)
	assert.Equal(t, 6, col)

	// Press Up arrow again
	mockInput.TypeKeysByName("Up")
	line, col = textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 6, col)

	// Press Down arrow
	mockInput.TypeKeysByName("Down")
	line, col = textArea.GetCursorPos()
	assert.Equal(t, 1, line)
	assert.Equal(t, 6, col)

	// Press Left arrow
	mockInput.TypeKeysByName("Left")
	line, col = textArea.GetCursorPos()
	assert.Equal(t, 1, line)
	assert.Equal(t, 5, col)

	// Press Right arrow
	mockInput.TypeKeysByName("Right")
	line, col = textArea.GetCursorPos()
	assert.Equal(t, 1, line)
	assert.Equal(t, 6, col)
}

// TestTextArea_ArrowKeysAcrossLines tests cursor navigation across line boundaries
func TestTextArea_ArrowKeysAcrossLines(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello\nWorld"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position at beginning of second line
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(1, 0)

	// Press Left arrow - should move to end of previous line
	mockInput.TypeKeysByName("Left")
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 5, col)

	// Press Right arrow - should move to beginning of next line
	mockInput.TypeKeysByName("Right")
	line, col = textArea.GetCursorPos()
	assert.Equal(t, 1, line)
	assert.Equal(t, 0, col)
}

// TestTextArea_HomeEnd tests Home and End keys
func TestTextArea_HomeEnd(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello World"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus
	textArea.Focus()
	layout.ActiveChild = textArea

	// Position cursor in middle
	textArea.SetCursorPos(0, 6)

	// Press Home
	mockInput.TypeKeysByName("Home")
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 0, col)

	// Press End
	mockInput.TypeKeysByName("End")
	line, col = textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 11, col)
}

// TestTextArea_Selection_ShiftArrows tests text selection with Shift+arrow keys
func TestTextArea_Selection_ShiftArrows(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello\nWorld"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position at beginning
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(0, 0)

	// Select with Shift+Right
	mockInput.TypeKeysByName("Shift+Right")
	startLine, startCol, endLine, endCol := textArea.GetSelection()
	assert.Equal(t, 0, startLine)
	assert.Equal(t, 0, startCol)
	assert.Equal(t, 0, endLine)
	assert.Equal(t, 1, endCol)

	// Extend selection
	mockInput.TypeKeysByName("Shift+Right")
	mockInput.TypeKeysByName("Shift+Right")
	startLine, startCol, endLine, endCol = textArea.GetSelection()
	assert.Equal(t, 0, startLine)
	assert.Equal(t, 0, startCol)
	assert.Equal(t, 0, endLine)
	assert.Equal(t, 3, endCol)
}

// TestTextArea_Selection_ShiftArrowsMultiLine tests selection across lines
func TestTextArea_Selection_ShiftArrowsMultiLine(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello\nWorld"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position at end of first line
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(0, 5)

	// Select across lines with Shift+Down
	mockInput.TypeKeysByName("Shift+Down")
	startLine, startCol, endLine, endCol := textArea.GetSelection()
	assert.Equal(t, 0, startLine)
	assert.Equal(t, 5, startCol)
	assert.Equal(t, 1, endLine)
	assert.Equal(t, 5, endCol)
}

// TestTextArea_Selection_Delete tests deleting selected text
func TestTextArea_Selection_Delete(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello World"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and select "World" (characters 6-11)
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(0, 6)
	textArea.SetSelection(0, 6, 0, 11)

	// Type a character - should replace selection
	mockInput.TypeKeys("X")

	assert.Equal(t, "Hello X", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 7, col)
}

// TestTextArea_Selection_DeleteMultiLine tests deleting multi-line selection
func TestTextArea_Selection_DeleteMultiLine(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Line 1\nLine 2\nLine 3"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and select from middle of first line to middle of last line
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(0, 3)
	textArea.SetSelection(0, 3, 2, 3)

	// Type a character - should replace entire selection
	mockInput.TypeKeys("X")

	assert.Equal(t, "LinXe 3", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 4, col)
}

// TestTextArea_SetText tests setting text programmatically
func TestTextArea_SetText(t *testing.T) {
	textArea := tui.NewTextArea(nil, tui.WithRectangle(0, 0, 20, 5))

	textArea.SetText("First\nSecond\nThird")

	assert.Equal(t, "First\nSecond\nThird", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 2, line) // Last line
	assert.Equal(t, 5, col)  // End of "Third"
}

// TestTextArea_Scrolling tests that scrolling works when cursor moves out of view
func TestTextArea_Scrolling(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create a text area with many lines but limited height
	text := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10"
	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText(text),
		tui.WithRectangle(0, 0, 20, 3), // Only 3 lines visible
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position at beginning
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(0, 0)

	// Move down several times - should trigger vertical scrolling
	for i := 0; i < 5; i++ {
		mockInput.TypeKeysByName("Down")
	}

	line, col := textArea.GetCursorPos()
	assert.Equal(t, 5, line)
	assert.Equal(t, 0, col)

	// Draw and verify rendering doesn't panic
	layout.Draw(screen)
}

// TestTextArea_HorizontalScrolling tests horizontal scrolling with long lines
func TestTextArea_HorizontalScrolling(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create a text area with a long line but limited width
	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("This is a very long line that should trigger horizontal scrolling"),
		tui.WithRectangle(0, 0, 10, 3), // Only 10 chars wide
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position at beginning
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(0, 0)

	// Move right several times - should trigger horizontal scrolling
	for i := 0; i < 15; i++ {
		mockInput.TypeKeysByName("Right")
	}

	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 15, col)

	// Draw and verify rendering doesn't panic
	layout.Draw(screen)
}

// TestTextArea_MouseSelection tests mouse drag selection
func TestTextArea_MouseSelection(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello\nWorld"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Simulate mouse drag from (10, 10) to (15, 10) - select 5 characters
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.MouseDrag(10, 10, 15, 10)

	// Verify selection
	startLine, startCol, endLine, endCol := textArea.GetSelection()
	assert.Equal(t, 0, startLine)
	assert.Equal(t, 0, startCol)
	assert.Equal(t, 0, endLine)
	assert.Equal(t, 5, endCol)
}

// TestTextArea_BlurClearsSelection tests that blur clears selection
func TestTextArea_BlurClearsSelection(t *testing.T) {
	textArea := tui.NewTextArea(
		nil,
		tui.WithTextAreaText("Hello World"),
		tui.WithRectangle(0, 0, 20, 5),
	)

	// Focus and create selection
	textArea.Focus()
	textArea.SetCursorPos(0, 5)
	textArea.SetSelection(0, 0, 0, 5)

	startLine, startCol, endLine, endCol := textArea.GetSelection()
	assert.Equal(t, 0, startLine)
	assert.Equal(t, 0, startCol)
	assert.Equal(t, 0, endLine)
	assert.Equal(t, 5, endCol)

	// Blur should clear selection - selection will be at cursor position
	textArea.Blur()

	startLine, startCol, endLine, endCol = textArea.GetSelection()
	line, col := textArea.GetCursorPos()
	assert.Equal(t, line, startLine)
	assert.Equal(t, col, startCol)
	assert.Equal(t, line, endLine)
	assert.Equal(t, col, endCol)
}

// TestTextArea_DeleteKey tests Delete key functionality
func TestTextArea_DeleteKey(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position cursor at beginning
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(0, 0)

	// Press Delete - should delete 'H'
	mockInput.TypeKeysByName("Delete")

	// Verify character was deleted
	assert.Equal(t, "ello", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 0, col)

	// Position cursor in middle
	textArea.SetCursorPos(0, 1)

	// Press Delete - should delete 'l'
	mockInput.TypeKeysByName("Delete")

	// Verify character was deleted
	assert.Equal(t, "elo", textArea.GetText())
	line, col = textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 1, col)
}

// TestTextArea_DeleteKeyMultiLine tests Delete key at end of line
func TestTextArea_DeleteKeyMultiLine(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello\nWorld"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position cursor at end of first line
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(0, 5)

	// Press Delete - should merge lines
	mockInput.TypeKeysByName("Delete")

	// Verify lines were merged
	assert.Equal(t, "HelloWorld", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 5, col)
}

// TestTextArea_DeleteKeyWithSelection tests Delete key with selection
func TestTextArea_DeleteKeyWithSelection(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Hello World"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and select "World" (characters 6-11)
	textArea.Focus()
	layout.ActiveChild = textArea
	textArea.SetCursorPos(0, 6)
	textArea.SetSelection(0, 6, 0, 11)

	// Press Delete - should delete selection
	mockInput.TypeKeysByName("Delete")

	assert.Equal(t, "Hello ", textArea.GetText())
	line, col := textArea.GetCursorPos()
	assert.Equal(t, 0, line)
	assert.Equal(t, 6, col)
}

// TestTextArea_SetKeyHandler tests setting and using custom key handler
func TestTextArea_SetKeyHandler(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText(""),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and set as active
	textArea.Focus()
	layout.ActiveChild = textArea

	// Track if handler was called
	handlerCalled := false
	handlerEvent := (*gtv.InputEvent)(nil)

	// Set custom key handler for Alt+Enter
	textArea.SetKeyHandler(func(event *gtv.InputEvent) bool {
		handlerCalled = true
		handlerEvent = event
		// Check for Alt+Enter (Enter key is \r or \n with ModAlt)
		if (event.Key == '\r' || event.Key == '\n') && event.Modifiers&gtv.ModAlt != 0 {
			return true // Handled, don't process further
		}
		return false // Not handled, continue with default handling
	})

	// Type some text first
	mockInput.TypeKeys("Test")
	assert.Equal(t, "Test", textArea.GetText())

	// Send Alt+Enter key
	handlerCalled = false
	mockInput.PressKey('\r', gtv.ModAlt)

	// Verify handler was called and event was handled
	assert.True(t, handlerCalled, "Custom key handler should be called")
	assert.NotNil(t, handlerEvent, "Handler should receive event")
	assert.Equal(t, rune('\r'), handlerEvent.Key)
	assert.True(t, handlerEvent.Modifiers&gtv.ModAlt != 0, "Event should have ModAlt")

	// Verify text was NOT modified (handler returned true, so default handling was skipped)
	assert.Equal(t, "Test", textArea.GetText(), "Text should not change when handler returns true")

	// Now send normal Enter key (without Alt)
	handlerCalled = false
	mockInput.TypeKeysByName("Enter")

	// Verify handler was called but didn't handle it
	assert.True(t, handlerCalled, "Handler should be called for all key events")

	// Verify new line was added (default handling)
	assert.Equal(t, "Test\n", textArea.GetText(), "Normal Enter should still work when handler returns false")
}

// TestTextArea_KeyHandlerAltEnter tests Alt+Enter handler that prevents newline
func TestTextArea_KeyHandlerAltEnter(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	textArea := tui.NewTextArea(
		layout,
		tui.WithTextAreaText("Line 1"),
		tui.WithRectangle(10, 10, 20, 5),
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and set as active
	textArea.Focus()
	layout.ActiveChild = textArea

	submittedText := ""

	// Set handler that captures text on Alt+Enter
	textArea.SetKeyHandler(func(event *gtv.InputEvent) bool {
		if (event.Key == '\r' || event.Key == '\n') && event.Modifiers&gtv.ModAlt != 0 {
			submittedText = textArea.GetText()
			return true // Prevent default newline insertion
		}
		return false
	})

	// Send Alt+Enter
	mockInput.PressKey('\r', gtv.ModAlt)

	// Verify text was captured
	assert.Equal(t, "Line 1", submittedText, "Handler should capture text on Alt+Enter")
	// Verify no newline was added
	assert.Equal(t, "Line 1", textArea.GetText(), "Alt+Enter should not add newline when handler returns true")

	// Send normal Enter
	mockInput.TypeKeysByName("Enter")

	// Verify newline was added
	assert.Equal(t, "Line 1\n", textArea.GetText(), "Normal Enter should add newline")
}
