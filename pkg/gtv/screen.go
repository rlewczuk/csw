package gtv

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Option is a function that configures an object.
// It is used mostlyu for widget construction but can be used for other objects as well.
type Option func(any)

// TextAttributes represents additional text modifiers, eg. underline, bold, etc.
// This complements TextColor and BackColor which are separate.
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

// TextColor represents a 24-bit color in RGB format.
type TextColor uint32

// CellAttributes represents attributes of a single character cell.
type CellAttributes struct {
	// Attrs is a bitfield of attributes. It is a combination of the following:
	Attributes TextAttributes `json:"attributes,omitempty"`
	// Foreground sets color of the text. It is 24-bit color in RGB format.
	TextColor uint32 `json:"text-color,omitempty"`
	// Background sets color of the background. It is 24-bit color in RGB format.
	BackColor uint32 `json:"back-color,omitempty"`
	// StrikeColor sets color of the strike through line. It is 24-bit color in RGB format.
	StrikeColor uint32 `json:"strike-color,omitempty"`
	// ThemeTag is a tag that can be used to apply theme to the text.
	// If non-zero, it will automatically fill color fields if they are not explicitly set (i.e. zero)
	ThemeTag uint32 `json:"theme-tag,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for CellAttributes.
// It supports both decimal numbers and hex strings for color fields.
// Hex strings can be in the format "#RRGGBB" or "0xRRGGBB".
func (c *CellAttributes) UnmarshalJSON(data []byte) error {
	// Use an auxiliary type to avoid infinite recursion
	type Alias CellAttributes

	// First, try to unmarshal as a map to handle mixed types
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("CellAttributes.UnmarshalJSON(): %w", err)
	}

	// Parse each field
	for key, value := range raw {
		switch key {
		case "attributes":
			if err := json.Unmarshal(value, &c.Attributes); err != nil {
				return fmt.Errorf("CellAttributes.UnmarshalJSON(): invalid attributes: %w", err)
			}
		case "text-color":
			color, err := parseColorValue(value)
			if err != nil {
				return fmt.Errorf("CellAttributes.UnmarshalJSON(): invalid text-color: %w", err)
			}
			c.TextColor = color
		case "back-color":
			color, err := parseColorValue(value)
			if err != nil {
				return fmt.Errorf("CellAttributes.UnmarshalJSON(): invalid back-color: %w", err)
			}
			c.BackColor = color
		case "strike-color":
			color, err := parseColorValue(value)
			if err != nil {
				return fmt.Errorf("CellAttributes.UnmarshalJSON(): invalid strike-color: %w", err)
			}
			c.StrikeColor = color
		case "theme-tag":
			if err := json.Unmarshal(value, &c.ThemeTag); err != nil {
				return fmt.Errorf("CellAttributes.UnmarshalJSON(): invalid theme-tag: %w", err)
			}
		default:
			// Ignore unknown fields
		}
	}

	return nil
}

// parseColorValue parses a color value from JSON, supporting both numbers and hex strings.
func parseColorValue(data []byte) (uint32, error) {
	// Try to parse as a number first
	var num uint32
	if err := json.Unmarshal(data, &num); err == nil {
		return num, nil
	}

	// Try to parse as a string (hex color)
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return 0, fmt.Errorf("parseColorValue(): expected number or hex string, got %s", string(data))
	}

	// Parse hex string
	return parseHexColor(str)
}

// parseHexColor parses a hex color string in the format "#RRGGBB" or "0xRRGGBB".
func parseHexColor(s string) (uint32, error) {
	// Remove leading # or 0x prefix
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") {
		s = s[1:]
	} else if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
	}

	// Parse the hex string
	val, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("parseHexColor(): invalid hex color %q: %w", s, err)
	}

	return uint32(val), nil
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

	// PutContent puts raw cell content at the specified position.
	// The rect parameter specifies the position (X, Y) and optional clipping rectangle (W, H).
	// If W and H are 0, the content is clipped only to screen boundaries.
	// If W and H are non-zero, the content is clipped to both the rectangle and screen boundaries.
	// Content is always rendered on a single line (Y coordinate from rect).
	PutContent(rect TRect, content []Cell)

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

// ParseKey parses a key name and returns an InputEvent.
// Supported formats:
// - Single character: "a", "A", "1", "!", etc.
// - Special keys: "Enter", "Tab", "Esc", "Backspace", "Delete", "Insert"
// - Navigation keys: "Up", "Down", "Left", "Right", "Home", "End", "PageUp", "PageDown"
// - Function keys: "F1", "F2", ..., "F12"
// - Modified keys: "Ctrl+C", "Alt+Enter", "Shift+F1", "Ctrl+Alt+Delete"
func ParseKey(name string) (InputEvent, error) {
	if name == "" {
		return InputEvent{}, fmt.Errorf("ParseKey(): empty key name")
	}

	var mods EventModifiers
	var key rune
	var eventType = InputEventKey

	// Parse modifiers (Ctrl+, Alt+, Meta+, Shift+)
	parts := strings.Split(name, "+")
	if len(parts) > 1 {
		// Last part is the actual key, everything before is modifiers
		for i := 0; i < len(parts)-1; i++ {
			mod := strings.ToLower(strings.TrimSpace(parts[i]))
			switch mod {
			case "ctrl", "control":
				mods |= ModCtrl
			case "alt":
				mods |= ModAlt
			case "meta":
				mods |= ModMeta
			case "shift":
				mods |= ModShift
			default:
				return InputEvent{}, fmt.Errorf("ParseKey(): unknown modifier: %s", parts[i])
			}
		}
		name = strings.TrimSpace(parts[len(parts)-1])
	}

	// Parse the key name (case-insensitive for special keys)
	lowerName := strings.ToLower(name)

	// Check for special keys
	switch lowerName {
	case "enter", "return":
		key = '\r'
	case "tab":
		key = '\t'
	case "esc", "escape":
		key = 0x1B
	case "backspace":
		key = 0x7F
	case "space":
		key = ' '
	case "delete":
		key = 'D'
		mods |= ModFn
	case "insert":
		key = 'I'
		mods |= ModFn
	case "up":
		key = 'A'
		mods |= ModFn
	case "down":
		key = 'B'
		mods |= ModFn
	case "right":
		key = 'C'
		mods |= ModFn
	case "left":
		key = 'D'
		mods |= ModFn
	case "home":
		key = 'H'
		mods |= ModFn
	case "end":
		key = 'F'
		mods |= ModFn
	case "pageup":
		key = 'G'
		mods |= ModFn
	case "pagedown":
		key = 'N'
		mods |= ModFn
	default:
		// Check for function keys (F1-F12)
		if strings.HasPrefix(lowerName, "f") && len(lowerName) >= 2 {
			fnNumStr := lowerName[1:]
			fnNum, err := strconv.Atoi(fnNumStr)
			if err == nil && fnNum >= 1 && fnNum <= 12 {
				// Map function key number to letter code
				fnKeys := []rune{'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', '['}
				key = fnKeys[fnNum-1]
				mods |= ModFn
			} else {
				return InputEvent{}, fmt.Errorf("ParseKey(): invalid function key: %s", name)
			}
		} else if len(name) == 1 {
			// Single character key
			key = rune(name[0])
			// If it's an uppercase letter and Ctrl is present, normalize to lowercase
			// Ctrl+letter combinations should always use lowercase keys
			if mods&ModCtrl != 0 && key >= 'A' && key <= 'Z' {
				key = key + ('a' - 'A') // Convert to lowercase
			} else if key >= 'A' && key <= 'Z' && mods&ModShift == 0 {
				// If it's an uppercase letter and Shift wasn't explicitly specified
				mods |= ModShift
			}
		} else {
			return InputEvent{}, fmt.Errorf("ParseKey(): unknown key name: %s", name)
		}
	}

	// Handle Ctrl combinations for letter keys
	if mods&ModCtrl != 0 && key >= 'a' && key <= 'z' {
		// For Ctrl+letter, the key should remain lowercase
		// The ModCtrl flag indicates it's a Ctrl combination
	}

	return InputEvent{
		Type:      eventType,
		Key:       key,
		Modifiers: mods,
	}, nil
}
