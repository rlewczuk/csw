package tio

import (
	"bytes"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/gophertv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInputEventReader_RegularKeys tests parsing of regular key presses.
func TestInputEventReader_RegularKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []gophertv.InputEvent
	}{
		{
			name:  "single letter",
			input: []byte("a"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventKey, Key: 'a'},
			},
		},
		{
			name:  "multiple letters",
			input: []byte("abc"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventKey, Key: 'a'},
				{Type: gophertv.InputEventKey, Key: 'b'},
				{Type: gophertv.InputEventKey, Key: 'c'},
			},
		},
		{
			name:  "numbers",
			input: []byte("123"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventKey, Key: '1'},
				{Type: gophertv.InputEventKey, Key: '2'},
				{Type: gophertv.InputEventKey, Key: '3'},
			},
		},
		{
			name:  "special characters",
			input: []byte("!@#"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventKey, Key: '!'},
				{Type: gophertv.InputEventKey, Key: '@'},
				{Type: gophertv.InputEventKey, Key: '#'},
			},
		},
		{
			name:  "enter key",
			input: []byte{'\r'},
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventKey, Key: '\r', Modifiers: gophertv.ModCtrl},
			},
		},
		{
			name:  "tab key",
			input: []byte{'\t'},
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventKey, Key: '\t', Modifiers: gophertv.ModCtrl},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		expected []gophertv.InputEvent
	}{
		{
			name:  "Ctrl+A",
			input: []byte{0x01},
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventKey, Key: 'a', Modifiers: gophertv.ModCtrl},
			},
		},
		{
			name:  "Ctrl+C",
			input: []byte{0x03},
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventKey, Key: 'c', Modifiers: gophertv.ModCtrl},
			},
		},
		{
			name:  "Ctrl+Z",
			input: []byte{0x1A},
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventKey, Key: 'z', Modifiers: gophertv.ModCtrl},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		expected gophertv.InputEvent
	}{
		{
			name:     "Up arrow",
			input:    []byte("\x1b[A"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModFn},
		},
		{
			name:     "Down arrow",
			input:    []byte("\x1b[B"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'B', Modifiers: gophertv.ModFn},
		},
		{
			name:     "Right arrow",
			input:    []byte("\x1b[C"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'C', Modifiers: gophertv.ModFn},
		},
		{
			name:     "Left arrow",
			input:    []byte("\x1b[D"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'D', Modifiers: gophertv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		expected gophertv.InputEvent
	}{
		{
			name:     "Home key (CSI H)",
			input:    []byte("\x1b[H"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'H', Modifiers: gophertv.ModFn},
		},
		{
			name:     "End key (CSI F)",
			input:    []byte("\x1b[F"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'F', Modifiers: gophertv.ModFn},
		},
		{
			name:     "Home key (tilde)",
			input:    []byte("\x1b[1~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'H', Modifiers: gophertv.ModFn},
		},
		{
			name:     "Insert key",
			input:    []byte("\x1b[2~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'I', Modifiers: gophertv.ModFn},
		},
		{
			name:     "Delete key",
			input:    []byte("\x1b[3~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'D', Modifiers: gophertv.ModFn},
		},
		{
			name:     "Page Up",
			input:    []byte("\x1b[5~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'G', Modifiers: gophertv.ModFn},
		},
		{
			name:     "Page Down",
			input:    []byte("\x1b[6~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'N', Modifiers: gophertv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		expected gophertv.InputEvent
	}{
		// F1-F4 use ESC O sequences (vt100/xterm)
		{
			name:     "F1 (ESC O P)",
			input:    []byte("\x1bOP"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F2 (ESC O Q)",
			input:    []byte("\x1bOQ"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Q', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F3 (ESC O R)",
			input:    []byte("\x1bOR"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'R', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F4 (ESC O S)",
			input:    []byte("\x1bOS"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'S', Modifiers: gophertv.ModFn},
		},
		// F1-F4 also have CSI tilde format (urxvt)
		{
			name:     "F1 (CSI 11~)",
			input:    []byte("\x1b[11~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F2 (CSI 12~)",
			input:    []byte("\x1b[12~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Q', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F3 (CSI 13~)",
			input:    []byte("\x1b[13~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'R', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F4 (CSI 14~)",
			input:    []byte("\x1b[14~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'S', Modifiers: gophertv.ModFn},
		},
		// F5-F12 use CSI tilde format
		{
			name:     "F5",
			input:    []byte("\x1b[15~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'T', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F6",
			input:    []byte("\x1b[17~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'U', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F7",
			input:    []byte("\x1b[18~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'V', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F8",
			input:    []byte("\x1b[19~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'W', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F9",
			input:    []byte("\x1b[20~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'X', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F10",
			input:    []byte("\x1b[21~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Y', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F11",
			input:    []byte("\x1b[23~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Z', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F12",
			input:    []byte("\x1b[24~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: '[', Modifiers: gophertv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		expected gophertv.InputEvent
	}{
		{
			name:     "F1 should be one event, not Esc+O+P",
			input:    []byte("\x1bOP"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F2 should be one event, not Esc+O+Q",
			input:    []byte("\x1bOQ"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Q', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F3 should be one event, not Esc+O+R",
			input:    []byte("\x1bOR"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'R', Modifiers: gophertv.ModFn},
		},
		{
			name:     "F4 should be one event, not Esc+O+S",
			input:    []byte("\x1bOS"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'S', Modifiers: gophertv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		expected gophertv.InputEvent
	}{
		// Shift+F1-F4 (also known as F13-F16 in some contexts)
		{
			name:     "Shift+F1",
			input:    []byte("\x1b[1;2P"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn | gophertv.ModShift},
		},
		{
			name:     "Shift+F2",
			input:    []byte("\x1b[1;2Q"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Q', Modifiers: gophertv.ModFn | gophertv.ModShift},
		},
		{
			name:     "Shift+F3",
			input:    []byte("\x1b[1;2R"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'R', Modifiers: gophertv.ModFn | gophertv.ModShift},
		},
		{
			name:     "Shift+F4",
			input:    []byte("\x1b[1;2S"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'S', Modifiers: gophertv.ModFn | gophertv.ModShift},
		},
		// Alt+F1-F4
		{
			name:     "Alt+F1",
			input:    []byte("\x1b[1;3P"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		{
			name:     "Alt+F2",
			input:    []byte("\x1b[1;3Q"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Q', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		{
			name:     "Alt+F3",
			input:    []byte("\x1b[1;3R"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'R', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		{
			name:     "Alt+F4",
			input:    []byte("\x1b[1;3S"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'S', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		// Shift+Alt+F1-F4
		{
			name:     "Shift+Alt+F1",
			input:    []byte("\x1b[1;4P"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn | gophertv.ModShift | gophertv.ModAlt},
		},
		{
			name:     "Shift+Alt+F2",
			input:    []byte("\x1b[1;4Q"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Q', Modifiers: gophertv.ModFn | gophertv.ModShift | gophertv.ModAlt},
		},
		{
			name:     "Shift+Alt+F3",
			input:    []byte("\x1b[1;4R"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'R', Modifiers: gophertv.ModFn | gophertv.ModShift | gophertv.ModAlt},
		},
		{
			name:     "Shift+Alt+F4",
			input:    []byte("\x1b[1;4S"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'S', Modifiers: gophertv.ModFn | gophertv.ModShift | gophertv.ModAlt},
		},
		// Ctrl+F1-F4
		{
			name:     "Ctrl+F1",
			input:    []byte("\x1b[1;5P"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn | gophertv.ModCtrl},
		},
		{
			name:     "Ctrl+F2",
			input:    []byte("\x1b[1;5Q"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Q', Modifiers: gophertv.ModFn | gophertv.ModCtrl},
		},
		{
			name:     "Ctrl+F3",
			input:    []byte("\x1b[1;5R"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'R', Modifiers: gophertv.ModFn | gophertv.ModCtrl},
		},
		{
			name:     "Ctrl+F4",
			input:    []byte("\x1b[1;5S"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'S', Modifiers: gophertv.ModFn | gophertv.ModCtrl},
		},
		// Ctrl+Shift+F1-F4
		{
			name:     "Ctrl+Shift+F1",
			input:    []byte("\x1b[1;6P"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModShift},
		},
		{
			name:     "Ctrl+Shift+F2",
			input:    []byte("\x1b[1;6Q"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Q', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModShift},
		},
		{
			name:     "Ctrl+Shift+F3",
			input:    []byte("\x1b[1;6R"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'R', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModShift},
		},
		{
			name:     "Ctrl+Shift+F4",
			input:    []byte("\x1b[1;6S"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'S', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModShift},
		},
		// Ctrl+Alt+F1-F4
		{
			name:     "Ctrl+Alt+F1",
			input:    []byte("\x1b[1;7P"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModAlt},
		},
		{
			name:     "Ctrl+Alt+F2",
			input:    []byte("\x1b[1;7Q"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Q', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModAlt},
		},
		{
			name:     "Ctrl+Alt+F3",
			input:    []byte("\x1b[1;7R"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'R', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModAlt},
		},
		{
			name:     "Ctrl+Alt+F4",
			input:    []byte("\x1b[1;7S"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'S', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModAlt},
		},
		// Ctrl+Shift+Alt+F1-F4
		{
			name:     "Ctrl+Shift+Alt+F1",
			input:    []byte("\x1b[1;8P"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModShift | gophertv.ModAlt},
		},
		{
			name:     "Ctrl+Shift+Alt+F2",
			input:    []byte("\x1b[1;8Q"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Q', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModShift | gophertv.ModAlt},
		},
		{
			name:     "Ctrl+Shift+Alt+F3",
			input:    []byte("\x1b[1;8R"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'R', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModShift | gophertv.ModAlt},
		},
		{
			name:     "Ctrl+Shift+Alt+F4",
			input:    []byte("\x1b[1;8S"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'S', Modifiers: gophertv.ModFn | gophertv.ModCtrl | gophertv.ModShift | gophertv.ModAlt},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		expected gophertv.InputEvent
	}{
		// Alt+F5-F12 (CSI tilde format with modifier)
		{
			name:     "Alt+F5",
			input:    []byte("\x1b[15;3~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'T', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		{
			name:     "Alt+F6",
			input:    []byte("\x1b[17;3~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'U', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		{
			name:     "Alt+F7",
			input:    []byte("\x1b[18;3~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'V', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		{
			name:     "Alt+F8",
			input:    []byte("\x1b[19;3~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'W', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		{
			name:     "Alt+F9",
			input:    []byte("\x1b[20;3~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'X', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		{
			name:     "Alt+F10",
			input:    []byte("\x1b[21;3~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Y', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		{
			name:     "Alt+F11",
			input:    []byte("\x1b[23;3~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Z', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		{
			name:     "Alt+F12",
			input:    []byte("\x1b[24;3~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: '[', Modifiers: gophertv.ModFn | gophertv.ModAlt},
		},
		// Ctrl+F5-F12
		{
			name:     "Ctrl+F5",
			input:    []byte("\x1b[15;5~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'T', Modifiers: gophertv.ModFn | gophertv.ModCtrl},
		},
		{
			name:     "Ctrl+F6",
			input:    []byte("\x1b[17;5~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'U', Modifiers: gophertv.ModFn | gophertv.ModCtrl},
		},
		{
			name:     "Ctrl+F12",
			input:    []byte("\x1b[24;5~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: '[', Modifiers: gophertv.ModFn | gophertv.ModCtrl},
		},
		// Shift+F5-F12
		{
			name:     "Shift+F5",
			input:    []byte("\x1b[15;2~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'T', Modifiers: gophertv.ModFn | gophertv.ModShift},
		},
		{
			name:     "Shift+F6",
			input:    []byte("\x1b[17;2~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'U', Modifiers: gophertv.ModFn | gophertv.ModShift},
		},
		{
			name:     "Shift+F12",
			input:    []byte("\x1b[24;2~"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: '[', Modifiers: gophertv.ModFn | gophertv.ModShift},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
	handler := gophertv.NewMockInputEventHandler()
	input := []byte{0x1B}
	reader := bytes.NewReader(input)
	eventReader := NewInputEventReader(reader, nil, handler)

	eventReader.parseInput(input)

	events := handler.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 0x1B}, events[0])
}

// TestInputEventReader_StartStop tests starting and stopping the reader.
func TestInputEventReader_StartStop(t *testing.T) {
	handler := gophertv.NewMockInputEventHandler()
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
	assert.Equal(t, gophertv.InputEventResize, events[0].Type)
}

// TestInputEventReader_InitialResizeEvent tests that initial resize event is sent.
func TestInputEventReader_InitialResizeEvent(t *testing.T) {
	handler := gophertv.NewMockInputEventHandler()
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
	assert.Equal(t, gophertv.InputEventResize, events[0].Type)
	// Should have some width and height (defaults to 80x24 if can't get terminal size)
	assert.Greater(t, events[0].X, uint16(0))
	assert.Greater(t, events[0].Y, uint16(0))
}

// TestInputEventReader_EventOrder tests that events are delivered in correct order.
func TestInputEventReader_EventOrder(t *testing.T) {
	handler := gophertv.NewMockInputEventHandler()
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
	handler := gophertv.NewMockInputEventHandler()
	// Input: 'a', Up arrow, 'b', Down arrow, 'c'
	input := []byte{'a', 0x1B, '[', 'A', 'b', 0x1B, '[', 'B', 'c'}
	reader := bytes.NewReader(input)
	eventReader := NewInputEventReader(reader, nil, handler)

	eventReader.parseInput(input)

	events := handler.GetEvents()
	require.Len(t, events, 5)

	assert.Equal(t, gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'a'}, events[0])
	assert.Equal(t, gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModFn}, events[1])
	assert.Equal(t, gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'b'}, events[2])
	assert.Equal(t, gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'B', Modifiers: gophertv.ModFn}, events[3])
	assert.Equal(t, gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'c'}, events[4])
}

// TestMockInputEventHandler tests the mock handler functionality.
func TestMockInputEventHandler(t *testing.T) {
	handler := gophertv.NewMockInputEventHandler()

	// Initially empty
	assert.Equal(t, 0, handler.EventCount())
	assert.Empty(t, handler.GetEvents())

	// Add some events
	event1 := gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'a'}
	event2 := gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'b'}

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
	handler := gophertv.NewMockInputEventHandler()
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
		expected gophertv.InputEvent
	}{
		{
			name:     "Shift+Up",
			input:    []byte("\x1b[1;2A"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModShift | gophertv.ModFn},
		},
		{
			name:     "Shift+Down",
			input:    []byte("\x1b[1;2B"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'B', Modifiers: gophertv.ModShift | gophertv.ModFn},
		},
		{
			name:     "Shift+Right",
			input:    []byte("\x1b[1;2C"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'C', Modifiers: gophertv.ModShift | gophertv.ModFn},
		},
		{
			name:     "Shift+Left",
			input:    []byte("\x1b[1;2D"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'D', Modifiers: gophertv.ModShift | gophertv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		expected gophertv.InputEvent
	}{
		{
			name:     "Ctrl+Up",
			input:    []byte("\x1b[1;5A"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModCtrl | gophertv.ModFn},
		},
		{
			name:     "Ctrl+Down",
			input:    []byte("\x1b[1;5B"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'B', Modifiers: gophertv.ModCtrl | gophertv.ModFn},
		},
		{
			name:     "Ctrl+Right",
			input:    []byte("\x1b[1;5C"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'C', Modifiers: gophertv.ModCtrl | gophertv.ModFn},
		},
		{
			name:     "Ctrl+Left",
			input:    []byte("\x1b[1;5D"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'D', Modifiers: gophertv.ModCtrl | gophertv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		expected gophertv.InputEvent
	}{
		{
			name:     "Alt+Up",
			input:    []byte("\x1b[1;3A"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModAlt | gophertv.ModFn},
		},
		{
			name:     "Alt+Down",
			input:    []byte("\x1b[1;3B"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'B', Modifiers: gophertv.ModAlt | gophertv.ModFn},
		},
		{
			name:     "Alt+Right",
			input:    []byte("\x1b[1;3C"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'C', Modifiers: gophertv.ModAlt | gophertv.ModFn},
		},
		{
			name:     "Alt+Left",
			input:    []byte("\x1b[1;3D"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'D', Modifiers: gophertv.ModAlt | gophertv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		expected gophertv.InputEvent
	}{
		{
			name:     "Ctrl+Shift+Up",
			input:    []byte("\x1b[1;6A"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModCtrl | gophertv.ModShift | gophertv.ModFn},
		},
		{
			name:     "Ctrl+Shift+Down",
			input:    []byte("\x1b[1;6B"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'B', Modifiers: gophertv.ModCtrl | gophertv.ModShift | gophertv.ModFn},
		},
		{
			name:     "Ctrl+Shift+Right",
			input:    []byte("\x1b[1;6C"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'C', Modifiers: gophertv.ModCtrl | gophertv.ModShift | gophertv.ModFn},
		},
		{
			name:     "Ctrl+Shift+Left",
			input:    []byte("\x1b[1;6D"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'D', Modifiers: gophertv.ModCtrl | gophertv.ModShift | gophertv.ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
	handler := gophertv.NewMockInputEventHandler()
	reader := bytes.NewReader([]byte{})
	eventReader := NewInputEventReader(reader, nil, handler)

	// Call NotifyResize to simulate a resize event
	eventReader.NotifyResize(120, 40)

	events := handler.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, gophertv.InputEventResize, events[0].Type)
	assert.Equal(t, uint16(120), events[0].X)
	assert.Equal(t, uint16(40), events[0].Y)
}

// TestInputEventReader_NotifyResizeMultiple tests that multiple resize events are handled correctly.
func TestInputEventReader_NotifyResizeMultiple(t *testing.T) {
	handler := gophertv.NewMockInputEventHandler()
	reader := bytes.NewReader([]byte{})
	eventReader := NewInputEventReader(reader, nil, handler)

	// Simulate multiple resize events
	eventReader.NotifyResize(80, 24)
	eventReader.NotifyResize(100, 50)
	eventReader.NotifyResize(120, 40)

	events := handler.GetEvents()
	require.Len(t, events, 3)

	// Verify first resize
	assert.Equal(t, gophertv.InputEventResize, events[0].Type)
	assert.Equal(t, uint16(80), events[0].X)
	assert.Equal(t, uint16(24), events[0].Y)

	// Verify second resize
	assert.Equal(t, gophertv.InputEventResize, events[1].Type)
	assert.Equal(t, uint16(100), events[1].X)
	assert.Equal(t, uint16(50), events[1].Y)

	// Verify third resize
	assert.Equal(t, gophertv.InputEventResize, events[2].Type)
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
		expected gophertv.InputEvent
	}{
		{
			name:     "Shift+A (uppercase A)",
			input:    []byte("A"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModShift},
		},
		{
			name:     "Shift+D (uppercase D)",
			input:    []byte("D"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'D', Modifiers: gophertv.ModShift},
		},
		{
			name:     "Shift+Z (uppercase Z)",
			input:    []byte("Z"),
			expected: gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Z', Modifiers: gophertv.ModShift},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
	handler := gophertv.NewMockInputEventHandler()
	eventReader := NewInputEventReader(bytes.NewReader([]byte{}), nil, handler)

	// First, simulate pressing Shift+A (uppercase letter)
	eventReader.parseInput([]byte("A"))

	// Then, simulate pressing Up arrow
	eventReader.parseInput([]byte("\x1b[A"))

	events := handler.GetEvents()
	require.Len(t, events, 2)

	// First event: Shift+A letter - should be uppercase 'A' with Shift modifier
	assert.Equal(t, gophertv.InputEventKey, events[0].Type)
	assert.Equal(t, 'A', events[0].Key)
	assert.Equal(t, gophertv.ModShift, events[0].Modifiers)

	// Second event: Up arrow - should be 'A' with ModFn modifier
	assert.Equal(t, gophertv.InputEventKey, events[1].Type)
	assert.Equal(t, 'A', events[1].Key)
	assert.Equal(t, gophertv.ModFn, events[1].Modifiers)
}

// TestInputEventReader_SGRMouseEvents tests parsing of SGR mouse events.
// SGR mouse mode uses sequences like ESC[<btn;x;y;m (release) or ESC[<btn;x;y;M (press).
// This test ensures that mouse events are correctly parsed and not misinterpreted as key events.
func TestInputEventReader_SGRMouseEvents(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []gophertv.InputEvent
	}{
		{
			name:  "SGR left button press",
			input: []byte("\x1b[<0;10;20;M"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 9, Y: 19, Modifiers: 0},
			},
		},
		{
			name:  "SGR left button release",
			input: []byte("\x1b[<0;10;20;m"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 9, Y: 19, Modifiers: 0},
			},
		},
		{
			name:  "SGR right button press",
			input: []byte("\x1b[<2;50;10;M"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 49, Y: 9, Modifiers: 0},
			},
		},
		{
			name:  "SGR right button release",
			input: []byte("\x1b[<2;50;10;m"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 49, Y: 9, Modifiers: 0},
			},
		},
		{
			name:  "SGR middle button press",
			input: []byte("\x1b[<1;25;15;M"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 24, Y: 14, Modifiers: 0},
			},
		},
		{
			name:  "SGR mouse drag with right button",
			input: []byte("\x1b[<34;50;10;M"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 49, Y: 9, Modifiers: gophertv.ModMove},
			},
		},
		{
			name:  "SGR mouse drag with left button",
			input: []byte("\x1b[<32;30;20;M"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 29, Y: 19, Modifiers: gophertv.ModMove},
			},
		},
		{
			name:  "SGR scroll up",
			input: []byte("\x1b[<64;40;30;M"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 39, Y: 29, Modifiers: gophertv.ModScrollUp},
			},
		},
		{
			name:  "SGR scroll down",
			input: []byte("\x1b[<65;40;30;M"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 39, Y: 29, Modifiers: gophertv.ModScrollDown},
			},
		},
		{
			name:  "SGR mouse with Shift modifier",
			input: []byte("\x1b[<4;10;10;M"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 9, Y: 9, Modifiers: gophertv.ModShift},
			},
		},
		{
			name:  "SGR mouse with Alt modifier",
			input: []byte("\x1b[<8;10;10;M"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 9, Y: 9, Modifiers: gophertv.ModAlt},
			},
		},
		{
			name:  "SGR mouse with Ctrl modifier",
			input: []byte("\x1b[<16;10;10;M"),
			expected: []gophertv.InputEvent{
				{Type: gophertv.InputEventMouse, X: 9, Y: 9, Modifiers: gophertv.ModCtrl},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := gophertv.NewMockInputEventHandler()
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
		event    gophertv.InputEvent
		expected string
	}{
		{
			name:     "lowercase letter",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'a'},
			expected: "a",
		},
		{
			name:     "uppercase letter with Shift",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModShift},
			expected: "A",
		},
		{
			name:     "Ctrl+C",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'c', Modifiers: gophertv.ModCtrl},
			expected: "Ctrl-c",
		},
		{
			name:     "Ctrl+Shift+A",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModCtrl | gophertv.ModShift},
			expected: "Ctrl-A",
		},
		{
			name:     "Up arrow",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModFn},
			expected: "Up",
		},
		{
			name:     "Down arrow",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'B', Modifiers: gophertv.ModFn},
			expected: "Down",
		},
		{
			name:     "Left arrow",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'D', Modifiers: gophertv.ModFn},
			expected: "Left",
		},
		{
			name:     "Right arrow",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'C', Modifiers: gophertv.ModFn},
			expected: "Right",
		},
		{
			name:     "Shift+Up",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'A', Modifiers: gophertv.ModShift | gophertv.ModFn},
			expected: "Shift-Up",
		},
		{
			name:     "Ctrl+Left",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'D', Modifiers: gophertv.ModCtrl | gophertv.ModFn},
			expected: "Ctrl-Left",
		},
		{
			name:     "Home",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'H', Modifiers: gophertv.ModFn},
			expected: "Home",
		},
		{
			name:     "End",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'F', Modifiers: gophertv.ModFn},
			expected: "End",
		},
		{
			name:     "PageUp",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'G', Modifiers: gophertv.ModFn},
			expected: "PageUp",
		},
		{
			name:     "PageDown",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'N', Modifiers: gophertv.ModFn},
			expected: "PageDown",
		},
		{
			name:     "Insert",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'I', Modifiers: gophertv.ModFn},
			expected: "Insert",
		},
		{
			name:     "F1",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModFn},
			expected: "F1",
		},
		{
			name:     "F5",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'T', Modifiers: gophertv.ModFn},
			expected: "F5",
		},
		{
			name:     "F10",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'Y', Modifiers: gophertv.ModFn},
			expected: "F10",
		},
		{
			name:     "F12",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: '[', Modifiers: gophertv.ModFn},
			expected: "F12",
		},
		{
			name:     "Shift+F1",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 'P', Modifiers: gophertv.ModShift | gophertv.ModFn},
			expected: "Shift-F1",
		},
		{
			name:     "Tab",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: '\t', Modifiers: gophertv.ModCtrl},
			expected: "Ctrl-Tab",
		},
		{
			name:     "Enter",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: '\r', Modifiers: gophertv.ModCtrl},
			expected: "Ctrl-Enter",
		},
		{
			name:     "Esc",
			event:    gophertv.InputEvent{Type: gophertv.InputEventKey, Key: 0x1B},
			expected: "Esc",
		},
		{
			name:     "Mouse event",
			event:    gophertv.InputEvent{Type: gophertv.InputEventMouse},
			expected: "Mouse",
		},
		{
			name:     "Resize event",
			event:    gophertv.InputEvent{Type: gophertv.InputEventResize},
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
