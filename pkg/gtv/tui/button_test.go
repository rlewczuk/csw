package tui_test

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestButton_BasicRendering tests that a button is rendered correctly in normal state.
func TestButton_BasicRendering(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create button with specific text and position
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)   // White on blue
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)  // Black on yellow
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000) // Gray on black

	button := tui.NewButton(
		nil,
		"OK",
		gtv.TRect{X: 10, Y: 10, W: 0, H: 0}, // Auto-sized
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	require.NotNil(t, button)

	// Draw the button
	button.Draw(screen)

	// Verify button text is rendered
	// Expected: [ OK ]
	width, height, content := screen.GetContent()
	assert.Equal(t, 80, width)
	assert.Equal(t, 24, height)

	// Check button position and size
	pos := button.GetPos()
	assert.Equal(t, uint16(10), pos.X)
	assert.Equal(t, uint16(10), pos.Y)
	assert.Equal(t, uint16(6), pos.W) // [ OK ] = 6 characters
	assert.Equal(t, uint16(1), pos.H)

	// Check button content at position (10, 10)
	expectedText := "[ OK ]"
	for i, ch := range expectedText {
		idx := 10*width + 10 + i
		assert.Equal(t, ch, content[idx].Rune, "Character at position %d", i)
		assert.Equal(t, uint32(0xFFFFFF), content[idx].Attrs.TextColor)
		assert.Equal(t, uint32(0x0000FF), content[idx].Attrs.BackColor)
	}
}

// TestButton_FocusedRendering tests that a button is rendered correctly when focused.
func TestButton_FocusedRendering(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)   // White on blue
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)  // Black on yellow
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000) // Gray on black

	button := tui.NewButton(
		nil,
		"Submit",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0}, // Auto-sized
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Focus the button
	button.Focus()
	assert.True(t, button.IsFocused())

	// Draw the button
	button.Draw(screen)

	// Verify button is rendered with focused attributes
	width, _, content := screen.GetContent()

	expectedText := "[ Submit ]"
	for i, ch := range expectedText {
		idx := 5*width + 5 + i
		assert.Equal(t, ch, content[idx].Rune, "Character at position %d", i)
		assert.Equal(t, uint32(0x000000), content[idx].Attrs.TextColor)
		assert.Equal(t, uint32(0xFFFF00), content[idx].Attrs.BackColor)
	}
}

// TestButton_DisabledRendering tests that a button is rendered correctly when disabled.
func TestButton_DisabledRendering(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)   // White on blue
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)  // Black on yellow
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000) // Gray on black

	button := tui.NewButton(
		nil,
		"Cancel",
		gtv.TRect{X: 15, Y: 15, W: 0, H: 0}, // Auto-sized
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Disable the button
	button.SetEnabled(false)
	assert.False(t, button.IsEnabled())

	// Draw the button
	button.Draw(screen)

	// Verify button is rendered with disabled attributes
	width, _, content := screen.GetContent()

	expectedText := "[ Cancel ]"
	for i, ch := range expectedText {
		idx := 15*width + 15 + i
		assert.Equal(t, ch, content[idx].Rune, "Character at position %d", i)
		assert.Equal(t, uint32(0x808080), content[idx].Attrs.TextColor)
		assert.Equal(t, uint32(0x000000), content[idx].Attrs.BackColor)
	}
}

// TestButton_EnableDisable tests that enabling and disabling a button works correctly.
func TestButton_EnableDisable(t *testing.T) {
	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		nil,
		"Test",
		gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Initially enabled
	assert.True(t, button.IsEnabled())

	// Disable
	button.SetEnabled(false)
	assert.False(t, button.IsEnabled())

	// Re-enable
	button.SetEnabled(true)
	assert.True(t, button.IsEnabled())
}

// TestButton_TextGetterSetter tests getting and setting button text.
func TestButton_TextGetterSetter(t *testing.T) {
	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		nil,
		"Initial",
		gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Get initial text
	assert.Equal(t, "Initial", button.GetText())

	// Set new text
	button.SetText("Updated")
	assert.Equal(t, "Updated", button.GetText())

	// Verify size was updated
	pos := button.GetPos()
	assert.Equal(t, uint16(11), pos.W) // [ Updated ] = 11 characters
}

// TestButton_PressCallback tests that the press callback is called when the button is activated.
func TestButton_PressCallback(t *testing.T) {
	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		nil,
		"Click Me",
		gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Set up press callback
	pressed := false
	button.SetOnPress(func() {
		pressed = true
	})

	// Press the button directly
	button.Press()
	assert.True(t, pressed)
}

// TestButton_PressCallbackNotCalledWhenDisabled tests that the press callback is not called when disabled.
func TestButton_PressCallbackNotCalledWhenDisabled(t *testing.T) {
	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		nil,
		"Click Me",
		gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Set up press callback
	pressed := false
	button.SetOnPress(func() {
		pressed = true
	})

	// Disable the button
	button.SetEnabled(false)

	// Press the button directly
	button.Press()
	assert.False(t, pressed, "Callback should not be called when button is disabled")
}

