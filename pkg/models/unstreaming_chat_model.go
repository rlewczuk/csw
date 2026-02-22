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

// Chat calls the wrapped model's ChatStream method and concatenates all fragments
// into a single message. Text parts and reasoning content from assistant messages
// are concatenated into single parts, while tool calls and other parts are preserved as-is.
func (u *UnstreamingChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	var role ChatRole
	var allText strings.Builder
	var allReasoning strings.Builder
	var otherParts []ChatMessagePart

	for fragment := range u.wrapped.ChatStream(ctx, messages, options, tools) {
		if role == "" && fragment.Role != "" {
			role = fragment.Role
		}
		for _, part := range fragment.Parts {
			if part.ToolCall != nil {
				// Tool call parts are preserved as-is
				otherParts = append(otherParts, part)
			} else if part.ToolResponse != nil {
				// Tool response parts are preserved as-is
				otherParts = append(otherParts, part)
			} else {
				// Text part (possibly with ReasoningContent) - concatenate text and reasoning
				allText.WriteString(part.Text)
				if part.ReasoningContent != "" {
					allReasoning.WriteString(part.ReasoningContent)
				}
			}
		}
	}

	// Build result: concatenated text first, then other parts (tool calls, tool responses)
	result := &ChatMessage{
		Role:  role,
		Parts: nil,
	}
	if role == "" {
		result.Role = ChatRoleAssistant
	}

	if allText.Len() > 0 {
		result.Parts = append(result.Parts, ChatMessagePart{Text: allText.String()})
	}
	result.Parts = append(result.Parts, otherParts...)

	// Add concatenated reasoning content at the end if present
	if allReasoning.Len() > 0 {
		result.Parts = append(result.Parts, ChatMessagePart{ReasoningContent: allReasoning.String()})
	}

	return result, nil
}
