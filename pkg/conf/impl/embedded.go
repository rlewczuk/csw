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
// Since the configuration is embedded at build time, it is immutable.
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
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/vfs"
)

//go:embed all:conf
var embeddedConfigFS embed.FS

// EmbeddedConfigStore implements conf.ConfigStore interface for embedded configuration.
// It provides read-only access to configuration files that are embedded in the binary.
type EmbeddedConfigStore struct {
	mu                   sync.RWMutex
	globalConfig         *conf.GlobalConfig
	modelProviderConfigs map[string]*conf.ModelProviderConfig
	modelAliases         map[string]conf.ModelAliasValue
	agentRoleConfigs     map[string]*conf.AgentRoleConfig
	loaded               bool
}

// NewEmbeddedConfigStore creates a new EmbeddedConfigStore instance that reads configuration
// from the embedded filesystem.
func NewEmbeddedConfigStore() (conf.ConfigStore, error) {
	store := &EmbeddedConfigStore{
		globalConfig:         &conf.GlobalConfig{},
		modelProviderConfigs: make(map[string]*conf.ModelProviderConfig),
		modelAliases:         make(map[string]conf.ModelAliasValue),
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

	return s.globalConfig.Clone(), nil
}

// GetModelProviderConfigs returns a map of model provider configurations.
func (s *EmbeddedConfigStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	configs := make(map[string]*conf.ModelProviderConfig, len(s.modelProviderConfigs))
	for k, v := range s.modelProviderConfigs {
		configs[k] = v.Clone()
	}

	return configs, nil
}

// GetModelAliases returns configured model aliases.
func (s *EmbeddedConfigStore) GetModelAliases() (map[string]conf.ModelAliasValue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	aliases := make(map[string]conf.ModelAliasValue, len(s.modelAliases))
	for key, value := range s.modelAliases {
		aliases[key] = conf.ModelAliasValue{Values: append([]string(nil), value.Values...)}
	}

	return aliases, nil
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

// GetAgentConfigFile returns file content from embedded conf/agent/<subdir>/<filename>.
func (s *EmbeddedConfigStore) GetAgentConfigFile(subdir, filename string) ([]byte, error) {
	if filename == "" {
		return nil, fmt.Errorf("EmbeddedConfigStore.GetAgentConfigFile() [embedded.go]: filename cannot be empty")
	}

	if filepath.Base(filename) != filename {
		return nil, fmt.Errorf("EmbeddedConfigStore.GetAgentConfigFile() [embedded.go]: filename cannot contain path separators")
	}

	cleanSubdir := filepath.Clean(subdir)
	if cleanSubdir == "." {
		cleanSubdir = ""
	}
	if cleanSubdir != "" && (filepath.IsAbs(cleanSubdir) || cleanSubdir == ".." || strings.HasPrefix(cleanSubdir, "../")) {
		return nil, fmt.Errorf("EmbeddedConfigStore.GetAgentConfigFile() [embedded.go]: invalid subdir: %s", subdir)
	}

	path := filepath.Join("conf", "agent", cleanSubdir, filename)
	data, err := embeddedConfigFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("EmbeddedConfigStore.GetAgentConfigFile() [embedded.go]: failed to read %s: %w", path, err)
	}

	return data, nil
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
	if err := s.loadModelAliases(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load model aliases: %w", err)
	}
	if err := s.loadAgentRoleConfigs(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load agent role configs: %w", err)
	}

	s.loaded = true
	return nil
}

func (s *EmbeddedConfigStore) loadModelAliases() error {
	aliases, loaded, err := loadEmbeddedModelAliases()
	if err != nil {
		return err
	}
	if !loaded {
		aliases = make(map[string]conf.ModelAliasValue)
	}
	s.modelAliases = aliases

	return nil
}

func loadEmbeddedModelAliases() (map[string]conf.ModelAliasValue, bool, error) {
	jsonlPath := "conf/model_aliases.jsonl"

	if data, err := embeddedConfigFS.ReadFile(jsonlPath); err == nil {
		aliases, parseErr := parseModelAliasesJSONL(data, jsonlPath)
		if parseErr != nil {
			return nil, false, parseErr
		}
		return aliases, true, nil
	} else if !isNotExist(err) {
		return nil, false, fmt.Errorf("loadEmbeddedModelAliases() [embedded.go]: failed to read %s: %w", jsonlPath, err)
	}

	return nil, false, nil
}

// loadGlobalConfig loads the global configuration from embedded global.json.
func (s *EmbeddedConfigStore) loadGlobalConfig() error {
	jsonPath := "conf/global.json"
	data, err := embeddedConfigFS.ReadFile(jsonPath)
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
	if len(config.ShadowPaths) == 0 {
		config.ShadowPaths = append([]string(nil), vfs.DefaultShadowPatterns()...)
	}

	s.globalConfig = &config
	return nil
}

// loadModelProviderConfigs loads all model provider configurations from the embedded models directory.
// Supports .json files.
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

		providerName := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
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

func (s *EmbeddedConfigStore) loadModelProviderConfigMapDir(dir string) (map[string]conf.ModelProviderConfig, error) {
	entries, err := embeddedConfigFS.ReadDir(dir)
	if err != nil {
		if isNotExist(err) {
			return map[string]conf.ModelProviderConfig{}, nil
		}
		return nil, fmt.Errorf("loadModelProviderConfigMapDir(): failed to read %s: %w", dir, err)
	}

	result := make(map[string]conf.ModelProviderConfig)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".json" {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		path := filepath.Join(dir, entry.Name())
		data, err := embeddedConfigFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("loadModelProviderConfigMapDir(): failed to read %s: %w", path, err)
		}

		var item conf.ModelProviderConfig
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("loadModelProviderConfigMapDir(): failed to parse %s: %w", path, err)
		}
		result[key] = item
	}

	return result, nil
}

