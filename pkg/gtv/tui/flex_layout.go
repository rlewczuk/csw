package tui

import (
	"github.com/rlewczuk/csw/pkg/gtv"
)

// FlexDirection represents the direction of the flex layout (horizontal or vertical).
type FlexDirection uint8

const (
	FlexDirectionRow    FlexDirection = iota // Horizontal layout
	FlexDirectionColumn                      // Vertical layout
)

// FlexJustifyContent represents how items are justified along the main axis.
type FlexJustifyContent uint8

const (
	FlexJustifyStart        FlexJustifyContent = iota // Items packed to start
	FlexJustifyEnd                                    // Items packed to end
	FlexJustifyCenter                                 // Items centered
	FlexJustifySpaceBetween                           // Items evenly distributed, first at start, last at end
	FlexJustifySpaceAround                            // Items evenly distributed with equal space around
	FlexJustifySpaceEvenly                            // Items evenly distributed with equal space between
)

// FlexAlignItems represents how items are aligned along the cross axis.
type FlexAlignItems uint8

const (
	FlexAlignStart   FlexAlignItems = iota // Items aligned to start
	FlexAlignEnd                           // Items aligned to end
	FlexAlignCenter                        // Items centered
	FlexAlignStretch                       // Items stretched to fill container
)

// FlexItemProperties represents properties of a flex item.
type FlexItemProperties struct {
	// FlexGrow defines how much the item should grow relative to other items.
	// Default is 0 (no grow).
	FlexGrow float64

	// FlexShrink defines how much the item should shrink relative to other items.
	// Default is 1 (can shrink).
	FlexShrink float64

	// FlexBasis defines the initial main size of the item before flex-grow/shrink.
	// 0 means use the widget's current size.
	// This is used as the base size for flex calculations.
	FlexBasis uint16

	// MinSize defines the minimum size in the direction of the layout.
	// 0 means no minimum (widget can shrink to 0).
	MinSize uint16

	// MaxSize defines the maximum size in the direction of the layout.
	// 0 means no maximum (widget can grow indefinitely).
	MaxSize uint16

	// FixedWidth defines a fixed width for the item (horizontal direction).
	// 0 means width is determined by flex properties or widget size.
	FixedWidth uint16

	// FixedHeight defines a fixed height for the item (vertical direction).
	// 0 means height is determined by flex properties or widget size.
	FixedHeight uint16

	// PaddingTop defines padding above the item.
	PaddingTop uint16

	// PaddingBottom defines padding below the item.
	PaddingBottom uint16

	// PaddingLeft defines padding to the left of the item.
	PaddingLeft uint16

	// PaddingRight defines padding to the right of the item.
	PaddingRight uint16
}

// IFlexLayout is an interface for flex layout widgets.
// It extends ILayout with flex-specific methods.
type IFlexLayout interface {
	ILayout

	// SetDirection sets the flex direction (row or column).
	SetDirection(direction FlexDirection)

	// GetDirection returns the current flex direction.
	GetDirection() FlexDirection

	// SetJustifyContent sets how items are justified along the main axis.
	SetJustifyContent(justify FlexJustifyContent)

	// GetJustifyContent returns the current justify content setting.
	GetJustifyContent() FlexJustifyContent

	// SetAlignItems sets how items are aligned along the cross axis.
	SetAlignItems(align FlexAlignItems)

	// GetAlignItems returns the current align items setting.
	GetAlignItems() FlexAlignItems

	// SetItemProperties sets flex properties for a specific child widget.
	SetItemProperties(child IWidget, props FlexItemProperties)

	// GetItemProperties returns flex properties for a specific child widget.
	GetItemProperties(child IWidget) FlexItemProperties

	// SetPadding sets padding for the layout.
	SetPadding(top, bottom, left, right uint16)

	// SetDefaultItemPadding sets default padding for new items.
	SetDefaultItemPadding(top, bottom, left, right uint16)
}

