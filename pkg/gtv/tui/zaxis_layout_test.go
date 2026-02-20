package tui_test

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/rlewczuk/csw/pkg/gtv/tio"
	"github.com/rlewczuk/csw/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestZAxisLayout_BasicRendering tests that the Z-axis layout correctly renders
// widgets in Z-order (lower z-index first, higher z-index on top).
func TestZAxisLayout_BasicRendering(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout (fills entire screen)
	layout := tui.NewZAxisLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create three overlapping labels with different z-indices
	// Label 1: z-index 0 (bottom layer)
	label1 := tui.NewLabel(
		nil, // Don't use parent.AddChild, we'll use AddZWidget
		"Layer 0",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFF0000, 0x000000),
	)
	layout.AddZWidget(label1, 0)

	// Label 2: z-index 1 (middle layer)
	label2 := tui.NewLabel(
		nil,
		"Layer 1",
		gtv.TRect{X: 10, Y: 6, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0x00FF00, 0x000000),
	)
	layout.AddZWidget(label2, 1)

	// Label 3: z-index 2 (top layer)
	label3 := tui.NewLabel(
		nil,
		"Layer 2",
		gtv.TRect{X: 15, Y: 7, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0x0000FF, 0x000000),
	)
	layout.AddZWidget(label3, 2)

	// Create application
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Draw initial frame
	layout.Draw(screen)

	// Verify screen content
	width, height, content := screen.GetContent()
	assert.Equal(t, 80, width)
	assert.Equal(t, 24, height)

	// Check that label1 is rendered at position (5, 5)
	expectedText1 := "Layer 0"
	for i, ch := range expectedText1 {
		idx := 5*width + 5 + i
		assert.Equal(t, ch, content[idx].Rune, "Label1 character at position %d", i)
		assert.Equal(t, gtv.TextColor(0xFF0000), content[idx].Attrs.TextColor)
	}

	// Check that label2 is rendered at position (10, 6)
	expectedText2 := "Layer 1"
	for i, ch := range expectedText2 {
		idx := 6*width + 10 + i
		assert.Equal(t, ch, content[idx].Rune, "Label2 character at position %d", i)
		assert.Equal(t, gtv.TextColor(0x00FF00), content[idx].Attrs.TextColor)
	}

	// Check that label3 is rendered at position (15, 7) and is on top
	expectedText3 := "Layer 2"
	for i, ch := range expectedText3 {
		idx := 7*width + 15 + i
		assert.Equal(t, ch, content[idx].Rune, "Label3 character at position %d", i)
		assert.Equal(t, gtv.TextColor(0x0000FF), content[idx].Attrs.TextColor)
	}
}

// TestZAxisLayout_ZIndexOrdering tests that widgets are drawn in correct z-index order
// regardless of the order they were added.
func TestZAxisLayout_ZIndexOrdering(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout
	layout := tui.NewZAxisLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Add widgets in non-sequential z-index order
	// Widget with z-index 2 (should be on top)
	label2 := tui.NewLabel(
		nil,
		"Top",
		gtv.TRect{X: 10, Y: 10, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0x0000FF, 0x000000),
	)
	layout.AddZWidget(label2, 2)

	// Widget with z-index 0 (should be on bottom)
	label0 := tui.NewLabel(
		nil,
		"Bottom",
		gtv.TRect{X: 10, Y: 10, W: 0, H: 0}, // Same position as label2
		gtv.AttrsWithColor(0, 0xFF0000, 0x000000),
	)
	layout.AddZWidget(label0, 0)

	// Widget with z-index 1 (should be in middle)
	label1 := tui.NewLabel(
		nil,
		"Middle",
		gtv.TRect{X: 10, Y: 10, W: 0, H: 0}, // Same position as others
		gtv.AttrsWithColor(0, 0x00FF00, 0x000000),
	)
	layout.AddZWidget(label1, 1)

	// Draw
	layout.Draw(screen)

	// Verify screen content - should show "Top" (z-index 2) since it's on top
	width, _, content := screen.GetContent()
	expectedText := "Top"
	for i, ch := range expectedText {
		idx := 10*width + 10 + i
		assert.Equal(t, ch, content[idx].Rune, "Character at position %d", i)
		// Should have blue color from label2 (z-index 2)
		assert.Equal(t, gtv.TextColor(0x0000FF), content[idx].Attrs.TextColor)
	}
}

