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

func (d *cliSessionInputThreadDouble) PermissionResponse(queryID string, response string) error {
	_ = queryID
	_ = response
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
			name:        "interactive short mode uses text session input",
			params:      &CLIParams{Interactive: true, OutputFormat: "short"},
			input:       reader,
			expectedNil: false,
			expected:    &io.TextSessionInput{},
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
			name:        "short mode attaches text session input",
			params:      &CLIParams{Interactive: false, OutputFormat: "short"},
			input:       strings.NewReader("hello\n"),
			expectedNil: false,
			expected:    &io.TextSessionInput{},
		},
		{
			name:        "full mode attaches text session input",
			params:      &CLIParams{Interactive: false, OutputFormat: "full"},
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

func TestBuildCLISessionOutput(t *testing.T) {
	tests := []struct {
		name     string
		params   *CLIParams
		expected any
	}{
		{
			name:     "nil params defaults to text output",
			params:   nil,
			expected: &io.TextSessionOutput{},
		},
		{
			name:     "jsonl format uses jsonl output",
			params:   &CLIParams{OutputFormat: "jsonl"},
			expected: &io.JsonlSessionOutput{},
		},
		{
			name:     "short format uses text output",
			params:   &CLIParams{OutputFormat: "short"},
			expected: &io.TextSessionOutput{},
		},
		{
			name:     "full format uses text output",
			params:   &CLIParams{OutputFormat: "full"},
			expected: &io.TextSessionOutput{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := buildCLISessionOutput(tt.params, &strings.Builder{})
			assert.NotNil(t, actual)
			assert.IsType(t, tt.expected, actual)
		})
	}
}
