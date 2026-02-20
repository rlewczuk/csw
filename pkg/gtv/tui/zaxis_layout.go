package tui

import (
	"sort"

	"github.com/rlewczuk/csw/pkg/gtv"
)

// IZAxisLayout is an interface for Z-axis layout widgets that stack children in Z-order.
// It extends ILayout with Z-axis specific methods.
type IZAxisLayout interface {
	ILayout

	// AddZWidget adds a widget to the layout with the specified z-index and options.
	// Higher z-index values are drawn on top of lower z-index values.
	AddZWidget(widget IWidget, zIndex int, opts ...ZWidgetOption)

	// RemoveZWidget removes a widget from the layout.
	RemoveZWidget(widget IWidget)

	// SetForwardMouseToLowerWidgets sets whether mouse events should be forwarded to
	// visible parts of lower z-index widgets.
	SetForwardMouseToLowerWidgets(forward bool)
}

// ZWidgetBehavior specifies how lower z-index widgets should behave when a widget is on top.
type ZWidgetBehavior int

const (
	// ZWidgetBehaviorNone means lower widgets remain fully visible and interactive.
	ZWidgetBehaviorNone ZWidgetBehavior = iota
	// ZWidgetBehaviorDim means lower widgets are dimmed but remain visible.
	ZWidgetBehaviorDim
	// ZWidgetBehaviorHide means lower widgets are completely hidden.
	ZWidgetBehaviorHide
)

// ZWidgetOption is a function that configures ZWidgetEntry options.
type ZWidgetOption func(*ZWidgetEntry)

// WithZBehavior sets the behavior for widgets below this one in the Z-order.
func WithZBehavior(behavior ZWidgetBehavior) ZWidgetOption {
	return func(entry *ZWidgetEntry) {
		entry.Behavior = behavior
	}
}

// ZWidgetEntry represents a widget with its z-index and behavior settings.
type ZWidgetEntry struct {
	Widget   IWidget
	ZIndex   int
	Behavior ZWidgetBehavior
}

// TZAxisLayout is a layout widget that stacks children in Z-axis order.
// It extends TLayout and implements IZAxisLayout interface.
//
// The layout:
// - Positions children at absolute positions (like TAbsoluteLayout)
// - Draws children in Z-order (lower z-index first, higher z-index on top)
// - Can dim or hide lower z-index widgets based on configuration
// - Routes input events only to topmost visible widget
// - Optionally forwards mouse events to lower z-index widgets if they're visible
type TZAxisLayout struct {
	TLayout

	// zWidgets stores widgets with their z-index and behavior settings.
	// This is separate from Children slice to maintain z-order information.
	zWidgets []ZWidgetEntry

	// forwardMouseToLowerWidgets controls whether mouse events should be forwarded
	// to visible parts of lower z-index widgets.
	forwardMouseToLowerWidgets bool
}

// NewZAxisLayout creates a new Z-axis layout widget.
// The parent parameter is optional (can be nil for root widgets).
// The rect parameter specifies the position and size of the layout.
// The background parameter specifies background attributes. If nil, layout is transparent.
// The tabOrderEnabled parameter specifies whether tab navigation should be enabled (default is false for Z-axis layouts).
// If additional parameters are omitted, TabOrderEnabled defaults to false.
func NewZAxisLayout(parent IWidget, rect gtv.TRect, background *gtv.CellAttributes, tabOrderEnabled ...bool) *TZAxisLayout {
	// Default TabOrderEnabled to false for Z-axis layouts (usually used for popups/overlays)
	enabled := false
	if len(tabOrderEnabled) > 0 {
		enabled = tabOrderEnabled[0]
	}

	layoutBase := newLayoutBase(parent, rect, background, enabled)
	layout := &TZAxisLayout{
		TLayout:                    *layoutBase,
		zWidgets:                   make([]ZWidgetEntry, 0),
		forwardMouseToLowerWidgets: false,
	}

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(layout)
	}

	return layout
}

