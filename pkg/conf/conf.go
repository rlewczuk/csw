package conf

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

type AccessFlag string

const (
	AccessAuto  AccessFlag = "auto"
	AccessAllow AccessFlag = "allow"
	AccessDeny  AccessFlag = "deny"
	AccessAsk   AccessFlag = "ask"
)

// AuthMode represents the authentication mode for a model provider.
type AuthMode string

const (
	// AuthModeNone indicates no authentication is required.
	AuthModeNone AuthMode = "none"
	// AuthModeAPIKey indicates API key authentication.
	AuthModeAPIKey AuthMode = "api_key"
	// AuthModeOAuth2 indicates OAuth2 authentication with token renewal.
	AuthModeOAuth2 AuthMode = "oauth2"
)

type FileAccess struct {
	Read   AccessFlag `json:"read" yaml:"read"`
	Write  AccessFlag `json:"write" yaml:"write"`
	Delete AccessFlag `json:"delete" yaml:"delete"`
	List   AccessFlag `json:"list" yaml:"list"`
	Find   AccessFlag `json:"find" yaml:"find"`
	Move   AccessFlag `json:"move" yaml:"move"`
}

type AgentRoleConfig struct {
	// Name of the role (short name, used to select role and identify it in logs etc.)
	Name string `json:"name" yaml:"name"`

	// Description of the role (longer text, used in UI to describe role to user)
	Description string `json:"description" yaml:"description"`

	// Privileges for VFS and runtime
	VFSPrivileges map[string]FileAccess `json:"vfs-privileges" yaml:"vfs-privileges"`

	// Tools available
	ToolsAccess map[string]AccessFlag `json:"tools-access" yaml:"tools-access"`

	// Run privileges maps command regex patterns to access flags
	RunPrivileges map[string]AccessFlag `json:"run-privileges" yaml:"run-privileges"`

	// Prompt fragments for this role (as a map of filename without extension ->content), this is transient field, not serialized to JSON
	PromptFragments map[string]string `json:"-" yaml:"-"`

	// Tool fragments for this role (as a map of "<tool-name>/<file-name>" -> content), this is transient field, not serialized to JSON
	// Example keys: "vfsRead/vfsRead.schema.json", "vfsRead/vfsRead.md", "vfsRead/vfsRead-kimi.md"
	ToolFragments map[string]string `json:"-" yaml:"-"`

	// HiddenPatterns contains glob patterns for files and directories that should be hidden from VFS operations
	// Supports .gitignore-compatible syntax
	HiddenPatterns []string `json:"hidden-patterns,omitempty" yaml:"hidden-patterns,omitempty"`
}

// ModelTagMapping represents a single model-to-tag mapping rule.
// Model names are matched against the Model regexp pattern, and if they match,
// the Tag is assigned to the model.
type ModelTagMapping struct {
	// Model is a regexp pattern to match model names
	Model string `json:"model" yaml:"model"`
	// Tag is the tag name to assign to matching models
	Tag string `json:"tag" yaml:"tag"`
	// Compiled is the Compiled regexp pattern
	Compiled *regexp.Regexp
}

