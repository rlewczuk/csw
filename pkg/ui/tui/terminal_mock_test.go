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

// Terminal is a mock terminal that implements io.Reader and io.Writer
// for testing TUI applications with bubbletea.
type Terminal struct {
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

// NewTerminal creates a new mock terminal.
func NewTerminal() *Terminal {
	t := &Terminal{
		inputBuffer:  &bytes.Buffer{},
		outputChunks: make([][]byte, 0),
	}
	t.inputCond = sync.NewCond(&t.mu)
	return t
}

func (t *Terminal) Run(model tea.Model) *tea.Program {
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
func (t *Terminal) Send(msg tea.Msg) {
	if t.program != nil {
		t.program.Send(msg)
	}
}

// Read implements io.Reader. It reads from the input buffer.
func (t *Terminal) Read(p []byte) (n int, err error) {
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
func (t *Terminal) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Make a copy of the data
	chunk := make([]byte, len(p))
	copy(chunk, p)
	t.outputChunks = append(t.outputChunks, chunk)

	return len(p), nil
}

// SendInput sends input data to the terminal.
func (t *Terminal) SendInput(data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.inputBuffer.Write(data)
	t.inputCond.Broadcast()
}

// SendString sends a string input to the terminal.
func (t *Terminal) SendString(s string) {
	t.SendInput([]byte(s))
}

// SendKey sends a special key to the terminal.
// Supported keys: "enter", "ctrl+c", "alt+enter", "esc", etc.
func (t *Terminal) SendKey(key string) {
	var seq []byte

	switch key {
	case "enter":
		seq = []byte{'\r'}
	case "ctrl+c":
		seq = []byte{0x03} // ETX
	case "ctrl+d":
		seq = []byte{0x04} // EOT
	case "esc":
		seq = []byte{0x1b}
	case "alt+enter":
		// Alt+Enter is typically ESC followed by Enter
		seq = []byte{0x1b, '\r'}
	case "backspace":
		seq = []byte{0x7f}
	case "tab":
		seq = []byte{'\t'}
	case "up":
		seq = []byte{0x1b, '[', 'A'}
	case "down":
		seq = []byte{0x1b, '[', 'B'}
	case "right":
		seq = []byte{0x1b, '[', 'C'}
	case "left":
		seq = []byte{0x1b, '[', 'D'}
	default:
		// Unknown key, just send the string
		seq = []byte(key)
	}

	t.SendInput(seq)
}

// GetOutput returns all output as a single string.
func (t *Terminal) GetOutput() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var buf bytes.Buffer
	for _, chunk := range t.outputChunks {
		buf.Write(chunk)
	}
	return buf.String()
}

// GetOutputChunks returns all output chunks.
func (t *Terminal) GetOutputChunks() [][]byte {
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
func (t *Terminal) ContainsText(text string) bool {
	output := t.GetOutput()
	return strings.Contains(output, text)
}

// WaitForText waits for specific text to appear in the output with a timeout.
// Returns true if the text was found, false if the timeout was reached.
func (t *Terminal) WaitForText(text string, timeout time.Duration) bool {
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
func (t *Terminal) WaitForTextWithRetry(text string, timeout, checkInterval time.Duration) bool {
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
func (t *Terminal) GetOutputSince(since time.Time) string {
	// For simplicity, this implementation returns all output
	// A more sophisticated implementation could track timestamps per chunk
	return t.GetOutput()
}

// Clear clears all recorded output.
func (t *Terminal) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.outputChunks = make([][]byte, 0)
}

// ClearInput clears the input buffer.
func (t *Terminal) ClearInput() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.inputBuffer.Reset()
}

// Reset clears both input and output.
func (t *Terminal) Reset() {
	t.Clear()
	t.ClearInput()
}

// Close closes the terminal, causing Read() to return EOF.
// This is useful for cleanly shutting down bubbletea programs.
func (t *Terminal) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true
	t.inputCond.Broadcast()
}

// String returns a string representation of the terminal state (for debugging).
func (t *Terminal) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return fmt.Sprintf("Terminal{input: %d bytes, output: %d chunks}",
		t.inputBuffer.Len(), len(t.outputChunks))
}

// Ensure Terminal implements io.Reader and io.Writer
var _ io.Reader = (*Terminal)(nil)
var _ io.Writer = (*Terminal)(nil)
