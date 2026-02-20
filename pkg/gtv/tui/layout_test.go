package tui

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/rlewczuk/csw/pkg/gtv/tio"

	"github.com/stretchr/testify/assert"
)

func TestNewLayout_BasicCreation(t *testing.T) {
	// Create layout with background
	bgAttrs := gtv.CellColor(0xFF0000).WithBackColor(0x00FF00)
	rect := gtv.TRect{X: 10, Y: 5, W: 20, H: 10}
	layout := NewLayout(nil, rect, &bgAttrs)

	assert.NotNil(t, layout, "Layout should be created")
	assert.Equal(t, uint16(10), layout.Position.X, "X position should match")
	assert.Equal(t, uint16(5), layout.Position.Y, "Y position should match")
	assert.Equal(t, uint16(20), layout.Position.W, "Width should match")
	assert.Equal(t, uint16(10), layout.Position.H, "Height should match")
	assert.Nil(t, layout.Parent, "Parent should be nil")
	assert.NotNil(t, layout.GetBackground(), "Background should be set")
	assert.Equal(t, bgAttrs, *layout.GetBackground(), "Background attributes should match")
}

func TestNewLayout_TransparentBackground(t *testing.T) {
	// Create layout without background (transparent)
	rect := gtv.TRect{X: 0, Y: 0, W: 10, H: 5}
	layout := NewLayout(nil, rect, nil)

	assert.NotNil(t, layout, "Layout should be created")
	assert.Nil(t, layout.GetBackground(), "Background should be nil for transparent layout")
}

func TestNewLayout_WithParent(t *testing.T) {
	// Create parent widget
	parent := &TWidget{
		Position: gtv.TRect{X: 10, Y: 20, W: 100, H: 100},
	}

	// Create child layout
	rect := gtv.TRect{X: 5, Y: 10, W: 50, H: 40}
	layout := NewLayout(parent, rect, nil)

	assert.NotNil(t, layout, "Layout should be created")
	assert.Equal(t, parent, layout.Parent, "Parent should be set")
	assert.Len(t, parent.Children, 1, "Parent should have one child")
	assert.Equal(t, layout, parent.Children[0], "Child should be the layout")
}

func TestTLayout_SetBackground(t *testing.T) {
	layout := NewLayout(nil, gtv.TRect{X: 0, Y: 0, W: 10, H: 5}, nil)
	assert.Nil(t, layout.GetBackground(), "Initial background should be nil")

	newBg := gtv.CellBackground(0xFF0000)
	layout.SetBackground(&newBg)
	assert.NotNil(t, layout.GetBackground(), "Background should be set")
	assert.Equal(t, newBg, *layout.GetBackground(), "Background should match")

	layout.SetBackground(nil)
	assert.Nil(t, layout.GetBackground(), "Background should be nil after clearing")
}

func TestTLayout_Draw_TransparentBackground(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create layout without background
	layout := NewLayout(nil, gtv.TRect{X: 5, Y: 10, W: 20, H: 5}, nil)

	// Draw layout
	layout.Draw(screen)

	// Verify that background is not drawn (all spaces)
	_, _, content := screen.GetContent()
	verifier := gtv.NewScreenVerifier(80, 24, content)

	// Check a few cells in the layout area - should all be empty spaces
	for x := 5; x < 25; x++ {
		for y := 10; y < 15; y++ {
			cell := verifier.GetCell(x, y)
			assert.Equal(t, ' ', cell.Rune, "Cell should be space")
		}
	}
}

func TestTLayout_Draw_WithBackground(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create layout with background
	bgAttrs := gtv.CellBackground(0xFF0000)
	layout := NewLayout(nil, gtv.TRect{X: 5, Y: 10, W: 10, H: 3}, &bgAttrs)

	// Draw layout
	layout.Draw(screen)

	// Verify background is drawn
	_, _, content := screen.GetContent()
	verifier := gtv.NewScreenVerifier(80, 24, content)

	// Check that all cells in layout area have background color
	for x := 5; x < 15; x++ {
		for y := 10; y < 13; y++ {
			cell := verifier.GetCell(x, y)
			assert.Equal(t, ' ', cell.Rune, "Cell should be space")
			assert.Equal(t, gtv.TextColor(0xFF0000), cell.Attrs.BackColor, "Cell should have background color")
		}
	}
}

