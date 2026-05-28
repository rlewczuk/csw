package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
)

// KimiCompactor compacts old conversation context using a chat model summary.
type KimiCompactor struct {
	model        models.ChatModel
	nmessages    int
	systemPrompt string
	prompt       string
	prefix       string
}

// NewKimiCompactor creates kimi-style compactor preserving last n user/assistant messages.
func NewKimiCompactor(model models.ChatModel, nmessages int, config *conf.CswConfig) ChatCompactor {
	systemPrompt, prompt, prefix, _ := LoadKimiCompactorPromptTemplates(config)

	return &KimiCompactor{
		model:        model,
		nmessages:    nmessages,
		systemPrompt: systemPrompt,
		prompt:       prompt,
		prefix:       prefix,
	}
}

// LoadKimiCompactorPromptTemplates loads compactor prompts from configuration store.
func LoadKimiCompactorPromptTemplates(config *conf.CswConfig) (string, string, string, error) {
	if config == nil {
		return "", "", "", fmt.Errorf("LoadKimiCompactorPromptTemplates() [compact_kimi.go]: config cannot be nil")
	}

	compactFiles, ok := config.AgentConfigFiles["compact"]
	if !ok {
		return "", "", "", fmt.Errorf("LoadKimiCompactorPromptTemplates() [compact_kimi.go]: failed to read compact/system.md: compact files not found")
	}

	systemPrompt, ok := compactFiles["system.md"]
	if !ok {
		return "", "", "", fmt.Errorf("LoadKimiCompactorPromptTemplates() [compact_kimi.go]: failed to read compact/system.md: file not found")
	}

	prompt, ok := compactFiles["prompt.md"]
	if !ok {
		return "", "", "", fmt.Errorf("LoadKimiCompactorPromptTemplates() [compact_kimi.go]: failed to read compact/prompt.md: file not found")
	}

	prefix, ok := compactFiles["prefix.md"]
	if !ok {
		return "", "", "", fmt.Errorf("LoadKimiCompactorPromptTemplates() [compact_kimi.go]: failed to read compact/prefix.md: file not found")
	}

	return systemPrompt, prompt, prefix, nil
}

// CompactMessages compacts messages and preserves the latest configured user/assistant messages.
func (c *KimiCompactor) CompactMessages(messages []*models.ChatMessage) []*models.ChatMessage {
	toCompact, preserved := c.prepareMessages(messages)
	if len(toCompact) == 0 {
		return preserved
	}

	if c == nil || c.model == nil {
		return cloneMessages(messages)
	}
	if strings.TrimSpace(c.systemPrompt) == "" || strings.TrimSpace(c.prompt) == "" || strings.TrimSpace(c.prefix) == "" {
		return cloneMessages(messages)
	}

	compacted, ok := c.compactChunks(toCompact)
	if !ok {
		return cloneMessages(messages)
	}

	return c.mergeCompactedWithPreserved(compacted, preserved)
}

func (c *KimiCompactor) compactChunks(messages []*models.ChatMessage) ([]*models.ChatMessage, bool) {
	compactMessage := c.prepareChunk(messages)
	if compactMessage == nil {
		return nil, false
	}

	summary, err := c.model.Chat(context.Background(), []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleSystem, c.systemPrompt),
		compactMessage,
	}, nil, nil)
	if err != nil {
		if !errors.Is(err, models.ErrTooManyInputTokens) || len(messages) < 2 {
			return nil, false
		}

		first, second := splitKimiCompactorMessages(messages)
		if len(first) == 0 || len(second) == 0 {
			return nil, false
		}

		firstCompacted, ok := c.compactChunks(first)
		if !ok {
			return nil, false
		}

		secondCompacted, ok := c.compactChunks(second)
		if !ok {
			return nil, false
		}

		result := make([]*models.ChatMessage, 0, len(firstCompacted)+len(secondCompacted))
		result = append(result, firstCompacted...)
		result = append(result, secondCompacted...)

		return result, true
	}
	if summary == nil {
		return nil, false
	}

	return []*models.ChatMessage{c.summaryMessage(summary)}, true
}

func (c *KimiCompactor) summaryMessage(summary *models.ChatMessage) *models.ChatMessage {
	parts := []models.ChatMessagePart{{Text: c.prefix}}
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

	return &models.ChatMessage{Role: models.ChatRoleUser, Parts: parts}
}

func (c *KimiCompactor) mergeCompactedWithPreserved(compacted []*models.ChatMessage, preserved []*models.ChatMessage) []*models.ChatMessage {
	result := make([]*models.ChatMessage, 0, len(preserved)+len(compacted))

	firstUserIndex := -1
	for index, message := range preserved {
		if message.Role == models.ChatRoleUser {
			firstUserIndex = index
			break
		}
	}

	if firstUserIndex >= 0 {
		result = append(result, preserved[firstUserIndex])
		result = append(result, compacted...)
		result = append(result, preserved[:firstUserIndex]...)
		result = append(result, preserved[firstUserIndex+1:]...)

		return result
	}

	result = append(result, compacted...)
	result = append(result, preserved...)

	return result
}

func (c *KimiCompactor) prepare(messages []*models.ChatMessage) (*models.ChatMessage, []*models.ChatMessage) {
	toCompact, toPreserve := c.prepareMessages(messages)
	if len(toCompact) == 0 {
		return nil, toPreserve
	}

	return c.prepareChunk(toCompact), toPreserve
}

func (c *KimiCompactor) prepareMessages(messages []*models.ChatMessage) ([]*models.ChatMessage, []*models.ChatMessage) {
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

	return toCompact, toPreserve
}

func (c *KimiCompactor) prepareChunk(messages []*models.ChatMessage) *models.ChatMessage {
	if len(messages) == 0 || c == nil {
		return nil
	}

	var builder strings.Builder
	for i, msg := range messages {
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
	builder.WriteString(c.prompt)

	return models.NewTextMessage(models.ChatRoleUser, builder.String())
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

func splitKimiCompactorMessages(messages []*models.ChatMessage) ([]*models.ChatMessage, []*models.ChatMessage) {
	if len(messages) < 2 {
		return cloneMessages(messages), nil
	}

	mid := len(messages) / 2
	if mid <= 0 {
		mid = 1
	}
	if mid >= len(messages) {
		mid = len(messages) - 1
	}

	return cloneMessages(messages[:mid]), cloneMessages(messages[mid:])
}
