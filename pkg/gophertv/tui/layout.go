package tui

import (
	"github.com/codesnort/codesnort-swe/pkg/gophertv"
)

// IAbsoluteLayout is an interface for absolute layout widgets that position children at absolute positions.
// It extends IWidget with absolute layout-specific methods.
type IAbsoluteLayout interface {
	IWidget

	// SetBackground sets the background attributes for the layout.
	// If nil, the layout is transparent and only children are visible.
	SetBackground(attrs *gophertv.CellAttributes)

	// GetBackground returns the current background attributes.
	GetBackground() *gophertv.CellAttributes
}

// TAbsoluteLayout is a widget that positions children at absolute positions (relative to itself).
// It extends TWidget and implements IAbsoluteLayout interface.
//
// The layout:
// - Uses each child widget's Position field to position children
// - Does NOT change child widget's position
// - Does NOT change child widget's size
// - Does NOT wrap child widgets if they don't fit
// - Properly handles widget resize events (triggering redraw of affected child widgets)
// - Properly handles redraw events (triggering full redraw of itself and affected child widgets)
// - Routes input events to affected children (keyboard events to active child, mouse events to children under cursor)
type TAbsoluteLayout struct {
	TWidget

	// Background attributes for the layout. If nil, layout is transparent.
	background *gophertv.CellAttributes
}

// NewAbsoluteLayout creates a new absolute layout widget.
// The parent parameter is optional (can be nil for root widgets).
// The rect parameter specifies the position and size of the layout.
// The background parameter specifies background attributes. If nil, layout is transparent.
func NewAbsoluteLayout(parent IWidget, rect gophertv.TRect, background *gophertv.CellAttributes) *TAbsoluteLayout {
	layout := &TAbsoluteLayout{
		TWidget: TWidget{
			Position: rect,
			Parent:   parent,
		},
		background: background,
	}

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(layout)
	}

	return layout
}

// AbsoluteLayout returns the IAbsoluteLayout interface for the given TAbsoluteLayout instance.
func AbsoluteLayout(layout *TAbsoluteLayout) IAbsoluteLayout {
	return layout
}

// SetBackground sets the background attributes for the layout.
// If nil, the layout is transparent and only children are visible.
func (l *TAbsoluteLayout) SetBackground(attrs *gophertv.CellAttributes) {
	l.background = attrs
}

// GetBackground returns the current background attributes.
func (l *TAbsoluteLayout) GetBackground() *gophertv.CellAttributes {
	return l.background
}

// Draw draws the layout on the screen.
// If background is set, it fills the layout area with background color first.
// Then it draws all children at their absolute positions.
func (l *TAbsoluteLayout) Draw(screen gophertv.IScreenOutput) {
	// Don't draw if hidden
	if l.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Get absolute position
	absPos := l.GetAbsolutePos()

	// Draw background if set
	if l.background != nil {
		// Fill the background with spaces using the background attributes
		for y := uint16(0); y < absPos.H; y++ {
			for x := uint16(0); x < absPos.W; x++ {
				screen.PutContent(
					gophertv.TRect{X: absPos.X + x, Y: absPos.Y + y, W: 1, H: 1},
					[]gophertv.Cell{{Rune: ' ', Attrs: *l.background}},
				)
			}
		}
	}

	// Draw children at their absolute positions
	// Children use their own Position field which is already absolute relative to the layout
	l.TWidget.Draw(screen)
}

// HandleEvent handles events for the layout.
// It handles resize events, redraw events, and routes input events to children.
func (l *TAbsoluteLayout) HandleEvent(event *TEvent) {
	// Handle resize events directly
	if event.Type == TEventTypeResize {
		// Store old position for comparison
		oldPos := l.Position

		// Update position
		l.Position.X = event.Rect.X
		l.Position.Y = event.Rect.Y
		l.Position.W = event.Rect.W
		l.Position.H = event.Rect.H

		// If size changed, trigger redraw of all children
		if oldPos.W != l.Position.W || oldPos.H != l.Position.H {
			redrawEvent := &TEvent{Type: TEventTypeRedraw}
			for _, child := range l.Children {
				child.HandleEvent(redrawEvent)
			}
		}
		return
	}

	// Handle redraw events by propagating to all children
	if event.Type == TEventTypeRedraw {
		for _, child := range l.Children {
			child.HandleEvent(event)
		}
		return
	}

	// Handle input events
	if event.Type == TEventTypeInput && event.InputEvent != nil {
		inputEvent := event.InputEvent

		// Handle mouse events - route to child under cursor
		if inputEvent.Type == gophertv.InputEventMouse {
			// Get absolute position of layout
			absPos := l.GetAbsolutePos()

			// Check if mouse is within layout bounds
			if inputEvent.X >= absPos.X && inputEvent.X < absPos.X+absPos.W &&
				inputEvent.Y >= absPos.Y && inputEvent.Y < absPos.Y+absPos.H {

				// Calculate mouse position relative to layout
				relativeX := inputEvent.X - absPos.X
				relativeY := inputEvent.Y - absPos.Y

				// Find child widget under cursor (iterate in reverse order to handle overlapping widgets)
				for i := len(l.Children) - 1; i >= 0; i-- {
					child := l.Children[i]

					// Get child's position relative to this layout (parent)
					childPos := child.GetPos()

					// Check if mouse is within child bounds using relative coordinates
					if relativeX >= childPos.X && relativeX < childPos.X+childPos.W &&
						relativeY >= childPos.Y && relativeY < childPos.Y+childPos.H {
						// Route event to this child
						child.HandleEvent(event)
						return
					}
				}
			}
			return
		}

		// Handle keyboard events - route to active child
		if inputEvent.Type == gophertv.InputEventKey {
			if l.ActiveChild != nil {
				l.ActiveChild.HandleEvent(event)
			}
			return
		}

		// Handle other input events (focus, blur, etc.) - route to active child
		if l.ActiveChild != nil {
			l.ActiveChild.HandleEvent(event)
		}
		return
	}

	// For other event types, delegate to base widget
	l.TWidget.HandleEvent(event)
}
