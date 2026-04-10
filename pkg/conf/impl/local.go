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
// The store caches configuration in memory and watches for file changes
// to automatically reload when configuration files are modified.
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rlewczuk/csw/pkg/conf"
	"gopkg.in/yaml.v3"
)

// LocalConfigStore implements conf.ConfigStore interface for local filesystem-based configuration.
// It caches configuration in memory and watches for file changes to reload automatically.
type LocalConfigStore struct {
	configDir string

	mu                         sync.RWMutex
	globalConfig               *conf.GlobalConfig
	globalConfigUpdate         time.Time
	modelProviderConfigs       map[string]*conf.ModelProviderConfig
	modelProviderConfigsUpdate time.Time
	modelAliases               map[string]conf.ModelAliasValue
	modelAliasesUpdate         time.Time
	mcpServerConfigs           map[string]*conf.MCPServerConfig
	mcpServerConfigsUpdate     time.Time
	hookConfigs                map[string]*conf.HookConfig
	hookConfigsUpdate          time.Time
	agentRoleConfigs           map[string]*conf.AgentRoleConfig
	agentRoleConfigsUpdate     time.Time

	watcher       *fsnotify.Watcher
	watcherCtx    context.Context
	watcherCancel context.CancelFunc
	watcherWg     sync.WaitGroup
}

