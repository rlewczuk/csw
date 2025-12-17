package models

import "context"

// ChatRole represents role of a message in chat.
type ChatRole string

const (
	ChatRoleAssistant ChatRole = "assistant"
	ChatRoleSystem    ChatRole = "system"
	ChatRoleUser      ChatRole = "user"
)

type ChatOptions struct {
	Model       string
	Temperature float32
	TopP        float32
	TopK        int
}

// ChatMessage represents a message in chat. It can contain multiple parts. Parts are represented as serialized strings,
// converting various types of content to strings (i.e. base64 plus mime type) is the responsibility of the caller.
type ChatMessage struct {
	// Role of the message.
	Role ChatRole
	// Parts of the message - when concatenated, should form a single message.
	Parts []string
}

// ChatModel represents a model that can be used for chat.
type ChatModel interface {
	// Chat sends a chat request to the model and returns the response. This method is blocking and returns full response.
	Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions) (*ChatMessage, error)
}
