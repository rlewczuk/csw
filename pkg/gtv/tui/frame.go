package tui

import (
	"github.com/rlewczuk/csw/pkg/gtv"
)

// BorderStyle defines the style of the frame border
type BorderStyle uint8

const (
	// BorderStyleNone represents no border (borderless frame)
	BorderStyleNone BorderStyle = iota
	// BorderStyleSingle represents a single-line border
	BorderStyleSingle
	// BorderStyleDouble represents a double-line border
	BorderStyleDouble
	// BorderStyleRounded represents a rounded border
	BorderStyleRounded
	// BorderStyleBold represents a bold border
	BorderStyleBold
)

// TitlePosition defines where the title is displayed on the top border
type TitlePosition uint8

const (
	// TitlePositionLeft aligns the title to the left
	TitlePositionLeft TitlePosition = iota
	// TitlePositionCenter centers the title
	TitlePositionCenter
	// TitlePositionRight aligns the title to the right
	TitlePositionRight
)

// IconPosition defines where icons are displayed on the top border
type IconPosition uint8

const (
	// IconPositionLeft displays icons on the left side of the top border
	IconPositionLeft IconPosition = iota
	// IconPositionRight displays icons on the right side of the top border
	IconPositionRight
)

// FrameIcon represents an icon displayed on the frame border
type FrameIcon struct {
	// ID is a unique identifier for this icon
	ID int
	// Rune is the Unicode character to display
	Rune rune
	// Color is the normal color of the icon
	Color gtv.TextColor
	// HoverColor is the color when mouse is over the icon (optional, uses Color if not set)
	HoverColor gtv.TextColor
	// Position determines if icon is on left or right side
	Position IconPosition
	// Handler is called when the icon is clicked (optional)
	Handler func()
	// Hidden determines if the icon is visible
	Hidden bool
}

// IFrame is an interface for frame widgets that draw borders around child widgets.
// It extends IResizable with frame-specific methods.
type IFrame interface {
	IResizable

	// SetTitle sets the title text displayed on the top border
	SetTitle(title string)

	// GetTitle returns the current title text
	GetTitle() string

	// SetTitlePosition sets the position of the title on the top border
	SetTitlePosition(pos TitlePosition)

	// GetTitlePosition returns the current title position
	GetTitlePosition() TitlePosition

	// SetBorderStyle sets the border style
	SetBorderStyle(style BorderStyle)

	// GetBorderStyle returns the current border style
	GetBorderStyle() BorderStyle

	// AddIcon adds an icon to the frame and returns its ID
	AddIcon(icon *FrameIcon) int

	// ShowIcon shows an icon by ID
	ShowIcon(id int)

	// HideIcon hides an icon by ID
	HideIcon(id int)

	// GetIcon returns an icon by ID (nil if not found)
	GetIcon(id int) *FrameIcon

	// GetAttrs returns the normal frame attributes
	GetAttrs() gtv.CellAttributes

	// SetAttrs sets the normal frame attributes
	SetAttrs(attrs gtv.CellAttributes)

	// GetFocusedAttrs returns the focused frame attributes
	GetFocusedAttrs() gtv.CellAttributes

	// SetFocusedAttrs sets the focused frame attributes
	SetFocusedAttrs(attrs gtv.CellAttributes)

	// IsFocused returns true if the frame is focused
	IsFocused() bool
}

// TFrame is a widget that draws a border around a single child widget.
// It acts as a layout with a single child and supports:
// - Configurable border styles (single, double, rounded, etc.)
// - Optional title with configurable position
// - Optional icons with handlers
// - Focus support with different theme tags for normal and focused states
type TFrame struct {
	TResizable

	// child is the single child widget
	child IWidget

	// borderStyle defines the border style
	borderStyle BorderStyle

	// title is the text displayed on the top border
	title string

	// titlePosition defines where the title is displayed
	titlePosition TitlePosition

	// icons stores all icons added to the frame
	icons []*FrameIcon

	// nextIconID is the next ID to assign to an icon
	nextIconID int

	// focusedAttrs are the cell attributes used when frame is focused
	focusedAttrs gtv.CellAttributes

	// focused tracks if the frame is currently focused
	focused bool

	// mouseOverIconID tracks which icon the mouse is currently over (-1 if none)
	mouseOverIconID int
}

