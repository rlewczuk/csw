package main

import (
	"errors"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmitSessionSummary validates summary behavior on success and error paths.
func TestEmitSessionSummary(t *testing.T) {
	tests := []struct {
		name                   string
		saveErr                error
		sessionRunErr          error
		expectErr              string
		expectInfoMessageCount int
		expectWarnMessageCount int
		expectWarningSubstring string
	}{
		{
			name:                   "session error still prints summary",
			sessionRunErr:          errors.New("agent failed"),
			expectErr:              "agent failed",
			expectInfoMessageCount: 1,
		},
		{
			name:                   "session error with summary save failure returns session error and warns",
			saveErr:                errors.New("disk full"),
			sessionRunErr:          errors.New("agent failed"),
			expectErr:              "agent failed",
			expectInfoMessageCount: 1,
			expectWarnMessageCount: 1,
			expectWarningSubstring: "Failed to save session summary: disk full",
		},
		{
			name:                   "no session error and summary save failure returns wrapped error",
			saveErr:                errors.New("disk full"),
			expectErr:              "emitSessionSummary() [cli.go]: failed to save session summary: disk full",
			expectInfoMessageCount: 0,
			expectWarnMessageCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			originalSaveSummary := saveSessionSummaryMarkdownFunc
			t.Cleanup(func() {
				saveSessionSummaryMarkdownFunc = originalSaveSummary
			})

			saveSessionSummaryMarkdownFunc = func(logsDir string, session *core.SweSession, sessionInfo string) error {
				return tc.saveErr
			}

			view := &summaryCaptureAppView{}
			err := emitSessionSummary(
				time.Now().Add(-5*time.Second),
				nil,
				system.BuildSystemResult{LogsDir: "any"},
				view,
				tc.sessionRunErr,
			)

			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr)
			}

			assert.Len(t, view.infoMessages, tc.expectInfoMessageCount)
			assert.Len(t, view.warnMessages, tc.expectWarnMessageCount)
			if tc.expectInfoMessageCount > 0 {
				assert.Contains(t, view.infoMessages[0], "Session completed in")
			}
			if tc.expectWarningSubstring != "" {
				assert.Contains(t, view.warnMessages[0], tc.expectWarningSubstring)
			}
		})
	}
}

type summaryCaptureAppView struct {
	infoMessages []string
	warnMessages []string
}

func (v *summaryCaptureAppView) ShowChat(presenter ui.IChatPresenter) ui.IChatView {
	_ = presenter
	return nil
}

func (v *summaryCaptureAppView) ShowSettings() {
}

func (v *summaryCaptureAppView) ShowMessage(message string, messageType ui.MessageType) {
	switch messageType {
	case ui.MessageTypeWarning:
		v.warnMessages = append(v.warnMessages, message)
	case ui.MessageTypeInfo:
		v.infoMessages = append(v.infoMessages, message)
	}
}
