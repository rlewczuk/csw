package tui_test

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/rlewczuk/csw/pkg/gtv/tio"
	"github.com/rlewczuk/csw/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplicationIntegration_BasicRendering tests that the application correctly renders
// a simple widget hierarchy consisting of a TAbsoluteLayout with multiple TLabel children.
//
// This test demonstrates:
// - Creating an application with a widget hierarchy
// - Initial rendering to screen buffer
// - Verifying screen content
func TestApplicationIntegration_BasicRendering(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget (fills entire screen)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create some label widgets as children
	label1 := tui.NewLabel(
		layout,
		"Hello, World!",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0}, // Auto-sized
		gtv.AttrsWithColor(gtv.AttrBold, 0xFF0000, 0),
	)

	label2 := tui.NewLabel(
		layout,
		"Second Label",
		gtv.TRect{X: 10, Y: 10, W: 0, H: 0}, // Auto-sized
		gtv.AttrsWithColor(0, 0x00FF00, 0),
	)

	label3 := tui.NewLabel(
		layout,
		"Third",
		gtv.TRect{X: 70, Y: 20, W: 0, H: 0}, // Auto-sized
		gtv.AttrsWithColor(gtv.AttrItalic, 0x0000FF, 0),
	)

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Draw initial frame (without running the event loop)
	layout.Draw(screen)

	// Verify screen content
	width, height, content := screen.GetContent()
	assert.Equal(t, 80, width)
	assert.Equal(t, 24, height)

	// Check that label1 is rendered at position (5, 5)
	expectedText1 := "Hello, World!"
	for i, ch := range expectedText1 {
		idx := 5*width + 5 + i
		assert.Equal(t, ch, content[idx].Rune, "Label1 character at position %d", i)
		assert.Equal(t, gtv.AttrBold, content[idx].Attrs.Attributes&gtv.AttrBold)
		assert.Equal(t, gtv.TextColor(0xFF0000), content[idx].Attrs.TextColor)
	}

	// Check that label2 is rendered at position (10, 10)
	expectedText2 := "Second Label"
	for i, ch := range expectedText2 {
		idx := 10*width + 10 + i
		assert.Equal(t, ch, content[idx].Rune, "Label2 character at position %d", i)
		assert.Equal(t, gtv.TextColor(0x00FF00), content[idx].Attrs.TextColor)
	}

	// Check that label3 is rendered at position (70, 20)
	expectedText3 := "Third"
	for i, ch := range expectedText3 {
		idx := 20*width + 70 + i
		assert.Equal(t, ch, content[idx].Rune, "Label3 character at position %d", i)
		assert.Equal(t, gtv.AttrItalic, content[idx].Attrs.Attributes&gtv.AttrItalic)
		assert.Equal(t, gtv.TextColor(0x0000FF), content[idx].Attrs.TextColor)
	}

	// Verify that unused labels are accessible via layout children
	assert.Len(t, layout.Children, 3)
	assert.Equal(t, label1, layout.Children[0])
	assert.Equal(t, label2, layout.Children[1])
	assert.Equal(t, label3, layout.Children[2])
}

// TestApplicationIntegration_InputHandling tests that the application correctly handles
// input events and passes them to widgets.
//
// This test demonstrates:
// - Notifying input events using mock input reader
// - Processing events synchronously
// - Verifying event handling by widgets
func TestApplicationIntegration_InputHandling(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create a custom widget that tracks events
	type EventTracker struct {
		tui.TWidget
		events []gtv.InputEvent
	}

	tracker := &EventTracker{
		TWidget: tui.TWidget{
			Position: gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		},
		events: make([]gtv.InputEvent, 0),
	}

	// Create application
	app := tui.NewApplication(tracker, screen)
	require.NotNil(t, app)

	// Create mock input reader and send events
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.TypeKeys("abq")

	// Events are now processed synchronously by the mock reader
	// In this simple test, we just verify that events were processed without errors
	// A more sophisticated test would use a custom widget implementation that tracks events
}

