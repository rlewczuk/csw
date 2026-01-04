package models

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromConfig(t *testing.T) {
	// Setup: Register mock providers for testing
	RegisterProvider("ollama", func(baseURL string, options *ModelConnectionOptions) (ModelProvider, error) {
		return &mockProviderForTest{}, nil
	})
	RegisterProvider("openai", func(baseURL string, options *ModelConnectionOptions) (ModelProvider, error) {
		return &mockProviderForTest{}, nil
	})
	RegisterProvider("anthropic", func(baseURL string, options *ModelConnectionOptions) (ModelProvider, error) {
		return &mockProviderForTest{}, nil
	})
	tests := []struct {
		name        string
		config      *ModelProviderConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "creates ollama provider with valid config",
			config: &ModelProviderConfig{
				Type:               "ollama",
				Name:               "local-ollama",
				URL:                "http://localhost:11434",
				ConnectTimeout:     5 * time.Second,
				RequestTimeout:     30 * time.Second,
				DefaultTemperature: 0.7,
				DefaultTopP:        0.9,
				DefaultTopK:        40,
				ContextLengthLimit: 4096,
			},
			expectError: false,
		},
		{
			name: "creates openai provider with valid config",
			config: &ModelProviderConfig{
				Type:               "openai",
				Name:               "openai-cloud",
				URL:                "http://localhost:11434/v1",
				APIKey:             "test-key",
				ConnectTimeout:     5 * time.Second,
				RequestTimeout:     30 * time.Second,
				DefaultTemperature: 0.7,
				DefaultTopP:        0.9,
				ContextLengthLimit: 8192,
			},
			expectError: false,
		},
		{
			name: "creates anthropic provider with valid config",
			config: &ModelProviderConfig{
				Type:               "anthropic",
				Name:               "anthropic-cloud",
				URL:                "https://api.anthropic.com",
				APIKey:             "test-key",
				ConnectTimeout:     5 * time.Second,
				RequestTimeout:     30 * time.Second,
				DefaultTemperature: 0.7,
				DefaultTopP:        0.9,
				DefaultTopK:        40,
				ContextLengthLimit: 100000,
			},
			expectError: false,
		},
		{
			name: "uses default timeouts when not specified",
			config: &ModelProviderConfig{
				Type: "ollama",
				Name: "local-ollama",
				URL:  "http://localhost:11434",
			},
			expectError: false,
		},
		{
			name:        "returns error for nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "config cannot be nil",
		},
		{
			name: "returns error for empty URL",
			config: &ModelProviderConfig{
				Type: "ollama",
				Name: "local-ollama",
				URL:  "",
			},
			expectError: true,
			errorMsg:    "URL cannot be empty",
		},
		{
			name: "returns error for unsupported provider type",
			config: &ModelProviderConfig{
				Type: "unsupported",
				Name: "test",
				URL:  "http://localhost:11434",
			},
			expectError: true,
			errorMsg:    "unsupported provider type: unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := FromConfig(tt.config)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, provider)
			} else {
				require.NoError(t, err)
				require.NotNil(t, provider)

				// Verify the provider can create chat models
				chatModel := provider.ChatModel("test-model", nil)
				assert.NotNil(t, chatModel)

				// Verify the provider can create embedding models
				embedModel := provider.EmbeddingModel("test-model")
				assert.NotNil(t, embedModel)
			}
		})
	}
}

func TestRegisterProvider(t *testing.T) {
	t.Run("registers custom provider factory", func(t *testing.T) {
		// Save original registry
		originalRegistry := make(map[string]ProviderFactory)
		for k, v := range providerRegistry {
			originalRegistry[k] = v
		}
		defer func() {
			// Restore original registry
			providerRegistry = originalRegistry
		}()

		// Register a custom provider
		customProviderCalled := false
		RegisterProvider("custom", func(baseURL string, options *ModelConnectionOptions) (ModelProvider, error) {
			customProviderCalled = true
			// Return a mock provider for testing
			return &mockProviderForTest{}, nil
		})

		// Create provider from config
		config := &ModelProviderConfig{
			Type: "custom",
			Name: "custom-provider",
			URL:  "http://localhost:8080",
		}

		provider, err := FromConfig(config)
		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.True(t, customProviderCalled)
	})
}

// mockProviderForTest is a minimal mock provider for testing registration
type mockProviderForTest struct{}

func (m *mockProviderForTest) ListModels() ([]ModelInfo, error) {
	return nil, nil
}

func (m *mockProviderForTest) ChatModel(model string, options *ChatOptions) ChatModel {
	return &mockChatModelForTest{}
}

func (m *mockProviderForTest) EmbeddingModel(model string) EmbeddingModel {
	return &mockEmbeddingModelForTest{}
}

// mockChatModelForTest is a minimal mock chat model for testing
type mockChatModelForTest struct{}

func (m *mockChatModelForTest) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	return nil, nil
}

func (m *mockChatModelForTest) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {}
}

// mockEmbeddingModelForTest is a minimal mock embedding model for testing
type mockEmbeddingModelForTest struct{}

func (m *mockEmbeddingModelForTest) Embed(ctx context.Context, input string) ([]float64, error) {
	return nil, nil
}
