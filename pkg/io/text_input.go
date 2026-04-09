package io

import (
	"bufio"
	"errors"
	stdio "io"
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/core"
)

// TextSessionInput reads plain-text lines and forwards them to a session thread input.
type TextSessionInput struct {
	input  stdio.Reader
	thread core.SessionThreadInput

	startOnce sync.Once
}

// NewTextSessionInput creates a text input adapter for session thread input callbacks.
func NewTextSessionInput(input stdio.Reader, thread core.SessionThreadInput) *TextSessionInput {
	return &TextSessionInput{input: input, thread: thread}
}

// StartReadingInput starts a background loop that reads plain text lines from input.
func (i *TextSessionInput) StartReadingInput() {
	if i == nil || i.input == nil || i.thread == nil {
		return
	}

	i.startOnce.Do(func() {
		go i.readLoop()
	})
}

func (i *TextSessionInput) readLoop() {
	scanner := bufio.NewScanner(i.input)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" {
			_ = i.thread.Interrupt()
			continue
		}

		if err := i.thread.PermissionResponse("", trimmed); err == nil {
			continue
		} else if !errors.Is(err, core.ErrNoPendingPermissionQuery) {
			continue
		}

		_ = i.thread.UserPrompt(trimmed)
	}
}