// borderChars holds the characters used for drawing a border
type borderChars struct {
	topLeft     rune
	topRight    rune
	bottomLeft  rune
	bottomRight rune
	horizontal  rune
	vertical    rune
}

// getBorderChars returns the border characters for the given style
func getBorderChars(style BorderStyle) borderChars {
	switch style {
	case BorderStyleSingle:
		return borderChars{
			topLeft:     '┌',
			topRight:    '┐',
			bottomLeft:  '└',
			bottomRight: '┘',
			horizontal:  '─',
			vertical:    '│',
		}
	case BorderStyleDouble:
		return borderChars{
			topLeft:     '╔',
			topRight:    '╗',
			bottomLeft:  '╚',
			bottomRight: '╝',
			horizontal:  '═',
			vertical:    '║',
		}
	case BorderStyleRounded:
		return borderChars{
			topLeft:     '╭',
			topRight:    '╮',
			bottomLeft:  '╰',
			bottomRight: '╯',
			horizontal:  '─',
			vertical:    '│',
		}
	case BorderStyleBold:
		return borderChars{
			topLeft:     '┏',
			topRight:    '┓',
			bottomLeft:  '┗',
			bottomRight: '┛',
			horizontal:  '━',
			vertical:    '┃',
		}
	default: // BorderStyleNone
		return borderChars{}
	}
}

// NewFrame creates a new frame widget with the specified border style.
// The parent parameter is optional (can be nil for root widgets).
// The rect parameter specifies the position and size of the frame.
// The borderStyle parameter specifies the border style.
// The attrs parameter specifies the normal frame attributes (use CellTag for theme support).
// The focusedAttrs parameter specifies the focused frame attributes (optional, uses attrs if empty).
func NewFrame(parent IWidget, rect gtv.TRect, borderStyle BorderStyle, attrs gtv.CellAttributes, focusedAttrs ...gtv.CellAttributes) *TFrame {
	// Default to "frame" theme tag if no theme tag or colors are specified
	if attrs.ThemeTag == "" && attrs.TextColor == gtv.NoColor && attrs.BackColor == gtv.NoColor {
		attrs = gtv.CellTag("frame")
	}

	frame := &TFrame{
		TResizable:      *newResizableBase(parent, rect),
		borderStyle:     borderStyle,
		titlePosition:   TitlePositionLeft,
		icons:           make([]*FrameIcon, 0),
		nextIconID:      1,
		mouseOverIconID: -1,
	}

	// Set normal attributes
	frame.TResizable.TWidget.cellAttrs = attrs

	// Set focused attributes
	if len(focusedAttrs) > 0 {
		frame.focusedAttrs = focusedAttrs[0]
	} else {
		// Default to "frame-focused" theme tag if no focused attrs specified
		if attrs.ThemeTag != "" {
			frame.focusedAttrs = gtv.CellTag(attrs.ThemeTag + "-focused")
		} else {
			frame.focusedAttrs = attrs
		}
	}

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(frame)
	}

	return frame
}

// SetChild sets the single child widget for the frame.
// If a child already exists, it is replaced.
func (f *TFrame) SetChild(child IWidget) {
	f.child = child
	f.Children = []IWidget{child}
	f.updateChildPosition()
}

// GetChild returns the current child widget (nil if none)
func (f *TFrame) GetChild() IWidget {
	return f.child
}

