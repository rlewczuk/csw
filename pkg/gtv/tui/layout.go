package tui

import (
	"github.com/codesnort/codesnort-swe/pkg/gtv"
)

// ILayout is an interface for layout widgets that manage children.
// It extends IResizable with layout-specific methods.
type ILayout interface {
	IResizable

	// SetBackground sets the background attributes for the layout.
	// If nil, the layout is transparent and only children are visible.
	SetBackground(attrs *gtv.CellAttributes)

	// GetBackground returns the current background attributes.
	GetBackground() *gtv.CellAttributes

	// SetTabOrder sets custom tab order for widgets in the layout.
	SetTabOrder(widgets []IWidget)

	// Next moves focus to the next widget in the tab order.
	Next()

	// Prev moves focus to the previous widget in the tab order.
	Prev()
}

// TLayout is a base widget that provides common layout functionality for all layout widgets.
// It extends TResizable and implements ILayout interface.
//
// The layout provides:
// - Background rendering (fills area with spaces using background attributes)
// - Child management (adding children, drawing children)
// - Event routing (resize, redraw, mouse, keyboard, focus events)
// - Active child tracking for focus management
// - Tab navigation support with customizable tab order
//
// Derived layout widgets should:
// - Override HandleEvent to implement custom event routing logic (e.g., different coordinate systems)
// - Override Draw if they need custom rendering before/after children
type TLayout struct {
	TResizable

	// Background attributes for the layout. If nil, layout is transparent.
	background *gtv.CellAttributes

	// TabOrderEnabled specifies whether tab navigation is enabled.
	// If true, layout handles Tab and Shift+Tab keys to move focus.
	// Default is true.
	TabOrderEnabled bool

	// tabOrder stores custom tab order of widgets.
	// If nil, tab order is the order in which widgets were added (Children slice order).
	tabOrder []IWidget

	// focusIndex tracks current focus position in tab order.
	// -1 means no widget has focus.
	focusIndex int
}

// NewLayout creates a new layout widget.
// The parent parameter is optional (can be nil for root widgets).
// The rect parameter specifies the position and size of the layout.
// The background parameter specifies background attributes. If nil, layout is transparent.
// The tabOrderEnabled parameter specifies whether tab navigation should be enabled (default is true).
// If additional parameters are omitted, TabOrderEnabled defaults to true.
//
// Note: When creating a derived widget that embeds TLayout, use newLayoutBase
// to avoid double registration with the parent. This constructor registers with the parent.
func NewLayout(parent IWidget, rect gtv.TRect, background *gtv.CellAttributes, tabOrderEnabled ...bool) *TLayout {
	// Default TabOrderEnabled to true if not provided
	enabled := true
	if len(tabOrderEnabled) > 0 {
		enabled = tabOrderEnabled[0]
	}

	layout := newLayoutBase(parent, rect, background, enabled)

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(layout)
	}

	return layout
}

// newLayoutBase creates a layout widget without registering with parent.
// This is used internally by derived widgets to avoid double registration.
// The tabOrderEnabled parameter specifies whether tab navigation should be enabled.
func newLayoutBase(parent IWidget, rect gtv.TRect, background *gtv.CellAttributes, tabOrderEnabled bool) *TLayout {
	resizableBase := newResizableBase(parent, rect)
	return &TLayout{
		TResizable:      *resizableBase,
		background:      background,
		TabOrderEnabled: tabOrderEnabled,
		tabOrder:        nil,
		focusIndex:      -1,
	}
}

// SetBackground sets the background attributes for the layout.
// If nil, the layout is transparent and only children are visible.
func (l *TLayout) SetBackground(attrs *gtv.CellAttributes) {
	l.background = attrs
}

// GetBackground returns the current background attributes.
func (l *TLayout) GetBackground() *gtv.CellAttributes {
	return l.background
}

// Draw draws the layout on the screen.
// If background is set, it fills the layout area with background color first.
// Then it draws all children.
func (l *TLayout) Draw(screen gtv.IScreenOutput) {
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
					gtv.TRect{X: absPos.X + x, Y: absPos.Y + y, W: 1, H: 1},
					[]gtv.Cell{{Rune: ' ', Attrs: *l.background}},
				)
			}
		}
	}

	// Draw children
	l.TResizable.Draw(screen)
}

