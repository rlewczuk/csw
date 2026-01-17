package cswterm

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

// InputEventReader reads input events from terminal and converts them to InputEvent objects.
// It supports basic keys, terminal resize events, and mouse events.
type InputEventReader struct {
	reader  io.Reader
	writer  io.Writer
	handler InputEventHandler
	stopCh  chan struct{}
	doneCh  chan struct{}
	mu      sync.Mutex
	running bool
}

// NewInputEventReader creates a new InputEventReader instance.
// The reader is used to read input from terminal.
// The writer is optional and used for terminal control sequences (e.g., querying terminal size).
// The handler is called for each input event.
func NewInputEventReader(reader io.Reader, writer io.Writer, handler InputEventHandler) *InputEventReader {
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

	r.handler.Notify(InputEvent{
		Type: InputEventResize,
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
	r.handler.Notify(InputEvent{
		Type: InputEventResize,
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
				// Other escape sequences can be added here
			}

			// Single ESC key
			r.handler.Notify(InputEvent{
				Type: InputEventKey,
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

// parseCSI parses a CSI (Control Sequence Introducer) sequence.
// Returns the number of bytes consumed, or 0 if not a valid CSI sequence.
func (r *InputEventReader) parseCSI(data []byte) int {
	if len(data) < 3 || data[0] != 0x1B || data[1] != '[' {
		return 0
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
	switch lastChar {
	case 'A': // Up arrow
		r.handler.Notify(InputEvent{
			Type:      InputEventKey,
			Key:       'A',
			Modifiers: r.getModifiers(params),
		})
		return end

	case 'B': // Down arrow
		r.handler.Notify(InputEvent{
			Type:      InputEventKey,
			Key:       'B',
			Modifiers: r.getModifiers(params),
		})
		return end

	case 'C': // Right arrow
		r.handler.Notify(InputEvent{
			Type:      InputEventKey,
			Key:       'C',
			Modifiers: r.getModifiers(params),
		})
		return end

	case 'D': // Left arrow
		r.handler.Notify(InputEvent{
			Type:      InputEventKey,
			Key:       'D',
			Modifiers: r.getModifiers(params),
		})
		return end

	case 'H': // Home
		r.handler.Notify(InputEvent{
			Type:      InputEventKey,
			Key:       'H',
			Modifiers: r.getModifiers(params),
		})
		return end

	case 'F': // End
		r.handler.Notify(InputEvent{
			Type:      InputEventKey,
			Key:       'F',
			Modifiers: r.getModifiers(params),
		})
		return end

	case '~': // Special keys (F1-F12, Insert, Delete, Page Up/Down, etc.)
		if len(params) > 0 {
			r.handleTildeKey(params[0], params)
		}
		return end

	case 'M': // Mouse event
		if len(data) >= end+3 {
			r.handleMouseEvent(data[end:end+3], false)
			return end + 3
		}
		return end

	case 'm': // Mouse event (SGR mode)
		r.handleSGRMouse(params)
		return end

	case 'R': // Cursor position report (response to CSI 6 n)
		// This is typically used for terminal size detection
		// We can ignore it for now or handle it specially
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
func (r *InputEventReader) getModifiers(params []int) EventModifiers {
	var mods EventModifiers
	if len(params) >= 2 {
		// xterm modifier encoding: modParam = modifier + 1
		// So we need to subtract 1 to get the actual modifier bits.
		modParam := params[1] - 1
		if modParam&1 != 0 {
			mods |= ModShift
		}
		if modParam&2 != 0 {
			mods |= ModAlt
		}
		if modParam&4 != 0 {
			mods |= ModCtrl
		}
		if modParam&8 != 0 {
			mods |= ModMeta
		}
	}
	return mods
}

// handleTildeKey handles special keys that end with '~'.
func (r *InputEventReader) handleTildeKey(keyCode int, params []int) {
	var key rune
	var mods EventModifiers

	switch keyCode {
	case 1, 7: // Home
		key = 'H'
	case 2: // Insert
		key = 'I'
	case 3: // Delete
		key = 'D'
	case 4, 8: // End
		key = 'F'
	case 5: // Page Up
		key = 'P'
	case 6: // Page Down
		key = 'N'
	case 11: // F1
		key = 'F'
		mods |= ModFn
	case 12: // F2
		key = 'G'
		mods |= ModFn
	case 13: // F3
		key = 'J'
		mods |= ModFn
	case 14: // F4
		key = 'K'
		mods |= ModFn
	case 15: // F5
		key = 'L'
		mods |= ModFn
	case 17: // F6
		key = 'M'
		mods |= ModFn
	case 18: // F7
		key = 'O'
		mods |= ModFn
	case 19: // F8
		key = 'Q'
		mods |= ModFn
	case 20: // F9
		key = 'R'
		mods |= ModFn
	case 21: // F10
		key = 'S'
		mods |= ModFn
	case 23: // F11
		key = 'T'
		mods |= ModFn
	case 24: // F12
		key = 'U'
		mods |= ModFn
	default:
		// Unknown key code, ignore
		return
	}

	mods |= r.getModifiers(params)

	r.handler.Notify(InputEvent{
		Type:      InputEventKey,
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

	var mods EventModifiers

	// Parse button and modifiers
	button := btn & 3
	if btn&4 != 0 {
		mods |= ModShift
	}
	if btn&8 != 0 {
		mods |= ModAlt
	}
	if btn&16 != 0 {
		mods |= ModCtrl
	}

	// Check for mouse movement/drag
	if btn&32 != 0 {
		mods |= ModMove
	}

	// Check for scroll events
	if btn&64 != 0 {
		if button == 0 {
			mods |= ModScrollUp
		} else if button == 1 {
			mods |= ModScrollDown
		}
	}

	// Determine press/release
	if btn&3 == 3 {
		mods |= ModRelease
	} else {
		mods |= ModPress
	}

	r.handler.Notify(InputEvent{
		Type:      InputEventMouse,
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

	var mods EventModifiers

	// Parse modifiers
	if btn&4 != 0 {
		mods |= ModShift
	}
	if btn&8 != 0 {
		mods |= ModAlt
	}
	if btn&16 != 0 {
		mods |= ModCtrl
	}

	// Check for mouse movement/drag
	if btn&32 != 0 {
		mods |= ModMove
	}

	// Check for scroll events
	if btn&64 != 0 {
		button := btn & 3
		if button == 0 {
			mods |= ModScrollUp
		} else if button == 1 {
			mods |= ModScrollDown
		}
	}

	r.handler.Notify(InputEvent{
		Type:      InputEventMouse,
		X:         uint16(x),
		Y:         uint16(y),
		Modifiers: mods,
	})
}

// parseRegularKey parses a regular key press.
func (r *InputEventReader) parseRegularKey(b byte) {
	var mods EventModifiers

	// Handle Ctrl combinations (0x00-0x1F except special cases)
	if b < 0x20 {
		// Special control characters that should not be converted
		switch b {
		case 0x09: // Tab
			mods |= ModCtrl
			r.handler.Notify(InputEvent{
				Type:      InputEventKey,
				Key:       rune(b),
				Modifiers: mods,
			})
			return
		case 0x0A: // LF (newline)
			mods |= ModCtrl
			r.handler.Notify(InputEvent{
				Type:      InputEventKey,
				Key:       rune(b),
				Modifiers: mods,
			})
			return
		case 0x0D: // CR (carriage return)
			mods |= ModCtrl
			r.handler.Notify(InputEvent{
				Type:      InputEventKey,
				Key:       rune(b),
				Modifiers: mods,
			})
			return
		}

		// Convert Ctrl+letter to the corresponding letter
		if b >= 1 && b <= 26 {
			// Ctrl+A = 1, Ctrl+B = 2, etc.
			mods |= ModCtrl
			r.handler.Notify(InputEvent{
				Type:      InputEventKey,
				Key:       rune('a' + b - 1),
				Modifiers: mods,
			})
			return
		}

		// Other control characters
		mods |= ModCtrl
		r.handler.Notify(InputEvent{
			Type:      InputEventKey,
			Key:       rune(b),
			Modifiers: mods,
		})
		return
	}

	// Handle uppercase letters (A-Z) - these indicate Shift was pressed
	if b >= 'A' && b <= 'Z' {
		r.handler.Notify(InputEvent{
			Type:      InputEventKey,
			Key:       rune(b - 'A' + 'a'), // Convert to lowercase
			Modifiers: ModShift,
		})
		return
	}

	// Regular printable character or DEL
	r.handler.Notify(InputEvent{
		Type: InputEventKey,
		Key:  rune(b),
	})
}
