package models

import (
	"fmt"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
)

// ProviderRegistry manages a collection of model providers.
// It loads provider configurations from a ConfigStore and caches the created providers.
// The cache is invalidated when the ConfigStore reports that configurations have changed.
type ProviderRegistry struct {
	mu          sync.RWMutex
	configStore conf.ConfigStore
	providers   map[string]ModelProvider
	lastUpdate  time.Time
}

// NewProviderRegistry creates a new provider registry that uses the given ConfigStore.
// Providers are loaded lazily when accessed via Get() or List().
func NewProviderRegistry(configStore conf.ConfigStore) *ProviderRegistry {
	return &ProviderRegistry{
		configStore: configStore,
		providers:   make(map[string]ModelProvider),
		lastUpdate:  time.Time{},
	}
}

// ensureLoaded checks if the provider cache needs to be refreshed and reloads if necessary.
// It must be called with the write lock held.
func (r *ProviderRegistry) ensureLoaded() error {
	// Get the last update timestamp from config store
	lastConfigUpdate, err := r.configStore.LastModelProviderConfigsUpdate()
	if err != nil {
		return fmt.Errorf("ProviderRegistry.ensureLoaded() [providers.go]: failed to get last update timestamp: %w", err)
	}

	// If cache is up to date, no need to reload
	if !r.lastUpdate.IsZero() && !lastConfigUpdate.After(r.lastUpdate) {
		return nil
	}

	// Load configurations from config store
	configs, err := r.configStore.GetModelProviderConfigs()
	if err != nil {
		return fmt.Errorf("ProviderRegistry.ensureLoaded() [providers.go]: failed to load provider configs: %w", err)
	}

	// Clear existing providers
	r.providers = make(map[string]ModelProvider)

	// Create providers from configurations
	for name, config := range configs {
		provider, err := ModelFromConfig(config)
		if err != nil {
			return fmt.Errorf("ProviderRegistry.ensureLoaded() [providers.go]: failed to create provider %s: %w", name, err)
		}
		r.providers[name] = provider
	}

	// Update the last loaded timestamp
	r.lastUpdate = lastConfigUpdate

	return nil
}

// Get retrieves a provider by name.
// It loads providers from the ConfigStore if the cache is stale or empty.
// It returns an error if the provider is not found.
func (r *ProviderRegistry) Get(name string) (ModelProvider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure providers are loaded and up to date
	if err := r.ensureLoaded(); err != nil {
		return nil, err
	}

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("ProviderRegistry.Get() [providers.go]: %w", ErrProviderNotFound)
	}

	return provider, nil
}

// List returns a list of all registered provider names.
// It loads providers from the ConfigStore if the cache is stale or empty.
func (r *ProviderRegistry) List() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure providers are loaded and up to date
	// Ignore errors here - just return empty list
	_ = r.ensureLoaded()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ModelFromConfig creates a new ModelProvider instance from the configuration.
// It automatically selects the right implementation based on the Type field.
func ModelFromConfig(config *conf.ModelProviderConfig) (ModelProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("ModelFromConfig() [config.go]: config cannot be nil")
	}

	if config.URL == "" {
		return nil, fmt.Errorf("ModelFromConfig() [config.go]: URL cannot be empty")
	}

	// Set defaults if not specified
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 60 * time.Second
	}

	// Call factory function directly based on provider type
	switch config.Type {
	case "ollama":
		return NewOllamaClient(config)
	case "openai":
		return NewOpenAIClient(config)
	case "anthropic":
		return NewAnthropicClient(config)
	case "responses":
		return NewResponsesClient(config)
	case "jetbrains":
		return NewJetBrainsClient(config)
	default:
		return nil, fmt.Errorf("ModelFromConfig() [config.go]: unsupported provider type: %s", config.Type)
	}
}
