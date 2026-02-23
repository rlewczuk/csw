package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/presenter"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/ui/cli"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLINonStreamingAssistantMessageNotDuplicated verifies assistant messages are not
// rendered multiple times when a non-streaming response includes both text and tool calls.
func TestCLINonStreamingAssistantMessageNotDuplicated(t *testing.T) {
	vfsInstance := vfs.NewMockVFS()
	err := vfsInstance.WriteFile("notes.txt", []byte("hello"))
	require.NoError(t, err)

	mockProvider := models.NewMockProvider(nil)
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: &models.ChatMessage{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{
				{Text: "Let me read the file."},
				{ToolCall: &tool.ToolCall{
					ID:        "call-1",
					Function:  "vfsRead",
					Arguments: tool.NewToolValue(map[string]any{"path": "notes.txt"}),
				}},
			},
		},
	})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, "Done."),
	})

	fixture := newCliSystemFixture(t, "You are a helpful assistant.",
		coretestfixture.WithProviderName("mock"),
		coretestfixture.WithModelProvider(mockProvider),
		coretestfixture.WithVFS(vfsInstance),
	)
	system := fixture.System

	thread := core.NewSessionThread(system, nil)
	err = thread.StartSession("mock/test-model")
	require.NoError(t, err)

	chatPresenter := presenter.NewChatPresenter(system, thread)
	output := &bytes.Buffer{}
	view := cli.NewCliChatView(chatPresenter, output, nil, false, false)
	err = chatPresenter.SetView(view)
	require.NoError(t, err)

	userMsg := &ui.ChatMessageUI{
		Role: ui.ChatRoleUser,
		Text: "Read notes.txt",
	}
	err = chatPresenter.SendUserMessage(userMsg)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return !thread.IsRunning()
	}, 5*time.Second, 10*time.Millisecond)

	outputStr := output.String()
	assert.Equal(t, 1, bytes.Count([]byte(outputStr), []byte("Assistant: Let me read the file.")))
	assert.Contains(t, outputStr, "✅ read notes.txt")
	assert.Contains(t, outputStr, "Assistant: Done.")
}
