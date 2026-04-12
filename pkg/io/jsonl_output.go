package io

import (
	"encoding/json"
	"fmt"
	stdio "io"
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/tool"
)

// JsonlSessionOutput writes session output in JSONL-oriented console mode.
type JsonlSessionOutput struct {
	output stdio.Writer

	mu            sync.Mutex
	renderedTools map[string]string
}

// AddUserMessage writes a full user message.
func (o *JsonlSessionOutput) AddUserMessage(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}

	o.writef("\nUser: %s\n", text)
}

// NewJsonlSessionOutput creates a JSONL output adapter for session thread callbacks.
func NewJsonlSessionOutput(output stdio.Writer) *JsonlSessionOutput {
	return &JsonlSessionOutput{
		output:        output,
		renderedTools: make(map[string]string),
	}
}

// ShowMessage shows a status message from the session loop.
func (o *JsonlSessionOutput) ShowMessage(message string, messageType string) {
	prefix := "[INFO]"
	switch strings.TrimSpace(messageType) {
	case "warning":
		prefix = "[WARNING]"
	case "error":
		prefix = "[ERROR]"
	}

	o.writef("%s %s\n", prefix, message)
}

// AddAssistantMessage writes a full assistant message.
func (o *JsonlSessionOutput) AddAssistantMessage(text string, thinking string) {
	if thinking != "" {
		o.writef("\n*%s*\n", thinking)
	}

	if text != "" {
		o.writef("\nAssistant: %s\n", text)
	}
}

// AddToolCall handles tool-call start events.
func (o *JsonlSessionOutput) AddToolCall(call *tool.ToolCall) {
	_ = call
}

// AddToolCallResult handles final tool-call events.
func (o *JsonlSessionOutput) AddToolCallResult(result *tool.ToolResponse) {
	if result == nil || result.Call == nil {
		return
	}

	display := strings.TrimSpace(result.Result.String("jsonl"))
	if display == "" {
		display = strings.TrimSpace(result.Result.String("summary"))
	}
	if display == "" {
		display = strings.TrimSpace(result.Result.String("details"))
	}
	if display == "" {
		display = strings.TrimSpace(result.Call.Function)
	}
	notificationLines := buildJSONLToolNotificationLines(result.Notifications)
	if display == "" {
		display = notificationLines
	} else if notificationLines != "" {
		display = strings.TrimSuffix(display, "\n") + "\n" + strings.TrimSuffix(notificationLines, "\n")
	}
	if display == "" {
		return
	}

	outputLine := display + "\n"

	o.mu.Lock()
	if lastOutput, exists := o.renderedTools[result.Call.ID]; exists && lastOutput == outputLine {
		o.mu.Unlock()
		return
	}
	o.renderedTools[result.Call.ID] = outputLine
	o.mu.Unlock()

	o.write(outputLine)
}

func buildJSONLToolNotificationLines(notifications []tool.ToolNotification) string {
	if len(notifications) == 0 {
		return ""
	}

	builder := strings.Builder{}
	for _, notification := range notifications {
		message := strings.TrimSpace(notification.Message)
		if message == "" {
			continue
		}

		payload := map[string]any{
			"type":         "notification",
			"notification": notification,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		builder.Write(data)
		builder.WriteByte('\n')
	}

	return builder.String()
}

// RunFinished handles end-of-run callback.
func (o *JsonlSessionOutput) RunFinished(err error) {
	_ = err
}

// OnPermissionQuery handles permission query callback.
func (o *JsonlSessionOutput) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	if query == nil {
		return
	}

	payload := map[string]any{
		"type":  "permission_query",
		"query": query,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	o.write(string(data) + "\n")
}

// OnRateLimitError handles rate-limit callback.
func (o *JsonlSessionOutput) OnRateLimitError(retryAfterSeconds int) {
	_ = retryAfterSeconds
}

// ShouldRetryAfterFailure always declines retry prompt in direct jsonl mode.
func (o *JsonlSessionOutput) ShouldRetryAfterFailure(message string) bool {
	_ = message
	return false
}

func (o *JsonlSessionOutput) writef(format string, args ...any) {
	o.write(fmt.Sprintf(format, args...))
}

func (o *JsonlSessionOutput) write(message string) {
	if o == nil || o.output == nil {
		return
	}
	_, _ = fmt.Fprint(o.output, message)
}

var _ core.SessionThreadOutput = (*JsonlSessionOutput)(nil)