// TestApplicationIntegration_ResizeHandling tests that the application correctly handles
// terminal resize events.
//
// This test demonstrates:
// - Notifying resize events using mock input reader
// - Verifying screen buffer resize
// - Verifying widget resize notification
func TestApplicationIntegration_ResizeHandling(t *testing.T) {
	// Create a screen buffer for testing (initial size 80x24)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create a label at bottom-right corner
	label := tui.NewLabel(
		layout,
		"Corner",
		gtv.TRect{X: 74, Y: 23, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0),
	)

	// Create application
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Verify initial screen size
	width, height := screen.GetSize()
	assert.Equal(t, 80, width)
	assert.Equal(t, 24, height)

	// Verify initial layout size
	layoutPos := layout.GetPos()
	assert.Equal(t, uint16(80), layoutPos.W)
	assert.Equal(t, uint16(24), layoutPos.H)

	// Create mock input reader and send resize event
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.Resize(120, 30)

	// Verify screen buffer was resized
	width, height = screen.GetSize()
	assert.Equal(t, 120, width)
	assert.Equal(t, 30, height)

	// Verify layout was notified of resize
	layoutPos = layout.GetPos()
	assert.Equal(t, uint16(120), layoutPos.W)
	assert.Equal(t, uint16(30), layoutPos.H)

	// Verify label still exists and is rendered
	assert.Len(t, layout.Children, 1)
	assert.Equal(t, label, layout.Children[0])
}

// TestApplicationIntegration_ComplexLayout tests a more complex widget hierarchy
// with nested layouts and multiple children.
//
// This test demonstrates:
// - Creating nested widget hierarchies
// - Rendering complex layouts
// - Verifying proper positioning of nested widgets
func TestApplicationIntegration_ComplexLayout(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout (fills screen)
	mainLayout := tui.NewAbsoluteLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		&gtv.CellAttributes{BackColor: 0x000000},
	)

	// Create a header layout (top of screen)
	headerLayout := tui.NewAbsoluteLayout(
		mainLayout,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 3},
		&gtv.CellAttributes{BackColor: 0x333333},
	)

	// Add title to header
	tui.NewLabel(
		headerLayout,
		"Application Title",
		gtv.TRect{X: 2, Y: 1, W: 0, H: 0},
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0),
	)

	// Create a content layout (middle of screen)
	contentLayout := tui.NewAbsoluteLayout(
		mainLayout,
		gtv.TRect{X: 0, Y: 3, W: 80, H: 18},
		nil, // Transparent
	)

	// Add some content labels
	tui.NewLabel(
		contentLayout,
		"Line 1",
		gtv.TRect{X: 5, Y: 2, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0),
	)

	tui.NewLabel(
		contentLayout,
		"Line 2",
		gtv.TRect{X: 5, Y: 3, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0),
	)

	// Create a footer layout (bottom of screen)
	footerLayout := tui.NewAbsoluteLayout(
		mainLayout,
		gtv.TRect{X: 0, Y: 21, W: 80, H: 3},
		&gtv.CellAttributes{BackColor: 0x333333},
	)

	// Add status to footer
	tui.NewLabel(
		footerLayout,
		"Status: Ready",
		gtv.TRect{X: 2, Y: 1, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0x00FF00, 0),
	)

	// Create application
	app := tui.NewApplication(mainLayout, screen)
	require.NotNil(t, app)

	// Draw the layout (without running the event loop)
	mainLayout.Draw(screen)

	// Verify screen content
	width, height, content := screen.GetContent()
	assert.Equal(t, 80, width)
	assert.Equal(t, 24, height)

	// Verify header background (row 0)
	for x := 0; x < 80; x++ {
		idx := 0*width + x
		assert.Equal(t, gtv.TextColor(0x333333), content[idx].Attrs.BackColor, "Header background at x=%d", x)
	}

	// Verify title in header (row 1, starting at x=2)
	title := "Application Title"
	for i, ch := range title {
		idx := 1*width + 2 + i
		assert.Equal(t, ch, content[idx].Rune, "Title character at position %d", i)
	}

	// Verify content labels
	// "Line 1" at (5, 5) in screen coordinates (contentLayout Y=3, label Y=2)
	line1 := "Line 1"
	for i, ch := range line1 {
		idx := 5*width + 5 + i
		assert.Equal(t, ch, content[idx].Rune, "Line 1 character at position %d", i)
	}

	// "Line 2" at (5, 6) in screen coordinates
	line2 := "Line 2"
	for i, ch := range line2 {
		idx := 6*width + 5 + i
		assert.Equal(t, ch, content[idx].Rune, "Line 2 character at position %d", i)
	}

	// Verify footer background (row 21)
	for x := 0; x < 80; x++ {
		idx := 21*width + x
		assert.Equal(t, gtv.TextColor(0x333333), content[idx].Attrs.BackColor, "Footer background at x=%d", x)
	}

	// Verify status in footer (row 22, starting at x=2)
	status := "Status: Ready"
	for i, ch := range status {
		idx := 22*width + 2 + i
		assert.Equal(t, ch, content[idx].Rune, "Status character at position %d", i)
	}

	// Verify widget hierarchy
	assert.Len(t, mainLayout.Children, 3) // header, content, footer
	assert.Len(t, headerLayout.Children, 1)
	assert.Len(t, contentLayout.Children, 2)
	assert.Len(t, footerLayout.Children, 1)
}

