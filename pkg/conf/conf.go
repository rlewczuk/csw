package conf

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

type AccessFlag string

const (
	AccessAllow AccessFlag = "allow"
	AccessDeny  AccessFlag = "deny"
	AccessAsk   AccessFlag = "ask"
)

type FileAccess struct {
	Read   AccessFlag `json:"read"`
	Write  AccessFlag `json:"write"`
	Delete AccessFlag `json:"delete"`
	List   AccessFlag `json:"list"`
	Find   AccessFlag `json:"find"`
	Move   AccessFlag `json:"move"`
}

type AgentRoleConfig struct {
	// Name of the role (short name, used to select role and identify it in logs etc.)
	Name string `json:"name"`

	// Description of the role (longer text, used in UI to describe role to user)
	Description string `json:"description"`

	// Privileges for VFS and runtime
	VFSPrivileges map[string]FileAccess `json:"vfs-privileges"`

	// Tools available
	ToolsAccess map[string]AccessFlag `json:"tools-access"`

	// Run privileges maps command regex patterns to access flags
	RunPrivileges map[string]AccessFlag `json:"run-privileges"`

	// Prompt fragments for this role (as a map of filename without extension ->content), this is transient field, not serialized to JSON
	PromptFragments map[string]string `json:"-"`

	// Tool fragments for this role (as a map of "<tool-name>/<file-name>" -> content), this is transient field, not serialized to JSON
	// Example keys: "vfs.read/vfs.read.schema.json", "vfs.read/vfs.read.md", "vfs.read/vfs.read-kimi.md"
	ToolFragments map[string]string `json:"-"`

	// HiddenPatterns contains glob patterns for files and directories that should be hidden from VFS operations
	// Supports .gitignore-compatible syntax
	HiddenPatterns []string `json:"hidden-patterns,omitempty"`
}

// ModelTagMapping represents a single model-to-tag mapping rule.
// Model names are matched against the Model regexp pattern, and if they match,
// the Tag is assigned to the model.
type ModelTagMapping struct {
	// Model is a regexp pattern to match model names
	Model string `json:"model"`
	// Tag is the tag name to assign to matching models
	Tag string `json:"tag"`
	// Compiled is the Compiled regexp pattern
	Compiled *regexp.Regexp
}

// ModelProviderConfig represents common configuration for model providers.
type ModelProviderConfig struct {
	// Type specifies the provider type (e.g., "ollama", "openai", "anthropic")
	Type string `json:"type"`
	// Name is a short computer-friendly name for the provider instance (eg. `ollama-local`)
	Name string `json:"name"`
	// Description is a user-friendly description for the provider instance
	Description string `json:"description,omitempty"`
	// URL is the base URL for the provider's API
	URL string `json:"url"`
	// APIKey is the API key for authentication (if required)
	APIKey string `json:"api_key,omitempty"`
	// ConnectTimeout is the timeout for establishing connections
	ConnectTimeout time.Duration `json:"connect_timeout,omitempty"`
	// RequestTimeout is the timeout for complete requests
	RequestTimeout time.Duration `json:"request_timeout,omitempty"`
	// DefaultTemperature is the default temperature for chat completions
	DefaultTemperature float32 `json:"default_temperature,omitempty"`
	// DefaultTopP is the default top_p for chat completions
	DefaultTopP float32 `json:"default_top_p,omitempty"`
	// DefaultTopK is the default top_k for chat completions
	DefaultTopK int `json:"default_top_k,omitempty"`
	// ContextLengthLimit is the maximum context length in tokens
	ContextLengthLimit int `json:"context_length_limit,omitempty"`
	// Tags is a list of tags for the provider (deprecated, use ModelTags instead)
	Tags []string `json:"tags,omitempty"`
	// ModelTags contains model-to-tag mappings specific to this provider.
	// Each mapping has a regexp pattern for model names and a tag to assign.
	ModelTags []ModelTagMapping `json:"model_tags,omitempty"`
	// Streaming controls whether to use streaming API for chat completions
	// Defaults to true for backward compatibility
	Streaming *bool `json:"streaming,omitempty"`
	// Verbose controls whether to print raw response and headers to stdout
	Verbose bool `json:"verbose,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for ModelProviderConfig.
// It handles duration fields that are represented as strings in JSON (e.g., "30s", "5m").
func (c *ModelProviderConfig) UnmarshalJSON(data []byte) error {
	// Define a temporary struct with string fields for durations
	type Alias ModelProviderConfig
	aux := &struct {
		ConnectTimeout string `json:"connect_timeout,omitempty"`
		RequestTimeout string `json:"request_timeout,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("ModelProviderConfig.UnmarshalJSON(): %w", err)
	}

	// Parse duration strings
	if aux.ConnectTimeout != "" {
		d, err := time.ParseDuration(aux.ConnectTimeout)
		if err != nil {
			return fmt.Errorf("ModelProviderConfig.UnmarshalJSON(): invalid connect_timeout: %w", err)
		}
		c.ConnectTimeout = d
	}

	if aux.RequestTimeout != "" {
		d, err := time.ParseDuration(aux.RequestTimeout)
		if err != nil {
			return fmt.Errorf("ModelProviderConfig.UnmarshalJSON(): invalid request_timeout: %w", err)
		}
		c.RequestTimeout = d
	}

	return nil
}

// GlobalConfig represents the global configuration file structure.
type GlobalConfig struct {
	// ModelTags contains global model-to-tag mappings
	ModelTags []ModelTagMapping `json:"model_tags,omitempty"`
	// DefaultProvider is the name of the default model provider to use
	DefaultProvider string `json:"default_provider,omitempty"`
	// DefaultRole is the name of the default agent role to use
	DefaultRole string `json:"default_role,omitempty"`
}

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

// WritableConfigStore extends ConfigStore with write operations.
type WritableConfigStore interface {
	ConfigStore

	// SaveModelProviderConfig saves or updates a model provider configuration.
	SaveModelProviderConfig(config *ModelProviderConfig) error

	// DeleteModelProviderConfig deletes a model provider configuration.
	DeleteModelProviderConfig(name string) error

	// SaveGlobalConfig saves global configuration.
	SaveGlobalConfig(config *GlobalConfig) error
}
