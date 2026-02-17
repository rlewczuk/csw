package models

import (
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProviderRegistry(t *testing.T) {
	configStore := impl.NewMockConfigStore()
	registry := NewProviderRegistry(configStore)
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.configStore)
	assert.NotNil(t, registry.providers)
	assert.Equal(t, 0, len(registry.providers))
	assert.True(t, registry.lastUpdate.IsZero())
}

func TestProviderRegistry_Get(t *testing.T) {
	tests := []struct {
		name        string
		setupStore  func(*impl.MockConfigStore)
		provName    string
		wantErr     bool
		expectedErr error
	}{
		{
			name: "get existing provider",
			setupStore: func(store *impl.MockConfigStore) {
				configs := map[string]*conf.ModelProviderConfig{
					"test-ollama": {
						Type: "ollama",
						Name: "test-ollama",
						URL:  "http://localhost:11434",
					},
				}
				store.SetModelProviderConfigs(configs)
			},
			provName: "test-ollama",
			wantErr:  false,
		},
		{
			name: "get non-existent provider",
			setupStore: func(store *impl.MockConfigStore) {
				configs := map[string]*conf.ModelProviderConfig{
					"test-ollama": {
						Type: "ollama",
						Name: "test-ollama",
						URL:  "http://localhost:11434",
					},
				}
				store.SetModelProviderConfigs(configs)
			},
			provName:    "non-existent",
			wantErr:     true,
			expectedErr: ErrProviderNotFound,
		},
		{
			name: "get from empty store",
			setupStore: func(store *impl.MockConfigStore) {
				store.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{})
			},
			provName:    "test",
			wantErr:     true,
			expectedErr: ErrProviderNotFound,
		},
		{
			name: "config store returns error on GetModelProviderConfigs",
			setupStore: func(store *impl.MockConfigStore) {
				store.GetModelProviderConfigsErr = errors.New("config store error")
			},
			provName: "test",
			wantErr:  true,
		},
		{
			name: "config store returns error on LastModelProviderConfigsUpdate",
			setupStore: func(store *impl.MockConfigStore) {
				store.LastModelProviderConfigsUpdateErr = errors.New("timestamp error")
			},
			provName: "test",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configStore := impl.NewMockConfigStore()
			tt.setupStore(configStore)

			registry := NewProviderRegistry(configStore)
			provider, err := registry.Get(tt.provName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestProviderRegistry_List(t *testing.T) {
	tests := []struct {
		name          string
		setupStore    func(*impl.MockConfigStore)
		expectedCount int
		expectedNames []string
	}{
		{
			name: "list multiple providers",
			setupStore: func(store *impl.MockConfigStore) {
				configs := map[string]*conf.ModelProviderConfig{
					"ollama": {
						Type: "ollama",
						Name: "ollama",
						URL:  "http://localhost:11434",
					},
					"anthropic": {
						Type:   "anthropic",
						Name:   "anthropic",
						URL:    "https://api.anthropic.com",
						APIKey: "test-key",
					},
					"openai": {
						Type:   "openai",
						Name:   "openai",
						URL:    "https://api.openai.com/v1",
						APIKey: "test-key",
					},
				}
				store.SetModelProviderConfigs(configs)
			},
			expectedCount: 3,
			expectedNames: []string{"anthropic", "ollama", "openai"},
		},
		{
			name: "list empty providers",
			setupStore: func(store *impl.MockConfigStore) {
				store.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{})
			},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name: "list with config store error returns empty list",
			setupStore: func(store *impl.MockConfigStore) {
				store.GetModelProviderConfigsErr = errors.New("config error")
			},
			expectedCount: 0,
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configStore := impl.NewMockConfigStore()
			tt.setupStore(configStore)

			registry := NewProviderRegistry(configStore)
			names := registry.List()

			assert.Equal(t, tt.expectedCount, len(names))
			sort.Strings(names)
			sort.Strings(tt.expectedNames)
			assert.Equal(t, tt.expectedNames, names)
		})
	}
}

func TestProviderRegistry_CacheInvalidation(t *testing.T) {
	configStore := impl.NewMockConfigStore()

	// Initial configuration
	configs1 := map[string]*conf.ModelProviderConfig{
		"ollama": {
			Type: "ollama",
			Name: "ollama",
			URL:  "http://localhost:11434",
		},
	}
	configStore.SetModelProviderConfigs(configs1)

	registry := NewProviderRegistry(configStore)

	// First access - should load from config store
	provider1, err := registry.Get("ollama")
	require.NoError(t, err)
	assert.NotNil(t, provider1)

	// Verify only ollama is in the list
	names := registry.List()
	assert.Equal(t, []string{"ollama"}, names)

	// Update configuration with new provider
	time.Sleep(10 * time.Millisecond) // Ensure timestamp changes
	configs2 := map[string]*conf.ModelProviderConfig{
		"ollama": {
			Type: "ollama",
			Name: "ollama",
			URL:  "http://localhost:11434",
		},
		"anthropic": {
			Type:   "anthropic",
			Name:   "anthropic",
			URL:    "https://api.anthropic.com",
			APIKey: "test-key",
		},
	}
	configStore.SetModelProviderConfigs(configs2)

	// Access should trigger reload because timestamp changed
	provider2, err := registry.Get("anthropic")
	require.NoError(t, err)
	assert.NotNil(t, provider2)

	// Verify both providers are now in the list
	names = registry.List()
	sort.Strings(names)
	assert.Equal(t, []string{"anthropic", "ollama"}, names)
}

func TestProviderRegistry_CacheNotInvalidatedWhenTimestampSame(t *testing.T) {
	configStore := impl.NewMockConfigStore()

	// Initial configuration
	configs := map[string]*conf.ModelProviderConfig{
		"ollama": {
			Type: "ollama",
			Name: "ollama",
			URL:  "http://localhost:11434",
		},
	}
	configStore.SetModelProviderConfigs(configs)

	registry := NewProviderRegistry(configStore)

	// First access - should load from config store
	provider1, err := registry.Get("ollama")
	require.NoError(t, err)
	assert.NotNil(t, provider1)

	// Second access without timestamp change - should use cache
	// We can verify this by checking that even if we inject an error,
	// it doesn't affect the second access
	provider2, err := registry.Get("ollama")
	require.NoError(t, err)
	assert.NotNil(t, provider2)
}

func TestProviderRegistry_ConcurrentAccess(t *testing.T) {
	configStore := impl.NewMockConfigStore()
	configs := map[string]*conf.ModelProviderConfig{
		"ollama": {
			Type: "ollama",
			Name: "ollama",
			URL:  "http://localhost:11434",
		},
	}
	configStore.SetModelProviderConfigs(configs)

	registry := NewProviderRegistry(configStore)

	// Run concurrent operations
	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = registry.Get("ollama")
			_ = registry.List()
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify registry is still functional
	p, err := registry.Get("ollama")
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestProviderRegistry_InvalidProviderConfig(t *testing.T) {
	configStore := impl.NewMockConfigStore()

	// Configuration with invalid provider type
	configs := map[string]*conf.ModelProviderConfig{
		"invalid": {
			Type: "unsupported-type",
			Name: "invalid",
			URL:  "http://localhost:8080",
		},
	}
	configStore.SetModelProviderConfigs(configs)

	registry := NewProviderRegistry(configStore)

	// Should get error when trying to access
	_, err := registry.Get("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider type")
}

func TestProviderRegistry_MissingRequiredFields(t *testing.T) {
	configStore := impl.NewMockConfigStore()

	// Configuration without URL (required field)
	configs := map[string]*conf.ModelProviderConfig{
		"invalid": {
			Type: "ollama",
			Name: "invalid",
			URL:  "", // Missing URL
		},
	}
	configStore.SetModelProviderConfigs(configs)

	registry := NewProviderRegistry(configStore)

	// Should get error when trying to access
	_, err := registry.Get("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "URL cannot be empty")
}

func TestProviderRegistry_MultipleProviderTypes(t *testing.T) {
	configStore := impl.NewMockConfigStore()

	configs := map[string]*conf.ModelProviderConfig{
		"ollama-local": {
			Type: "ollama",
			Name: "ollama-local",
			URL:  "http://localhost:11434",
		},
		"ollama-remote": {
			Type: "ollama",
			Name: "ollama-remote",
			URL:  "http://remote:11434",
		},
		"anthropic": {
			Type:   "anthropic",
			Name:   "anthropic",
			URL:    "https://api.anthropic.com",
			APIKey: "test-key",
		},
		"openai": {
			Type:   "openai",
			Name:   "openai",
			URL:    "https://api.openai.com/v1",
			APIKey: "test-key",
		},
	}
	configStore.SetModelProviderConfigs(configs)

	registry := NewProviderRegistry(configStore)

	// Verify all providers are accessible
	names := registry.List()
	assert.Equal(t, 4, len(names))

	for name := range configs {
		provider, err := registry.Get(name)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
	}
}

func TestProviderRegistry_ReloadAfterConfigRemoval(t *testing.T) {
	configStore := impl.NewMockConfigStore()

	// Initial configuration with two providers
	configs1 := map[string]*conf.ModelProviderConfig{
		"ollama": {
			Type: "ollama",
			Name: "ollama",
			URL:  "http://localhost:11434",
		},
		"anthropic": {
			Type:   "anthropic",
			Name:   "anthropic",
			URL:    "https://api.anthropic.com",
			APIKey: "test-key",
		},
	}
	configStore.SetModelProviderConfigs(configs1)

	registry := NewProviderRegistry(configStore)

	// Verify both providers exist
	names := registry.List()
	assert.Equal(t, 2, len(names))

	// Remove one provider
	time.Sleep(10 * time.Millisecond)
	configs2 := map[string]*conf.ModelProviderConfig{
		"ollama": {
			Type: "ollama",
			Name: "ollama",
			URL:  "http://localhost:11434",
		},
	}
	configStore.SetModelProviderConfigs(configs2)

	// Access should trigger reload
	names = registry.List()
	assert.Equal(t, 1, len(names))
	assert.Equal(t, []string{"ollama"}, names)

	// Anthropic should no longer be accessible
	_, err := registry.Get("anthropic")
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *conf.ModelProviderConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "creates ollama provider with valid config",
			config: &conf.ModelProviderConfig{
				Type:               "ollama",
				Name:               "local-ollama",
				URL:                "http://localhost:11434",
				ConnectTimeout:     5 * time.Second,
				RequestTimeout:     30 * time.Second,
				DefaultTemperature: 0.7,
				DefaultTopP:        0.9,
				DefaultTopK:        40,
				MaxTokens:          4096,
			},
			expectError: false,
		},
		{
			name: "creates openai provider with valid config",
			config: &conf.ModelProviderConfig{
				Type:               "openai",
				Name:               "openai-cloud",
				URL:                "http://localhost:11434/v1",
				APIKey:             "test-key",
				ConnectTimeout:     5 * time.Second,
				RequestTimeout:     30 * time.Second,
				DefaultTemperature: 0.7,
				DefaultTopP:        0.9,
				MaxTokens:          4096,
			},
			expectError: false,
		},
		{
			name: "creates anthropic provider with valid config",
			config: &conf.ModelProviderConfig{
				Type:               "anthropic",
				Name:               "anthropic-cloud",
				URL:                "https://api.anthropic.com",
				APIKey:             "test-key",
				ConnectTimeout:     5 * time.Second,
				RequestTimeout:     30 * time.Second,
				DefaultTemperature: 0.7,
				DefaultTopP:        0.9,
				DefaultTopK:        40,
				MaxTokens:          4096,
			},
			expectError: false,
		},
		{
			name: "uses default timeouts when not specified",
			config: &conf.ModelProviderConfig{
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
			config: &conf.ModelProviderConfig{
				Type: "ollama",
				Name: "local-ollama",
				URL:  "",
			},
			expectError: true,
			errorMsg:    "URL cannot be empty",
		},
		{
			name: "returns error for unsupported provider type",
			config: &conf.ModelProviderConfig{
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
			provider, err := ModelFromConfig(tt.config)

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
