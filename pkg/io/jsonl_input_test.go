package io

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonlSessionInput_StartReadingInputRoutesActions(t *testing.T) {
	reader := strings.NewReader(strings.Join([]string{
		`{"action":"prompt","input":"  hello  "}`,
		`{"action":"interrupt"}`,
		`{"action":"prompt","input":"\tworld\t"}`,
	}, "\n") + "\n")
	thread := newInputThreadDouble()
	input := NewJsonlSessionInput(reader, thread)

	input.StartReadingInput()
	thread.waitForEvents(t, 3)

	prompts, interrupts := thread.snapshot()
	assert.Equal(t, []string{"hello", "world"}, prompts)
	assert.Equal(t, 1, interrupts)
}

func TestJsonlSessionInput_StartReadingInputRoutesMultipleInterrupts(t *testing.T) {
	reader := strings.NewReader(strings.Join([]string{
		`{"action":"interrupt"}`,
		`{"action":"interrupt"}`,
		`{"action":"prompt","input":"resume"}`,
	}, "\n") + "\n")
	thread := newInputThreadDouble()
	input := NewJsonlSessionInput(reader, thread)

	input.StartReadingInput()
	thread.waitForEvents(t, 3)

	prompts, interrupts := thread.snapshot()
	assert.Equal(t, []string{"resume"}, prompts)
	assert.Equal(t, 2, interrupts)
}

func TestJsonlSessionInput_StartReadingInputSkipsInvalidEntries(t *testing.T) {
	reader := strings.NewReader(strings.Join([]string{
		"",
		"not-json",
		`{"action":"unknown","input":"x"}`,
		`{"action":"prompt","input":"ok"}`,
	}, "\n") + "\n")
	thread := newInputThreadDouble()
	input := NewJsonlSessionInput(reader, thread)

	input.StartReadingInput()
	thread.waitForEvents(t, 1)

	prompts, interrupts := thread.snapshot()
	assert.Equal(t, []string{"ok"}, prompts)
	assert.Equal(t, 0, interrupts)
}

func TestJsonlSessionInput_StartReadingInputCanBeCalledOnce(t *testing.T) {
	reader := strings.NewReader(`{"action":"interrupt"}` + "\n")
	thread := newInputThreadDouble()
	input := NewJsonlSessionInput(reader, thread)

	input.StartReadingInput()
	input.StartReadingInput()
	thread.waitForEvents(t, 1)

	prompts, interrupts := thread.snapshot()
	assert.Empty(t, prompts)
	assert.Equal(t, 1, interrupts)
}

func TestJsonlSessionInput_StartReadingInputNoopForNilDependencies(t *testing.T) {
	require.NotPanics(t, func() {
		var nilInput *JsonlSessionInput
		nilInput.StartReadingInput()

		NewJsonlSessionInput(nil, newInputThreadDouble()).StartReadingInput()
		NewJsonlSessionInput(strings.NewReader(`{"action":"interrupt"}`+"\n"), nil).StartReadingInput()
	})
}