// TestZAxisLayout_DimmingBehavior tests that lower z-index widgets are dimmed
// when a higher z-index widget has ZWidgetBehaviorDim.
func TestZAxisLayout_DimmingBehavior(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout
	layout := tui.NewZAxisLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Add a bottom layer widget
	label1 := tui.NewLabel(
		nil,
		"Background",
		gtv.TRect{X: 5, Y: 5, W: 20, H: 1},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000),
	)
	layout.AddZWidget(label1, 0)

	// Add a top layer widget with dimming behavior
	label2 := tui.NewLabel(
		nil,
		"Popup",
		gtv.TRect{X: 30, Y: 10, W: 10, H: 1},
		gtv.AttrsWithColor(0, 0xFF0000, 0x000000),
	)
	layout.AddZWidget(label2, 1, tui.WithZBehavior(tui.ZWidgetBehaviorDim))

	// Draw
	layout.Draw(screen)

	// Verify that label1 is rendered
	width, _, content := screen.GetContent()
	expectedText1 := "Background"
	for i, ch := range expectedText1 {
		idx := 5*width + 5 + i
		assert.Equal(t, ch, content[idx].Rune, "Label1 character at position %d", i)
	}

	// Verify that label2 is rendered
	expectedText2 := "Popup"
	for i, ch := range expectedText2 {
		idx := 10*width + 30 + i
		assert.Equal(t, ch, content[idx].Rune, "Label2 character at position %d", i)
	}

	// Verify that label1 area has dimming overlay applied
	// Check a cell in label1's area - it should have AttrDim
	idx := 5*width + 5
	assert.Equal(t, gtv.AttrDim, content[idx].Attrs.Attributes&gtv.AttrDim,
		"Label1 should have dimming overlay applied")
}

// TestZAxisLayout_HidingBehavior tests that lower z-index widgets are completely hidden
// when a higher z-index widget has ZWidgetBehaviorHide.
func TestZAxisLayout_HidingBehavior(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout with a background color
	layout := tui.NewZAxisLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		&gtv.CellAttributes{BackColor: 0x000000},
	)

	// Add a bottom layer widget
	label1 := tui.NewLabel(
		nil,
		"Hidden",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000),
	)
	layout.AddZWidget(label1, 0)

	// Add a top layer widget with hiding behavior
	label2 := tui.NewLabel(
		nil,
		"Splash",
		gtv.TRect{X: 30, Y: 10, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFF0000, 0x000000),
	)
	layout.AddZWidget(label2, 1, tui.WithZBehavior(tui.ZWidgetBehaviorHide))

	// Draw
	layout.Draw(screen)

	// Verify that label1 is NOT rendered (should be hidden)
	width, _, content := screen.GetContent()
	// Check position where label1 should be - should have background color, not label1's text
	idx := 5*width + 5
	assert.NotEqual(t, 'H', content[idx].Rune, "Label1 should be hidden")
	// Should have background color (black)
	assert.Equal(t, ' ', content[idx].Rune, "Should have background space")

	// Verify that label2 IS rendered
	expectedText2 := "Splash"
	for i, ch := range expectedText2 {
		idx := 10*width + 30 + i
		assert.Equal(t, ch, content[idx].Rune, "Label2 character at position %d", i)
	}
}

// TestZAxisLayout_RemoveWidget tests that widgets can be removed from the layout.
func TestZAxisLayout_RemoveWidget(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout
	layout := tui.NewZAxisLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Add widgets
	label1 := tui.NewLabel(
		nil,
		"Widget1",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFF0000, 0x000000),
	)
	layout.AddZWidget(label1, 0)

	label2 := tui.NewLabel(
		nil,
		"Widget2",
		gtv.TRect{X: 10, Y: 10, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0x00FF00, 0x000000),
	)
	layout.AddZWidget(label2, 1)

	// Draw with both widgets
	layout.Draw(screen)

	// Verify both widgets are rendered
	width, _, content := screen.GetContent()
	idx1 := 5*width + 5
	assert.Equal(t, 'W', content[idx1].Rune, "Widget1 should be rendered")

	idx2 := 10*width + 10
	assert.Equal(t, 'W', content[idx2].Rune, "Widget2 should be rendered")

	// Remove label1
	layout.RemoveZWidget(label1)

	// Create a new screen buffer to ensure clean state
	screen = tio.NewScreenBuffer(80, 24, 0)
	layout.Draw(screen)

	// Verify label1 is not rendered anymore
	_, _, content = screen.GetContent()
	idx1 = 5*width + 5
	assert.NotEqual(t, 'W', content[idx1].Rune, "Widget1 should be removed")
	assert.Equal(t, ' ', content[idx1].Rune, "Should be empty space")

	// Verify label2 is still rendered
	idx2 = 10*width + 10
	assert.Equal(t, 'W', content[idx2].Rune, "Widget2 should still be rendered")
}

