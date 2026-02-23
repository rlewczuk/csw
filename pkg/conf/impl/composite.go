// Package impl provides a composite configuration store implementation
// that merges configurations from multiple sources.
//
// The composite config store loads and merges configurations from multiple sources
// in a specific order, where subsequent sources override previous ones entirely.
//
// Configuration sources are specified using CSW_CONF_PATH environment variable as a
// colon-separated list of paths with special tokens:
//   - @DEFAULTS - embedded configuration (use NewEmbeddedConfigStore())
//   - ~/path - path relative to user home directory
//   - ./path - path relative to current working directory
//   - @PROJ/path - path relative to project root directory
//   - /path/ - local filesystem directory (trailing slash)
//
// Example:
//
//	store, err := NewCompositeConfigStore("/path/to/project", "@DEFAULTS:/etc/csw/:~/.config/csw/:@PROJ/.csw/")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	globalConfig, err := store.GetGlobalConfig()
//	providers, err := store.GetModelProviderConfigs()
//	roles, err := store.GetAgentRoleConfigs()
package impl

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
)

// CompositeConfigStore implements conf.ConfigStore interface for multiple configuration sources.
// It merges configurations from multiple sources, where later sources override earlier ones.
type CompositeConfigStore struct {
	mu      sync.RWMutex
	stores  []conf.ConfigStore
	projDir string

	// Cached configuration
	globalConfig               *conf.GlobalConfig
	globalConfigUpdate         time.Time
	modelProviderConfigs       map[string]*conf.ModelProviderConfig
	modelProviderConfigsUpdate time.Time
	agentRoleConfigs           map[string]*conf.AgentRoleConfig
	agentRoleConfigsUpdate     time.Time

	// Track last known update times from stores
	storeGlobalUpdates []time.Time
	storeModelUpdates  []time.Time
	storeRoleUpdates   []time.Time
}

// NewCompositeConfigStore creates a new CompositeConfigStore that merges configurations
// from multiple sources specified in configPath.
//
// Parameters:
//   - projDir: The project root directory for @PROJ/ path expansion
//   - configPath: Colon-separated list of configuration source paths with special tokens
//
// Returns a ConfigStore that provides a merged view of all configuration sources.
func NewCompositeConfigStore(projDir string, configPath string) (conf.ConfigStore, error) {
	if projDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("NewCompositeConfigStore(): failed to get working directory: %w", err)
		}
		projDir = cwd
	}

	// Parse config path and create stores
	paths := parseConfigPath(configPath)
	if len(paths) == 0 {
		return nil, fmt.Errorf("NewCompositeConfigStore(): no configuration paths specified")
	}

	stores := make([]conf.ConfigStore, 0, len(paths))
	for _, path := range paths {
		store, err := createConfigStore(path, projDir)
		if err != nil {
			// Clean up already created stores
			for _, s := range stores {
				if closer, ok := s.(interface{ Close() error }); ok {
					closer.Close()
				}
			}
			return nil, fmt.Errorf("NewCompositeConfigStore(): failed to create store for path %s: %w", path, err)
		}
		if store != nil {
			stores = append(stores, store)
		}
	}

	if len(stores) == 0 {
		return nil, fmt.Errorf("NewCompositeConfigStore(): no valid configuration stores created")
	}

	composite := &CompositeConfigStore{
		stores:             stores,
		projDir:            projDir,
		storeGlobalUpdates: make([]time.Time, len(stores)),
		storeModelUpdates:  make([]time.Time, len(stores)),
		storeRoleUpdates:   make([]time.Time, len(stores)),
	}

	// Initial load
	if err := composite.refresh(); err != nil {
		return nil, fmt.Errorf("NewCompositeConfigStore(): failed to load initial configuration: %w", err)
	}

	return composite, nil
}

