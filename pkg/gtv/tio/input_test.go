package tio

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
)

// TestInputEventReader_RegularKeys tests parsing of regular key presses.
func TestInputEventReader_RegularKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []gtv.InputEvent
	}{
		{
			name:  "single letter",
			input: []byte("a"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventKey, Key: 'a'},
			},
		},
		{
			name:  "multiple letters",
			input: []byte("abc"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventKey, Key: 'a'},
				{Type: gtv.InputEventKey, Key: 'b'},
				{Type: gtv.InputEventKey, Key: 'c'},
			},
		},
		{
			name:  "numbers",
			input: []byte("123"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventKey, Key: '1'},
				{Type: gtv.InputEventKey, Key: '2'},
				{Type: gtv.InputEventKey, Key: '3'},
			},
		},
		{
			name:  "special characters",
			input: []byte("!@#"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventKey, Key: '!'},
				{Type: gtv.InputEventKey, Key: '@'},
				{Type: gtv.InputEventKey, Key: '#'},
			},
		},
		{
			name:  "enter key",
			input: []byte{'\r'},
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventKey, Key: '\r', Modifiers: gtv.ModCtrl},
			},
		},
		{
			name:  "tab key",
			input: []byte{'\t'},
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventKey, Key: '\t', Modifiers: gtv.ModCtrl},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			// Parse the input directly (without starting the reader)
			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			assert.Equal(t, tt.expected, events)
		})
	}
}

// TestInputEventReader_CtrlKeys tests parsing of Ctrl key combinations.
func TestInputEventReader_CtrlKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []gtv.InputEvent
	}{
		{
			name:  "Ctrl+A",
			input: []byte{0x01},
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventKey, Key: 'a', Modifiers: gtv.ModCtrl},
			},
		},
		{
			name:  "Ctrl+C",
			input: []byte{0x03},
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventKey, Key: 'c', Modifiers: gtv.ModCtrl},
			},
		},
		{
			name:  "Ctrl+Z",
			input: []byte{0x1A},
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventKey, Key: 'z', Modifiers: gtv.ModCtrl},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			assert.Equal(t, tt.expected, events)
		})
	}
}

// TestInputEventReader_ArrowKeys tests parsing of arrow keys.
func TestInputEventReader_ArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		{
			name:     "Up arrow",
			input:    []byte("\x1b[A"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModFn},
		},
		{
			name:     "Down arrow",
			input:    []byte("\x1b[B"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'B', Modifiers: gtv.ModFn},
		},
		{
			name:     "Right arrow",
			input:    []byte("\x1b[C"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'C', Modifiers: gtv.ModFn},
		},
		{
			name:     "Left arrow",
			input:    []byte("\x1b[D"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'D', Modifiers: gtv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1)
			assert.Equal(t, tt.expected, events[0])
		})
	}
}

