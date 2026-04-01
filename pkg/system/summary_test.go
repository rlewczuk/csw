package system

import (
	"errors"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
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
			expectErr:              "EmitSessionSummary() [cli.go]: failed to save session summary: disk full",
			expectInfoMessageCount: 0,
			expectWarnMessageCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			originalSaveSummary := SaveSessionSummaryMarkdownFunc
			originalSaveSummaryJSON := SaveSessionSummaryJSONFunc
			t.Cleanup(func() {
				SaveSessionSummaryMarkdownFunc = originalSaveSummary
				SaveSessionSummaryJSONFunc = originalSaveSummaryJSON
			})

			SaveSessionSummaryMarkdownFunc = func(logsDir string, session *core.SweSession, sessionInfo string) error {
				return tc.saveErr
			}
			SaveSessionSummaryJSONFunc = func(logsDir string, session *core.SweSession, buildResult BuildSystemResult, startTime time.Time, endTime time.Time, baseCommitID string, headCommitID string) error {
				return nil
			}

			view := &summaryCaptureAppView{}
			err := EmitSessionSummary(
				time.Now().Add(-5*time.Second),
				time.Now(),
				nil,
				BuildSystemResult{LogsDir: "any"},
				view.ShowMessage,
				tc.sessionRunErr,
				"",
				"",
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

func TestBuildSessionSummaryMessage(t *testing.T) {
	t.Run("includes token and context stats", func(t *testing.T) {
		session := &core.SweSession{}
		summary := BuildSessionSummaryMessage(5*time.Second, session, BuildSystemResult{})
		assert.Contains(t, summary, "Session completed in 5s | tokens(input=0[cached=0,noncached=0], output=0, total=0) | context=0")
		assert.Contains(t, summary, "Model: -")
		assert.Contains(t, summary, "Thinking: -")
		assert.Contains(t, summary, "LSP server: -")
		assert.Contains(t, summary, "Container image: -")
		assert.Contains(t, summary, "Roles used: -")
		assert.Contains(t, summary, "Tools used: -")
		assert.Contains(t, summary, "Edited files:\n-")
	})

	t.Run("nil session returns base summary", func(t *testing.T) {
		assert.Equal(t,
			"Session completed in 5s",
			BuildSessionSummaryMessage(5*time.Second, nil, BuildSystemResult{}),
		)
	})
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
