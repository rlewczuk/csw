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
		TokenUsage:          &models.TokenUsage{InputTokens: 10, InputCachedTokens: 6, InputNonCachedTokens: 4, OutputTokens: 4, TotalTokens: 14},
		ContextLengthTokens: 14,
	}, "outgoing", "test")

	session.appendConversationMessage(&models.ChatMessage{
		Role: models.ChatRoleAssistant,
		Parts: []models.ChatMessagePart{
			{Text: "second"},
		},
		TokenUsage:          &models.TokenUsage{InputTokens: 12, InputCachedTokens: 2, InputNonCachedTokens: 10, OutputTokens: 6, TotalTokens: 18},
		ContextLengthTokens: 32,
	}, "outgoing", "test")

	assert.Equal(t, models.TokenUsage{InputTokens: 22, InputCachedTokens: 8, InputNonCachedTokens: 14, OutputTokens: 10, TotalTokens: 32}, session.TokenUsage())
	assert.Equal(t, 32, session.ContextLengthTokens())

	state := session.GetState()
	assert.Equal(t, 32, state.Info.ContextLengthTokens)
	assert.Equal(t, models.TokenUsage{InputTokens: 22, InputCachedTokens: 8, InputNonCachedTokens: 14, OutputTokens: 10, TotalTokens: 32}, state.Info.TokenUsage)
}

func TestSweSessionAppendConversationMessageUsesLatestContextLength(t *testing.T) {
	session := &SweSession{}

	session.appendConversationMessage(&models.ChatMessage{
		Role:                models.ChatRoleAssistant,
		Parts:               []models.ChatMessagePart{{Text: "first"}},
		TokenUsage:          &models.TokenUsage{InputTokens: 10, InputNonCachedTokens: 10, OutputTokens: 4, TotalTokens: 14},
		ContextLengthTokens: 14,
	}, "outgoing", "test")

	session.appendConversationMessage(&models.ChatMessage{
		Role:                models.ChatRoleAssistant,
		Parts:               []models.ChatMessagePart{{Text: "second"}},
		TokenUsage:          &models.TokenUsage{InputTokens: 12, InputNonCachedTokens: 12, OutputTokens: 6, TotalTokens: 18},
		ContextLengthTokens: 18,
	}, "outgoing", "test")

	assert.Equal(t, 18, session.ContextLengthTokens())
}

func TestSweSessionAppendConversationMessageFallsBackContextLengthToMessageUsage(t *testing.T) {
	session := &SweSession{}

	session.appendConversationMessage(&models.ChatMessage{
		Role:  models.ChatRoleAssistant,
		Parts: []models.ChatMessagePart{{Text: "usage only"}},
		TokenUsage: &models.TokenUsage{
			InputTokens:          7,
			InputCachedTokens:    2,
			InputNonCachedTokens: 5,
			OutputTokens:         3,
			TotalTokens:          10,
		},
	}, "outgoing", "test")

	assert.Equal(t, 10, session.ContextLengthTokens())
}
