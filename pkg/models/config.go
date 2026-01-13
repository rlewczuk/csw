package models

import (
	"fmt"
	"time"
)

// ModelProviderConfig represents common configuration for model providers.
type ModelProviderConfig struct {
	// Type specifies the provider type (e.g., "ollama", "openai", "anthropic")
	Type string `json:"type"`
	// Name is a short computer-friendly name for the provider instance (eg. `ollama-local`)
	Name string `json:"name"`
	// Description is a user-friendly description for the provider instance
	Description string `json:"description,omitempty"`
	// URL is the base URL for the provider's API
	URL string `json:"url"`
	// APIKey is the API key for authentication (if required)
	APIKey string `json:"api_key,omitempty"`
	// ConnectTimeout is the timeout for establishing connections
	ConnectTimeout time.Duration `json:"connect_timeout,omitempty"`
	// RequestTimeout is the timeout for complete requests
	RequestTimeout time.Duration `json:"request_timeout,omitempty"`
	// DefaultTemperature is the default temperature for chat completions
	DefaultTemperature float32 `json:"default_temperature,omitempty"`
	// DefaultTopP is the default top_p for chat completions
	DefaultTopP float32 `json:"default_top_p,omitempty"`
	// DefaultTopK is the default top_k for chat completions
	DefaultTopK int `json:"default_top_k,omitempty"`
	// ContextLengthLimit is the maximum context length in tokens
	ContextLengthLimit int `json:"context_length_limit,omitempty"`
}

// FromConfig creates a new ModelProvider instance from the configuration.
// It automatically selects the right implementation based on the Type field.
func FromConfig(config *ModelProviderConfig) (ModelProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("FromConfig() [config.go]: config cannot be nil")
	}

	if config.URL == "" {
		return nil, fmt.Errorf("FromConfig() [config.go]: URL cannot be empty")
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
		return nil, fmt.Errorf("FromConfig() [config.go]: unsupported provider type: %s", config.Type)
	}
}