func TestTLayout_Draw_Hidden(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create layout with background
	bgAttrs := gtv.CellBackground(0xFF0000)
	layout := NewLayout(nil, gtv.TRect{X: 0, Y: 0, W: 10, H: 5}, &bgAttrs)
	layout.Flags = WidgetFlagHidden

	// Draw layout
	layout.Draw(screen)

	// Verify nothing is drawn
	_, _, content := screen.GetContent()
	verifier := gtv.NewScreenVerifier(80, 24, content)

	// Check that cells don't have the background color
	for x := 0; x < 10; x++ {
		for y := 0; y < 5; y++ {
			cell := verifier.GetCell(x, y)
			assert.NotEqual(t, gtv.TextColor(0xFF0000), cell.Attrs.BackColor, "Hidden layout should not draw")
		}
	}
}

func TestTLayout_Draw_WithChildren(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create layout
	layout := NewLayout(nil, gtv.TRect{X: 10, Y: 10, W: 30, H: 10}, nil)

	// Create child labels at different positions
	_ = NewLabel(layout, "Label1", gtv.TRect{X: 2, Y: 2, W: 6, H: 1}, gtv.CellAttributes{})
	_ = NewLabel(layout, "Label2", gtv.TRect{X: 10, Y: 5, W: 6, H: 1}, gtv.CellAttributes{})

	// Draw layout
	layout.Draw(screen)

	// Verify children are drawn at correct absolute positions
	_, _, content := screen.GetContent()
	verifier := gtv.NewScreenVerifier(80, 24, content)

	// Label1 should be at layout position (10,10) + child position (2,2) = (12,12)
	assert.True(t, verifier.HasText(12, 12, 6, 1, "Label1"), "Label1 should be at absolute position")

	// Label2 should be at layout position (10,10) + child position (10,5) = (20,15)
	assert.True(t, verifier.HasText(20, 15, 6, 1, "Label2"), "Label2 should be at absolute position")
}

func TestTLayout_Draw_WithBackgroundAndChildren(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create layout with background
	bgAttrs := gtv.CellBackground(0x0000FF)
	layout := NewLayout(nil, gtv.TRect{X: 5, Y: 5, W: 20, H: 10}, &bgAttrs)

	// Create child label
	attrs := gtv.CellColor(0xFF0000)
	_ = NewLabel(layout, "Test", gtv.TRect{X: 5, Y: 3, W: 4, H: 1}, attrs)

	// Draw layout
	layout.Draw(screen)

	// Verify background is drawn
	_, _, content := screen.GetContent()
	verifier := gtv.NewScreenVerifier(80, 24, content)

	// Check background in an area without children
	cell := verifier.GetCell(7, 7)
	assert.Equal(t, gtv.TextColor(0x0000FF), cell.Attrs.BackColor, "Background should be drawn")

	// Check that child is drawn on top
	assert.True(t, verifier.HasText(10, 8, 4, 1, "Test"), "Child should be drawn at correct position")
}

func TestTLayout_HandleEvent_Resize(t *testing.T) {
	// Create layout with initial size
	layout := NewLayout(nil, gtv.TRect{X: 10, Y: 20, W: 30, H: 40}, nil)

	// Create resize event
	event := &TEvent{
		Type: TEventTypeResize,
		Rect: gtv.TRect{X: 50, Y: 60, W: 70, H: 80},
	}

	// Handle event
	layout.HandleEvent(event)

	// Verify position was updated
	assert.Equal(t, uint16(50), layout.Position.X, "X should be updated")
	assert.Equal(t, uint16(60), layout.Position.Y, "Y should be updated")
	assert.Equal(t, uint16(70), layout.Position.W, "W should be updated")
	assert.Equal(t, uint16(80), layout.Position.H, "H should be updated")
}

func TestTLayout_HandleEvent_ResizeTriggersChildRedraw(t *testing.T) {
	// Create layout
	layout := NewLayout(nil, gtv.TRect{X: 0, Y: 0, W: 30, H: 40}, nil)

	// Create mock children
	child1 := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 10}}}
	child2 := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 10, Y: 10, W: 10, H: 10}}}
	layout.Children = []IWidget{child1, child2}

	// Create resize event with size change
	event := &TEvent{
		Type: TEventTypeResize,
		Rect: gtv.TRect{X: 0, Y: 0, W: 50, H: 60}, // Size changed
	}

	// Handle event
	layout.HandleEvent(event)

	// Verify that children received redraw events
	assert.True(t, child1.handleEventCalled, "Child1 should receive redraw event")
	assert.True(t, child2.handleEventCalled, "Child2 should receive redraw event")
	assert.Equal(t, TEventTypeRedraw, child1.lastEvent.Type, "Child1 should receive redraw event type")
	assert.Equal(t, TEventTypeRedraw, child2.lastEvent.Type, "Child2 should receive redraw event type")
}

