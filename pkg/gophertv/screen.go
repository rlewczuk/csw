package gophertv

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

// Cell represents a single character cell in the screen buffer.
type Cell struct {
	Rune  rune
	Attrs CellAttributes
}

// TRect represents a rectangle on the screen.
type TRect struct {
	X, Y, W, H uint16
}

func (r TRect) Contains(x, y uint16) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// RelativeTo converts relative-to-parent coordinates to absolute-to-parent coordinates.
// If the relative rectangle would overflow the parent, it is clipped.
func (r TRect) RelativeTo(parent TRect) TRect {
	w := r.W
	if w+r.X > parent.W {
		if r.X >= parent.W {
			// Child starts beyond parent's right edge
			w = 0
		} else {
			w = parent.W - r.X
		}
	}
	h := r.H
	if h+r.Y > parent.H {
		if r.Y >= parent.H {
			// Child starts beyond parent's bottom edge
			h = 0
		} else {
			h = parent.H - r.Y
		}
	}

	return TRect{
		X: r.X + parent.X,
		Y: r.Y + parent.Y,
		W: w,
		H: h,
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

// CursorStyle represents how cursor is displayed
type CursorStyle uint32

const (
	// CursorStyleDefault is the default cursor style (dependend on terminal)
	CursorStyleDefault CursorStyle = 1 << iota
	// CursorStyleBlock is a block cursor
	CursorStyleBlock
	// CursorStyleUnderline is an underline cursor
	CursorStyleUnderline
	// CursorStyleBar is a bar cursor
	CursorStyleBar
	// CursorStyleHidden is a hidden cursor
	CursorStyleHidden
	CursorStyleBlinking
)

// IScreenOutput represents a terminal screen. It consists of a grid of cells.
// Each cell contains a rune and a set of attributes.
type IScreenOutput interface {

	// GetSize returns the size of the screen in characters.
	GetSize() (width int, height int)

	// SetSize changes the size of the screen in characters.
	// When resizing, content is preserved:
	// - horizontal expansion: new cells on the right are filled with spaces
	// - vertical expansion: new rows at the bottom are filled with spaces
	// - horizontal shrinking: leftmost columns are kept
	// - vertical shrinking: topmost rows are kept
	SetSize(width int, height int)

	// GetContent returns the whole content of the screen.
	// Returns width, height, and the internal buffer array.
	// The content is a single dimensional array where index = y*width + x.
	GetContent() (width int, height int, content []Cell)

	// PutText puts text at the specified position with the specified attributes.
	// The rect parameter specifies the position (X, Y) and optional clipping rectangle (W, H).
	// If W and H are 0, the text is clipped only to screen boundaries.
	// If W and H are non-zero, the text is clipped to both the rectangle and screen boundaries.
	// Text is always rendered on a single line (Y coordinate from rect).
	PutText(rect TRect, text string, attrs CellAttributes)

	// MoveCursor moves the cursor to the specified position.
	MoveCursor(x int, y int)

	// SetCursorStyle sets the cursor style.
	SetCursorStyle(style CursorStyle)
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

// InputEvent represents an input event from the terminal.
type InputEvent struct {
	// Type is a general type of the input event.
	Type InputEventType

	// Key is a key code for keyboard events.
	// For letter keys, Key is a Unicode code point of the letter (uppercase if shift is pressed plus shift modifier set)
	// For function keys F1-F12, Key is a letter from their CSI codes (P-S for F1-F4, T-Z and [ for F5-F12) and ModFn modifier set
	// For arrow keys and other special navigation keys, Key is a letter and ModFn modifier set
	Key rune

	// Content is a content of the copy/paste event
	// For other keys content is nil
	Content string

	// Modifiers is a bitfield of modifiers
	Modifiers EventModifiers

	// X and Y are coordinates of the mouse event or width and height for resize event
	X, Y uint16
}

func (e *InputEvent) String() string {
	// Handle non-keyboard events
	switch e.Type {
	case InputEventMouse:
		return "Mouse"
	case InputEventResize:
		return "Resize"
	case InputEventCopy:
		return "Copy"
	case InputEventPaste:
		return "Paste"
	case InputEventFocus:
		return "Focus"
	case InputEventBlur:
		return "Blur"
	}

	// Handle keyboard events
	var result string

	// Add modifiers prefix
	if e.Modifiers&ModCtrl != 0 {
		result += "Ctrl-"
	}
	if e.Modifiers&ModAlt != 0 {
		result += "Alt-"
	}
	if e.Modifiers&ModMeta != 0 {
		result += "Meta-"
	}

	// Determine the key name
	var keyName string

	// Function keys F1-F12 (using letter codes from CSI sequences)
	if e.Modifiers&ModFn != 0 {
		if e.Modifiers&ModShift != 0 {
			result += "Shift-"
		}
		// Map letter codes to function key numbers
		var fnNum int
		switch e.Key {
		case 'P':
			fnNum = 1
		case 'Q':
			fnNum = 2
		case 'R':
			fnNum = 3
		case 'S':
			fnNum = 4
		case 'T':
			fnNum = 5
		case 'U':
			fnNum = 6
		case 'V':
			fnNum = 7
		case 'W':
			fnNum = 8
		case 'X':
			fnNum = 9
		case 'Y':
			fnNum = 10
		case 'Z':
			fnNum = 11
		case '[':
			fnNum = 12
		}
		if fnNum > 0 {
			keyName = "F" + string(rune('0'+fnNum/10)) + string(rune('0'+fnNum%10))
			if fnNum < 10 {
				keyName = "F" + string(rune('0'+fnNum))
			}
			return result + keyName
		}

		// Navigation keys (arrow keys, Home, End, etc.)
		switch e.Key {
		case 'A':
			return result + "Up"
		case 'B':
			return result + "Down"
		case 'C':
			return result + "Right"
		case 'D':
			return result + "Left"
		case 'H':
			return result + "Home"
		case 'F':
			return result + "End"
		case 'I':
			return result + "Insert"
		case 'G':
			return result + "PageUp"
		case 'N':
			return result + "PageDown"
		default:
			return result + string(e.Key)
		}
	}

	// Regular letter keys with Shift
	if e.Modifiers&ModShift != 0 && e.Key >= 'A' && e.Key <= 'Z' {
		return result + string(e.Key)
	}

	// Regular keys
	if e.Key >= 32 && e.Key <= 126 {
		return result + string(e.Key)
	}

	// Special control characters
	switch e.Key {
	case '\t':
		return result + "Tab"
	case '\r':
		return result + "Enter"
	case '\n':
		return result + "Enter"
	case 0x1B:
		return result + "Esc"
	case 0x7F:
		return result + "Backspace"
	default:
		// For other control characters, show as Ctrl-letter
		if e.Key >= 1 && e.Key <= 26 {
			return result + string(rune('a'+e.Key-1))
		}
		return result + string(e.Key)
	}
}