// GetGlobalConfig returns the merged global configuration from all sources.
func (c *CompositeConfigStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to refresh
	if c.needsRefreshGlobal() {
		if err := c.refreshGlobalConfig(); err != nil {
			return nil, fmt.Errorf("CompositeConfigStore.GetGlobalConfig(): refresh failed: %w", err)
		}
	}

	// Return a copy to prevent external modification
	config := &conf.GlobalConfig{
		DefaultProvider: c.globalConfig.DefaultProvider,
		DefaultRole:     c.globalConfig.DefaultRole,
		ModelTags:       make([]conf.ModelTagMapping, len(c.globalConfig.ModelTags)),
		ToolSelection: conf.ToolSelectionConfig{
			Default: make(map[string]bool, len(c.globalConfig.ToolSelection.Default)),
			Tags:    make(map[string]map[string]bool, len(c.globalConfig.ToolSelection.Tags)),
		},
	}
	copy(config.ModelTags, c.globalConfig.ModelTags)
	for toolName, enabled := range c.globalConfig.ToolSelection.Default {
		config.ToolSelection.Default[toolName] = enabled
	}
	for tag, tools := range c.globalConfig.ToolSelection.Tags {
		copiedTools := make(map[string]bool, len(tools))
		for toolName, enabled := range tools {
			copiedTools[toolName] = enabled
		}
		config.ToolSelection.Tags[tag] = copiedTools
	}

	return config, nil
}

// LastGlobalConfigUpdate returns the timestamp of the most recent global config update.
func (c *CompositeConfigStore) LastGlobalConfigUpdate() (time.Time, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.globalConfigUpdate, nil
}

// GetModelProviderConfigs returns the merged model provider configurations from all sources.
func (c *CompositeConfigStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to refresh
	if c.needsRefreshModelProviders() {
		if err := c.refreshModelProviderConfigs(); err != nil {
			return nil, fmt.Errorf("CompositeConfigStore.GetModelProviderConfigs(): refresh failed: %w", err)
		}
	}

	// Return a copy to prevent external modification
	configs := make(map[string]*conf.ModelProviderConfig, len(c.modelProviderConfigs))
	for k, v := range c.modelProviderConfigs {
		configCopy := *v
		configCopy.ModelTags = make([]conf.ModelTagMapping, len(v.ModelTags))
		copy(configCopy.ModelTags, v.ModelTags)
		configs[k] = &configCopy
	}

	return configs, nil
}

// LastModelProviderConfigsUpdate returns the timestamp of the most recent model provider configs update.
func (c *CompositeConfigStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.modelProviderConfigsUpdate, nil
}

// GetAgentRoleConfigs returns the merged agent role configurations from all sources.
func (c *CompositeConfigStore) GetAgentRoleConfigs() (map[string]*conf.AgentRoleConfig, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to refresh
	if c.needsRefreshAgentRoles() {
		if err := c.refreshAgentRoleConfigs(); err != nil {
			return nil, fmt.Errorf("CompositeConfigStore.GetAgentRoleConfigs(): refresh failed: %w", err)
		}
	}

	// Return a copy to prevent external modification
	configs := make(map[string]*conf.AgentRoleConfig, len(c.agentRoleConfigs))
	for k, v := range c.agentRoleConfigs {
		configCopy := *v
		if v.VFSPrivileges != nil {
			configCopy.VFSPrivileges = make(map[string]conf.FileAccess, len(v.VFSPrivileges))
			for pk, pv := range v.VFSPrivileges {
				configCopy.VFSPrivileges[pk] = pv
			}
		}
		if v.ToolsAccess != nil {
			configCopy.ToolsAccess = make(map[string]conf.AccessFlag, len(v.ToolsAccess))
			for tk, tv := range v.ToolsAccess {
				configCopy.ToolsAccess[tk] = tv
			}
		}
		if v.RunPrivileges != nil {
			configCopy.RunPrivileges = make(map[string]conf.AccessFlag, len(v.RunPrivileges))
			for rk, rv := range v.RunPrivileges {
				configCopy.RunPrivileges[rk] = rv
			}
		}
		if v.PromptFragments != nil {
			configCopy.PromptFragments = make(map[string]string, len(v.PromptFragments))
			for fk, fv := range v.PromptFragments {
				configCopy.PromptFragments[fk] = fv
			}
		}
		if v.HiddenPatterns != nil {
			configCopy.HiddenPatterns = make([]string, len(v.HiddenPatterns))
			copy(configCopy.HiddenPatterns, v.HiddenPatterns)
		}
		configs[k] = &configCopy
	}

	return configs, nil
}