func TestTLayout_HandleEvent_ResizeNoSizeChange(t *testing.T) {
	// Create layout
	layout := NewLayout(nil, gtv.TRect{X: 10, Y: 20, W: 30, H: 40}, nil)

	// Create mock children
	child := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 10}}}
	layout.Children = []IWidget{child}

	// Create resize event with only position change (no size change)
	event := &TEvent{
		Type: TEventTypeResize,
		Rect: gtv.TRect{X: 50, Y: 60, W: 30, H: 40}, // Same size, different position
	}

	// Handle event
	layout.HandleEvent(event)

	// Verify that children did NOT receive redraw events
	assert.False(t, child.handleEventCalled, "Child should not receive event if size unchanged")
}

func TestTLayout_HandleEvent_Redraw(t *testing.T) {
	// Create layout
	layout := NewLayout(nil, gtv.TRect{X: 0, Y: 0, W: 30, H: 40}, nil)

	// Create mock children
	child1 := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 10}}}
	child2 := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 10, Y: 10, W: 10, H: 10}}}
	layout.Children = []IWidget{child1, child2}

	// Create redraw event
	event := &TEvent{Type: TEventTypeRedraw}

	// Handle event
	layout.HandleEvent(event)

	// Verify that all children received redraw events
	assert.True(t, child1.handleEventCalled, "Child1 should receive redraw event")
	assert.True(t, child2.handleEventCalled, "Child2 should receive redraw event")
	assert.Equal(t, TEventTypeRedraw, child1.lastEvent.Type, "Event type should be redraw")
	assert.Equal(t, TEventTypeRedraw, child2.lastEvent.Type, "Event type should be redraw")
}

func TestTLayout_HandleEvent_KeyboardToActiveChild(t *testing.T) {
	// Create layout
	layout := NewLayout(nil, gtv.TRect{X: 0, Y: 0, W: 30, H: 40}, nil)

	// Create children
	child1 := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 10}}}
	child2 := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 10, Y: 10, W: 10, H: 10}}}
	layout.Children = []IWidget{child1, child2}
	layout.ActiveChild = child1

	// Create keyboard event
	event := &TEvent{
		Type: TEventTypeInput,
		InputEvent: &gtv.InputEvent{
			Type: gtv.InputEventKey,
			Key:  'a',
		},
	}

	// Handle event
	layout.HandleEvent(event)

	// Verify only active child received the event
	assert.True(t, child1.handleEventCalled, "Active child should receive keyboard event")
	assert.False(t, child2.handleEventCalled, "Non-active child should not receive event")
}

func TestTLayout_HandleEvent_NoActiveChild(t *testing.T) {
	// Create layout with no active child
	layout := NewLayout(nil, gtv.TRect{X: 0, Y: 0, W: 30, H: 40}, nil)

	// Create child
	child := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 10}}}
	layout.Children = []IWidget{child}
	// Explicitly set no active child
	layout.ActiveChild = nil

	// Create keyboard event
	event := &TEvent{
		Type: TEventTypeInput,
		InputEvent: &gtv.InputEvent{
			Type: gtv.InputEventKey,
			Key:  'a',
		},
	}

	// Handle event (should not panic)
	layout.HandleEvent(event)

	// Verify child did not receive the event
	assert.False(t, child.handleEventCalled, "Child should not receive event without being active")
}

func TestTLayout_HandleEvent_FocusEvents(t *testing.T) {
	// Create layout
	layout := NewLayout(nil, gtv.TRect{X: 0, Y: 0, W: 30, H: 40}, nil)

	// Create active child
	child := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 10}}}
	layout.ActiveChild = child

	// Create focus event
	event := &TEvent{
		Type: TEventTypeInput,
		InputEvent: &gtv.InputEvent{
			Type: gtv.InputEventFocus,
		},
	}

	// Handle event
	layout.HandleEvent(event)

	// Verify active child received the event
	assert.True(t, child.handleEventCalled, "Active child should receive focus event")
}

