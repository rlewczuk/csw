package tv

import "github.com/codesnort/codesnort-swe/pkg/cswterm"

// WidgetPosition represents position and size of the widget.
// Depending on context, position can be absolute (i.e. relative to the screen) or relative (to parent widgets)
type WidgetPosition struct {
	X int
	Y int
	W int
	H int
}

// WidgetFlag represents flags that can be set on the widget
type WidgetFlag uint32

const (
	WidgetFlagNone   WidgetFlag = 0
	WidgetFlagHidden WidgetFlag = 1 << iota
	WidgetFlagDisabled
	WidgetFlagFocused
	WidgetFlagModal
	WidgetFlagCursor
)

// CursorState represents state of the cursor: style and position
type CursorState struct {
	// Cursor style
	Style cswterm.CursorStyle
	// Cursor position relative to the widget
	X, Y int
}

// TEventType represents type of the event (internal to TV)
type TEventType uint32

const (
	TEventTypeNone TEventType = iota
	TEventTypeInput
	TEventTypeTimer
	TEventTypePosition
	TEventTypeRedraw
)

type TEvent struct {
	Type TEventType
	// Input event for TEventTypeInput if any
	InputEvent *cswterm.InputEvent

	// Position data for TEventTypePosition
	X, Y, W, H uint32
}

// IWidget is an interface implemented by all widgets
// It contains basic widget functionality for positioning, drawing and event handling
type IWidget interface {
	// GetAbsolutePosition returns absolute position of the widget.
	// If widget has a parent, returns parent position + position to the parent
	GetAbsolutePosition() WidgetPosition

	// Draw draws the widget on the screen
	Draw(screen cswterm.IScreenOutput)

	// HandleEvent handles an event
	HandleEvent(event *TEvent)
}

type TWidget struct {
	// Position is relative to parent widget; if parent does not exist, it is relative to the screen
	Position WidgetPosition
	// Parent widget, if any
	Parent IWidget
	// Child widgets
	Children []IWidget
	// Active child widget, if any
	ActiveChild IWidget
	// Widget flags
	Flags WidgetFlag
	// Cursor state
	Cursor CursorState
}

// GetAbsolutePosition returns absolute position of the widget.
// If widget has a parent, returns parent position + position to the parent
func (w *TWidget) GetAbsolutePosition() WidgetPosition {
	if w.Parent == nil {
		return w.Position
	}
	parentPos := w.Parent.GetAbsolutePosition()
	return WidgetPosition{
		X: parentPos.X + w.Position.X,
		Y: parentPos.Y + w.Position.Y,
		W: w.Position.W,
		H: w.Position.H,
	}
}

// Draw draws the widget on the screen by calling Draw() on all children.
func (w *TWidget) Draw(screen cswterm.IScreenOutput) {
	for _, child := range w.Children {
		child.Draw(screen)
	}
}

// HandleEvent handles an event by delegating to the active child if any.
func (w *TWidget) HandleEvent(event *TEvent) {
	// Handle position events directly without propagating to children
	if event.Type == TEventTypePosition {
		w.Position.X = int(event.X)
		w.Position.Y = int(event.Y)
		w.Position.W = int(event.W)
		w.Position.H = int(event.H)
		return
	}

	if w.ActiveChild != nil {
		w.ActiveChild.HandleEvent(event)
	}
}
