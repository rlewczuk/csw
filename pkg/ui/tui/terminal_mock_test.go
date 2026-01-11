package tui

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TerminalMock is a mock terminal that implements io.Reader and io.Writer
// for testing TUI applications with bubbletea.
type TerminalMock struct {
	// inputBuffer holds the input data to be read
	inputBuffer *bytes.Buffer

	// outputChunks stores all output written to the terminal
	outputChunks [][]byte

	// closed indicates if the terminal has been closed
	closed bool

	// mu protects access to fields
	mu sync.Mutex

	// inputCond is a condition variable for waiting on input
	inputCond *sync.Cond

	program *tea.Program
}

// NewTerminalMock creates a new mock terminal.
func NewTerminalMock() *TerminalMock {
	t := &TerminalMock{
		inputBuffer:  &bytes.Buffer{},
		outputChunks: make([][]byte, 0),
	}
	t.inputCond = sync.NewCond(&t.mu)
	return t
}

func (t *TerminalMock) Run(model tea.Model) *tea.Program {
	t.program = tea.NewProgram(
		model,
		tea.WithInput(t),
		tea.WithOutput(t),
		tea.WithoutSignalHandler(),
	)

	go func() {
		t.program.Run()
	}()

	return t.program
}

// Send sends a message to the bubbletea program.
func (t *TerminalMock) Send(msg tea.Msg) {
	if t.program != nil {
		t.program.Send(msg)
	}
}

// Read implements io.Reader. It reads from the input buffer.
func (t *TerminalMock) Read(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Wait for input if buffer is empty
	for t.inputBuffer.Len() == 0 && !t.closed {
		t.inputCond.Wait()
	}

	// Return EOF if closed and no more data
	if t.closed && t.inputBuffer.Len() == 0 {
		return 0, io.EOF
	}

	return t.inputBuffer.Read(p)
}

// Write implements io.Writer. It records output in chunks.
func (t *TerminalMock) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Make a copy of the data
	chunk := make([]byte, len(p))
	copy(chunk, p)
	t.outputChunks = append(t.outputChunks, chunk)

	return len(p), nil
}

// SendInput sends input data to the terminal.
func (t *TerminalMock) SendInput(data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.inputBuffer.Write(data)
	t.inputCond.Broadcast()
}

// SendString sends a string input to the terminal.
func (t *TerminalMock) SendString(s string) {
	t.SendInput([]byte(s))
}