// TestInputEventReader_SpecialKeys tests parsing of special keys (Home, End, etc.).
func TestInputEventReader_SpecialKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		{
			name:     "Home key (CSI H)",
			input:    []byte("\x1b[H"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'H', Modifiers: gtv.ModFn},
		},
		{
			name:     "End key (CSI F)",
			input:    []byte("\x1b[F"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'F', Modifiers: gtv.ModFn},
		},
		{
			name:     "Home key (tilde)",
			input:    []byte("\x1b[1~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'H', Modifiers: gtv.ModFn},
		},
		{
			name:     "Insert key",
			input:    []byte("\x1b[2~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'I', Modifiers: gtv.ModFn},
		},
		{
			name:     "Delete key",
			input:    []byte("\x1b[3~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'D', Modifiers: gtv.ModFn},
		},
		{
			name:     "Page Up",
			input:    []byte("\x1b[5~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'G', Modifiers: gtv.ModFn},
		},
		{
			name:     "Page Down",
			input:    []byte("\x1b[6~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'N', Modifiers: gtv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1)
			assert.Equal(t, tt.expected, events[0])
		})
	}
}

// TestInputEventReader_FunctionKeys tests parsing of function keys.
func TestInputEventReader_FunctionKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		// F1-F4 use ESC O sequences (vt100/xterm)
		{
			name:     "F1 (ESC O P)",
			input:    []byte("\x1bOP"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn},
		},
		{
			name:     "F2 (ESC O Q)",
			input:    []byte("\x1bOQ"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Q', Modifiers: gtv.ModFn},
		},
		{
			name:     "F3 (ESC O R)",
			input:    []byte("\x1bOR"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'R', Modifiers: gtv.ModFn},
		},
		{
			name:     "F4 (ESC O S)",
			input:    []byte("\x1bOS"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'S', Modifiers: gtv.ModFn},
		},
		// F1-F4 also have CSI tilde format (urxvt)
		{
			name:     "F1 (CSI 11~)",
			input:    []byte("\x1b[11~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn},
		},
		{
			name:     "F2 (CSI 12~)",
			input:    []byte("\x1b[12~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Q', Modifiers: gtv.ModFn},
		},
		{
			name:     "F3 (CSI 13~)",
			input:    []byte("\x1b[13~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'R', Modifiers: gtv.ModFn},
		},
		{
			name:     "F4 (CSI 14~)",
			input:    []byte("\x1b[14~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'S', Modifiers: gtv.ModFn},
		},
		// F5-F12 use CSI tilde format
		{
			name:     "F5",
			input:    []byte("\x1b[15~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'T', Modifiers: gtv.ModFn},
		},
		{
			name:     "F6",
			input:    []byte("\x1b[17~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'U', Modifiers: gtv.ModFn},
		},
		{
			name:     "F7",
			input:    []byte("\x1b[18~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'V', Modifiers: gtv.ModFn},
		},
		{
			name:     "F8",
			input:    []byte("\x1b[19~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'W', Modifiers: gtv.ModFn},
		},
		{
			name:     "F9",
			input:    []byte("\x1b[20~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'X', Modifiers: gtv.ModFn},
		},
		{
			name:     "F10",
			input:    []byte("\x1b[21~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Y', Modifiers: gtv.ModFn},
		},
		{
			name:     "F11",
			input:    []byte("\x1b[23~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Z', Modifiers: gtv.ModFn},
		},
		{
			name:     "F12",
			input:    []byte("\x1b[24~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: '[', Modifiers: gtv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1, "Expected exactly one event for %s", tt.name)
			assert.Equal(t, tt.expected, events[0], "Event mismatch for %s", tt.name)
		})
	}
}

// TestInputEventReader_F1ToF4_NotParsedAsThreeEvents tests that F1-F4 keys
// are parsed as single events, not as three separate events (Esc, O, P).
// This is a regression test for the bug where ESC O sequences were not recognized.
func TestInputEventReader_F1ToF4_NotParsedAsThreeEvents(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		{
			name:     "F1 should be one event, not Esc+O+P",
			input:    []byte("\x1bOP"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn},
		},
		{
			name:     "F2 should be one event, not Esc+O+Q",
			input:    []byte("\x1bOQ"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Q', Modifiers: gtv.ModFn},
		},
		{
			name:     "F3 should be one event, not Esc+O+R",
			input:    []byte("\x1bOR"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'R', Modifiers: gtv.ModFn},
		},
		{
			name:     "F4 should be one event, not Esc+O+S",
			input:    []byte("\x1bOS"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'S', Modifiers: gtv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			// This is the key assertion: we should get exactly 1 event, not 3
			require.Len(t, events, 1, "F-keys should produce exactly one event, not multiple")
			assert.Equal(t, tt.expected, events[0])
		})
	}
}

// TestInputEventReader_ModifiedFunctionKeys tests parsing of F1-F4 with Shift/Ctrl/Alt modifiers.
func TestInputEventReader_ModifiedFunctionKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		// Shift+F1-F4 (also known as F13-F16 in some contexts)
		{
			name:     "Shift+F1",
			input:    []byte("\x1b[1;2P"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn | gtv.ModShift},
		},
		{
			name:     "Shift+F2",
			input:    []byte("\x1b[1;2Q"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Q', Modifiers: gtv.ModFn | gtv.ModShift},
		},
		{
			name:     "Shift+F3",
			input:    []byte("\x1b[1;2R"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'R', Modifiers: gtv.ModFn | gtv.ModShift},
		},
		{
			name:     "Shift+F4",
			input:    []byte("\x1b[1;2S"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'S', Modifiers: gtv.ModFn | gtv.ModShift},
		},
		// Alt+F1-F4
		{
			name:     "Alt+F1",
			input:    []byte("\x1b[1;3P"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		{
			name:     "Alt+F2",
			input:    []byte("\x1b[1;3Q"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Q', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		{
			name:     "Alt+F3",
			input:    []byte("\x1b[1;3R"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'R', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		{
			name:     "Alt+F4",
			input:    []byte("\x1b[1;3S"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'S', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		// Shift+Alt+F1-F4
		{
			name:     "Shift+Alt+F1",
			input:    []byte("\x1b[1;4P"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn | gtv.ModShift | gtv.ModAlt},
		},
		{
			name:     "Shift+Alt+F2",
			input:    []byte("\x1b[1;4Q"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Q', Modifiers: gtv.ModFn | gtv.ModShift | gtv.ModAlt},
		},
		{
			name:     "Shift+Alt+F3",
			input:    []byte("\x1b[1;4R"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'R', Modifiers: gtv.ModFn | gtv.ModShift | gtv.ModAlt},
		},
		{
			name:     "Shift+Alt+F4",
			input:    []byte("\x1b[1;4S"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'S', Modifiers: gtv.ModFn | gtv.ModShift | gtv.ModAlt},
		},
		// Ctrl+F1-F4
		{
			name:     "Ctrl+F1",
			input:    []byte("\x1b[1;5P"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn | gtv.ModCtrl},
		},
		{
			name:     "Ctrl+F2",
			input:    []byte("\x1b[1;5Q"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Q', Modifiers: gtv.ModFn | gtv.ModCtrl},
		},
		{
			name:     "Ctrl+F3",
			input:    []byte("\x1b[1;5R"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'R', Modifiers: gtv.ModFn | gtv.ModCtrl},
		},
		{
			name:     "Ctrl+F4",
			input:    []byte("\x1b[1;5S"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'S', Modifiers: gtv.ModFn | gtv.ModCtrl},
		},
		// Ctrl+Shift+F1-F4
		{
			name:     "Ctrl+Shift+F1",
			input:    []byte("\x1b[1;6P"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModShift},
		},
		{
			name:     "Ctrl+Shift+F2",
			input:    []byte("\x1b[1;6Q"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Q', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModShift},
		},
		{
			name:     "Ctrl+Shift+F3",
			input:    []byte("\x1b[1;6R"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'R', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModShift},
		},
		{
			name:     "Ctrl+Shift+F4",
			input:    []byte("\x1b[1;6S"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'S', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModShift},
		},
		// Ctrl+Alt+F1-F4
		{
			name:     "Ctrl+Alt+F1",
			input:    []byte("\x1b[1;7P"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModAlt},
		},
		{
			name:     "Ctrl+Alt+F2",
			input:    []byte("\x1b[1;7Q"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Q', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModAlt},
		},
		{
			name:     "Ctrl+Alt+F3",
			input:    []byte("\x1b[1;7R"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'R', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModAlt},
		},
		{
			name:     "Ctrl+Alt+F4",
			input:    []byte("\x1b[1;7S"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'S', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModAlt},
		},
		// Ctrl+Shift+Alt+F1-F4
		{
			name:     "Ctrl+Shift+Alt+F1",
			input:    []byte("\x1b[1;8P"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModShift | gtv.ModAlt},
		},
		{
			name:     "Ctrl+Shift+Alt+F2",
			input:    []byte("\x1b[1;8Q"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Q', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModShift | gtv.ModAlt},
		},
		{
			name:     "Ctrl+Shift+Alt+F3",
			input:    []byte("\x1b[1;8R"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'R', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModShift | gtv.ModAlt},
		},
		{
			name:     "Ctrl+Shift+Alt+F4",
			input:    []byte("\x1b[1;8S"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'S', Modifiers: gtv.ModFn | gtv.ModCtrl | gtv.ModShift | gtv.ModAlt},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1, "Expected exactly one event for %s", tt.name)
			assert.Equal(t, tt.expected, events[0], "Event mismatch for %s", tt.name)
		})
	}
}

// TestInputEventReader_ModifiedF5toF12Keys tests parsing of F5-F12 with modifiers.
func TestInputEventReader_ModifiedF5toF12Keys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		// Alt+F5-F12 (CSI tilde format with modifier)
		{
			name:     "Alt+F5",
			input:    []byte("\x1b[15;3~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'T', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		{
			name:     "Alt+F6",
			input:    []byte("\x1b[17;3~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'U', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		{
			name:     "Alt+F7",
			input:    []byte("\x1b[18;3~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'V', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		{
			name:     "Alt+F8",
			input:    []byte("\x1b[19;3~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'W', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		{
			name:     "Alt+F9",
			input:    []byte("\x1b[20;3~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'X', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		{
			name:     "Alt+F10",
			input:    []byte("\x1b[21;3~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Y', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		{
			name:     "Alt+F11",
			input:    []byte("\x1b[23;3~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Z', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		{
			name:     "Alt+F12",
			input:    []byte("\x1b[24;3~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: '[', Modifiers: gtv.ModFn | gtv.ModAlt},
		},
		// Ctrl+F5-F12
		{
			name:     "Ctrl+F5",
			input:    []byte("\x1b[15;5~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'T', Modifiers: gtv.ModFn | gtv.ModCtrl},
		},
		{
			name:     "Ctrl+F6",
			input:    []byte("\x1b[17;5~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'U', Modifiers: gtv.ModFn | gtv.ModCtrl},
		},
		{
			name:     "Ctrl+F12",
			input:    []byte("\x1b[24;5~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: '[', Modifiers: gtv.ModFn | gtv.ModCtrl},
		},
		// Shift+F5-F12
		{
			name:     "Shift+F5",
			input:    []byte("\x1b[15;2~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'T', Modifiers: gtv.ModFn | gtv.ModShift},
		},
		{
			name:     "Shift+F6",
			input:    []byte("\x1b[17;2~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'U', Modifiers: gtv.ModFn | gtv.ModShift},
		},
		{
			name:     "Shift+F12",
			input:    []byte("\x1b[24;2~"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: '[', Modifiers: gtv.ModFn | gtv.ModShift},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1, "Expected exactly one event for %s", tt.name)
			assert.Equal(t, tt.expected, events[0], "Event mismatch for %s", tt.name)
		})
	}
}

// TestInputEventReader_EscapeKey tests parsing of ESC key.
func TestInputEventReader_EscapeKey(t *testing.T) {
	handler := gtv.NewMockInputEventHandler()
	input := []byte{0x1B}
	reader := bytes.NewReader(input)
	eventReader := NewInputEventReader(reader, nil, handler)

	eventReader.parseInput(input)

	events := handler.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, gtv.InputEvent{Type: gtv.InputEventKey, Key: 0x1B}, events[0])
}

// TestInputEventReader_StartStop tests starting and stopping the reader.
func TestInputEventReader_StartStop(t *testing.T) {
	handler := gtv.NewMockInputEventHandler()
	input := bytes.NewReader([]byte("test"))
	eventReader := NewInputEventReader(input, nil, handler)

	err := eventReader.Start()
	require.NoError(t, err)

	// Wait a bit for the goroutine to process
	time.Sleep(50 * time.Millisecond)

	eventReader.Stop()

	// Verify that we got at least the initial resize event
	events := handler.GetEvents()
	require.NotEmpty(t, events)
	assert.Equal(t, gtv.InputEventResize, events[0].Type)
}

// TestInputEventReader_InitialResizeEvent tests that initial resize event is sent.
func TestInputEventReader_InitialResizeEvent(t *testing.T) {
	handler := gtv.NewMockInputEventHandler()
	input := bytes.NewReader([]byte{})
	eventReader := NewInputEventReader(input, nil, handler)

	err := eventReader.Start()
	require.NoError(t, err)

	// Wait a bit for the initial resize event
	time.Sleep(10 * time.Millisecond)

	eventReader.Stop()

	events := handler.GetEvents()
	require.NotEmpty(t, events)

	// First event should be resize
	assert.Equal(t, gtv.InputEventResize, events[0].Type)
	// Should have some width and height (defaults to 80x24 if can't get terminal size)
	assert.Greater(t, events[0].X, uint16(0))
	assert.Greater(t, events[0].Y, uint16(0))
}

// TestInputEventReader_EventOrder tests that events are delivered in correct order.
func TestInputEventReader_EventOrder(t *testing.T) {
	handler := gtv.NewMockInputEventHandler()
	input := []byte("abc")
	reader := bytes.NewReader(input)
	eventReader := NewInputEventReader(reader, nil, handler)

	eventReader.parseInput(input)

	events := handler.GetEvents()
	require.Len(t, events, 3)

	// Verify order
	assert.Equal(t, 'a', events[0].Key)
	assert.Equal(t, 'b', events[1].Key)
	assert.Equal(t, 'c', events[2].Key)
}

// TestInputEventReader_MixedInput tests parsing of mixed input (keys and escape sequences).
func TestInputEventReader_MixedInput(t *testing.T) {
	handler := gtv.NewMockInputEventHandler()
	// Input: 'a', Up arrow, 'b', Down arrow, 'c'
	input := []byte{'a', 0x1B, '[', 'A', 'b', 0x1B, '[', 'B', 'c'}
	reader := bytes.NewReader(input)
	eventReader := NewInputEventReader(reader, nil, handler)

	eventReader.parseInput(input)

	events := handler.GetEvents()
	require.Len(t, events, 5)

	assert.Equal(t, gtv.InputEvent{Type: gtv.InputEventKey, Key: 'a'}, events[0])
	assert.Equal(t, gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModFn}, events[1])
	assert.Equal(t, gtv.InputEvent{Type: gtv.InputEventKey, Key: 'b'}, events[2])
	assert.Equal(t, gtv.InputEvent{Type: gtv.InputEventKey, Key: 'B', Modifiers: gtv.ModFn}, events[3])
	assert.Equal(t, gtv.InputEvent{Type: gtv.InputEventKey, Key: 'c'}, events[4])
}

// TestMockInputEventHandler tests the mock handler functionality.
func TestMockInputEventHandler(t *testing.T) {
	handler := gtv.NewMockInputEventHandler()

	// Initially empty
	assert.Equal(t, 0, handler.EventCount())
	assert.Empty(t, handler.GetEvents())

	// Add some events
	event1 := gtv.InputEvent{Type: gtv.InputEventKey, Key: 'a'}
	event2 := gtv.InputEvent{Type: gtv.InputEventKey, Key: 'b'}

	handler.Notify(event1)
	handler.Notify(event2)

	// Verify count
	assert.Equal(t, 2, handler.EventCount())

	// Verify events
	events := handler.GetEvents()
	require.Len(t, events, 2)
	assert.Equal(t, event1, events[0])
	assert.Equal(t, event2, events[1])

	// Clear
	handler.Clear()
	assert.Equal(t, 0, handler.EventCount())
	assert.Empty(t, handler.GetEvents())
}

// TestInputEventReader_DoubleStart tests that starting twice returns an error.
func TestInputEventReader_DoubleStart(t *testing.T) {
	handler := gtv.NewMockInputEventHandler()
	input := bytes.NewReader([]byte{})
	eventReader := NewInputEventReader(input, nil, handler)

	err := eventReader.Start()
	require.NoError(t, err)

	// Try to start again
	err = eventReader.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	eventReader.Stop()
}

// TestInputEventReader_ShiftArrowKeys tests that Shift+arrow keys are parsed correctly with Shift modifier.
// This is a regression test for a bug where Shift+A was incorrectly reported as arrow up without Shift modifier.
func TestInputEventReader_ShiftArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		{
			name:     "Shift+Up",
			input:    []byte("\x1b[1;2A"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModShift | gtv.ModFn},
		},
		{
			name:     "Shift+Down",
			input:    []byte("\x1b[1;2B"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'B', Modifiers: gtv.ModShift | gtv.ModFn},
		},
		{
			name:     "Shift+Right",
			input:    []byte("\x1b[1;2C"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'C', Modifiers: gtv.ModShift | gtv.ModFn},
		},
		{
			name:     "Shift+Left",
			input:    []byte("\x1b[1;2D"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'D', Modifiers: gtv.ModShift | gtv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1)
			assert.Equal(t, tt.expected, events[0])
		})
	}
}

// TestInputEventReader_CtrlArrowKeys tests that Ctrl+arrow keys are parsed correctly.
func TestInputEventReader_CtrlArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		{
			name:     "Ctrl+Up",
			input:    []byte("\x1b[1;5A"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModCtrl | gtv.ModFn},
		},
		{
			name:     "Ctrl+Down",
			input:    []byte("\x1b[1;5B"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'B', Modifiers: gtv.ModCtrl | gtv.ModFn},
		},
		{
			name:     "Ctrl+Right",
			input:    []byte("\x1b[1;5C"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'C', Modifiers: gtv.ModCtrl | gtv.ModFn},
		},
		{
			name:     "Ctrl+Left",
			input:    []byte("\x1b[1;5D"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'D', Modifiers: gtv.ModCtrl | gtv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1)
			assert.Equal(t, tt.expected, events[0])
		})
	}
}

// TestInputEventReader_AltArrowKeys tests that Alt+arrow keys are parsed correctly.
func TestInputEventReader_AltArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		{
			name:     "Alt+Up",
			input:    []byte("\x1b[1;3A"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModAlt | gtv.ModFn},
		},
		{
			name:     "Alt+Down",
			input:    []byte("\x1b[1;3B"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'B', Modifiers: gtv.ModAlt | gtv.ModFn},
		},
		{
			name:     "Alt+Right",
			input:    []byte("\x1b[1;3C"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'C', Modifiers: gtv.ModAlt | gtv.ModFn},
		},
		{
			name:     "Alt+Left",
			input:    []byte("\x1b[1;3D"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'D', Modifiers: gtv.ModAlt | gtv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1)
			assert.Equal(t, tt.expected, events[0])
		})
	}
}

// TestInputEventReader_CtrlShiftArrowKeys tests that Ctrl+Shift+arrow keys are parsed correctly.
func TestInputEventReader_CtrlShiftArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		{
			name:     "Ctrl+Shift+Up",
			input:    []byte("\x1b[1;6A"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModCtrl | gtv.ModShift | gtv.ModFn},
		},
		{
			name:     "Ctrl+Shift+Down",
			input:    []byte("\x1b[1;6B"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'B', Modifiers: gtv.ModCtrl | gtv.ModShift | gtv.ModFn},
		},
		{
			name:     "Ctrl+Shift+Right",
			input:    []byte("\x1b[1;6C"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'C', Modifiers: gtv.ModCtrl | gtv.ModShift | gtv.ModFn},
		},
		{
			name:     "Ctrl+Shift+Left",
			input:    []byte("\x1b[1;6D"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'D', Modifiers: gtv.ModCtrl | gtv.ModShift | gtv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1)
			assert.Equal(t, tt.expected, events[0])
		})
	}
}

// TestInputEventReader_NotifyResize tests that resize events can be triggered via NotifyResize.
// This is a regression test for the bug where terminal resize events were not reported
// because SIGWINCH signal handling was missing.
func TestInputEventReader_NotifyResize(t *testing.T) {
	handler := gtv.NewMockInputEventHandler()
	reader := bytes.NewReader([]byte{})
	eventReader := NewInputEventReader(reader, nil, handler)

	// Call NotifyResize to simulate a resize event
	eventReader.NotifyResize(120, 40)

	events := handler.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, gtv.InputEventResize, events[0].Type)
	assert.Equal(t, uint16(120), events[0].X)
	assert.Equal(t, uint16(40), events[0].Y)
}

// TestInputEventReader_NotifyResizeMultiple tests that multiple resize events are handled correctly.
func TestInputEventReader_NotifyResizeMultiple(t *testing.T) {
	handler := gtv.NewMockInputEventHandler()
	reader := bytes.NewReader([]byte{})
	eventReader := NewInputEventReader(reader, nil, handler)

	// Simulate multiple resize events
	eventReader.NotifyResize(80, 24)
	eventReader.NotifyResize(100, 50)
	eventReader.NotifyResize(120, 40)

	events := handler.GetEvents()
	require.Len(t, events, 3)

	// Verify first resize
	assert.Equal(t, gtv.InputEventResize, events[0].Type)
	assert.Equal(t, uint16(80), events[0].X)
	assert.Equal(t, uint16(24), events[0].Y)

	// Verify second resize
	assert.Equal(t, gtv.InputEventResize, events[1].Type)
	assert.Equal(t, uint16(100), events[1].X)
	assert.Equal(t, uint16(50), events[1].Y)

	// Verify third resize
	assert.Equal(t, gtv.InputEventResize, events[2].Type)
	assert.Equal(t, uint16(120), events[2].X)
	assert.Equal(t, uint16(40), events[2].Y)
}

// TestInputEventReader_ShiftLetterKeys tests that Shift+letter keys are parsed correctly.
// This is a regression test for a bug where Shift+A was incorrectly reported as arrow up
// because uppercase 'A' was not recognized as Shift+a.
func TestInputEventReader_ShiftLetterKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected gtv.InputEvent
	}{
		{
			name:     "Shift+A (uppercase A)",
			input:    []byte("A"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModShift},
		},
		{
			name:     "Shift+D (uppercase D)",
			input:    []byte("D"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'D', Modifiers: gtv.ModShift},
		},
		{
			name:     "Shift+Z (uppercase Z)",
			input:    []byte("Z"),
			expected: gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Z', Modifiers: gtv.ModShift},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1)
			assert.Equal(t, tt.expected, events[0])
		})
	}
}

// TestInputEventReader_ShiftLetterVsArrowKey tests that Shift+A letter and Up arrow
// are correctly distinguished from each other.
func TestInputEventReader_ShiftLetterVsArrowKey(t *testing.T) {
	handler := gtv.NewMockInputEventHandler()
	eventReader := NewInputEventReader(bytes.NewReader([]byte{}), nil, handler)

	// First, simulate pressing Shift+A (uppercase letter)
	eventReader.parseInput([]byte("A"))

	// Then, simulate pressing Up arrow
	eventReader.parseInput([]byte("\x1b[A"))

	events := handler.GetEvents()
	require.Len(t, events, 2)

	// First event: Shift+A letter - should be uppercase 'A' with Shift modifier
	assert.Equal(t, gtv.InputEventKey, events[0].Type)
	assert.Equal(t, 'A', events[0].Key)
	assert.Equal(t, gtv.ModShift, events[0].Modifiers)

	// Second event: Up arrow - should be 'A' with ModFn modifier
	assert.Equal(t, gtv.InputEventKey, events[1].Type)
	assert.Equal(t, 'A', events[1].Key)
	assert.Equal(t, gtv.ModFn, events[1].Modifiers)
}

// TestInputEventReader_SGRMouseEvents tests parsing of SGR mouse events.
// SGR mouse mode uses sequences like ESC[<btn;x;y;m (release) or ESC[<btn;x;y;M (press).
// This test ensures that mouse events are correctly parsed and not misinterpreted as key events.
func TestInputEventReader_SGRMouseEvents(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []gtv.InputEvent
	}{
		{
			name:  "SGR left button press",
			input: []byte("\x1b[<0;10;20;M"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 9, Y: 19, Modifiers: 0},
			},
		},
		{
			name:  "SGR left button release",
			input: []byte("\x1b[<0;10;20;m"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 9, Y: 19, Modifiers: 0},
			},
		},
		{
			name:  "SGR right button press",
			input: []byte("\x1b[<2;50;10;M"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 49, Y: 9, Modifiers: 0},
			},
		},
		{
			name:  "SGR right button release",
			input: []byte("\x1b[<2;50;10;m"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 49, Y: 9, Modifiers: 0},
			},
		},
		{
			name:  "SGR middle button press",
			input: []byte("\x1b[<1;25;15;M"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 24, Y: 14, Modifiers: 0},
			},
		},
		{
			name:  "SGR mouse drag with right button",
			input: []byte("\x1b[<34;50;10;M"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 49, Y: 9, Modifiers: gtv.ModMove},
			},
		},
		{
			name:  "SGR mouse drag with left button",
			input: []byte("\x1b[<32;30;20;M"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 29, Y: 19, Modifiers: gtv.ModMove},
			},
		},
		{
			name:  "SGR scroll up",
			input: []byte("\x1b[<64;40;30;M"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 39, Y: 29, Modifiers: gtv.ModScrollUp},
			},
		},
		{
			name:  "SGR scroll down",
			input: []byte("\x1b[<65;40;30;M"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 39, Y: 29, Modifiers: gtv.ModScrollDown},
			},
		},
		{
			name:  "SGR mouse with Shift modifier",
			input: []byte("\x1b[<4;10;10;M"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 9, Y: 9, Modifiers: gtv.ModShift},
			},
		},
		{
			name:  "SGR mouse with Alt modifier",
			input: []byte("\x1b[<8;10;10;M"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 9, Y: 9, Modifiers: gtv.ModAlt},
			},
		},
		{
			name:  "SGR mouse with Ctrl modifier",
			input: []byte("\x1b[<16;10;10;M"),
			expected: []gtv.InputEvent{
				{Type: gtv.InputEventMouse, X: 9, Y: 9, Modifiers: gtv.ModCtrl},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gtv.NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			// Parse the input directly (without starting the reader)
			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, len(tt.expected), "Expected %d events, got %d", len(tt.expected), len(events))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.Type, events[i].Type, "Event %d: type mismatch", i)
				assert.Equal(t, expected.X, events[i].X, "Event %d: X coordinate mismatch", i)
				assert.Equal(t, expected.Y, events[i].Y, "Event %d: Y coordinate mismatch", i)
				assert.Equal(t, expected.Modifiers, events[i].Modifiers, "Event %d: modifiers mismatch", i)
			}
		})
	}
}