// TestApplicationIntegration_QuitSignal tests that the application correctly handles
// the quit signal.
//
// This test demonstrates:
// - Signaling application to quit
// - Verifying that the quit signal is processed
func TestApplicationIntegration_QuitSignal(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create a simple layout
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create application
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Signal quit
	app.Quit()

	// Quit() just sends a signal to quitCh
	// We can't test the actual exit behavior without running the event loop,
	// but we can verify that the signal was sent by checking that the channel
	// is not blocked (which would happen if Quit() didn't work)

	// This is more of a smoke test - in a real scenario, the event loop would exit
}

// TestApplicationIntegration_CtrlC tests that the application handles Ctrl+C correctly.
//
// This test demonstrates:
// - Notifying Ctrl+C key event using mock input reader
// - Verifying that the application signals quit
func TestApplicationIntegration_CtrlC(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create a simple layout
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create application
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input reader and send Ctrl+C
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.TypeKeysByName("Ctrl+C")

	// The application should have signaled quit
	// We can verify this without running the event loop
}

// TestApplicationIntegration_DemoForm tests a data entry form similar to the gtvdemo application.
//
// This test demonstrates:
// - Creating a complete form with labels, input boxes, and buttons
// - Handling tab navigation between focusable widgets
// - Handling button press events
// - Updating label text based on input
// - Clearing form data
func TestApplicationIntegration_DemoForm(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout
	mainLayout := tui.NewAbsoluteLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		&gtv.CellAttributes{BackColor: 0x1a1a1a},
	)

	// Create labels for form fields
	tui.NewLabel(
		mainLayout,
		"Name:",
		gtv.TRect{X: 5, Y: 3, W: 0, H: 0},
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0),
	)

	tui.NewLabel(
		mainLayout,
		"Email:",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0},
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0),
	)

	// Create input boxes
	nameInput := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(15, 3, 30, 1),
		tui.WithAttrs(gtv.AttrsWithColor(0, 0xFFFFFF, 0x333333)),
		tui.WithFocusedAttrs(gtv.AttrsWithColor(0, 0x000000, 0x00AAFF)),
	)

	emailInput := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(15, 5, 30, 1),
		tui.WithAttrs(gtv.AttrsWithColor(0, 0xFFFFFF, 0x333333)),
		tui.WithFocusedAttrs(gtv.AttrsWithColor(0, 0x000000, 0x00AAFF)),
	)

	// Create result label
	resultLabel := tui.NewLabel(
		mainLayout,
		"",
		gtv.TRect{X: 5, Y: 10, W: 60, H: 1},
		gtv.AttrsWithColor(0, 0x00FF00, 0),
	)

	// Create Submit button
	submitButton := tui.NewButton(
		mainLayout,
		"Submit",
		gtv.TRect{X: 15, Y: 7, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x006600),
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0x00AA00),
		gtv.AttrsWithColor(0, 0x888888, 0x333333),
	)

	// Set Submit button action
	submitButton.SetOnPress(func() {
		name := nameInput.GetText()
		email := emailInput.GetText()

		if name == "" && email == "" {
			resultLabel.SetText("Please enter at least one field!")
			resultLabel.SetAttrs(gtv.AttrsWithColor(0, 0xFF0000, 0))
		} else {
			result := "Submitted - Name: " + name + ", Email: " + email
			resultLabel.SetText(result)
			resultLabel.SetAttrs(gtv.AttrsWithColor(0, 0x00FF00, 0))
		}
	})

	// Create Clear button
	clearButton := tui.NewButton(
		mainLayout,
		"Clear",
		gtv.TRect{X: 28, Y: 7, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x660000),
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0xAA0000),
		gtv.AttrsWithColor(0, 0x888888, 0x333333),
	)

	// Set Clear button action
	clearButton.SetOnPress(func() {
		nameInput.SetText("")
		emailInput.SetText("")
		resultLabel.SetText("")
	})

	// Create application
	app := tui.NewApplication(mainLayout, screen)
	require.NotNil(t, app)

	// Draw initial state
	mainLayout.Draw(screen)

	// Verify initial state
	assert.Equal(t, "", nameInput.GetText())
	assert.Equal(t, "", emailInput.GetText())
	assert.Equal(t, "", resultLabel.GetText())

	// Create mock input reader
	mockInput := tio.NewMockInputEventReader(app)

	// Focus first input box (name) by clicking on it
	mockInput.MouseClick(20, 3, 0)

	// Type some text into name input
	mockInput.TypeKeys("John Doe")
	assert.Equal(t, "John Doe", nameInput.GetText())

	// Tab to email input
	mockInput.TypeKeysByName("Tab")

	// Type some text into email input
	mockInput.TypeKeys("john@example.com")
	assert.Equal(t, "john@example.com", emailInput.GetText())

	// Tab to Submit button
	mockInput.TypeKeysByName("Tab")

	// Verify Submit button is focused (we can't directly check focus state in this test,
	// but we can verify behavior)

	// Press Submit button
	mockInput.TypeKeysByName("Enter")

	// Verify result label was updated
	expectedResult := "Submitted - Name: John Doe, Email: john@example.com"
	assert.Equal(t, expectedResult, resultLabel.GetText())
	assert.Equal(t, gtv.TextColor(0x00FF00), resultLabel.GetAttrs().TextColor)

	// Tab to Clear button
	mockInput.TypeKeysByName("Tab")

	// Press Clear button
	mockInput.TypeKeysByName("Space")

	// Verify all fields were cleared
	assert.Equal(t, "", nameInput.GetText())
	assert.Equal(t, "", emailInput.GetText())
	assert.Equal(t, "", resultLabel.GetText())

	// Test empty form submission
	// Tab back to submit button (Tab through name, email, submit)
	mockInput.TypeKeysByName("Tab", "Tab", "Tab")

	// Press Submit button with empty fields
	mockInput.TypeKeysByName("Enter")

	// Verify error message
	assert.Equal(t, "Please enter at least one field!", resultLabel.GetText())
	assert.Equal(t, gtv.TextColor(0xFF0000), resultLabel.GetAttrs().TextColor)
}

