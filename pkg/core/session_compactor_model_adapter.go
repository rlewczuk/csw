package core

import "github.com/rlewczuk/csw/pkg/models"

// modelChatCompactorAdapter bridges models.ChatCompator into core.ChatCompactor.
type modelChatCompactorAdapter struct {
	modelCompactor models.ChatCompator
	fallback       ChatCompactor
}

// CompactMessages compacts messages with model compactor and falls back on failure.
func (c *modelChatCompactorAdapter) CompactMessages(messages []*models.ChatMessage) []*models.ChatMessage {
	if c == nil || c.modelCompactor == nil {
		if c != nil && c.fallback != nil {
			return c.fallback.CompactMessages(messages)
		}
		return cloneMessages(messages)
	}

	compacted, err := c.modelCompactor.CompactMessages(messages)
	if err != nil || compacted == nil {
		if c.fallback != nil {
			return c.fallback.CompactMessages(messages)
		}
		return cloneMessages(messages)
	}

	return compacted
}