// updateChildPosition updates the child's position to fit inside the frame border
func (f *TFrame) updateChildPosition() {
	if f.child == nil {
		return
	}

	// Calculate inner area (excluding border)
	var innerRect gtv.TRect
	if f.borderStyle == BorderStyleNone {
		// No border - child fills entire frame
		innerRect = gtv.TRect{X: 0, Y: 0, W: f.Position.W, H: f.Position.H}
	} else {
		// With border - child is inside border (1 cell on each side)
		if f.Position.W >= 2 && f.Position.H >= 2 {
			innerRect = gtv.TRect{
				X: 1,
				Y: 1,
				W: f.Position.W - 2,
				H: f.Position.H - 2,
			}
		} else {
			// Frame too small for border
			innerRect = gtv.TRect{X: 0, Y: 0, W: 0, H: 0}
		}
	}

	// Send resize event to child
	resizeEvent := &TEvent{
		Type: TEventTypeResize,
		Rect: innerRect,
	}
	f.child.HandleEvent(resizeEvent)
}

// AddChild overrides TWidget.AddChild to only allow one child
func (f *TFrame) AddChild(child IWidget) {
	f.SetChild(child)
}

// SetTitle sets the title text displayed on the top border
func (f *TFrame) SetTitle(title string) {
	if f.title != title {
		f.title = title
	}
}

// GetTitle returns the current title text
func (f *TFrame) GetTitle() string {
	return f.title
}

// SetTitlePosition sets the position of the title on the top border
func (f *TFrame) SetTitlePosition(pos TitlePosition) {
	f.titlePosition = pos
}

// GetTitlePosition returns the current title position
func (f *TFrame) GetTitlePosition() TitlePosition {
	return f.titlePosition
}

// SetBorderStyle sets the border style
func (f *TFrame) SetBorderStyle(style BorderStyle) {
	if f.borderStyle != style {
		f.borderStyle = style
		f.updateChildPosition()
	}
}

// GetBorderStyle returns the current border style
func (f *TFrame) GetBorderStyle() BorderStyle {
	return f.borderStyle
}

// AddIcon adds an icon to the frame and returns its ID
func (f *TFrame) AddIcon(icon *FrameIcon) int {
	if icon == nil {
		return -1
	}

	icon.ID = f.nextIconID
	f.nextIconID++
	f.icons = append(f.icons, icon)
	return icon.ID
}

// ShowIcon shows an icon by ID
func (f *TFrame) ShowIcon(id int) {
	for _, icon := range f.icons {
		if icon.ID == id {
			icon.Hidden = false
			return
		}
	}
}

// HideIcon hides an icon by ID
func (f *TFrame) HideIcon(id int) {
	for _, icon := range f.icons {
		if icon.ID == id {
			icon.Hidden = true
			return
		}
	}
}

// GetIcon returns an icon by ID (nil if not found)
func (f *TFrame) GetIcon(id int) *FrameIcon {
	for _, icon := range f.icons {
		if icon.ID == id {
			return icon
		}
	}
	return nil
}

// GetAttrs returns the normal frame attributes
func (f *TFrame) GetAttrs() gtv.CellAttributes {
	return f.TResizable.TWidget.GetAttrs()
}

// SetAttrs sets the normal frame attributes
func (f *TFrame) SetAttrs(attrs gtv.CellAttributes) {
	f.TResizable.TWidget.SetAttrs(attrs)
}

// GetFocusedAttrs returns the focused frame attributes
func (f *TFrame) GetFocusedAttrs() gtv.CellAttributes {
	return f.focusedAttrs
}

// SetFocusedAttrs sets the focused frame attributes
func (f *TFrame) SetFocusedAttrs(attrs gtv.CellAttributes) {
	f.focusedAttrs = attrs
}

// IsFocused returns true if the frame is focused
func (f *TFrame) IsFocused() bool {
	return f.focused
}

// OnResize is called when the frame is resized
func (f *TFrame) OnResize(oldRect, newRect gtv.TRect) {
	f.updateChildPosition()
}