// NewLocalConfigStore creates a new LocalConfigStore instance that reads configuration
// from the specified directory and watches for changes.
func NewLocalConfigStore(configDir string) (*LocalConfigStore, error) {
	if err := validateConfigDir(configDir); err != nil {
		return nil, fmt.Errorf("NewLocalConfigStore(): invalid config directory: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		if !errors.Is(err, syscall.EMFILE) && !errors.Is(err, syscall.ENFILE) {
			return nil, fmt.Errorf("NewLocalConfigStore(): failed to create file watcher: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	store := &LocalConfigStore{
		configDir:            configDir,
		globalConfig:         &conf.GlobalConfig{},
		modelProviderConfigs: make(map[string]*conf.ModelProviderConfig),
		modelAliases:         make(map[string]conf.ModelAliasValue),
		mcpServerConfigs:     make(map[string]*conf.MCPServerConfig),
		hookConfigs:          make(map[string]*conf.HookConfig),
		agentRoleConfigs:     make(map[string]*conf.AgentRoleConfig),
		watcher:              watcher,
		watcherCtx:           ctx,
		watcherCancel:        cancel,
	}

	// Initial load of all configuration
	if err := store.loadAllConfig(); err != nil {
		cancel()
		watcher.Close()
		return nil, fmt.Errorf("NewLocalConfigStore(): failed to load initial configuration: %w", err)
	}

	// Start watching for changes
	if watcher != nil {
		if err := store.setupWatchers(); err != nil {
			cancel()
			watcher.Close()
			return nil, fmt.Errorf("NewLocalConfigStore(): failed to setup file watchers: %w", err)
		}

		store.watcherWg.Add(1)
		go store.watchLoop()
	}

	return store, nil
}

// Close stops the file watcher and releases resources.
func (s *LocalConfigStore) Close() error {
	if s.watcherCancel != nil {
		s.watcherCancel()
	}
	s.watcherWg.Wait()
	if s.watcher == nil {
		return nil
	}
	return s.watcher.Close()
}

// GetGlobalConfig returns the global configuration.
func (s *LocalConfigStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.globalConfig.Clone(), nil
}

// LastGlobalConfigUpdate returns the timestamp of the last global config update.
func (s *LocalConfigStore) LastGlobalConfigUpdate() (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.globalConfigUpdate, nil
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

// LastModelProviderConfigsUpdate returns the timestamp of the last model provider configs update.
func (s *LocalConfigStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelProviderConfigsUpdate, nil
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

// LastModelAliasesUpdate returns timestamp of model aliases updates.
func (s *LocalConfigStore) LastModelAliasesUpdate() (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelAliasesUpdate, nil
}

// GetMCPServerConfigs returns a map of MCP server configurations.
func (s *LocalConfigStore) GetMCPServerConfigs() (map[string]*conf.MCPServerConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configs := make(map[string]*conf.MCPServerConfig, len(s.mcpServerConfigs))
	for key, value := range s.mcpServerConfigs {
		configs[key] = value.Clone()
	}

	return configs, nil
}

// LastMCPServerConfigsUpdate returns timestamp of last MCP server configs update.
func (s *LocalConfigStore) LastMCPServerConfigsUpdate() (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mcpServerConfigsUpdate, nil
}

// GetHookConfigs returns a map of hook configurations.
func (s *LocalConfigStore) GetHookConfigs() (map[string]*conf.HookConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configs := make(map[string]*conf.HookConfig, len(s.hookConfigs))
	for key, value := range s.hookConfigs {
		configs[key] = value.Clone()
	}

	return configs, nil
}

// LastHookConfigsUpdate returns timestamp of last hook configs update.
func (s *LocalConfigStore) LastHookConfigsUpdate() (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hookConfigsUpdate, nil
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

// LastAgentRoleConfigsUpdate returns the timestamp of the last agent role configs update.
func (s *LocalConfigStore) LastAgentRoleConfigsUpdate() (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agentRoleConfigsUpdate, nil
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
	if err := s.loadModelTemplateConfig(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load model template config: %w", err)
	}
	if err := s.loadModelProviderConfigs(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load model provider configs: %w", err)
	}
	if err := s.loadModelAliases(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load model aliases: %w", err)
	}
	if err := s.loadMCPServerConfigs(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load MCP server configs: %w", err)
	}
	if err := s.loadHookConfigs(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load hook configs: %w", err)
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
					s.globalConfigUpdate = time.Now()
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
	s.globalConfigUpdate = time.Now()

	return nil
}

func (s *LocalConfigStore) loadModelTemplateConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	families, err := s.loadModelProviderConfigMapDir(filepath.Join(s.configDir, "models", "families"))
	if err != nil {
		return fmt.Errorf("loadModelTemplateConfig(): failed to load families: %w", err)
	}
	vendors, err := s.loadModelProviderConfigMapDir(filepath.Join(s.configDir, "models", "vendors"))
	if err != nil {
		return fmt.Errorf("loadModelTemplateConfig(): failed to load vendors: %w", err)
	}
	templates, err := s.loadModelTemplateGroupsDir(filepath.Join(s.configDir, "models", "templates"))
	if err != nil {
		return fmt.Errorf("loadModelTemplateConfig(): failed to load templates: %w", err)
	}

	s.globalConfig.ModelFamilies = families
	s.globalConfig.ModelVendors = vendors
	s.globalConfig.ModelTemplates = templates

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
			s.modelProviderConfigsUpdate = time.Now()
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
	s.modelProviderConfigsUpdate = time.Now()

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
	s.modelAliasesUpdate = time.Now()

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

// loadMCPServerConfigs loads MCP server configurations from mcp directory.
// Supports .json and .yml/.yaml files, with YAML taking precedence.
func (s *LocalConfigStore) loadMCPServerConfigs() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mcpDir := filepath.Join(s.configDir, "mcp")

	entries, err := os.ReadDir(mcpDir)
	if err != nil {
		if os.IsNotExist(err) {
			s.mcpServerConfigs = make(map[string]*conf.MCPServerConfig)
			s.mcpServerConfigsUpdate = time.Now()
			return nil
		}
		return fmt.Errorf("loadMCPServerConfigs() [local.go]: failed to read mcp directory: %w", err)
	}

	loadedServers := make(map[string]bool)
	configs := make(map[string]*conf.MCPServerConfig)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yml" && ext != ".yaml" {
			continue
		}

		serverPath := filepath.Join(mcpDir, entry.Name())
		data, readErr := os.ReadFile(serverPath)
		if readErr != nil {
			return fmt.Errorf("loadMCPServerConfigs() [local.go]: failed to read %s: %w", serverPath, readErr)
		}

		var config conf.MCPServerConfig
		if unmarshalErr := yaml.Unmarshal(data, &config); unmarshalErr != nil {
			return fmt.Errorf("loadMCPServerConfigs() [local.go]: failed to parse %s: %w", serverPath, unmarshalErr)
		}

		baseName := entry.Name()[:len(entry.Name())-len(ext)]
		if strings.TrimSpace(config.Name) == "" {
			config.Name = baseName
		}
		configs[baseName] = &config
		loadedServers[baseName] = true
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		baseName := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		if loadedServers[baseName] {
			continue
		}

		serverPath := filepath.Join(mcpDir, entry.Name())
		data, readErr := os.ReadFile(serverPath)
		if readErr != nil {
			return fmt.Errorf("loadMCPServerConfigs() [local.go]: failed to read %s: %w", serverPath, readErr)
		}

		var config conf.MCPServerConfig
		if unmarshalErr := json.Unmarshal(data, &config); unmarshalErr != nil {
			return fmt.Errorf("loadMCPServerConfigs() [local.go]: failed to parse %s: %w", serverPath, unmarshalErr)
		}

		if strings.TrimSpace(config.Name) == "" {
			config.Name = baseName
		}
		configs[baseName] = &config
	}

	s.mcpServerConfigs = configs
	s.mcpServerConfigsUpdate = time.Now()

	return nil
}

// loadHookConfigs loads hook configurations from hooks directory.
// Supports .json and .yml/.yaml files, with YAML taking precedence.
func (s *LocalConfigStore) loadHookConfigs() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hooksDir := filepath.Join(s.configDir, "hooks")

	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		if os.IsNotExist(err) {
			s.hookConfigs = make(map[string]*conf.HookConfig)
			s.hookConfigsUpdate = time.Now()
			return nil
		}
		return fmt.Errorf("loadHookConfigs() [local.go]: failed to read hooks directory: %w", err)
	}

	configs := make(map[string]*conf.HookConfig)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		hookDirName := strings.TrimSpace(entry.Name())
		hookDir := filepath.Join(hooksDir, hookDirName)
		hookPath, hookBaseName, hookData, parseAsYAML, selectErr := selectLocalHookConfigFile(hookDir, hookDirName)
		if selectErr != nil {
			return fmt.Errorf("loadHookConfigs() [local.go]: %w", selectErr)
		}
		if strings.TrimSpace(hookPath) == "" {
			continue
		}

		var hookConfig conf.HookConfig
		if parseAsYAML {
			if unmarshalErr := yaml.Unmarshal(hookData, &hookConfig); unmarshalErr != nil {
				return fmt.Errorf("loadHookConfigs() [local.go]: failed to parse %s: %w", hookPath, unmarshalErr)
			}
		} else {
			if unmarshalErr := json.Unmarshal(hookData, &hookConfig); unmarshalErr != nil {
				return fmt.Errorf("loadHookConfigs() [local.go]: failed to parse %s: %w", hookPath, unmarshalErr)
			}
		}

		hookConfig.HookDir = hookDir
		nameInConfig := strings.TrimSpace(hookConfig.Name)
		nameMatches := nameInConfig != "" && nameInConfig == hookDirName
		filenameMatches := hookBaseName == hookDirName
		if !nameMatches || !filenameMatches {
			fmt.Fprintf(os.Stderr, "loadHookConfigs() [local.go]: warning: disabled hook in %s because hook name/filename mismatch (dir=%q file=%q name=%q)\n", hookDir, hookDirName, hookBaseName, nameInConfig)
			hookConfig.Enabled = false
			hookConfig.Name = hookDirName
		} else {
			hookConfig.Name = nameInConfig
		}

		configs[hookConfig.Name] = &hookConfig
	}

	s.hookConfigs = configs
	s.hookConfigsUpdate = time.Now()

	return nil
}

