// Package impl provides a filesystem-based configuration store implementation
// that satisfies the conf.ConfigStore interface.
//
// The local config store reads configuration from a directory structure:
//   - global.json - global configuration
//   - models/*.json - model provider configurations (one file per provider)
//   - roles/*/config.json - agent role configurations (one directory per role)
//   - roles/all/ - special meta-role directory containing prompt fragments that are
//     merged into all other roles (config.json is optional for this role)
//
// The store caches configuration in memory.
//
// Example usage:
//
//	store, err := local.NewLocalConfigStore("/path/to/config")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer store.Close()
//
//	globalConfig, err := store.GetGlobalConfig()
//	providers, err := store.GetModelProviderConfigs()
//	roles, err := store.GetAgentRoleConfigs()
package impl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/conf"
	"gopkg.in/yaml.v3"
)

// LocalConfigStore implements conf.ConfigStore interface for local filesystem-based configuration.
// It caches configuration in memory.
type LocalConfigStore struct {
	configDir string

	mu                         sync.RWMutex
	globalConfig               *conf.GlobalConfig
	modelProviderConfigs       map[string]*conf.ModelProviderConfig
	modelAliases               map[string]conf.ModelAliasValue
	agentRoleConfigs           map[string]*conf.AgentRoleConfig
}

// NewLocalConfigStore creates a new LocalConfigStore instance that reads configuration
// from the specified directory.
func NewLocalConfigStore(configDir string) (*LocalConfigStore, error) {
	if err := validateConfigDir(configDir); err != nil {
		return nil, fmt.Errorf("NewLocalConfigStore(): invalid config directory: %w", err)
	}

	store := &LocalConfigStore{
		configDir:            configDir,
		globalConfig:         &conf.GlobalConfig{},
		modelProviderConfigs: make(map[string]*conf.ModelProviderConfig),
		modelAliases:         make(map[string]conf.ModelAliasValue),
		agentRoleConfigs:     make(map[string]*conf.AgentRoleConfig),
	}

	// Initial load of all configuration
	if err := store.loadAllConfig(); err != nil {
		return nil, fmt.Errorf("NewLocalConfigStore(): failed to load initial configuration: %w", err)
	}

	return store, nil
}

// Close releases resources.
func (s *LocalConfigStore) Close() error {
	return nil
}

// GetGlobalConfig returns the global configuration.
func (s *LocalConfigStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.globalConfig.Clone(), nil
}

// GetModelProviderConfigs returns a map of model provider configurations.
func (s *LocalConfigStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
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
func (s *LocalConfigStore) GetModelAliases() (map[string]conf.ModelAliasValue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	aliases := make(map[string]conf.ModelAliasValue, len(s.modelAliases))
	for key, value := range s.modelAliases {
		aliases[key] = conf.ModelAliasValue{Values: append([]string(nil), value.Values...)}
	}

	return aliases, nil
}

// GetAgentRoleConfigs returns a map of agent role configurations.
func (s *LocalConfigStore) GetAgentRoleConfigs() (map[string]*conf.AgentRoleConfig, error) {
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
		if v.HiddenPatterns != nil {
			configCopy.HiddenPatterns = make([]string, len(v.HiddenPatterns))
			copy(configCopy.HiddenPatterns, v.HiddenPatterns)
		}
		configs[k] = &configCopy
	}

	return configs, nil
}

// GetAgentConfigFile returns file content from local agent/<subdir>/<filename>.
func (s *LocalConfigStore) GetAgentConfigFile(subdir, filename string) ([]byte, error) {
	if filename == "" {
		return nil, fmt.Errorf("LocalConfigStore.GetAgentConfigFile() [local.go]: filename cannot be empty")
	}

	if filepath.Base(filename) != filename {
		return nil, fmt.Errorf("LocalConfigStore.GetAgentConfigFile() [local.go]: filename cannot contain path separators")
	}

	cleanSubdir := filepath.Clean(subdir)
	if cleanSubdir == "." {
		cleanSubdir = ""
	}
	if cleanSubdir != "" && (filepath.IsAbs(cleanSubdir) || cleanSubdir == ".." || strings.HasPrefix(cleanSubdir, "../")) {
		return nil, fmt.Errorf("LocalConfigStore.GetAgentConfigFile() [local.go]: invalid subdir: %s", subdir)
	}

	path := filepath.Join(s.configDir, "agent", cleanSubdir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LocalConfigStore.GetAgentConfigFile() [local.go]: failed to read %s: %w", path, err)
	}

	return data, nil
}