// TestInputEvent_String tests the String() method of InputEvent.
func TestInputEvent_String(t *testing.T) {
	tests := []struct {
		name     string
		event    gtv.InputEvent
		expected string
	}{
		{
			name:     "lowercase letter",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'a'},
			expected: "a",
		},
		{
			name:     "uppercase letter with Shift",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModShift},
			expected: "A",
		},
		{
			name:     "Ctrl+C",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'c', Modifiers: gtv.ModCtrl},
			expected: "Ctrl-c",
		},
		{
			name:     "Ctrl+Shift+A",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModCtrl | gtv.ModShift},
			expected: "Ctrl-A",
		},
		{
			name:     "Up arrow",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModFn},
			expected: "Up",
		},
		{
			name:     "Down arrow",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'B', Modifiers: gtv.ModFn},
			expected: "Down",
		},
		{
			name:     "Left arrow",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'D', Modifiers: gtv.ModFn},
			expected: "Left",
		},
		{
			name:     "Right arrow",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'C', Modifiers: gtv.ModFn},
			expected: "Right",
		},
		{
			name:     "Shift+Up",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'A', Modifiers: gtv.ModShift | gtv.ModFn},
			expected: "Shift-Up",
		},
		{
			name:     "Ctrl+Left",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'D', Modifiers: gtv.ModCtrl | gtv.ModFn},
			expected: "Ctrl-Left",
		},
		{
			name:     "Home",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'H', Modifiers: gtv.ModFn},
			expected: "Home",
		},
		{
			name:     "End",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'F', Modifiers: gtv.ModFn},
			expected: "End",
		},
		{
			name:     "PageUp",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'G', Modifiers: gtv.ModFn},
			expected: "PageUp",
		},
		{
			name:     "PageDown",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'N', Modifiers: gtv.ModFn},
			expected: "PageDown",
		},
		{
			name:     "Insert",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'I', Modifiers: gtv.ModFn},
			expected: "Insert",
		},
		{
			name:     "F1",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModFn},
			expected: "F1",
		},
		{
			name:     "F5",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'T', Modifiers: gtv.ModFn},
			expected: "F5",
		},
		{
			name:     "F10",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'Y', Modifiers: gtv.ModFn},
			expected: "F10",
		},
		{
			name:     "F12",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: '[', Modifiers: gtv.ModFn},
			expected: "F12",
		},
		{
			name:     "Shift+F1",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 'P', Modifiers: gtv.ModShift | gtv.ModFn},
			expected: "Shift-F1",
		},
		{
			name:     "Tab",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: '\t', Modifiers: gtv.ModCtrl},
			expected: "Ctrl-Tab",
		},
		{
			name:     "Enter",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: '\r', Modifiers: gtv.ModCtrl},
			expected: "Ctrl-Enter",
		},
		{
			name:     "Esc",
			event:    gtv.InputEvent{Type: gtv.InputEventKey, Key: 0x1B},
			expected: "Esc",
		},
		{
			name:     "Mouse event",
			event:    gtv.InputEvent{Type: gtv.InputEventMouse},
			expected: "Mouse",
		},
		{
			name:     "Resize event",
			event:    gtv.InputEvent{Type: gtv.InputEventResize},
			expected: "Resize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
