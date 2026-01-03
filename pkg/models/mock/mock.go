package mock

import (
	"context"
	"iter"
	"os"
	"strings"
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/tool"
)

// MockProvider implements models.ModelProvider interface for testing purposes.
type MockProvider struct {
	models         []models.ModelInfo
	chatResponses  map[string]*ChatResponse
	embedResponses map[string][]float64
	mu             sync.RWMutex
	// RecordedToolCalls stores tool calls received from the LLM during tests
	RecordedToolCalls []tool.ToolCall
	// RecordedToolResponses stores tool responses sent to the LLM during tests
	RecordedToolResponses []tool.ToolResponse
}

// ChatResponse holds the configuration for mock chat responses.
type ChatResponse struct {
	// Response is the full response message for non-streaming calls
	Response *models.ChatMessage
	// StreamFragments are the message fragments for streaming calls
	StreamFragments []*models.ChatMessage
	// Error to return (if any)
	Error error
}

// NewMockProvider creates a new mock provider with the given model list.
func NewMockProvider(models []models.ModelInfo) *MockProvider {
	return &MockProvider{
		models:         models,
		chatResponses:  make(map[string]*ChatResponse),
		embedResponses: make(map[string][]float64),
	}
}

// SetChatResponse configures the mock response for a specific model.
func (p *MockProvider) SetChatResponse(modelName string, response *ChatResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.chatResponses[modelName] = response
}

// SetResponsesFromFile configures mock responses for a specific model from a file.
func (p *MockProvider) SetResponsesFromFile(modelName string, filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	prompts := strings.Split(string(content), "@@@@@@@")
	fragments := make([]*models.ChatMessage, len(prompts))
	for i, prompt := range prompts {
		fragments[i] = &models.ChatMessage{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{
				{Text: strings.TrimSpace(prompt)},
			},
		}
	}

	p.SetChatResponse(modelName, &ChatResponse{
		StreamFragments: fragments,
	})
	return nil
}

// SetEmbedResponse configures the mock embedding response for a specific model.
func (p *MockProvider) SetEmbedResponse(modelName string, embedding []float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.embedResponses[modelName] = embedding
}

// ListModels returns the predefined list of models.
func (p *MockProvider) ListModels() ([]models.ModelInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.models, nil
}

// ChatModel returns a MockChatModel implementation for the given model and options.
func (p *MockProvider) ChatModel(model string, options *models.ChatOptions) models.ChatModel {
	return &MockChatModel{
		provider: p,
		model:    model,
		options:  options,
	}
}

// EmbeddingModel returns a MockEmbeddingModel implementation for the given model.
func (p *MockProvider) EmbeddingModel(model string) models.EmbeddingModel {
	return &MockEmbeddingModel{
		provider: p,
		model:    model,
	}
}

// MockChatModel implements models.ChatModel interface for testing purposes.
type MockChatModel struct {
	provider *MockProvider
	model    string
	options  *models.ChatOptions
}

// Chat sends a chat request and returns the full response.
func (m *MockChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) (*models.ChatMessage, error) {
	// Check for cancellation first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Record tool responses from incoming messages
	m.provider.mu.Lock()
	for _, msg := range messages {
		for _, resp := range msg.GetToolResponses() {
			m.provider.RecordedToolResponses = append(m.provider.RecordedToolResponses, *resp)
		}
	}
	m.provider.mu.Unlock()

	m.provider.mu.RLock()
	response := m.provider.chatResponses[m.model]
	m.provider.mu.RUnlock()

	if response == nil {
		// Default response if none configured
		return &models.ChatMessage{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{
				{Text: "mock response"},
			},
		}, nil
	}

	if response.Error != nil {
		return nil, response.Error
	}

	// Record tool calls from response
	if response.Response != nil {
		m.provider.mu.Lock()
		for _, call := range response.Response.GetToolCalls() {
			m.provider.RecordedToolCalls = append(m.provider.RecordedToolCalls, *call)
		}
		m.provider.mu.Unlock()
	}

	return response.Response, nil
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses.
func (m *MockChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) iter.Seq[*models.ChatMessage] {
	return func(yield func(*models.ChatMessage) bool) {
		// Record tool responses from incoming messages
		m.provider.mu.Lock()
		for _, msg := range messages {
			for _, resp := range msg.GetToolResponses() {
				m.provider.RecordedToolResponses = append(m.provider.RecordedToolResponses, *resp)
			}
		}
		m.provider.mu.Unlock()

		m.provider.mu.RLock()
		response := m.provider.chatResponses[m.model]
		m.provider.mu.RUnlock()

		var fragments []*models.ChatMessage

		if response == nil {
			// Default streaming response if none configured
			fragments = []*models.ChatMessage{
				{Role: models.ChatRoleAssistant, Parts: []models.ChatMessagePart{{Text: "mock"}}},
				{Role: models.ChatRoleAssistant, Parts: []models.ChatMessagePart{{Text: " stream"}}},
			}
		} else if response.Error != nil {
			// If there's an error configured, just return without yielding anything
			return
		} else {
			fragments = response.StreamFragments
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
	provider *MockProvider
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
