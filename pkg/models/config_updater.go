package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rlewczuk/csw/pkg/conf"
)

// ConfigUpdater is a callback function type that can be used to persist
// configuration changes made by model providers (e.g., updated API keys
// or refresh tokens after OAuth2 token renewal).
type ConfigUpdater func(config *conf.ModelProviderConfig) error

// ConfigUpdaterImpl provides a mechanism for model providers to update
// their configuration. It wraps a WritableConfigStore and provides a
// callback function that can be called by providers to persist changes.
type ConfigUpdaterImpl struct {
	provider string
}

// NewConfigUpdater creates a new ConfigUpdaterImpl for the given provider
// and config store. The returned instance can be used to create a callback
// function for the provider to persist configuration changes.
func NewConfigUpdater(providerName string) *ConfigUpdaterImpl {
	return &ConfigUpdaterImpl{
		provider: providerName,
	}
}

// Update returns a ConfigUpdater callback function that can be passed to
// model providers. When called, it will persist the updated configuration
// to the underlying config store.
func (u *ConfigUpdaterImpl) Update() ConfigUpdater {
	return func(config *conf.ModelProviderConfig) error {
		if config == nil {
			return fmt.Errorf("ConfigUpdater.Update() [config_updater.go]: config cannot be nil")
		}
		if config.Name == "" {
			return fmt.Errorf("ConfigUpdater.Update() [config_updater.go]: config name cannot be empty")
		}
		return SaveProviderConfigToUserModelsDir(config)
	}
}

// ProviderName returns the name of the provider this updater is associated with.
func (u *ConfigUpdaterImpl) ProviderName() string {
	return u.provider
}

// SaveProviderConfigToUserModelsDir saves provider config as JSON in ~/.config/csw/models.
func SaveProviderConfigToUserModelsDir(config *conf.ModelProviderConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("SaveProviderConfigToUserModelsDir() [config_updater.go]: failed to get home directory: %w", err)
	}

	modelsDir := filepath.Join(homeDir, ".config", "csw", "models")
	return SaveProviderConfigToModelsDir(config, modelsDir)
}

// SaveProviderConfigToModelsDir saves provider config as JSON in the provided models directory.
func SaveProviderConfigToModelsDir(config *conf.ModelProviderConfig, modelsDir string) error {
	if config == nil {
		return fmt.Errorf("SaveProviderConfigToModelsDir() [config_updater.go]: provider config is nil")
	}
	if config.Name == "" {
		return fmt.Errorf("SaveProviderConfigToModelsDir() [config_updater.go]: provider config name is empty")
	}

	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return fmt.Errorf("SaveProviderConfigToModelsDir() [config_updater.go]: failed to create models directory: %w", err)
	}

	configPath := filepath.Join(modelsDir, config.Name+".json")
	backupPath := configPath + ".bak"
	if _, err := os.Stat(configPath); err == nil {
		existingData, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("SaveProviderConfigToModelsDir() [config_updater.go]: failed to read existing provider config: %w", err)
		}
		if err := os.WriteFile(backupPath, existingData, 0644); err != nil {
			return fmt.Errorf("SaveProviderConfigToModelsDir() [config_updater.go]: failed to create provider config backup: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("SaveProviderConfigToModelsDir() [config_updater.go]: failed to inspect provider config path: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("SaveProviderConfigToModelsDir() [config_updater.go]: failed to marshal provider config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("SaveProviderConfigToModelsDir() [config_updater.go]: failed to save provider config: %w", err)
	}

	return nil
}