// TestButton_EnterKeyPress tests that pressing Enter triggers the button.
func TestButton_EnterKeyPress(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		nil,
		"Press Enter",
		gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Set up press callback
	pressed := false
	button.SetOnPress(func() {
		pressed = true
	})

	// Focus the button
	button.Focus()

	// Create application with the button
	app := tui.NewApplication(button, screen)
	require.NotNil(t, app)

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Press Enter key
	mockInput.TypeKeysByName("Enter")

	assert.True(t, pressed, "Button should be pressed when Enter is pressed")
}

// TestButton_SpaceKeyPress tests that pressing Space triggers the button.
func TestButton_SpaceKeyPress(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		nil,
		"Press Space",
		gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Set up press callback
	pressed := false
	button.SetOnPress(func() {
		pressed = true
	})

	// Focus the button
	button.Focus()

	// Create application with the button
	app := tui.NewApplication(button, screen)
	require.NotNil(t, app)

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Press Space key
	mockInput.TypeKeysByName("Space")

	assert.True(t, pressed, "Button should be pressed when Space is pressed")
}

// TestButton_NotPressedWhenNotFocused tests that the button is not triggered when not focused.
func TestButton_NotPressedWhenNotFocused(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		nil,
		"Not Focused",
		gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Set up press callback
	pressed := false
	button.SetOnPress(func() {
		pressed = true
	})

	// Don't focus the button (it's not focused by default)
	assert.False(t, button.IsFocused())

	// Create application with the button
	app := tui.NewApplication(button, screen)
	require.NotNil(t, app)

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Press Enter key
	mockInput.TypeKeysByName("Enter")

	assert.False(t, pressed, "Button should not be pressed when not focused")
}

// TestButton_NotPressedWhenDisabledViaEvent tests that the button is not triggered when disabled.
func TestButton_NotPressedWhenDisabledViaEvent(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		nil,
		"Disabled",
		gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Set up press callback
	pressed := false
	button.SetOnPress(func() {
		pressed = true
	})

	// Focus and disable the button
	button.Focus()
	button.SetEnabled(false)

	// Create application with the button
	app := tui.NewApplication(button, screen)
	require.NotNil(t, app)

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Press Enter key
	mockInput.TypeKeysByName("Enter")

	assert.False(t, pressed, "Button should not be pressed when disabled")
}

// TestButton_WithinLayout tests that a button works correctly when placed in a layout.
func TestButton_WithinLayout(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create layout
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create button within layout
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		layout,
		"Button in Layout",
		gtv.TRect{X: 20, Y: 10, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Set up press callback
	pressed := false
	button.SetOnPress(func() {
		pressed = true
	})

	// Focus the button
	button.Focus()

	// Draw the layout (which will draw the button)
	layout.Draw(screen)

	// Verify button is rendered at correct position
	width, _, content := screen.GetContent()

	expectedText := "[ Button in Layout ]"
	for i, ch := range expectedText {
		idx := 10*width + 20 + i
		assert.Equal(t, ch, content[idx].Rune, "Character at position %d", i)
		// Should use focused attributes since button is focused
		assert.Equal(t, uint32(0x000000), content[idx].Attrs.TextColor)
		assert.Equal(t, uint32(0xFFFF00), content[idx].Attrs.BackColor)
	}

	// Test button press through application
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Set button as active child of layout
	layout.ActiveChild = button

	// Create mock input
	mockInput := tio.NewMockInputEventReader(app)

	// Press Enter key
	mockInput.TypeKeysByName("Enter")

	assert.True(t, pressed, "Button should be pressed when Enter is pressed in layout")
}

// TestButton_FocusBlur tests focus and blur functionality.
func TestButton_FocusBlur(t *testing.T) {
	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		nil,
		"Focus Test",
		gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Initially not focused
	assert.False(t, button.IsFocused())

	// Focus
	button.Focus()
	assert.True(t, button.IsFocused())

	// Blur
	button.Blur()
	assert.False(t, button.IsFocused())
}

// TestButton_AttributeGettersSetters tests attribute getters and setters.
func TestButton_AttributeGettersSetters(t *testing.T) {
	// Create button
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x0000FF)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFF00)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button := tui.NewButton(
		nil,
		"Test",
		gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)

	// Get initial attributes
	assert.Equal(t, normalAttrs, button.GetAttrs())
	assert.Equal(t, focusedAttrs, button.GetFocusedAttrs())

	// Set new attributes
	newNormalAttrs := gtv.AttrsWithColor(gtv.AttrBold, 0xFF0000, 0x00FF00)
	newFocusedAttrs := gtv.AttrsWithColor(gtv.AttrItalic, 0x00FF00, 0xFF0000)

	button.SetAttrs(newNormalAttrs)
	button.SetFocusedAttrs(newFocusedAttrs)

	// Verify new attributes
	assert.Equal(t, newNormalAttrs, button.GetAttrs())
	assert.Equal(t, newFocusedAttrs, button.GetFocusedAttrs())
}
