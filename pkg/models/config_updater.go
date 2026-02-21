package models

import (
	"fmt"

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
	store    conf.WritableConfigStore
	provider string
}

// NewConfigUpdater creates a new ConfigUpdaterImpl for the given provider
// and config store. The returned instance can be used to create a callback
// function for the provider to persist configuration changes.
func NewConfigUpdater(store conf.WritableConfigStore, providerName string) *ConfigUpdaterImpl {
	return &ConfigUpdaterImpl{
		store:    store,
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
		return u.store.SaveModelProviderConfig(config)
	}
}

// ProviderName returns the name of the provider this updater is associated with.
func (u *ConfigUpdaterImpl) ProviderName() string {
	return u.provider
}