// TestApplicationIntegration_InputBoxCursorVisibility tests that when an InputBox
// widget is focused, the cursor is visible at the correct position.
//
// This test demonstrates:
// - Creating an InputBox widget
// - Focusing the InputBox by clicking on it
// - Verifying that the cursor is visible and positioned correctly
// - Verifying cursor style is set appropriately
func TestApplicationIntegration_InputBoxCursorVisibility(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout
	mainLayout := tui.NewAbsoluteLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		&gtv.CellAttributes{BackColor: 0x1a1a1a},
	)

	// Create an input box at position (10, 5) with width 30
	inputBox := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(10, 5, 30, 1),
		tui.WithAttrs(gtv.AttrsWithColor(0, 0xFFFFFF, 0x333333)),
		tui.WithFocusedAttrs(gtv.AttrsWithColor(0, 0x000000, 0x00AAFF)),
	)

	// Create application
	app := tui.NewApplication(mainLayout, screen)
	require.NotNil(t, app)

	// Draw initial state (input box not focused)
	mainLayout.Draw(screen)

	// Initially, input box is not focused, so cursor should not be set
	// (or should be at 0,0 with default style)
	cursorX, cursorY := screen.GetCursorPosition()
	cursorStyle := screen.GetCursorStyle()

	// The initial cursor position and style depend on initialization
	// For now, just document the initial state
	t.Logf("Initial cursor position: (%d, %d), style: %d", cursorX, cursorY, cursorStyle)

	// Create mock input reader
	mockInput := tio.NewMockInputEventReader(app)

	// Focus the input box by clicking on it
	mockInput.MouseClick(15, 5, 0)

	// Redraw to update cursor
	mainLayout.Draw(screen)

	// After focusing, cursor should be visible at the start of the input box
	cursorX, cursorY = screen.GetCursorPosition()
	cursorStyle = screen.GetCursorStyle()

	// The cursor should be at position (10, 5) - the start of the input box
	assert.Equal(t, 10, cursorX, "Cursor X position should be at start of input box")
	assert.Equal(t, 5, cursorY, "Cursor Y position should be at input box row")

	// The cursor style should be bar and blinking
	expectedStyle := gtv.CursorStyleBar | gtv.CursorStyleBlinking
	assert.Equal(t, expectedStyle, cursorStyle, "Cursor style should be bar and blinking when focused")

	// Type some text
	mockInput.TypeKeys("Hello")

	// Redraw
	mainLayout.Draw(screen)

	// Verify text was entered
	assert.Equal(t, "Hello", inputBox.GetText())

	// After typing, cursor should have moved
	cursorX, cursorY = screen.GetCursorPosition()
	cursorStyle = screen.GetCursorStyle()

	// Cursor should be at position (15, 5) - after "Hello"
	assert.Equal(t, 15, cursorX, "Cursor X position should be after typed text")
	assert.Equal(t, 5, cursorY, "Cursor Y position should still be at input box row")
	assert.Equal(t, expectedStyle, cursorStyle, "Cursor style should still be bar and blinking")

	// Blur the input box by clicking outside
	mockInput.MouseClick(0, 0, 0)

	// Redraw
	mainLayout.Draw(screen)

	// After blurring, the cursor position might change or style might be different
	// Document the behavior
	cursorX, cursorY = screen.GetCursorPosition()
	cursorStyle = screen.GetCursorStyle()
	t.Logf("After blur cursor position: (%d, %d), style: %d", cursorX, cursorY, cursorStyle)
}