// Draw draws the frame and its child on the screen
func (f *TFrame) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if f.Flags&WidgetFlagHidden != 0 {
		return
	}

	// Get absolute position
	absPos := f.GetAbsolutePos()

	// Skip drawing if frame has no size
	if absPos.W == 0 || absPos.H == 0 {
		return
	}

	// Draw border if not borderless
	if f.borderStyle != BorderStyleNone {
		f.drawBorder(screen, absPos)
	}

	// Draw child
	if f.child != nil {
		f.child.Draw(screen)
	}
}

// drawBorder draws the frame border
func (f *TFrame) drawBorder(screen gtv.IScreenOutput, absPos gtv.TRect) {
	// Get border characters
	chars := getBorderChars(f.borderStyle)

	// Get attributes (use focused attributes if focused)
	attrs := f.TResizable.TWidget.cellAttrs
	if f.focused && f.focusedAttrs.ThemeTag != "" {
		attrs = f.focusedAttrs
	} else if f.focused && (f.focusedAttrs.TextColor != gtv.NoColor || f.focusedAttrs.BackColor != gtv.NoColor) {
		attrs = f.focusedAttrs
	}

	// Draw corners
	screen.PutContent(gtv.TRect{X: absPos.X, Y: absPos.Y, W: 1, H: 1},
		[]gtv.Cell{{Rune: chars.topLeft, Attrs: attrs}})
	screen.PutContent(gtv.TRect{X: absPos.X + absPos.W - 1, Y: absPos.Y, W: 1, H: 1},
		[]gtv.Cell{{Rune: chars.topRight, Attrs: attrs}})
	screen.PutContent(gtv.TRect{X: absPos.X, Y: absPos.Y + absPos.H - 1, W: 1, H: 1},
		[]gtv.Cell{{Rune: chars.bottomLeft, Attrs: attrs}})
	screen.PutContent(gtv.TRect{X: absPos.X + absPos.W - 1, Y: absPos.Y + absPos.H - 1, W: 1, H: 1},
		[]gtv.Cell{{Rune: chars.bottomRight, Attrs: attrs}})

	// Draw top and bottom borders (with title and icons)
	f.drawTopBorder(screen, absPos, chars, attrs)
	f.drawBottomBorder(screen, absPos, chars, attrs)

	// Draw left and right borders
	for y := uint16(1); y < absPos.H-1; y++ {
		screen.PutContent(gtv.TRect{X: absPos.X, Y: absPos.Y + y, W: 1, H: 1},
			[]gtv.Cell{{Rune: chars.vertical, Attrs: attrs}})
		screen.PutContent(gtv.TRect{X: absPos.X + absPos.W - 1, Y: absPos.Y + y, W: 1, H: 1},
			[]gtv.Cell{{Rune: chars.vertical, Attrs: attrs}})
	}
}