// LastAgentRoleConfigsUpdate returns the timestamp of the most recent agent role configs update.
func (c *CompositeConfigStore) LastAgentRoleConfigsUpdate() (time.Time, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentRoleConfigsUpdate, nil
}

// GetAgentConfigFile returns agent config file from the highest-priority source.
// Later stores override earlier ones.
func (c *CompositeConfigStore) GetAgentConfigFile(subdir, filename string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var lastErr error

	for i := len(c.stores) - 1; i >= 0; i-- {
		data, err := c.stores[i].GetAgentConfigFile(subdir, filename)
		if err == nil {
			return data, nil
		}
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("CompositeConfigStore.GetAgentConfigFile() [composite.go]: failed to read agent/%s/%s: %w", subdir, filename, lastErr)
	}

	return nil, fmt.Errorf("CompositeConfigStore.GetAgentConfigFile() [composite.go]: file not found in any config store: agent/%s/%s", subdir, filename)
}

// refresh reloads all configurations from all sources and merges them.
func (c *CompositeConfigStore) refresh() error {
	if err := c.refreshGlobalConfig(); err != nil {
		return err
	}
	if err := c.refreshModelProviderConfigs(); err != nil {
		return err
	}
	if err := c.refreshAgentRoleConfigs(); err != nil {
		return err
	}
	return nil
}

// needsRefreshGlobal checks if global config needs to be refreshed.
func (c *CompositeConfigStore) needsRefreshGlobal() bool {
	for i, store := range c.stores {
		lastUpdate, err := store.LastGlobalConfigUpdate()
		if err != nil {
			continue
		}
		if !lastUpdate.Equal(c.storeGlobalUpdates[i]) {
			return true
		}
	}
	return false
}

// needsRefreshModelProviders checks if model provider configs need to be refreshed.
func (c *CompositeConfigStore) needsRefreshModelProviders() bool {
	for i, store := range c.stores {
		lastUpdate, err := store.LastModelProviderConfigsUpdate()
		if err != nil {
			continue
		}
		if !lastUpdate.Equal(c.storeModelUpdates[i]) {
			return true
		}
	}
	return false
}

// needsRefreshAgentRoles checks if agent role configs need to be refreshed.
func (c *CompositeConfigStore) needsRefreshAgentRoles() bool {
	for i, store := range c.stores {
		lastUpdate, err := store.LastAgentRoleConfigsUpdate()
		if err != nil {
			continue
		}
		if !lastUpdate.Equal(c.storeRoleUpdates[i]) {
			return true
		}
	}
	return false
}

