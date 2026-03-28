package models

import (
	"context"
	"iter"
	"os"
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/tool"
)

// MockClient implements models.ModelProvider interface for testing purposes.
type MockClient struct {
	models          []ModelInfo
	chatResponses   map[string]*MockChatResponse
	chatResponseQue map[string][]*MockChatResponse
	embedResponses  map[string][]float64
	rawLLMCallback  func(string)
	mu              sync.RWMutex
	// Config holds provider configuration for this mock.
	Config *conf.ModelProviderConfig
	// RecordedToolCalls stores tool calls received from the LLM during tests
	RecordedToolCalls []tool.ToolCall
	// RecordedToolResponses stores tool responses sent to the LLM during tests
	RecordedToolResponses []tool.ToolResponse
	// RecordedMessages stores all messages sent to the mock provider during tests
	RecordedMessages [][]*ChatMessage
	// RateLimitError configures the mock to return a rate limit error
	// If set, the mock will return this error for every request
	RateLimitError *RateLimitError
	// RateLimitErrorCount limits how many rate limit errors to return.
	// If nil, the mock returns the error on every request.
	RateLimitErrorCount *int
	// NetworkError configures the mock to return a network error
	// If set, the mock will return this error for every request
	NetworkError *NetworkError
	// NetworkErrorCount limits how many network errors to return.
	// If nil, the mock returns the error on every request.
	NetworkErrorCount *int
}

// MockChatResponse holds the configuration for mock chat responses.
type MockChatResponse struct {
	// Response is the full response message for non-streaming calls
	Response *ChatMessage
	// StreamFragments are the message fragments for streaming calls
	StreamFragments []*ChatMessage
	// FinishWithEmptyMessage adds an empty final fragment to the stream.
	FinishWithEmptyMessage bool
	// Error to return (if any)
	Error error
}

// NewMockProvider creates a new mock provider with the given model list.
func NewMockProvider(models []ModelInfo) *MockClient {
	return &MockClient{
		models:          models,
		chatResponses:   make(map[string]*MockChatResponse),
		chatResponseQue: make(map[string][]*MockChatResponse),
		embedResponses:  make(map[string][]float64),
	}
}

// SetChatResponse configures the mock response for a specific model.
func (p *MockClient) SetChatResponse(modelName string, response *MockChatResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.chatResponseQue[modelName] = append(p.chatResponseQue[modelName], response)
}

// AddChatResponse appends a response to the response queue for a specific model.
func (p *MockClient) AddChatResponse(modelName string, response *MockChatResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.chatResponseQue[modelName] = append(p.chatResponseQue[modelName], response)
}

// SetResponsesFromFile configures mock responses for a specific model from a file.
func (p *MockClient) SetResponsesFromFile(modelName string, filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	prompts := strings.Split(string(content), "@@@@@@@")
	fragments := make([]*ChatMessage, len(prompts))
	for i, prompt := range prompts {
		fragments[i] = &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{Text: strings.TrimSpace(prompt)},
			},
		}
	}

	p.SetChatResponse(modelName, &MockChatResponse{
		StreamFragments: fragments,
	})
	return nil
}

// SetEmbedResponse configures the mock embedding response for a specific model.
func (p *MockClient) SetEmbedResponse(modelName string, embedding []float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.embedResponses[modelName] = embedding
}

// ListModels returns the predefined list of models.
func (p *MockClient) ListModels() ([]ModelInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.models, nil
}

// GetConfig returns the provider configuration for this mock client.
func (p *MockClient) GetConfig() *conf.ModelProviderConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Config
}

// ChatModel returns a MockChatModel implementation for the given model and options.
func (p *MockClient) ChatModel(model string, options *ChatOptions) ChatModel {
	return &MockChatModel{
		provider: p,
		model:    model,
		options:  options,
	}
}

// EmbeddingModel returns a MockEmbeddingModel implementation for the given model.
func (p *MockClient) EmbeddingModel(model string) EmbeddingModel {
	return &MockEmbeddingModel{
		provider: p,
		model:    model,
	}
}

// SetVerbose is a no-op for the mock client as it doesn't make HTTP requests.
func (p *MockClient) SetVerbose(verbose bool) {
	// No-op: Mock client doesn't make HTTP requests
}

// SetRawLLMCallback sets callback used by tests to capture raw LLM communication lines.
func (p *MockClient) SetRawLLMCallback(callback func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rawLLMCallback = callback
}

// MockChatModel implements models.ChatModel interface for testing purposes.
type MockChatModel struct {
	provider *MockClient
	model    string
	options  *ChatOptions
}

