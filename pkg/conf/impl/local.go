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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		return nil, fmt.Errorf("NewLocalConfigStore(): failed to create file watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	store := &LocalConfigStore{
		configDir:            configDir,
		globalConfig:         &conf.GlobalConfig{},
		modelProviderConfigs: make(map[string]*conf.ModelProviderConfig),
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
	if err := store.setupWatchers(); err != nil {
		cancel()
		watcher.Close()
		return nil, fmt.Errorf("NewLocalConfigStore(): failed to setup file watchers: %w", err)
	}

	store.watcherWg.Add(1)
	go store.watchLoop()

	return store, nil
}

// Close stops the file watcher and releases resources.
func (s *LocalConfigStore) Close() error {
	s.watcherCancel()
	s.watcherWg.Wait()
	return s.watcher.Close()
}

// GetGlobalConfig returns the global configuration.
func (s *LocalConfigStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	config := &conf.GlobalConfig{
		DefaultProvider: s.globalConfig.DefaultProvider,
		DefaultRole:     s.globalConfig.DefaultRole,
		ModelTags:       make([]conf.ModelTagMapping, len(s.globalConfig.ModelTags)),
	}
	copy(config.ModelTags, s.globalConfig.ModelTags)

	return config, nil
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
		configCopy := *v
		configCopy.ModelTags = make([]conf.ModelTagMapping, len(v.ModelTags))
		copy(configCopy.ModelTags, v.ModelTags)
		configs[k] = &configCopy
	}

	return configs, nil
}

// LastModelProviderConfigsUpdate returns the timestamp of the last model provider configs update.
func (s *LocalConfigStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelProviderConfigsUpdate, nil
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

// loadAllConfig loads all configuration from disk.
func (s *LocalConfigStore) loadAllConfig() error {
	if err := s.loadGlobalConfig(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load global config: %w", err)
	}
	if err := s.loadModelProviderConfigs(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load model provider configs: %w", err)
	}
	if err := s.loadAgentRoleConfigs(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load agent role configs: %w", err)
	}
	return nil
}

// loadGlobalConfig loads the global configuration from global.yml or global.json.
// YAML takes precedence over JSON if both exist.
func (s *LocalConfigStore) loadGlobalConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	yamlPath := filepath.Join(s.configDir, "global.yml")
	jsonPath := filepath.Join(s.configDir, "global.json")

	var data []byte
	var configPath string
	var err error

	// Try YAML first (takes precedence)
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

	// Track which provider names have been loaded (YAML takes precedence)
	loadedProviders := make(map[string]bool)
	configs := make(map[string]*conf.ModelProviderConfig)

	// First pass: load YAML files (takes precedence)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yml" && ext != ".yaml" {
			continue
		}

		providerPath := filepath.Join(modelsDir, entry.Name())
		data, err := os.ReadFile(providerPath)
		if err != nil {
			return fmt.Errorf("loadModelProviderConfigs(): failed to read %s: %w", providerPath, err)
		}

		var config conf.ModelProviderConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("loadModelProviderConfigs(): failed to parse %s: %w", providerPath, err)
		}

		// If name is not set, use filename without extension
		baseName := entry.Name()[:len(entry.Name())-len(ext)]
		if config.Name == "" {
			config.Name = baseName
		}

		configs[config.Name] = &config
		loadedProviders[baseName] = true
	}

	// Second pass: load JSON files (only if no YAML version exists)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		baseName := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		if loadedProviders[baseName] {
			// Skip JSON if YAML version was already loaded
			continue
		}

		providerPath := filepath.Join(modelsDir, entry.Name())
		data, err := os.ReadFile(providerPath)
		if err != nil {
			return fmt.Errorf("loadModelProviderConfigs(): failed to read %s: %w", providerPath, err)
		}

		var config conf.ModelProviderConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("loadModelProviderConfigs(): failed to parse %s: %w", providerPath, err)
		}

		// If name is not set, use filename without extension
		if config.Name == "" {
			config.Name = baseName
		}

		configs[config.Name] = &config
	}

	s.modelProviderConfigs = configs
	s.modelProviderConfigsUpdate = time.Now()

	return nil
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
							configs["all"] = &conf.AgentRoleConfig{
								Name:            "all",
								PromptFragments: promptFragments,
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

// setupWatchers sets up file system watchers for all configuration directories.
func (s *LocalConfigStore) setupWatchers() error {
	// Watch global config files (YAML takes precedence but watch both)
	yamlPath := filepath.Join(s.configDir, "global.yml")
	jsonPath := filepath.Join(s.configDir, "global.json")
	if _, err := os.Stat(yamlPath); err == nil {
		if err := s.watcher.Add(yamlPath); err != nil {
			return fmt.Errorf("setupWatchers(): failed to watch global.yml: %w", err)
		}
	}
	if _, err := os.Stat(jsonPath); err == nil {
		if err := s.watcher.Add(jsonPath); err != nil {
			return fmt.Errorf("setupWatchers(): failed to watch global.json: %w", err)
		}
	}

	// Watch models directory
	modelsDir := filepath.Join(s.configDir, "models")
	if _, err := os.Stat(modelsDir); err == nil {
		if err := s.watcher.Add(modelsDir); err != nil {
			return fmt.Errorf("setupWatchers(): failed to watch models directory: %w", err)
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

	globalYAMLPath := filepath.Join(s.configDir, "global.yml")
	globalJSONPath := filepath.Join(s.configDir, "global.json")
	modelsDir := filepath.Join(s.configDir, "models")
	rolesDir := filepath.Join(s.configDir, "roles")

	// Check if it's global config file (YAML or JSON)
	if event.Name == globalYAMLPath || event.Name == globalJSONPath {
		if err := s.loadGlobalConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to reload global config: %v\n", err)
		}
		return
	}

	// Check if it's in models directory
	if filepath.Dir(event.Name) == modelsDir {
		ext := strings.ToLower(filepath.Ext(event.Name))
		if ext == ".json" || ext == ".yml" || ext == ".yaml" {
			if err := s.loadModelProviderConfigs(); err != nil {
				fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to reload model provider configs: %v\n", err)
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
