package models

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sync"
)

// ModelTagMapping represents a single model-to-tag mapping rule.
// Model names are matched against the Model regexp pattern, and if they match,
// the Tag is assigned to the model.
type ModelTagMapping struct {
	// Model is a regexp pattern to match model names
	Model string `json:"model"`
	// Tag is the tag name to assign to matching models
	Tag string `json:"tag"`
	// compiled is the compiled regexp pattern
	compiled *regexp.Regexp
}

// GlobalConfig represents the global configuration file structure.
// This file is loaded from configs/global.json.
type GlobalConfig struct {
	// ModelTags contains global model-to-tag mappings
	ModelTags []ModelTagMapping `json:"model_tags,omitempty"`
}

// ModelTagRegistry manages model tag assignments from global and provider-specific sources.
type ModelTagRegistry struct {
	mu sync.RWMutex
	// globalMappings contains mappings from the global config
	globalMappings []ModelTagMapping
	// providerMappings contains mappings from provider configs, keyed by provider name
	providerMappings map[string][]ModelTagMapping
}

// NewModelTagRegistry creates a new ModelTagRegistry.
func NewModelTagRegistry() *ModelTagRegistry {
	return &ModelTagRegistry{
		providerMappings: make(map[string][]ModelTagMapping),
	}
}

// LoadGlobalConfig loads model tag mappings from the global configuration file.
func (r *ModelTagRegistry) LoadGlobalConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// If the file doesn't exist, that's fine - just use empty mappings
			return nil
		}
		return fmt.Errorf("ModelTagRegistry.LoadGlobalConfig() [tags.go]: failed to read file: %w", err)
	}

	var config GlobalConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("ModelTagRegistry.LoadGlobalConfig() [tags.go]: failed to parse JSON: %w", err)
	}

	// Compile regexps
	for i := range config.ModelTags {
		compiled, err := regexp.Compile(config.ModelTags[i].Model)
		if err != nil {
			return fmt.Errorf("ModelTagRegistry.LoadGlobalConfig() [tags.go]: invalid regexp %q: %w", config.ModelTags[i].Model, err)
		}
		config.ModelTags[i].compiled = compiled
	}

	r.mu.Lock()
	r.globalMappings = config.ModelTags
	r.mu.Unlock()

	return nil
}

// SetGlobalMappings sets the global model tag mappings.
// This is useful for testing or programmatic configuration.
func (r *ModelTagRegistry) SetGlobalMappings(mappings []ModelTagMapping) error {
	// Compile regexps
	for i := range mappings {
		compiled, err := regexp.Compile(mappings[i].Model)
		if err != nil {
			return fmt.Errorf("ModelTagRegistry.SetGlobalMappings() [tags.go]: invalid regexp %q: %w", mappings[i].Model, err)
		}
		mappings[i].compiled = compiled
	}

	r.mu.Lock()
	r.globalMappings = mappings
	r.mu.Unlock()

	return nil
}

// SetProviderMappings sets the model tag mappings for a specific provider.
func (r *ModelTagRegistry) SetProviderMappings(providerName string, mappings []ModelTagMapping) error {
	// Compile regexps
	for i := range mappings {
		compiled, err := regexp.Compile(mappings[i].Model)
		if err != nil {
			return fmt.Errorf("ModelTagRegistry.SetProviderMappings() [tags.go]: invalid regexp %q: %w", mappings[i].Model, err)
		}
		mappings[i].compiled = compiled
	}

	r.mu.Lock()
	r.providerMappings[providerName] = mappings
	r.mu.Unlock()

	return nil
}

// GetTagsForModel returns all tags that match the given model name.
// Tags are collected from both global mappings and provider-specific mappings.
// The result is the union of all matching tags.
func (r *ModelTagRegistry) GetTagsForModel(providerName, modelName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tagSet := make(map[string]bool)

	// Match against global mappings first
	for _, mapping := range r.globalMappings {
		if mapping.compiled != nil && mapping.compiled.MatchString(modelName) {
			tagSet[mapping.Tag] = true
		}
	}

	// Match against provider-specific mappings
	if providerMappings, ok := r.providerMappings[providerName]; ok {
		for _, mapping := range providerMappings {
			if mapping.compiled != nil && mapping.compiled.MatchString(modelName) {
				tagSet[mapping.Tag] = true
			}
		}
	}

	// Convert set to slice
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	return tags
}

// GetAllProviderNames returns a list of all provider names that have mappings.
func (r *ModelTagRegistry) GetAllProviderNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providerMappings))
	for name := range r.providerMappings {
		names = append(names, name)
	}
	return names
}