// ClickTracker is a test widget that tracks mouse clicks.
type ClickTracker struct {
	tui.TWidget
	id            int
	clickedWidget *int
}

func (c *ClickTracker) HandleEvent(event *tui.TEvent) {
	if event.Type == tui.TEventTypeInput && event.InputEvent != nil {
		if event.InputEvent.Type == gtv.InputEventMouse {
			*c.clickedWidget = c.id
		}
	}
	c.TWidget.HandleEvent(event)
}

// TestZAxisLayout_MouseEventRouting tests that mouse events are routed to the
// topmost widget under the cursor.
func TestZAxisLayout_MouseEventRouting(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout
	layout := tui.NewZAxisLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Track which widget received events
	var clickedWidget int

	tracker1 := &ClickTracker{
		TWidget: tui.TWidget{
			Position: gtv.TRect{X: 10, Y: 10, W: 20, H: 5},
		},
		id:            1,
		clickedWidget: &clickedWidget,
	}

	tracker2 := &ClickTracker{
		TWidget: tui.TWidget{
			Position: gtv.TRect{X: 15, Y: 12, W: 10, H: 3}, // Overlaps with tracker1
		},
		id:            2,
		clickedWidget: &clickedWidget,
	}

	// Add widgets (tracker2 has higher z-index)
	layout.AddZWidget(tracker1, 0)
	layout.AddZWidget(tracker2, 1)

	// Create application
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input reader
	mockInput := tio.NewMockInputEventReader(app)

	// Click on overlapping area - should hit tracker2 (higher z-index)
	clickedWidget = 0
	mockInput.MouseClick(20, 13, 0)
	assert.Equal(t, 2, clickedWidget, "Should route to topmost widget (tracker2)")

	// Click on tracker1 only area (not overlapping with tracker2)
	clickedWidget = 0
	mockInput.MouseClick(12, 11, 0)
	assert.Equal(t, 1, clickedWidget, "Should route to tracker1 when not overlapping")
}

// TestZAxisLayout_HiddenWidgetNoEvents tests that hidden widgets don't receive events.
func TestZAxisLayout_HiddenWidgetNoEvents(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout
	layout := tui.NewZAxisLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Track which widget received events
	var clickedWidget int

	tracker1 := &ClickTracker{
		TWidget: tui.TWidget{
			Position: gtv.TRect{X: 10, Y: 10, W: 20, H: 5},
		},
		id:            1,
		clickedWidget: &clickedWidget,
	}

	tracker2 := &ClickTracker{
		TWidget: tui.TWidget{
			Position: gtv.TRect{X: 10, Y: 10, W: 20, H: 5}, // Same position as tracker1
		},
		id:            2,
		clickedWidget: &clickedWidget,
	}

	// Add widgets (tracker2 has higher z-index and hides tracker1)
	layout.AddZWidget(tracker1, 0)
	layout.AddZWidget(tracker2, 1, tui.WithZBehavior(tui.ZWidgetBehaviorHide))

	// Create application
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input reader
	mockInput := tio.NewMockInputEventReader(app)

	// Click on area where both widgets are - should only hit tracker2
	clickedWidget = 0
	mockInput.MouseClick(15, 12, 0)
	assert.Equal(t, 2, clickedWidget, "Should only route to visible widget (tracker2)")
}

