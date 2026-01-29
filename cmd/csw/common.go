package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
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
	configPathStr := "@DEFAULTS:./.csw/config:" + filepath.Join(homeDir, ".config", "csw")

	store, err := impl.NewCompositeConfigStore(cwd, configPathStr)
	if err != nil {
		return nil, fmt.Errorf("GetCompositeConfigStore() [common.go]: failed to create composite config store: %w", err)
	}

	return store, nil
}
