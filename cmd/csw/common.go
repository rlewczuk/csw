package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/system"
)

// ConfigScope represents the scope of configuration (global or local).
type ConfigScope string

const (
	ConfigScopeLocal  ConfigScope = "local"
	ConfigScopeGlobal ConfigScope = "global"
)

// GetConfigStore returns a local config store for the specified scope or custom path.
func GetConfigStore(scope ConfigScope) (conf.ConfigStore, error) {
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
		configDir = string(scope)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("GetConfigStore() [common.go]: failed to create config directory: %w", err)
	}

	store, err := impl.NewLocalConfigStore(configDir)
	if err != nil {
		return nil, fmt.Errorf("GetConfigStore() [common.go]: failed to create config store: %w", err)
	}

	return store, nil
}

// GetCompositeConfigStore returns a composite config store that merges configurations.
func GetCompositeConfigStore() (conf.ConfigStore, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("GetCompositeConfigStore() [common.go]: failed to get current directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("GetCompositeConfigStore() [common.go]: failed to get home directory: %w", err)
	}

	configPathStr := "@DEFAULTS:" + filepath.Join(homeDir, ".config", "csw") + ":./.csw/config"

	store, err := impl.NewCompositeConfigStore(cwd, configPathStr)
	if err != nil {
		return nil, fmt.Errorf("GetCompositeConfigStore() [common.go]: failed to create composite config store: %w", err)
	}

	return store, nil
}

// BuildConfigPath builds a config path hierarchy string.
func BuildConfigPath(projectConfig, customConfigPath string) (string, error) {
	return system.BuildConfigPath(projectConfig, customConfigPath)
}

// ValidateConfigPaths validates that all paths in a colon-separated string exist and are directories.
func ValidateConfigPaths(configPath string) error {
	return system.ValidateConfigPaths(configPath)
}

// ResolveWorkDir resolves working directory from an optional path argument.
func ResolveWorkDir(dirPath string) (string, error) {
	return system.ResolveWorkDir(dirPath)
}

// ResolveModelName determines model name to use.
func ResolveModelName(modelName string, configStore conf.ConfigStore, providerRegistry *models.ProviderRegistry) (string, error) {
	return system.ResolveModelName(modelName, configStore, providerRegistry)
}

// CreateProviderMap creates a map of provider names to ModelProvider instances from a registry.
func CreateProviderMap(providerRegistry *models.ProviderRegistry) (map[string]models.ModelProvider, error) {
	return system.CreateProviderMap(providerRegistry)
}

// CreateModelTagRegistry creates and populates model tag registry from config store.
func CreateModelTagRegistry(configStore conf.ConfigStore, providerRegistry *models.ProviderRegistry) (*models.ModelTagRegistry, error) {
	return system.CreateModelTagRegistry(configStore, providerRegistry)
}
