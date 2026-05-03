package core

import (
	"strings"

	"github.com/rlewczuk/csw/pkg/models"
)

// ChatCompactorVerifier wraps ChatCompactor and fixes invalid tool call/response pairs.
type ChatCompactorVerifier struct {
	wrapped ChatCompactor
}

// NewChatCompactorVerifier creates a verifier wrapper for a ChatCompactor.
func NewChatCompactorVerifier(wrapped ChatCompactor) ChatCompactor {
	return &ChatCompactorVerifier{wrapped: wrapped}
}

// CompactMessages compacts messages with wrapped compactor and removes unmatched tool parts.
func (c *ChatCompactorVerifier) CompactMessages(messages []*models.ChatMessage) []*models.ChatMessage {
	if c == nil || c.wrapped == nil {
		return validateCompactedMessages(cloneMessages(messages))
	}

	return validateCompactedMessages(c.wrapped.CompactMessages(messages))
}

func validateCompactedMessages(messages []*models.ChatMessage) []*models.ChatMessage {
	cloned := cloneMessages(messages)
	if len(cloned) == 0 {
		return cloned
	}

	toolCallIDs := make(map[string]struct{})
	toolResponseIDs := make(map[string]struct{})
	for _, msg := range cloned {
		if msg == nil {
			continue
		}

		for _, part := range msg.Parts {
			if part.ToolCall != nil {
				callID := strings.TrimSpace(part.ToolCall.ID)
				if callID != "" {
					toolCallIDs[callID] = struct{}{}
				}
			}
			if part.ToolResponse != nil && part.ToolResponse.Call != nil {
				responseID := strings.TrimSpace(part.ToolResponse.Call.ID)
				if responseID != "" {
					toolResponseIDs[responseID] = struct{}{}
				}
			}
		}
	}

	result := make([]*models.ChatMessage, 0, len(cloned))
	for _, msg := range cloned {
		if msg == nil {
			continue
		}

		updatedParts := make([]models.ChatMessagePart, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			if shouldRemoveUnmatchedToolPart(part, toolCallIDs, toolResponseIDs) {
				continue
			}
			updatedParts = append(updatedParts, part)
		}

		if len(updatedParts) == 0 {
			continue
		}

		updatedMsg := cloneMessage(msg)
		updatedMsg.Parts = updatedParts
		result = append(result, updatedMsg)
	}

	return result
}

func shouldRemoveUnmatchedToolPart(part models.ChatMessagePart, toolCallIDs map[string]struct{}, toolResponseIDs map[string]struct{}) bool {
	if part.ToolCall != nil {
		callID := strings.TrimSpace(part.ToolCall.ID)
		if callID == "" {
			return true
		}
		if _, ok := toolResponseIDs[callID]; !ok {
			return true
		}
	}

	if part.ToolResponse != nil {
		responseID := ""
		if part.ToolResponse.Call != nil {
			responseID = strings.TrimSpace(part.ToolResponse.Call.ID)
		}
		if responseID == "" {
			return true
		}
		if _, ok := toolCallIDs[responseID]; !ok {
			return true
		}
	}

	return false
}