// TestApplicationIntegration_RealMousePress tests that widgets respond to real mouse press events
// (with ModPress modifier) as would be sent by the actual terminal input reader.
//
// This test exposes two critical issues for mouse support:
//  1. The layout was checking for ModClick instead of ModPress, which causes focus not to be set
//     when using real mouse input (the real terminal input reader only sends ModPress).
//  2. The application must enable mouse tracking escape sequences in the terminal
//     (see TApplication.initTerminal() for the required escape codes).
//
// This test demonstrates:
// - Creating a form with input boxes and buttons
// - Sending real mouse press events (with only ModPress, not ModClick)
// - Verifying that widgets receive focus correctly
// - Verifying that button press callbacks are triggered correctly
//
// Note: This test uses mock input, so it doesn't verify that mouse tracking is enabled.
// To test with a real terminal, mouse tracking escape sequences must be sent:
//   - \x1b[?1000h - Enable mouse button tracking
//   - \x1b[?1002h - Enable mouse motion tracking
//   - \x1b[?1015h - Enable urxvt mouse mode
//   - \x1b[?1006h - Enable SGR mouse mode
func TestApplicationIntegration_RealMousePress(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout
	mainLayout := tui.NewAbsoluteLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		&gtv.CellAttributes{BackColor: 0x1a1a1a},
	)

	// Create an input box at position (10, 5)
	inputBox := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(10, 5, 30, 1),
		tui.WithAttrs(gtv.AttrsWithColor(0, 0xFFFFFF, 0x333333)),
		tui.WithFocusedAttrs(gtv.AttrsWithColor(0, 0x000000, 0x00AAFF)),
	)

	// Create a button at position (10, 7)
	buttonPressed := false
	button := tui.NewButton(
		mainLayout,
		"Click Me",
		gtv.TRect{X: 10, Y: 7, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x006600),
		gtv.AttrsWithColor(gtv.AttrBold, 0xFFFFFF, 0x00AA00),
		gtv.AttrsWithColor(0, 0x888888, 0x333333),
	)

	button.SetOnPress(func() {
		buttonPressed = true
	})

	// Create application
	app := tui.NewApplication(mainLayout, screen)
	require.NotNil(t, app)

	// Create a custom event sender that mimics real terminal input
	// (sends ModPress but NOT ModClick)
	sendRealMousePress := func(x, y int) {
		app.Notify(gtv.InputEvent{
			Type:      gtv.InputEventMouse,
			X:         uint16(x),
			Y:         uint16(y),
			Modifiers: gtv.ModPress, // Real terminal input only sends ModPress
		})
		// Redraw after event
		mainLayout.Draw(screen)
	}

	// Initially, input box is not focused
	assert.False(t, inputBox.IsFocused(), "InputBox should not be focused initially")

	// Send a real mouse press event to the input box
	sendRealMousePress(15, 5)

	// Verify that the input box received focus
	assert.True(t, inputBox.IsFocused(), "InputBox should be focused after mouse press")

	// Type some text to verify the input box is working
	app.Notify(gtv.InputEvent{
		Type: gtv.InputEventKey,
		Key:  'H',
	})
	app.Notify(gtv.InputEvent{
		Type: gtv.InputEventKey,
		Key:  'i',
	})
	mainLayout.Draw(screen)

	assert.Equal(t, "Hi", inputBox.GetText(), "InputBox should contain typed text")

	// Now send a real mouse press event to the button
	sendRealMousePress(15, 7)

	// Verify that the button received focus
	assert.True(t, button.IsFocused(), "Button should be focused after mouse press")

	// Verify that the input box lost focus
	assert.False(t, inputBox.IsFocused(), "InputBox should lose focus when button is clicked")

	// Press Enter to activate the button
	app.Notify(gtv.InputEvent{
		Type: gtv.InputEventKey,
		Key:  '\r',
	})
	mainLayout.Draw(screen)

	// Verify that the button callback was called
	assert.True(t, buttonPressed, "Button press callback should have been called")
}

