package io

import (
	"bufio"
	"encoding/json"
	stdio "io"
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/core"
)

// JsonlSessionInput reads JSONL command lines and forwards them to a session thread input.
type JsonlSessionInput struct {
	input  stdio.Reader
	thread core.SessionThreadInput

	startOnce sync.Once
}

type jsonlSessionInputLine struct {
	Type     string `json:"type"`
	Action   string `json:"action"`
	Input    string `json:"input"`
	QueryID  string `json:"query_id"`
	Response string `json:"response"`
}

// NewJsonlSessionInput creates a JSONL input adapter for session thread input callbacks.
func NewJsonlSessionInput(input stdio.Reader, thread core.SessionThreadInput) *JsonlSessionInput {
	return &JsonlSessionInput{input: input, thread: thread}
}

// StartReadingInput starts a background loop that reads JSONL action commands from input.
func (i *JsonlSessionInput) StartReadingInput() {
	if i == nil || i.input == nil || i.thread == nil {
		return
	}

	i.startOnce.Do(func() {
		go i.readLoop()
	})
}

func (i *JsonlSessionInput) readLoop() {
	scanner := bufio.NewScanner(i.input)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var payload jsonlSessionInputLine
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			continue
		}

		lineType := strings.TrimSpace(payload.Type)
		if lineType == "query_response" {
			_ = i.thread.PermissionResponse(strings.TrimSpace(payload.QueryID), strings.TrimSpace(payload.Response))
			continue
		}

		switch strings.TrimSpace(payload.Action) {
		case "interrupt":
			_ = i.thread.Interrupt()
		case "prompt":
			_ = i.thread.UserPrompt(strings.TrimSpace(payload.Input))
		}
	}
}