// refreshGlobalConfig reloads and merges global configuration from all sources.
func (c *CompositeConfigStore) refreshGlobalConfig() error {
	merged := &conf.GlobalConfig{}
	var latestUpdate time.Time

	for i, store := range c.stores {
		config, err := store.GetGlobalConfig()
		if err != nil {
			return fmt.Errorf("CompositeConfigStore.refreshGlobalConfig(): failed to get config from store %d: %w", i, err)
		}

		// For GlobalConfig, we append ModelTags (they're additive)
		merged.ModelTags = append(merged.ModelTags, config.ModelTags...)
		if merged.ToolSelection.Default == nil {
			merged.ToolSelection.Default = make(map[string]bool)
		}
		for toolName, enabled := range config.ToolSelection.Default {
			merged.ToolSelection.Default[toolName] = enabled
		}
		if merged.ToolSelection.Tags == nil {
			merged.ToolSelection.Tags = make(map[string]map[string]bool)
		}
		for tag, tools := range config.ToolSelection.Tags {
			if merged.ToolSelection.Tags[tag] == nil {
				merged.ToolSelection.Tags[tag] = make(map[string]bool)
			}
			for toolName, enabled := range tools {
				merged.ToolSelection.Tags[tag][toolName] = enabled
			}
		}
		// DefaultProvider from later sources overrides earlier ones
		if config.DefaultProvider != "" {
			merged.DefaultProvider = config.DefaultProvider
		}
		// DefaultRole from later sources overrides earlier ones
		if config.DefaultRole != "" {
			merged.DefaultRole = config.DefaultRole
		}

		// Track update time
		lastUpdate, err := store.LastGlobalConfigUpdate()
		if err == nil {
			c.storeGlobalUpdates[i] = lastUpdate
			if lastUpdate.After(latestUpdate) {
				latestUpdate = lastUpdate
			}
		}
	}

	c.globalConfig = merged
	c.globalConfigUpdate = latestUpdate
	return nil
}

// refreshModelProviderConfigs reloads and merges model provider configurations from all sources.
func (c *CompositeConfigStore) refreshModelProviderConfigs() error {
	merged := make(map[string]*conf.ModelProviderConfig)
	var latestUpdate time.Time

	for i, store := range c.stores {
		configs, err := store.GetModelProviderConfigs()
		if err != nil {
			return fmt.Errorf("CompositeConfigStore.refreshModelProviderConfigs(): failed to get configs from store %d: %w", i, err)
		}

		// Merge: later sources override earlier ones entirely (per provider)
		for name, config := range configs {
			// Deep copy the config
			configCopy := *config
			configCopy.ModelTags = make([]conf.ModelTagMapping, len(config.ModelTags))
			copy(configCopy.ModelTags, config.ModelTags)
			merged[name] = &configCopy
		}

		// Track update time
		lastUpdate, err := store.LastModelProviderConfigsUpdate()
		if err == nil {
			c.storeModelUpdates[i] = lastUpdate
			if lastUpdate.After(latestUpdate) {
				latestUpdate = lastUpdate
			}
		}
	}

	c.modelProviderConfigs = merged
	c.modelProviderConfigsUpdate = latestUpdate
	return nil
}