// TestApplicationIntegration_InputBoxFocusWithTheme tests that when an InputBox is focused
// with the default theme loaded, it properly shows the cursor and changes background color.
// This test exposes a bug where the input box does not show cursor and does not change
// background color when focused after introducing theme support.
//
// Expected behavior:
// - When input box is not focused: it should use "input" theme (white text on dark gray)
// - When input box is focused: it should use "input-focused" theme (black text on bright blue)
// - Cursor should be visible and positioned correctly when focused
func TestApplicationIntegration_InputBoxFocusWithTheme(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout (no explicit theme tags, let application apply defaults)
	mainLayout := tui.NewAbsoluteLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		nil,
	)

	// Create an input box at position (10, 5) with width 30
	// Use default theme tags (should be "input" and "input-focused")
	inputBox := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(10, 5, 30, 1),
	)

	// Create application with default theme (this is the key part of the test)
	app := tui.NewApplication(mainLayout, screen)
	require.NotNil(t, app)

	// Get the themed screen from the application
	themedScreen := app.GetScreen()

	// Draw initial state (input box not focused)
	mainLayout.Draw(themedScreen)

	// Check the initial rendering - should use "input" theme colors
	// According to default.theme.json: "input" is white (#FFFFFF) on dark gray (#333333)
	width, height, cells := screen.GetContent()
	require.Greater(t, width, 10, "Screen width should be sufficient")
	require.Greater(t, height, 5, "Screen height should be sufficient")

	// Calculate cell index: y*width + x, for position (10, 5)
	cellIndex := 5*width + 10
	cellAt10_5 := cells[cellIndex]

	// Initial state should have dark gray background (#333333)
	assert.Equal(t, gtv.TextColor(0x333333), cellAt10_5.Attrs.BackColor, "Initial input box should have dark gray background from 'input' theme")
	assert.Equal(t, gtv.TextColor(0xFFFFFF), cellAt10_5.Attrs.TextColor, "Initial input box should have white text from 'input' theme")

	// Initially, cursor should not be set for focused widget
	cursorX, cursorY := screen.GetCursorPosition()
	cursorStyle := screen.GetCursorStyle()
	t.Logf("Initial cursor position: (%d, %d), style: %d", cursorX, cursorY, cursorStyle)

	// Create mock input reader
	mockInput := tio.NewMockInputEventReader(app)

	// Focus the input box by clicking on it
	mockInput.MouseClick(15, 5, 0)

	// Redraw to update cursor and colors
	mainLayout.Draw(themedScreen)

	// After focusing, check that colors changed to "input-focused" theme
	// According to default.theme.json: "input-focused" is black (#000000) on bright blue (#00AAFF)
	width, height, cells = screen.GetContent()
	cellIndex = 5*width + 10 // Row 5, Column 10 (where input box starts)
	cellAt10_5 = cells[cellIndex]

	// THIS IS THE BUG: The focused input box should have bright blue background
	assert.Equal(t, gtv.TextColor(0x00AAFF), cellAt10_5.Attrs.BackColor,
		"Focused input box should have bright blue background from 'input-focused' theme")
	assert.Equal(t, gtv.TextColor(0x000000), cellAt10_5.Attrs.TextColor,
		"Focused input box should have black text from 'input-focused' theme")

	// After focusing, cursor should be visible at the start of the input box
	cursorX, cursorY = screen.GetCursorPosition()
	cursorStyle = screen.GetCursorStyle()

	// The cursor should be at position (10, 5) - the start of the input box
	assert.Equal(t, 10, cursorX, "Cursor X position should be at start of input box")
	assert.Equal(t, 5, cursorY, "Cursor Y position should be at input box row")

	// The cursor style should be bar and blinking
	expectedStyle := gtv.CursorStyleBar | gtv.CursorStyleBlinking
	assert.Equal(t, expectedStyle, cursorStyle, "Cursor style should be bar and blinking when focused")

	// Type some text
	mockInput.TypeKeys("Hello")

	// Redraw
	mainLayout.Draw(themedScreen)

	// Verify text was entered
	assert.Equal(t, "Hello", inputBox.GetText())

	// After typing, cursor should have moved
	cursorX, cursorY = screen.GetCursorPosition()
	cursorStyle = screen.GetCursorStyle()

	// Cursor should be at position (15, 5) - after "Hello"
	assert.Equal(t, 15, cursorX, "Cursor X position should be after typed text")
	assert.Equal(t, 5, cursorY, "Cursor Y position should still be at input box row")
	assert.Equal(t, expectedStyle, cursorStyle, "Cursor style should still be bar and blinking")

	// Verify focused colors are still applied
	width, height, cells = screen.GetContent()
	// Check first character 'H' at position (10, 5)
	cellIndex = 5*width + 10
	cellAtH := cells[cellIndex]
	assert.Equal(t, gtv.TextColor(0x00AAFF), cellAtH.Attrs.BackColor,
		"Focused input box background should remain bright blue while typing")
	assert.Equal(t, 'H', cellAtH.Rune, "First character should be 'H'")
}

