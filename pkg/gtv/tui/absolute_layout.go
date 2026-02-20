package tui

import (
	"github.com/rlewczuk/csw/pkg/gtv"
)

// IAbsoluteLayout is an interface for absolute layout widgets that position children at absolute positions.
// It extends ILayout with absolute layout-specific methods.
type IAbsoluteLayout interface {
	ILayout
}

// TAbsoluteLayout is a widget that positions children at absolute positions (relative to itself).
// It extends TLayout and implements IAbsoluteLayout interface.
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
	TLayout
}

// NewAbsoluteLayout creates a new absolute layout widget.
// The parent parameter is optional (can be nil for root widgets).
// The rect parameter specifies the position and size of the layout.
// The background parameter specifies background attributes. If nil, layout is transparent.
// The tabOrderEnabled parameter specifies whether tab navigation should be enabled (default is true).
// If additional parameters are omitted, TabOrderEnabled defaults to true.
func NewAbsoluteLayout(parent IWidget, rect gtv.TRect, background *gtv.CellAttributes, tabOrderEnabled ...bool) *TAbsoluteLayout {
	// Default TabOrderEnabled to true if not provided
	enabled := true
	if len(tabOrderEnabled) > 0 {
		enabled = tabOrderEnabled[0]
	}

	layoutBase := newLayoutBase(parent, rect, background, enabled)
	layout := &TAbsoluteLayout{
		TLayout: *layoutBase,
	}

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(layout)
	}

	return layout
}