// refreshAgentRoleConfigs reloads and merges agent role configurations from all sources.
func (c *CompositeConfigStore) refreshAgentRoleConfigs() error {
	merged := make(map[string]*conf.AgentRoleConfig)
	var latestUpdate time.Time

	for i, store := range c.stores {
		configs, err := store.GetAgentRoleConfigs()
		if err != nil {
			return fmt.Errorf("CompositeConfigStore.refreshAgentRoleConfigs(): failed to get configs from store %d: %w", i, err)
		}

		// Merge: later sources override earlier ones entirely (per role)
		for name, config := range configs {
			// Check if we already have this role from a previous source
			existingConfig, exists := merged[name]

			// Deep copy the config
			configCopy := *config
			if config.VFSPrivileges != nil {
				configCopy.VFSPrivileges = make(map[string]conf.FileAccess, len(config.VFSPrivileges))
				for k, v := range config.VFSPrivileges {
					configCopy.VFSPrivileges[k] = v
				}
			}
			if config.ToolsAccess != nil {
				configCopy.ToolsAccess = make(map[string]conf.AccessFlag, len(config.ToolsAccess))
				for k, v := range config.ToolsAccess {
					configCopy.ToolsAccess[k] = v
				}
			}
			if config.RunPrivileges != nil {
				configCopy.RunPrivileges = make(map[string]conf.AccessFlag, len(config.RunPrivileges))
				for k, v := range config.RunPrivileges {
					configCopy.RunPrivileges[k] = v
				}
			}

			// Merge PromptFragments: per-filename from all sources
			if exists && existingConfig.PromptFragments != nil {
				// Start with existing fragments
				configCopy.PromptFragments = make(map[string]string, len(existingConfig.PromptFragments))
				for k, v := range existingConfig.PromptFragments {
					configCopy.PromptFragments[k] = v
				}
			} else {
				configCopy.PromptFragments = make(map[string]string)
			}

			// Merge in new fragments from current source
			if config.PromptFragments != nil {
				for filename, content := range config.PromptFragments {
					// Check if content is empty or only whitespace
					trimmedContent := strings.TrimSpace(content)
					if trimmedContent == "" {
						// Remove the fragment if it exists
						delete(configCopy.PromptFragments, filename)
					} else {
						// Override with new content
						configCopy.PromptFragments[filename] = content
					}
				}
			}

			// Merge HiddenPatterns: append from all sources
			if exists && existingConfig.HiddenPatterns != nil {
				// Start with existing patterns
				configCopy.HiddenPatterns = make([]string, len(existingConfig.HiddenPatterns))
				copy(configCopy.HiddenPatterns, existingConfig.HiddenPatterns)
			} else {
				configCopy.HiddenPatterns = make([]string, 0)
			}

			// Append new patterns from current source
			if config.HiddenPatterns != nil {
				configCopy.HiddenPatterns = append(configCopy.HiddenPatterns, config.HiddenPatterns...)
			}

			merged[name] = &configCopy
		}

		// Track update time
		lastUpdate, err := store.LastAgentRoleConfigsUpdate()
		if err == nil {
			c.storeRoleUpdates[i] = lastUpdate
			if lastUpdate.After(latestUpdate) {
				latestUpdate = lastUpdate
			}
		}
	}

	c.agentRoleConfigs = merged
	c.agentRoleConfigsUpdate = latestUpdate
	return nil
}

// parseConfigPath parses the colon-separated config path string into individual paths.
func parseConfigPath(configPath string) []string {
	if configPath == "" {
		return []string{"@DEFAULTS"}
	}

	parts := strings.Split(configPath, ":")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// createConfigStore creates a config store for the given path.
func createConfigStore(path string, projDir string) (conf.ConfigStore, error) {
	// Handle special @DEFAULTS token
	if path == "@DEFAULTS" {
		return NewEmbeddedConfigStore()
	}

	// Expand path
	expandedPath, err := expandPath(path, projDir)
	if err != nil {
		return nil, fmt.Errorf("createConfigStore(): failed to expand path %s: %w", path, err)
	}

	// Check if path exists
	info, err := os.Stat(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Path doesn't exist, skip it (optional path)
			return nil, nil
		}
		return nil, fmt.Errorf("createConfigStore(): failed to stat path %s: %w", expandedPath, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("createConfigStore(): path %s is not a directory", expandedPath)
	}

	// Create local config store
	return NewLocalConfigStore(expandedPath)
}

// expandPath expands special tokens in configuration paths.
func expandPath(path string, projDir string) (string, error) {
	// Handle @PROJ/ prefix
	if strings.HasPrefix(path, "@PROJ/") {
		rest := strings.TrimPrefix(path, "@PROJ/")
		return filepath.Join(projDir, rest), nil
	}

	// Handle ~/ prefix
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expandPath(): failed to get home directory: %w", err)
		}
		rest := strings.TrimPrefix(path, "~/")
		return filepath.Join(homeDir, rest), nil
	}

	// Handle ./ prefix
	if strings.HasPrefix(path, "./") {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("expandPath(): failed to get working directory: %w", err)
		}
		rest := strings.TrimPrefix(path, "./")
		return filepath.Join(cwd, rest), nil
	}

	// Absolute path or already expanded
	return path, nil
}
