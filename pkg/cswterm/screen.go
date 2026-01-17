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

// InputEventType represents general type of the input event.
type InputEventType uint16

const (
	// InputEventKey represents a keyboard event
	InputEventKey InputEventType = iota
	// InputEventMouse represents a mouse event
	InputEventMouse
	// InputEventResize represents terminal resize event
	InputEventResize
	// InputEventPaste represents a paste event
	InputEventPaste
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

	// Modifiers is a bitfield of modifiers
	Modifiers EventModifiers

	// X and Y are coordinates of the mouse event or width and height for resize event
	X, Y uint16
}

// Screen represents a terminal screen. It consists of a grid of cells.
// Each cell contains a rune and a set of attributes.
type Screen interface {

	// Size returns the size of the screen in characters.
	Size() (width int, height int)

	// PutText puts text at the specified position with the specified attributes.
	// if the text is longer than the width of the screen, it is truncated.
	PutText(x int, y int, text string, attrs TextAttributes)

	// Notify notifies the screen about an input event.
	Notify(event InputEvent)

	// Listen registers a channel to receive input events.
	Listen(ch chan InputEvent)
}