// handleResizeEvent handles resize events for the layout.
// This is a helper method used by HandleEvent implementations.
func (l *TLayout) handleResizeEvent(event *TEvent) {
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
}

// handleRedrawEvent handles redraw events for the layout.
// This is a helper method used by HandleEvent implementations.
func (l *TLayout) handleRedrawEvent(event *TEvent) {
	for _, child := range l.Children {
		child.HandleEvent(event)
	}
}

// handleKeyboardEvent handles keyboard events for the layout.
// This is a helper method used by HandleEvent implementations.
// It handles Tab and Shift+Tab for focus navigation if TabOrderEnabled is true.
func (l *TLayout) handleKeyboardEvent(event *TEvent) {
	inputEvent := event.InputEvent

	// Handle Tab navigation if enabled
	if l.TabOrderEnabled && inputEvent.Key == '\t' {
		if inputEvent.Modifiers&gtv.ModShift != 0 {
			// Shift+Tab - move to previous widget
			l.Prev()
		} else {
			// Tab - move to next widget
			l.Next()
		}
		return
	}

	// Route other keyboard events to active child
	if l.ActiveChild != nil {
		l.ActiveChild.HandleEvent(event)
	}
}

// handleOtherInputEvent handles other input events (focus, blur, etc.) for the layout.
// This is a helper method used by HandleEvent implementations.
func (l *TLayout) handleOtherInputEvent(event *TEvent) {
	if l.ActiveChild != nil {
		l.ActiveChild.HandleEvent(event)
	}
}

// HandleEvent handles events for the layout.
// It handles resize events, redraw events, and routes input events to children.
//
// This base implementation provides:
// - Resize event handling with child redraw triggering on size change
// - Redraw event propagation to all children
// - Mouse event routing to child under cursor (uses GetChildAt for hit-testing)
// - Keyboard/focus event routing to active child
//
// Derived layouts should override this method to implement custom event routing logic,
// especially for mouse events where coordinate systems may differ.
func (l *TLayout) HandleEvent(event *TEvent) {
	// Handle resize events directly
	if event.Type == TEventTypeResize {
		l.handleResizeEvent(event)
		return
	}

	// Handle redraw events by propagating to all children
	if event.Type == TEventTypeRedraw {
		l.handleRedrawEvent(event)
		return
	}

	// Handle input events
	if event.Type == TEventTypeInput && event.InputEvent != nil {
		inputEvent := event.InputEvent

		// Handle mouse events - route to child under cursor
		// Derived layouts should override this to implement custom coordinate systems
		if inputEvent.Type == gtv.InputEventMouse {
			// Get child under cursor (if any)
			child := l.GetChildAt(inputEvent.X, inputEvent.Y)
			if child != nil {
				// On mouse click, set focus to the clicked widget
				if inputEvent.Modifiers&gtv.ModClick != 0 {
					l.setFocus(child)
				}
				// Forward the event to the child
				child.HandleEvent(event)
			}
			return
		}

		// Handle keyboard events - route to active child
		if inputEvent.Type == gtv.InputEventKey {
			l.handleKeyboardEvent(event)
			return
		}

		// Handle other input events (focus, blur, etc.) - route to active child
		l.handleOtherInputEvent(event)
		return
	}

	// For other event types, delegate to base widget
	l.TResizable.HandleEvent(event)
}

// GetChildAt returns the topmost child widget at the given absolute screen coordinates.
// Returns nil if no child is found at the given coordinates.
//
// This implementation performs hit-testing on children by:
// 1. Checking if coordinates are within layout bounds
// 2. Converting absolute coordinates to relative coordinates (relative to layout)
// 3. Iterating children in reverse order (for proper z-order - last drawn is on top)
// 4. Checking if the relative coordinates fall within each child's bounds
//
// Derived layouts should override this method only if they use a different coordinate
// system (e.g., scrolling layouts, transformed layouts).
func (l *TLayout) GetChildAt(x, y uint16) IWidget {
	// Get absolute position of layout
	absPos := l.GetAbsolutePos()

	// Check if mouse is within layout bounds
	if x >= absPos.X && x < absPos.X+absPos.W &&
		y >= absPos.Y && y < absPos.Y+absPos.H {

		// Calculate mouse position relative to layout
		relativeX := x - absPos.X
		relativeY := y - absPos.Y

		// Find child widget under cursor (iterate in reverse order to handle overlapping widgets)
		for i := len(l.Children) - 1; i >= 0; i-- {
			child := l.Children[i]

			// Get child's position relative to this layout (parent)
			childPos := child.GetPos()

			// Check if mouse is within child bounds using relative coordinates
			if relativeX >= childPos.X && relativeX < childPos.X+childPos.W &&
				relativeY >= childPos.Y && relativeY < childPos.Y+childPos.H {
				return child
			}
		}
	}

	return nil
}

