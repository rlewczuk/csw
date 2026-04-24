package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rlewczuk/csw/pkg/models"
)

const (
	kimiCompactorSystemPrompt = "You are a helpful assistant that compacts conversation context."
	kimiCompactorPrompt       = "Please compact the conversation above into a concise summary that preserves key decisions, constraints, tool outcomes, and current next steps."
	kimiCompactorPrefix       = "Previous context has been compacted. Here is the compaction output:"
)

// KimiCompactor compacts old conversation context using a chat model summary.
type KimiCompactor struct {
	model     models.ChatModel
	nmessages int
}

// NewKimiCompactor creates kimi-style compactor preserving last n user/assistant messages.
func NewKimiCompactor(model models.ChatModel, nmessages int) ChatCompactor {
	return &KimiCompactor{model: model, nmessages: nmessages}
}

// CompactMessages compacts messages and preserves the latest configured user/assistant messages.
func (c *KimiCompactor) CompactMessages(messages []*models.ChatMessage) []*models.ChatMessage {
	compactMessage, preserved := c.prepare(messages)
	if compactMessage == nil {
		return preserved
	}

	if c == nil || c.model == nil {
		return cloneMessages(messages)
	}

	summary, err := c.model.Chat(context.Background(), []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleSystem, kimiCompactorSystemPrompt),
		compactMessage,
	}, nil, nil)
	if err != nil || summary == nil {
		return cloneMessages(messages)
	}

	parts := []models.ChatMessagePart{{Text: kimiCompactorPrefix}}
	for _, part := range summary.Parts {
		if part.ReasoningContent != "" {
			continue
		}

		serialized := kimiCompactorSerializePart(part)
		if strings.TrimSpace(serialized) == "" {
			continue
		}

		parts = append(parts, models.ChatMessagePart{Text: serialized})
	}

	compacted := &models.ChatMessage{Role: models.ChatRoleUser, Parts: parts}
	result := make([]*models.ChatMessage, 0, len(preserved)+1)

	firstUserIndex := -1
	for index, message := range preserved {
		if message.Role == models.ChatRoleUser {
			firstUserIndex = index
			break
		}
	}

	if firstUserIndex >= 0 {
		result = append(result, preserved[firstUserIndex])
		result = append(result, compacted)
		result = append(result, preserved[:firstUserIndex]...)
		result = append(result, preserved[firstUserIndex+1:]...)

		return result
	}

	result = append(result, compacted)
	result = append(result, preserved...)

	return result
}

func (c *KimiCompactor) prepare(messages []*models.ChatMessage) (*models.ChatMessage, []*models.ChatMessage) {
	history := cloneMessages(messages)
	if len(history) == 0 || c == nil || c.nmessages <= 0 {
		return nil, history
	}

	preserveStartIndex := len(history)
	npreserved := 0
	for index := len(history) - 1; index >= 0; index-- {
		if history[index].Role == models.ChatRoleUser || history[index].Role == models.ChatRoleAssistant {
			npreserved++
			if npreserved == c.nmessages {
				preserveStartIndex = index
				break
			}
		}
	}

	if npreserved < c.nmessages {
		return nil, history
	}

	if preserveStartIndex > 0 && kimiCompactorMessageHasToolCalls(history[preserveStartIndex-1]) {
		preserveStartIndex--
	}

	toCompact := history[:preserveStartIndex]
	toPreserve := history[preserveStartIndex:]
	if len(toCompact) == 0 {
		return nil, toPreserve
	}

	var builder strings.Builder
	for i, msg := range toCompact {
		builder.WriteString(fmt.Sprintf("## Message %d\nRole: %s\nContent:\n", i+1, msg.Role))
		for _, part := range msg.Parts {
			if part.ReasoningContent != "" {
				continue
			}

			serialized := kimiCompactorSerializePart(part)
			if strings.TrimSpace(serialized) == "" {
				continue
			}

			builder.WriteString(serialized)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\n")
	builder.WriteString(kimiCompactorPrompt)

	return models.NewTextMessage(models.ChatRoleUser, builder.String()), toPreserve
}

func kimiCompactorSerializePart(part models.ChatMessagePart) string {
	if strings.TrimSpace(part.Text) != "" {
		return part.Text
	}

	if part.ToolCall != nil {
		bytes, err := json.Marshal(part.ToolCall)
		if err == nil {
			return string(bytes)
		}
	}

	if part.ToolResponse != nil {
		bytes, err := json.Marshal(part.ToolResponse)
		if err == nil {
			return string(bytes)
		}
	}

	return ""
}

func kimiCompactorMessageHasToolCalls(message *models.ChatMessage) bool {
	if message == nil {
		return false
	}

	for _, part := range message.Parts {
		if part.ToolCall != nil {
			return true
		}
	}

	return false
}
