package cswterm

type TextAttributes uint32

const (
	AttrBold TextAttributes = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrDoubleUnderline
	AttrCurlyUnderline
	AttrDottedUnderline
	AttrDashedUnderline
	AttrBlink
	AttrReverse
	AttrHidden
	AttrStrikethrough
	Overline
)

type TextColor uint32

type CellAttributes struct {
	// Attrs is a bitfield of attributes. It is a combination of the following:
	Attributes TextAttributes
	// Foreground sets color of the text. It is 24-bit color in RGB format.
	TextColor uint32
	// Background sets color of the background. It is 24-bit color in RGB format.
	BackColor uint32
	// StrikeColor sets color of the strike through line. It is 24-bit color in RGB format.
	StrikeColor uint32
}

// Attrs creates CellAttributes with only text attributes (no colors).
func Attrs(attrs TextAttributes) CellAttributes {
	return CellAttributes{Attributes: attrs}
}

// AttrsWithColor creates CellAttributes with text attributes and colors.
func AttrsWithColor(attrs TextAttributes, textColor, backColor uint32) CellAttributes {
	return CellAttributes{
		Attributes: attrs,
		TextColor:  textColor,
		BackColor:  backColor,
	}
}

// InputEventType represents general type of the input event.
type InputEventType uint16

const (
	// InputEventKey represents a keyboard event
	InputEventKey InputEventType = iota
	// InputEventMouse represents a mouse event
	InputEventMouse
	// InputEventResize represents terminal resize event
	InputEventResize
	// InputEventCopy represents a copy event
	InputEventCopy
	// InputEventPaste represents a paste event
	InputEventPaste
	// InputEventFocus represents a focus event
	InputEventFocus
	// InputEventBlur represents loss of focus event
	InputEventBlur
)

// EventModifiers represents modifiers of the input event.
type EventModifiers uint16

const (
	// ModShift represents Shift modifier
	ModShift EventModifiers = 1 << iota
	// ModAlt represents Alt modifier
	ModAlt
	// ModCtrl represents Ctrl modifier
	ModCtrl
	// ModMeta represents Meta modifier
	ModMeta
	// ModClick represents mouse click modifier
	ModClick
	// ModDoubleClick represents double click modifier
	ModDoubleClick
	// ModDrag represents mouse drag modifier
	ModDrag
	// ModPress represents mouse or key press modifier
	ModPress
	// ModRelease represents mouse release modifier
	ModRelease
	// ModMove represents mouse move modifier
	ModMove
	// ModScrollUp represents mouse scroll up modifier
	ModScrollUp
	// ModScrollDown represents mouse scroll down modifier
	ModScrollDown
	// ModFn represents function key modifier (F1, F2, etc.)
	ModFn
)

// InputEvent represents an input event from the terminal.
type InputEvent struct {
	// Type is a general type of the input event.
	Type InputEventType

	// Key is a key code for keyboard events.
	Key rune

	// Content is a content of the copy/paste event
	Content string

	// Modifiers is a bitfield of modifiers
	Modifiers EventModifiers

	// X and Y are coordinates of the mouse event or width and height for resize event
	X, Y uint16
}

// ScreenOutput represents a terminal screen. It consists of a grid of cells.
// Each cell contains a rune and a set of attributes.
type ScreenOutput interface {

	// Size returns the size of the screen in characters.
	Size() (width int, height int)

	// GetContent returns the whole content of the screen.
	// Returns width, height, and the internal buffer array.
	// The content is a single dimensional array where index = y*width + x.
	GetContent() (width int, height int, content []Cell)

	// PutText puts text at the specified position with the specified attributes.
	// if the text is longer than the width of the screen, it is truncated.
	PutText(x int, y int, text string, attrs CellAttributes)
}

// InputEventHandler represents a terminal screen input handler.
// It is responsible for handling input events from the terminal.
// Events representation is agnostic to the underlying terminal implementation.
type InputEventHandler interface {
	// Notify notifies the screen about an input event.
	Notify(event InputEvent)
}

type InputEventSource interface {
	// Listen registers a channel to receive input events.
	Listen(ch chan InputEvent)
}
