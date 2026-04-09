package main

import (
	"bytes"
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	sessionio "github.com/rlewczuk/csw/pkg/io"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
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

	output := &bytes.Buffer{}
	thread := core.NewSessionThread(system, sessionio.NewTextSessionOutput(output))
	err = thread.StartSession("mock/test-model")
	require.NoError(t, err)
	err = thread.UserPrompt("Read notes.txt")
	require.NoError(t, err)

	waitForThreadToFinish(t, thread)

	outputStr := output.String()
	assert.Equal(t, 1, bytes.Count([]byte(outputStr), []byte("Assistant: Let me read the file.")))
	assert.Contains(t, outputStr, "✅ read notes.txt")
	assert.Contains(t, outputStr, "Assistant: Done.")
}
