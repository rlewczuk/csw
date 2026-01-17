package conf

import "time"

// ConfigStore is an interface for accessing configuration data.
// For single config source, it returns up to date data from it.
// For multiple config sources, implementation behind is responsible for collecting config data
// from all sources and present merged view of it.
type ConfigStore interface {
	// GetModelProviderConfigs returns a map of model provider configurations, keyed by provider name.
	GetModelProviderConfigs() (map[string]*ModelProviderConfig, error)

	// LastModelProviderConfigsUpdate returns timestamp of last update of model provider configs
	// this is used by client code to determine if model provider configuration has changed and needs to be reloaded
	LastModelProviderConfigsUpdate() (time.Time, error)

	// GetAgentRoleConfigs returns a map of agent role configurations, keyed by role name.
	GetAgentRoleConfigs() (map[string]*AgentRoleConfig, error)

	// LastAgentRoleConfigsUpdate returns timestamp of last update of agent role configs
	// this is used by client code to determine if agent role configuration has changed and needs to be reloaded
	LastAgentRoleConfigsUpdate() (time.Time, error)

	// GetGlobalConfig returns global configuration
	GetGlobalConfig() (*GlobalConfig, error)

	// LastGlobalConfigUpdate returns timestamp of last update of global config
	LastGlobalConfigUpdate() (time.Time, error)
}