// loadAllConfig loads all configuration from disk.
func (s *LocalConfigStore) loadAllConfig() error {
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
	return nil
}

// loadGlobalConfig loads the global configuration from global.yml/global.yaml or global.json.
// YAML takes precedence over JSON if both exist.
func (s *LocalConfigStore) loadGlobalConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ymlPath := filepath.Join(s.configDir, "global.yml")
	yamlPath := filepath.Join(s.configDir, "global.yaml")
	jsonPath := filepath.Join(s.configDir, "global.json")

	var data []byte
	var configPath string
	var err error

	// Try YAML first (takes precedence)
	data, err = os.ReadFile(ymlPath)
	if err == nil {
		configPath = ymlPath
	} else if os.IsNotExist(err) {
		data, err = os.ReadFile(yamlPath)
		if err == nil {
			configPath = yamlPath
		} else if os.IsNotExist(err) {
			// Try JSON if YAML doesn't exist
			data, err = os.ReadFile(jsonPath)
			if err != nil {
				if os.IsNotExist(err) {
					// Global config is optional, use empty config
					s.globalConfig = &conf.GlobalConfig{}
					return nil
				}
				return fmt.Errorf("loadGlobalConfig(): failed to read %s: %w", jsonPath, err)
			}
			configPath = jsonPath
		} else {
			return fmt.Errorf("loadGlobalConfig(): failed to read %s: %w", yamlPath, err)
		}
	} else {
		return fmt.Errorf("loadGlobalConfig(): failed to read %s: %w", ymlPath, err)
	}

	var config conf.GlobalConfig
	if strings.HasSuffix(configPath, ".yml") || strings.HasSuffix(configPath, ".yaml") {
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("loadGlobalConfig(): failed to parse %s: %w", configPath, err)
		}
	} else {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("loadGlobalConfig(): failed to parse %s: %w", configPath, err)
		}
	}

	s.globalConfig = &config

	return nil
}

func (s *LocalConfigStore) loadModelProviderConfigMapDir(dir string) (map[string]conf.ModelProviderConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
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
		if ext != ".yml" && ext != ".yaml" && ext != ".json" {
			continue
		}

		key := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("loadModelProviderConfigMapDir(): failed to read %s: %w", path, err)
		}

		var item conf.ModelProviderConfig
		if ext == ".json" {
			if err := json.Unmarshal(data, &item); err != nil {
				return nil, fmt.Errorf("loadModelProviderConfigMapDir(): failed to parse %s: %w", path, err)
			}
		} else {
			if err := yaml.Unmarshal(data, &item); err != nil {
				return nil, fmt.Errorf("loadModelProviderConfigMapDir(): failed to parse %s: %w", path, err)
			}
		}
		result[key] = item
	}

	return result, nil
}

func (s *LocalConfigStore) loadModelTemplateGroupsDir(dir string) (map[string]map[string]conf.ModelProviderConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
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
		if ext != ".yml" && ext != ".yaml" && ext != ".json" {
			continue
		}

		group := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("loadModelTemplateGroupsDir(): failed to read %s: %w", path, err)
		}

		items := make(map[string]conf.ModelProviderConfig)
		if ext == ".json" {
			if err := json.Unmarshal(data, &items); err != nil {
				return nil, fmt.Errorf("loadModelTemplateGroupsDir(): failed to parse %s: %w", path, err)
			}
		} else {
			if err := yaml.Unmarshal(data, &items); err != nil {
				return nil, fmt.Errorf("loadModelTemplateGroupsDir(): failed to parse %s: %w", path, err)
			}
		}
		result[group] = items
	}

	return result, nil
}