func (s *EmbeddedConfigStore) loadModelTemplateGroupsDir(dir string) (map[string]map[string]conf.ModelProviderConfig, error) {
	entries, err := embeddedConfigFS.ReadDir(dir)
	if err != nil {
		if isNotExist(err) {
			return map[string]map[string]conf.ModelProviderConfig{}, nil
		}
		return nil, fmt.Errorf("loadModelTemplateGroupsDir(): failed to read %s: %w", dir, err)
	}

	result := make(map[string]map[string]conf.ModelProviderConfig)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".json" {
			continue
		}
		group := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		path := filepath.Join(dir, entry.Name())
		data, err := embeddedConfigFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("loadModelTemplateGroupsDir(): failed to read %s: %w", path, err)
		}

		items := make(map[string]conf.ModelProviderConfig)
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("loadModelTemplateGroupsDir(): failed to parse %s: %w", path, err)
		}
		result[group] = items
	}

	return result, nil
}

// loadAgentRoleConfigs loads all agent role configurations from the embedded roles directory.
// The special "all" meta-role is loaded without requiring config.json, as it only contains
// prompt fragments that are merged into other roles.
// Supports config.json.
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
				// Special case: "all" is a meta-role that only contains prompt fragments
				// to be merged into other roles. It doesn't require a config file.
				if roleName == "all" {
					// Load only prompt fragments for the "all" meta-role
					promptFragments, loadErr := s.loadPromptFragments(filepath.Join(rolesDir, roleName))
					if loadErr != nil {
						return fmt.Errorf("loadAgentRoleConfigs(): failed to load prompt fragments for role %s: %w", roleName, loadErr)
					}
					// Load tool fragments from conf/tools directory (shared by all roles)
					toolFragments, loadErr := s.loadToolFragments()
					if loadErr != nil {
						return fmt.Errorf("loadAgentRoleConfigs(): failed to load tool fragments: %w", loadErr)
					}
					configs["all"] = &conf.AgentRoleConfig{
						Name:            "all",
						PromptFragments: promptFragments,
						ToolFragments:   toolFragments,
					}
					continue
				}
				// config file is required for all other roles
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
		fragments[fmt.Sprintf("%s/.tooldir", toolName)] = filepath.Join("conf", "tools", toolName)

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
