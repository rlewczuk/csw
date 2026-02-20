package tio

import (
	"github.com/rlewczuk/csw/pkg/gtv"
)

// MockInputEventReader is a mock implementation of InputEventReader for testing.
// It allows sending input events programmatically without requiring a real terminal.
type MockInputEventReader struct {
	handler gtv.InputEventHandler
}

// NewMockInputEventReader creates a new MockInputEventReader instance.
// The handler is called for each input event sent via the mock methods.
func NewMockInputEventReader(handler gtv.InputEventHandler) *MockInputEventReader {
	return &MockInputEventReader{
		handler: handler,
	}
}

// TypeKeys sends key events for each character in the given string.
// Each character is sent as a separate key event.
// For uppercase letters, the Shift modifier is automatically added.
func (m *MockInputEventReader) TypeKeys(keys string) {
	for _, r := range keys {
		var mods gtv.EventModifiers
		key := r

		// Handle uppercase letters
		if r >= 'A' && r <= 'Z' {
			mods |= gtv.ModShift
		}

		m.handler.Notify(gtv.InputEvent{
			Type:      gtv.InputEventKey,
			Key:       key,
			Modifiers: mods,
		})
	}
}

// TypeKeysByName sends key events for each key name in the given list.
// Key names are parsed using gophertv.ParseKey.
// Examples: "a", "Enter", "Ctrl+C", "F1", "Shift+Up"
func (m *MockInputEventReader) TypeKeysByName(keys ...string) {
	for _, keyName := range keys {
		event, err := gtv.ParseKey(keyName)
		if err != nil {
			// Skip invalid key names silently in tests
			// In a real scenario, this could panic or log an error
			continue
		}
		m.handler.Notify(event)
	}
}

// PressKey sends a single key event with the given key and modifiers.
func (m *MockInputEventReader) PressKey(key rune, modifiers gtv.EventModifiers) {
	m.handler.Notify(gtv.InputEvent{
		Type:      gtv.InputEventKey,
		Key:       key,
		Modifiers: modifiers,
	})
}

// Resize sends a resize event with the given width and height.
func (m *MockInputEventReader) Resize(width, height int) {
	m.handler.Notify(gtv.InputEvent{
		Type: gtv.InputEventResize,
		X:    uint16(width),
		Y:    uint16(height),
	})
}

// MouseClick sends a mouse click event at the given coordinates with the specified button.
// The button parameter should be one of the mouse button modifiers (ModClick, etc.).
func (m *MockInputEventReader) MouseClick(x, y int, button gtv.EventModifiers) {
	m.handler.Notify(gtv.InputEvent{
		Type:      gtv.InputEventMouse,
		X:         uint16(x),
		Y:         uint16(y),
		Modifiers: button | gtv.ModClick | gtv.ModPress,
	})

	// Send release event
	m.handler.Notify(gtv.InputEvent{
		Type:      gtv.InputEventMouse,
		X:         uint16(x),
		Y:         uint16(y),
		Modifiers: button | gtv.ModRelease,
	})
}

// MouseWheel sends a mouse wheel event at the given coordinates with the specified direction.
// The direction parameter should be either ModScrollUp or ModScrollDown.
func (m *MockInputEventReader) MouseWheel(x, y int, direction gtv.EventModifiers) {
	m.handler.Notify(gtv.InputEvent{
		Type:      gtv.InputEventMouse,
		X:         uint16(x),
		Y:         uint16(y),
		Modifiers: direction,
	})
}

// MouseDrag sends mouse drag events from (x1, y1) to (x2, y2).
// It sends a press event at the start, move events along the path, and a release event at the end.
func (m *MockInputEventReader) MouseDrag(x1, y1, x2, y2 int) {
	// Send press event at start position
	m.handler.Notify(gtv.InputEvent{
		Type:      gtv.InputEventMouse,
		X:         uint16(x1),
		Y:         uint16(y1),
		Modifiers: gtv.ModPress,
	})

	// Send drag events
	// For simplicity, we'll send a few intermediate positions
	steps := 5
	for i := 1; i <= steps; i++ {
		x := x1 + (x2-x1)*i/steps
		y := y1 + (y2-y1)*i/steps
		m.handler.Notify(gtv.InputEvent{
			Type:      gtv.InputEventMouse,
			X:         uint16(x),
			Y:         uint16(y),
			Modifiers: gtv.ModDrag | gtv.ModMove,
		})
	}

	// Send release event at end position
	m.handler.Notify(gtv.InputEvent{
		Type:      gtv.InputEventMouse,
		X:         uint16(x2),
		Y:         uint16(y2),
		Modifiers: gtv.ModRelease,
	})
}
