package core

import (
	"context"
	"iter"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
)

type kimiTestChatModel struct {
	response     *models.ChatMessage
	err          error
	calls        int
	lastMessages []*models.ChatMessage
}

func (m *kimiTestChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) (*models.ChatMessage, error) {
	_ = ctx
	_ = options
	_ = tools
	m.calls++
	m.lastMessages = cloneMessages(messages)
	if m.err != nil {
		return nil, m.err
	}

	return m.response, nil
}

func (m *kimiTestChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) iter.Seq[*models.ChatMessage] {
	_ = ctx
	_ = messages
	_ = options
	_ = tools

	return func(yield func(*models.ChatMessage) bool) {
		_ = yield
	}
}

func (m *kimiTestChatModel) Compactor() models.ChatCompator {
	return nil
}

func TestKimiCompactorCompactMessages(t *testing.T) {
	t.Run("returns compacted summary and preserves last messages", func(t *testing.T) {
		chatModel := &kimiTestChatModel{
			response: &models.ChatMessage{Role: models.ChatRoleAssistant, Parts: []models.ChatMessagePart{
				{ReasoningContent: "hidden"},
				{Text: "summary line"},
			}},
		}

		compactor := NewKimiCompactor(chatModel, 2)
		messages := []*models.ChatMessage{
			models.NewTextMessage(models.ChatRoleSystem, "system prompt"),
			models.NewTextMessage(models.ChatRoleUser, "old user"),
			models.NewTextMessage(models.ChatRoleAssistant, "old assistant"),
			models.NewTextMessage(models.ChatRoleUser, "keep user"),
			models.NewTextMessage(models.ChatRoleAssistant, "keep assistant"),
		}

		got := compactor.CompactMessages(messages)

		require.Len(t, got, 3)
		assert.Equal(t, "keep user", got[0].GetText())
		assert.Equal(t, models.ChatRoleUser, got[1].Role)
		assert.Contains(t, got[1].GetText(), kimiCompactorPrefix)
		assert.Contains(t, got[1].GetText(), "summary line")
		assert.NotContains(t, got[1].GetText(), "hidden")
		assert.Equal(t, "keep assistant", got[2].GetText())

		require.Equal(t, 1, chatModel.calls)
		require.Len(t, chatModel.lastMessages, 2)
		assert.Equal(t, models.ChatRoleSystem, chatModel.lastMessages[0].Role)
		assert.Equal(t, kimiCompactorSystemPrompt, chatModel.lastMessages[0].GetText())
		assert.Contains(t, chatModel.lastMessages[1].GetText(), "## Message 1")
		assert.Contains(t, chatModel.lastMessages[1].GetText(), kimiCompactorPrompt)
	})

	t.Run("places first preserved user before summary when preserved starts with assistant", func(t *testing.T) {
		chatModel := &kimiTestChatModel{
			response: models.NewTextMessage(models.ChatRoleAssistant, "summary"),
		}

		compactor := NewKimiCompactor(chatModel, 2)
		messages := []*models.ChatMessage{
			models.NewTextMessage(models.ChatRoleSystem, "system prompt"),
			models.NewTextMessage(models.ChatRoleUser, "old user"),
			models.NewTextMessage(models.ChatRoleAssistant, "keep assistant"),
			models.NewTextMessage(models.ChatRoleUser, "keep user"),
		}

		got := compactor.CompactMessages(messages)

		require.Len(t, got, 3)
		assert.Equal(t, "keep user", got[0].GetText())
		assert.Contains(t, got[1].GetText(), kimiCompactorPrefix)
		assert.Equal(t, "keep assistant", got[2].GetText())
	})

	t.Run("returns original messages when chat model fails", func(t *testing.T) {
		chatModel := &kimiTestChatModel{err: assert.AnError}
		compactor := NewKimiCompactor(chatModel, 2)
		messages := []*models.ChatMessage{
			models.NewTextMessage(models.ChatRoleSystem, "system"),
			models.NewTextMessage(models.ChatRoleUser, "u1"),
			models.NewTextMessage(models.ChatRoleAssistant, "a1"),
			models.NewTextMessage(models.ChatRoleUser, "u2"),
			models.NewTextMessage(models.ChatRoleAssistant, "a2"),
		}

		got := compactor.CompactMessages(messages)
		require.Len(t, got, len(messages))
		assert.Equal(t, "system", got[0].GetText())
		assert.Equal(t, "u2", got[3].GetText())
	})

	t.Run("skips compaction when not enough user assistant messages", func(t *testing.T) {
		chatModel := &kimiTestChatModel{}
		compactor := NewKimiCompactor(chatModel, 3)
		messages := []*models.ChatMessage{
			models.NewTextMessage(models.ChatRoleSystem, "system"),
			models.NewTextMessage(models.ChatRoleUser, "u1"),
			models.NewTextMessage(models.ChatRoleAssistant, "a1"),
		}

		got := compactor.CompactMessages(messages)
		require.Len(t, got, 3)
		assert.Equal(t, "system", got[0].GetText())
		assert.Equal(t, 0, chatModel.calls)
	})
}

func TestKimiCompactorPrepare(t *testing.T) {
	compactor := &KimiCompactor{nmessages: 2}
	toolCall := compactTestToolCall("c1", "vfsRead", map[string]any{"path": "a"})
	messages := []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleSystem, "system"),
		{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{
				{ReasoningContent: "internal"},
				{ToolCall: toolCall},
				{Text: "visible"},
			},
		},
		models.NewTextMessage(models.ChatRoleUser, "keep user"),
		models.NewTextMessage(models.ChatRoleAssistant, "keep assistant"),
	}

	compactMsg, preserved := compactor.prepare(messages)
	require.NotNil(t, compactMsg)
	require.Len(t, preserved, 2)
	assert.Equal(t, "keep user", preserved[0].GetText())
	assert.Equal(t, "keep assistant", preserved[1].GetText())
	assert.NotContains(t, compactMsg.GetText(), "internal")
	assert.Contains(t, compactMsg.GetText(), "visible")
	assert.Contains(t, compactMsg.GetText(), "\"Function\":\"vfsRead\"")
	assert.True(t, strings.Contains(compactMsg.GetText(), kimiCompactorPrompt))
}
