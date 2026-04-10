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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/vfs"
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
	modelAliases         map[string]conf.ModelAliasValue
	mcpServerConfigs     map[string]*conf.MCPServerConfig
	hookConfigs          map[string]*conf.HookConfig
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
		mcpServerConfigs:     make(map[string]*conf.MCPServerConfig),
		hookConfigs:          make(map[string]*conf.HookConfig),
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
		configs[k] = v.Clone()
	}

	return configs, nil
}

// LastModelProviderConfigsUpdate returns the timestamp of the last model provider configs update.
// For embedded configuration, this always returns a constant timestamp.
func (s *EmbeddedConfigStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	return embeddedTimestamp, nil
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

// LastModelAliasesUpdate returns timestamp of last model aliases update.
func (s *EmbeddedConfigStore) LastModelAliasesUpdate() (time.Time, error) {
	return embeddedTimestamp, nil
}

// GetMCPServerConfigs returns map of MCP server configurations.
func (s *EmbeddedConfigStore) GetMCPServerConfigs() (map[string]*conf.MCPServerConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configs := make(map[string]*conf.MCPServerConfig, len(s.mcpServerConfigs))
	for key, value := range s.mcpServerConfigs {
		configs[key] = value.Clone()
	}

	return configs, nil
}

// LastMCPServerConfigsUpdate returns timestamp of last MCP server config update.
func (s *EmbeddedConfigStore) LastMCPServerConfigsUpdate() (time.Time, error) {
	return embeddedTimestamp, nil
}

// GetHookConfigs returns map of hook configurations.
func (s *EmbeddedConfigStore) GetHookConfigs() (map[string]*conf.HookConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configs := make(map[string]*conf.HookConfig, len(s.hookConfigs))
	for key, value := range s.hookConfigs {
		configs[key] = value.Clone()
	}

	return configs, nil
}

// LastHookConfigsUpdate returns timestamp of last hook config update.
func (s *EmbeddedConfigStore) LastHookConfigsUpdate() (time.Time, error) {
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
	if err := s.loadMCPServerConfigs(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load MCP server configs: %w", err)
	}
	if err := s.loadHookConfigs(); err != nil {
		return fmt.Errorf("loadAllConfig(): failed to load hook configs: %w", err)
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
	yamlPath := "conf/model_aliases.yaml"
	ymlPath := "conf/model_aliases.yml"
	jsonlPath := "conf/model_aliases.jsonl"

	if data, err := embeddedConfigFS.ReadFile(yamlPath); err == nil {
		aliases, parseErr := parseModelAliasesYAML(data, yamlPath)
		if parseErr != nil {
			return nil, false, parseErr
		}
		return aliases, true, nil
	} else if !isNotExist(err) {
		return nil, false, fmt.Errorf("loadEmbeddedModelAliases() [embedded.go]: failed to read %s: %w", yamlPath, err)
	}

	if data, err := embeddedConfigFS.ReadFile(ymlPath); err == nil {
		aliases, parseErr := parseModelAliasesYAML(data, ymlPath)
		if parseErr != nil {
			return nil, false, parseErr
		}
		return aliases, true, nil
	} else if !isNotExist(err) {
		return nil, false, fmt.Errorf("loadEmbeddedModelAliases() [embedded.go]: failed to read %s: %w", ymlPath, err)
	}

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
		if len(config.ShadowPaths) == 0 {
			config.ShadowPaths = append([]string(nil), vfs.DefaultShadowPatterns()...)
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
	if len(config.ShadowPaths) == 0 {
		config.ShadowPaths = append([]string(nil), vfs.DefaultShadowPatterns()...)
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

// loadMCPServerConfigs loads MCP server configurations from embedded mcp directory.
// YAML files take precedence over JSON files if both exist for the same server.
func (s *EmbeddedConfigStore) loadMCPServerConfigs() error {
	mcpDir := "conf/mcp"

	entries, err := embeddedConfigFS.ReadDir(mcpDir)
	if err != nil {
		if isNotExist(err) {
			s.mcpServerConfigs = make(map[string]*conf.MCPServerConfig)
			return nil
		}
		return fmt.Errorf("loadMCPServerConfigs() [embedded.go]: failed to read mcp directory: %w", err)
	}

	configs := make(map[string]*conf.MCPServerConfig)
	loadedServers := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}

		serverName := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		serverPath := filepath.Join(mcpDir, entry.Name())
		data, readErr := embeddedConfigFS.ReadFile(serverPath)
		if readErr != nil {
			return fmt.Errorf("loadMCPServerConfigs() [embedded.go]: failed to read %s: %w", serverPath, readErr)
		}

		var config conf.MCPServerConfig
		if unmarshalErr := yaml.Unmarshal(data, &config); unmarshalErr != nil {
			return fmt.Errorf("loadMCPServerConfigs() [embedded.go]: failed to parse %s: %w", serverPath, unmarshalErr)
		}

		if strings.TrimSpace(config.Name) == "" {
			config.Name = serverName
		}
		configs[serverName] = &config
		loadedServers[serverName] = true
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		serverName := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		if loadedServers[serverName] {
			continue
		}

		serverPath := filepath.Join(mcpDir, entry.Name())
		data, readErr := embeddedConfigFS.ReadFile(serverPath)
		if readErr != nil {
			return fmt.Errorf("loadMCPServerConfigs() [embedded.go]: failed to read %s: %w", serverPath, readErr)
		}

		var config conf.MCPServerConfig
		if unmarshalErr := json.Unmarshal(data, &config); unmarshalErr != nil {
			return fmt.Errorf("loadMCPServerConfigs() [embedded.go]: failed to parse %s: %w", serverPath, unmarshalErr)
		}

		if strings.TrimSpace(config.Name) == "" {
			config.Name = serverName
		}
		configs[serverName] = &config
	}

	s.mcpServerConfigs = configs
	return nil
}

// loadHookConfigs loads hook configurations from embedded hooks directory.
// YAML files take precedence over JSON files if both exist for the same hook.
func (s *EmbeddedConfigStore) loadHookConfigs() error {
	hooksDir := "conf/hooks"

	entries, err := embeddedConfigFS.ReadDir(hooksDir)
	if err != nil {
		if isNotExist(err) {
			s.hookConfigs = make(map[string]*conf.HookConfig)
			return nil
		}
		return fmt.Errorf("loadHookConfigs() [embedded.go]: failed to read hooks directory: %w", err)
	}

	configs := make(map[string]*conf.HookConfig)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		hookDirName := strings.TrimSpace(entry.Name())
		hookDir := filepath.Join(hooksDir, hookDirName)
		hookPath, hookBaseName, hookData, parseAsYAML, embeddedFiles, selectErr := selectEmbeddedHookConfigFile(hookDir, hookDirName)
		if selectErr != nil {
			return fmt.Errorf("loadHookConfigs() [embedded.go]: %w", selectErr)
		}
		if strings.TrimSpace(hookPath) == "" {
			continue
		}

		var hookConfig conf.HookConfig
		if parseAsYAML {
			if unmarshalErr := yaml.Unmarshal(hookData, &hookConfig); unmarshalErr != nil {
				return fmt.Errorf("loadHookConfigs() [embedded.go]: failed to parse %s: %w", hookPath, unmarshalErr)
			}
		} else {
			if unmarshalErr := json.Unmarshal(hookData, &hookConfig); unmarshalErr != nil {
				return fmt.Errorf("loadHookConfigs() [embedded.go]: failed to parse %s: %w", hookPath, unmarshalErr)
			}
		}

		hookConfig.HookDir = hookDir
		hookConfig.EmbeddedFiles = embeddedFiles
		hookConfig.EmbeddedSource = true
		nameInConfig := strings.TrimSpace(hookConfig.Name)
		nameMatches := nameInConfig != "" && nameInConfig == hookDirName
		filenameMatches := hookBaseName == hookDirName
		if !nameMatches || !filenameMatches {
			fmt.Fprintf(os.Stderr, "loadHookConfigs() [embedded.go]: warning: disabled hook in %s because hook name/filename mismatch (dir=%q file=%q name=%q)\n", hookDir, hookDirName, hookBaseName, nameInConfig)
			hookConfig.Enabled = false
			hookConfig.Name = hookDirName
		} else {
			hookConfig.Name = nameInConfig
		}

		configs[hookConfig.Name] = &hookConfig
	}

	s.hookConfigs = configs
	return nil
}

func selectEmbeddedHookConfigFile(hookDir string, hookDirName string) (string, string, []byte, bool, map[string][]byte, error) {
	entries, err := embeddedConfigFS.ReadDir(hookDir)
	if err != nil {
		return "", "", nil, false, nil, fmt.Errorf("selectEmbeddedHookConfigFile() [embedded.go]: failed to read %s: %w", hookDir, err)
	}

	additionalFiles := make(map[string][]byte)
	candidatePath := ""
	candidateBase := ""
	candidateYAML := false
	var fallbackPath string
	var fallbackBase string
	var fallbackYAML bool

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		path := filepath.Join(hookDir, entry.Name())
		if ext != ".yml" && ext != ".yaml" && ext != ".json" {
			data, readErr := embeddedConfigFS.ReadFile(path)
			if readErr != nil {
				return "", "", nil, false, nil, fmt.Errorf("selectEmbeddedHookConfigFile() [embedded.go]: failed to read %s: %w", path, readErr)
			}
			additionalFiles[entry.Name()] = data
			continue
		}

		baseName := strings.TrimSuffix(entry.Name(), ext)
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
		data, readErr := embeddedConfigFS.ReadFile(candidatePath)
		if readErr != nil {
			return "", "", nil, false, nil, fmt.Errorf("selectEmbeddedHookConfigFile() [embedded.go]: failed to read %s: %w", candidatePath, readErr)
		}
		return candidatePath, candidateBase, data, candidateYAML, additionalFiles, nil
	}

	if fallbackPath == "" {
		return "", "", nil, false, additionalFiles, nil
	}

	data, readErr := embeddedConfigFS.ReadFile(fallbackPath)
	if readErr != nil {
		return "", "", nil, false, nil, fmt.Errorf("selectEmbeddedHookConfigFile() [embedded.go]: failed to read %s: %w", fallbackPath, readErr)
	}

	return fallbackPath, fallbackBase, data, fallbackYAML, additionalFiles, nil
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
		if ext != ".yml" && ext != ".yaml" && ext != ".json" {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		path := filepath.Join(dir, entry.Name())
		data, err := embeddedConfigFS.ReadFile(path)
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
		if ext != ".yml" && ext != ".yaml" && ext != ".json" {
			continue
		}
		group := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		path := filepath.Join(dir, entry.Name())
		data, err := embeddedConfigFS.ReadFile(path)
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