func selectLocalHookConfigFile(hookDir string, hookDirName string) (string, string, []byte, bool, error) {
	entries, err := os.ReadDir(hookDir)
	if err != nil {
		return "", "", nil, false, fmt.Errorf("selectLocalHookConfigFile() [local.go]: failed to read %s: %w", hookDir, err)
	}

	var candidatePath string
	var candidateBase string
	var candidateYAML bool
	var fallbackPath string
	var fallbackBase string
	var fallbackYAML bool

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yml" && ext != ".yaml" && ext != ".json" {
			continue
		}
		baseName := strings.TrimSuffix(entry.Name(), ext)
		path := filepath.Join(hookDir, entry.Name())
		isYAML := ext == ".yml" || ext == ".yaml"

		if baseName == hookDirName {
			if candidatePath == "" || (isYAML && !candidateYAML) {
				candidatePath = path
				candidateBase = baseName
				candidateYAML = isYAML
			}
			continue
		}

		if fallbackPath == "" || (isYAML && !fallbackYAML) {
			fallbackPath = path
			fallbackBase = baseName
			fallbackYAML = isYAML
		}
	}

	if candidatePath != "" {
		data, readErr := os.ReadFile(candidatePath)
		if readErr != nil {
			return "", "", nil, false, fmt.Errorf("selectLocalHookConfigFile() [local.go]: failed to read %s: %w", candidatePath, readErr)
		}
		return candidatePath, candidateBase, data, candidateYAML, nil
	}

	if fallbackPath == "" {
		return "", "", nil, false, nil
	}

	data, readErr := os.ReadFile(fallbackPath)
	if readErr != nil {
		return "", "", nil, false, fmt.Errorf("selectLocalHookConfigFile() [local.go]: failed to read %s: %w", fallbackPath, readErr)
	}

	return fallbackPath, fallbackBase, data, fallbackYAML, nil
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
			s.agentRoleConfigsUpdate = time.Now()
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
	s.agentRoleConfigsUpdate = time.Now()

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

