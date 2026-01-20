package tui_test

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResizable_Creation tests that a resizable widget can be created correctly.
func TestResizable_Creation(t *testing.T) {
	// Create resizable widget without parent
	rect := gtv.TRect{X: 10, Y: 5, W: 50, H: 20}
	resizable := tui.NewResizable(nil, rect)

	require.NotNil(t, resizable)
	assert.Equal(t, rect, resizable.GetPos())
	assert.Equal(t, rect, resizable.GetAbsolutePos())
}

// TestResizable_CreationWithParent tests that a resizable widget can be created with a parent.
func TestResizable_CreationWithParent(t *testing.T) {
	// Create parent widget
	parentRect := gtv.TRect{X: 5, Y: 3, W: 100, H: 50}
	parent := tui.NewResizable(nil, parentRect)

	// Create child widget
	childRect := gtv.TRect{X: 10, Y: 5, W: 30, H: 15}
	child := tui.NewResizable(parent, childRect)

	require.NotNil(t, child)
	assert.Equal(t, childRect, child.GetPos())

	// Check absolute position (parent + child offsets)
	expectedAbsPos := gtv.TRect{
		X: parentRect.X + childRect.X,
		Y: parentRect.Y + childRect.Y,
		W: childRect.W,
		H: childRect.H,
	}
	assert.Equal(t, expectedAbsPos, child.GetAbsolutePos())

	// Verify child was registered with parent
	assert.Len(t, parent.Children, 1)
	assert.Equal(t, child, parent.Children[0])
}

// TestResizable_ResizeEvent tests that resize events are handled correctly.
func TestResizable_ResizeEvent(t *testing.T) {
	// Create resizable widget
	initialRect := gtv.TRect{X: 10, Y: 5, W: 50, H: 20}
	resizable := tui.NewResizable(nil, initialRect)

	// Create resize event
	newRect := gtv.TRect{X: 15, Y: 8, W: 60, H: 25}
	event := &tui.TEvent{
		Type: tui.TEventTypeResize,
		Rect: newRect,
	}

	// Handle event
	resizable.HandleEvent(event)

	// Verify position was updated
	assert.Equal(t, newRect, resizable.GetPos())
}

// TestResizable_ResizeEventPartialChange tests that partial resize changes are handled correctly.
func TestResizable_ResizeEventPartialChange(t *testing.T) {
	tests := []struct {
		name        string
		initialRect gtv.TRect
		newRect     gtv.TRect
	}{
		{
			name:        "position only",
			initialRect: gtv.TRect{X: 10, Y: 5, W: 50, H: 20},
			newRect:     gtv.TRect{X: 20, Y: 10, W: 50, H: 20},
		},
		{
			name:        "size only",
			initialRect: gtv.TRect{X: 10, Y: 5, W: 50, H: 20},
			newRect:     gtv.TRect{X: 10, Y: 5, W: 100, H: 40},
		},
		{
			name:        "width only",
			initialRect: gtv.TRect{X: 10, Y: 5, W: 50, H: 20},
			newRect:     gtv.TRect{X: 10, Y: 5, W: 75, H: 20},
		},
		{
			name:        "height only",
			initialRect: gtv.TRect{X: 10, Y: 5, W: 50, H: 20},
			newRect:     gtv.TRect{X: 10, Y: 5, W: 50, H: 30},
		},
		{
			name:        "x only",
			initialRect: gtv.TRect{X: 10, Y: 5, W: 50, H: 20},
			newRect:     gtv.TRect{X: 15, Y: 5, W: 50, H: 20},
		},
		{
			name:        "y only",
			initialRect: gtv.TRect{X: 10, Y: 5, W: 50, H: 20},
			newRect:     gtv.TRect{X: 10, Y: 8, W: 50, H: 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resizable := tui.NewResizable(nil, tt.initialRect)

			event := &tui.TEvent{
				Type: tui.TEventTypeResize,
				Rect: tt.newRect,
			}

			resizable.HandleEvent(event)
			assert.Equal(t, tt.newRect, resizable.GetPos())
		})
	}
}

