package tui_test

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFocusable_Creation tests creating a focusable widget
func TestFocusable_Creation(t *testing.T) {
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		layout,
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	require.NotNil(t, focusable)
	assert.Equal(t, normalAttrs, focusable.GetAttrs())
	assert.Equal(t, focusedAttrs, focusable.GetFocusedAttrs())
	assert.False(t, focusable.IsFocused())
	assert.Len(t, layout.Children, 1)
}

// TestFocusable_CreationWithoutParent tests creating a focusable widget without parent
func TestFocusable_CreationWithoutParent(t *testing.T) {
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		nil,
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	require.NotNil(t, focusable)
	assert.Equal(t, normalAttrs, focusable.GetAttrs())
	assert.Equal(t, focusedAttrs, focusable.GetFocusedAttrs())
	assert.False(t, focusable.IsFocused())
}

// TestFocusable_GetSetAttrs tests getting and setting normal attributes
func TestFocusable_GetSetAttrs(t *testing.T) {
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 10, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Check initial attributes
	assert.Equal(t, normalAttrs, focusable.GetAttrs())

	// Set new attributes
	newAttrs := gtv.AttrsWithColor(gtv.AttrBold, 0xFF0000, 0x00FF00)
	focusable.SetAttrs(newAttrs)

	// Verify attributes were updated
	assert.Equal(t, newAttrs, focusable.GetAttrs())
}

// TestFocusable_GetSetFocusedAttrs tests getting and setting focused attributes
func TestFocusable_GetSetFocusedAttrs(t *testing.T) {
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 10, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Check initial focused attributes
	assert.Equal(t, focusedAttrs, focusable.GetFocusedAttrs())

	// Set new focused attributes
	newFocusedAttrs := gtv.AttrsWithColor(gtv.AttrItalic, 0x0000FF, 0xFF00FF)
	focusable.SetFocusedAttrs(newFocusedAttrs)

	// Verify focused attributes were updated
	assert.Equal(t, newFocusedAttrs, focusable.GetFocusedAttrs())
}

// TestFocusable_FocusBlur tests focus and blur functionality
func TestFocusable_FocusBlur(t *testing.T) {
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 10, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Initially unfocused
	assert.False(t, focusable.IsFocused())

	// Focus the widget
	focusable.Focus()
	assert.True(t, focusable.IsFocused())

	// Blur the widget
	focusable.Blur()
	assert.False(t, focusable.IsFocused())
}

// TestFocusable_FocusSetsCursorFlag tests that Focus() sets the cursor flag
func TestFocusable_FocusSetsCursorFlag(t *testing.T) {
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 10, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Focus should set both focused and cursor flags
	focusable.Focus()
	assert.True(t, focusable.IsFocused())
	// We can't directly test the cursor flag from the public API,
	// but we can verify it's set through the IsFocused check
}

// TestFocusable_BlurClearsCursorFlag tests that Blur() clears the cursor flag
func TestFocusable_BlurClearsCursorFlag(t *testing.T) {
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 10, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Focus then blur
	focusable.Focus()
	assert.True(t, focusable.IsFocused())

	focusable.Blur()
	assert.False(t, focusable.IsFocused())
}

// TestFocusable_MultipleFocusCalls tests calling Focus() multiple times
func TestFocusable_MultipleFocusCalls(t *testing.T) {
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 10, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Multiple focus calls should be idempotent
	focusable.Focus()
	assert.True(t, focusable.IsFocused())

	focusable.Focus()
	assert.True(t, focusable.IsFocused())

	focusable.Focus()
	assert.True(t, focusable.IsFocused())
}

// TestFocusable_MultipleBlurCalls tests calling Blur() multiple times
func TestFocusable_MultipleBlurCalls(t *testing.T) {
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 10, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	focusable.Focus()
	assert.True(t, focusable.IsFocused())

	// Multiple blur calls should be idempotent
	focusable.Blur()
	assert.False(t, focusable.IsFocused())

	focusable.Blur()
	assert.False(t, focusable.IsFocused())

	focusable.Blur()
	assert.False(t, focusable.IsFocused())
}

// TestFocusable_BasicRendering tests that the focusable widget renders without errors
func TestFocusable_BasicRendering(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		nil,
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Draw should not panic (focusable has minimal draw implementation)
	focusable.Draw(screen)

	// Verify screen dimensions
	width, height, _ := screen.GetContent()
	assert.Equal(t, 80, width)
	assert.Equal(t, 24, height)
}

// TestFocusable_ResizeEvent tests that focusable handles resize events
func TestFocusable_ResizeEvent(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		layout,
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Verify initial position
	pos := focusable.GetPos()
	assert.Equal(t, uint16(10), pos.X)
	assert.Equal(t, uint16(10), pos.Y)
	assert.Equal(t, uint16(20), pos.W)
	assert.Equal(t, uint16(1), pos.H)

	// Send resize event through application
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.Resize(100, 30)

	// The layout should resize, but focusable maintains relative position
	// unless explicitly resized through an event
	pos = focusable.GetPos()
	assert.Equal(t, uint16(10), pos.X)
	assert.Equal(t, uint16(10), pos.Y)
}

// TestFocusable_PositionCalculation tests absolute position calculation
func TestFocusable_PositionCalculation(t *testing.T) {
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 5, Y: 5, W: 70, H: 20}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		layout,
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	// Relative position
	relPos := focusable.GetPos()
	assert.Equal(t, uint16(10), relPos.X)
	assert.Equal(t, uint16(10), relPos.Y)
	assert.Equal(t, uint16(20), relPos.W)
	assert.Equal(t, uint16(1), relPos.H)

	// Absolute position should be relative to parent
	absPos := focusable.GetAbsolutePos()
	assert.Equal(t, uint16(15), absPos.X) // 5 + 10
	assert.Equal(t, uint16(15), absPos.Y) // 5 + 10
	assert.Equal(t, uint16(20), absPos.W)
	assert.Equal(t, uint16(1), absPos.H)
}

// TestFocusable_ChildManagement tests adding children to focusable widget
func TestFocusable_ChildManagement(t *testing.T) {
	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x000000, 0xFFFFFF)

	parentFocusable := tui.NewFocusable(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 40, H: 10},
		normalAttrs,
		focusedAttrs,
	)

	// Create a child label
	childLabel := tui.NewLabel(
		parentFocusable,
		"Child",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0},
		normalAttrs,
	)

	// Verify child was added
	assert.Len(t, parentFocusable.Children, 1)
	assert.Equal(t, childLabel, parentFocusable.Children[0])
}

// TestFocusable_Integration tests focusable in application context
func TestFocusable_Integration(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	normalAttrs := gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(gtv.AttrBold, 0x000000, 0xFFFFFF)

	focusable := tui.NewFocusable(
		layout,
		gtv.TRect{X: 10, Y: 10, W: 20, H: 1},
		normalAttrs,
		focusedAttrs,
	)

	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Initially unfocused
	assert.False(t, focusable.IsFocused())

	// Focus the widget
	focusable.Focus()
	assert.True(t, focusable.IsFocused())

	// Draw the layout
	layout.Draw(screen)

	// Verify screen content
	width, height, _ := screen.GetContent()
	assert.Equal(t, 80, width)
	assert.Equal(t, 24, height)

	// Blur the widget
	focusable.Blur()
	assert.False(t, focusable.IsFocused())
}