// TFlexLayout is a widget that positions children in a flex layout (horizontal or vertical).
// It implements IFlexLayout interface and extends TLayout.
//
// The flex layout supports:
// - Horizontal and vertical directions
// - Flex-grow and flex-shrink for dynamic sizing
// - Minimum and maximum sizes for items
// - Fixed sizes for items (width and/or height)
// - Alignment (start, end, center, stretch)
// - Justification (flex-start, flex-end, center, space-between, space-around, space-evenly)
// - Padding for layout and individual items
// - Automatic resize handling
type TFlexLayout struct {
	TLayout

	// direction defines the direction of the flex layout
	direction FlexDirection

	// justifyContent defines how items are justified along the main axis
	justifyContent FlexJustifyContent

	// alignItems defines how items are aligned along the cross axis
	alignItems FlexAlignItems

	// itemProperties maps child widgets to their flex properties
	itemProperties map[IWidget]FlexItemProperties

	// defaultItemPadding defines default padding for new items
	defaultItemPadding FlexItemProperties

	// paddingTop, paddingBottom, paddingLeft, paddingRight define layout padding
	paddingTop    uint16
	paddingBottom uint16
	paddingLeft   uint16
	paddingRight  uint16
}

// NewFlexLayout creates a new flex layout widget.
// The parent parameter is optional (can be nil for root widgets).
// The rect parameter specifies the position and size of the layout.
// The direction parameter specifies the flex direction (row or column).
// Additional options can be passed to configure background, alignment, etc.
//
// Common options:
// - WithFlexJustify(justify) - sets justify content
// - WithFlexAlign(align) - sets align items
// - WithFlexPadding(top, bottom, left, right) - sets layout padding
// - WithFlexDefaultItemPadding(top, bottom, left, right) - sets default item padding
func NewFlexLayout(parent IWidget, rect gtv.TRect, direction FlexDirection, opts ...gtv.Option) *TFlexLayout {
	flexLayout := newFlexLayoutBase(parent, rect, direction, opts...)

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(flexLayout)
	}

	return flexLayout
}

// newFlexLayoutBase creates a flex layout widget without registering with parent.
// This is used internally by derived widgets to avoid double registration.
func newFlexLayoutBase(parent IWidget, rect gtv.TRect, direction FlexDirection, opts ...gtv.Option) *TFlexLayout {
	// Create base layout without background by default
	layoutBase := newLayoutBase(parent, rect, nil, true)

	flexLayout := &TFlexLayout{
		TLayout:            *layoutBase,
		direction:          direction,
		justifyContent:     FlexJustifyStart,
		alignItems:         FlexAlignStretch,
		itemProperties:     make(map[IWidget]FlexItemProperties),
		defaultItemPadding: FlexItemProperties{FlexShrink: 1.0},
		paddingTop:         0,
		paddingBottom:      0,
		paddingLeft:        0,
		paddingRight:       0,
	}

	// Apply options
	for _, opt := range opts {
		opt(flexLayout)
	}

	return flexLayout
}

// WithFlexJustify creates an option to set justify content.
func WithFlexJustify(justify FlexJustifyContent) gtv.Option {
	return func(w any) {
		if fl, ok := w.(*TFlexLayout); ok {
			fl.justifyContent = justify
		}
	}
}

// WithFlexAlign creates an option to set align items.
func WithFlexAlign(align FlexAlignItems) gtv.Option {
	return func(w any) {
		if fl, ok := w.(*TFlexLayout); ok {
			fl.alignItems = align
		}
	}
}

// WithFlexPadding creates an option to set layout padding.
func WithFlexPadding(top, bottom, left, right uint16) gtv.Option {
	return func(w any) {
		if fl, ok := w.(*TFlexLayout); ok {
			fl.paddingTop = top
			fl.paddingBottom = bottom
			fl.paddingLeft = left
			fl.paddingRight = right
		}
	}
}

// WithFlexDefaultItemPadding creates an option to set default item padding.
func WithFlexDefaultItemPadding(top, bottom, left, right uint16) gtv.Option {
	return func(w any) {
		if fl, ok := w.(*TFlexLayout); ok {
			fl.defaultItemPadding.PaddingTop = top
			fl.defaultItemPadding.PaddingBottom = bottom
			fl.defaultItemPadding.PaddingLeft = left
			fl.defaultItemPadding.PaddingRight = right
		}
	}
}

// WithFlexBackground creates an option to set background attributes.
func WithFlexBackground(attrs gtv.CellAttributes) gtv.Option {
	return func(w any) {
		if fl, ok := w.(*TFlexLayout); ok {
			// Default to flex-layout-background theme tag if no theme tag or colors are specified
			if attrs.ThemeTag == "" && attrs.TextColor == gtv.NoColor && attrs.BackColor == gtv.NoColor {
				attrs = gtv.CellTag("flex-layout-background")
			}
			fl.SetBackground(&attrs)
		}
	}
}

// SetDirection sets the flex direction (row or column).
func (fl *TFlexLayout) SetDirection(direction FlexDirection) {
	fl.direction = direction
	fl.performLayout()
}

