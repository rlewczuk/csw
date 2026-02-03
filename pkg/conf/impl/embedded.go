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
//   - roles/all/ - special meta-role directory containing prompt fragments that are
//     merged into all other roles (config.json is optional for this role)
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
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"gopkg.in/yaml.v3"
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
		ModelTags:       make([]conf.ModelTagMapping, len(s.globalConfig.ModelTags)),
		DefaultProvider: s.globalConfig.DefaultProvider,
		DefaultRole:     s.globalConfig.DefaultRole,
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
		if v.PromptFragments != nil {
			configCopy.PromptFragments = make(map[string]string, len(v.PromptFragments))
			for fk, fv := range v.PromptFragments {
				configCopy.PromptFragments[fk] = fv
			}
		}
		if v.ToolFragments != nil {
			configCopy.ToolFragments = make(map[string]string, len(v.ToolFragments))
			for fk, fv := range v.ToolFragments {
				configCopy.ToolFragments[fk] = fv
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

// loadGlobalConfig loads the global configuration from embedded global.yml or global.json.
// YAML takes precedence over JSON if both exist.
func (s *EmbeddedConfigStore) loadGlobalConfig() error {
	// Try YAML first (takes precedence)
	yamlPath := "conf/global.yml"
	data, err := embeddedConfigFS.ReadFile(yamlPath)
	if err == nil {
		var config conf.GlobalConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("loadGlobalConfig(): failed to parse %s: %w", yamlPath, err)
		}
		s.globalConfig = &config
		return nil
	}

	if !isNotExist(err) {
		return fmt.Errorf("loadGlobalConfig(): failed to read %s: %w", yamlPath, err)
	}

	// Fall back to JSON
	jsonPath := "conf/global.json"
	data, err = embeddedConfigFS.ReadFile(jsonPath)
	if err != nil {
		if isNotExist(err) {
			// Global config is optional, use empty config
			s.globalConfig = &conf.GlobalConfig{}
			return nil
		}
		return fmt.Errorf("loadGlobalConfig(): failed to read %s: %w", jsonPath, err)
	}

	var config conf.GlobalConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("loadGlobalConfig(): failed to parse %s: %w", jsonPath, err)
	}

	s.globalConfig = &config
	return nil
}

// loadModelProviderConfigs loads all model provider configurations from the embedded models directory.
// YAML files take precedence over JSON files if both exist for the same provider.
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
	// Track which providers we've loaded (to handle YAML precedence)
	loadedProviders := make(map[string]bool)

	// First pass: load YAML files (takes precedence)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}

		providerName := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		providerPath := filepath.Join(modelsDir, entry.Name())
		data, err := embeddedConfigFS.ReadFile(providerPath)
		if err != nil {
			return fmt.Errorf("loadModelProviderConfigs(): failed to read %s: %w", providerPath, err)
		}

		var config conf.ModelProviderConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("loadModelProviderConfigs(): failed to parse %s: %w", providerPath, err)
		}

		// If name is not set, use filename without extension
		if config.Name == "" {
			config.Name = providerName
		}

		configs[config.Name] = &config
		loadedProviders[providerName] = true
	}

	// Second pass: load JSON files (only if YAML doesn't exist for that provider)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		providerName := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		// Skip if already loaded from YAML
		if loadedProviders[providerName] {
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
			config.Name = providerName
		}

		configs[config.Name] = &config
	}

	s.modelProviderConfigs = configs
	return nil
}

