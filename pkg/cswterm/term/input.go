package term

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"github.com/codesnort/codesnort-swe/pkg/cswterm"
)

// InputEventReader reads input events from terminal and converts them to InputEvent objects.
// It supports basic keys, terminal resize events, and mouse events.
type InputEventReader struct {
	reader  io.Reader
	writer  io.Writer
	handler cswterm.InputEventHandler
	stopCh  chan struct{}
	doneCh  chan struct{}
	mu      sync.Mutex
	running bool
}

// NewInputEventReader creates a new InputEventReader instance.
// The reader is used to read input from terminal.
// The writer is optional and used for terminal control sequences (e.g., querying terminal size).
// The handler is called for each input event.
func NewInputEventReader(reader io.Reader, writer io.Writer, handler cswterm.InputEventHandler) *InputEventReader {
	return &InputEventReader{
		reader:  reader,
		writer:  writer,
		handler: handler,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// Start starts reading input events from the terminal.
// It sends an initial resize event with the current terminal size.
// This method runs in a separate goroutine and returns immediately.
func (r *InputEventReader) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("InputEventReader.Start(): already running")
	}

	r.running = true

	// Send initial resize event
	width, height, err := r.getTerminalSize()
	if err != nil {
		// If we can't get terminal size, use default
		width, height = 80, 24
	}

	r.handler.Notify(cswterm.InputEvent{
		Type: cswterm.InputEventResize,
		X:    uint16(width),
		Y:    uint16(height),
	})

	// Start reading input in a goroutine
	go r.readLoop()

	return nil
}

// Stop stops reading input events.
func (r *InputEventReader) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	close(r.stopCh)
	<-r.doneCh
	r.running = false
}

// NotifyResize sends a resize event to the handler with the given dimensions.
// This method should be called when the terminal is resized (e.g., from a SIGWINCH handler).
func (r *InputEventReader) NotifyResize(width, height int) {
	r.handler.Notify(cswterm.InputEvent{
		Type: cswterm.InputEventResize,
		X:    uint16(width),
		Y:    uint16(height),
	})
}

// getTerminalSize returns the current terminal size in characters.
func (r *InputEventReader) getTerminalSize() (width, height int, err error) {
	// Try to get terminal size from file descriptor
	if f, ok := r.reader.(*os.File); ok {
		fd := f.Fd()
		var ws struct {
			Row    uint16
			Col    uint16
			Xpixel uint16
			Ypixel uint16
		}
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
			uintptr(fd),
			uintptr(syscall.TIOCGWINSZ),
			uintptr(unsafe.Pointer(&ws)))

		if errno != 0 {
			return 0, 0, fmt.Errorf("InputEventReader.getTerminalSize(): ioctl failed: %v", errno)
		}

		return int(ws.Col), int(ws.Row), nil
	}

	return 0, 0, fmt.Errorf("InputEventReader.getTerminalSize(): reader is not a file")
}

// readLoop reads input events from the terminal in a loop.
func (r *InputEventReader) readLoop() {
	defer close(r.doneCh)

	buf := make([]byte, 1024)
	for {
		select {
		case <-r.stopCh:
			return
		default:
		}

		n, err := r.reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				// Notify about error? For now, just stop
			}
			return
		}

		if n > 0 {
			r.parseInput(buf[:n])
		}
	}
}

// parseInput parses input data and generates input events.
func (r *InputEventReader) parseInput(data []byte) {
	i := 0
	for i < len(data) {
		// Check for escape sequence
		if data[i] == 0x1B { // ESC
			if i+1 < len(data) {
				// CSI sequence: ESC [
				if data[i+1] == '[' {
					consumed := r.parseCSI(data[i:])
					if consumed > 0 {
						i += consumed
						continue
					}
				}
				// ESC O sequence (function keys F1-F4, arrow keys in some modes)
				if data[i+1] == 'O' {
					consumed := r.parseEscO(data[i:])
					if consumed > 0 {
						i += consumed
						continue
					}
				}
				// Other escape sequences can be added here
			}

			// Single ESC key
			r.handler.Notify(cswterm.InputEvent{
				Type: cswterm.InputEventKey,
				Key:  0x1B,
			})
			i++
			continue
		}

		// Regular key
		r.parseRegularKey(data[i])
		i++
	}
}

