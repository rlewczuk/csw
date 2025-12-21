package mock

import (
	"context"
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/models"
)

// MockProvider implements models.ModelProvider interface for testing purposes.
type MockProvider struct {
	models         []models.ModelInfo
	chatResponses  map[string]*ChatResponse
	embedResponses map[string][]float64
	mu             sync.RWMutex
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
func (m *MockChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions) (*models.ChatMessage, error) {
	// Check for cancellation first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	m.provider.mu.RLock()
	response := m.provider.chatResponses[m.model]
	m.provider.mu.RUnlock()

	if response == nil {
		// Default response if none configured
		return &models.ChatMessage{
			Role:  models.ChatRoleAssistant,
			Parts: []string{"mock response"},
		}, nil
	}

	if response.Error != nil {
		return nil, response.Error
	}

	return response.Response, nil
}

// ChatStream sends a chat request and returns an iterator for streaming responses.
func (m *MockChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions) (models.ChatStreamIterator, error) {
	m.provider.mu.RLock()
	response := m.provider.chatResponses[m.model]
	m.provider.mu.RUnlock()

	if response == nil {
		// Default streaming response if none configured
		return &MockStreamIterator{
			ctx: ctx,
			fragments: []*models.ChatMessage{
				{Role: models.ChatRoleAssistant, Parts: []string{"mock"}},
				{Role: models.ChatRoleAssistant, Parts: []string{" stream"}},
			},
			index: 0,
		}, nil
	}

	if response.Error != nil {
		return nil, response.Error
	}

	return &MockStreamIterator{
		ctx:       ctx,
		fragments: response.StreamFragments,
		index:     0,
	}, nil
}

// MockStreamIterator implements models.ChatStreamIterator for testing purposes.
type MockStreamIterator struct {
	ctx       context.Context
	fragments []*models.ChatMessage
	index     int
	mu        sync.Mutex
}

// Next returns the next fragment of the chat response.
func (i *MockStreamIterator) Next() (*models.ChatMessage, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Check for cancellation
	select {
	case <-i.ctx.Done():
		return nil, i.ctx.Err()
	default:
	}

	if i.index >= len(i.fragments) {
		return nil, models.ErrEndOfStream
	}

	fragment := i.fragments[i.index]
	i.index++
	return fragment, nil
}

// Close releases any resources associated with the iterator.
func (i *MockStreamIterator) Close() error {
	return nil
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
