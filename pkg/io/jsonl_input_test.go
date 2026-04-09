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

	prompts, interrupts, responses := thread.snapshot()
	assert.Equal(t, []string{"hello", "world"}, prompts)
	assert.Equal(t, 1, interrupts)
	assert.Empty(t, responses)
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

	prompts, interrupts, responses := thread.snapshot()
	assert.Equal(t, []string{"ok"}, prompts)
	assert.Equal(t, 0, interrupts)
	assert.Empty(t, responses)
}

func TestJsonlSessionInput_StartReadingInputCanBeCalledOnce(t *testing.T) {
	reader := strings.NewReader(`{"action":"interrupt"}` + "\n")
	thread := newInputThreadDouble()
	input := NewJsonlSessionInput(reader, thread)

	input.StartReadingInput()
	input.StartReadingInput()
	thread.waitForEvents(t, 1)

	prompts, interrupts, responses := thread.snapshot()
	assert.Empty(t, prompts)
	assert.Equal(t, 1, interrupts)
	assert.Empty(t, responses)
}

func TestJsonlSessionInput_StartReadingInputRoutesPermissionResponse(t *testing.T) {
	reader := strings.NewReader(strings.Join([]string{
		`{"type":"query_response","query_id":"019d7138-dbf1-7fc6-bdfd-7e8bece29727","response":"deny"}`,
	}, "\n") + "\n")
	thread := newInputThreadDouble()
	thread.acceptPermissionResponses = true
	input := NewJsonlSessionInput(reader, thread)

	input.StartReadingInput()
	thread.waitForEvents(t, 1)

	prompts, interrupts, responses := thread.snapshot()
	assert.Empty(t, prompts)
	assert.Equal(t, 0, interrupts)
	require.Len(t, responses, 1)
	assert.Equal(t, "019d7138-dbf1-7fc6-bdfd-7e8bece29727", responses[0].queryID)
	assert.Equal(t, "deny", responses[0].response)
}

func TestJsonlSessionInput_StartReadingInputNoopForNilDependencies(t *testing.T) {
	require.NotPanics(t, func() {
		var nilInput *JsonlSessionInput
		nilInput.StartReadingInput()

		NewJsonlSessionInput(nil, newInputThreadDouble()).StartReadingInput()
		NewJsonlSessionInput(strings.NewReader(`{"action":"interrupt"}`+"\n"), nil).StartReadingInput()
	})
}