// SendKey sends a special key to the terminal.
// Supports all keys recognized by bubbletea, including control keys,
// arrow keys with modifiers, navigation keys, and function keys.
// Examples: "enter", "ctrl+c", "alt+enter", "esc", "shift+tab", "f1", etc.
func (t *TerminalMock) SendKey(key string) {
	var seq []byte

	switch key {
	// Control keys
	case "ctrl+@":
		seq = []byte{0x00}
	case "ctrl+a":
		seq = []byte{0x01}
	case "ctrl+b":
		seq = []byte{0x02}
	case "ctrl+c":
		seq = []byte{0x03}
	case "ctrl+d":
		seq = []byte{0x04}
	case "ctrl+e":
		seq = []byte{0x05}
	case "ctrl+f":
		seq = []byte{0x06}
	case "ctrl+g":
		seq = []byte{0x07}
	case "ctrl+h":
		seq = []byte{0x08}
	case "ctrl+i", "tab":
		seq = []byte{0x09}
	case "ctrl+j":
		seq = []byte{0x0a}
	case "ctrl+k":
		seq = []byte{0x0b}
	case "ctrl+l":
		seq = []byte{0x0c}
	case "ctrl+m", "enter":
		seq = []byte{0x0d}
	case "ctrl+n":
		seq = []byte{0x0e}
	case "ctrl+o":
		seq = []byte{0x0f}
	case "ctrl+p":
		seq = []byte{0x10}
	case "ctrl+q":
		seq = []byte{0x11}
	case "ctrl+r":
		seq = []byte{0x12}
	case "ctrl+s":
		seq = []byte{0x13}
	case "ctrl+t":
		seq = []byte{0x14}
	case "ctrl+u":
		seq = []byte{0x15}
	case "ctrl+v":
		seq = []byte{0x16}
	case "ctrl+w":
		seq = []byte{0x17}
	case "ctrl+x":
		seq = []byte{0x18}
	case "ctrl+y":
		seq = []byte{0x19}
	case "ctrl+z":
		seq = []byte{0x1a}
	case "ctrl+[", "esc":
		seq = []byte{0x1b}
	case "ctrl+\\":
		seq = []byte{0x1c}
	case "ctrl+]":
		seq = []byte{0x1d}
	case "ctrl+^":
		seq = []byte{0x1e}
	case "ctrl+_":
		seq = []byte{0x1f}
	case "ctrl+?", "backspace":
		seq = []byte{0x7f}

	// Special keys
	case " ":
		seq = []byte{' '}

	// Arrow keys
	case "up":
		seq = []byte{0x1b, '[', 'A'}
	case "down":
		seq = []byte{0x1b, '[', 'B'}
	case "right":
		seq = []byte{0x1b, '[', 'C'}
	case "left":
		seq = []byte{0x1b, '[', 'D'}

	// Shift + Arrow keys
	case "shift+up":
		seq = []byte{0x1b, '[', '1', ';', '2', 'A'}
	case "shift+down":
		seq = []byte{0x1b, '[', '1', ';', '2', 'B'}
	case "shift+right":
		seq = []byte{0x1b, '[', '1', ';', '2', 'C'}
	case "shift+left":
		seq = []byte{0x1b, '[', '1', ';', '2', 'D'}

	// Ctrl + Arrow keys
	case "ctrl+up":
		seq = []byte{0x1b, '[', '1', ';', '5', 'A'}
	case "ctrl+down":
		seq = []byte{0x1b, '[', '1', ';', '5', 'B'}
	case "ctrl+right":
		seq = []byte{0x1b, '[', '1', ';', '5', 'C'}
	case "ctrl+left":
		seq = []byte{0x1b, '[', '1', ';', '5', 'D'}

	// Ctrl + Shift + Arrow keys
	case "ctrl+shift+up":
		seq = []byte{0x1b, '[', '1', ';', '6', 'A'}
	case "ctrl+shift+down":
		seq = []byte{0x1b, '[', '1', ';', '6', 'B'}
	case "ctrl+shift+right":
		seq = []byte{0x1b, '[', '1', ';', '6', 'C'}
	case "ctrl+shift+left":
		seq = []byte{0x1b, '[', '1', ';', '6', 'D'}

	// Alt + Arrow keys
	case "alt+up":
		seq = []byte{0x1b, '[', '1', ';', '3', 'A'}
	case "alt+down":
		seq = []byte{0x1b, '[', '1', ';', '3', 'B'}
	case "alt+right":
		seq = []byte{0x1b, '[', '1', ';', '3', 'C'}
	case "alt+left":
		seq = []byte{0x1b, '[', '1', ';', '3', 'D'}

	// Navigation keys
	case "home":
		seq = []byte{0x1b, '[', 'H'}
	case "end":
		seq = []byte{0x1b, '[', 'F'}
	case "pgup":
		seq = []byte{0x1b, '[', '5', '~'}
	case "pgdown":
		seq = []byte{0x1b, '[', '6', '~'}
	case "insert":
		seq = []byte{0x1b, '[', '2', '~'}
	case "delete":
		seq = []byte{0x1b, '[', '3', '~'}

	// Ctrl + Navigation keys
	case "ctrl+home":
		seq = []byte{0x1b, '[', '1', ';', '5', 'H'}
	case "ctrl+end":
		seq = []byte{0x1b, '[', '1', ';', '5', 'F'}
	case "ctrl+pgup":
		seq = []byte{0x1b, '[', '5', ';', '5', '~'}
	case "ctrl+pgdown":
		seq = []byte{0x1b, '[', '6', ';', '5', '~'}

	// Shift + Navigation keys
	case "shift+home":
		seq = []byte{0x1b, '[', '1', ';', '2', 'H'}
	case "shift+end":
		seq = []byte{0x1b, '[', '1', ';', '2', 'F'}
	case "shift+tab":
		seq = []byte{0x1b, '[', 'Z'}

	// Ctrl + Shift + Navigation keys
	case "ctrl+shift+home":
		seq = []byte{0x1b, '[', '1', ';', '6', 'H'}
	case "ctrl+shift+end":
		seq = []byte{0x1b, '[', '1', ';', '6', 'F'}

	// Alt + Navigation keys
	case "alt+home":
		seq = []byte{0x1b, '[', '1', ';', '3', 'H'}
	case "alt+end":
		seq = []byte{0x1b, '[', '1', ';', '3', 'F'}
	case "alt+pgup":
		seq = []byte{0x1b, '[', '5', ';', '3', '~'}
	case "alt+pgdown":
		seq = []byte{0x1b, '[', '6', ';', '3', '~'}
	case "alt+delete":
		seq = []byte{0x1b, '[', '3', ';', '3', '~'}
	case "alt+insert":
		seq = []byte{0x1b, '[', '3', ';', '2', '~'}

	// Function keys F1-F4 (vt100/xterm)
	case "f1":
		seq = []byte{0x1b, 'O', 'P'}
	case "f2":
		seq = []byte{0x1b, 'O', 'Q'}
	case "f3":
		seq = []byte{0x1b, 'O', 'R'}
	case "f4":
		seq = []byte{0x1b, 'O', 'S'}

	// Function keys F5-F12
	case "f5":
		seq = []byte{0x1b, '[', '1', '5', '~'}
	case "f6":
		seq = []byte{0x1b, '[', '1', '7', '~'}
	case "f7":
		seq = []byte{0x1b, '[', '1', '8', '~'}
	case "f8":
		seq = []byte{0x1b, '[', '1', '9', '~'}
	case "f9":
		seq = []byte{0x1b, '[', '2', '0', '~'}
	case "f10":
		seq = []byte{0x1b, '[', '2', '1', '~'}
	case "f11":
		seq = []byte{0x1b, '[', '2', '3', '~'}
	case "f12":
		seq = []byte{0x1b, '[', '2', '4', '~'}

	// Function keys F13-F20
	case "f13":
		seq = []byte{0x1b, '[', '1', ';', '2', 'P'}
	case "f14":
		seq = []byte{0x1b, '[', '1', ';', '2', 'Q'}
	case "f15":
		seq = []byte{0x1b, '[', '1', ';', '2', 'R'}
	case "f16":
		seq = []byte{0x1b, '[', '1', ';', '2', 'S'}
	case "f17":
		seq = []byte{0x1b, '[', '1', '5', ';', '2', '~'}
	case "f18":
		seq = []byte{0x1b, '[', '1', '7', ';', '2', '~'}
	case "f19":
		seq = []byte{0x1b, '[', '1', '8', ';', '2', '~'}
	case "f20":
		seq = []byte{0x1b, '[', '1', '9', ';', '2', '~'}

	// Alt + Function keys F1-F4
	case "alt+f1":
		seq = []byte{0x1b, '[', '1', ';', '3', 'P'}
	case "alt+f2":
		seq = []byte{0x1b, '[', '1', ';', '3', 'Q'}
	case "alt+f3":
		seq = []byte{0x1b, '[', '1', ';', '3', 'R'}
	case "alt+f4":
		seq = []byte{0x1b, '[', '1', ';', '3', 'S'}

	// Alt + Function keys F5-F16
	case "alt+f5":
		seq = []byte{0x1b, '[', '1', '5', ';', '3', '~'}
	case "alt+f6":
		seq = []byte{0x1b, '[', '1', '7', ';', '3', '~'}
	case "alt+f7":
		seq = []byte{0x1b, '[', '1', '8', ';', '3', '~'}
	case "alt+f8":
		seq = []byte{0x1b, '[', '1', '9', ';', '3', '~'}
	case "alt+f9":
		seq = []byte{0x1b, '[', '2', '0', ';', '3', '~'}
	case "alt+f10":
		seq = []byte{0x1b, '[', '2', '1', ';', '3', '~'}
	case "alt+f11":
		seq = []byte{0x1b, '[', '2', '3', ';', '3', '~'}
	case "alt+f12":
		seq = []byte{0x1b, '[', '2', '4', ';', '3', '~'}
	case "alt+f13":
		seq = []byte{0x1b, '[', '2', '5', ';', '3', '~'}
	case "alt+f14":
		seq = []byte{0x1b, '[', '2', '6', ';', '3', '~'}
	case "alt+f15":
		seq = []byte{0x1b, '[', '2', '8', ';', '3', '~'}
	case "alt+f16":
		seq = []byte{0x1b, '[', '2', '9', ';', '3', '~'}

	// Alt + Enter (commonly used)
	case "alt+enter":
		seq = []byte{0x1b, '\r'}

	default:
		// For alt+<char> patterns, send ESC followed by the character
		if strings.HasPrefix(key, "alt+") && len(key) == 5 {
			char := key[4]
			seq = []byte{0x1b, char}
		} else {
			// Unknown key, just send the string as-is
			seq = []byte(key)
		}
	}

	t.SendInput(seq)
}

