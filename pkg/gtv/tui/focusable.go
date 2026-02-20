package tui

import (
	"github.com/rlewczuk/csw/pkg/gtv"
)

// IFocusable is an interface for widgets that can receive focus.
// It extends IWidget with focus-specific methods.
type IFocusable interface {
	IWidget

	// GetAttrs returns the text attributes for normal state.
	GetAttrs() gtv.CellAttributes

	// SetAttrs sets the text attributes for normal state.
	SetAttrs(attrs gtv.CellAttributes)

	// GetFocusedAttrs returns the text attributes for focused state.
	GetFocusedAttrs() gtv.CellAttributes

	// SetFocusedAttrs sets the text attributes for focused state.
	SetFocusedAttrs(attrs gtv.CellAttributes)

	// Focus sets focus to the widget.
	Focus()

	// Blur removes focus from the widget.
	Blur()

	// IsFocused returns true if the widget is focused.
	IsFocused() bool
}

// TFocusable is a base widget that provides focus management functionality.
// It extends TWidget and implements IFocusable interface.
//
// This widget is designed to be embedded in other widgets that need focus management,
// such as input boxes, buttons, text areas, etc.
//
// Features:
// - Focus/blur state management
// - Normal and focused text attributes (normal attrs inherited from TWidget)
// - Automatic flag management for focus and cursor
type TFocusable struct {
	TWidget

	// Text attributes for focused state
	focusedAttrs gtv.CellAttributes
}

func WithFocusedAttrs(attrs gtv.CellAttributes) gtv.Option {
	return func(w any) {
		if w, ok := w.(*TFocusable); ok {
			w.focusedAttrs = attrs
		}
	}
}

// NewFocusable creates a new focusable widget with the specified parent and options.
// The parent parameter is optional (can be nil for root widgets).
// Options can be used to configure position, attributes, and other properties.
//
// Default values:
// - Position: gtv.TRect{X: 0, Y: 0, W: 0, H: 0}
// - Attrs: gtv.CellAttributes{}
// - FocusedAttrs: gtv.CellAttributes{}
// - Flags: WidgetFlagNone
//
// Available options:
// - WithRectangle(X, Y, W, H) - sets position and size
// - WithPosition(X, Y) - sets position only
// - WithAttrs(attrs) - sets normal state attributes
// - WithFocusedAttrs(attrs) - sets focused state attributes
// - WithFlags(flags) - sets widget flags
// - WithChild(child) - adds a child widget
//
// Note: When creating a derived widget that embeds TFocusable, use newFocusableBase
// to avoid double registration with the parent. This constructor registers with the parent.
func NewFocusable(parent IWidget, opts ...gtv.Option) *TFocusable {
	focusable := newFocusableBase(parent, opts...)

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(focusable)
	}

	return focusable
}

// newFocusableBase creates a focusable widget without registering with parent.
// This is used internally by derived widgets to avoid double registration.
func newFocusableBase(parent IWidget, opts ...gtv.Option) *TFocusable {
	focusable := &TFocusable{
		TWidget: TWidget{
			Position:  gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
			Parent:    parent,
			Flags:     WidgetFlagNone,
			cellAttrs: gtv.CellAttributes{},
		},
		focusedAttrs: gtv.CellAttributes{},
	}

	// Apply options to TWidget first
	for _, opt := range opts {
		opt(&focusable.TWidget)
	}

	// Apply options to TFocusable
	for _, opt := range opts {
		opt(focusable)
	}

	return focusable
}

// GetAttrs returns the text attributes for normal state.
// This method overrides TWidget.GetAttrs() but delegates to it.
func (f *TFocusable) GetAttrs() gtv.CellAttributes {
	return f.TWidget.GetAttrs()
}

// SetAttrs sets the text attributes for normal state.
// This method overrides TWidget.SetAttrs() but delegates to it.
func (f *TFocusable) SetAttrs(attrs gtv.CellAttributes) {
	f.TWidget.SetAttrs(attrs)
}

// GetFocusedAttrs returns the text attributes for focused state.
func (f *TFocusable) GetFocusedAttrs() gtv.CellAttributes {
	return f.focusedAttrs
}

// SetFocusedAttrs sets the text attributes for focused state.
func (f *TFocusable) SetFocusedAttrs(attrs gtv.CellAttributes) {
	f.focusedAttrs = attrs
}

// Focus sets focus to the widget.
func (f *TFocusable) Focus() {
	f.Flags |= WidgetFlagFocused | WidgetFlagCursor
}

// Blur removes focus from the widget.
func (f *TFocusable) Blur() {
	f.Flags &= ^(WidgetFlagFocused | WidgetFlagCursor)
}

// IsFocused returns true if the widget is focused.
func (f *TFocusable) IsFocused() bool {
	return f.Flags&WidgetFlagFocused != 0
}

// HandleEvent handles events for the focusable widget.
// It handles focus and blur events by calling Focus() and Blur() methods.
// Other events are delegated to the base widget.
func (f *TFocusable) HandleEvent(event *TEvent) {
	// Handle focus/blur events
	if event.Type == TEventTypeInput && event.InputEvent != nil {
		switch event.InputEvent.Type {
		case gtv.InputEventFocus:
			f.Focus()
			return
		case gtv.InputEventBlur:
			f.Blur()
			return
		}
	}

	// Delegate other events to base widget
	f.TWidget.HandleEvent(event)
}

// Draw draws the focusable widget on the screen.
// This is a minimal implementation that just draws children.
// Derived widgets should override this method to provide custom rendering.
func (f *TFocusable) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if f.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Draw children (if any)
	f.TWidget.Draw(screen)
}
