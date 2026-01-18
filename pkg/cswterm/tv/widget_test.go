package tv

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/codesnort/codesnort-swe/pkg/cswterm"
	"github.com/codesnort/codesnort-swe/pkg/cswterm/term"
)

// mockWidget is a test double for IWidget that tracks method calls
type mockWidget struct {
	TWidget
	drawCalled        bool
	handleEventCalled bool
	lastEvent         *TEvent
}

func (m *mockWidget) Draw(screen cswterm.IScreenOutput) {
	m.drawCalled = true
	// Also call the base implementation to test composition
	m.TWidget.Draw(screen)
}

func (m *mockWidget) HandleEvent(event *TEvent) {
	m.handleEventCalled = true
	m.lastEvent = event
	// Also call the base implementation to test composition
	m.TWidget.HandleEvent(event)
}

func TestTWidget_Draw_NoChildren(t *testing.T) {
	// Create a widget with no children
	widget := &TWidget{
		Position: WidgetPosition{X: 0, Y: 0, W: 10, H: 5},
	}

	// Create a mock screen
	screen := term.NewScreenBuffer(80, 24, 0)

	// Draw should not panic when there are no children
	widget.Draw(screen)

	// Test passes if no panic occurred
}

func TestTWidget_Draw_WithChildren(t *testing.T) {
	// Create child widgets
	child1 := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 0, Y: 0, W: 10, H: 5},
		},
	}
	child2 := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 10, Y: 0, W: 10, H: 5},
		},
	}
	child3 := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 20, Y: 0, W: 10, H: 5},
		},
	}

	// Create parent widget with children
	parent := &TWidget{
		Position: WidgetPosition{X: 0, Y: 0, W: 30, H: 5},
		Children: []IWidget{child1, child2, child3},
	}

	// Create a mock screen
	screen := term.NewScreenBuffer(80, 24, 0)

	// Draw the parent
	parent.Draw(screen)

	// Verify that Draw was called on all children
	assert.True(t, child1.drawCalled, "Draw should be called on child1")
	assert.True(t, child2.drawCalled, "Draw should be called on child2")
	assert.True(t, child3.drawCalled, "Draw should be called on child3")
}

func TestTWidget_Draw_WithNestedChildren(t *testing.T) {
	// Create nested structure: parent -> child1 -> grandchild
	grandchild := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 0, Y: 0, W: 5, H: 3},
		},
	}

	child1 := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 0, Y: 0, W: 10, H: 5},
			Children: []IWidget{grandchild},
		},
	}

	child2 := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 10, Y: 0, W: 10, H: 5},
		},
	}

	parent := &TWidget{
		Position: WidgetPosition{X: 0, Y: 0, W: 30, H: 5},
		Children: []IWidget{child1, child2},
	}

	// Create a mock screen
	screen := term.NewScreenBuffer(80, 24, 0)

	// Draw the parent
	parent.Draw(screen)

	// Verify that Draw was called on all widgets in the hierarchy
	assert.True(t, child1.drawCalled, "Draw should be called on child1")
	assert.True(t, child2.drawCalled, "Draw should be called on child2")
	assert.True(t, grandchild.drawCalled, "Draw should be called on grandchild")
}

func TestTWidget_HandleEvent_NoActiveChild(t *testing.T) {
	// Create a widget with no active child
	widget := &TWidget{
		Position: WidgetPosition{X: 0, Y: 0, W: 10, H: 5},
	}

	// Create an event
	event := &TEvent{
		Type: TEventTypeInput,
		InputEvent: &cswterm.InputEvent{
			Type: cswterm.InputEventKey,
		},
	}

	// HandleEvent should not panic when there's no active child
	widget.HandleEvent(event)

	// Test passes if no panic occurred
}

func TestTWidget_HandleEvent_WithActiveChild(t *testing.T) {
	// Create an active child widget
	activeChild := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 0, Y: 0, W: 10, H: 5},
		},
	}

	// Create parent widget with active child
	parent := &TWidget{
		Position:    WidgetPosition{X: 0, Y: 0, W: 30, H: 5},
		ActiveChild: activeChild,
	}

	// Create an event
	event := &TEvent{
		Type: TEventTypeInput,
		InputEvent: &cswterm.InputEvent{
			Type: cswterm.InputEventKey,
		},
	}

	// Handle the event
	parent.HandleEvent(event)

	// Verify that HandleEvent was called on the active child
	assert.True(t, activeChild.handleEventCalled, "HandleEvent should be called on active child")
	assert.Equal(t, event, activeChild.lastEvent, "Event should be passed to active child")
}