// drawTopBorder draws the top border with title and icons
func (f *TFrame) drawTopBorder(screen gtv.IScreenOutput, absPos gtv.TRect, chars borderChars, attrs gtv.CellAttributes) {
	if absPos.W < 2 {
		return
	}

	// Available width for content (excluding corners)
	availableWidth := int(absPos.W - 2)

	// Collect visible icons
	leftIcons := make([]*FrameIcon, 0)
	rightIcons := make([]*FrameIcon, 0)
	for _, icon := range f.icons {
		if !icon.Hidden {
			if icon.Position == IconPositionLeft {
				leftIcons = append(leftIcons, icon)
			} else {
				rightIcons = append(rightIcons, icon)
			}
		}
	}

	// Calculate positions
	// Format: corner [2 spaces] [left icons with spaces] [2 spaces] [title] [2 spaces] [right icons with spaces] [2 spaces] corner

	// Calculate left icons width (each icon + space, plus 2 spaces gap)
	leftIconsWidth := 0
	if len(leftIcons) > 0 {
		leftIconsWidth = len(leftIcons) + 2 // icons + gap
	}

	// Calculate right icons width
	rightIconsWidth := 0
	if len(rightIcons) > 0 {
		rightIconsWidth = len(rightIcons) + 2 // icons + gap
	}

	// Calculate title width
	titleRunes := []rune(f.title)
	titleWidth := len(titleRunes)

	// Total required width
	totalRequired := leftIconsWidth + titleWidth + rightIconsWidth
	if titleWidth > 0 && (leftIconsWidth > 0 || rightIconsWidth > 0) {
		totalRequired += 2 // gap between title and icons
	}

	// Create top border content
	content := make([]gtv.Cell, availableWidth)
	for i := 0; i < availableWidth; i++ {
		content[i] = gtv.Cell{Rune: chars.horizontal, Attrs: attrs}
	}

	// Draw left icons
	pos := 2
	if len(leftIcons) > 0 && pos < availableWidth {
		for _, icon := range leftIcons {
			if pos >= availableWidth {
				break
			}
			iconAttrs := attrs
			if icon.Color != gtv.NoColor {
				iconAttrs.TextColor = icon.Color
			}
			if f.mouseOverIconID == icon.ID && icon.HoverColor != gtv.NoColor {
				iconAttrs.TextColor = icon.HoverColor
			}
			content[pos] = gtv.Cell{Rune: icon.Rune, Attrs: iconAttrs}
			pos++
		}
		pos += 2 // gap after icons
	}

	// Draw title
	if titleWidth > 0 && pos < availableWidth {
		titleStartPos := pos

		// Adjust position based on title position
		if f.titlePosition == TitlePositionCenter {
			// Center the title in available space
			remainingWidth := availableWidth - leftIconsWidth - rightIconsWidth
			if titleWidth < remainingWidth {
				titleStartPos = leftIconsWidth + (remainingWidth-titleWidth)/2
			}
		} else if f.titlePosition == TitlePositionRight {
			// Right-align the title (with 2-cell gap from right edge)
			if rightIconsWidth > 0 {
				titleStartPos = availableWidth - rightIconsWidth - titleWidth - 2 // gap before icons
			} else {
				titleStartPos = availableWidth - titleWidth - 1 // 1 less for proper right alignment
			}
		}

		// Ensure title starts within bounds
		if titleStartPos < 0 {
			titleStartPos = 0
		}
		if titleStartPos > availableWidth {
			titleStartPos = availableWidth
		}

		// Draw title characters
		for i, ch := range titleRunes {
			if titleStartPos+i >= availableWidth {
				break
			}
			content[titleStartPos+i] = gtv.Cell{Rune: ch, Attrs: attrs}
		}
	}

	// Draw right icons
	if len(rightIcons) > 0 {
		pos := availableWidth - rightIconsWidth
		if pos < 0 {
			pos = 0
		}
		for _, icon := range rightIcons {
			if pos >= availableWidth {
				break
			}
			iconAttrs := attrs
			if icon.Color != gtv.NoColor {
				iconAttrs.TextColor = icon.Color
			}
			if f.mouseOverIconID == icon.ID && icon.HoverColor != gtv.NoColor {
				iconAttrs.TextColor = icon.HoverColor
			}
			content[pos] = gtv.Cell{Rune: icon.Rune, Attrs: iconAttrs}
			pos++
		}
	}

	// Put the top border content
	screen.PutContent(gtv.TRect{X: absPos.X + 1, Y: absPos.Y, W: uint16(availableWidth), H: 1}, content)
}

// drawBottomBorder draws the bottom border
func (f *TFrame) drawBottomBorder(screen gtv.IScreenOutput, absPos gtv.TRect, chars borderChars, attrs gtv.CellAttributes) {
	if absPos.W < 2 {
		return
	}

	// Bottom border is just horizontal line
	availableWidth := int(absPos.W - 2)
	content := make([]gtv.Cell, availableWidth)
	for i := 0; i < availableWidth; i++ {
		content[i] = gtv.Cell{Rune: chars.horizontal, Attrs: attrs}
	}
	screen.PutContent(gtv.TRect{X: absPos.X + 1, Y: absPos.Y + absPos.H - 1, W: uint16(availableWidth), H: 1}, content)
}

