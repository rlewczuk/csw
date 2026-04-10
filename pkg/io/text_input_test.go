package io

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type inputThreadDouble struct {
	mu         sync.Mutex
	prompts    []string
	interrupts int
	acceptPermissionResponses bool
	permissionResponses []permissionResponseRecord
	events     chan struct{}
}

type permissionResponseRecord struct {
	queryID  string
	response string
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

func (d *inputThreadDouble) PermissionResponse(queryID string, response string) error {
	d.mu.Lock()
	accept := d.acceptPermissionResponses
	d.mu.Unlock()
	if !accept {
		return fmt.Errorf("inputThreadDouble.PermissionResponse() [text_input_test.go]: %w", core.ErrNoPendingPermissionQuery)
	}

	d.mu.Lock()
	d.permissionResponses = append(d.permissionResponses, permissionResponseRecord{queryID: queryID, response: response})
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

func (d *inputThreadDouble) snapshot() ([]string, int, []permissionResponseRecord) {
	d.mu.Lock()
	defer d.mu.Unlock()
	copyPrompts := append([]string(nil), d.prompts...)
	copyResponses := append([]permissionResponseRecord(nil), d.permissionResponses...)
	return copyPrompts, d.interrupts, copyResponses
}

func TestTextSessionInput_StartReadingInputRoutesLines(t *testing.T) {
	reader := strings.NewReader("  hello  \n   \n\tworld\t\n")
	thread := newInputThreadDouble()
	input := NewTextSessionInput(reader, thread)

	input.StartReadingInput()
	thread.waitForEvents(t, 3)

	prompts, interrupts, responses := thread.snapshot()
	assert.Equal(t, []string{"hello", "world"}, prompts)
	assert.Equal(t, 1, interrupts)
	assert.Empty(t, responses)
}

func TestTextSessionInput_StartReadingInputRoutesMultipleInterrupts(t *testing.T) {
	reader := strings.NewReader("\n\nresume\n")
	thread := newInputThreadDouble()
	input := NewTextSessionInput(reader, thread)

	input.StartReadingInput()
	thread.waitForEvents(t, 3)

	prompts, interrupts, responses := thread.snapshot()
	assert.Equal(t, []string{"resume"}, prompts)
	assert.Equal(t, 2, interrupts)
	assert.Empty(t, responses)
}

func TestTextSessionInput_StartReadingInputCanBeCalledOnce(t *testing.T) {
	reader := strings.NewReader("hello\n")
	thread := newInputThreadDouble()
	input := NewTextSessionInput(reader, thread)

	input.StartReadingInput()
	input.StartReadingInput()
	thread.waitForEvents(t, 1)

	prompts, interrupts, responses := thread.snapshot()
	assert.Equal(t, []string{"hello"}, prompts)
	assert.Equal(t, 0, interrupts)
	assert.Empty(t, responses)
}

func TestTextSessionInput_StartReadingInputRoutesPermissionResponse(t *testing.T) {
	reader := strings.NewReader("Allow\n")
	thread := newInputThreadDouble()
	thread.acceptPermissionResponses = true
	input := NewTextSessionInput(reader, thread)

	input.StartReadingInput()
	thread.waitForEvents(t, 1)

	prompts, interrupts, responses := thread.snapshot()
	assert.Empty(t, prompts)
	assert.Equal(t, 0, interrupts)
	require.Len(t, responses, 1)
	assert.Equal(t, "", responses[0].queryID)
	assert.Equal(t, "Allow", responses[0].response)
}

func TestTextSessionInput_StartReadingInputNoopForNilDependencies(t *testing.T) {
	require.NotPanics(t, func() {
		var nilInput *TextSessionInput
		nilInput.StartReadingInput()

		NewTextSessionInput(nil, newInputThreadDouble()).StartReadingInput()
		NewTextSessionInput(strings.NewReader("hello\n"), nil).StartReadingInput()
	})
}
