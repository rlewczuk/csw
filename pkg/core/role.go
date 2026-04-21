package core

import (
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/conf"
)

// AgentRoleRegistry manages agent role configurations loaded from a CswConfig.
// It caches loaded roles for quick lookup.
type AgentRoleRegistry struct {
	config       *conf.CswConfig
	mu           sync.RWMutex
	cache        map[string]conf.AgentRoleConfig
	loaded       bool
}

func normalizeRoleLookupName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// NewAgentRoleRegistry creates a new AgentRoleRegistry with the given CswConfig.
func NewAgentRoleRegistry(config *conf.CswConfig) *AgentRoleRegistry {
	if config == nil {
		config = &conf.CswConfig{}
	}

	return &AgentRoleRegistry{
		config:       config,
		cache:        make(map[string]conf.AgentRoleConfig),
		loaded:       false,
	}
}

// refreshCacheIfNeeded checks if the cache needs to be refreshed and does so if necessary.
// Must be called with r.mu held for writing.
func (r *AgentRoleRegistry) refreshCacheIfNeeded() error {
	if r.loaded {
		return nil
	}

	configs := r.config.AgentRoleConfigs
	if configs == nil {
		configs = map[string]*conf.AgentRoleConfig{}
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

			normalizedRoleName := normalizeRoleLookupName(configCopy.Name)
			if normalizedRoleName != "" {
				r.cache[normalizedRoleName] = configCopy
			}

			for _, alias := range configCopy.Aliases {
				normalizedAlias := normalizeRoleLookupName(alias)
				if normalizedAlias == "" {
					continue
				}
				r.cache[normalizedAlias] = configCopy
			}
		}
	}

	r.loaded = true

	return nil
}

// Get returns a role by name and a boolean indicating if it was found.
func (r *AgentRoleRegistry) Get(name string) (conf.AgentRoleConfig, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refresh cache if needed
	if err := r.refreshCacheIfNeeded(); err != nil {
		// Log error but return not found rather than panicking
		// In production code, consider proper logging
		return conf.AgentRoleConfig{}, false
	}

	lookupName := normalizeRoleLookupName(name)
	role, ok := r.cache[lookupName]
	if ok {
		return role, ok
	}

	role, ok = r.cache[name]
	return role, ok
}

// List returns all role names in the registry.
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
	seen := make(map[string]struct{}, len(r.cache))
	for _, role := range r.cache {
		canonicalName := strings.TrimSpace(role.Name)
		if canonicalName == "" {
			continue
		}
		if _, exists := seen[canonicalName]; exists {
			continue
		}
		seen[canonicalName] = struct{}{}
		names = append(names, canonicalName)
	}
	return names
}
