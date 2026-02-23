package cli

import (
	"fmt"
	"io"

	"github.com/rlewczuk/csw/pkg/ui"
)

// NewAppView creates a CLI app-level view.
func NewAppView(output io.Writer) *CliAppView {
	return NewCliAppView(output)
}

// CliAppView implements ui.IAppView for plain CLI mode.
type CliAppView struct {
	output io.Writer
}

// NewCliAppView creates a new CLI app view writing to output.
func NewCliAppView(output io.Writer) *CliAppView {
	return &CliAppView{output: output}
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
}

// AskRetry asks user whether retry should continue after retries are exhausted.
// In CLI mode this is always false to exit after configured attempts.
func (v *CliAppView) AskRetry(message string) bool {
	v.ShowMessage(message, ui.MessageTypeError)
	return false
}

var _ ui.IAppView = (*CliAppView)(nil)
var _ ui.IRetryPromptView = (*CliAppView)(nil)
