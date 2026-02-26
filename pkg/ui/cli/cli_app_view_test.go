package cli

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
)

func TestCliAppView_ShowChatAndSettings_NoOp(t *testing.T) {
	output := &bytes.Buffer{}
	view := NewCliAppView(output)
	presenter := mock.NewMockChatPresenter()

	chatView := view.ShowChat(presenter)
	view.ShowSettings()

	assert.Nil(t, chatView)
	assert.Equal(t, "", output.String())
}

func TestCliAppView_ShowMessage(t *testing.T) {
	tests := []struct {
		name        string
		messageType ui.MessageType
		expected    string
	}{
		{name: "info", messageType: ui.MessageTypeInfo, expected: "[INFO] hello\n"},
		{name: "warning", messageType: ui.MessageTypeWarning, expected: "[WARNING] hello\n"},
		{name: "error", messageType: ui.MessageTypeError, expected: "[ERROR] hello\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			view := NewCliAppView(output)

			view.ShowMessage("hello", tt.messageType)

			assert.Equal(t, tt.expected, output.String())
		})
	}
}

func TestCliAppView_ShowMessage_LogsDiagnosticMessage(t *testing.T) {
	tests := []struct {
		name         string
		messageType  ui.MessageType
		expectedLevel string
		expectedType string
	}{
		{name: "info", messageType: ui.MessageTypeInfo, expectedLevel: "INFO", expectedType: "info"},
		{name: "warning", messageType: ui.MessageTypeWarning, expectedLevel: "WARN", expectedType: "warning"},
		{name: "error", messageType: ui.MessageTypeError, expectedLevel: "ERROR", expectedType: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			logBuffer := &bytes.Buffer{}
			logger := slog.New(slog.NewJSONHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
			view := NewCliAppView(output)
			view.SetSessionLogger(logger)

			view.ShowMessage("hello", tt.messageType)

			logLine := logBuffer.String()
			assert.Contains(t, logLine, `"msg":"diagnostic_message"`)
			assert.Contains(t, logLine, `"diagnostic":true`)
			assert.Contains(t, logLine, `"message":"hello"`)
			assert.Contains(t, logLine, `"message_type":"`+tt.expectedType+`"`)
			assert.Contains(t, logLine, `"level":"`+tt.expectedLevel+`"`)
		})
	}
}

func TestCliAppView_SetSessionLogger_FlushesPendingDiagnosticMessages(t *testing.T) {
	output := &bytes.Buffer{}
	logBuffer := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	view := NewCliAppView(output)

	view.ShowMessage("before logger", ui.MessageTypeWarning)
	view.SetSessionLogger(logger)

	logLine := logBuffer.String()
	assert.Contains(t, logLine, `"msg":"diagnostic_message"`)
	assert.Contains(t, logLine, `"diagnostic":true`)
	assert.Contains(t, logLine, `"message":"before logger"`)
	assert.Contains(t, logLine, `"message_type":"warning"`)
	assert.True(t, strings.Count(logLine, "diagnostic_message") >= 1)
}
