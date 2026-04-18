package models

import (
	"context"
	"iter"
	"strings"

	"github.com/rlewczuk/csw/pkg/tool"
)

// UnstreamingChatModel wraps a ChatModel and provides both streaming and non-streaming
// interfaces. The ChatStream method passes through to the wrapped model, while the Chat
// method collects all stream fragments into a single message with concatenated text parts.
type UnstreamingChatModel struct {
	wrapped ChatModel
}

// NewUnstreamingChatModel creates a new UnstreamingChatModel that wraps the given ChatModel.
func NewUnstreamingChatModel(wrapped ChatModel) *UnstreamingChatModel {
	return &UnstreamingChatModel{wrapped: wrapped}
}

// ChatStream passes the request to the wrapped model's ChatStream method and returns
// the result as-is.
func (u *UnstreamingChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return u.wrapped.ChatStream(ctx, messages, options, tools)
}

// Compactor returns nil because unstreaming wrapper does not provide session compaction.
func (u *UnstreamingChatModel) Compactor() ChatCompator {
	return nil
}

// Chat calls the wrapped model's Chat method to perform synchronous communication.
// If a model returns an empty response without error, it falls back to collecting
// fragments from ChatStream.
func (u *UnstreamingChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	response, err := u.wrapped.Chat(ctx, messages, options, tools)
	if err != nil {
		return nil, err
	}
	if response != nil {
		aggregated := aggregateAssistantMessage(response.Role, response.Parts)
		aggregated.TokenUsage = response.TokenUsage
		aggregated.ContextLengthTokens = response.ContextLengthTokens
		return aggregated, nil
	}

	var role ChatRole
	allParts := make([]ChatMessagePart, 0)
	tokenUsage := &TokenUsage{}
	hasUsage := false
	contextLength := 0
	for fragment := range u.wrapped.ChatStream(ctx, messages, options, tools) {
		if fragment == nil {
			continue
		}
		if role == "" && fragment.Role != "" {
			role = fragment.Role
		}
		if fragment.TokenUsage != nil {
			hasUsage = true
			tokenUsage.InputTokens += fragment.TokenUsage.InputTokens
			tokenUsage.InputCachedTokens += fragment.TokenUsage.InputCachedTokens
			tokenUsage.InputNonCachedTokens += fragment.TokenUsage.InputNonCachedTokens
			tokenUsage.OutputTokens += fragment.TokenUsage.OutputTokens
			tokenUsage.TotalTokens += fragment.TokenUsage.TotalTokens
		}
		if fragment.ContextLengthTokens > 0 {
			contextLength = fragment.ContextLengthTokens
		}
		allParts = append(allParts, fragment.Parts...)
	}

	result := aggregateAssistantMessage(role, allParts)
	if hasUsage {
		result.TokenUsage = tokenUsage
	}
	result.ContextLengthTokens = contextLength
	return result, nil
}

// aggregateAssistantMessage collapses text and reasoning chunks into single parts while
// preserving tool call and tool response parts.
func aggregateAssistantMessage(role ChatRole, parts []ChatMessagePart) *ChatMessage {
	if role == "" {
		role = ChatRoleAssistant
	}

	var allText strings.Builder
	var allReasoning strings.Builder
	otherParts := make([]ChatMessagePart, 0)

	for _, part := range parts {
		if part.ToolCall != nil || part.ToolResponse != nil {
			otherParts = append(otherParts, part)
			continue
		}
		allText.WriteString(part.Text)
		if part.ReasoningContent != "" {
			allReasoning.WriteString(part.ReasoningContent)
		}
	}

	result := &ChatMessage{Role: role}
	if allText.Len() > 0 {
		result.Parts = append(result.Parts, ChatMessagePart{Text: allText.String()})
	}
	result.Parts = append(result.Parts, otherParts...)
	if allReasoning.Len() > 0 {
		result.Parts = append(result.Parts, ChatMessagePart{ReasoningContent: allReasoning.String()})
	}

	return result
}
