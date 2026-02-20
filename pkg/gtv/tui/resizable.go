package tui

import (
	"github.com/rlewczuk/csw/pkg/gtv"
)

// IResizable is an interface for widgets that can handle resize events.
// It extends IWidget with resize-specific functionality.
type IResizable interface {
	IWidget

	// OnResize is called when the widget is resized.
	// Derived widgets can override this to perform custom actions on resize.
	OnResize(oldRect, newRect gtv.TRect)
}

// TResizable is a base widget that provides resize event handling functionality.
// It extends TWidget and implements IResizable interface.
//
// This widget is designed to be embedded in other widgets that need resize event handling,
// such as layouts, containers, and other composite widgets.
//
// Features:
// - Automatic position and size updates on resize events
// - OnResize callback for custom resize handling in derived widgets
// - Proper event propagation to base widget for other event types
type TResizable struct {
	TWidget
}

// NewResizable creates a new resizable widget with the specified position.
// The parent parameter is optional (can be nil for root widgets).
// The rect parameter specifies the position and size of the widget.
//
// Note: When creating a derived widget that embeds TResizable, use newResizableBase
// to avoid double registration with the parent. This constructor registers with the parent.
func NewResizable(parent IWidget, rect gtv.TRect) *TResizable {
	resizable := newResizableBase(parent, rect)

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(resizable)
	}

	return resizable
}

// newResizableBase creates a resizable widget without registering with parent.
// This is used internally by derived widgets to avoid double registration.
func newResizableBase(parent IWidget, rect gtv.TRect) *TResizable {
	return &TResizable{
		TWidget: TWidget{
			Position: rect,
			Parent:   parent,
			Flags:    WidgetFlagNone,
		},
	}
}

// OnResize is called when the widget is resized.
// This is a default implementation that does nothing.
// Derived widgets can override this to perform custom actions on resize.
func (r *TResizable) OnResize(oldRect, newRect gtv.TRect) {
	// Default implementation does nothing
}

// Draw draws the resizable widget on the screen.
// This is a minimal implementation that just draws children.
// Derived widgets should override this method to provide custom rendering.
func (r *TResizable) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if r.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Draw children (if any)
	r.TWidget.Draw(screen)
}

// HandleEvent handles events for the resizable widget.
// This implementation handles resize events and delegates other events to the base widget.
// Derived widgets can override this method to provide custom event handling.
func (r *TResizable) HandleEvent(event *TEvent) {
	// Handle resize events directly
	if event.Type == TEventTypeResize {
		// Store old position for OnResize callback
		oldPos := r.Position

		// Update position
		r.Position.X = event.Rect.X
		r.Position.Y = event.Rect.Y
		r.Position.W = event.Rect.W
		r.Position.H = event.Rect.H

		// Call OnResize callback to allow derived widgets to react
		r.OnResize(oldPos, r.Position)
		return
	}

	// Delegate other events to base widget
	r.TWidget.HandleEvent(event)
}
