package models

import (
	"errors"
	"time"
)

// ModelProviderConfig represents common configuration for model providers.
type ModelProviderConfig struct {
	// Type specifies the provider type (e.g., "ollama", "openai", "anthropic")
	Type string
	// Name is a user-friendly name for the provider instance
	Name string
	// URL is the base URL for the provider's API
	URL string
	// APIKey is the API key for authentication (if required)
	APIKey string
	// ConnectTimeout is the timeout for establishing connections
	ConnectTimeout time.Duration
	// RequestTimeout is the timeout for complete requests
	RequestTimeout time.Duration
	// DefaultTemperature is the default temperature for chat completions
	DefaultTemperature float32
	// DefaultTopP is the default top_p for chat completions
	DefaultTopP float32
	// DefaultTopK is the default top_k for chat completions
	DefaultTopK int
	// ContextLengthLimit is the maximum context length in tokens
	ContextLengthLimit int
}

// FromConfig creates a new ModelProvider instance from the configuration.
// It automatically selects the right implementation based on the Type field.
func FromConfig(config *ModelProviderConfig) (ModelProvider, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if config.URL == "" {
		return nil, errors.New("URL cannot be empty")
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
	default:
		return nil, errors.New("unsupported provider type: " + config.Type)
	}
}