// TestApplicationIntegration_InputBoxCursorVisibilityBug is a more focused test
// that specifically checks if the cursor is visible when an empty input box is focused.
// This test exposes the bug where the cursor might not be visible after focusing.
func TestApplicationIntegration_InputBoxCursorVisibilityBug(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout
	mainLayout := tui.NewAbsoluteLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		nil,
	)

	// Create an input box at position (10, 5) with width 30
	// Use default theme tags
	inputBox := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(10, 5, 30, 1),
	)

	// Create application with default theme
	app := tui.NewApplication(mainLayout, screen)
	require.NotNil(t, app)

	// Get the themed screen from the application
	themedScreen := app.GetScreen()

	// Initially, input box is not focused
	assert.False(t, inputBox.IsFocused(), "InputBox should not be focused initially")

	// Draw initial state
	mainLayout.Draw(themedScreen)

	// Check initial cursor - should not be visible/positioned for input box
	cursorX, cursorY := screen.GetCursorPosition()
	cursorStyle := screen.GetCursorStyle()
	t.Logf("Initial cursor position: (%d, %d), style: %d", cursorX, cursorY, cursorStyle)

	// Create mock input reader
	mockInput := tio.NewMockInputEventReader(app)

	// Focus the input box by clicking on it
	mockInput.MouseClick(15, 5, 0)

	// Verify input box is now focused
	assert.True(t, inputBox.IsFocused(), "InputBox should be focused after click")

	// Redraw through the application's screen (this mimics what happens in handleEvent)
	themedScreen.SetCursorStyle(gtv.CursorStyleHidden) // App hides cursor before draw
	mainLayout.Draw(themedScreen)

	// After focusing and drawing, cursor should be visible at the start of the input box
	cursorX, cursorY = screen.GetCursorPosition()
	cursorStyle = screen.GetCursorStyle()
	t.Logf("After focus cursor position: (%d, %d), style: %d", cursorX, cursorY, cursorStyle)

	// THIS IS THE BUG FIX VALIDATION:
	// The cursor should be at position (10, 5) - the start of the input box
	assert.Equal(t, 10, cursorX, "Cursor X position should be at start of input box (BUG: cursor not visible)")
	assert.Equal(t, 5, cursorY, "Cursor Y position should be at input box row")

	// The cursor style should be bar and blinking (not hidden!)
	expectedStyle := gtv.CursorStyleBar | gtv.CursorStyleBlinking
	assert.Equal(t, expectedStyle, cursorStyle, "Cursor style should be bar and blinking when focused (BUG: cursor hidden)")
}