// GetDirection returns the current flex direction.
func (fl *TFlexLayout) GetDirection() FlexDirection {
	return fl.direction
}

// SetJustifyContent sets how items are justified along the main axis.
func (fl *TFlexLayout) SetJustifyContent(justify FlexJustifyContent) {
	fl.justifyContent = justify
	fl.performLayout()
}

// GetJustifyContent returns the current justify content setting.
func (fl *TFlexLayout) GetJustifyContent() FlexJustifyContent {
	return fl.justifyContent
}

// SetAlignItems sets how items are aligned along the cross axis.
func (fl *TFlexLayout) SetAlignItems(align FlexAlignItems) {
	fl.alignItems = align
	fl.performLayout()
}

// GetAlignItems returns the current align items setting.
func (fl *TFlexLayout) GetAlignItems() FlexAlignItems {
	return fl.alignItems
}

// SetItemProperties sets flex properties for a specific child widget.
func (fl *TFlexLayout) SetItemProperties(child IWidget, props FlexItemProperties) {
	// Preserve FlexBasis if not explicitly set in new properties
	if props.FlexBasis == 0 {
		if oldProps, ok := fl.itemProperties[child]; ok {
			props.FlexBasis = oldProps.FlexBasis
		}
	}
	fl.itemProperties[child] = props
	fl.performLayout()
}

// GetItemProperties returns flex properties for a specific child widget.
// If the widget has no custom properties, returns default properties.
func (fl *TFlexLayout) GetItemProperties(child IWidget) FlexItemProperties {
	if props, ok := fl.itemProperties[child]; ok {
		return props
	}
	return fl.defaultItemPadding
}

// SetPadding sets padding for the layout.
func (fl *TFlexLayout) SetPadding(top, bottom, left, right uint16) {
	fl.paddingTop = top
	fl.paddingBottom = bottom
	fl.paddingLeft = left
	fl.paddingRight = right
	fl.performLayout()
}

// SetDefaultItemPadding sets default padding for new items.
func (fl *TFlexLayout) SetDefaultItemPadding(top, bottom, left, right uint16) {
	fl.defaultItemPadding.PaddingTop = top
	fl.defaultItemPadding.PaddingBottom = bottom
	fl.defaultItemPadding.PaddingLeft = left
	fl.defaultItemPadding.PaddingRight = right
}

// AddChild adds a child widget to the flex layout and performs layout.
func (fl *TFlexLayout) AddChild(child IWidget) {
	fl.TLayout.AddChild(child)
	// Initialize with default properties if not set
	if _, ok := fl.itemProperties[child]; !ok {
		// Capture initial size as FlexBasis
		childPos := child.GetPos()
		props := fl.defaultItemPadding
		if props.FlexBasis == 0 {
			// Store initial width as flex basis for row direction
			// For column direction, we'll use height
			if fl.direction == FlexDirectionRow {
				props.FlexBasis = childPos.W
			} else {
				props.FlexBasis = childPos.H
			}
		}
		fl.itemProperties[child] = props
	}
	fl.performLayout()
}

// RemoveChild removes a child widget from the flex layout and performs layout.
func (fl *TFlexLayout) RemoveChild(child IWidget) {
	// Remove from children list
	for i, c := range fl.Children {
		if c == child {
			fl.Children = append(fl.Children[:i], fl.Children[i+1:]...)
			break
		}
	}
	// Remove from item properties
	delete(fl.itemProperties, child)
	// Clear active child if it was the removed child
	if fl.ActiveChild == child {
		fl.ActiveChild = nil
	}
	fl.performLayout()
}

// ReplaceChild replaces an old child widget with a new one, preserving position and flex properties.
// If preserveProperties is true, the new child will inherit the old child's flex properties.
// Returns true if the replacement was successful, false if oldChild was not found.
func (fl *TFlexLayout) ReplaceChild(oldChild, newChild IWidget, preserveProperties bool) bool {
	// Find the old child's index
	index := -1
	for i, c := range fl.Children {
		if c == oldChild {
			index = i
			break
		}
	}

	if index == -1 {
		return false
	}

	// Preserve properties if requested
	var props FlexItemProperties
	if preserveProperties {
		if oldProps, ok := fl.itemProperties[oldChild]; ok {
			props = oldProps
		} else {
			props = fl.defaultItemPadding
		}
	} else {
		// Capture initial size as FlexBasis for new child
		childPos := newChild.GetPos()
		props = fl.defaultItemPadding
		if props.FlexBasis == 0 {
			if fl.direction == FlexDirectionRow {
				props.FlexBasis = childPos.W
			} else {
				props.FlexBasis = childPos.H
			}
		}
	}

	// Replace in children list
	fl.Children[index] = newChild

	// Update item properties
	delete(fl.itemProperties, oldChild)
	fl.itemProperties[newChild] = props

	// Update active child if it was the old child
	if fl.ActiveChild == oldChild {
		fl.ActiveChild = newChild
	}

	fl.performLayout()
	return true
}

