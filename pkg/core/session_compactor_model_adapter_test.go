package core

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type compactorProviderChatModel struct {
	compactor models.ChatCompator
}

func (m *compactorProviderChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) (*models.ChatMessage, error) {
	_ = ctx
	_ = messages
	_ = options
	_ = tools
	return models.NewTextMessage(models.ChatRoleAssistant, "ok"), nil
}

func (m *compactorProviderChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) iter.Seq[*models.ChatMessage] {
	_ = ctx
	_ = messages
	_ = options
	_ = tools
	return func(yield func(*models.ChatMessage) bool) {
		_ = yield
	}
}

func (m *compactorProviderChatModel) Compactor() models.ChatCompator {
	return m.compactor
}

type testModelCompactor struct {
	compacted []*models.ChatMessage
	err       error
}

func (c *testModelCompactor) CompactMessages(messages []*models.ChatMessage) ([]*models.ChatMessage, error) {
	_ = messages
	if c.err != nil {
		return nil, c.err
	}

	return c.compacted, nil
}

type testCoreCompactor struct {
	compacted []*models.ChatMessage
	called    bool
}

func (c *testCoreCompactor) CompactMessages(messages []*models.ChatMessage) []*models.ChatMessage {
	_ = messages
	c.called = true
	return c.compacted
}

func TestSweSessionConfigureCompactor_UsesModelCompactor(t *testing.T) {
	session := &SweSession{compactor: NewKimiCompactor(nil, defaultKimiCompactorMessagesToKeep)}
	modelCompactor := &testModelCompactor{compacted: []*models.ChatMessage{models.NewTextMessage(models.ChatRoleUser, "model compacted")}}
	providerModel := &compactorProviderChatModel{compactor: modelCompactor}
	fallbackModel := &compactorProviderChatModel{}

	session.configureCompactor(fallbackModel, providerModel)

	adapter, ok := session.compactor.(*modelChatCompactorAdapter)
	require.True(t, ok)
	result := adapter.CompactMessages([]*models.ChatMessage{models.NewTextMessage(models.ChatRoleUser, "input")})
	require.Len(t, result, 1)
	assert.Equal(t, "model compacted", result[0].GetText())
}

func TestSweSessionConfigureCompactor_UsesDefaultCompactorWhenModelCompactorMissing(t *testing.T) {
	session := &SweSession{compactor: NewKimiCompactor(nil, defaultKimiCompactorMessagesToKeep)}
	chatModel := &compactorProviderChatModel{}

	session.configureCompactor(chatModel, &compactorProviderChatModel{})

	kimiCompactor, ok := session.compactor.(*KimiCompactor)
	require.True(t, ok)
	assert.Equal(t, chatModel, kimiCompactor.model)
	assert.Equal(t, defaultKimiCompactorMessagesToKeep, kimiCompactor.nmessages)
}

func TestModelChatCompactorAdapter_FallsBackWhenModelCompactorFails(t *testing.T) {
	fallback := &testCoreCompactor{compacted: []*models.ChatMessage{models.NewTextMessage(models.ChatRoleUser, "fallback compacted")}}
	adapter := &modelChatCompactorAdapter{
		modelCompactor: &testModelCompactor{err: errors.New("compact failed")},
		fallback:       fallback,
	}

	result := adapter.CompactMessages([]*models.ChatMessage{models.NewTextMessage(models.ChatRoleUser, "input")})

	require.Len(t, result, 1)
	assert.True(t, fallback.called)
	assert.Equal(t, "fallback compacted", result[0].GetText())
}