// loadModelProviderConfigs loads all model provider configurations from the models directory.
// Supports both .json and .yml/.yaml files, with YAML taking precedence over JSON if both exist.
func (s *LocalConfigStore) loadModelProviderConfigs() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	modelsDir := filepath.Join(s.configDir, "models")

	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Models directory is optional
			s.modelProviderConfigs = make(map[string]*conf.ModelProviderConfig)
			return nil
		}
		return fmt.Errorf("loadModelProviderConfigs(): failed to read models directory: %w", err)
	}

	configs := make(map[string]*conf.ModelProviderConfig)

	selectedByProviderName := make(map[string]os.DirEntry)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !isAllowedModelProviderConfigExt(ext) {
			continue
		}

		providerName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		existingEntry, exists := selectedByProviderName[providerName]
		if !exists {
			selectedByProviderName[providerName] = entry
			continue
		}

		existingExt := strings.ToLower(filepath.Ext(existingEntry.Name()))
		if modelProviderConfigExtPriority(ext) < modelProviderConfigExtPriority(existingExt) {
			selectedByProviderName[providerName] = entry
		}
	}

	for providerName, entry := range selectedByProviderName {
		providerPath := filepath.Join(modelsDir, entry.Name())
		data, err := os.ReadFile(providerPath)
		if err != nil {
			return fmt.Errorf("loadModelProviderConfigs(): failed to read %s: %w", providerPath, err)
		}

		var config conf.ModelProviderConfig
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".json" {
			if err := json.Unmarshal(data, &config); err != nil {
				return fmt.Errorf("loadModelProviderConfigs(): failed to parse %s: %w", providerPath, err)
			}
		} else {
			if err := yaml.Unmarshal(data, &config); err != nil {
				return fmt.Errorf("loadModelProviderConfigs(): failed to parse %s: %w", providerPath, err)
			}
		}

		config.Name = providerName
		configs[providerName] = &config
	}

	s.modelProviderConfigs = configs

	return nil
}

func isAllowedModelProviderConfigExt(ext string) bool {
	switch ext {
	case ".yml", ".yaml", ".json", ".conf":
		return true
	default:
		return false
	}
}

func modelProviderConfigExtPriority(ext string) int {
	switch ext {
	case ".yml":
		return 1
	case ".yaml":
		return 2
	case ".json":
		return 3
	case ".conf":
		return 4
	default:
		return 100
	}
}

// loadModelAliases loads model aliases from model_aliases.yaml/yml/jsonl files.
// YAML takes precedence over JSONL.
func (s *LocalConfigStore) loadModelAliases() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	yamlPath := filepath.Join(s.configDir, "model_aliases.yaml")
	ymlPath := filepath.Join(s.configDir, "model_aliases.yml")
	jsonlPath := filepath.Join(s.configDir, "model_aliases.jsonl")

	aliases, loaded, err := loadModelAliasesFromFiles(yamlPath, ymlPath, jsonlPath)
	if err != nil {
		return err
	}
	if !loaded {
		aliases = make(map[string]conf.ModelAliasValue)
	}

	s.modelAliases = aliases

	return nil
}

func loadModelAliasesFromFiles(yamlPath string, ymlPath string, jsonlPath string) (map[string]conf.ModelAliasValue, bool, error) {
	if data, err := os.ReadFile(yamlPath); err == nil {
		aliases, parseErr := parseModelAliasesYAML(data, yamlPath)
		if parseErr != nil {
			return nil, false, parseErr
		}
		return aliases, true, nil
	} else if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("loadModelAliasesFromFiles() [local.go]: failed to read %s: %w", yamlPath, err)
	}

	if data, err := os.ReadFile(ymlPath); err == nil {
		aliases, parseErr := parseModelAliasesYAML(data, ymlPath)
		if parseErr != nil {
			return nil, false, parseErr
		}
		return aliases, true, nil
	} else if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("loadModelAliasesFromFiles() [local.go]: failed to read %s: %w", ymlPath, err)
	}

	if data, err := os.ReadFile(jsonlPath); err == nil {
		aliases, parseErr := parseModelAliasesJSONL(data, jsonlPath)
		if parseErr != nil {
			return nil, false, parseErr
		}
		return aliases, true, nil
	} else if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("loadModelAliasesFromFiles() [local.go]: failed to read %s: %w", jsonlPath, err)
	}

	return nil, false, nil
}

func parseModelAliasesYAML(data []byte, source string) (map[string]conf.ModelAliasValue, error) {
	aliases := make(map[string]conf.ModelAliasValue)
	if err := yaml.Unmarshal(data, &aliases); err != nil {
		return nil, fmt.Errorf("parseModelAliasesYAML() [local.go]: failed to parse %s: %w", source, err)
	}

	return aliases, nil
}