// AddZWidget adds a widget to the layout with the specified z-index and options.
// Higher z-index values are drawn on top of lower z-index values.
func (z *TZAxisLayout) AddZWidget(widget IWidget, zIndex int, opts ...ZWidgetOption) {
	entry := ZWidgetEntry{
		Widget:   widget,
		ZIndex:   zIndex,
		Behavior: ZWidgetBehaviorNone,
	}

	// Apply options
	for _, opt := range opts {
		opt(&entry)
	}

	// Add to zWidgets list
	z.zWidgets = append(z.zWidgets, entry)

	// Also add to Children slice (required by base widget)
	z.Children = append(z.Children, widget)
	widget.SetParent(z)

	// Sort zWidgets by z-index (lower z-index first)
	z.sortZWidgets()
}

// RemoveZWidget removes a widget from the layout.
func (z *TZAxisLayout) RemoveZWidget(widget IWidget) {
	// Remove from zWidgets
	for i, entry := range z.zWidgets {
		if entry.Widget == widget {
			z.zWidgets = append(z.zWidgets[:i], z.zWidgets[i+1:]...)
			break
		}
	}

	// Remove from Children
	for i, child := range z.Children {
		if child == widget {
			z.Children = append(z.Children[:i], z.Children[i+1:]...)
			break
		}
	}

	// If the removed widget was active, clear active child
	if z.ActiveChild == widget {
		z.ActiveChild = nil
	}
}

// SetForwardMouseToLowerWidgets sets whether mouse events should be forwarded to
// visible parts of lower z-index widgets.
func (z *TZAxisLayout) SetForwardMouseToLowerWidgets(forward bool) {
	z.forwardMouseToLowerWidgets = forward
}

// sortZWidgets sorts zWidgets by z-index (lower z-index first).
func (z *TZAxisLayout) sortZWidgets() {
	sort.Slice(z.zWidgets, func(i, j int) bool {
		return z.zWidgets[i].ZIndex < z.zWidgets[j].ZIndex
	})
}

// Draw draws the layout on the screen.
// It draws widgets in Z-order, applying dimming or hiding to lower z-index widgets as needed.
func (z *TZAxisLayout) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if z.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Get absolute position
	absPos := z.GetAbsolutePos()

	// Draw background if set
	if z.hasBackground {
		bgAttrs := z.TResizable.TWidget.cellAttrs
		for y := uint16(0); y < absPos.H; y++ {
			for x := uint16(0); x < absPos.W; x++ {
				screen.PutContent(
					gtv.TRect{X: absPos.X + x, Y: absPos.Y + y, W: 1, H: 1},
					[]gtv.Cell{{Rune: ' ', Attrs: bgAttrs}},
				)
			}
		}
	}

	// Draw widgets in Z-order
	for i, entry := range z.zWidgets {
		widget := entry.Widget

		// Check if this widget should be dimmed or hidden
		shouldDim := false
		shouldHide := false

		// Check if any higher z-index widget wants to dim or hide lower widgets
		for j := i + 1; j < len(z.zWidgets); j++ {
			higherEntry := z.zWidgets[j]
			if higherEntry.Behavior == ZWidgetBehaviorHide {
				shouldHide = true
				break
			} else if higherEntry.Behavior == ZWidgetBehaviorDim {
				shouldDim = true
				// Don't break - a higher widget might want to hide
			}
		}

		// Skip hidden widgets
		if shouldHide {
			continue
		}

		// Draw the widget
		widget.Draw(screen)

		// Apply dimming overlay if needed
		if shouldDim {
			z.drawDimmingOverlay(screen, widget)
		}
	}
}

// drawDimmingOverlay draws a dimming overlay over the specified widget.
// It reads back the screen content and re-renders each cell with AttrDim added.
// This preserves the content while applying dimming effect.
func (z *TZAxisLayout) drawDimmingOverlay(screen gtv.IScreenOutput, widget IWidget) {
	// Get widget's absolute position
	widgetPos := widget.GetAbsolutePos()

	// Get screen content to read back what was drawn
	width, height, content := screen.GetContent()

	// Modify each cell in the widget area to add dimming
	for y := uint16(0); y < widgetPos.H; y++ {
		for x := uint16(0); x < widgetPos.W; x++ {
			screenX := widgetPos.X + x
			screenY := widgetPos.Y + y

			// Check bounds
			if screenX >= uint16(width) || screenY >= uint16(height) {
				continue
			}

			// Calculate index
			idx := int(screenY)*width + int(screenX)

			// Read the existing cell
			existingCell := content[idx]

			// Add AttrDim to existing attributes
			existingCell.Attrs.Attributes |= gtv.AttrDim

			// Write back the modified cell
			screen.PutContent(
				gtv.TRect{X: screenX, Y: screenY, W: 1, H: 1},
				[]gtv.Cell{existingCell},
			)
		}
	}
}