// Chat sends a chat request and returns the full response.
func (m *MockChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	// Check for cancellation first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Record all messages and tool responses from incoming messages
	m.provider.mu.Lock()
	messageCopy := make([]*ChatMessage, len(messages))
	for i, msg := range messages {
		messageCopy[i] = msg
		for _, resp := range msg.GetToolResponses() {
			m.provider.RecordedToolResponses = append(m.provider.RecordedToolResponses, *resp)
		}
	}
	m.provider.RecordedMessages = append(m.provider.RecordedMessages, messageCopy)
	m.provider.mu.Unlock()

	// Check for rate limit error
	m.provider.mu.Lock()
	rateLimitErr := m.provider.RateLimitError
	rateLimitCount := m.provider.RateLimitErrorCount
	if rateLimitErr != nil {
		if rateLimitCount == nil {
			m.provider.mu.Unlock()
			return nil, rateLimitErr
		}
		if *rateLimitCount > 0 {
			*rateLimitCount = *rateLimitCount - 1
			m.provider.mu.Unlock()
			return nil, rateLimitErr
		}
	}

	// Check for network error
	networkErr := m.provider.NetworkError
	networkErrCount := m.provider.NetworkErrorCount
	if networkErr != nil {
		if networkErrCount == nil {
			m.provider.mu.Unlock()
			return nil, networkErr
		}
		if *networkErrCount > 0 {
			*networkErrCount = *networkErrCount - 1
			m.provider.mu.Unlock()
			return nil, networkErr
		}
	}
	m.provider.mu.Unlock()

	var response *MockChatResponse
	m.provider.mu.Lock()
	if queue := m.provider.chatResponseQue[m.model]; len(queue) > 0 {
		response = queue[0]
		m.provider.chatResponseQue[m.model] = queue[1:]
	}
	m.provider.mu.Unlock()

	if response == nil {
		// Default response if none configured
		return &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{Text: "mock response"},
			},
		}, nil
	}

	if response.Error != nil {
		return nil, response.Error
	}

	result := response.Response
	if result == nil && len(response.StreamFragments) > 0 {
		var role ChatRole
		var combinedText strings.Builder
		var combinedReasoning strings.Builder
		parts := make([]ChatMessagePart, 0)
		for _, fragment := range response.StreamFragments {
			if role == "" && fragment != nil && fragment.Role != "" {
				role = fragment.Role
			}
			if fragment == nil {
				continue
			}
			for _, part := range fragment.Parts {
				if part.ToolCall != nil || part.ToolResponse != nil {
					parts = append(parts, part)
					continue
				}
				combinedText.WriteString(part.Text)
				if part.ReasoningContent != "" {
					combinedReasoning.WriteString(part.ReasoningContent)
				}
			}
		}
		if role == "" {
			role = ChatRoleAssistant
		}
		result = &ChatMessage{Role: role}
		if combinedText.Len() > 0 {
			result.Parts = append(result.Parts, ChatMessagePart{Text: combinedText.String()})
		}
		result.Parts = append(result.Parts, parts...)
		if combinedReasoning.Len() > 0 {
			result.Parts = append(result.Parts, ChatMessagePart{ReasoningContent: combinedReasoning.String()})
		}
	}

	if result == nil {
		result = &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{{Text: ""}},
		}
	}

	// Record tool calls from response
	m.provider.mu.Lock()
	for _, call := range result.GetToolCalls() {
		m.provider.RecordedToolCalls = append(m.provider.RecordedToolCalls, *call)
	}
	m.provider.mu.Unlock()

	return result, nil
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses.
func (m *MockChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		// Record all messages and tool responses from incoming messages
		m.provider.mu.Lock()
		messageCopy := make([]*ChatMessage, len(messages))
		for i, msg := range messages {
			messageCopy[i] = msg
			for _, resp := range msg.GetToolResponses() {
				m.provider.RecordedToolResponses = append(m.provider.RecordedToolResponses, *resp)
			}
		}
		m.provider.RecordedMessages = append(m.provider.RecordedMessages, messageCopy)
		m.provider.mu.Unlock()

		// Check for rate limit error
		m.provider.mu.Lock()
		rateLimitErr := m.provider.RateLimitError
		rateLimitCount := m.provider.RateLimitErrorCount
		if rateLimitErr != nil {
			if rateLimitCount == nil {
				m.provider.mu.Unlock()
				return
			}
			if *rateLimitCount > 0 {
				*rateLimitCount = *rateLimitCount - 1
				m.provider.mu.Unlock()
				return
			}
		}

		var response *MockChatResponse
		if queue := m.provider.chatResponseQue[m.model]; len(queue) > 0 {
			response = queue[0]
			m.provider.chatResponseQue[m.model] = queue[1:]
		}
		m.provider.mu.Unlock()

		var fragments []*ChatMessage

		if response == nil {
			// Default streaming response if none configured
			fragments = []*ChatMessage{
				{Role: ChatRoleAssistant, Parts: []ChatMessagePart{{Text: "mock"}}},
				{Role: ChatRoleAssistant, Parts: []ChatMessagePart{{Text: " stream"}}},
			}
		} else if response.Error != nil {
			// If there's an error configured, just return without yielding anything
			return
		} else {
			fragments = response.StreamFragments
		}

		if response != nil && response.FinishWithEmptyMessage {
			fragments = append(fragments, &ChatMessage{Role: ChatRoleAssistant})
		}

		// Record tool calls from fragments
		m.provider.mu.Lock()
		for _, fragment := range fragments {
			for _, call := range fragment.GetToolCalls() {
				m.provider.RecordedToolCalls = append(m.provider.RecordedToolCalls, *call)
			}
		}
		m.provider.mu.Unlock()

		// Yield each fragment
		for _, fragment := range fragments {
			// Check for cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !yield(fragment) {
				return
			}
		}
	}
}

// MockEmbeddingModel implements models.EmbeddingModel interface for testing purposes.
type MockEmbeddingModel struct {
	provider *MockClient
	model    string
}

// Embed returns the predefined embedding for the input.
func (m *MockEmbeddingModel) Embed(ctx context.Context, input string) ([]float64, error) {
	// Check for cancellation first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	m.provider.mu.RLock()
	embedding := m.provider.embedResponses[m.model]
	m.provider.mu.RUnlock()

	if embedding == nil {
		// Default embedding if none configured
		return []float64{0.1, 0.2, 0.3}, nil
	}

	return embedding, nil
}
