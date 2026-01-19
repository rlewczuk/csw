package tui_test

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInputBox_Creation tests creating an input box widget
func TestInputBox_Creation(t *testing.T) {
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	require.NotNil(t, inputBox)
	assert.Equal(t, "Hello", inputBox.GetText())
	assert.Equal(t, 5, inputBox.GetCursorPos()) // Cursor at end
	assert.False(t, inputBox.IsFocused())
	assert.Len(t, layout.Children, 1)
}

// TestInputBox_BasicRendering tests that the input box renders correctly
func TestInputBox_BasicRendering(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	_ = tui.NewInputBox(
		layout,
		"Test",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Draw the layout
	layout.Draw(screen)

	// Verify text is rendered at correct position
	width, height, content := screen.GetContent()
	assert.Equal(t, 80, width)
	assert.Equal(t, 24, height)

	// Check text at position (10, 10)
	expectedText := "Test"
	for i, ch := range expectedText {
		idx := 10*width + 10 + i
		assert.Equal(t, ch, content[idx].Rune, "Character at position %d", i)
	}

	// Check remaining cells are spaces
	for i := len(expectedText); i < 20; i++ {
		idx := 10*width + 10 + i
		assert.Equal(t, ' ', content[idx].Rune, "Space at position %d", i)
	}
}

// TestInputBox_FocusBlur tests focus and blur functionality
func TestInputBox_FocusBlur(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		nil,
		"Test",
		gtv.TRect{X: 0, Y: 0, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Initially unfocused
	assert.False(t, inputBox.IsFocused())

	// Focus
	inputBox.Focus()
	assert.True(t, inputBox.IsFocused())

	// Draw and check cursor is visible
	inputBox.Draw(screen)
	cursorX, cursorY := screen.GetCursorPosition()
	assert.Equal(t, 4, cursorX) // Cursor at end of "Test"
	assert.Equal(t, 0, cursorY)
	assert.Equal(t, gtv.CursorStyleBar|gtv.CursorStyleBlinking, screen.GetCursorStyle())

	// Blur
	inputBox.Blur()
	assert.False(t, inputBox.IsFocused())
}

// TestInputBox_MouseClick tests mouse click for focus
func TestInputBox_MouseClick(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Initially unfocused
	assert.False(t, inputBox.IsFocused())

	// Click inside the input box
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.MouseClick(15, 10, 0)

	// Should now be focused
	assert.True(t, inputBox.IsFocused())

	// Manually blur for now (focus management by layout would be better but not in scope)
	inputBox.Blur()
	assert.False(t, inputBox.IsFocused())
}

// TestInputBox_TextInput tests typing text into the input box
func TestInputBox_TextInput(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus the input box and set as active child to receive keyboard events
	inputBox.Focus()
	layout.ActiveChild = inputBox

	// Type some text
	mockInput.TypeKeys("Hello")

	// Verify text was entered
	assert.Equal(t, "Hello", inputBox.GetText())
	assert.Equal(t, 5, inputBox.GetCursorPos())
}

// TestInputBox_Backspace tests backspace functionality
func TestInputBox_Backspace(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and position cursor at end
	inputBox.Focus()
	layout.ActiveChild = inputBox
	assert.Equal(t, 5, inputBox.GetCursorPos())

	// Press backspace
	mockInput.TypeKeysByName("Backspace")

	// Verify last character was deleted
	assert.Equal(t, "Hell", inputBox.GetText())
	assert.Equal(t, 4, inputBox.GetCursorPos())

	// Press backspace again
	mockInput.TypeKeysByName("Backspace")

	assert.Equal(t, "Hel", inputBox.GetText())
	assert.Equal(t, 3, inputBox.GetCursorPos())
}

// TestInputBox_ArrowKeys tests cursor navigation with arrow keys
func TestInputBox_ArrowKeys(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus
	inputBox.Focus()
	layout.ActiveChild = inputBox
	assert.Equal(t, 5, inputBox.GetCursorPos())

	// Press Left arrow
	mockInput.TypeKeysByName("Left")
	assert.Equal(t, 4, inputBox.GetCursorPos())

	// Press Left arrow again
	mockInput.TypeKeysByName("Left")
	assert.Equal(t, 3, inputBox.GetCursorPos())

	// Press Right arrow
	mockInput.TypeKeysByName("Right")
	assert.Equal(t, 4, inputBox.GetCursorPos())

	// Press Home
	mockInput.TypeKeysByName("Home")
	assert.Equal(t, 0, inputBox.GetCursorPos())

	// Press End
	mockInput.TypeKeysByName("End")
	assert.Equal(t, 5, inputBox.GetCursorPos())
}

// TestInputBox_Selection_ShiftArrows tests text selection with Shift+arrow keys
func TestInputBox_Selection_ShiftArrows(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and move to middle
	inputBox.Focus()
	layout.ActiveChild = inputBox
	inputBox.SetCursorPos(2)
	assert.Equal(t, 2, inputBox.GetCursorPos())

	// Select with Shift+Right
	mockInput.TypeKeysByName("Shift+Right")
	start, end := inputBox.GetSelection()
	assert.Equal(t, 2, start)
	assert.Equal(t, 3, end)

	// Extend selection
	mockInput.TypeKeysByName("Shift+Right")
	start, end = inputBox.GetSelection()
	assert.Equal(t, 2, start)
	assert.Equal(t, 4, end)

	// Select in opposite direction
	mockInput.TypeKeysByName("Shift+Left")
	start, end = inputBox.GetSelection()
	assert.Equal(t, 2, start)
	assert.Equal(t, 3, end)
}

// TestInputBox_Selection_ShiftHome tests selecting to beginning with Shift+Home
func TestInputBox_Selection_ShiftHome(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and move to position 3
	inputBox.Focus()
	layout.ActiveChild = inputBox
	inputBox.SetCursorPos(3)

	// Select to beginning with Shift+Home
	mockInput.TypeKeysByName("Shift+Home")
	start, end := inputBox.GetSelection()
	assert.Equal(t, 0, start)
	assert.Equal(t, 3, end)
	assert.Equal(t, 0, inputBox.GetCursorPos())
}

// TestInputBox_Selection_ShiftEnd tests selecting to end with Shift+End
func TestInputBox_Selection_ShiftEnd(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and move to position 2
	inputBox.Focus()
	layout.ActiveChild = inputBox
	inputBox.SetCursorPos(2)

	// Select to end with Shift+End
	mockInput.TypeKeysByName("Shift+End")
	start, end := inputBox.GetSelection()
	assert.Equal(t, 2, start)
	assert.Equal(t, 5, end)
	assert.Equal(t, 5, inputBox.GetCursorPos())
}

// TestInputBox_DeleteSelection tests deleting selected text with backspace
func TestInputBox_DeleteSelection(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello World",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus
	inputBox.Focus()
	layout.ActiveChild = inputBox

	// Select characters 0-5 ("Hello")
	inputBox.SetSelection(0, 5)

	// Delete selection with backspace
	mockInput.TypeKeysByName("Backspace")

	// Verify selection was deleted
	assert.Equal(t, " World", inputBox.GetText())
	assert.Equal(t, 0, inputBox.GetCursorPos())
}

// TestInputBox_ReplaceSelection tests replacing selected text when typing
func TestInputBox_ReplaceSelection(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello World",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus
	inputBox.Focus()
	layout.ActiveChild = inputBox

	// Select characters 0-5 ("Hello")
	inputBox.SetSelection(0, 5)

	// Type new text
	mockInput.TypeKeys("Hi")

	// Verify selection was replaced
	assert.Equal(t, "Hi World", inputBox.GetText())
	assert.Equal(t, 2, inputBox.GetCursorPos())
}

// TestInputBox_MouseSelection tests selecting text with mouse drag
func TestInputBox_MouseSelection(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello World",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus the input box first
	inputBox.Focus()

	// Simulate mouse drag from position 0 to position 5
	// Input box is at screen position (10, 10)
	mockInput.MouseDrag(10, 10, 15, 10)

	// Verify selection
	start, end := inputBox.GetSelection()
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)
}

// TestInputBox_LongText_Scrolling tests horizontal scrolling with long text
func TestInputBox_LongText_Scrolling(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	// Create input box with width of 10 characters
	inputBox := tui.NewInputBox(
		layout,
		"This is a very long text that exceeds the width",
		gtv.TRect{X: 10, Y: 10, W: 10, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Focus
	inputBox.Focus()

	// Cursor should be at end (position 47)
	assert.Equal(t, 47, inputBox.GetCursorPos())

	// Draw and verify that text is scrolled to show cursor
	layout.Draw(screen)

	// Verify that only last 10 characters are visible (scrolled)
	width, _, content := screen.GetContent()
	expectedVisible := " the width"
	for i, ch := range expectedVisible {
		idx := 10*width + 10 + i
		assert.Equal(t, ch, content[idx].Rune, "Character at position %d", i)
	}
}

// TestInputBox_InsertInMiddle tests inserting text in the middle
func TestInputBox_InsertInMiddle(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"HelloWorld",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and move cursor to position 5 (between "Hello" and "World")
	inputBox.Focus()
	layout.ActiveChild = inputBox
	inputBox.SetCursorPos(5)

	// Type a space
	mockInput.TypeKeys(" ")

	// Verify text was inserted
	assert.Equal(t, "Hello World", inputBox.GetText())
	assert.Equal(t, 6, inputBox.GetCursorPos())
}

// TestInputBox_SetText tests programmatically setting text
func TestInputBox_SetText(t *testing.T) {
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		nil,
		"Initial",
		gtv.TRect{X: 0, Y: 0, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Set new text
	inputBox.SetText("New Text")

	// Verify text and cursor position
	assert.Equal(t, "New Text", inputBox.GetText())
	// Cursor should be at end
	assert.Equal(t, 8, inputBox.GetCursorPos())

	// Set cursor to position 3
	inputBox.SetCursorPos(3)

	// Set shorter text
	inputBox.SetText("Hi")

	// Cursor should be adjusted to valid position
	assert.Equal(t, 2, inputBox.GetCursorPos())
}

// TestInputBox_SelectionRendering tests that selection is rendered with reverse attributes
func TestInputBox_SelectionRendering(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Focus and select first 3 characters
	inputBox.Focus()
	inputBox.SetSelection(0, 3)

	// Draw
	layout.Draw(screen)

	// Verify selection is rendered with reverse attribute
	width, _, content := screen.GetContent()

	// Check first 3 characters have reverse attribute
	for i := 0; i < 3; i++ {
		idx := 10*width + 10 + i
		assert.True(t, content[idx].Attrs.Attributes&gtv.AttrReverse != 0,
			"Character %d should have reverse attribute", i)
	}

	// Check remaining characters don't have reverse attribute
	for i := 3; i < 5; i++ {
		idx := 10*width + 10 + i
		assert.False(t, content[idx].Attrs.Attributes&gtv.AttrReverse != 0,
			"Character %d should not have reverse attribute", i)
	}
}

// TestInputBox_EmptyText tests input box with empty text
func TestInputBox_EmptyText(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus
	inputBox.Focus()
	layout.ActiveChild = inputBox

	// Type text
	mockInput.TypeKeys("Test")

	// Verify text was entered
	assert.Equal(t, "Test", inputBox.GetText())
	assert.Equal(t, 4, inputBox.GetCursorPos())
}

// TestInputBox_BackspaceAtBeginning tests that backspace at beginning does nothing
func TestInputBox_BackspaceAtBeginning(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus and move to beginning
	inputBox.Focus()
	layout.ActiveChild = inputBox
	inputBox.SetCursorPos(0)

	// Press backspace
	mockInput.TypeKeysByName("Backspace")

	// Verify text unchanged
	assert.Equal(t, "Hello", inputBox.GetText())
	assert.Equal(t, 0, inputBox.GetCursorPos())
}

// TestInputBox_ArrowKeysBoundaries tests arrow keys at boundaries
func TestInputBox_ArrowKeysBoundaries(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	inputBox := tui.NewInputBox(
		layout,
		"Hello",
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	mockInput := tio.NewMockInputEventReader(app)

	// Focus
	inputBox.Focus()
	layout.ActiveChild = inputBox

	// At end - press Right
	assert.Equal(t, 5, inputBox.GetCursorPos())
	mockInput.TypeKeysByName("Right")
	assert.Equal(t, 5, inputBox.GetCursorPos()) // Should stay at end

	// Go to beginning
	mockInput.TypeKeysByName("Home")
	assert.Equal(t, 0, inputBox.GetCursorPos())

	// Press Left
	mockInput.TypeKeysByName("Left")
	assert.Equal(t, 0, inputBox.GetCursorPos()) // Should stay at beginning
}
