// Package impl provides an embedded configuration store implementation
// that satisfies the conf.ConfigStore interface.
//
// The embedded config store provides read-only access to configuration
// that is built into the binary at compile time using go:embed directive.
// Configuration files are embedded from the conf/ directory in the repository root.
//
// The store reads configuration from the embedded filesystem with the same structure
// as the local config store:
//   - global.json - global configuration (optional)
//   - models/*.json - model provider configurations (one file per provider)
//   - roles/*/config.json - agent role configurations (one directory per role)
//
// Since the configuration is embedded at build time, it is immutable and all
// LastUpdate() methods return a constant timestamp.
//
// Example usage:
//
//	store := embedded.NewEmbeddedConfigStore()
//	globalConfig, err := store.GetGlobalConfig()
//	providers, err := store.GetModelProviderConfigs()
//	roles, err := store.GetAgentRoleConfigs()
package impl

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
)

//go:embed all:conf
var embeddedConfigFS embed.FS

// embeddedTimestamp is a constant timestamp returned by all LastUpdate() methods.
// This is set to a fixed point in time since embedded configuration doesn't change.
var embeddedTimestamp = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// EmbeddedConfigStore implements conf.ConfigStore interface for embedded configuration.
// It provides read-only access to configuration files that are embedded in the binary.
type EmbeddedConfigStore struct {
	mu                   sync.RWMutex
	globalConfig         *conf.GlobalConfig
	modelProviderConfigs map[string]*conf.ModelProviderConfig
	agentRoleConfigs     map[string]*conf.AgentRoleConfig
	loaded               bool
}

// NewEmbeddedConfigStore creates a new EmbeddedConfigStore instance that reads configuration
// from the embedded filesystem.
func NewEmbeddedConfigStore() (conf.ConfigStore, error) {
	store := &EmbeddedConfigStore{
		globalConfig:         &conf.GlobalConfig{},
		modelProviderConfigs: make(map[string]*conf.ModelProviderConfig),
		agentRoleConfigs:     make(map[string]*conf.AgentRoleConfig),
	}

	// Load all configuration from embedded filesystem
	if err := store.loadAllConfig(); err != nil {
		return nil, fmt.Errorf("NewEmbeddedConfigStore(): failed to load embedded configuration: %w", err)
	}

	return store, nil
}

// GetGlobalConfig returns the global configuration.
func (s *EmbeddedConfigStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	config := &conf.GlobalConfig{
		ModelTags: make([]conf.ModelTagMapping, len(s.globalConfig.ModelTags)),
	}
	copy(config.ModelTags, s.globalConfig.ModelTags)

	return config, nil
}

// LastGlobalConfigUpdate returns the timestamp of the last global config update.
// For embedded configuration, this always returns a constant timestamp.
func (s *EmbeddedConfigStore) LastGlobalConfigUpdate() (time.Time, error) {
	return embeddedTimestamp, nil
}

// GetModelProviderConfigs returns a map of model provider configurations.
func (s *EmbeddedConfigStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	configs := make(map[string]*conf.ModelProviderConfig, len(s.modelProviderConfigs))
	for k, v := range s.modelProviderConfigs {
		configCopy := *v
		configCopy.ModelTags = make([]conf.ModelTagMapping, len(v.ModelTags))
		copy(configCopy.ModelTags, v.ModelTags)
		configs[k] = &configCopy
	}

	return configs, nil
}

// LastModelProviderConfigsUpdate returns the timestamp of the last model provider configs update.
// For embedded configuration, this always returns a constant timestamp.
func (s *EmbeddedConfigStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	return embeddedTimestamp, nil
}

// GetAgentRoleConfigs returns a map of agent role configurations.
func (s *EmbeddedConfigStore) GetAgentRoleConfigs() (map[string]*conf.AgentRoleConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	configs := make(map[string]*conf.AgentRoleConfig, len(s.agentRoleConfigs))
	for k, v := range s.agentRoleConfigs {
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
		configs[k] = &configCopy
	}

	return configs, nil
}