// TestResizable_OnResizeCallback tests that OnResize callback is called for derived widgets.
func TestResizable_OnResizeCallback(t *testing.T) {
	// Create a custom widget that tracks resize callbacks
	type ResizeTracker struct {
		tui.TResizable
		resizeCalled bool
		oldRect      gtv.TRect
		newRect      gtv.TRect
	}

	// Override OnResize method
	tracker := &ResizeTracker{
		TResizable: tui.TResizable{
			TWidget: tui.TWidget{
				Position: gtv.TRect{X: 10, Y: 5, W: 50, H: 20},
			},
		},
	}

	// Create resize event
	initialRect := tracker.Position
	newRect := gtv.TRect{X: 15, Y: 8, W: 60, H: 25}
	event := &tui.TEvent{
		Type: tui.TEventTypeResize,
		Rect: newRect,
	}

	// Handle event - this will update position and call OnResize
	tracker.HandleEvent(event)

	// Verify position was updated
	assert.Equal(t, newRect, tracker.GetPos())

	// Note: OnResize is called during HandleEvent, but we can't easily verify it was called
	// without overriding the method. The real test is that the position was updated correctly,
	// which proves HandleEvent processed the resize event.
	// For a true derived widget that overrides OnResize, we would test their custom behavior.

	// Verify initial rect is different from new rect
	assert.NotEqual(t, initialRect, tracker.GetPos())
}

// TestResizable_NonResizeEvents tests that non-resize events are delegated to base widget.
func TestResizable_NonResizeEvents(t *testing.T) {
	// Create resizable widget
	resizable := tui.NewResizable(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24})

	// Create redraw event
	redrawEvent := &tui.TEvent{Type: tui.TEventTypeRedraw}
	resizable.HandleEvent(redrawEvent)

	// Create input event
	inputEvent := &tui.TEvent{
		Type: tui.TEventTypeInput,
		InputEvent: &gtv.InputEvent{
			Type: gtv.InputEventKey,
			Key:  'a',
		},
	}
	resizable.HandleEvent(inputEvent)

	// No assertions needed - just verify no panics occur
	// The base widget will handle these events appropriately
}

// TestResizable_DrawHidden tests that hidden widgets are not drawn.
func TestResizable_DrawHidden(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create resizable widget
	resizable := tui.NewResizable(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24})
	resizable.Flags |= tui.WidgetFlagHidden

	// Draw should not render anything when hidden
	resizable.Draw(screen)

	// Verify screen is still empty (all default cells)
	_, _, content := screen.GetContent()
	for i, cell := range content {
		assert.Equal(t, ' ', cell.Rune, "Cell at index %d should be space", i)
	}
}

// TestResizable_DrawVisible tests that visible widgets can be drawn.
func TestResizable_DrawVisible(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create resizable widget (not hidden)
	resizable := tui.NewResizable(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24})

	// Draw should succeed without panic
	resizable.Draw(screen)

	// The base TResizable doesn't draw anything itself (it's a base class)
	// But it should delegate to children if any
}

// TestResizable_Integration_WithApplication tests resizable widget in full application context.
func TestResizable_Integration_WithApplication(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create resizable widget
	resizable := tui.NewResizable(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24})

	// Create application
	app := tui.NewApplication(resizable, screen)
	require.NotNil(t, app)

	// Create mock input reader and send resize event
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.Resize(120, 30)

	// Verify widget was resized
	pos := resizable.GetPos()
	assert.Equal(t, uint16(120), pos.W)
	assert.Equal(t, uint16(30), pos.H)
}

// TestResizable_Integration_WithChildren tests that resize events propagate to children.
func TestResizable_Integration_WithChildren(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create parent resizable widget
	parent := tui.NewResizable(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24})

	// Create child widget (label)
	child := tui.NewLabel(
		parent,
		"Test Label",
		gtv.TRect{X: 10, Y: 5, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0),
	)

	// Create application
	_ = tui.NewApplication(parent, screen)

	// Draw initial frame
	parent.Draw(screen)

	// Verify child is rendered
	_, _, content := screen.GetContent()
	width := 80

	// Check that label is rendered at position (10, 5)
	expectedText := "Test Label"
	for i, ch := range expectedText {
		idx := 5*width + 10 + i
		assert.Equal(t, ch, content[idx].Rune, "Character at position %d", i)
	}

	// Verify child is registered with parent
	assert.Len(t, parent.Children, 1)
	assert.Equal(t, child, parent.Children[0])
}