// HandleEvent handles events for the frame
func (f *TFrame) HandleEvent(event *TEvent) {
	// Handle resize events
	if event.Type == TEventTypeResize {
		// Store old position
		oldPos := f.Position

		// Update position
		f.Position.X = event.Rect.X
		f.Position.Y = event.Rect.Y
		f.Position.W = event.Rect.W
		f.Position.H = event.Rect.H

		// Call OnResize to update child position
		f.OnResize(oldPos, f.Position)
		return
	}

	// Handle input events
	if event.Type == TEventTypeInput && event.InputEvent != nil {
		inputEvent := event.InputEvent

		// Handle focus/blur events
		if inputEvent.Type == gtv.InputEventFocus {
			f.focused = true
			// Forward to child if it can receive focus
			if f.child != nil {
				if _, ok := f.child.(IFocusable); ok {
					f.child.HandleEvent(event)
				}
			}
			return
		}
		if inputEvent.Type == gtv.InputEventBlur {
			f.focused = false
			// Forward to child
			if f.child != nil {
				if _, ok := f.child.(IFocusable); ok {
					f.child.HandleEvent(event)
				}
			}
			return
		}

		// Handle mouse events for icons
		if inputEvent.Type == gtv.InputEventMouse {
			absPos := f.GetAbsolutePos()

			// Check if mouse is over top border (where icons are)
			if inputEvent.Y == absPos.Y && inputEvent.X >= absPos.X+1 && inputEvent.X < absPos.X+absPos.W-1 {
				// Check if mouse is over an icon
				f.handleIconMouse(inputEvent, absPos)
				return
			} else {
				// Mouse not over top border - clear hover state
				f.mouseOverIconID = -1
			}

			// Forward mouse event to child if inside child area
			if f.child != nil {
				f.child.HandleEvent(event)
			}
			return
		}
	}

	// Forward other events to child
	if f.child != nil {
		f.child.HandleEvent(event)
	}
}

// handleIconMouse handles mouse events for icons on the top border
func (f *TFrame) handleIconMouse(event *gtv.InputEvent, absPos gtv.TRect) {
	// Collect visible icons with their positions
	leftIcons := make([]*FrameIcon, 0)
	rightIcons := make([]*FrameIcon, 0)
	for _, icon := range f.icons {
		if !icon.Hidden {
			if icon.Position == IconPositionLeft {
				leftIcons = append(leftIcons, icon)
			} else {
				rightIcons = append(rightIcons, icon)
			}
		}
	}

	// Calculate icon positions (same logic as drawTopBorder)
	availableWidth := int(absPos.W - 2)

	// Left icons start at position 2
	pos := absPos.X + 1 + 2
	for _, icon := range leftIcons {
		if event.X == pos {
			// Mouse over this icon
			f.mouseOverIconID = icon.ID

			// Handle click
			if event.Modifiers&gtv.ModClick != 0 && icon.Handler != nil {
				icon.Handler()
			}
			return
		}
		pos++
	}

	// Right icons
	rightIconsWidth := 0
	if len(rightIcons) > 0 {
		rightIconsWidth = len(rightIcons) + 2
	}
	pos = absPos.X + 1 + uint16(availableWidth) - uint16(rightIconsWidth)
	for _, icon := range rightIcons {
		if event.X == pos {
			// Mouse over this icon
			f.mouseOverIconID = icon.ID

			// Handle click
			if event.Modifiers&gtv.ModClick != 0 && icon.Handler != nil {
				icon.Handler()
			}
			return
		}
		pos++
	}

	// Not over any icon
	f.mouseOverIconID = -1
}