// HandleEvent handles events for the layout.
// It routes input events only to the topmost visible widget (highest z-index).
func (z *TZAxisLayout) HandleEvent(event *TEvent) {
	// Handle resize events directly
	if event.Type == TEventTypeResize {
		z.handleResizeEvent(event)

		// Propagate resize event to all Z-widgets that should fill the layout
		// Only resize widgets that were originally positioned at (0,0) with full size
		for _, entry := range z.zWidgets {
			widgetPos := entry.Widget.GetPos()
			// If widget was positioned at (0,0) and sized to fill parent, resize it
			if widgetPos.X == 0 && widgetPos.Y == 0 {
				childResizeEvent := &TEvent{
					Type: TEventTypeResize,
					Rect: gtv.TRect{X: 0, Y: 0, W: z.Position.W, H: z.Position.H},
				}
				entry.Widget.HandleEvent(childResizeEvent)
			}
		}
		return
	}

	// Handle redraw events by propagating to all children
	if event.Type == TEventTypeRedraw {
		z.handleRedrawEvent(event)
		return
	}

	// Handle input events
	if event.Type == TEventTypeInput && event.InputEvent != nil {
		inputEvent := event.InputEvent

		// Handle mouse events - route to topmost widget under cursor
		if inputEvent.Type == gtv.InputEventMouse {
			// Find the topmost widget under cursor (iterate in reverse order)
			var targetWidget IWidget
			for i := len(z.zWidgets) - 1; i >= 0; i-- {
				entry := z.zWidgets[i]
				widget := entry.Widget

				// Check if any higher z-index widget wants to hide this widget
				shouldHide := false
				for j := i + 1; j < len(z.zWidgets); j++ {
					if z.zWidgets[j].Behavior == ZWidgetBehaviorHide {
						shouldHide = true
						break
					}
				}

				// Skip hidden widgets
				if shouldHide {
					continue
				}

				// Check if mouse is within widget bounds
				widgetPos := widget.GetPos()
				absPos := z.GetAbsolutePos()
				relativeX := inputEvent.X - absPos.X
				relativeY := inputEvent.Y - absPos.Y

				if relativeX >= widgetPos.X && relativeX < widgetPos.X+widgetPos.W &&
					relativeY >= widgetPos.Y && relativeY < widgetPos.Y+widgetPos.H {
					targetWidget = widget
					break
				}
			}

			// If we found a target widget, handle the event
			if targetWidget != nil {
				// On mouse press, set focus to the clicked widget
				if inputEvent.Modifiers&gtv.ModPress != 0 {
					z.setFocus(targetWidget)
				}
				// Forward the event to the target widget
				targetWidget.HandleEvent(event)
				return
			}

			// If forwardMouseToLowerWidgets is disabled or no widget found, don't forward
			return
		}

		// Handle keyboard events - route to topmost visible widget (active child)
		if inputEvent.Type == gtv.InputEventKey {
			z.handleKeyboardEvent(event)
			return
		}

		// Handle other input events (focus, blur, etc.) - route to active child
		z.handleOtherInputEvent(event)
		return
	}

	// For other event types, delegate to base layout
	z.TLayout.HandleEvent(event)
}

// setFocus changes focus to the specified widget.
// It sends blur event to the previously focused widget (if any) and focus event to the new widget.
// Override from TLayout to ensure only the topmost widget can be focused.
func (z *TZAxisLayout) setFocus(widget IWidget) {
	// If widget is not nil, check if it's visible (not hidden by higher z-index widgets)
	if widget != nil {
		// Find the widget's z-index
		widgetIndex := -1
		for i, entry := range z.zWidgets {
			if entry.Widget == widget {
				widgetIndex = i
				break
			}
		}

		// If widget not found or is hidden by higher z-index widget, don't focus
		if widgetIndex == -1 {
			return
		}

		// Check if any higher z-index widget wants to hide this widget
		for j := widgetIndex + 1; j < len(z.zWidgets); j++ {
			if z.zWidgets[j].Behavior == ZWidgetBehaviorHide {
				return // Don't focus hidden widget
			}
		}
	}

	// Call parent's setFocus implementation
	z.TLayout.setFocus(widget)
}