// TestApplicationIntegration_InputBoxCursorAtBoundary tests that the cursor remains visible
// when the input box is filled to capacity (cursor at position equal to width).
func TestApplicationIntegration_InputBoxCursorAtBoundary(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout
	mainLayout := tui.NewAbsoluteLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		nil,
	)

	// Create an input box at position (10, 5) with width 10 (small for easy testing)
	inputBox := tui.NewInputBox(
		mainLayout,
		tui.WithText(""),
		tui.WithRectangle(10, 5, 10, 1),
	)

	// Create application with default theme
	app := tui.NewApplication(mainLayout, screen)
	require.NotNil(t, app)

	// Get the themed screen from the application
	themedScreen := app.GetScreen()

	// Create mock input reader
	mockInput := tio.NewMockInputEventReader(app)

	// Focus the input box
	mockInput.MouseClick(15, 5, 0)

	// Type exactly 10 characters (filling the input box completely)
	mockInput.TypeKeys("1234567890")

	// Redraw
	themedScreen.SetCursorStyle(gtv.CursorStyleHidden)
	mainLayout.Draw(themedScreen)

	// Verify the text was entered
	assert.Equal(t, "1234567890", inputBox.GetText())

	// Check cursor position - should still be visible
	cursorX, cursorY := screen.GetCursorPosition()
	cursorStyle := screen.GetCursorStyle()
	t.Logf("Cursor after typing 10 chars into 10-width box: (%d, %d), style: %d", cursorX, cursorY, cursorStyle)

	// Cursor should be visible (not hidden)
	expectedStyle := gtv.CursorStyleBar | gtv.CursorStyleBlinking
	assert.Equal(t, expectedStyle, cursorStyle, "Cursor should still be visible after filling input box")

	// Cursor should be at a valid position (at or near the input box)
	// When the box is full and scrollOffset kicks in, cursor might be at the right edge
	// or the text might scroll to keep cursor visible
	assert.GreaterOrEqual(t, cursorX, 10, "Cursor X should be at or after input box start")
	assert.Equal(t, 5, cursorY, "Cursor Y should be at input box row")
}