func TestTWidget_HandleEvent_MultipleEvents(t *testing.T) {
	// Create an active child widget
	activeChild := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 0, Y: 0, W: 10, H: 5},
		},
	}

	// Create parent widget with active child
	parent := &TWidget{
		Position:    WidgetPosition{X: 0, Y: 0, W: 30, H: 5},
		ActiveChild: activeChild,
	}

	// Create multiple events
	event1 := &TEvent{Type: TEventTypeInput}
	event2 := &TEvent{Type: TEventTypeTimer}
	event3 := &TEvent{Type: TEventTypeRedraw}

	// Handle events
	parent.HandleEvent(event1)
	assert.Equal(t, event1, activeChild.lastEvent, "First event should be passed to active child")

	parent.HandleEvent(event2)
	assert.Equal(t, event2, activeChild.lastEvent, "Second event should be passed to active child")

	parent.HandleEvent(event3)
	assert.Equal(t, event3, activeChild.lastEvent, "Third event should be passed to active child")
}

func TestTWidget_HandleEvent_ChildrenButNoActiveChild(t *testing.T) {
	// Create child widgets
	child1 := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 0, Y: 0, W: 10, H: 5},
		},
	}
	child2 := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 10, Y: 0, W: 10, H: 5},
		},
	}

	// Create parent with children but no active child
	parent := &TWidget{
		Position:    WidgetPosition{X: 0, Y: 0, W: 30, H: 5},
		Children:    []IWidget{child1, child2},
		ActiveChild: nil,
	}

	// Create an event
	event := &TEvent{Type: TEventTypeInput}

	// Handle the event
	parent.HandleEvent(event)

	// Verify that HandleEvent was NOT called on children
	assert.False(t, child1.handleEventCalled, "HandleEvent should not be called on child1 when not active")
	assert.False(t, child2.handleEventCalled, "HandleEvent should not be called on child2 when not active")
}

func TestTWidget_GetAbsolutePosition_NoParent(t *testing.T) {
	// Create a widget with no parent
	widget := &TWidget{
		Position: WidgetPosition{X: 10, Y: 20, W: 30, H: 40},
	}

	// Get absolute position
	absPos := widget.GetAbsolutePosition()

	// Should return the widget's own position
	assert.Equal(t, 10, absPos.X, "X should match widget position")
	assert.Equal(t, 20, absPos.Y, "Y should match widget position")
	assert.Equal(t, 30, absPos.W, "W should match widget position")
	assert.Equal(t, 40, absPos.H, "H should match widget position")
}

func TestTWidget_GetAbsolutePosition_WithParent(t *testing.T) {
	// Create parent widget
	parent := &TWidget{
		Position: WidgetPosition{X: 10, Y: 20, W: 100, H: 100},
	}

	// Create child widget
	child := &TWidget{
		Position: WidgetPosition{X: 5, Y: 10, W: 30, H: 40},
		Parent:   parent,
	}

	// Get absolute position
	absPos := child.GetAbsolutePosition()

	// Should return parent position + child position
	assert.Equal(t, 15, absPos.X, "X should be parent.X + child.X")
	assert.Equal(t, 30, absPos.Y, "Y should be parent.Y + child.Y")
	assert.Equal(t, 30, absPos.W, "W should match child position")
	assert.Equal(t, 40, absPos.H, "H should match child position")
}

func TestTWidget_GetAbsolutePosition_NestedParents(t *testing.T) {
	// Create grandparent widget
	grandparent := &TWidget{
		Position: WidgetPosition{X: 10, Y: 20, W: 200, H: 200},
	}

	// Create parent widget
	parent := &TWidget{
		Position: WidgetPosition{X: 5, Y: 10, W: 100, H: 100},
		Parent:   grandparent,
	}

	// Create child widget
	child := &TWidget{
		Position: WidgetPosition{X: 3, Y: 7, W: 30, H: 40},
		Parent:   parent,
	}

	// Get absolute position
	absPos := child.GetAbsolutePosition()

	// Should return grandparent.X + parent.X + child.X
	assert.Equal(t, 18, absPos.X, "X should be sum of all parent X positions")
	assert.Equal(t, 37, absPos.Y, "Y should be sum of all parent Y positions")
	assert.Equal(t, 30, absPos.W, "W should match child position")
	assert.Equal(t, 40, absPos.H, "H should match child position")
}

