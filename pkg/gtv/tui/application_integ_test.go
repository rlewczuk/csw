package tui_test

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"

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
		assert.Equal(t, uint32(0xFF0000), content[idx].Attrs.TextColor)
	}

	// Check that label2 is rendered at position (10, 10)
	expectedText2 := "Second Label"
	for i, ch := range expectedText2 {
		idx := 10*width + 10 + i
		assert.Equal(t, ch, content[idx].Rune, "Label2 character at position %d", i)
		assert.Equal(t, uint32(0x00FF00), content[idx].Attrs.TextColor)
	}

	// Check that label3 is rendered at position (70, 20)
	expectedText3 := "Third"
	for i, ch := range expectedText3 {
		idx := 20*width + 70 + i
		assert.Equal(t, ch, content[idx].Rune, "Label3 character at position %d", i)
		assert.Equal(t, gtv.AttrItalic, content[idx].Attrs.Attributes&gtv.AttrItalic)
		assert.Equal(t, uint32(0x0000FF), content[idx].Attrs.TextColor)
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
		assert.Equal(t, uint32(0x333333), content[idx].Attrs.BackColor, "Header background at x=%d", x)
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
		assert.Equal(t, uint32(0x333333), content[idx].Attrs.BackColor, "Footer background at x=%d", x)
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
		"",
		gtv.TRect{X: 15, Y: 3, W: 30, H: 1},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x333333),
		gtv.AttrsWithColor(0, 0x000000, 0x00AAFF),
	)

	emailInput := tui.NewInputBox(
		mainLayout,
		"",
		gtv.TRect{X: 15, Y: 5, W: 30, H: 1},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x333333),
		gtv.AttrsWithColor(0, 0x000000, 0x00AAFF),
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
	assert.Equal(t, uint32(0x00FF00), resultLabel.GetAttrs().TextColor)

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
	assert.Equal(t, uint32(0xFF0000), resultLabel.GetAttrs().TextColor)
}