// GetOutput returns all output as a single string.
func (t *TerminalMock) GetOutput() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var buf bytes.Buffer
	for _, chunk := range t.outputChunks {
		buf.Write(chunk)
	}
	return buf.String()
}

// GetOutputChunks returns all output chunks.
func (t *TerminalMock) GetOutputChunks() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Return a copy to avoid data races
	chunks := make([][]byte, len(t.outputChunks))
	for i, chunk := range t.outputChunks {
		chunks[i] = make([]byte, len(chunk))
		copy(chunks[i], chunk)
	}
	return chunks
}

// ContainsText searches for specific text in the recorded output.
func (t *TerminalMock) ContainsText(text string) bool {
	output := t.GetOutput()
	return strings.Contains(output, text)
}

// WaitForText waits for specific text to appear in the output with a timeout.
// Returns true if the text was found, false if the timeout was reached.
func (t *TerminalMock) WaitForText(text string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for {
		if t.ContainsText(text) {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}

		// Sleep a bit before checking again
		time.Sleep(1 * time.Millisecond)
	}
}

// WaitForTextWithRetry waits for specific text to appear in the output with a timeout,
// checking every checkInterval.
func (t *TerminalMock) WaitForTextWithRetry(text string, timeout, checkInterval time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for {
		if t.ContainsText(text) {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}

		time.Sleep(checkInterval)
	}
}

// GetOutputSince returns output written since the given time.
// This is useful for checking output generated after a specific action.
func (t *TerminalMock) GetOutputSince(since time.Time) string {
	// For simplicity, this implementation returns all output
	// A more sophisticated implementation could track timestamps per chunk
	return t.GetOutput()
}

// Clear clears all recorded output.
func (t *TerminalMock) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.outputChunks = make([][]byte, 0)
}

// ClearInput clears the input buffer.
func (t *TerminalMock) ClearInput() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.inputBuffer.Reset()
}

// Reset clears both input and output.
func (t *TerminalMock) Reset() {
	t.Clear()
	t.ClearInput()
}

// Close closes the terminal, causing Read() to return EOF.
// This is useful for cleanly shutting down bubbletea programs.
func (t *TerminalMock) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true
	t.inputCond.Broadcast()
}

// String returns a string representation of the terminal state (for debugging).
func (t *TerminalMock) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return fmt.Sprintf("TerminalMock{input: %d bytes, output: %d chunks}",
		t.inputBuffer.Len(), len(t.outputChunks))
}

// Ensure TerminalMock implements io.Reader and io.Writer
var _ io.Reader = (*TerminalMock)(nil)
var _ io.Writer = (*TerminalMock)(nil)
