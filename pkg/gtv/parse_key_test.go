package gtv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseKey_SingleCharacters tests parsing single character keys.
func TestParseKey_SingleCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKey  rune
		wantMods EventModifiers
	}{
		{"lowercase letter", "a", 'a', 0},
		{"uppercase letter", "A", 'A', ModShift},
		{"digit", "1", '1', 0},
		{"special char", "!", '!', 0},
		{"space", "Space", ' ', 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseKey(tt.input)
			require.NoError(t, err)
			assert.Equal(t, InputEventKey, event.Type)
			assert.Equal(t, tt.wantKey, event.Key)
			assert.Equal(t, tt.wantMods, event.Modifiers)
		})
	}
}

// TestParseKey_SpecialKeys tests parsing special keys.
func TestParseKey_SpecialKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKey  rune
		wantMods EventModifiers
	}{
		{"Enter", "Enter", '\r', 0},
		{"Return", "Return", '\r', 0},
		{"Tab", "Tab", '\t', 0},
		{"Esc", "Esc", 0x1B, 0},
		{"Escape", "Escape", 0x1B, 0},
		{"Backspace", "Backspace", 0x7F, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseKey(tt.input)
			require.NoError(t, err)
			assert.Equal(t, InputEventKey, event.Type)
			assert.Equal(t, tt.wantKey, event.Key)
			assert.Equal(t, tt.wantMods, event.Modifiers)
		})
	}
}

// TestParseKey_NavigationKeys tests parsing navigation keys.
func TestParseKey_NavigationKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKey  rune
		wantMods EventModifiers
	}{
		{"Up arrow", "Up", 'A', ModFn},
		{"Down arrow", "Down", 'B', ModFn},
		{"Left arrow", "Left", 'D', ModFn},
		{"Right arrow", "Right", 'C', ModFn},
		{"Home", "Home", 'H', ModFn},
		{"End", "End", 'F', ModFn},
		{"PageUp", "PageUp", 'G', ModFn},
		{"PageDown", "PageDown", 'N', ModFn},
		{"Insert", "Insert", 'I', ModFn},
		{"Delete", "Delete", 'E', ModFn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseKey(tt.input)
			require.NoError(t, err)
			assert.Equal(t, InputEventKey, event.Type)
			assert.Equal(t, tt.wantKey, event.Key)
			assert.Equal(t, tt.wantMods, event.Modifiers)
		})
	}
}

// TestParseKey_FunctionKeys tests parsing function keys F1-F12.
func TestParseKey_FunctionKeys(t *testing.T) {
	fnKeys := []rune{'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', '['}

	for i := 1; i <= 12; i++ {
		t.Run("F"+string(rune('0'+i/10))+string(rune('0'+i%10)), func(t *testing.T) {
			input := "F"
			if i < 10 {
				input += string(rune('0' + i))
			} else {
				input += string(rune('0'+i/10)) + string(rune('0'+i%10))
			}

			event, err := ParseKey(input)
			require.NoError(t, err)
			assert.Equal(t, InputEventKey, event.Type)
			assert.Equal(t, fnKeys[i-1], event.Key)
			assert.Equal(t, ModFn, event.Modifiers)
		})
	}
}

// TestParseKey_ModifiedKeys tests parsing keys with modifiers.
func TestParseKey_ModifiedKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKey  rune
		wantMods EventModifiers
	}{
		{"Ctrl+C", "Ctrl+C", 'c', ModCtrl},
		{"Ctrl+A", "Ctrl+A", 'a', ModCtrl},
		{"Alt+Enter", "Alt+Enter", '\r', ModAlt},
		{"Shift+F1", "Shift+F1", 'P', ModShift | ModFn},
		{"Ctrl+Alt+Delete", "Ctrl+Alt+Delete", 'E', ModCtrl | ModAlt | ModFn},
		{"Meta+A", "Meta+A", 'A', ModMeta | ModShift},             // Uppercase A gets Shift modifier
		{"Ctrl+Shift+A", "Ctrl+Shift+A", 'a', ModCtrl | ModShift}, // Ctrl normalizes to lowercase
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseKey(tt.input)
			require.NoError(t, err)
			assert.Equal(t, InputEventKey, event.Type)
			assert.Equal(t, tt.wantKey, event.Key, "key mismatch")
			assert.Equal(t, tt.wantMods, event.Modifiers, "modifiers mismatch")
		})
	}
}

// TestParseKey_CaseInsensitive tests that key names are case-insensitive.
func TestParseKey_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []string
		wantKey  rune
		wantMods EventModifiers
	}{
		{"Enter variations", []string{"Enter", "enter", "ENTER"}, '\r', 0},
		{"Tab variations", []string{"Tab", "tab", "TAB"}, '\t', 0},
		{"F1 variations", []string{"F1", "f1"}, 'P', ModFn},
		{"Up variations", []string{"Up", "up", "UP"}, 'A', ModFn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, input := range tt.inputs {
				event, err := ParseKey(input)
				require.NoError(t, err, "input: %s", input)
				assert.Equal(t, tt.wantKey, event.Key, "input: %s", input)
				assert.Equal(t, tt.wantMods, event.Modifiers, "input: %s", input)
			}
		})
	}
}

// TestParseKey_Errors tests error cases.
func TestParseKey_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"unknown modifier", "Unknown+A"},
		{"invalid function key", "F13"},
		{"unknown key name", "UnknownKey"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseKey(tt.input)
			assert.Error(t, err)
		})
	}
}

// TestParseKey_CtrlLetterNormalization tests that Ctrl+letter combinations
// use lowercase letters even when specified as uppercase.
func TestParseKey_CtrlLetterNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKey  rune
		wantMods EventModifiers
	}{
		{"Ctrl+c lowercase", "Ctrl+c", 'c', ModCtrl},
		{"Ctrl+C uppercase", "Ctrl+C", 'c', ModCtrl},
		{"Ctrl+a lowercase", "Ctrl+a", 'a', ModCtrl},
		{"Ctrl+A uppercase", "Ctrl+A", 'a', ModCtrl},
		{"Ctrl+Z uppercase", "Ctrl+Z", 'z', ModCtrl},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseKey(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, event.Key)
			assert.Equal(t, tt.wantMods, event.Modifiers)
		})
	}
}