func parseModelAliasesJSONL(data []byte, source string) (map[string]conf.ModelAliasValue, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return map[string]conf.ModelAliasValue{}, nil
	}

	aliases := make(map[string]conf.ModelAliasValue)
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &aliases); err != nil {
			return nil, fmt.Errorf("parseModelAliasesJSONL() [local.go]: failed to parse %s: %w", source, err)
		}
		return aliases, nil
	}

	lines := strings.Split(trimmed, "\n")
	for lineIndex, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		entry := make(map[string]conf.ModelAliasValue)
		if err := json.Unmarshal([]byte(trimmedLine), &entry); err != nil {
			return nil, fmt.Errorf("parseModelAliasesJSONL() [local.go]: failed to parse %s line %d: %w", source, lineIndex+1, err)
		}
		for key, value := range entry {
			aliases[key] = value
		}
	}

	return aliases, nil
}

// loadAgentRoleConfigs loads all agent role configurations from the roles directory.
// The special "all" meta-role is loaded without requiring config.json, as it only contains
// prompt fragments that are merged into other roles.
// Supports both config.yaml and config.json, with YAML taking precedence.
func (s *LocalConfigStore) loadAgentRoleConfigs() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rolesDir := filepath.Join(s.configDir, "roles")

	entries, err := os.ReadDir(rolesDir)
	if err != nil {
		if os.IsNotExist(err) {
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
		roleDir := filepath.Join(rolesDir, roleName)

		// Try YAML first (takes precedence), then JSON
		yamlPath := filepath.Join(roleDir, "config.yaml")
		ymlPath := filepath.Join(roleDir, "config.yml")
		jsonPath := filepath.Join(roleDir, "config.json")

		var data []byte
		var configPath string
		var useYAML bool

		// Try config.yaml first
		data, err = os.ReadFile(yamlPath)
		if err == nil {
			configPath = yamlPath
			useYAML = true
		} else if os.IsNotExist(err) {
			// Try config.yml next
			data, err = os.ReadFile(ymlPath)
			if err == nil {
				configPath = ymlPath
				useYAML = true
			} else if os.IsNotExist(err) {
				// Try config.json last
				data, err = os.ReadFile(jsonPath)
				if err != nil {
					if os.IsNotExist(err) {
						// Special case: "all" is a meta-role that only contains prompt fragments
						// to be merged into other roles. It doesn't require a config file.
						if roleName == "all" {
							// Load only prompt fragments for the "all" meta-role
							promptFragments, err := s.loadPromptFragments(roleDir)
							if err != nil {
								return fmt.Errorf("loadAgentRoleConfigs(): failed to load prompt fragments for role %s: %w", roleName, err)
							}
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
						return fmt.Errorf("loadAgentRoleConfigs(): role %s missing config.json or config.yaml", roleName)
					}
					return fmt.Errorf("loadAgentRoleConfigs(): failed to read %s: %w", jsonPath, err)
				}
				configPath = jsonPath
				useYAML = false
			} else {
				return fmt.Errorf("loadAgentRoleConfigs(): failed to read %s: %w", ymlPath, err)
			}
		} else {
			return fmt.Errorf("loadAgentRoleConfigs(): failed to read %s: %w", yamlPath, err)
		}

		var config conf.AgentRoleConfig
		if useYAML {
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
		promptFragments, err := s.loadPromptFragments(roleDir)
		if err != nil {
			return fmt.Errorf("loadAgentRoleConfigs(): failed to load prompt fragments for role %s: %w", roleName, err)
		}
		config.PromptFragments = promptFragments

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
func (s *LocalConfigStore) loadPromptFragments(roleDir string) (map[string]string, error) {
	fragments := make(map[string]string)

	entries, err := os.ReadDir(roleDir)
	if err != nil {
		return nil, fmt.Errorf("loadPromptFragments(): failed to read role directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		fragmentPath := filepath.Join(roleDir, entry.Name())
		data, err := os.ReadFile(fragmentPath)
		if err != nil {
			return nil, fmt.Errorf("loadPromptFragments(): failed to read %s: %w", fragmentPath, err)
		}

		// Use filename without extension as the key
		fragmentName := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		fragments[fragmentName] = string(data)
	}

	return fragments, nil
}

// loadToolFragments loads all tool description/config files from conf/tools.
// Returns a map with keys in the form "<tool-name>/<file-name>".
func (s *LocalConfigStore) loadToolFragments() (map[string]string, error) {
	fragments := make(map[string]string)
	toolsDir := filepath.Join(s.configDir, "tools")

	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fragments, nil
		}
		return nil, fmt.Errorf("loadToolFragments(): failed to read tools directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		toolName := entry.Name()
		toolDir := filepath.Join(toolsDir, toolName)
		toolEntries, err := os.ReadDir(toolDir)
		if err != nil {
			return nil, fmt.Errorf("loadToolFragments(): failed to read tool directory %s: %w", toolDir, err)
		}

		fragments[fmt.Sprintf("%s/.tooldir", toolName)] = toolDir

		for _, toolEntry := range toolEntries {
			if toolEntry.IsDir() {
				continue
			}

			path := filepath.Join(toolDir, toolEntry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("loadToolFragments(): failed to read %s: %w", path, err)
			}
			fragments[fmt.Sprintf("%s/%s", toolName, toolEntry.Name())] = string(data)
		}
	}

	return fragments, nil
}

// SaveModelProviderConfig saves or updates a model provider configuration.
func (s *LocalConfigStore) SaveModelProviderConfig(config *conf.ModelProviderConfig) error {
	if config == nil {
		return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: config cannot be nil")
	}
	if config.Name == "" {
		return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: config name cannot be empty")
	}

	modelsDir := filepath.Join(s.configDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to create models directory: %w", err)
	}

	yamlPath := filepath.Join(modelsDir, config.Name+".yaml")
	ymlPath := filepath.Join(modelsDir, config.Name+".yml")
	jsonPath := filepath.Join(modelsDir, config.Name+".json")
	confPath := filepath.Join(modelsDir, config.Name+".conf")

	providerPath := jsonPath
	marshalYAML := false
	if _, err := os.Stat(yamlPath); err == nil {
		providerPath = yamlPath
		marshalYAML = true
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to stat %s: %w", yamlPath, err)
	} else if _, err := os.Stat(ymlPath); err == nil {
		providerPath = ymlPath
		marshalYAML = true
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to stat %s: %w", ymlPath, err)
	} else if _, err := os.Stat(jsonPath); err == nil {
		providerPath = jsonPath
		marshalYAML = false
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to stat %s: %w", jsonPath, err)
	} else if _, err := os.Stat(confPath); err == nil {
		providerPath = confPath
		marshalYAML = true
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to stat %s: %w", confPath, err)
	}

	if existingData, err := os.ReadFile(providerPath); err == nil {
		if err := os.WriteFile(providerPath+".bkp", existingData, 0644); err != nil {
			return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to create backup file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to read existing config file: %w", err)
	}

	var (
		data []byte
		err  error
	)
	if marshalYAML {
		data, err = yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to marshal config as yaml: %w", err)
		}
	} else {
		data, err = json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to marshal config as json: %w", err)
		}
	}

	if err := os.WriteFile(providerPath, data, 0644); err != nil {
		return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to write config file: %w", err)
	}

	// Reload configuration to update cache
	if err := s.loadModelProviderConfigs(); err != nil {
		return fmt.Errorf("LocalConfigStore.SaveModelProviderConfig() [local.go]: failed to reload configs: %w", err)
	}

	return nil
}

// SaveGlobalConfig saves global configuration.
func (s *LocalConfigStore) SaveGlobalConfig(config *conf.GlobalConfig) error {
	if config == nil {
		return fmt.Errorf("LocalConfigStore.SaveGlobalConfig() [local.go]: config cannot be nil")
	}

	if err := os.MkdirAll(s.configDir, 0755); err != nil {
		return fmt.Errorf("LocalConfigStore.SaveGlobalConfig() [local.go]: failed to create config directory: %w", err)
	}

	globalPath := filepath.Join(s.configDir, "global.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("LocalConfigStore.SaveGlobalConfig() [local.go]: failed to marshal config: %w", err)
	}

	if err := os.WriteFile(globalPath, data, 0644); err != nil {
		return fmt.Errorf("LocalConfigStore.SaveGlobalConfig() [local.go]: failed to write config file: %w", err)
	}

	// Reload configuration to update cache
	if err := s.loadGlobalConfig(); err != nil {
		return fmt.Errorf("LocalConfigStore.SaveGlobalConfig() [local.go]: failed to reload config: %w", err)
	}

	return nil
}

// validateConfigDir checks if the configuration directory exists and is readable.
func validateConfigDir(configDir string) error {
	info, err := os.Stat(configDir)
	if err != nil {
		return fmt.Errorf("validateConfigDir(): %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("validateConfigDir(): %s is not a directory", configDir)
	}
	return nil
}
