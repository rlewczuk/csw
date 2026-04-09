package io

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type inputThreadDouble struct {
	mu         sync.Mutex
	prompts    []string
	interrupts int
	events     chan struct{}
}

func newInputThreadDouble() *inputThreadDouble {
	return &inputThreadDouble{events: make(chan struct{}, 32)}
}

func (d *inputThreadDouble) UserPrompt(input string) error {
	d.mu.Lock()
	d.prompts = append(d.prompts, input)
	d.mu.Unlock()
	d.events <- struct{}{}
	return nil
}

func (d *inputThreadDouble) Interrupt() error {
	d.mu.Lock()
	d.interrupts++
	d.mu.Unlock()
	d.events <- struct{}{}
	return nil
}

func (d *inputThreadDouble) waitForEvents(t *testing.T, expected int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for i := 0; i < expected; i++ {
		select {
		case <-d.events:
		case <-deadline:
			t.Fatalf("timed out waiting for event %d/%d", i+1, expected)
		}
	}
}

func (d *inputThreadDouble) snapshot() ([]string, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	copyPrompts := append([]string(nil), d.prompts...)
	return copyPrompts, d.interrupts
}

func TestTextSessionInput_StartReadingInputRoutesLines(t *testing.T) {
	reader := strings.NewReader("  hello  \n   \n\tworld\t\n")
	thread := newInputThreadDouble()
	input := NewTextSessionInput(reader, thread)

	input.StartReadingInput()
	thread.waitForEvents(t, 3)

	prompts, interrupts := thread.snapshot()
	assert.Equal(t, []string{"hello", "world"}, prompts)
	assert.Equal(t, 1, interrupts)
}

func TestTextSessionInput_StartReadingInputCanBeCalledOnce(t *testing.T) {
	reader := strings.NewReader("hello\n")
	thread := newInputThreadDouble()
	input := NewTextSessionInput(reader, thread)

	input.StartReadingInput()
	input.StartReadingInput()
	thread.waitForEvents(t, 1)

	prompts, interrupts := thread.snapshot()
	assert.Equal(t, []string{"hello"}, prompts)
	assert.Equal(t, 0, interrupts)
}

func TestTextSessionInput_StartReadingInputNoopForNilDependencies(t *testing.T) {
	require.NotPanics(t, func() {
		var nilInput *TextSessionInput
		nilInput.StartReadingInput()

		NewTextSessionInput(nil, newInputThreadDouble()).StartReadingInput()
		NewTextSessionInput(strings.NewReader("hello\n"), nil).StartReadingInput()
	})
}