// TestZAxisLayout_MultipleDimmingLayers tests that multiple dimming layers
// are handled correctly.
func TestZAxisLayout_MultipleDimmingLayers(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout
	layout := tui.NewZAxisLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Add three layers, each with dimming behavior
	label1 := tui.NewLabel(
		nil,
		"Layer0",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFF0000, 0x000000),
	)
	layout.AddZWidget(label1, 0)

	label2 := tui.NewLabel(
		nil,
		"Layer1",
		gtv.TRect{X: 30, Y: 10, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0x00FF00, 0x000000),
	)
	layout.AddZWidget(label2, 1, tui.WithZBehavior(tui.ZWidgetBehaviorDim))

	label3 := tui.NewLabel(
		nil,
		"Layer2",
		gtv.TRect{X: 50, Y: 15, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0x0000FF, 0x000000),
	)
	layout.AddZWidget(label3, 2, tui.WithZBehavior(tui.ZWidgetBehaviorDim))

	// Draw
	layout.Draw(screen)

	// Verify all widgets are rendered
	width, _, content := screen.GetContent()

	// Check label1 - should have dimming from both label2 and label3
	idx1 := 5*width + 5
	assert.Equal(t, 'L', content[idx1].Rune, "Label1 should be rendered")
	assert.Equal(t, gtv.AttrDim, content[idx1].Attrs.Attributes&gtv.AttrDim,
		"Label1 should have dimming overlay")

	// Check label2 - should have dimming from label3 only
	idx2 := 10*width + 30
	assert.Equal(t, 'L', content[idx2].Rune, "Label2 should be rendered")
	assert.Equal(t, gtv.AttrDim, content[idx2].Attrs.Attributes&gtv.AttrDim,
		"Label2 should have dimming overlay")

	// Check label3 - should NOT have dimming (it's on top)
	idx3 := 15*width + 50
	assert.Equal(t, 'L', content[idx3].Rune, "Label3 should be rendered")
	assert.NotEqual(t, gtv.AttrDim, content[idx3].Attrs.Attributes&gtv.AttrDim,
		"Label3 should NOT have dimming overlay")
}

// TestZAxisLayout_HideOverridesDim tests that hide behavior takes precedence over dim behavior.
func TestZAxisLayout_HideOverridesDim(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout
	layout := tui.NewZAxisLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		&gtv.CellAttributes{BackColor: 0x000000},
	)

	// Add three layers
	label1 := tui.NewLabel(
		nil,
		"Bottom",
		gtv.TRect{X: 5, Y: 5, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFF0000, 0x000000),
	)
	layout.AddZWidget(label1, 0)

	// Middle layer with dim behavior
	label2 := tui.NewLabel(
		nil,
		"Middle",
		gtv.TRect{X: 30, Y: 10, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0x00FF00, 0x000000),
	)
	layout.AddZWidget(label2, 1, tui.WithZBehavior(tui.ZWidgetBehaviorDim))

	// Top layer with hide behavior
	label3 := tui.NewLabel(
		nil,
		"Top",
		gtv.TRect{X: 50, Y: 15, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0x0000FF, 0x000000),
	)
	layout.AddZWidget(label3, 2, tui.WithZBehavior(tui.ZWidgetBehaviorHide))

	// Draw
	layout.Draw(screen)

	// Verify results
	width, _, content := screen.GetContent()

	// Label1 and label2 should be hidden (not rendered)
	idx1 := 5*width + 5
	assert.NotEqual(t, 'B', content[idx1].Rune, "Label1 should be hidden")

	idx2 := 10*width + 30
	assert.NotEqual(t, 'M', content[idx2].Rune, "Label2 should be hidden")

	// Label3 should be visible
	idx3 := 15*width + 50
	assert.Equal(t, 'T', content[idx3].Rune, "Label3 should be visible")
}

// TestZAxisLayout_BackgroundRendering tests that background is rendered correctly.
func TestZAxisLayout_BackgroundRendering(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout with background color
	layout := tui.NewZAxisLayout(
		nil,
		gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
		&gtv.CellAttributes{BackColor: 0x123456},
	)

	// Draw
	layout.Draw(screen)

	// Verify background is rendered
	_, _, content := screen.GetContent()

	// Check a random cell - should have background color
	idx := 10*80 + 10
	assert.Equal(t, ' ', content[idx].Rune, "Should be background space")
	assert.Equal(t, gtv.TextColor(0x123456), content[idx].Attrs.BackColor,
		"Should have background color")
}

// TestZAxisLayout_ResizeEvent tests that resize events are handled correctly.
func TestZAxisLayout_ResizeEvent(t *testing.T) {
	// Create a screen buffer for testing
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create Z-axis layout
	layout := tui.NewZAxisLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create application
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Verify initial size
	pos := layout.GetPos()
	assert.Equal(t, uint16(80), pos.W)
	assert.Equal(t, uint16(24), pos.H)

	// Create mock input reader and send resize event
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.Resize(100, 30)

	// Verify layout was resized
	pos = layout.GetPos()
	assert.Equal(t, uint16(100), pos.W)
	assert.Equal(t, uint16(30), pos.H)
}
