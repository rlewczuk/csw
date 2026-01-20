package tui

import (
	"github.com/codesnort/codesnort-swe/pkg/gtv"
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
// - Normal and focused text attributes
// - Automatic flag management for focus and cursor
type TFocusable struct {
	TWidget

	// Text attributes for normal state
	attrs gtv.CellAttributes

	// Text attributes for focused state
	focusedAttrs gtv.CellAttributes
}

// NewFocusable creates a new focusable widget with the specified position and attributes.
// The parent parameter is optional (can be nil for root widgets).
// The rect parameter specifies the position and size of the widget.
// The attrs parameter specifies text attributes for normal state.
// The focusedAttrs parameter specifies text attributes for focused state.
//
// Note: When creating a derived widget that embeds TFocusable, use newFocusableBase
// to avoid double registration with the parent. This constructor registers with the parent.
func NewFocusable(parent IWidget, rect gtv.TRect, attrs, focusedAttrs gtv.CellAttributes) *TFocusable {
	focusable := newFocusableBase(parent, rect, attrs, focusedAttrs)

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(focusable)
	}

	return focusable
}

// newFocusableBase creates a focusable widget without registering with parent.
// This is used internally by derived widgets to avoid double registration.
func newFocusableBase(parent IWidget, rect gtv.TRect, attrs, focusedAttrs gtv.CellAttributes) *TFocusable {
	return &TFocusable{
		TWidget: TWidget{
			Position: rect,
			Parent:   parent,
			Flags:    WidgetFlagNone,
		},
		attrs:        attrs,
		focusedAttrs: focusedAttrs,
	}
}

// GetAttrs returns the text attributes for normal state.
func (f *TFocusable) GetAttrs() gtv.CellAttributes {
	return f.attrs
}

// SetAttrs sets the text attributes for normal state.
func (f *TFocusable) SetAttrs(attrs gtv.CellAttributes) {
	f.attrs = attrs
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
