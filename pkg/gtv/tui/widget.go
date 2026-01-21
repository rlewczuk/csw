package tui

import (
	"github.com/codesnort/codesnort-swe/pkg/gtv"
)

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
	Style gtv.CursorStyle
	// Cursor position relative to the widget
	X, Y int
}

// TEventType represents type of the event (internal to TV)
type TEventType uint32

const (
	TEventTypeNone TEventType = iota
	// TEventTypeInput conveys input event from the terminal
	TEventTypeInput
	// TEventTypeTimer conveys timer event
	TEventTypeTimer
	// TEventTypeResize conveys resize/reposition event
	TEventTypeResize
	// TEventTypeRedraw conveys redraw event
	TEventTypeRedraw
)

type TEvent struct {
	Type TEventType
	// Input event for TEventTypeInput if any
	InputEvent *gtv.InputEvent

	// Position data for TEventTypeResize
	Rect gtv.TRect
}

// IWidget is an interface implemented by all widgets
// It contains basic widget functionality for positioning, drawing and event handling
type IWidget interface {
	// GetPos returns position of the widget relative to its parent.
	// If widget has no parent, returns position relative to the screen.
	GetPos() gtv.TRect

	// GetAbsolutePos returns absolute position of the widget.
	// If widget has a parent, returns parent position + position to the parent
	GetAbsolutePos() gtv.TRect

	// Draw draws the widget on the screen
	Draw(screen gtv.IScreenOutput)

	// HandleEvent handles an event
	HandleEvent(event *TEvent)

	AddChild(child IWidget)
}

type TWidget struct {
	// Position is relative to parent widget; if parent does not exist, it is relative to the screen
	Position gtv.TRect
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

func WithPosition(X, Y int) gtv.Option {
	return func(w any) {
		if w, ok := w.(*TWidget); ok {
			w.Position.X = uint16(X)
			w.Position.Y = uint16(Y)
		}
	}
}

func WithRectangle(X, Y, W, H int) gtv.Option {
	return func(w any) {
		if w, ok := w.(*TWidget); ok {
			w.Position = gtv.TRect{X: uint16(X), Y: uint16(Y), W: uint16(W), H: uint16(H)}
		}
	}
}

func WithChild(child IWidget) gtv.Option {
	return func(w any) {
		if w, ok := w.(IWidget); ok {
			w.AddChild(child)
		}
	}
}

func WithFlags(flags WidgetFlag) gtv.Option {
	return func(w any) {
		if w, ok := w.(*TWidget); ok {
			w.Flags = flags
		}
	}
}

func WithCursorState(cursor CursorState) gtv.Option {
	return func(w any) {
		if w, ok := w.(*TWidget); ok {
			w.Cursor = cursor
		}
	}
}

// GetPos returns position of the widget relative to its parent.
// If widget has no parent, returns position relative to the screen.
func (w *TWidget) GetPos() gtv.TRect {
	return w.Position
}

// GetAbsolutePos returns absolute position of the widget.
// If widget has a parent, returns parent position + position to the parent
func (w *TWidget) GetAbsolutePos() gtv.TRect {
	if w.Parent == nil {
		return w.Position
	}
	parentPos := w.Parent.GetAbsolutePos()
	return gtv.TRect{
		X: parentPos.X + w.Position.X,
		Y: parentPos.Y + w.Position.Y,
		W: w.Position.W,
		H: w.Position.H,
	}
}

// Draw draws the widget on the screen by calling Draw() on all children.
func (w *TWidget) Draw(screen gtv.IScreenOutput) {
	for _, child := range w.Children {
		child.Draw(screen)
	}
}

// HandleEvent handles an event by delegating to the active child if any.
func (w *TWidget) HandleEvent(event *TEvent) {
	// Handle position events directly without propagating to children
	if event.Type == TEventTypeResize {
		w.Position.X = event.Rect.X
		w.Position.Y = event.Rect.Y
		w.Position.W = event.Rect.W
		w.Position.H = event.Rect.H
		return
	}

	if w.ActiveChild != nil {
		w.ActiveChild.HandleEvent(event)
	}
}

func (w *TWidget) AddChild(child IWidget) {
	w.Children = append(w.Children, child)
}
