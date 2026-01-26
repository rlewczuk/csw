package tui

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplicationMultipleChildren tests that TApplication can manage multiple children
func TestApplicationMultipleChildren(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create application without initial widget
	app := NewApplicationEmpty(screen)
	require.NotNil(t, app)

	// Add multiple children
	widget1 := &TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 40, H: 24}}
	widget2 := &TWidget{Position: gtv.TRect{X: 40, Y: 0, W: 40, H: 24}}
	widget3 := &TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 80, H: 12}}

	app.AddChild(widget1)
	app.AddChild(widget2)
	app.AddChild(widget3)

	// Verify children were added (accessing through embedded TFlexLayout)
	children := app.TFlexLayout.TLayout.TResizable.TWidget.Children
	assert.Len(t, children, 3)
	assert.Equal(t, widget1, children[0])
	assert.Equal(t, widget2, children[1])
	assert.Equal(t, widget3, children[2])
}

// TestApplicationFlexLayoutIntegration tests that TApplication properly uses TFlexLayout
func TestApplicationFlexLayoutIntegration(t *testing.T) {
	screen := tio.NewScreenBuffer(100, 50, 0)
	app := NewApplicationEmpty(screen)

	// Create children with flex properties
	widget1 := &TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 50}}
	widget2 := &TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 50}}

	app.AddChild(widget1)
	app.AddChild(widget2)

	// Set flex properties for children to grow and fill space
	app.SetItemProperties(widget1, FlexItemProperties{FlexGrow: 1.0, FlexShrink: 1.0})
	app.SetItemProperties(widget2, FlexItemProperties{FlexGrow: 1.0, FlexShrink: 1.0})

	// Set direction to row (horizontal layout)
	app.SetDirection(FlexDirectionRow)

	// After layout, widgets should be resized to share space equally
	// Widget 1 should get approximately 50 width
	// Widget 2 should get approximately 50 width
	pos1 := widget1.GetPos()
	pos2 := widget2.GetPos()

	// Check that both widgets have been laid out horizontally
	assert.Greater(t, pos1.W, uint16(40)) // Should be around 50
	assert.Greater(t, pos2.W, uint16(40)) // Should be around 50
	assert.Equal(t, uint16(50), pos1.H)
	assert.Equal(t, uint16(50), pos2.H)
}

// TestApplicationEventRouting tests that events are routed to children
func TestApplicationEventRouting(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	app := NewApplicationEmpty(screen)

	// Create simple widgets
	widget1 := &TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 40, H: 24}}
	widget2 := &TWidget{Position: gtv.TRect{X: 40, Y: 0, W: 40, H: 24}}

	app.AddChild(widget1)
	app.AddChild(widget2)

	// Set first widget as active
	app.ActiveChild = widget1

	// Send key event - verify no panic occurs
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.TypeKeys("a")

	// Verify active child is still set correctly
	assert.Equal(t, widget1, app.ActiveChild)
}

// TestApplicationDrawsChildren tests that TApplication draws all children
func TestApplicationDrawsChildren(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	app := NewApplicationEmpty(screen)

	// Create test widgets that write to screen
	widget1 := NewLabel(nil, "Widget1", gtv.TRect{X: 0, Y: 0, W: 20, H: 1}, gtv.CellAttributes{})
	widget2 := NewLabel(nil, "Widget2", gtv.TRect{X: 0, Y: 1, W: 20, H: 1}, gtv.CellAttributes{})

	app.AddChild(widget1)
	app.AddChild(widget2)

	// Draw application
	app.Draw(screen)

	// Verify both widgets were drawn by reading the screen content
	width, height, cells := screen.GetContent()
	assert.Equal(t, 80, width)
	assert.Equal(t, 24, height)

	// Extract text from first row
	text1 := ""
	for i := 0; i < 7 && i < len(cells); i++ {
		text1 += string(cells[i].Rune)
	}

	// Extract text from second row (offset by screen width)
	text2 := ""
	for i := 0; i < 7 && 80+i < len(cells); i++ {
		text2 += string(cells[80+i].Rune)
	}

	assert.Equal(t, "Widget1", text1)
	assert.Equal(t, "Widget2", text2)
}

