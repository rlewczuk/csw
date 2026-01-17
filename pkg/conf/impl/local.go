// Package impl provides a filesystem-based configuration store implementation
// that satisfies the conf.ConfigStore interface.
//
// The local config store reads configuration from a directory structure:
//   - global.json - global configuration
//   - models/*.json - model provider configurations (one file per provider)
//   - roles/*/config.json - agent role configurations (one directory per role)
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
	"sync"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/fsnotify/fsnotify"
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
		ModelTags: make([]conf.ModelTagMapping, len(s.globalConfig.ModelTags)),
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

// loadGlobalConfig loads the global configuration from global.json.
func (s *LocalConfigStore) loadGlobalConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	globalPath := filepath.Join(s.configDir, "global.json")

	data, err := os.ReadFile(globalPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Global config is optional, use empty config
			s.globalConfig = &conf.GlobalConfig{}
			s.globalConfigUpdate = time.Now()
			return nil
		}
		return fmt.Errorf("loadGlobalConfig(): failed to read %s: %w", globalPath, err)
	}

	var config conf.GlobalConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("loadGlobalConfig(): failed to parse %s: %w", globalPath, err)
	}

	s.globalConfig = &config
	s.globalConfigUpdate = time.Now()

	return nil
}

// loadModelProviderConfigs loads all model provider configurations from the models directory.
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
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
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
			config.Name = entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		}

		configs[config.Name] = &config
	}

	s.modelProviderConfigs = configs
	s.modelProviderConfigsUpdate = time.Now()

	return nil
}

// loadAgentRoleConfigs loads all agent role configurations from the roles directory.
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
		configPath := filepath.Join(rolesDir, roleName, "config.json")

		data, err := os.ReadFile(configPath)
		if err != nil {
			if os.IsNotExist(err) {
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

		// Load prompt fragments from .md files in the role directory
		promptFragments, err := s.loadPromptFragments(filepath.Join(rolesDir, roleName))
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
	// Watch global.json
	globalPath := filepath.Join(s.configDir, "global.json")
	if _, err := os.Stat(globalPath); err == nil {
		if err := s.watcher.Add(globalPath); err != nil {
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

	globalPath := filepath.Join(s.configDir, "global.json")
	modelsDir := filepath.Join(s.configDir, "models")
	rolesDir := filepath.Join(s.configDir, "roles")

	// Check if it's global.json
	if event.Name == globalPath {
		if err := s.loadGlobalConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to reload global config: %v\n", err)
		}
		return
	}

	// Check if it's in models directory
	if filepath.Dir(event.Name) == modelsDir && filepath.Ext(event.Name) == ".json" {
		if err := s.loadModelProviderConfigs(); err != nil {
			fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to reload model provider configs: %v\n", err)
		}
		return
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

	// Check if it's in a role directory (config.json or .md file)
	eventDir := filepath.Dir(event.Name)
	eventExt := filepath.Ext(event.Name)
	if filepath.Dir(eventDir) == rolesDir && (filepath.Base(event.Name) == "config.json" || eventExt == ".md") {
		if err := s.loadAgentRoleConfigs(); err != nil {
			fmt.Fprintf(os.Stderr, "LocalConfigStore.handleFileEvent(): failed to reload agent role configs: %v\n", err)
		}
		return
	}
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
