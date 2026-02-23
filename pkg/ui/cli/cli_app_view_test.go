package cli

import (
	"bytes"
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