// performLayout calculates and applies positions and sizes to all children.
func (fl *TFlexLayout) performLayout() {
	if len(fl.Children) == 0 {
		return
	}

	// Calculate available space (subtract layout padding)
	availableWidth := int(fl.Position.W) - int(fl.paddingLeft) - int(fl.paddingRight)
	availableHeight := int(fl.Position.H) - int(fl.paddingTop) - int(fl.paddingBottom)

	if availableWidth < 0 {
		availableWidth = 0
	}
	if availableHeight < 0 {
		availableHeight = 0
	}

	if fl.direction == FlexDirectionRow {
		fl.layoutRow(availableWidth, availableHeight)
	} else {
		fl.layoutColumn(availableWidth, availableHeight)
	}
}

// layoutRow performs layout for horizontal (row) direction.
func (fl *TFlexLayout) layoutRow(availableWidth, availableHeight int) {
	// Calculate sizes and positions
	type itemLayout struct {
		child  IWidget
		props  FlexItemProperties
		width  int
		height int
	}

	items := make([]itemLayout, 0, len(fl.Children))
	totalFlexGrow := 0.0
	totalFlexShrink := 0.0
	usedWidth := 0

	// First pass: calculate base sizes and collect flex properties
	for _, child := range fl.Children {
		props := fl.GetItemProperties(child)
		item := itemLayout{
			child: child,
			props: props,
		}

		// Calculate width (main axis)
		if props.FixedWidth > 0 {
			item.width = int(props.FixedWidth)
		} else {
			// Use FlexBasis as the base size
			item.width = int(props.FlexBasis)
		}

		// Add padding to width
		item.width += int(props.PaddingLeft) + int(props.PaddingRight)

		// Calculate height (cross axis)
		if props.FixedHeight > 0 {
			item.height = int(props.FixedHeight)
		} else {
			childPos := child.GetPos()
			item.height = int(childPos.H)
		}

		// Add padding to height
		item.height += int(props.PaddingTop) + int(props.PaddingBottom)

		usedWidth += item.width
		totalFlexGrow += props.FlexGrow
		totalFlexShrink += props.FlexShrink

		items = append(items, item)
	}

	// Second pass: distribute extra space or shrink
	extraSpace := availableWidth - usedWidth

	if extraSpace > 0 && totalFlexGrow > 0 {
		// Distribute extra space proportionally to flex-grow
		for i := range items {
			if items[i].props.FlexGrow > 0 {
				grow := int(float64(extraSpace) * items[i].props.FlexGrow / totalFlexGrow)
				items[i].width += grow

				// Apply max size constraint
				if items[i].props.MaxSize > 0 {
					contentWidth := items[i].width - int(items[i].props.PaddingLeft) - int(items[i].props.PaddingRight)
					if contentWidth > int(items[i].props.MaxSize) {
						items[i].width = int(items[i].props.MaxSize) + int(items[i].props.PaddingLeft) + int(items[i].props.PaddingRight)
					}
				}
			}
		}
	} else if extraSpace < 0 && totalFlexShrink > 0 {
		// Shrink items proportionally to flex-shrink
		shrinkSpace := -extraSpace
		for i := range items {
			if items[i].props.FlexShrink > 0 {
				shrink := int(float64(shrinkSpace) * items[i].props.FlexShrink / totalFlexShrink)
				items[i].width -= shrink

				// Apply min size constraint
				contentWidth := items[i].width - int(items[i].props.PaddingLeft) - int(items[i].props.PaddingRight)
				if contentWidth < int(items[i].props.MinSize) {
					items[i].width = int(items[i].props.MinSize) + int(items[i].props.PaddingLeft) + int(items[i].props.PaddingRight)
				}
			}
		}
	}

	// Calculate actual total width after grow/shrink
	actualTotalWidth := 0
	for i := range items {
		actualTotalWidth += items[i].width
	}

	// Third pass: calculate positions based on justifyContent
	x := int(fl.paddingLeft)
	spacing := 0
	startOffset := 0

	switch fl.justifyContent {
	case FlexJustifyStart:
		// Nothing to do, start from left
	case FlexJustifyEnd:
		startOffset = availableWidth - actualTotalWidth
	case FlexJustifyCenter:
		startOffset = (availableWidth - actualTotalWidth) / 2
	case FlexJustifySpaceBetween:
		if len(items) > 1 && actualTotalWidth <= availableWidth {
			spacing = (availableWidth - actualTotalWidth) / (len(items) - 1)
		}
	case FlexJustifySpaceAround:
		if actualTotalWidth <= availableWidth {
			spacing = (availableWidth - actualTotalWidth) / len(items)
			startOffset = spacing / 2
		}
	case FlexJustifySpaceEvenly:
		if actualTotalWidth <= availableWidth {
			spacing = (availableWidth - actualTotalWidth) / (len(items) + 1)
			startOffset = spacing
		}
	}

	x += startOffset

	// Fourth pass: position and size children
	for i := range items {
		item := items[i]

		// Calculate cross-axis position and size based on alignItems
		y := int(fl.paddingTop)
		height := item.height

		// Check if item has fixed height - if so, don't stretch
		hasFixedHeight := item.props.FixedHeight > 0

		switch fl.alignItems {
		case FlexAlignStart:
			// Align to top
		case FlexAlignEnd:
			// Align to bottom
			y = availableHeight - height + int(fl.paddingTop)
		case FlexAlignCenter:
			// Center vertically
			y = (availableHeight-height)/2 + int(fl.paddingTop)
		case FlexAlignStretch:
			// Stretch to fill height (unless item has fixed height)
			if !hasFixedHeight {
				height = availableHeight
			}
		}

		// Calculate content size (excluding padding)
		contentWidth := item.width - int(item.props.PaddingLeft) - int(item.props.PaddingRight)
		contentHeight := height - int(item.props.PaddingTop) - int(item.props.PaddingBottom)

		if contentWidth < 0 {
			contentWidth = 0
		}
		if contentHeight < 0 {
			contentHeight = 0
		}

		// Send resize event to child
		resizeEvent := &TEvent{
			Type: TEventTypeResize,
			Rect: gtv.TRect{
				X: uint16(x + int(item.props.PaddingLeft)),
				Y: uint16(y + int(item.props.PaddingTop)),
				W: uint16(contentWidth),
				H: uint16(contentHeight),
			},
		}
		item.child.HandleEvent(resizeEvent)

		x += item.width + spacing
	}
}