// setupWatchers sets up file system watchers for all configuration directories.
func (s *LocalConfigStore) setupWatchers() error {
	// Watch global config files (YAML takes precedence but watch all supported names)
	ymlPath := filepath.Join(s.configDir, "global.yml")
	yamlPath := filepath.Join(s.configDir, "global.yaml")
	jsonPath := filepath.Join(s.configDir, "global.json")
	if _, err := os.Stat(ymlPath); err == nil {
		if err := s.watcher.Add(ymlPath); err != nil {
			return fmt.Errorf("setupWatchers(): failed to watch global.yml: %w", err)
		}
	}
	if _, err := os.Stat(yamlPath); err == nil {
		if err := s.watcher.Add(yamlPath); err != nil {
			return fmt.Errorf("setupWatchers(): failed to watch global.yaml: %w", err)
		}
	}
	if _, err := os.Stat(jsonPath); err == nil {
		if err := s.watcher.Add(jsonPath); err != nil {
			return fmt.Errorf("setupWatchers(): failed to watch global.json: %w", err)
		}
	}

	// Watch models directory
	modelsDir := filepath.Join(s.configDir, "models")
	modelAliasesYAMLPath := filepath.Join(s.configDir, "model_aliases.yaml")
	modelAliasesYMLPath := filepath.Join(s.configDir, "model_aliases.yml")
	modelAliasesJSONLPath := filepath.Join(s.configDir, "model_aliases.jsonl")
	if _, err := os.Stat(modelsDir); err == nil {
		if err := s.watcher.Add(modelsDir); err != nil {
			return fmt.Errorf("setupWatchers(): failed to watch models directory: %w", err)
		}

		for _, nested := range []string{"families", "vendors", "templates"} {
			nestedDir := filepath.Join(modelsDir, nested)
			if _, err := os.Stat(nestedDir); err == nil {
				if err := s.watcher.Add(nestedDir); err != nil {
					return fmt.Errorf("setupWatchers(): failed to watch %s directory: %w", nestedDir, err)
				}
			}
		}
	}

	for _, aliasesPath := range []string{modelAliasesYAMLPath, modelAliasesYMLPath, modelAliasesJSONLPath} {
		if _, err := os.Stat(aliasesPath); err == nil {
			if err := s.watcher.Add(aliasesPath); err != nil {
				return fmt.Errorf("setupWatchers() [local.go]: failed to watch %s: %w", aliasesPath, err)
			}
		}
	}

	// Watch mcp directory
	mcpDir := filepath.Join(s.configDir, "mcp")
	if _, err := os.Stat(mcpDir); err == nil {
		if err := s.watcher.Add(mcpDir); err != nil {
			return fmt.Errorf("setupWatchers() [local.go]: failed to watch mcp directory: %w", err)
		}
	}

	// Watch hooks directory
	hooksDir := filepath.Join(s.configDir, "hooks")
	if _, err := os.Stat(hooksDir); err == nil {
		if err := s.watcher.Add(hooksDir); err != nil {
			return fmt.Errorf("setupWatchers() [local.go]: failed to watch hooks directory: %w", err)
		}

		hookEntries, readErr := os.ReadDir(hooksDir)
		if readErr == nil {
			for _, hookEntry := range hookEntries {
				if !hookEntry.IsDir() {
					continue
				}
				hookSubDir := filepath.Join(hooksDir, hookEntry.Name())
				if addErr := s.watcher.Add(hookSubDir); addErr != nil {
					return fmt.Errorf("setupWatchers() [local.go]: failed to watch hook directory %s: %w", hookSubDir, addErr)
				}
			}
		}
	}

	// Watch roles directory and all role subdirectories
	rolesDir := filepath.Join(s.configDir, "roles")
	if _, err := os.Stat(rolesDir); err == nil {
		if err := s.watcher.Add(rolesDir); err != nil {
			return fmt.Errorf("setupWatchers(): failed to watch roles directory: %w", err)
		}

		// Watch each role subdirectory
		entries, err := os.ReadDir(rolesDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					roleDir := filepath.Join(rolesDir, entry.Name())
					if err := s.watcher.Add(roleDir); err != nil {
						return fmt.Errorf("setupWatchers(): failed to watch role directory %s: %w", roleDir, err)
					}
				}
			}
		}
	}

	// Watch tools directory and tool subdirectories
	toolsDir := filepath.Join(s.configDir, "tools")
	if _, err := os.Stat(toolsDir); err == nil {
		if err := s.watcher.Add(toolsDir); err != nil {
			return fmt.Errorf("setupWatchers(): failed to watch tools directory: %w", err)
		}

		entries, err := os.ReadDir(toolsDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					toolDir := filepath.Join(toolsDir, entry.Name())
					if err := s.watcher.Add(toolDir); err != nil {
						return fmt.Errorf("setupWatchers(): failed to watch tool directory %s: %w", toolDir, err)
					}
				}
			}
		}
	}

	return nil
}