func TestTLayout_GetChildAt_OutsideBounds(t *testing.T) {
	// Create layout at position (10, 10) with size (20, 20)
	layout := NewLayout(nil, gtv.TRect{X: 10, Y: 10, W: 20, H: 20}, nil)

	// Test coordinates outside layout bounds
	child := layout.GetChildAt(5, 5) // Before layout
	assert.Nil(t, child, "Should return nil for coordinates outside layout")

	child = layout.GetChildAt(35, 35) // After layout
	assert.Nil(t, child, "Should return nil for coordinates outside layout")

	child = layout.GetChildAt(15, 5) // X inside, Y outside
	assert.Nil(t, child, "Should return nil for coordinates outside layout")

	child = layout.GetChildAt(5, 15) // X outside, Y inside
	assert.Nil(t, child, "Should return nil for coordinates outside layout")
}

func TestTLayout_GetChildAt_InsideBounds(t *testing.T) {
	// Create layout at position (10, 10) with size (20, 20)
	layout := NewLayout(nil, gtv.TRect{X: 10, Y: 10, W: 20, H: 20}, nil)

	// Test coordinates inside layout bounds
	// Base TLayout doesn't implement child hit-testing, so it should return nil
	child := layout.GetChildAt(15, 15)
	assert.Nil(t, child, "Base TLayout should return nil even for coordinates inside layout")

	child = layout.GetChildAt(10, 10) // Top-left corner
	assert.Nil(t, child, "Base TLayout should return nil even for coordinates inside layout")

	child = layout.GetChildAt(29, 29) // Bottom-right corner (inside)
	assert.Nil(t, child, "Base TLayout should return nil even for coordinates inside layout")
}

func TestTLayout_GetAbsolutePos_NoParent(t *testing.T) {
	layout := NewLayout(nil, gtv.TRect{X: 10, Y: 20, W: 30, H: 40}, nil)

	absPos := layout.GetAbsolutePos()
	assert.Equal(t, uint16(10), absPos.X, "X should match layout position")
	assert.Equal(t, uint16(20), absPos.Y, "Y should match layout position")
	assert.Equal(t, uint16(30), absPos.W, "W should match layout position")
	assert.Equal(t, uint16(40), absPos.H, "H should match layout position")
}

func TestTLayout_GetAbsolutePos_WithParent(t *testing.T) {
	// Create parent widget
	parent := &TWidget{
		Position: gtv.TRect{X: 10, Y: 20, W: 100, H: 100},
	}

	// Create child layout
	layout := NewLayout(parent, gtv.TRect{X: 5, Y: 10, W: 30, H: 40}, nil)

	absPos := layout.GetAbsolutePos()
	assert.Equal(t, uint16(15), absPos.X, "X should be parent.X + layout.X")
	assert.Equal(t, uint16(30), absPos.Y, "Y should be parent.Y + layout.Y")
	assert.Equal(t, uint16(30), absPos.W, "W should match layout position")
	assert.Equal(t, uint16(40), absPos.H, "H should match layout position")
}

func TestTLayout_InterfaceCompliance(t *testing.T) {
	// Verify that TLayout implements ILayout interface
	var _ ILayout = (*TLayout)(nil)
}

func TestTLayout_ComplexScenario(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout with background
	bgAttrs := gtv.CellBackground(0x111111)
	mainLayout := NewLayout(nil, gtv.TRect{X: 5, Y: 5, W: 40, H: 15}, &bgAttrs)

	// Create multiple labels at different positions
	label1Attrs := gtv.CellColor(0xFF0000)
	_ = NewLabel(mainLayout, "Header", gtv.TRect{X: 2, Y: 1, W: 6, H: 1}, label1Attrs)

	label2Attrs := gtv.CellColor(0x00FF00)
	_ = NewLabel(mainLayout, "Content", gtv.TRect{X: 2, Y: 5, W: 7, H: 1}, label2Attrs)

	label3Attrs := gtv.CellColor(0x0000FF)
	_ = NewLabel(mainLayout, "Footer", gtv.TRect{X: 2, Y: 13, W: 6, H: 1}, label3Attrs)

	// Draw everything
	mainLayout.Draw(screen)

	// Verify background is drawn
	_, _, content := screen.GetContent()
	verifier := gtv.NewScreenVerifier(80, 24, content)

	// Check background color in empty area (not covered by any label)
	// Layout is at (5, 5), so check position (15, 15) which is offset (10, 10) from layout origin
	// This position is not covered by any of the labels
	cell := verifier.GetCell(15, 15)
	assert.Equal(t, gtv.TextColor(0x111111), cell.Attrs.BackColor, "Background should be drawn")

	// Verify all labels are at correct positions
	// label1 at (5+2, 5+1) = (7, 6)
	assert.True(t, verifier.HasText(7, 6, 6, 1, "Header"), "Header should be at correct position")

	// label2 at (5+2, 5+5) = (7, 10)
	assert.True(t, verifier.HasText(7, 10, 7, 1, "Content"), "Content should be at correct position")

	// label3 at (5+2, 5+13) = (7, 18)
	assert.True(t, verifier.HasText(7, 18, 6, 1, "Footer"), "Footer should be at correct position")
}

