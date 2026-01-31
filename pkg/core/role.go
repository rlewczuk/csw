package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
)

// AgentRoleRegistry manages agent role configurations loaded from a ConfigStore.
// It implements caching with automatic invalidation based on config update timestamps.
type AgentRoleRegistry struct {
	configStore  conf.ConfigStore
	mu           sync.RWMutex
	cache        map[string]conf.AgentRoleConfig
	lastUpdate   time.Time
	cacheInvalid bool
}

// NewAgentRoleRegistry creates a new AgentRoleRegistry with the given ConfigStore.
func NewAgentRoleRegistry(configStore conf.ConfigStore) *AgentRoleRegistry {
	return &AgentRoleRegistry{
		configStore:  configStore,
		cache:        make(map[string]conf.AgentRoleConfig),
		cacheInvalid: true, // Force initial load
	}
}

// refreshCacheIfNeeded checks if the cache needs to be refreshed and does so if necessary.
// Must be called with r.mu held for writing.
func (r *AgentRoleRegistry) refreshCacheIfNeeded() error {
	// Check timestamp to determine if cache is stale
	lastConfigUpdate, err := r.configStore.LastAgentRoleConfigsUpdate()
	if err != nil {
		return fmt.Errorf("AgentRoleRegistry.refreshCacheIfNeeded() [role.go]: failed to get last update timestamp: %w", err)
	}

	// If cache is valid and timestamps match, no refresh needed
	if !r.cacheInvalid && !lastConfigUpdate.After(r.lastUpdate) {
		return nil
	}

	// Fetch fresh configs from store
	configs, err := r.configStore.GetAgentRoleConfigs()
	if err != nil {
		return fmt.Errorf("AgentRoleRegistry.refreshCacheIfNeeded() [role.go]: failed to get agent role configs: %w", err)
	}

	// Update cache, merging "all" role into other roles
	r.cache = make(map[string]conf.AgentRoleConfig, len(configs))

	// Get the "all" role config if it exists
	var allConfig *conf.AgentRoleConfig
	if all, ok := configs["all"]; ok {
		allConfig = all
	}

	for name, config := range configs {
		if config != nil {
			configCopy := *config

			// Merge "all" role's HiddenPatterns into this role (unless it's the "all" role itself)
			if name != "all" && allConfig != nil {
				// Prepend "all" role's HiddenPatterns
				if allConfig.HiddenPatterns != nil {
					merged := make([]string, 0, len(allConfig.HiddenPatterns)+len(configCopy.HiddenPatterns))
					merged = append(merged, allConfig.HiddenPatterns...)
					merged = append(merged, configCopy.HiddenPatterns...)
					configCopy.HiddenPatterns = merged
				}
			}

			r.cache[name] = configCopy
		}
	}

	r.lastUpdate = lastConfigUpdate
	r.cacheInvalid = false

	return nil
}

// Get returns a role by name and a boolean indicating if it was found.
// It automatically refreshes the cache if the configuration has been updated.
func (r *AgentRoleRegistry) Get(name string) (conf.AgentRoleConfig, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refresh cache if needed
	if err := r.refreshCacheIfNeeded(); err != nil {
		// Log error but return not found rather than panicking
		// In production code, consider proper logging
		return conf.AgentRoleConfig{}, false
	}

	role, ok := r.cache[name]
	return role, ok
}

// List returns all role names in the registry.
// It automatically refreshes the cache if the configuration has been updated.
func (r *AgentRoleRegistry) List() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refresh cache if needed
	if err := r.refreshCacheIfNeeded(); err != nil {
		// Log error but return empty list rather than panicking
		// In production code, consider proper logging
		return []string{}
	}

	names := make([]string, 0, len(r.cache))
	for name := range r.cache {
		names = append(names, name)
	}
	return names
}
