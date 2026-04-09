package main

import (
	stdio "io"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/io"
	"github.com/stretchr/testify/assert"
)

type cliSessionInputThreadDouble struct{}

func (d *cliSessionInputThreadDouble) UserPrompt(input string) error {
	_ = input
	return nil
}

func (d *cliSessionInputThreadDouble) Interrupt() error {
	return nil
}

var _ core.SessionThreadInput = (*cliSessionInputThreadDouble)(nil)

func TestBuildCLIStdinSessionInput(t *testing.T) {
	thread := &cliSessionInputThreadDouble{}
	reader := strings.NewReader("hello\n")

	tests := []struct {
		name        string
		params      *CLIParams
		input       stdio.Reader
		expectedNil bool
		expected    any
	}{
		{
			name:        "nil params returns nil",
			params:      nil,
			input:       reader,
			expectedNil: true,
		},
		{
			name:        "interactive mode returns nil",
			params:      &CLIParams{Interactive: true, OutputFormat: "short"},
			input:       reader,
			expectedNil: true,
		},
		{
			name:        "nil input returns nil",
			params:      &CLIParams{Interactive: false, OutputFormat: "short"},
			input:       nil,
			expectedNil: true,
		},
		{
			name:        "jsonl mode uses jsonl session input",
			params:      &CLIParams{Interactive: false, OutputFormat: "jsonl"},
			input:       strings.NewReader("{}\n"),
			expectedNil: false,
			expected:    &io.JsonlSessionInput{},
		},
		{
			name:        "default mode uses text session input",
			params:      &CLIParams{Interactive: false, OutputFormat: "short"},
			input:       strings.NewReader("hello\n"),
			expectedNil: false,
			expected:    &io.TextSessionInput{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := buildCLIStdinSessionInput(tt.params, thread, tt.input)
			if tt.expectedNil {
				assert.Nil(t, actual)
				return
			}

			assert.NotNil(t, actual)
			assert.IsType(t, tt.expected, actual)
		})
	}
}
