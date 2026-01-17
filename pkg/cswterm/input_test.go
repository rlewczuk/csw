package cswterm

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInputEventReader_RegularKeys tests parsing of regular key presses.
func TestInputEventReader_RegularKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []InputEvent
	}{
		{
			name:  "single letter",
			input: []byte("a"),
			expected: []InputEvent{
				{Type: InputEventKey, Key: 'a'},
			},
		},
		{
			name:  "multiple letters",
			input: []byte("abc"),
			expected: []InputEvent{
				{Type: InputEventKey, Key: 'a'},
				{Type: InputEventKey, Key: 'b'},
				{Type: InputEventKey, Key: 'c'},
			},
		},
		{
			name:  "numbers",
			input: []byte("123"),
			expected: []InputEvent{
				{Type: InputEventKey, Key: '1'},
				{Type: InputEventKey, Key: '2'},
				{Type: InputEventKey, Key: '3'},
			},
		},
		{
			name:  "special characters",
			input: []byte("!@#"),
			expected: []InputEvent{
				{Type: InputEventKey, Key: '!'},
				{Type: InputEventKey, Key: '@'},
				{Type: InputEventKey, Key: '#'},
			},
		},
		{
			name:  "enter key",
			input: []byte{'\r'},
			expected: []InputEvent{
				{Type: InputEventKey, Key: '\r', Modifiers: ModCtrl},
			},
		},
		{
			name:  "tab key",
			input: []byte{'\t'},
			expected: []InputEvent{
				{Type: InputEventKey, Key: '\t', Modifiers: ModCtrl},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMockInputEventHandler()
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
		expected []InputEvent
	}{
		{
			name:  "Ctrl+A",
			input: []byte{0x01},
			expected: []InputEvent{
				{Type: InputEventKey, Key: 'a', Modifiers: ModCtrl},
			},
		},
		{
			name:  "Ctrl+C",
			input: []byte{0x03},
			expected: []InputEvent{
				{Type: InputEventKey, Key: 'c', Modifiers: ModCtrl},
			},
		},
		{
			name:  "Ctrl+Z",
			input: []byte{0x1A},
			expected: []InputEvent{
				{Type: InputEventKey, Key: 'z', Modifiers: ModCtrl},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMockInputEventHandler()
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
		expected InputEvent
	}{
		{
			name:     "Up arrow",
			input:    []byte("\x1b[A"),
			expected: InputEvent{Type: InputEventKey, Key: 'A'},
		},
		{
			name:     "Down arrow",
			input:    []byte("\x1b[B"),
			expected: InputEvent{Type: InputEventKey, Key: 'B'},
		},
		{
			name:     "Right arrow",
			input:    []byte("\x1b[C"),
			expected: InputEvent{Type: InputEventKey, Key: 'C'},
		},
		{
			name:     "Left arrow",
			input:    []byte("\x1b[D"),
			expected: InputEvent{Type: InputEventKey, Key: 'D'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMockInputEventHandler()
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
		expected InputEvent
	}{
		{
			name:     "Home key (CSI H)",
			input:    []byte("\x1b[H"),
			expected: InputEvent{Type: InputEventKey, Key: 'H'},
		},
		{
			name:     "End key (CSI F)",
			input:    []byte("\x1b[F"),
			expected: InputEvent{Type: InputEventKey, Key: 'F'},
		},
		{
			name:     "Home key (tilde)",
			input:    []byte("\x1b[1~"),
			expected: InputEvent{Type: InputEventKey, Key: 'H'},
		},
		{
			name:     "Insert key",
			input:    []byte("\x1b[2~"),
			expected: InputEvent{Type: InputEventKey, Key: 'I'},
		},
		{
			name:     "Delete key",
			input:    []byte("\x1b[3~"),
			expected: InputEvent{Type: InputEventKey, Key: 'D'},
		},
		{
			name:     "Page Up",
			input:    []byte("\x1b[5~"),
			expected: InputEvent{Type: InputEventKey, Key: 'P'},
		},
		{
			name:     "Page Down",
			input:    []byte("\x1b[6~"),
			expected: InputEvent{Type: InputEventKey, Key: 'N'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMockInputEventHandler()
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
		expected InputEvent
	}{
		{
			name:     "F1",
			input:    []byte("\x1b[11~"),
			expected: InputEvent{Type: InputEventKey, Key: 'F', Modifiers: ModFn},
		},
		{
			name:     "F2",
			input:    []byte("\x1b[12~"),
			expected: InputEvent{Type: InputEventKey, Key: 'G', Modifiers: ModFn},
		},
		{
			name:     "F5",
			input:    []byte("\x1b[15~"),
			expected: InputEvent{Type: InputEventKey, Key: 'L', Modifiers: ModFn},
		},
		{
			name:     "F12",
			input:    []byte("\x1b[24~"),
			expected: InputEvent{Type: InputEventKey, Key: 'U', Modifiers: ModFn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMockInputEventHandler()
			reader := bytes.NewReader(tt.input)
			eventReader := NewInputEventReader(reader, nil, handler)

			eventReader.parseInput(tt.input)

			events := handler.GetEvents()
			require.Len(t, events, 1)
			assert.Equal(t, tt.expected, events[0])
		})
	}
}

// TestInputEventReader_EscapeKey tests parsing of ESC key.
func TestInputEventReader_EscapeKey(t *testing.T) {
	handler := NewMockInputEventHandler()
	input := []byte{0x1B}
	reader := bytes.NewReader(input)
	eventReader := NewInputEventReader(reader, nil, handler)

	eventReader.parseInput(input)

	events := handler.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, InputEvent{Type: InputEventKey, Key: 0x1B}, events[0])
}

// TestInputEventReader_StartStop tests starting and stopping the reader.
func TestInputEventReader_StartStop(t *testing.T) {
	handler := NewMockInputEventHandler()
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
	assert.Equal(t, InputEventResize, events[0].Type)
}

// TestInputEventReader_InitialResizeEvent tests that initial resize event is sent.
func TestInputEventReader_InitialResizeEvent(t *testing.T) {
	handler := NewMockInputEventHandler()
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
	assert.Equal(t, InputEventResize, events[0].Type)
	// Should have some width and height (defaults to 80x24 if can't get terminal size)
	assert.Greater(t, events[0].X, uint16(0))
	assert.Greater(t, events[0].Y, uint16(0))
}

// TestInputEventReader_EventOrder tests that events are delivered in correct order.
func TestInputEventReader_EventOrder(t *testing.T) {
	handler := NewMockInputEventHandler()
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
	handler := NewMockInputEventHandler()
	// Input: 'a', Up arrow, 'b', Down arrow, 'c'
	input := []byte{'a', 0x1B, '[', 'A', 'b', 0x1B, '[', 'B', 'c'}
	reader := bytes.NewReader(input)
	eventReader := NewInputEventReader(reader, nil, handler)

	eventReader.parseInput(input)

	events := handler.GetEvents()
	require.Len(t, events, 5)

	assert.Equal(t, InputEvent{Type: InputEventKey, Key: 'a'}, events[0])
	assert.Equal(t, InputEvent{Type: InputEventKey, Key: 'A'}, events[1])
	assert.Equal(t, InputEvent{Type: InputEventKey, Key: 'b'}, events[2])
	assert.Equal(t, InputEvent{Type: InputEventKey, Key: 'B'}, events[3])
	assert.Equal(t, InputEvent{Type: InputEventKey, Key: 'c'}, events[4])
}

// TestMockInputEventHandler tests the mock handler functionality.
func TestMockInputEventHandler(t *testing.T) {
	handler := NewMockInputEventHandler()

	// Initially empty
	assert.Equal(t, 0, handler.EventCount())
	assert.Empty(t, handler.GetEvents())

	// Add some events
	event1 := InputEvent{Type: InputEventKey, Key: 'a'}
	event2 := InputEvent{Type: InputEventKey, Key: 'b'}

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
	handler := NewMockInputEventHandler()
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
		expected InputEvent
	}{
		{
			name:     "Shift+Up",
			input:    []byte("\x1b[1;2A"),
			expected: InputEvent{Type: InputEventKey, Key: 'A', Modifiers: ModShift},
		},
		{
			name:     "Shift+Down",
			input:    []byte("\x1b[1;2B"),
			expected: InputEvent{Type: InputEventKey, Key: 'B', Modifiers: ModShift},
		},
		{
			name:     "Shift+Right",
			input:    []byte("\x1b[1;2C"),
			expected: InputEvent{Type: InputEventKey, Key: 'C', Modifiers: ModShift},
		},
		{
			name:     "Shift+Left",
			input:    []byte("\x1b[1;2D"),
			expected: InputEvent{Type: InputEventKey, Key: 'D', Modifiers: ModShift},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMockInputEventHandler()
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
		expected InputEvent
	}{
		{
			name:     "Ctrl+Up",
			input:    []byte("\x1b[1;5A"),
			expected: InputEvent{Type: InputEventKey, Key: 'A', Modifiers: ModCtrl},
		},
		{
			name:     "Ctrl+Down",
			input:    []byte("\x1b[1;5B"),
			expected: InputEvent{Type: InputEventKey, Key: 'B', Modifiers: ModCtrl},
		},
		{
			name:     "Ctrl+Right",
			input:    []byte("\x1b[1;5C"),
			expected: InputEvent{Type: InputEventKey, Key: 'C', Modifiers: ModCtrl},
		},
		{
			name:     "Ctrl+Left",
			input:    []byte("\x1b[1;5D"),
			expected: InputEvent{Type: InputEventKey, Key: 'D', Modifiers: ModCtrl},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMockInputEventHandler()
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
		expected InputEvent
	}{
		{
			name:     "Alt+Up",
			input:    []byte("\x1b[1;3A"),
			expected: InputEvent{Type: InputEventKey, Key: 'A', Modifiers: ModAlt},
		},
		{
			name:     "Alt+Down",
			input:    []byte("\x1b[1;3B"),
			expected: InputEvent{Type: InputEventKey, Key: 'B', Modifiers: ModAlt},
		},
		{
			name:     "Alt+Right",
			input:    []byte("\x1b[1;3C"),
			expected: InputEvent{Type: InputEventKey, Key: 'C', Modifiers: ModAlt},
		},
		{
			name:     "Alt+Left",
			input:    []byte("\x1b[1;3D"),
			expected: InputEvent{Type: InputEventKey, Key: 'D', Modifiers: ModAlt},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMockInputEventHandler()
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
		expected InputEvent
	}{
		{
			name:     "Ctrl+Shift+Up",
			input:    []byte("\x1b[1;6A"),
			expected: InputEvent{Type: InputEventKey, Key: 'A', Modifiers: ModCtrl | ModShift},
		},
		{
			name:     "Ctrl+Shift+Down",
			input:    []byte("\x1b[1;6B"),
			expected: InputEvent{Type: InputEventKey, Key: 'B', Modifiers: ModCtrl | ModShift},
		},
		{
			name:     "Ctrl+Shift+Right",
			input:    []byte("\x1b[1;6C"),
			expected: InputEvent{Type: InputEventKey, Key: 'C', Modifiers: ModCtrl | ModShift},
		},
		{
			name:     "Ctrl+Shift+Left",
			input:    []byte("\x1b[1;6D"),
			expected: InputEvent{Type: InputEventKey, Key: 'D', Modifiers: ModCtrl | ModShift},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMockInputEventHandler()
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
	handler := NewMockInputEventHandler()
	reader := bytes.NewReader([]byte{})
	eventReader := NewInputEventReader(reader, nil, handler)

	// Call NotifyResize to simulate a resize event
	eventReader.NotifyResize(120, 40)

	events := handler.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, InputEventResize, events[0].Type)
	assert.Equal(t, uint16(120), events[0].X)
	assert.Equal(t, uint16(40), events[0].Y)
}

// TestInputEventReader_NotifyResizeMultiple tests that multiple resize events are handled correctly.
func TestInputEventReader_NotifyResizeMultiple(t *testing.T) {
	handler := NewMockInputEventHandler()
	reader := bytes.NewReader([]byte{})
	eventReader := NewInputEventReader(reader, nil, handler)

	// Simulate multiple resize events
	eventReader.NotifyResize(80, 24)
	eventReader.NotifyResize(100, 50)
	eventReader.NotifyResize(120, 40)

	events := handler.GetEvents()
	require.Len(t, events, 3)

	// Verify first resize
	assert.Equal(t, InputEventResize, events[0].Type)
	assert.Equal(t, uint16(80), events[0].X)
	assert.Equal(t, uint16(24), events[0].Y)

	// Verify second resize
	assert.Equal(t, InputEventResize, events[1].Type)
	assert.Equal(t, uint16(100), events[1].X)
	assert.Equal(t, uint16(50), events[1].Y)

	// Verify third resize
	assert.Equal(t, InputEventResize, events[2].Type)
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
		expected InputEvent
	}{
		{
			name:     "Shift+A (uppercase A)",
			input:    []byte("A"),
			expected: InputEvent{Type: InputEventKey, Key: 'a', Modifiers: ModShift},
		},
		{
			name:     "Shift+D (uppercase D)",
			input:    []byte("D"),
			expected: InputEvent{Type: InputEventKey, Key: 'd', Modifiers: ModShift},
		},
		{
			name:     "Shift+Z (uppercase Z)",
			input:    []byte("Z"),
			expected: InputEvent{Type: InputEventKey, Key: 'z', Modifiers: ModShift},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMockInputEventHandler()
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
	handler := NewMockInputEventHandler()
	eventReader := NewInputEventReader(bytes.NewReader([]byte{}), nil, handler)

	// First, simulate pressing Shift+A (uppercase letter)
	eventReader.parseInput([]byte("A"))

	// Then, simulate pressing Up arrow
	eventReader.parseInput([]byte("\x1b[A"))

	events := handler.GetEvents()
	require.Len(t, events, 2)

	// First event: Shift+A letter - should be lowercase 'a' with Shift modifier
	assert.Equal(t, InputEventKey, events[0].Type)
	assert.Equal(t, 'a', events[0].Key)
	assert.Equal(t, ModShift, events[0].Modifiers)

	// Second event: Up arrow - should be uppercase 'A' without Shift modifier
	assert.Equal(t, InputEventKey, events[1].Type)
	assert.Equal(t, 'A', events[1].Key)
	assert.Equal(t, EventModifiers(0), events[1].Modifiers)
}