// SetTabOrder sets custom tab order for widgets in the layout.
// The widgets parameter specifies the order in which widgets should receive focus when Tab is pressed.
// If widgets is nil or empty, tab order returns to default (order in which widgets were added).
// Widgets not in the list will not be included in tab navigation.
func (l *TLayout) SetTabOrder(widgets []IWidget) {
	if len(widgets) == 0 {
		l.tabOrder = nil
		l.focusIndex = -1
		return
	}

	// Copy the widgets slice to avoid external modifications
	l.tabOrder = make([]IWidget, len(widgets))
	copy(l.tabOrder, widgets)

	// Reset focus index if current focus is not in new tab order
	if l.focusIndex >= 0 {
		found := false
		for i, w := range l.tabOrder {
			if w == l.ActiveChild {
				l.focusIndex = i
				found = true
				break
			}
		}
		if !found {
			l.focusIndex = -1
		}
	}
}

// getTabOrder returns the effective tab order.
// If custom tab order is set, returns it; otherwise returns Children slice.
func (l *TLayout) getTabOrder() []IWidget {
	if l.tabOrder != nil {
		return l.tabOrder
	}
	return l.Children
}

// setFocus changes focus to the specified widget.
// It sends blur event to the previously focused widget (if any) and focus event to the new widget.
// If the widget is nil, just removes focus from current widget.
func (l *TLayout) setFocus(widget IWidget) {
	// If already focused, do nothing
	if l.ActiveChild == widget {
		return
	}

	// Send blur event to previous active child
	if l.ActiveChild != nil {
		blurEvent := &TEvent{
			Type:       TEventTypeInput,
			InputEvent: &gtv.InputEvent{Type: gtv.InputEventBlur},
		}
		l.ActiveChild.HandleEvent(blurEvent)
	}

	// Update active child
	l.ActiveChild = widget

	// Send focus event to new active child
	if l.ActiveChild != nil {
		focusEvent := &TEvent{
			Type:       TEventTypeInput,
			InputEvent: &gtv.InputEvent{Type: gtv.InputEventFocus},
		}
		l.ActiveChild.HandleEvent(focusEvent)
	}

	// Update focus index
	if widget != nil {
		tabOrder := l.getTabOrder()
		for i, w := range tabOrder {
			if w == widget {
				l.focusIndex = i
				return
			}
		}
		// Widget not in tab order
		l.focusIndex = -1
	} else {
		l.focusIndex = -1
	}
}

// Next moves focus to the next widget in the tab order.
// If no widget has focus, focuses the first widget.
// If the last widget has focus, wraps around to the first widget.
func (l *TLayout) Next() {
	if !l.TabOrderEnabled {
		return
	}

	tabOrder := l.getTabOrder()
	if len(tabOrder) == 0 {
		return
	}

	// Find next widget in tab order
	nextIndex := l.focusIndex + 1
	if nextIndex >= len(tabOrder) {
		nextIndex = 0
	}

	l.focusIndex = nextIndex
	l.setFocus(tabOrder[nextIndex])
}

// Prev moves focus to the previous widget in the tab order.
// If no widget has focus, focuses the last widget.
// If the first widget has focus, wraps around to the last widget.
func (l *TLayout) Prev() {
	if !l.TabOrderEnabled {
		return
	}

	tabOrder := l.getTabOrder()
	if len(tabOrder) == 0 {
		return
	}

	// Find previous widget in tab order
	prevIndex := l.focusIndex - 1
	if prevIndex < 0 {
		prevIndex = len(tabOrder) - 1
	}

	l.focusIndex = prevIndex
	l.setFocus(tabOrder[prevIndex])
}