// parseEscO parses an ESC O sequence (used for F1-F4 and some other keys).
// Returns the number of bytes consumed, or 0 if not a valid ESC O sequence.
func (r *InputEventReader) parseEscO(data []byte) int {
	if len(data) < 3 || data[0] != 0x1B || data[1] != 'O' {
		return 0
	}

	// ESC O sequences are followed by a single character
	ch := data[2]

	switch ch {
	case 'P': // F1
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'P',
			Modifiers: cswterm.ModFn,
		})
		return 3
	case 'Q': // F2
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'Q',
			Modifiers: cswterm.ModFn,
		})
		return 3
	case 'R': // F3
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'R',
			Modifiers: cswterm.ModFn,
		})
		return 3
	case 'S': // F4
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'S',
			Modifiers: cswterm.ModFn,
		})
		return 3
	case 'A': // Up arrow (alternate mode)
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'A',
			Modifiers: cswterm.ModFn,
		})
		return 3
	case 'B': // Down arrow (alternate mode)
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'B',
			Modifiers: cswterm.ModFn,
		})
		return 3
	case 'C': // Right arrow (alternate mode)
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'C',
			Modifiers: cswterm.ModFn,
		})
		return 3
	case 'D': // Left arrow (alternate mode)
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'D',
			Modifiers: cswterm.ModFn,
		})
		return 3
	case 'H': // Home (alternate mode)
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'H',
			Modifiers: cswterm.ModFn,
		})
		return 3
	case 'F': // End (alternate mode)
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'F',
			Modifiers: cswterm.ModFn,
		})
		return 3
	default:
		// Unknown ESC O sequence, don't consume it
		return 0
	}
}

// parseCSI parses a CSI (Control Sequence Introducer) sequence.
// Returns the number of bytes consumed, or 0 if not a valid CSI sequence.
func (r *InputEventReader) parseCSI(data []byte) int {
	if len(data) < 3 || data[0] != 0x1B || data[1] != '[' {
		return 0
	}

	// Check for SGR mouse mode: ESC[<btn;x;y;M or ESC[<btn;x;y;m
	if data[2] == '<' {
		// Find the end of the SGR mouse sequence
		end := 3
		for end < len(data) {
			ch := data[end]
			if ch == 'M' || ch == 'm' {
				end++
				break
			}
			end++
		}

		if end > len(data) || (end == len(data) && data[end-1] != 'M' && data[end-1] != 'm') {
			// Incomplete sequence
			return 0
		}

		// Extract parameters from the sequence (everything between '<' and final 'M' or 'm')
		paramSeq := data[3 : end-1]
		params := r.parseCSIParams(paramSeq)

		// Handle SGR mouse event
		r.handleSGRMouse(params)
		return end
	}

	// Find the end of CSI sequence (a letter or specific character)
	end := 2
	for end < len(data) {
		ch := data[end]
		// CSI sequences end with a letter (A-Z, a-z) or specific characters
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '~' || ch == 'R' || ch == 'M' {
			end++
			break
		}
		end++
	}

	if end >= len(data) {
		// Incomplete sequence, wait for more data
		// For now, we'll process what we have
		end = len(data)
	}

	seq := data[2:end]
	if len(seq) == 0 {
		return 0
	}

	lastChar := data[end-1]

	// Parse CSI parameters
	params := r.parseCSIParams(seq[:len(seq)-1])

	// Handle different CSI sequences
	// According to doc: "For arrow keys and other special navigation keys, Key is a letter and ModFn modifier set"
	switch lastChar {
	case 'A': // Up arrow
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'A',
			Modifiers: r.getModifiers(params) | cswterm.ModFn,
		})
		return end

	case 'B': // Down arrow
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'B',
			Modifiers: r.getModifiers(params) | cswterm.ModFn,
		})
		return end

	case 'C': // Right arrow
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'C',
			Modifiers: r.getModifiers(params) | cswterm.ModFn,
		})
		return end

	case 'D': // Left arrow
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'D',
			Modifiers: r.getModifiers(params) | cswterm.ModFn,
		})
		return end

	case 'H': // Home
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'H',
			Modifiers: r.getModifiers(params) | cswterm.ModFn,
		})
		return end

	case 'F': // End
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       'F',
			Modifiers: r.getModifiers(params) | cswterm.ModFn,
		})
		return end

	case 'P': // F1 with modifiers (CSI format: ESC[1;modP)
		// Check if this is a modified F1 key (has params like [1;2P] for Shift+F1)
		if len(params) >= 2 && params[0] == 1 {
			r.handler.Notify(cswterm.InputEvent{
				Type:      cswterm.InputEventKey,
				Key:       'P',
				Modifiers: r.getModifiers(params) | cswterm.ModFn,
			})
			return end
		}
		// Unknown sequence, ignore
		return end

	case 'Q': // F2 with modifiers (CSI format: ESC[1;modQ)
		if len(params) >= 2 && params[0] == 1 {
			r.handler.Notify(cswterm.InputEvent{
				Type:      cswterm.InputEventKey,
				Key:       'Q',
				Modifiers: r.getModifiers(params) | cswterm.ModFn,
			})
			return end
		}
		return end

	case 'R': // F3 with modifiers (CSI format: ESC[1;modR) or cursor position report
		if len(params) >= 2 && params[0] == 1 {
			// Modified F3 key
			r.handler.Notify(cswterm.InputEvent{
				Type:      cswterm.InputEventKey,
				Key:       'R',
				Modifiers: r.getModifiers(params) | cswterm.ModFn,
			})
			return end
		}
		// Cursor position report (response to CSI 6 n)
		// This is typically used for terminal size detection
		// We can ignore it for now or handle it specially
		return end

	case 'S': // F4 with modifiers (CSI format: ESC[1;modS)
		if len(params) >= 2 && params[0] == 1 {
			r.handler.Notify(cswterm.InputEvent{
				Type:      cswterm.InputEventKey,
				Key:       'S',
				Modifiers: r.getModifiers(params) | cswterm.ModFn,
			})
			return end
		}
		return end

	case '~': // Special keys (F1-F12, Insert, Delete, Page Up/Down, etc.)
		if len(params) > 0 {
			r.handleTildeKey(params[0], params)
		}
		return end

	case 'M': // X10 mouse event (not SGR mode, which is handled earlier)
		// X10 mouse events: ESC[Mbxy (where b, x, y are single bytes)
		if len(data) >= end+3 {
			r.handleMouseEvent(data[end:end+3], false)
			return end + 3
		}
		return end

	default:
		// Unknown CSI sequence, ignore
		return end
	}
}

