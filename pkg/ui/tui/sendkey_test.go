package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendKey_ControlKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"ctrl+@", "ctrl+@", []byte{0x00}},
		{"ctrl+a", "ctrl+a", []byte{0x01}},
		{"ctrl+b", "ctrl+b", []byte{0x02}},
		{"ctrl+c", "ctrl+c", []byte{0x03}},
		{"ctrl+d", "ctrl+d", []byte{0x04}},
		{"ctrl+e", "ctrl+e", []byte{0x05}},
		{"ctrl+f", "ctrl+f", []byte{0x06}},
		{"ctrl+g", "ctrl+g", []byte{0x07}},
		{"ctrl+h", "ctrl+h", []byte{0x08}},
		{"ctrl+i", "ctrl+i", []byte{0x09}},
		{"tab", "tab", []byte{0x09}},
		{"ctrl+j", "ctrl+j", []byte{0x0a}},
		{"ctrl+k", "ctrl+k", []byte{0x0b}},
		{"ctrl+l", "ctrl+l", []byte{0x0c}},
		{"ctrl+m", "ctrl+m", []byte{0x0d}},
		{"enter", "enter", []byte{0x0d}},
		{"ctrl+n", "ctrl+n", []byte{0x0e}},
		{"ctrl+o", "ctrl+o", []byte{0x0f}},
		{"ctrl+p", "ctrl+p", []byte{0x10}},
		{"ctrl+q", "ctrl+q", []byte{0x11}},
		{"ctrl+r", "ctrl+r", []byte{0x12}},
		{"ctrl+s", "ctrl+s", []byte{0x13}},
		{"ctrl+t", "ctrl+t", []byte{0x14}},
		{"ctrl+u", "ctrl+u", []byte{0x15}},
		{"ctrl+v", "ctrl+v", []byte{0x16}},
		{"ctrl+w", "ctrl+w", []byte{0x17}},
		{"ctrl+x", "ctrl+x", []byte{0x18}},
		{"ctrl+y", "ctrl+y", []byte{0x19}},
		{"ctrl+z", "ctrl+z", []byte{0x1a}},
		{"ctrl+[", "ctrl+[", []byte{0x1b}},
		{"esc", "esc", []byte{0x1b}},
		{"ctrl+\\", "ctrl+\\", []byte{0x1c}},
		{"ctrl+]", "ctrl+]", []byte{0x1d}},
		{"ctrl+^", "ctrl+^", []byte{0x1e}},
		{"ctrl+_", "ctrl+_", []byte{0x1f}},
		{"ctrl+?", "ctrl+?", []byte{0x7f}},
		{"backspace", "backspace", []byte{0x7f}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_ArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"up", "up", []byte{0x1b, '[', 'A'}},
		{"down", "down", []byte{0x1b, '[', 'B'}},
		{"right", "right", []byte{0x1b, '[', 'C'}},
		{"left", "left", []byte{0x1b, '[', 'D'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_ShiftArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"shift+up", "shift+up", []byte{0x1b, '[', '1', ';', '2', 'A'}},
		{"shift+down", "shift+down", []byte{0x1b, '[', '1', ';', '2', 'B'}},
		{"shift+right", "shift+right", []byte{0x1b, '[', '1', ';', '2', 'C'}},
		{"shift+left", "shift+left", []byte{0x1b, '[', '1', ';', '2', 'D'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_CtrlArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"ctrl+up", "ctrl+up", []byte{0x1b, '[', '1', ';', '5', 'A'}},
		{"ctrl+down", "ctrl+down", []byte{0x1b, '[', '1', ';', '5', 'B'}},
		{"ctrl+right", "ctrl+right", []byte{0x1b, '[', '1', ';', '5', 'C'}},
		{"ctrl+left", "ctrl+left", []byte{0x1b, '[', '1', ';', '5', 'D'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_CtrlShiftArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"ctrl+shift+up", "ctrl+shift+up", []byte{0x1b, '[', '1', ';', '6', 'A'}},
		{"ctrl+shift+down", "ctrl+shift+down", []byte{0x1b, '[', '1', ';', '6', 'B'}},
		{"ctrl+shift+right", "ctrl+shift+right", []byte{0x1b, '[', '1', ';', '6', 'C'}},
		{"ctrl+shift+left", "ctrl+shift+left", []byte{0x1b, '[', '1', ';', '6', 'D'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_AltArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"alt+up", "alt+up", []byte{0x1b, '[', '1', ';', '3', 'A'}},
		{"alt+down", "alt+down", []byte{0x1b, '[', '1', ';', '3', 'B'}},
		{"alt+right", "alt+right", []byte{0x1b, '[', '1', ';', '3', 'C'}},
		{"alt+left", "alt+left", []byte{0x1b, '[', '1', ';', '3', 'D'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_NavigationKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"home", "home", []byte{0x1b, '[', 'H'}},
		{"end", "end", []byte{0x1b, '[', 'F'}},
		{"pgup", "pgup", []byte{0x1b, '[', '5', '~'}},
		{"pgdown", "pgdown", []byte{0x1b, '[', '6', '~'}},
		{"insert", "insert", []byte{0x1b, '[', '2', '~'}},
		{"delete", "delete", []byte{0x1b, '[', '3', '~'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_CtrlNavigationKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"ctrl+home", "ctrl+home", []byte{0x1b, '[', '1', ';', '5', 'H'}},
		{"ctrl+end", "ctrl+end", []byte{0x1b, '[', '1', ';', '5', 'F'}},
		{"ctrl+pgup", "ctrl+pgup", []byte{0x1b, '[', '5', ';', '5', '~'}},
		{"ctrl+pgdown", "ctrl+pgdown", []byte{0x1b, '[', '6', ';', '5', '~'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_ShiftNavigationKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"shift+home", "shift+home", []byte{0x1b, '[', '1', ';', '2', 'H'}},
		{"shift+end", "shift+end", []byte{0x1b, '[', '1', ';', '2', 'F'}},
		{"shift+tab", "shift+tab", []byte{0x1b, '[', 'Z'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_CtrlShiftNavigationKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"ctrl+shift+home", "ctrl+shift+home", []byte{0x1b, '[', '1', ';', '6', 'H'}},
		{"ctrl+shift+end", "ctrl+shift+end", []byte{0x1b, '[', '1', ';', '6', 'F'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_AltNavigationKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"alt+home", "alt+home", []byte{0x1b, '[', '1', ';', '3', 'H'}},
		{"alt+end", "alt+end", []byte{0x1b, '[', '1', ';', '3', 'F'}},
		{"alt+pgup", "alt+pgup", []byte{0x1b, '[', '5', ';', '3', '~'}},
		{"alt+pgdown", "alt+pgdown", []byte{0x1b, '[', '6', ';', '3', '~'}},
		{"alt+delete", "alt+delete", []byte{0x1b, '[', '3', ';', '3', '~'}},
		{"alt+insert", "alt+insert", []byte{0x1b, '[', '3', ';', '2', '~'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_FunctionKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		// F1-F4 use different sequences
		{"f1", "f1", []byte{0x1b, 'O', 'P'}},
		{"f2", "f2", []byte{0x1b, 'O', 'Q'}},
		{"f3", "f3", []byte{0x1b, 'O', 'R'}},
		{"f4", "f4", []byte{0x1b, 'O', 'S'}},
		// F5-F12
		{"f5", "f5", []byte{0x1b, '[', '1', '5', '~'}},
		{"f6", "f6", []byte{0x1b, '[', '1', '7', '~'}},
		{"f7", "f7", []byte{0x1b, '[', '1', '8', '~'}},
		{"f8", "f8", []byte{0x1b, '[', '1', '9', '~'}},
		{"f9", "f9", []byte{0x1b, '[', '2', '0', '~'}},
		{"f10", "f10", []byte{0x1b, '[', '2', '1', '~'}},
		{"f11", "f11", []byte{0x1b, '[', '2', '3', '~'}},
		{"f12", "f12", []byte{0x1b, '[', '2', '4', '~'}},
		// F13-F20
		{"f13", "f13", []byte{0x1b, '[', '1', ';', '2', 'P'}},
		{"f14", "f14", []byte{0x1b, '[', '1', ';', '2', 'Q'}},
		{"f15", "f15", []byte{0x1b, '[', '1', ';', '2', 'R'}},
		{"f16", "f16", []byte{0x1b, '[', '1', ';', '2', 'S'}},
		{"f17", "f17", []byte{0x1b, '[', '1', '5', ';', '2', '~'}},
		{"f18", "f18", []byte{0x1b, '[', '1', '7', ';', '2', '~'}},
		{"f19", "f19", []byte{0x1b, '[', '1', '8', ';', '2', '~'}},
		{"f20", "f20", []byte{0x1b, '[', '1', '9', ';', '2', '~'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_AltFunctionKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"alt+f1", "alt+f1", []byte{0x1b, '[', '1', ';', '3', 'P'}},
		{"alt+f2", "alt+f2", []byte{0x1b, '[', '1', ';', '3', 'Q'}},
		{"alt+f3", "alt+f3", []byte{0x1b, '[', '1', ';', '3', 'R'}},
		{"alt+f4", "alt+f4", []byte{0x1b, '[', '1', ';', '3', 'S'}},
		{"alt+f5", "alt+f5", []byte{0x1b, '[', '1', '5', ';', '3', '~'}},
		{"alt+f6", "alt+f6", []byte{0x1b, '[', '1', '7', ';', '3', '~'}},
		{"alt+f7", "alt+f7", []byte{0x1b, '[', '1', '8', ';', '3', '~'}},
		{"alt+f8", "alt+f8", []byte{0x1b, '[', '1', '9', ';', '3', '~'}},
		{"alt+f9", "alt+f9", []byte{0x1b, '[', '2', '0', ';', '3', '~'}},
		{"alt+f10", "alt+f10", []byte{0x1b, '[', '2', '1', ';', '3', '~'}},
		{"alt+f11", "alt+f11", []byte{0x1b, '[', '2', '3', ';', '3', '~'}},
		{"alt+f12", "alt+f12", []byte{0x1b, '[', '2', '4', ';', '3', '~'}},
		{"alt+f13", "alt+f13", []byte{0x1b, '[', '2', '5', ';', '3', '~'}},
		{"alt+f14", "alt+f14", []byte{0x1b, '[', '2', '6', ';', '3', '~'}},
		{"alt+f15", "alt+f15", []byte{0x1b, '[', '2', '8', ';', '3', '~'}},
		{"alt+f16", "alt+f16", []byte{0x1b, '[', '2', '9', ';', '3', '~'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_AltCharacterCombinations(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"alt+a", "alt+a", []byte{0x1b, 'a'}},
		{"alt+z", "alt+z", []byte{0x1b, 'z'}},
		{"alt+x", "alt+x", []byte{0x1b, 'x'}},
		{"alt+enter", "alt+enter", []byte{0x1b, '\r'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_SpecialKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected []byte
	}{
		{"space", " ", []byte{' '}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := NewTerminalMock()
			term.SendKey(tt.key)

			term.mu.Lock()
			defer term.mu.Unlock()

			actual := term.inputBuffer.Bytes()
			assert.Equal(t, tt.expected, actual, "Key %s should generate correct sequence", tt.key)
		})
	}
}

func TestSendKey_UnknownKey(t *testing.T) {
	term := NewTerminalMock()
	unknownKey := "unknown-key-sequence"
	term.SendKey(unknownKey)

	term.mu.Lock()
	defer term.mu.Unlock()

	// Unknown keys should be sent as-is
	actual := term.inputBuffer.Bytes()
	assert.Equal(t, []byte(unknownKey), actual, "Unknown keys should be sent as-is")
}