// ToolSelectionConfig defines tool availability rules across model tags.
type ToolSelectionConfig struct {
	// Default defines default availability of tools for all models.
	// Key is tool name, value true means enabled and false means disabled.
	Default map[string]bool `json:"default,omitempty" yaml:"default,omitempty"`
	// Tags defines per-tag tool overrides.
	// Key is tag name, value is a map of tool name to enabled status (true/false).
	Tags map[string]map[string]bool `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// ContainerConfig defines default container execution settings.
type ContainerConfig struct {
	// Mounts defines additional volume mappings in host_path:container_path format.
	Mounts []string `json:"mounts,omitempty" yaml:"mounts,omitempty"`
	// Env defines additional environment variables in KEY=VALUE format.
	Env []string `json:"env,omitempty" yaml:"env,omitempty"`
	// Image is the default container image used when container mode is enabled.
	Image string `json:"image,omitempty" yaml:"image,omitempty"`
	// Enabled enables container mode for all commands by default.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// ModelProviderConfig represents common configuration for model providers.
type ModelProviderConfig struct {
	// Type specifies the provider type (e.g., "ollama", "openai", "anthropic")
	Type string `json:"type" yaml:"type"`
	// Name is a short computer-friendly name for the provider instance (eg. `ollama-local`)
	Name string `json:"name" yaml:"name"`
	// Description is a user-friendly description for the provider instance
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// URL is the base URL for the provider's API
	URL string `json:"url" yaml:"url"`
	// APIKey is the API key for authentication (if required)
	APIKey string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	// ConnectTimeout is the timeout for establishing connections
	ConnectTimeout time.Duration `json:"connect_timeout,omitempty" yaml:"connect_timeout,omitempty"`
	// RequestTimeout is the timeout for complete requests
	RequestTimeout time.Duration `json:"request_timeout,omitempty" yaml:"request_timeout,omitempty"`
	// DefaultTemperature is the default temperature for chat completions
	DefaultTemperature float32 `json:"default_temperature,omitempty" yaml:"default_temperature,omitempty"`
	// DefaultTopP is the default top_p for chat completions
	DefaultTopP float32 `json:"default_top_p,omitempty" yaml:"default_top_p,omitempty"`
	// DefaultTopK is the default top_k for chat completions
	DefaultTopK int `json:"default_top_k,omitempty" yaml:"default_top_k,omitempty"`
	// ContextLengthLimit is the maximum number of tokens to use for context
	ContextLengthLimit int `json:"context_length_limit,omitempty" yaml:"context_length_limit,omitempty"`
	// MaxTokens is the maximum number of tokens to generate in the response
	MaxTokens int `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	// Tags is a list of tags for the provider (deprecated, use ModelTags instead)
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	// ModelTags contains model-to-tag mappings specific to this provider.
	// Each mapping has a regexp pattern for model names and a tag to assign.
	ModelTags []ModelTagMapping `json:"model_tags,omitempty" yaml:"model_tags,omitempty"`
	// Streaming controls whether to use streaming API for chat completions
	// Defaults to true for backward compatibility
	Streaming *bool `json:"streaming,omitempty" yaml:"streaming,omitempty"`
	// Verbose controls whether to print raw response and headers to stdout
	Verbose bool `json:"verbose,omitempty" yaml:"verbose,omitempty"`
	// Headers contains optional headers to send with provider requests
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// QueryParams contains optional query parameters to send with provider requests.
	QueryParams map[string]string `json:"query_params,omitempty" yaml:"query_params,omitempty"`
	// MaxRetries is the maximum number of retries for rate limit (429) errors
	// Defaults to 3 if not specified
	MaxRetries int `json:"max_retries,omitempty" yaml:"max_retries,omitempty"`
	// RateLimitBackoffScale is the base duration to scale rate limit backoff delays.
	// Defaults to 1s when unset or invalid.
	RateLimitBackoffScale time.Duration `json:"rate_limit_backoff_scale,omitempty" yaml:"rate_limit_backoff_scale,omitempty"`
	// AuthMode specifies the authentication mode for the provider.
	// Possible values: "none", "api_key" (default), "oauth2".
	AuthMode AuthMode `json:"auth_mode,omitempty" yaml:"auth_mode,omitempty"`
	// AuthURL is the OAuth2 authorization endpoint URL for browser-based authentication.
	AuthURL string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	// TokenURL is the OAuth2 token endpoint URL for token renewal.
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	// ClientID is the OAuth2 client identifier.
	ClientID string `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	// ClientSecret is the OAuth2 client secret (optional for some providers).
	ClientSecret string `json:"client_secret,omitempty" yaml:"client_secret,omitempty"`
	// RefreshToken is the OAuth2 refresh token used to obtain new access tokens.
	RefreshToken string `json:"refresh_token,omitempty" yaml:"refresh_token,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for ModelProviderConfig.
// It handles duration fields that are represented as strings in JSON (e.g., "30s", "5m").
func (c *ModelProviderConfig) UnmarshalJSON(data []byte) error {
	// Define a temporary struct with string fields for durations
	type Alias ModelProviderConfig
	aux := &struct {
		ConnectTimeout        string `json:"connect_timeout,omitempty"`
		RequestTimeout        string `json:"request_timeout,omitempty"`
		RateLimitBackoffScale string `json:"rate_limit_backoff_scale,omitempty"`
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

	if aux.RateLimitBackoffScale != "" {
		d, err := time.ParseDuration(aux.RateLimitBackoffScale)
		if err != nil {
			return fmt.Errorf("ModelProviderConfig.UnmarshalJSON(): invalid rate_limit_backoff_scale: %w", err)
		}
		c.RateLimitBackoffScale = d
	}

	return nil
}

// GlobalConfig represents the global configuration file structure.
type GlobalConfig struct {
	// ModelTags contains global model-to-tag mappings
	ModelTags []ModelTagMapping `json:"model_tags,omitempty" yaml:"model_tags,omitempty"`
	// ToolSelection defines model tag based tool selection rules.
	ToolSelection ToolSelectionConfig `json:"tool_selection,omitempty" yaml:"tool_selection,omitempty"`
	// ContextCompactionThreshold defines the ratio of current context length to max context length
	// at which message compaction is triggered. Defaults to 0.95 when unset or invalid.
	ContextCompactionThreshold float64 `json:"context_compaction_threshold,omitempty" yaml:"context_compaction_threshold,omitempty"`
	// DefaultProvider is the name of the default model provider to use
	DefaultProvider string `json:"default_provider,omitempty" yaml:"default_provider,omitempty"`
	// DefaultRole is the name of the default agent role to use
	DefaultRole string `json:"default_role,omitempty" yaml:"default_role,omitempty"`
	// LLMRetryMaxAttempts is the maximum number of attempts for temporary LLM API failures.
	// Defaults to 10 when unset or invalid.
	LLMRetryMaxAttempts int `json:"llm_retry_max_attempts,omitempty" yaml:"llm_retry_max_attempts,omitempty"`
	// LLMRetryMaxBackoffSeconds caps exponential backoff delay in seconds.
	// Defaults to 60 when unset or invalid.
	LLMRetryMaxBackoffSeconds int `json:"llm_retry_max_backoff_seconds,omitempty" yaml:"llm_retry_max_backoff_seconds,omitempty"`
	// Container defines default container execution settings.
	Container ContainerConfig `json:"container,omitempty" yaml:"container,omitempty"`
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

	// GetAgentConfigFile returns file content from agent configuration namespace.
	// The expected virtual location is conf/agent/<subdir>/<filename>.
	GetAgentConfigFile(subdir, filename string) ([]byte, error)
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