// watchLoop processes file system events and reloads configuration as needed.
func (s *LocalConfigStore) watchLoop() {
	defer s.watcherWg.Done()

	for {
		select {
		case <-s.watcherCtx.Done():
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			s.handleFileEvent(event)
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue watching
			fmt.Fprintf(os.Stderr, "LocalConfigStore.watchLoop(): watcher error: %v\n", err)
		}
	}
}

// handleFileEvent processes a single file system event and reloads affected configuration.
func (s *LocalConfigStore) handleFileEvent(event fsnotify.Event) {
	// Only process Write and Create events
	if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return
	}

	globalYMLPath := filepath.Join(s.configDir, "global.yml")
	globalYAMLPath := filepath.Join(s.configDir, "global.yaml")
	globalJSONPath := filepath.Join(s.configDir, "global.json")
	modelsDir := filepath.Join(s.configDir, "models")
	modelAliasesYAMLPath := filepath.Join(s.configDir, "model_aliases.yaml")
	modelAliasesYMLPath := filepath.Join(s.configDir, "model_aliases.yml")
	modelAliasesJSONLPath := filepath.Join(s.configDir, "model_aliases.jsonl")
	mcpDir := filepath.Join(s.configDir, "mcp")
	hooksDir := filepath.Join(s.configDir, "hooks")
	modelFamiliesDir := filepath.Join(modelsDir, "families")
	modelVendorsDir := filepath.Join(modelsDir, "vendors")
	modelTemplatesDir := filepath.Join(modelsDir, "templates")
	rolesDir := filepath.Join(s.configDir, "roles")
	toolsDir := filepath.Join(s.configDir, "tools")

	// Check if it's global config file (YAML or JSON)
	if event.Name == globalYMLPath || event.Name == globalYAMLPath || event.Name == globalJSONPath {
		if err := s.loadGlobalConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to reload global config: %v\n", err)
		}
		return
	}

	if event.Name == modelAliasesYAMLPath || event.Name == modelAliasesYMLPath || event.Name == modelAliasesJSONLPath {
		if err := s.loadModelAliases(); err != nil {
			fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent() [local.go]: failed to reload model aliases: %v\n", err)
		}
		return
	}

	// Check if it's in models directory
	if filepath.Dir(event.Name) == modelsDir {
		ext := strings.ToLower(filepath.Ext(event.Name))
		if isAllowedModelProviderConfigExt(ext) {
			if err := s.loadModelProviderConfigs(); err != nil {
				fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to reload model provider configs: %v\n", err)
			}
			return
		}
	}

	if filepath.Dir(event.Name) == mcpDir {
		ext := strings.ToLower(filepath.Ext(event.Name))
		if ext == ".json" || ext == ".yml" || ext == ".yaml" {
			if err := s.loadMCPServerConfigs(); err != nil {
				fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent() [local.go]: failed to reload mcp server configs: %v\n", err)
			}
			return
		}
	}

	if filepath.Dir(event.Name) == hooksDir && event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := s.watcher.Add(event.Name); err != nil {
				fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent() [local.go]: failed to watch new hook directory: %v\n", err)
			}
		}
	}

	if filepath.Dir(event.Name) == hooksDir || filepath.Dir(filepath.Dir(event.Name)) == hooksDir {
		ext := strings.ToLower(filepath.Ext(event.Name))
		if ext == ".json" || ext == ".yml" || ext == ".yaml" {
			if err := s.loadHookConfigs(); err != nil {
				fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent() [local.go]: failed to reload hook configs: %v\n", err)
			}
			return
		}
	}

	if filepath.Dir(event.Name) == modelFamiliesDir || filepath.Dir(event.Name) == modelVendorsDir || filepath.Dir(event.Name) == modelTemplatesDir {
		ext := strings.ToLower(filepath.Ext(event.Name))
		if ext == ".json" || ext == ".yml" || ext == ".yaml" {
			if err := s.loadModelTemplateConfig(); err != nil {
				fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to reload model template config: %v\n", err)
			}
			return
		}
	}

	// Check if it's a new role directory
	if filepath.Dir(event.Name) == rolesDir && event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			// Watch the new role directory
			if err := s.watcher.Add(event.Name); err != nil {
				fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to watch new role directory: %v\n", err)
			}
		}
	}

	if filepath.Dir(event.Name) == toolsDir && event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := s.watcher.Add(event.Name); err != nil {
				fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to watch new tool directory: %v\n", err)
			}
		}
	}

	// Check if it's in a role directory (config.yaml, config.yml, config.json, or .md file)
	eventDir := filepath.Dir(event.Name)
	baseName := filepath.Base(event.Name)
	eventExt := strings.ToLower(filepath.Ext(event.Name))
	if filepath.Dir(eventDir) == rolesDir {
		isConfigFile := baseName == "config.yaml" || baseName == "config.yml" || baseName == "config.json"
		isMarkdownFile := eventExt == ".md"
		if isConfigFile || isMarkdownFile {
			if err := s.loadAgentRoleConfigs(); err != nil {
				fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to reload agent role configs: %v\n", err)
			}
			return
		}
	}

	if filepath.Dir(eventDir) == toolsDir || filepath.Dir(filepath.Dir(eventDir)) == toolsDir {
		if err := s.loadAgentRoleConfigs(); err != nil {
			fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to reload agent role configs after tools change: %v\n", err)
		}
		return
	}
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

// DeleteModelProviderConfig deletes a model provider configuration.
func (s *LocalConfigStore) DeleteModelProviderConfig(name string) error {
	if name == "" {
		return fmt.Errorf("LocalConfigStore.DeleteModelProviderConfig() [local.go]: name cannot be empty")
	}

	modelsDir := filepath.Join(s.configDir, "models")
	providerPath := filepath.Join(modelsDir, name+".json")

	if err := os.Remove(providerPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("LocalConfigStore.DeleteModelProviderConfig() [local.go]: provider not found: %s", name)
		}
		return fmt.Errorf("LocalConfigStore.DeleteModelProviderConfig() [local.go]: failed to delete config file: %w", err)
	}

	// Reload configuration to update cache
	if err := s.loadModelProviderConfigs(); err != nil {
		return fmt.Errorf("LocalConfigStore.DeleteModelProviderConfig() [local.go]: failed to reload configs: %w", err)
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
