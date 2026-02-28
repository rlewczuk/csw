package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
)

// ConfigScope represents the scope of configuration (global or local).
type ConfigScope string

const (
	ConfigScopeLocal  ConfigScope = "local"
	ConfigScopeGlobal ConfigScope = "global"
)

// GetConfigStore returns a writable config store for the specified scope or custom path.
// For local scope, it uses .csw in the current directory.
// For global scope, it uses ~/.config/csw.
// For custom path, it uses the provided path directly.
func GetConfigStore(scope ConfigScope) (conf.WritableConfigStore, error) {
	var configDir string

	switch scope {
	case ConfigScopeLocal:
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("GetConfigStore() [common.go]: failed to get current directory: %w", err)
		}
		configDir = filepath.Join(cwd, ".csw", "config")
	case ConfigScopeGlobal:
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("GetConfigStore() [common.go]: failed to get home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config", "csw")
	default:
		// Treat as custom path
		configDir = string(scope)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("GetConfigStore() [common.go]: failed to create config directory: %w", err)
	}

	store, err := impl.NewLocalConfigStore(configDir)
	if err != nil {
		return nil, fmt.Errorf("GetConfigStore() [common.go]: failed to create config store: %w", err)
	}

	return store, nil
}

// GetCompositeConfigStore returns a composite config store that merges configurations
// from all paths used by the configuration system.
func GetCompositeConfigStore() (conf.ConfigStore, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("GetCompositeConfigStore() [common.go]: failed to get current directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("GetCompositeConfigStore() [common.go]: failed to get home directory: %w", err)
	}

	// Build config path hierarchy matching main.go
	configPathStr := "@DEFAULTS:" + filepath.Join(homeDir, ".config", "csw") + ":./.csw/config"

	store, err := impl.NewCompositeConfigStore(cwd, configPathStr)
	if err != nil {
		return nil, fmt.Errorf("GetCompositeConfigStore() [common.go]: failed to create composite config store: %w", err)
	}

	return store, nil
}

// BuildConfigPath builds a config path hierarchy string from the base path and optional custom paths.
// Returns a string in the format: "@DEFAULTS:~/.config/csw:<project-config>[:custom-paths]"
// If projectConfig is empty, defaults to "./.csw/config".
func BuildConfigPath(projectConfig, customConfigPath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("BuildConfigPath() [common.go]: failed to get user home directory: %w", err)
	}

	// Determine project config path
	projectConfigPath := "./.csw/config"
	if projectConfig != "" {
		// Validate project config directory exists and is a directory
		info, err := os.Stat(projectConfig)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("BuildConfigPath() [common.go]: project config directory does not exist: %s", projectConfig)
			}
			return "", fmt.Errorf("BuildConfigPath() [common.go]: failed to access project config directory %s: %w", projectConfig, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("BuildConfigPath() [common.go]: project config path is not a directory: %s", projectConfig)
		}
		projectConfigPath = projectConfig
	}

	// Start with default hierarchy
	configPathStr := "@DEFAULTS:" + filepath.Join(homeDir, ".config", "csw") + ":" + projectConfigPath

	// Validate and append custom config paths if provided
	if customConfigPath != "" {
		if err := ValidateConfigPaths(customConfigPath); err != nil {
			return "", err
		}
		configPathStr = configPathStr + ":" + customConfigPath
	}

	return configPathStr, nil
}

// ValidateConfigPaths validates that all paths in a colon-separated string exist and are directories.
func ValidateConfigPaths(configPath string) error {
	pathComponents := filepath.SplitList(configPath)
	for _, pathComponent := range pathComponents {
		if pathComponent == "" {
			continue
		}
		info, err := os.Stat(pathComponent)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("ValidateConfigPaths() [common.go]: config path does not exist: %s", pathComponent)
			}
			return fmt.Errorf("ValidateConfigPaths() [common.go]: failed to access config path %s: %w", pathComponent, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("ValidateConfigPaths() [common.go]: config path is not a directory: %s", pathComponent)
		}
	}
	return nil
}

// ResolveWorkDir resolves the working directory from an optional path argument.
// If dirPath is empty, returns current working directory.
// If dirPath is provided, resolves to absolute path and validates it exists and is a directory.
func ResolveWorkDir(dirPath string) (string, error) {
	if dirPath == "" {
		// Use current directory
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("ResolveWorkDir() [common.go]: failed to get current working directory: %w", err)
		}
		return wd, nil
	}

	// Directory provided as argument
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", fmt.Errorf("ResolveWorkDir() [common.go]: failed to resolve directory path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("ResolveWorkDir() [common.go]: failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("ResolveWorkDir() [common.go]: path is not a directory: %s", dirPath)
	}
	return absPath, nil
}

// ResolveModelName determines the model name to use.
// If modelName is empty, uses default provider from global config or first available provider.
func ResolveModelName(modelName string, configStore conf.ConfigStore, providerRegistry *models.ProviderRegistry) (string, error) {
	if modelName != "" {
		return modelName, nil
	}

	globalConfig, err := configStore.GetGlobalConfig()
	if err != nil {
		return "", fmt.Errorf("ResolveModelName() [common.go]: failed to get global config: %w", err)
	}

	if globalConfig.DefaultProvider != "" {
		return globalConfig.DefaultProvider + "/default", nil
	}

	providers := providerRegistry.List()
	if len(providers) > 0 {
		return providers[0] + "/default", nil
	}

	return "", fmt.Errorf("ResolveModelName() [common.go]: no default provider configured and no providers available")
}

// CreateProviderMap creates a map of provider names to ModelProvider instances from a registry.
func CreateProviderMap(providerRegistry *models.ProviderRegistry) (map[string]models.ModelProvider, error) {
	modelProviders := make(map[string]models.ModelProvider)
	for _, name := range providerRegistry.List() {
		provider, err := providerRegistry.Get(name)
		if err != nil {
			return nil, fmt.Errorf("CreateProviderMap() [common.go]: failed to get provider %s: %w", name, err)
		}
		modelProviders[name] = provider
	}
	return modelProviders, nil
}

// CreateModelTagRegistry creates and populates a model tag registry from config store.
func CreateModelTagRegistry(configStore conf.ConfigStore, providerRegistry *models.ProviderRegistry) (*models.ModelTagRegistry, error) {
	modelTagRegistry := models.NewModelTagRegistry()

	// Load global config model tags
	globalConfig, err := configStore.GetGlobalConfig()
	if err == nil && globalConfig != nil && len(globalConfig.ModelTags) > 0 {
		if err := modelTagRegistry.SetGlobalMappings(globalConfig.ModelTags); err != nil {
			return nil, fmt.Errorf("CreateModelTagRegistry() [common.go]: failed to set global model tags: %w", err)
		}
	}

	// Load provider-specific model tags
	for _, providerName := range providerRegistry.List() {
		provider, err := providerRegistry.Get(providerName)
		if err != nil {
			continue
		}
		if chatProvider, ok := provider.(interface{ GetConfig() interface{} }); ok {
			config := chatProvider.GetConfig()
			if providerConfig, ok := config.(*conf.ModelProviderConfig); ok && len(providerConfig.ModelTags) > 0 {
				if err := modelTagRegistry.SetProviderMappings(providerName, providerConfig.ModelTags); err != nil {
					return nil, fmt.Errorf("CreateModelTagRegistry() [common.go]: failed to set provider model tags: %w", err)
				}
			}
		}
	}

	return modelTagRegistry, nil
}