// LastAgentRoleConfigsUpdate returns the timestamp of the last agent role configs update.
// For embedded configuration, this always returns a constant timestamp.
func (s *EmbeddedConfigStore) LastAgentRoleConfigsUpdate() (time.Time, error) {
	return embeddedTimestamp, nil
}

// loadAllConfig loads all configuration from the embedded filesystem.
func (s *EmbeddedConfigStore) loadAllConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loaded {
		return nil
	}

	if err := s.loadGlobalConfig(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load global config: %w", err)
	}
	if err := s.loadModelProviderConfigs(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load model provider configs: %w", err)
	}
	if err := s.loadAgentRoleConfigs(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load agent role configs: %w", err)
	}

	s.loaded = true
	return nil
}

// loadGlobalConfig loads the global configuration from embedded global.json.
func (s *EmbeddedConfigStore) loadGlobalConfig() error {
	globalPath := "conf/global.json"

	data, err := embeddedConfigFS.ReadFile(globalPath)
	if err != nil {
		if isNotExist(err) {
			// Global config is optional, use empty config
			s.globalConfig = &conf.GlobalConfig{}
			return nil
		}
		return fmt.Errorf("loadGlobalConfig(): failed to read %s: %w", globalPath, err)
	}

	var config conf.GlobalConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("loadGlobalConfig(): failed to parse %s: %w", globalPath, err)
	}

	s.globalConfig = &config
	return nil
}

// loadModelProviderConfigs loads all model provider configurations from the embedded models directory.
func (s *EmbeddedConfigStore) loadModelProviderConfigs() error {
	modelsDir := "conf/models"

	entries, err := embeddedConfigFS.ReadDir(modelsDir)
	if err != nil {
		if isNotExist(err) {
			// Models directory is optional
			s.modelProviderConfigs = make(map[string]*conf.ModelProviderConfig)
			return nil
		}
		return fmt.Errorf("loadModelProviderConfigs(): failed to read models directory: %w", err)
	}

	configs := make(map[string]*conf.ModelProviderConfig)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		providerPath := filepath.Join(modelsDir, entry.Name())
		data, err := embeddedConfigFS.ReadFile(providerPath)
		if err != nil {
			return fmt.Errorf("loadModelProviderConfigs(): failed to read %s: %w", providerPath, err)
		}

		var config conf.ModelProviderConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("loadModelProviderConfigs(): failed to parse %s: %w", providerPath, err)
		}

		// If name is not set, use filename without extension
		if config.Name == "" {
			config.Name = entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		}

		configs[config.Name] = &config
	}

	s.modelProviderConfigs = configs
	return nil
}

// loadAgentRoleConfigs loads all agent role configurations from the embedded roles directory.
func (s *EmbeddedConfigStore) loadAgentRoleConfigs() error {
	rolesDir := "conf/roles"

	entries, err := embeddedConfigFS.ReadDir(rolesDir)
	if err != nil {
		if isNotExist(err) {
			// Roles directory is optional
			s.agentRoleConfigs = make(map[string]*conf.AgentRoleConfig)
			return nil
		}
		return fmt.Errorf("loadAgentRoleConfigs(): failed to read roles directory: %w", err)
	}

	configs := make(map[string]*conf.AgentRoleConfig)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		roleName := entry.Name()
		configPath := filepath.Join(rolesDir, roleName, "config.json")

		data, err := embeddedConfigFS.ReadFile(configPath)
		if err != nil {
			if isNotExist(err) {
				// config.json is required for each role
				return fmt.Errorf("loadAgentRoleConfigs(): role %s missing config.json", roleName)
			}
			return fmt.Errorf("loadAgentRoleConfigs(): failed to read %s: %w", configPath, err)
		}

		var config conf.AgentRoleConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("loadAgentRoleConfigs(): failed to parse %s: %w", configPath, err)
		}

		// If name is not set, use directory name
		if config.Name == "" {
			config.Name = roleName
		}

		configs[config.Name] = &config
	}

	s.agentRoleConfigs = configs
	return nil
}

// isNotExist checks if an error is a "not exist" error for embedded filesystem.
func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	// For embed.FS, both fs.ErrNotExist and path errors are possible
	return err == fs.ErrNotExist || err.Error() == "file does not exist"
}
