package io

import (
	"fmt"
	stdio "io"
	"regexp"
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/tool"
)

const textOutputDefaultSlug = "main"
const textOutputGrayPrefixColor = "\x1b[90m"
const textOutputColorReset = "\x1b[0m"

var textOutputSubAgentLinePrefixPattern = regexp.MustCompile(`^\*([a-z0-9]+(?:-[a-z0-9]+)*)\*\s+(.*)$`)

// TextSessionOutput writes session output in human-readable text format.
type TextSessionOutput struct {
	output stdio.Writer

	mu            sync.Mutex
	renderedTools map[string]string
}

// AddUserMessage writes a full user message.
func (o *TextSessionOutput) AddUserMessage(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}

	o.writef("\nUser: %s\n", text)
}

// NewTextSessionOutput creates a text output adapter for session thread callbacks.
func NewTextSessionOutput(output stdio.Writer) *TextSessionOutput {
	return &TextSessionOutput{
		output:        output,
		renderedTools: make(map[string]string),
	}
}

// ShowMessage shows a status message from the session loop.
func (o *TextSessionOutput) ShowMessage(message string, messageType string) {
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
func (o *TextSessionOutput) AddAssistantMessage(text string, thinking string) {
	if thinking != "" {
		o.writef("\n*%s*\n", thinking)
	}

	if text != "" {
		o.writef("\nAssistant: %s\n", text)
	}
}

// AddToolCall handles tool-call start events.
func (o *TextSessionOutput) AddToolCall(call *tool.ToolCall) {
	_ = call
}

// AddToolCallResult handles final tool-call events.
func (o *TextSessionOutput) AddToolCallResult(result *tool.ToolResponse) {
	if result == nil || result.Call == nil {
		return
	}

	outputLine, ok := buildTextToolOutputLine(result)
	if !ok {
		return
	}

	o.mu.Lock()
	if lastOutput, exists := o.renderedTools[result.Call.ID]; exists && lastOutput == outputLine {
		o.mu.Unlock()
		return
	}
	o.renderedTools[result.Call.ID] = outputLine
	o.mu.Unlock()

	o.write(outputLine)
}

// RunFinished handles end-of-run callback.
func (o *TextSessionOutput) RunFinished(err error) {
	_ = err
}

// OnPermissionQuery handles permission query callback.
func (o *TextSessionOutput) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	_ = query
}

// OnRateLimitError handles rate-limit callback.
func (o *TextSessionOutput) OnRateLimitError(retryAfterSeconds int) {
	_ = retryAfterSeconds
}

// ShouldRetryAfterFailure always declines retry prompt in direct text mode.
func (o *TextSessionOutput) ShouldRetryAfterFailure(message string) bool {
	_ = message
	return false
}

func buildTextToolOutputLine(result *tool.ToolResponse) (string, bool) {
	if result == nil || result.Call == nil {
		return "", false
	}

	display := strings.TrimSpace(result.Result.String("summary"))
	if display == "" {
		display = strings.TrimSpace(result.Result.String("details"))
	}
	if display == "" {
		display = strings.TrimSpace(result.Call.Function)
	}
	if display == "" {
		return "", false
	}

	icon := "✅"
	if result.Error != nil {
		icon = "❌"
	}

	return fmt.Sprintf("%s %s\n", icon, display), true
}

func (o *TextSessionOutput) writef(format string, args ...any) {
	o.write(fmt.Sprintf(format, args...))
}

func (o *TextSessionOutput) write(message string) {
	if o == nil || o.output == nil {
		return
	}
	_, _ = fmt.Fprint(o.output, addTextOutputSlugPrefix(textOutputDefaultSlug, message))
}

func addTextOutputSlugPrefix(slug string, message string) string {
	lines := strings.Split(message, "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		lineSlug := normalizeTextOutputSlug(slug)
		lineText := line
		if matches := textOutputSubAgentLinePrefixPattern.FindStringSubmatch(line); len(matches) == 3 {
			lineSlug = normalizeTextOutputSlug(matches[1])
			lineText = matches[2]
		}

		prefix := fmt.Sprintf("%s[%s]%s ", textOutputGrayPrefixColor, lineSlug, textOutputColorReset)
		lines[i] = prefix + lineText
	}

	return strings.Join(lines, "\n")
}

func normalizeTextOutputSlug(slug string) string {
	trimmed := strings.TrimSpace(slug)
	if trimmed == "" {
		return textOutputDefaultSlug
	}

	return trimmed
}

var _ core.SessionThreadOutput = (*TextSessionOutput)(nil)