// loadAgentRoleConfigs loads all agent role configurations from the embedded roles directory.
// The special "all" meta-role is loaded without requiring config.json, as it only contains
// prompt fragments that are merged into other roles.
// YAML files take precedence over JSON files if both exist for the same role.
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

		// Try YAML first (takes precedence)
		configPath := filepath.Join(rolesDir, roleName, "config.yml")
		data, err := embeddedConfigFS.ReadFile(configPath)
		if err != nil {
			if !isNotExist(err) {
				return fmt.Errorf("loadAgentRoleConfigs(): failed to read %s: %w", configPath, err)
			}
			// Fall back to JSON
			configPath = filepath.Join(rolesDir, roleName, "config.json")
			data, err = embeddedConfigFS.ReadFile(configPath)
			if err != nil {
				if isNotExist(err) {
					// Special case: "all" is a meta-role that only contains prompt fragments
					// to be merged into other roles. It doesn't require a config file.
					if roleName == "all" {
						// Load only prompt fragments for the "all" meta-role
						promptFragments, err := s.loadPromptFragments(filepath.Join(rolesDir, roleName))
						if err != nil {
							return fmt.Errorf("loadAgentRoleConfigs(): failed to load prompt fragments for role %s: %w", roleName, err)
						}
						// Load tool fragments from conf/tools directory (shared by all roles)
						toolFragments, err := s.loadToolFragments()
						if err != nil {
							return fmt.Errorf("loadAgentRoleConfigs(): failed to load tool fragments: %w", err)
						}
						configs["all"] = &conf.AgentRoleConfig{
							Name:            "all",
							PromptFragments: promptFragments,
							ToolFragments:   toolFragments,
						}
						continue
					}
					// config file is required for all other roles
					return fmt.Errorf("loadAgentRoleConfigs(): role %s missing config.yml or config.json", roleName)
				}
				return fmt.Errorf("loadAgentRoleConfigs(): failed to read %s: %w", configPath, err)
			}
		}

		var config conf.AgentRoleConfig
		if filepath.Ext(configPath) == ".yml" {
			if err := yaml.Unmarshal(data, &config); err != nil {
				return fmt.Errorf("loadAgentRoleConfigs(): failed to parse %s: %w", configPath, err)
			}
		} else {
			if err := json.Unmarshal(data, &config); err != nil {
				return fmt.Errorf("loadAgentRoleConfigs(): failed to parse %s: %w", configPath, err)
			}
		}

		// If name is not set, use directory name
		if config.Name == "" {
			config.Name = roleName
		}

		// Load prompt fragments from .md files in the role directory
		promptFragments, err := s.loadPromptFragments(filepath.Join(rolesDir, roleName))
		if err != nil {
			return fmt.Errorf("loadAgentRoleConfigs(): failed to load prompt fragments for role %s: %w", roleName, err)
		}
		config.PromptFragments = promptFragments

		// Load tool fragments from conf/tools directory (shared by all roles)
		toolFragments, err := s.loadToolFragments()
		if err != nil {
			return fmt.Errorf("loadAgentRoleConfigs(): failed to load tool fragments: %w", err)
		}
		config.ToolFragments = toolFragments

		configs[config.Name] = &config
	}

	s.agentRoleConfigs = configs
	return nil
}

// loadPromptFragments loads all .md files from the given role directory.
func (s *EmbeddedConfigStore) loadPromptFragments(roleDir string) (map[string]string, error) {
	fragments := make(map[string]string)

	entries, err := embeddedConfigFS.ReadDir(roleDir)
	if err != nil {
		return nil, fmt.Errorf("loadPromptFragments(): failed to read role directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		fragmentPath := filepath.Join(roleDir, entry.Name())
		data, err := embeddedConfigFS.ReadFile(fragmentPath)
		if err != nil {
			return nil, fmt.Errorf("loadPromptFragments(): failed to read %s: %w", fragmentPath, err)
		}

		// Use filename without extension as the key
		fragmentName := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		fragments[fragmentName] = string(data)
	}

	return fragments, nil
}

// loadToolFragments loads all tool description files from the conf/tools directory.
// Returns a map where keys are "<tool-name>/<file-name>" and values are file contents.
func (s *EmbeddedConfigStore) loadToolFragments() (map[string]string, error) {
	fragments := make(map[string]string)
	toolsDir := "conf/tools"

	entries, err := embeddedConfigFS.ReadDir(toolsDir)
	if err != nil {
		if isNotExist(err) {
			// Tools directory is optional
			return fragments, nil
		}
		return nil, fmt.Errorf("loadToolFragments(): failed to read tools directory: %w", err)
	}

	// Iterate through each tool directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		toolName := entry.Name()
		toolDir := filepath.Join(toolsDir, toolName)

		// Read all files in the tool directory
		toolEntries, err := embeddedConfigFS.ReadDir(toolDir)
		if err != nil {
			return nil, fmt.Errorf("loadToolFragments(): failed to read tool directory %s: %w", toolDir, err)
		}

		for _, toolEntry := range toolEntries {
			if toolEntry.IsDir() {
				continue
			}

			filePath := filepath.Join(toolDir, toolEntry.Name())
			data, err := embeddedConfigFS.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("loadToolFragments(): failed to read %s: %w", filePath, err)
			}

			// Key format: "<tool-name>/<file-name>"
			key := fmt.Sprintf("%s/%s", toolName, toolEntry.Name())
			fragments[key] = string(data)
		}
	}

	return fragments, nil
}

// isNotExist checks if an error is a "not exist" error for embedded filesystem.
func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	// For embed.FS, use errors.Is to properly check wrapped errors
	return errors.Is(err, fs.ErrNotExist)
}
