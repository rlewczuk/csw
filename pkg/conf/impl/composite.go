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

	"github.com/rlewczuk/csw/pkg/conf"
)

// CompositeConfigStore implements conf.ConfigStore interface for multiple configuration sources.
// It merges configurations from multiple sources, where later sources override earlier ones.
type CompositeConfigStore struct {
	mu      sync.RWMutex
	stores  []conf.ConfigStore
	projDir string

	// Cached configuration
	globalConfig         *conf.GlobalConfig
	modelProviderConfigs map[string]*conf.ModelProviderConfig
	modelAliases         map[string]conf.ModelAliasValue
	mcpServerConfigs     map[string]*conf.MCPServerConfig
	hookConfigs          map[string]*conf.HookConfig
	agentRoleConfigs     map[string]*conf.AgentRoleConfig
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
		stores:  stores,
		projDir: projDir,
	}

	// Initial load
	if err := composite.refresh(); err != nil {
		return nil, fmt.Errorf("NewCompositeConfigStore(): failed to load initial configuration: %w", err)
	}

	return composite, nil
}

// GetGlobalConfig returns the merged global configuration from all sources.
func (c *CompositeConfigStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.globalConfig.Clone(), nil
}

// GetModelProviderConfigs returns the merged model provider configurations from all sources.
func (c *CompositeConfigStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	configs := make(map[string]*conf.ModelProviderConfig, len(c.modelProviderConfigs))
	for k, v := range c.modelProviderConfigs {
		configs[k] = v.Clone()
	}

	return configs, nil
}

// GetModelAliases returns merged model aliases from all sources.
func (c *CompositeConfigStore) GetModelAliases() (map[string]conf.ModelAliasValue, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	aliases := make(map[string]conf.ModelAliasValue, len(c.modelAliases))
	for key, value := range c.modelAliases {
		aliases[key] = conf.ModelAliasValue{Values: append([]string(nil), value.Values...)}
	}

	return aliases, nil
}

// GetMCPServerConfigs returns merged MCP server configurations from all sources.
func (c *CompositeConfigStore) GetMCPServerConfigs() (map[string]*conf.MCPServerConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	configs := make(map[string]*conf.MCPServerConfig, len(c.mcpServerConfigs))
	for key, value := range c.mcpServerConfigs {
		configs[key] = value.Clone()
	}

	return configs, nil
}

// GetHookConfigs returns merged hook configurations from all sources.
func (c *CompositeConfigStore) GetHookConfigs() (map[string]*conf.HookConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	configs := make(map[string]*conf.HookConfig, len(c.hookConfigs))
	for key, value := range c.hookConfigs {
		configs[key] = value.Clone()
	}

	return configs, nil
}

// GetAgentRoleConfigs returns the merged agent role configurations from all sources.
func (c *CompositeConfigStore) GetAgentRoleConfigs() (map[string]*conf.AgentRoleConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	configs := make(map[string]*conf.AgentRoleConfig, len(c.agentRoleConfigs))
	for k, v := range c.agentRoleConfigs {
		configs[k] = v.Clone()
	}

	return configs, nil
}

// Stores returns a shallow copy of underlying config stores in merge order.
func (c *CompositeConfigStore) Stores() ([]conf.ConfigStore, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stores := make([]conf.ConfigStore, len(c.stores))
	copy(stores, c.stores)

	return stores, nil
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
	if err := c.refreshModelAliases(); err != nil {
		return err
	}
	if err := c.refreshMCPServerConfigs(); err != nil {
		return err
	}
	if err := c.refreshHookConfigs(); err != nil {
		return err
	}
	if err := c.refreshAgentRoleConfigs(); err != nil {
		return err
	}
	return nil
}

// refreshGlobalConfig reloads and merges global configuration from all sources.
func (c *CompositeConfigStore) refreshGlobalConfig() error {
	merged := &conf.GlobalConfig{}

	for i, store := range c.stores {
		config, err := store.GetGlobalConfig()
		if err != nil {
			return fmt.Errorf("CompositeConfigStore.refreshGlobalConfig(): failed to get config from store %d: %w", i, err)
		}

		merged.Merge(config)
	}

	c.globalConfig = merged
	return nil
}

// refreshModelProviderConfigs reloads and merges model provider configurations from all sources.
func (c *CompositeConfigStore) refreshModelProviderConfigs() error {
	merged := make(map[string]*conf.ModelProviderConfig)

	for i, store := range c.stores {
		configs, err := store.GetModelProviderConfigs()
		if err != nil {
			return fmt.Errorf("CompositeConfigStore.refreshModelProviderConfigs(): failed to get configs from store %d: %w", i, err)
		}

		// Merge: later sources override earlier ones entirely (per provider)
		for name, config := range configs {
			merged[name] = config.Clone()
		}

	}

	c.modelProviderConfigs = merged
	return nil
}

func (c *CompositeConfigStore) refreshModelAliases() error {
	merged := make(map[string]conf.ModelAliasValue)

	for i, store := range c.stores {
		aliases, err := store.GetModelAliases()
		if err != nil {
			return fmt.Errorf("CompositeConfigStore.refreshModelAliases() [composite.go]: failed to get aliases from store %d: %w", i, err)
		}

		for key, value := range aliases {
			merged[key] = conf.ModelAliasValue{Values: append([]string(nil), value.Values...)}
		}
	}

	c.modelAliases = merged

	return nil
}

// refreshMCPServerConfigs reloads and merges MCP server configurations from all sources.
func (c *CompositeConfigStore) refreshMCPServerConfigs() error {
	merged := make(map[string]*conf.MCPServerConfig)

	for i, store := range c.stores {
		configs, err := store.GetMCPServerConfigs()
		if err != nil {
			return fmt.Errorf("CompositeConfigStore.refreshMCPServerConfigs() [composite.go]: failed to get configs from store %d: %w", i, err)
		}

		for key, value := range configs {
			merged[key] = value.Clone()
		}
	}

	c.mcpServerConfigs = merged

	return nil
}

// refreshHookConfigs reloads and merges hook configurations from all sources.
func (c *CompositeConfigStore) refreshHookConfigs() error {
	merged := make(map[string]*conf.HookConfig)

	for i, store := range c.stores {
		configs, err := store.GetHookConfigs()
		if err != nil {
			return fmt.Errorf("CompositeConfigStore.refreshHookConfigs() [composite.go]: failed to get configs from store %d: %w", i, err)
		}

		for key, value := range configs {
			merged[key] = value.Clone()
		}
	}

	c.hookConfigs = merged

	return nil
}

// refreshAgentRoleConfigs reloads and merges agent role configurations from all sources.
func (c *CompositeConfigStore) refreshAgentRoleConfigs() error {
	merged := make(map[string]*conf.AgentRoleConfig)

	for i, store := range c.stores {
		configs, err := store.GetAgentRoleConfigs()
		if err != nil {
			return fmt.Errorf("CompositeConfigStore.refreshAgentRoleConfigs(): failed to get configs from store %d: %w", i, err)
		}

		// Merge: later sources override earlier ones entirely (per role), while
		// prompt/tool fragments are merged by key and hidden patterns are additive.
		for name, config := range configs {
			existingConfig, exists := merged[name]
			if !exists {
				merged[name] = config.Clone()
				continue
			}

			existingConfig.Merge(config)
		}
	}

	c.agentRoleConfigs = merged
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
