package core

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatCompactorVerifier_RemovesUnmatchedToolCallsAndResponses(t *testing.T) {
	wrapped := &testCoreCompactor{compacted: []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleUser, "keep user text"),
		{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{
				{Text: "keep assistant text"},
				{ToolCall: &tool.ToolCall{ID: "matched-call", Function: "matched"}},
				{ToolCall: &tool.ToolCall{ID: "orphan-call", Function: "orphan"}},
			},
		},
		{
			Role: models.ChatRoleUser,
			Parts: []models.ChatMessagePart{
				{ToolResponse: &tool.ToolResponse{Call: &tool.ToolCall{ID: "matched-call", Function: "matched"}, Done: true}},
				{ToolResponse: &tool.ToolResponse{Call: &tool.ToolCall{ID: "orphan-response", Function: "orphan"}, Done: true}},
			},
		},
	}}
	verifier := NewChatCompactorVerifier(wrapped)

	result := verifier.CompactMessages([]*models.ChatMessage{models.NewTextMessage(models.ChatRoleUser, "input")})

	require.True(t, wrapped.called)
	require.Len(t, result, 3)
	assert.Equal(t, "keep user text", result[0].GetText())
	require.Len(t, result[1].Parts, 2)
	assert.Equal(t, "keep assistant text", result[1].Parts[0].Text)
	require.NotNil(t, result[1].Parts[1].ToolCall)
	assert.Equal(t, "matched-call", result[1].Parts[1].ToolCall.ID)
	require.Len(t, result[2].Parts, 1)
	require.NotNil(t, result[2].Parts[0].ToolResponse)
	assert.Equal(t, "matched-call", result[2].Parts[0].ToolResponse.Call.ID)
}

func TestChatCompactorVerifier_DropsMessagesWithOnlyUnmatchedToolParts(t *testing.T) {
	wrapped := &testCoreCompactor{compacted: []*models.ChatMessage{
		models.NewToolCallMessage(&tool.ToolCall{ID: "orphan-call", Function: "orphan"}),
		models.NewToolResponseMessage(&tool.ToolResponse{Call: &tool.ToolCall{ID: "orphan-response", Function: "orphan"}, Done: true}),
		models.NewTextMessage(models.ChatRoleAssistant, "kept"),
	}}
	verifier := NewChatCompactorVerifier(wrapped)

	result := verifier.CompactMessages(nil)

	require.Len(t, result, 1)
	assert.Equal(t, "kept", result[0].GetText())
}

func TestChatCompactorVerifier_ClonesMessagesWhenWrappedCompactorMissing(t *testing.T) {
	input := []*models.ChatMessage{models.NewTextMessage(models.ChatRoleUser, "input")}
	verifier := NewChatCompactorVerifier(nil)

	result := verifier.CompactMessages(input)

	require.Len(t, result, 1)
	assert.Equal(t, "input", result[0].GetText())
	assert.NotSame(t, input[0], result[0])
}
