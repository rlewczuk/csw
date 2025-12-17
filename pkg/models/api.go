package models

import (
	"context"
	"errors"
	"time"
)

var (
	ErrEndpointNotFound    = errors.New("endpoint or host not found (either 404 or not host not in DNS)")
	ErrEndpointUnavailable = errors.New("endpoint is unavailable (network error, host not responding, or 5xx error)")
	ErrPermissionDenied    = errors.New("permission denied (eg. missing API key)")
	ErrRateExceeded        = errors.New("rate exceeded")
	ErrTooManyInputTokens  = errors.New("too many input tokens (i.e. exceeding context length)")
	ErrToBeContinued       = errors.New("to be continued (i.e. generated tokens limit reached)")
)

type ModelConnectionOptions struct {
	APIKey         string
	ConnectTimeout time.Duration
	RequestTimeout time.Duration
}

type ModelType string

const (
	ModelTypeChat  ModelType = "chat"
	ModelTypeEmbed ModelType = "embed"
)

type ModelInfo struct {
	Name       string
	Model      string
	ModifiedAt string
	Size       int64
	Family     string
}

// ChatRole represents role of a message in chat.
type ChatRole string

const (
	ChatRoleAssistant ChatRole = "assistant"
	ChatRoleSystem    ChatRole = "system"
	ChatRoleUser      ChatRole = "user"
)

type ChatOptions struct {
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

type EmbeddingModel interface {
	Embed(ctx context.Context, input string) ([]float64, error)
}

type ModelProvider interface {
	ListModels() ([]ModelInfo, error)
}
