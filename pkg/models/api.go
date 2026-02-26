package models

import (
	"context"
	"iter"
	"log/slog"
	"time"

	"github.com/rlewczuk/csw/pkg/tool"
)

// Default retry configuration constants.
const (
	// DefaultMaxRetries is the default number of retries for network/rate limit errors.
	DefaultMaxRetries = 10
	// DefaultRetryBackoffScale is the default backoff duration scale for retries.
	DefaultRetryBackoffScale = 10 * time.Second
)

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
	ChatRoleDeveloper ChatRole = "developer"
	ChatRoleSystem    ChatRole = "system"
	ChatRoleUser      ChatRole = "user"
)

// ChatOptions contains options for chat requests.
type ChatOptions struct {
	Temperature float32
	TopP        float32
	TopK        int
	Verbose     bool
	Logger      *slog.Logger
	SessionID   string
	Headers     map[string]string
	// Thinking controls the thinking/reasoning mode for LLM requests.
	// For providers supporting effort levels (like OpenAI Responses API),
	// use values like "low", "medium", "high", "xhigh".
	// For providers with boolean thinking (like Anthropic extended thinking),
	// use "true" or "false".
	// If empty, no thinking parameters are sent to the LLM.
	Thinking string
}

// TokenUsage contains token accounting returned by an LLM provider.
type TokenUsage struct {
	InputTokens          int
	InputCachedTokens    int
	InputNonCachedTokens int
	OutputTokens         int
	TotalTokens          int
}

// ChatMessagePart represents a part of a chat message. A message can contain text, tool calls, or tool responses.
type ChatMessagePart struct {
	// Text contains text content (if this part is text)
	Text string
	// ReasoningContent contains reasoning/thinking content from thinking models (e.g., GLM-5)
	ReasoningContent string
	// ToolCall contains a tool call request from the LLM (if this part is a tool call)
	ToolCall *tool.ToolCall
	// ToolResponse contains a tool execution result (if this part is a tool response)
	ToolResponse *tool.ToolResponse
}

// ChatMessage represents a message in chat. It can contain multiple parts of different types:
// text content, tool calls from the LLM, or tool responses from the application.
type ChatMessage struct {
	// Role of the message.
	Role ChatRole
	// Parts of the message - can contain text, tool calls, or tool responses.
	Parts []ChatMessagePart
	// TokenUsage contains token accounting for this response, when provided by the backend.
	TokenUsage *TokenUsage
	// ContextLengthTokens stores provider-reported context length in tokens.
	// When unavailable, this value is 0.
	ContextLengthTokens int
}

// NewTextMessage creates a new ChatMessage with text content.
func NewTextMessage(role ChatRole, text string) *ChatMessage {
	return &ChatMessage{
		Role: role,
		Parts: []ChatMessagePart{
			{Text: text},
		},
	}
}

// NewToolCallMessage creates a new ChatMessage with tool calls.
func NewToolCallMessage(calls ...*tool.ToolCall) *ChatMessage {
	parts := make([]ChatMessagePart, len(calls))
	for i, call := range calls {
		parts[i] = ChatMessagePart{ToolCall: call}
	}
	return &ChatMessage{
		Role:  ChatRoleAssistant,
		Parts: parts,
	}
}

// NewToolResponseMessage creates a new ChatMessage with tool responses.
func NewToolResponseMessage(responses ...*tool.ToolResponse) *ChatMessage {
	parts := make([]ChatMessagePart, len(responses))
	for i, resp := range responses {
		parts[i] = ChatMessagePart{ToolResponse: resp}
	}
	return &ChatMessage{
		Role:  ChatRoleUser,
		Parts: parts,
	}
}

// AddText adds text content to the message.
func (m *ChatMessage) AddText(text string) {
	m.Parts = append(m.Parts, ChatMessagePart{Text: text})
}

// AddToolCall adds a tool call to the message.
func (m *ChatMessage) AddToolCall(call *tool.ToolCall) {
	m.Parts = append(m.Parts, ChatMessagePart{ToolCall: call})
}

// AddToolResponse adds a tool response to the message.
func (m *ChatMessage) AddToolResponse(resp *tool.ToolResponse) {
	m.Parts = append(m.Parts, ChatMessagePart{ToolResponse: resp})
}

// GetText returns all text content concatenated.
func (m *ChatMessage) GetText() string {
	var result string
	for _, part := range m.Parts {
		result += part.Text
	}
	return result
}

// GetToolCalls returns all tool calls in the message.
func (m *ChatMessage) GetToolCalls() []*tool.ToolCall {
	var calls []*tool.ToolCall
	for _, part := range m.Parts {
		if part.ToolCall != nil {
			calls = append(calls, part.ToolCall)
		}
	}
	return calls
}

// GetToolResponses returns all tool responses in the message.
func (m *ChatMessage) GetToolResponses() []*tool.ToolResponse {
	var responses []*tool.ToolResponse
	for _, part := range m.Parts {
		if part.ToolResponse != nil {
			responses = append(responses, part.ToolResponse)
		}
	}
	return responses
}

// ChatModel represents a model that can be used for chat.
type ChatModel interface {
	// Chat sends a chat request to the model and returns the response. This method is blocking and returns full response.
	// Tools parameter is optional and can be nil if no tools are available.
	Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error)
	// ChatStream sends a chat request to the model and returns a standard Go iterator that yields fragments of the response as they arrive.
	// The iterator will stop when the stream is complete. Use with standard for-range loops.
	// Tools parameter is optional and can be nil if no tools are available.
	ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage]
}

type EmbeddingModel interface {
	Embed(ctx context.Context, input string) ([]float64, error)
}

type ModelProvider interface {
	ListModels() ([]ModelInfo, error)

	// ChatModel returns a ChatModel implementation for the given model and options.
	ChatModel(model string, options *ChatOptions) ChatModel

	// EmbeddingModel returns an EmbeddingModel implementation for the given model.
	EmbeddingModel(model string) EmbeddingModel

	// SetVerbose enables or disables verbose logging for HTTP requests and responses.
	SetVerbose(verbose bool)
}