func TestTLayout_NestedLayouts(t *testing.T) {
	// Create screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout
	mainLayout := NewLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create nested layout with background
	bgAttrs := gtv.CellBackground(0x222222)
	nestedLayout := NewLayout(mainLayout, gtv.TRect{X: 10, Y: 10, W: 20, H: 10}, &bgAttrs)

	// Create label in nested layout
	labelAttrs := gtv.CellColor(0xFFFFFF)
	_ = NewLabel(nestedLayout, "Nested", gtv.TRect{X: 5, Y: 5, W: 6, H: 1}, labelAttrs)

	// Draw everything
	mainLayout.Draw(screen)

	// Verify nested layout background
	_, _, content := screen.GetContent()
	verifier := gtv.NewScreenVerifier(80, 24, content)

	// Check background in nested layout area
	// Position (15, 15) will be covered by the label, so check position (12, 12) instead
	cell := verifier.GetCell(12, 12)
	assert.Equal(t, gtv.TextColor(0x222222), cell.Attrs.BackColor, "Nested layout background should be drawn")

	// Verify label in nested layout is at correct absolute position
	// Nested layout at (10, 10), label at (5, 5) relative to nested layout = (15, 15) absolute
	assert.True(t, verifier.HasText(15, 15, 6, 1, "Nested"), "Label in nested layout should be at correct position")
}

func TestTLayout_HandleEvent_MouseEventUsesGetChildAt(t *testing.T) {
	// Create layout with a child
	layout := NewLayout(nil, gtv.TRect{X: 10, Y: 10, W: 30, H: 40}, nil)

	// Create mock child
	child := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 10}}}
	child.Parent = layout
	layout.Children = []IWidget{child}

	// Create mouse event inside child bounds (relative to layout: 0-10, 0-10; absolute: 10-20, 10-20)
	event := &TEvent{
		Type: TEventTypeInput,
		InputEvent: &gtv.InputEvent{
			Type: gtv.InputEventMouse,
			X:    15, // Inside child (absolute position 15 = layout 10 + child relative 5)
			Y:    15, // Inside child (absolute position 15 = layout 10 + child relative 5)
		},
	}

	// Handle event
	layout.HandleEvent(event)

	// The base TLayout.GetChildAt now performs proper hit-testing with unified logic
	// This allows standard layouts to route mouse events to children without needing to override
	assert.True(t, child.handleEventCalled, "Base TLayout should route mouse events to children under cursor")
}

func TestTLayout_MultipleChildrenEventRouting(t *testing.T) {
	// Create layout
	layout := NewLayout(nil, gtv.TRect{X: 0, Y: 0, W: 50, H: 50}, nil)

	// Create multiple children
	child1 := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 0, Y: 0, W: 10, H: 10}}}
	child2 := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 10, Y: 10, W: 10, H: 10}}}
	child3 := &mockWidget{TWidget: TWidget{Position: gtv.TRect{X: 20, Y: 20, W: 10, H: 10}}}
	layout.Children = []IWidget{child1, child2, child3}

	// Test redraw event propagates to all children
	redrawEvent := &TEvent{Type: TEventTypeRedraw}
	layout.HandleEvent(redrawEvent)

	assert.True(t, child1.handleEventCalled, "All children should receive redraw event")
	assert.True(t, child2.handleEventCalled, "All children should receive redraw event")
	assert.True(t, child3.handleEventCalled, "All children should receive redraw event")

	// Reset mock widgets
	child1.handleEventCalled = false
	child2.handleEventCalled = false
	child3.handleEventCalled = false

	// Test keyboard event goes only to active child
	layout.ActiveChild = child2
	keyEvent := &TEvent{
		Type: TEventTypeInput,
		InputEvent: &gtv.InputEvent{
			Type: gtv.InputEventKey,
			Key:  'a',
		},
	}
	layout.HandleEvent(keyEvent)

	assert.False(t, child1.handleEventCalled, "Non-active child should not receive keyboard event")
	assert.True(t, child2.handleEventCalled, "Active child should receive keyboard event")
	assert.False(t, child3.handleEventCalled, "Non-active child should not receive keyboard event")
}
