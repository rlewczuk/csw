package cli

import (
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/rlewczuk/csw/pkg/ui"
)

// NewAppView creates a CLI app-level view.
func NewAppView(output io.Writer) *CliAppView {
	return NewCliAppView(output)
}

// CliAppView implements ui.IAppView for plain CLI mode.
type CliAppView struct {
	output        io.Writer
	mu            sync.Mutex
	sessionLogger *slog.Logger
	pendingLogs   []diagnosticLogMessage
}

type diagnosticLogMessage struct {
	message     string
	messageType ui.MessageType
}

// NewCliAppView creates a new CLI app view writing to output.
func NewCliAppView(output io.Writer) *CliAppView {
	return &CliAppView{output: output}
}

// SetSessionLogger configures session logger used for diagnostic message entries.
func (v *CliAppView) SetSessionLogger(logger *slog.Logger) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.sessionLogger = logger
	if logger == nil {
		return
	}

	for _, pending := range v.pendingLogs {
		v.logDiagnosticMessage(logger, pending.message, pending.messageType)
	}
	v.pendingLogs = nil
}

// ShowChat is a no-op for now in CLI app view.
func (v *CliAppView) ShowChat(presenter ui.IChatPresenter) ui.IChatView {
	_ = presenter
	return nil
}

// ShowSettings is a no-op for now in CLI app view.
func (v *CliAppView) ShowSettings() {
}

// ShowMessage prints a prefixed message to stdout.
func (v *CliAppView) ShowMessage(message string, messageType ui.MessageType) {
	prefix := "[INFO]"

	switch messageType {
	case ui.MessageTypeWarning:
		prefix = "[WARNING]"
	case ui.MessageTypeError:
		prefix = "[ERROR]"
	}

	_, _ = fmt.Fprintf(v.output, "%s %s\n", prefix, message)

	v.mu.Lock()
	defer v.mu.Unlock()

	if v.sessionLogger == nil {
		v.pendingLogs = append(v.pendingLogs, diagnosticLogMessage{message: message, messageType: messageType})
		return
	}

	v.logDiagnosticMessage(v.sessionLogger, message, messageType)
}

func (v *CliAppView) logDiagnosticMessage(logger *slog.Logger, message string, messageType ui.MessageType) {
	if logger == nil {
		return
	}

	attrs := []any{
		"diagnostic", true,
		"message", message,
		"message_type", string(messageType),
	}

	switch messageType {
	case ui.MessageTypeWarning:
		logger.Warn("diagnostic_message", attrs...)
	case ui.MessageTypeError:
		logger.Error("diagnostic_message", attrs...)
	default:
		logger.Info("diagnostic_message", attrs...)
	}
}

// AskRetry asks user whether retry should continue after retries are exhausted.
// In CLI mode this is always false to exit after configured attempts.
func (v *CliAppView) AskRetry(message string) bool {
	v.ShowMessage(message, ui.MessageTypeError)
	return false
}

var _ ui.IAppView = (*CliAppView)(nil)
var _ ui.IRetryPromptView = (*CliAppView)(nil)