// TestApplicationResizeWithChildren tests that resize events properly affect children
func TestApplicationResizeWithChildren(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	app := NewApplicationEmpty(screen)

	// Add a child with flex-grow
	widget := &TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 80, H: 24}}
	app.AddChild(widget)
	app.SetItemProperties(widget, FlexItemProperties{FlexGrow: 1.0, FlexShrink: 1.0})

	// Resize application
	mockInput := tio.NewMockInputEventReader(app)
	mockInput.Resize(100, 30)

	// Verify screen was resized
	w, h := screen.GetSize()
	assert.Equal(t, 100, w)
	assert.Equal(t, 30, h)

	// Verify application position was updated
	appPos := app.GetPos()
	assert.Equal(t, uint16(100), appPos.W)
	assert.Equal(t, uint16(30), appPos.H)

	// Verify child was resized by flex layout
	childPos := widget.GetPos()
	assert.Equal(t, uint16(100), childPos.W)
	assert.Equal(t, uint16(30), childPos.H)
}

// TestApplicationBackwardsCompatibility tests that old API (single main widget) still works
func TestApplicationBackwardsCompatibility(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 0, H: 0}}

	// Create application with single widget (old API)
	app := NewApplication(widget, screen)

	// Verify application was created
	require.NotNil(t, app)

	// Verify widget was added as a child (accessing through TFlexLayout.TLayout.TResizable.TWidget.Children)
	assert.Len(t, app.TFlexLayout.TLayout.TResizable.TWidget.Children, 1)
	assert.Equal(t, widget, app.TFlexLayout.TLayout.TResizable.TWidget.Children[0])

	// Verify widget was resized to screen size
	pos := widget.GetPos()
	assert.Equal(t, uint16(80), pos.W)
	assert.Equal(t, uint16(24), pos.H)
}

// TestApplicationFlexDirection tests changing flex direction
func TestApplicationFlexDirection(t *testing.T) {
	screen := tio.NewScreenBuffer(100, 50, 0)
	app := NewApplicationEmpty(screen)

	widget1 := &TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 10}}
	widget2 := &TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 10}}

	app.AddChild(widget1)
	app.AddChild(widget2)

	app.SetItemProperties(widget1, FlexItemProperties{FlexGrow: 1.0})
	app.SetItemProperties(widget2, FlexItemProperties{FlexGrow: 1.0})

	// Test row direction (horizontal)
	app.SetDirection(FlexDirectionRow)
	pos1Row := widget1.GetPos()
	pos2Row := widget2.GetPos()

	// In row direction, widgets should be side by side
	assert.Less(t, pos1Row.X, pos2Row.X)
	assert.Equal(t, pos1Row.Y, pos2Row.Y)

	// Test column direction (vertical)
	app.SetDirection(FlexDirectionColumn)
	pos1Col := widget1.GetPos()
	pos2Col := widget2.GetPos()

	// In column direction, widgets should be stacked vertically
	assert.Equal(t, pos1Col.X, pos2Col.X)
	assert.Less(t, pos1Col.Y, pos2Col.Y)
}

// TestApplicationRemoveChild tests removing children from application
func TestApplicationRemoveChild(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	app := NewApplicationEmpty(screen)

	widget1 := &TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 40, H: 24}}
	widget2 := &TWidget{Position: gtv.TRect{X: 40, Y: 0, W: 40, H: 24}}

	app.AddChild(widget1)
	app.AddChild(widget2)

	children := app.TFlexLayout.TLayout.TResizable.TWidget.Children
	assert.Len(t, children, 2)

	// Remove widget1
	app.RemoveChild(widget1)

	// Verify widget was removed
	children = app.TFlexLayout.TLayout.TResizable.TWidget.Children
	assert.Len(t, children, 1)
	assert.Equal(t, widget2, children[0])
}