// layoutColumn performs layout for vertical (column) direction.
func (fl *TFlexLayout) layoutColumn(availableWidth, availableHeight int) {
	// Calculate sizes and positions
	type itemLayout struct {
		child  IWidget
		props  FlexItemProperties
		width  int
		height int
	}

	items := make([]itemLayout, 0, len(fl.Children))
	totalFlexGrow := 0.0
	totalFlexShrink := 0.0
	usedHeight := 0

	// First pass: calculate base sizes and collect flex properties
	for _, child := range fl.Children {
		props := fl.GetItemProperties(child)
		item := itemLayout{
			child: child,
			props: props,
		}

		// Calculate height (main axis)
		if props.FixedHeight > 0 {
			item.height = int(props.FixedHeight)
		} else {
			// Use FlexBasis as the base size
			item.height = int(props.FlexBasis)
		}

		// Add padding to height
		item.height += int(props.PaddingTop) + int(props.PaddingBottom)

		// Calculate width (cross axis)
		if props.FixedWidth > 0 {
			item.width = int(props.FixedWidth)
		} else {
			childPos := child.GetPos()
			item.width = int(childPos.W)
		}

		// Add padding to width
		item.width += int(props.PaddingLeft) + int(props.PaddingRight)

		usedHeight += item.height
		totalFlexGrow += props.FlexGrow
		totalFlexShrink += props.FlexShrink

		items = append(items, item)
	}

	// Second pass: distribute extra space or shrink
	extraSpace := availableHeight - usedHeight

	if extraSpace > 0 && totalFlexGrow > 0 {
		// Distribute extra space proportionally to flex-grow
		for i := range items {
			if items[i].props.FlexGrow > 0 {
				grow := int(float64(extraSpace) * items[i].props.FlexGrow / totalFlexGrow)
				items[i].height += grow

				// Apply max size constraint
				if items[i].props.MaxSize > 0 {
					contentHeight := items[i].height - int(items[i].props.PaddingTop) - int(items[i].props.PaddingBottom)
					if contentHeight > int(items[i].props.MaxSize) {
						items[i].height = int(items[i].props.MaxSize) + int(items[i].props.PaddingTop) + int(items[i].props.PaddingBottom)
					}
				}
			}
		}
	} else if extraSpace < 0 && totalFlexShrink > 0 {
		// Shrink items proportionally to flex-shrink
		shrinkSpace := -extraSpace
		for i := range items {
			if items[i].props.FlexShrink > 0 {
				shrink := int(float64(shrinkSpace) * items[i].props.FlexShrink / totalFlexShrink)
				items[i].height -= shrink

				// Apply min size constraint
				contentHeight := items[i].height - int(items[i].props.PaddingTop) - int(items[i].props.PaddingBottom)
				if contentHeight < int(items[i].props.MinSize) {
					items[i].height = int(items[i].props.MinSize) + int(items[i].props.PaddingTop) + int(items[i].props.PaddingBottom)
				}
			}
		}
	}

	// Calculate actual total height after grow/shrink
	actualTotalHeight := 0
	for i := range items {
		actualTotalHeight += items[i].height
	}

	// Third pass: calculate positions based on justifyContent
	y := int(fl.paddingTop)
	spacing := 0
	startOffset := 0

	switch fl.justifyContent {
	case FlexJustifyStart:
		// Nothing to do, start from top
	case FlexJustifyEnd:
		startOffset = availableHeight - actualTotalHeight
	case FlexJustifyCenter:
		startOffset = (availableHeight - actualTotalHeight) / 2
	case FlexJustifySpaceBetween:
		if len(items) > 1 && actualTotalHeight <= availableHeight {
			spacing = (availableHeight - actualTotalHeight) / (len(items) - 1)
		}
	case FlexJustifySpaceAround:
		if actualTotalHeight <= availableHeight {
			spacing = (availableHeight - actualTotalHeight) / len(items)
			startOffset = spacing / 2
		}
	case FlexJustifySpaceEvenly:
		if actualTotalHeight <= availableHeight {
			spacing = (availableHeight - actualTotalHeight) / (len(items) + 1)
			startOffset = spacing
		}
	}

	y += startOffset

	// Fourth pass: position and size children
	for i := range items {
		item := items[i]

		// Calculate cross-axis position and size based on alignItems
		x := int(fl.paddingLeft)
		width := item.width

		// Check if item has fixed width - if so, don't stretch
		hasFixedWidth := item.props.FixedWidth > 0

		switch fl.alignItems {
		case FlexAlignStart:
			// Align to left
		case FlexAlignEnd:
			// Align to right
			x = availableWidth - width + int(fl.paddingLeft)
		case FlexAlignCenter:
			// Center horizontally
			x = (availableWidth-width)/2 + int(fl.paddingLeft)
		case FlexAlignStretch:
			// Stretch to fill width (unless item has fixed width)
			if !hasFixedWidth {
				width = availableWidth
			}
		}

		// Calculate content size (excluding padding)
		contentWidth := width - int(item.props.PaddingLeft) - int(item.props.PaddingRight)
		contentHeight := item.height - int(item.props.PaddingTop) - int(item.props.PaddingBottom)

		if contentWidth < 0 {
			contentWidth = 0
		}
		if contentHeight < 0 {
			contentHeight = 0
		}

		// Send resize event to child
		resizeEvent := &TEvent{
			Type: TEventTypeResize,
			Rect: gtv.TRect{
				X: uint16(x + int(item.props.PaddingLeft)),
				Y: uint16(y + int(item.props.PaddingTop)),
				W: uint16(contentWidth),
				H: uint16(contentHeight),
			},
		}
		item.child.HandleEvent(resizeEvent)

		y += item.height + spacing
	}
}

// Draw draws the flex layout on the screen.
func (fl *TFlexLayout) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if fl.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Draw background and children using base layout
	fl.TLayout.Draw(screen)
}

// HandleEvent handles events for the flex layout.
func (fl *TFlexLayout) HandleEvent(event *TEvent) {
	// Handle resize events directly
	if event.Type == TEventTypeResize {
		fl.handleResizeEvent(event)
		fl.performLayout()
		return
	}

	// Delegate other events to base layout
	fl.TLayout.HandleEvent(event)
}

// OnResize is called when the flex layout is resized.
func (fl *TFlexLayout) OnResize(oldRect, newRect gtv.TRect) {
	fl.performLayout()
}
