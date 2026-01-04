package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ErrProviderNotFound      = errors.New("provider not found")
	ErrProviderAlreadyExists = errors.New("provider already exists")
)

// ProviderRegistry manages a collection of model providers.
// It allows registration, retrieval, and listing of providers.
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]ModelProvider
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]ModelProvider),
	}
}

// Register registers a provider with the given name.
// It returns an error if a provider with the same name already exists.
func (r *ProviderRegistry) Register(name string, provider ModelProvider) error {
	if name == "" {
		return errors.New("provider name cannot be empty")
	}
	if provider == nil {
		return errors.New("provider cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return ErrProviderAlreadyExists
	}

	r.providers[name] = provider
	return nil
}

// Get retrieves a provider by name.
// It returns an error if the provider is not found.
func (r *ProviderRegistry) Get(name string) (ModelProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, ErrProviderNotFound
	}

	return provider, nil
}

// List returns a list of all registered provider names.
func (r *ProviderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// LoadFromDirectory loads provider configurations from JSON files in the specified directory.
// Each JSON file should contain a ModelProviderConfig.
// The provider name is derived from the filename (without extension).
// If the Name field in the JSON is empty or doesn't match the filename, it will be set to the filename.
func (r *ProviderRegistry) LoadFromDirectory(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .json files
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Extract provider name from filename
		fileName := entry.Name()
		providerName := strings.TrimSuffix(fileName, ".json")

		// Read and parse the config file
		filePath := filepath.Join(dirPath, fileName)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read config file %s: %w", filePath, err)
		}

		var config ModelProviderConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse config file %s: %w", filePath, err)
		}

		// Set or override the name to match the filename
		if config.Name == "" || config.Name != providerName {
			config.Name = providerName
		}

		// Create the provider from config
		provider, err := FromConfig(&config)
		if err != nil {
			return fmt.Errorf("failed to create provider from config file %s: %w", filePath, err)
		}

		// Register the provider
		if err := r.Register(providerName, provider); err != nil {
			return fmt.Errorf("failed to register provider %s: %w", providerName, err)
		}
	}

	return nil
}