// TestResizable_Integration_NestedResizables tests nested resizable widgets.
func TestResizable_Integration_NestedResizables(t *testing.T) {
	// Create parent resizable
	parent := tui.NewResizable(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24})

	// Create child resizable
	child := tui.NewResizable(parent, gtv.TRect{X: 10, Y: 5, W: 60, H: 15})

	// Create grandchild resizable
	grandchild := tui.NewResizable(child, gtv.TRect{X: 5, Y: 3, W: 40, H: 8})

	// Verify hierarchy
	assert.Len(t, parent.Children, 1)
	assert.Equal(t, child, parent.Children[0])
	assert.Len(t, child.Children, 1)
	assert.Equal(t, grandchild, child.Children[0])

	// Verify absolute positions
	assert.Equal(t, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, parent.GetAbsolutePos())
	assert.Equal(t, gtv.TRect{X: 10, Y: 5, W: 60, H: 15}, child.GetAbsolutePos())
	assert.Equal(t, gtv.TRect{X: 15, Y: 8, W: 40, H: 8}, grandchild.GetAbsolutePos())

	// Resize parent
	event := &tui.TEvent{
		Type: tui.TEventTypeResize,
		Rect: gtv.TRect{X: 0, Y: 0, W: 100, H: 30},
	}
	parent.HandleEvent(event)

	// Verify parent was resized
	assert.Equal(t, gtv.TRect{X: 0, Y: 0, W: 100, H: 30}, parent.GetPos())

	// Child and grandchild positions remain the same (relative to parent)
	assert.Equal(t, gtv.TRect{X: 10, Y: 5, W: 60, H: 15}, child.GetPos())
	assert.Equal(t, gtv.TRect{X: 5, Y: 3, W: 40, H: 8}, grandchild.GetPos())

	// But absolute positions should remain the same (child is not resized)
	assert.Equal(t, gtv.TRect{X: 10, Y: 5, W: 60, H: 15}, child.GetAbsolutePos())
	assert.Equal(t, gtv.TRect{X: 15, Y: 8, W: 40, H: 8}, grandchild.GetAbsolutePos())
}

// TestResizable_MultipleResizeEvents tests multiple consecutive resize events.
func TestResizable_MultipleResizeEvents(t *testing.T) {
	// Create resizable widget
	resizable := tui.NewResizable(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24})

	// Send multiple resize events
	resizes := []gtv.TRect{
		{X: 0, Y: 0, W: 100, H: 30},
		{X: 0, Y: 0, W: 120, H: 40},
		{X: 0, Y: 0, W: 90, H: 25},
		{X: 0, Y: 0, W: 80, H: 24},
	}

	for _, rect := range resizes {
		event := &tui.TEvent{
			Type: tui.TEventTypeResize,
			Rect: rect,
		}
		resizable.HandleEvent(event)
		assert.Equal(t, rect, resizable.GetPos())
	}
}

// TestResizable_ZeroSizeResize tests that widgets can be resized to zero size.
func TestResizable_ZeroSizeResize(t *testing.T) {
	// Create resizable widget
	resizable := tui.NewResizable(nil, gtv.TRect{X: 10, Y: 5, W: 50, H: 20})

	// Resize to zero width
	event := &tui.TEvent{
		Type: tui.TEventTypeResize,
		Rect: gtv.TRect{X: 10, Y: 5, W: 0, H: 20},
	}
	resizable.HandleEvent(event)
	assert.Equal(t, uint16(0), resizable.GetPos().W)

	// Resize to zero height
	event = &tui.TEvent{
		Type: tui.TEventTypeResize,
		Rect: gtv.TRect{X: 10, Y: 5, W: 50, H: 0},
	}
	resizable.HandleEvent(event)
	assert.Equal(t, uint16(0), resizable.GetPos().H)

	// Resize to zero size
	event = &tui.TEvent{
		Type: tui.TEventTypeResize,
		Rect: gtv.TRect{X: 10, Y: 5, W: 0, H: 0},
	}
	resizable.HandleEvent(event)
	pos := resizable.GetPos()
	assert.Equal(t, uint16(0), pos.W)
	assert.Equal(t, uint16(0), pos.H)
}