func TestTWidget_HandleEvent_PositionEvent(t *testing.T) {
	// Create a widget with initial position
	widget := &TWidget{
		Position: WidgetPosition{X: 10, Y: 20, W: 30, H: 40},
	}

	// Create a position event with new values
	event := &TEvent{
		Type: TEventTypePosition,
		X:    50,
		Y:    60,
		W:    70,
		H:    80,
	}

	// Handle the position event
	widget.HandleEvent(event)

	// Verify that the widget's position was updated
	assert.Equal(t, 50, widget.Position.X, "X should be updated to event.X")
	assert.Equal(t, 60, widget.Position.Y, "Y should be updated to event.Y")
	assert.Equal(t, 70, widget.Position.W, "W should be updated to event.W")
	assert.Equal(t, 80, widget.Position.H, "H should be updated to event.H")
}

func TestTWidget_HandleEvent_PositionEvent_NotPropagatedToChildren(t *testing.T) {
	// Create an active child widget
	activeChild := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 5, Y: 10, W: 15, H: 20},
		},
	}

	// Create parent widget with active child
	parent := &TWidget{
		Position:    WidgetPosition{X: 10, Y: 20, W: 30, H: 40},
		ActiveChild: activeChild,
	}

	// Create a position event
	event := &TEvent{
		Type: TEventTypePosition,
		X:    50,
		Y:    60,
		W:    70,
		H:    80,
	}

	// Handle the position event
	parent.HandleEvent(event)

	// Verify that the parent's position was updated
	assert.Equal(t, 50, parent.Position.X, "Parent X should be updated")
	assert.Equal(t, 60, parent.Position.Y, "Parent Y should be updated")
	assert.Equal(t, 70, parent.Position.W, "Parent W should be updated")
	assert.Equal(t, 80, parent.Position.H, "Parent H should be updated")

	// Verify that the event was NOT propagated to the active child
	assert.False(t, activeChild.handleEventCalled, "Position event should not be propagated to children")

	// Verify that the child's position remains unchanged
	assert.Equal(t, 5, activeChild.Position.X, "Child X should remain unchanged")
	assert.Equal(t, 10, activeChild.Position.Y, "Child Y should remain unchanged")
	assert.Equal(t, 15, activeChild.Position.W, "Child W should remain unchanged")
	assert.Equal(t, 20, activeChild.Position.H, "Child H should remain unchanged")
}

func TestTWidget_HandleEvent_PositionEvent_WithMultipleChildren(t *testing.T) {
	// Create multiple child widgets
	child1 := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 0, Y: 0, W: 10, H: 10},
		},
	}
	child2 := &mockWidget{
		TWidget: TWidget{
			Position: WidgetPosition{X: 10, Y: 0, W: 10, H: 10},
		},
	}

	// Create parent widget with children and an active child
	parent := &TWidget{
		Position:    WidgetPosition{X: 10, Y: 20, W: 30, H: 40},
		Children:    []IWidget{child1, child2},
		ActiveChild: child1,
	}

	// Create a position event
	event := &TEvent{
		Type: TEventTypePosition,
		X:    100,
		Y:    200,
		W:    300,
		H:    400,
	}

	// Handle the position event
	parent.HandleEvent(event)

	// Verify that the parent's position was updated
	assert.Equal(t, 100, parent.Position.X, "Parent X should be updated")
	assert.Equal(t, 200, parent.Position.Y, "Parent Y should be updated")
	assert.Equal(t, 300, parent.Position.W, "Parent W should be updated")
	assert.Equal(t, 400, parent.Position.H, "Parent H should be updated")

	// Verify that no children received the event
	assert.False(t, child1.handleEventCalled, "Position event should not be propagated to child1")
	assert.False(t, child2.handleEventCalled, "Position event should not be propagated to child2")
}