// parseCSIParams parses CSI parameters (semicolon-separated numbers).
func (r *InputEventReader) parseCSIParams(data []byte) []int {
	var params []int
	parts := bytes.Split(data, []byte(";"))
	for _, part := range parts {
		if len(part) == 0 {
			params = append(params, 0)
			continue
		}
		num, err := strconv.Atoi(string(part))
		if err == nil {
			params = append(params, num)
		} else {
			params = append(params, 0)
		}
	}
	return params
}

// getModifiers extracts modifiers from CSI parameters.
// The modifier parameter in xterm CSI sequences is encoded as modParam = modifier + 1,
// where modifier is a bitmask: bit 0 = Shift, bit 1 = Alt, bit 2 = Ctrl, bit 3 = Meta.
// For example: \x1b[1;2A means Shift+Up (modParam=2, modifier=1=Shift).
func (r *InputEventReader) getModifiers(params []int) cswterm.EventModifiers {
	var mods cswterm.EventModifiers
	if len(params) >= 2 {
		// xterm modifier encoding: modParam = modifier + 1
		// So we need to subtract 1 to get the actual modifier bits.
		modParam := params[1] - 1
		if modParam&1 != 0 {
			mods |= cswterm.ModShift
		}
		if modParam&2 != 0 {
			mods |= cswterm.ModAlt
		}
		if modParam&4 != 0 {
			mods |= cswterm.ModCtrl
		}
		if modParam&8 != 0 {
			mods |= cswterm.ModMeta
		}
	}
	return mods
}

// handleTildeKey handles special keys that end with '~'.
func (r *InputEventReader) handleTildeKey(keyCode int, params []int) {
	var key rune
	var mods cswterm.EventModifiers

	switch keyCode {
	case 1, 7: // Home
		key = 'H'
		mods |= cswterm.ModFn
	case 2: // Insert
		key = 'I'
		mods |= cswterm.ModFn
	case 3: // Delete
		key = 'D'
		mods |= cswterm.ModFn
	case 4, 8: // End
		key = 'F'
		mods |= cswterm.ModFn
	case 5: // Page Up
		key = 'G'
		mods |= cswterm.ModFn
	case 6: // Page Down
		key = 'N'
		mods |= cswterm.ModFn
	case 11: // F1
		key = 'P'
		mods |= cswterm.ModFn
	case 12: // F2
		key = 'Q'
		mods |= cswterm.ModFn
	case 13: // F3
		key = 'R'
		mods |= cswterm.ModFn
	case 14: // F4
		key = 'S'
		mods |= cswterm.ModFn
	case 15: // F5
		key = 'T'
		mods |= cswterm.ModFn
	case 17: // F6
		key = 'U'
		mods |= cswterm.ModFn
	case 18: // F7
		key = 'V'
		mods |= cswterm.ModFn
	case 19: // F8
		key = 'W'
		mods |= cswterm.ModFn
	case 20: // F9
		key = 'X'
		mods |= cswterm.ModFn
	case 21: // F10
		key = 'Y'
		mods |= cswterm.ModFn
	case 23: // F11
		key = 'Z'
		mods |= cswterm.ModFn
	case 24: // F12
		key = '['
		mods |= cswterm.ModFn
	default:
		// Unknown key code, ignore
		return
	}

	mods |= r.getModifiers(params)

	r.handler.Notify(cswterm.InputEvent{
		Type:      cswterm.InputEventKey,
		Key:       key,
		Modifiers: mods,
	})
}

