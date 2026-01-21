package tui_test

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFocusIntegration_MouseClickBetweenInputBoxAndButton tests the complete
// focus handling flow when clicking between input boxes and buttons.
// This is a regression test for the bug where:
// - clicking on input box shows visual focus but keyboard events go to previously focused widget
// - clicking on buttons doesn't work at all
func TestFocusIntegration_MouseClickBetweenInputBoxAndButton(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	// Create two input boxes and one button
	inputBox1 := tui.NewInputBox(
		layout,
		tui.WithText(""),
		tui.WithRectangle(10, 5, 30, 1),
		tui.WithAttrs(normalAttrs),
		tui.WithFocusedAttrs(focusedAttrs,
	),
	)

	inputBox2 := tui.NewInputBox(
		layout,
		tui.WithText(""),
		tui.WithRectangle(10, 10, 30, 1),
		tui.WithAttrs(normalAttrs),
		tui.WithFocusedAttrs(focusedAttrs,
	),
	)

	buttonPressed := false
	button := tui.NewButton(
		layout,
		"Submit",
		gtv.TRect{X: 10, Y: 15, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)
	button.SetOnPress(func() {
		buttonPressed = true
	})

	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)
	mockInput := tio.NewMockInputEventReader(app)

	// Initially nothing is focused
	assert.False(t, inputBox1.IsFocused())
	assert.False(t, inputBox2.IsFocused())
	assert.False(t, button.IsFocused())
	assert.Nil(t, layout.ActiveChild)

	// Click on first input box
	t.Log("Step 1: Click on inputBox1")
	mockInput.MouseClick(20, 5, 0)

	assert.True(t, inputBox1.IsFocused(), "inputBox1 should be focused")
	assert.False(t, inputBox2.IsFocused(), "inputBox2 should not be focused")
	assert.False(t, button.IsFocused(), "button should not be focused")
	assert.Equal(t, inputBox1, layout.ActiveChild, "ActiveChild should be inputBox1")

	// Type in first input box
	t.Log("Step 2: Type 'Hello' in inputBox1")
	mockInput.TypeKeys("Hello")
	assert.Equal(t, "Hello", inputBox1.GetText(), "inputBox1 should contain 'Hello'")
	assert.Equal(t, "", inputBox2.GetText(), "inputBox2 should be empty")

	// Click on second input box
	t.Log("Step 3: Click on inputBox2")
	mockInput.MouseClick(20, 10, 0)

	assert.False(t, inputBox1.IsFocused(), "inputBox1 should lose focus")
	assert.True(t, inputBox2.IsFocused(), "inputBox2 should be focused")
	assert.False(t, button.IsFocused(), "button should not be focused")
	assert.Equal(t, inputBox2, layout.ActiveChild, "ActiveChild should be inputBox2")

	// Type in second input box
	t.Log("Step 4: Type 'World' in inputBox2")
	mockInput.TypeKeys("World")
	assert.Equal(t, "Hello", inputBox1.GetText(), "inputBox1 should still contain 'Hello'")
	assert.Equal(t, "World", inputBox2.GetText(), "inputBox2 should contain 'World'")

	// Click on button
	t.Log("Step 5: Click on button")
	mockInput.MouseClick(12, 15, 0)

	assert.False(t, inputBox1.IsFocused(), "inputBox1 should not be focused")
	assert.False(t, inputBox2.IsFocused(), "inputBox2 should lose focus")
	assert.True(t, button.IsFocused(), "button should be focused")
	assert.Equal(t, button, layout.ActiveChild, "ActiveChild should be button")
	assert.True(t, buttonPressed, "button press callback should be called")

	// Click back on first input box
	t.Log("Step 6: Click back on inputBox1")
	mockInput.MouseClick(20, 5, 0)

	assert.True(t, inputBox1.IsFocused(), "inputBox1 should be focused again")
	assert.False(t, inputBox2.IsFocused(), "inputBox2 should not be focused")
	assert.False(t, button.IsFocused(), "button should lose focus")
	assert.Equal(t, inputBox1, layout.ActiveChild, "ActiveChild should be inputBox1")

	// Type more in first input box
	t.Log("Step 7: Type ' Again' in inputBox1")
	mockInput.TypeKeys(" Again")
	assert.Equal(t, "Hello Again", inputBox1.GetText(), "inputBox1 should contain 'Hello Again'")
	assert.Equal(t, "World", inputBox2.GetText(), "inputBox2 should still contain 'World'")
}

// TestFocusIntegration_MultipleButtons tests clicking between multiple buttons
func TestFocusIntegration_MultipleButtons(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)
	disabledAttrs := gtv.AttrsWithColor(0, 0x808080, 0x000000)

	button1Pressed := 0
	button1 := tui.NewButton(
		layout,
		"Button 1",
		gtv.TRect{X: 10, Y: 5, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)
	button1.SetOnPress(func() {
		button1Pressed++
	})

	button2Pressed := 0
	button2 := tui.NewButton(
		layout,
		"Button 2",
		gtv.TRect{X: 10, Y: 10, W: 0, H: 0},
		normalAttrs,
		focusedAttrs,
		disabledAttrs,
	)
	button2.SetOnPress(func() {
		button2Pressed++
	})

	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)
	mockInput := tio.NewMockInputEventReader(app)

	// Click on button1
	t.Log("Click on button1")
	mockInput.MouseClick(12, 5, 0)
	assert.True(t, button1.IsFocused(), "button1 should be focused")
	assert.False(t, button2.IsFocused(), "button2 should not be focused")
	assert.Equal(t, 1, button1Pressed, "button1 should be pressed once")
	assert.Equal(t, 0, button2Pressed, "button2 should not be pressed")

	// Click on button2
	t.Log("Click on button2")
	mockInput.MouseClick(12, 10, 0)
	assert.False(t, button1.IsFocused(), "button1 should lose focus")
	assert.True(t, button2.IsFocused(), "button2 should be focused")
	assert.Equal(t, 1, button1Pressed, "button1 should still be pressed once")
	assert.Equal(t, 1, button2Pressed, "button2 should be pressed once")

	// Click on button1 again
	t.Log("Click on button1 again")
	mockInput.MouseClick(12, 5, 0)
	assert.True(t, button1.IsFocused(), "button1 should be focused")
	assert.False(t, button2.IsFocused(), "button2 should lose focus")
	assert.Equal(t, 2, button1Pressed, "button1 should be pressed twice")
	assert.Equal(t, 1, button2Pressed, "button2 should still be pressed once")
}
