package core

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestSweSessionAppendConversationMessageTracksTokenStats(t *testing.T) {
	session := &SweSession{}

	session.appendConversationMessage(&models.ChatMessage{
		Role: models.ChatRoleAssistant,
		Parts: []models.ChatMessagePart{
			{Text: "first"},
		},
		TokenUsage:          &models.TokenUsage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14},
		ContextLengthTokens: 14,
	}, "outgoing", "test")

	session.appendConversationMessage(&models.ChatMessage{
		Role: models.ChatRoleAssistant,
		Parts: []models.ChatMessagePart{
			{Text: "second"},
		},
		TokenUsage:          &models.TokenUsage{InputTokens: 12, OutputTokens: 6, TotalTokens: 18},
		ContextLengthTokens: 32,
	}, "outgoing", "test")

	assert.Equal(t, models.TokenUsage{InputTokens: 22, OutputTokens: 10, TotalTokens: 32}, session.TokenUsage())
	assert.Equal(t, 32, session.ContextLengthTokens())

	state := session.GetState()
	assert.Equal(t, 32, state.Info.ContextLengthTokens)
	assert.Equal(t, models.TokenUsage{InputTokens: 22, OutputTokens: 10, TotalTokens: 32}, state.Info.TokenUsage)
}