// handleMouseEvent handles standard mouse events (X10/normal mouse mode).
func (r *InputEventReader) handleMouseEvent(data []byte, sgr bool) {
	if len(data) < 3 {
		return
	}

	btn := int(data[0]) - 32
	x := int(data[1]) - 32 - 1 // Convert to 0-based
	y := int(data[2]) - 32 - 1 // Convert to 0-based

	var mods cswterm.EventModifiers

	// Parse button and modifiers
	button := btn & 3
	if btn&4 != 0 {
		mods |= cswterm.ModShift
	}
	if btn&8 != 0 {
		mods |= cswterm.ModAlt
	}
	if btn&16 != 0 {
		mods |= cswterm.ModCtrl
	}

	// Check for mouse movement/drag
	if btn&32 != 0 {
		mods |= cswterm.ModMove
	}

	// Check for scroll events
	if btn&64 != 0 {
		if button == 0 {
			mods |= cswterm.ModScrollUp
		} else if button == 1 {
			mods |= cswterm.ModScrollDown
		}
	}

	// Determine press/release
	if btn&3 == 3 {
		mods |= cswterm.ModRelease
	} else {
		mods |= cswterm.ModPress
	}

	r.handler.Notify(cswterm.InputEvent{
		Type:      cswterm.InputEventMouse,
		X:         uint16(x),
		Y:         uint16(y),
		Modifiers: mods,
	})
}

// handleSGRMouse handles SGR mouse events.
func (r *InputEventReader) handleSGRMouse(params []int) {
	if len(params) < 3 {
		return
	}

	btn := params[0]
	x := params[1] - 1 // Convert to 0-based
	y := params[2] - 1 // Convert to 0-based

	var mods cswterm.EventModifiers

	// Parse modifiers
	if btn&4 != 0 {
		mods |= cswterm.ModShift
	}
	if btn&8 != 0 {
		mods |= cswterm.ModAlt
	}
	if btn&16 != 0 {
		mods |= cswterm.ModCtrl
	}

	// Check for mouse movement/drag
	if btn&32 != 0 {
		mods |= cswterm.ModMove
	}

	// Check for scroll events
	if btn&64 != 0 {
		button := btn & 3
		if button == 0 {
			mods |= cswterm.ModScrollUp
		} else if button == 1 {
			mods |= cswterm.ModScrollDown
		}
	}

	r.handler.Notify(cswterm.InputEvent{
		Type:      cswterm.InputEventMouse,
		X:         uint16(x),
		Y:         uint16(y),
		Modifiers: mods,
	})
}

// parseRegularKey parses a regular key press.
func (r *InputEventReader) parseRegularKey(b byte) {
	var mods cswterm.EventModifiers

	// Handle Ctrl combinations (0x00-0x1F except special cases)
	if b < 0x20 {
		// Special control characters that should not be converted
		switch b {
		case 0x09: // Tab
			mods |= cswterm.ModCtrl
			r.handler.Notify(cswterm.InputEvent{
				Type:      cswterm.InputEventKey,
				Key:       rune(b),
				Modifiers: mods,
			})
			return
		case 0x0A: // LF (newline)
			mods |= cswterm.ModCtrl
			r.handler.Notify(cswterm.InputEvent{
				Type:      cswterm.InputEventKey,
				Key:       rune(b),
				Modifiers: mods,
			})
			return
		case 0x0D: // CR (carriage return)
			mods |= cswterm.ModCtrl
			r.handler.Notify(cswterm.InputEvent{
				Type:      cswterm.InputEventKey,
				Key:       rune(b),
				Modifiers: mods,
			})
			return
		}

		// Convert Ctrl+letter to the corresponding letter
		if b >= 1 && b <= 26 {
			// Ctrl+A = 1, Ctrl+B = 2, etc.
			mods |= cswterm.ModCtrl
			r.handler.Notify(cswterm.InputEvent{
				Type:      cswterm.InputEventKey,
				Key:       rune('a' + b - 1),
				Modifiers: mods,
			})
			return
		}

		// Other control characters
		mods |= cswterm.ModCtrl
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       rune(b),
			Modifiers: mods,
		})
		return
	}

	// Handle uppercase letters (A-Z) - these indicate Shift was pressed
	// According to doc: "For letter keys, Key is a Unicode code point of the letter (uppercase if shift is pressed plus shift modifier set)"
	if b >= 'A' && b <= 'Z' {
		r.handler.Notify(cswterm.InputEvent{
			Type:      cswterm.InputEventKey,
			Key:       rune(b), // Keep uppercase
			Modifiers: cswterm.ModShift,
		})
		return
	}

	// Regular printable character or DEL
	r.handler.Notify(cswterm.InputEvent{
		Type: cswterm.InputEventKey,
		Key:  rune(b),
	})
}
