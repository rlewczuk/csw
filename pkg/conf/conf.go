package conf

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type AccessFlag string

const (
	AccessAuto  AccessFlag = "auto"
	AccessAllow AccessFlag = "allow"
	AccessDeny  AccessFlag = "deny"
	AccessAsk   AccessFlag = "ask"
)

// MCPTransportType defines transport used to connect to MCP server.
type MCPTransportType string

const (
	// MCPTransportTypeStdio runs MCP over stdio with local subprocess.
	MCPTransportTypeStdio MCPTransportType = "stdio"
	// MCPTransportTypeHTTP runs MCP over streamable HTTP.
	MCPTransportTypeHTTP MCPTransportType = "http"
	// MCPTransportTypeHTTPS runs MCP over streamable HTTPS.
	MCPTransportTypeHTTPS MCPTransportType = "https"
)

// HookType defines supported hook execution backend.
type HookType string

const (
	// HookTypeShell runs a shell command.
	HookTypeShell HookType = "shell"
	// HookTypeLLM runs a one-shot LLM query.
	HookTypeLLM HookType = "llm"
	// HookTypeSubAgent runs a delegated subagent task.
	HookTypeSubAgent HookType = "subagent"
)

// HookRunOn defines where shell hook commands are executed.
type HookRunOn string

const (
	// HookRunOnHost executes command on host system.
	HookRunOnHost HookRunOn = "host"
	// HookRunOnSandbox executes command in sandbox/container when available.
	HookRunOnSandbox HookRunOn = "sandbox"
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

// ModelProviderCost defines token pricing for a specific context tier.
type ModelProviderCost struct {
	// Input is the cost per 1M input tokens.
	Input float64 `json:"input,omitempty" yaml:"input,omitempty"`
	// Output is the cost per 1M output tokens.
	Output float64 `json:"output,omitempty" yaml:"output,omitempty"`
	// CacheRead is the cost per 1M cache read tokens.
	CacheRead float64 `json:"cache-read,omitempty" yaml:"cache-read,omitempty"`
	// CacheWrite is the cost per 1M cache write tokens.
	CacheWrite float64 `json:"cache-write,omitempty" yaml:"cache-write,omitempty"`
	// Context is the lower context token threshold for this pricing tier.
	Context int `json:"context,omitempty" yaml:"context,omitempty"`
}

// ModelProviderModalities defines model input/output modalities.
type ModelProviderModalities struct {
	// Input lists supported input modalities.
	Input []string `json:"input,omitempty" yaml:"input,omitempty"`
	// Output lists supported output modalities.
	Output []string `json:"output,omitempty" yaml:"output,omitempty"`
}

// ModelAliasValue defines one alias entry that can be a single model string
// or a list of nested model strings/aliases.
type ModelAliasValue struct {
	Values []string
}

// UnmarshalJSON decodes alias value from string or string list.
func (v *ModelAliasValue) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		trimmed := strings.TrimSpace(single)
		if trimmed == "" {
			return fmt.Errorf("ModelAliasValue.UnmarshalJSON() [conf.go]: alias value cannot be empty")
		}
		v.Values = []string{trimmed}
		return nil
	}

	var multi []string
	if err := json.Unmarshal(data, &multi); err == nil {
		if len(multi) == 0 {
			return fmt.Errorf("ModelAliasValue.UnmarshalJSON() [conf.go]: alias list cannot be empty")
		}
		values := make([]string, 0, len(multi))
		for _, item := range multi {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				return fmt.Errorf("ModelAliasValue.UnmarshalJSON() [conf.go]: alias list contains empty item")
			}
			values = append(values, trimmed)
		}
		v.Values = values
		return nil
	}

	return fmt.Errorf("ModelAliasValue.UnmarshalJSON() [conf.go]: alias value must be string or string array")
}

// UnmarshalYAML decodes alias value from string or string list.
func (v *ModelAliasValue) UnmarshalYAML(node *yaml.Node) error {
	if node == nil {
		return fmt.Errorf("ModelAliasValue.UnmarshalYAML() [conf.go]: node is nil")
	}

	switch node.Kind {
	case yaml.ScalarNode:
		trimmed := strings.TrimSpace(node.Value)
		if trimmed == "" {
			return fmt.Errorf("ModelAliasValue.UnmarshalYAML() [conf.go]: alias value cannot be empty")
		}
		v.Values = []string{trimmed}
		return nil
	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			return fmt.Errorf("ModelAliasValue.UnmarshalYAML() [conf.go]: alias list cannot be empty")
		}
		values := make([]string, 0, len(node.Content))
		for _, child := range node.Content {
			if child == nil || child.Kind != yaml.ScalarNode {
				return fmt.Errorf("ModelAliasValue.UnmarshalYAML() [conf.go]: alias list items must be strings")
			}
			trimmed := strings.TrimSpace(child.Value)
			if trimmed == "" {
				return fmt.Errorf("ModelAliasValue.UnmarshalYAML() [conf.go]: alias list contains empty item")
			}
			values = append(values, trimmed)
		}
		v.Values = values
		return nil
	default:
		return fmt.Errorf("ModelAliasValue.UnmarshalYAML() [conf.go]: alias value must be string or string array")
	}
}

// ModelVendorFamilyTemplateOverride groups vendor and family template overrides for a specific provider template set.
type ModelVendorFamilyTemplateOverride struct {
	// Vendor contains vendor-specific default parameters.
	Vendor ModelProviderConfig `json:"vendor,omitempty" yaml:"vendor,omitempty"`
	// Families contains family-specific overrides for the vendor.
	Families map[string]ModelProviderConfig `json:"families,omitempty" yaml:"families,omitempty"`
}

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

	// Aliases defines additional names that can be used to select this role.
	Aliases []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`

	// Description of the role (longer text, used in UI to describe role to user)
	Description string `json:"description" yaml:"description"`

	// Model defines default model selection for this role.
	// It accepts provider/model, comma-separated provider/model list,
	// or a configured model alias.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`

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

	// MCPServers contains MCP server names that should be enabled for this role.
	// Names must match files under conf/mcp/<server-name>.
	MCPServers []string `json:"mcp-servers,omitempty" yaml:"mcp-servers,omitempty"`
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

// MCPServerConfig defines configuration for running a local MCP server.
type MCPServerConfig struct {
	// Name is a short computer-friendly identifier of MCP server.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Description is optional human-readable server description.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Transport defines MCP transport type: stdio, http, or https.
	// Empty value defaults to stdio.
	Transport MCPTransportType `json:"transport,omitempty" yaml:"transport,omitempty"`
	// URL is MCP endpoint URL for HTTP(S) transports.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
	// APIKey is optional bearer token used for HTTP(S) transport requests.
	APIKey string `json:"api-key,omitempty" yaml:"api-key,omitempty"`
	// Cmd is the command to run MCP server (may include arguments).
	Cmd string `json:"cmd,omitempty" yaml:"cmd,omitempty"`
	// Enabled controls whether MCP server should be started.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Args are additional command arguments appended to Cmd arguments.
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`
	// Env contains environment variables set for MCP server process.
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	// Tools contains tool filters selecting enabled tool names.
	// Each item is either an exact tool name or a glob pattern.
	// Empty or nil slice means all tools enabled.
	Tools []string `json:"tools" yaml:"tools"`
}

// RunDefaultsConfig defines default values for the cli command flags.
type RunDefaultsConfig struct {
	// DefaultProvider is the default model provider to use.
	DefaultProvider string `json:"default-provider,omitempty" yaml:"default-provider,omitempty"`
	// DefaultRole is the default agent role to use.
	DefaultRole string `json:"default-role,omitempty" yaml:"default-role,omitempty"`
	// Container defines default container execution settings.
	Container *ContainerConfig `json:"container,omitempty" yaml:"container,omitempty"`
	// Model is the default --model value.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`
	// Worktree is the default --worktree value.
	Worktree string `json:"worktree,omitempty" yaml:"worktree,omitempty"`
	// Merge is the default --merge value.
	Merge bool `json:"merge,omitempty" yaml:"merge,omitempty"`
	// LogLLMRequests is the default --log-llm-requests value.
	LogLLMRequests bool `json:"log-llm-requests,omitempty" yaml:"log-llm-requests,omitempty"`
	// LogLLMRequestsRaw is the default --log-llm-requests-raw value.
	LogLLMRequestsRaw bool `json:"log-llm-requests-raw,omitempty" yaml:"log-llm-requests-raw,omitempty"`
	// Thinking is the default --thinking value.
	Thinking string `json:"thinking,omitempty" yaml:"thinking,omitempty"`
	// LSPServer is the default --lsp-server value.
	LSPServer string `json:"lsp-server,omitempty" yaml:"lsp-server,omitempty"`
	// GitUserName is the default --git-user value for git operations.
	GitUserName string `json:"git-user,omitempty" yaml:"git-user,omitempty"`
	// GitUserEmail is the default --git-email value for git operations.
	GitUserEmail string `json:"git-email,omitempty" yaml:"git-email,omitempty"`
	// MaxThreads is the default --max-threads value.
	MaxThreads int `json:"max-threads,omitempty" yaml:"max-threads,omitempty"`
	// TaskDir is the default task directory used by task commands.
	TaskDir string `json:"task-dir,omitempty" yaml:"task-dir,omitempty"`
	// ShadowDir is the default --shadow-dir value.
	ShadowDir string `json:"shadow-dir,omitempty" yaml:"shadow-dir,omitempty"`
	// AllowAllPermissions is the default --allow-all-permissions value.
	AllowAllPermissions bool `json:"allow-all-permissions,omitempty" yaml:"allow-all-permissions,omitempty"`
	// VFSAllow is the default --vfs-allow value.
	VFSAllow []string `json:"vfs-allow,omitempty" yaml:"vfs-allow,omitempty"`
}

// ModelProviderConfig represents common configuration for model providers.
type ModelProviderConfig struct {
	// Type specifies the provider type (e.g., "ollama", "openai", "anthropic")
	Type string `json:"type" yaml:"type"`
	// Name is a short computer-friendly name for the provider instance (eg. `ollama-local`).
	// It is transient and derived from the configuration file basename.
	Name string `json:"-" yaml:"-"`
	// Family is the model family identifier.
	Family string `json:"family,omitempty" yaml:"family,omitempty"`
	// ReleaseDate is the model/template release date in string form.
	ReleaseDate string `json:"release-date,omitempty" yaml:"release-date,omitempty"`
	// Description is a user-friendly description for the provider instance
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// URL is the base URL for the provider's API
	URL string `json:"url" yaml:"url"`
	// APIKey is the API key for authentication (if required)
	APIKey string `json:"api-key,omitempty" yaml:"api-key,omitempty"`
	// ConnectTimeout is the timeout for establishing connections
	ConnectTimeout time.Duration `json:"connect-timeout,omitempty" yaml:"connect-timeout,omitempty"`
	// RequestTimeout is the timeout for complete requests
	RequestTimeout time.Duration `json:"request-timeout,omitempty" yaml:"request-timeout,omitempty"`
	// DefaultTemperature is the default temperature for chat completions
	DefaultTemperature float32 `json:"default-temperature,omitempty" yaml:"default-temperature,omitempty"`
	// DefaultTopP is the default top_p for chat completions
	DefaultTopP float32 `json:"default-top-p,omitempty" yaml:"default-top-p,omitempty"`
	// DefaultTopK is the default top_k for chat completions
	DefaultTopK int `json:"default-top-k,omitempty" yaml:"default-top-k,omitempty"`
	// ContextLengthLimit is the maximum number of tokens to use for context
	ContextLengthLimit int `json:"context-length-limit,omitempty" yaml:"context-length-limit,omitempty"`
	// MaxTokens is the maximum number of tokens to generate in the response
	MaxTokens int `json:"max-tokens,omitempty" yaml:"max-tokens,omitempty"`
	// MaxInputTokens is the maximum number of input tokens accepted by the model.
	MaxInputTokens int `json:"max-input-tokens,omitempty" yaml:"max-input-tokens,omitempty"`
	// MaxOutputTokens is the maximum number of output tokens generated by the model.
	MaxOutputTokens int `json:"max-output-tokens,omitempty" yaml:"max-output-tokens,omitempty"`
	// ModelTags contains model-to-tag mappings specific to this provider.
	// Each mapping has a regexp pattern for model names and a tag to assign.
	ModelTags []ModelTagMapping `json:"model-tags,omitempty" yaml:"model-tags,omitempty"`
	// Reasoning maps effort levels (none, low, medium, high, xhigh) to provider-specific reasoning mode names.
	Reasoning map[string]string `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
	// ReasoningContent defines provider reasoning content field name.
	ReasoningContent string `json:"reasoning-content,omitempty" yaml:"reasoning-content,omitempty"`
	// Temperature indicates whether model/template supports temperature control.
	Temperature *bool `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	// ToolCall indicates whether model/template supports tool calling.
	ToolCall *bool `json:"tool-call,omitempty" yaml:"tool-call,omitempty"`
	// Interleaved indicates optional interleaving mode configuration.
	Interleaved string `json:"interleaved,omitempty" yaml:"interleaved,omitempty"`
	// Cost contains pricing tiers for token costs.
	Cost []ModelProviderCost `json:"cost,omitempty" yaml:"cost,omitempty"`
	// Modalities defines input/output modalities supported by the model.
	Modalities *ModelProviderModalities `json:"modalities,omitempty" yaml:"modalities,omitempty"`
	// Experimental indicates whether this model/template is experimental.
	Experimental *bool `json:"experimental,omitempty" yaml:"experimental,omitempty"`
	// Status indicates model/template lifecycle status (e.g. alpha/beta/deprecated).
	Status string `json:"status,omitempty" yaml:"status,omitempty"`
	// Options contains provider/model-specific arbitrary options.
	Options map[string]any `json:"options,omitempty" yaml:"options,omitempty"`
	// Streaming controls whether to use streaming API for chat completions
	// Defaults to true for backward compatibility
	Streaming *bool `json:"streaming,omitempty" yaml:"streaming,omitempty"`
	// Verbose controls whether to print raw response and headers to stdout
	Verbose bool `json:"verbose,omitempty" yaml:"verbose,omitempty"`
	// Headers contains optional headers to send with provider requests
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// QueryParams contains optional query parameters to send with provider requests.
	QueryParams map[string]string `json:"query-params,omitempty" yaml:"query-params,omitempty"`
	// MaxRetries is the maximum number of retries for rate limit (429) errors
	// Defaults to 3 if not specified
	MaxRetries int `json:"max-retries,omitempty" yaml:"max-retries,omitempty"`
	// RateLimitBackoffScale is the base duration to scale rate limit backoff delays.
	// Defaults to 1s when unset or invalid.
	RateLimitBackoffScale time.Duration `json:"rate-limit-backoff-scale,omitempty" yaml:"rate-limit-backoff-scale,omitempty"`
	// AuthMode specifies the authentication mode for the provider.
	// Possible values: "none", "api_key" (default), "oauth2".
	AuthMode AuthMode `json:"auth-mode,omitempty" yaml:"auth-mode,omitempty"`
	// AuthURL is the OAuth2 authorization endpoint URL for browser-based authentication.
	AuthURL string `json:"auth-url,omitempty" yaml:"auth-url,omitempty"`
	// TokenURL is the OAuth2 token endpoint URL for token renewal.
	TokenURL string `json:"token-url,omitempty" yaml:"token-url,omitempty"`
	// ClientID is the OAuth2 client identifier.
	ClientID string `json:"client-id,omitempty" yaml:"client-id,omitempty"`
	// ClientSecret is the OAuth2 client secret (optional for some providers).
	ClientSecret string `json:"client-secret,omitempty" yaml:"client-secret,omitempty"`
	// RefreshToken is the OAuth2 refresh token used to obtain new access tokens.
	RefreshToken string `json:"refresh-token,omitempty" yaml:"refresh-token,omitempty"`
	// DisableRefresh disables OAuth access token refresh for this provider.
	DisableRefresh bool `json:"disable-refresh,omitempty" yaml:"disable-refresh,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for ModelProviderConfig.
// It serializes duration fields as strings (e.g., "30s", "5m", "1h0m0s").
func (c ModelProviderConfig) MarshalJSON() ([]byte, error) {
	type Alias ModelProviderConfig
	aux := &struct {
		ConnectTimeout        string `json:"connect-timeout,omitempty"`
		RequestTimeout        string `json:"request-timeout,omitempty"`
		RateLimitBackoffScale string `json:"rate-limit-backoff-scale,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(&c),
	}

	if c.ConnectTimeout != 0 {
		aux.ConnectTimeout = formatDurationForConfig(c.ConnectTimeout)
	}
	if c.RequestTimeout != 0 {
		aux.RequestTimeout = formatDurationForConfig(c.RequestTimeout)
	}
	if c.RateLimitBackoffScale != 0 {
		aux.RateLimitBackoffScale = formatDurationForConfig(c.RateLimitBackoffScale)
	}

	data, err := json.Marshal(aux)
	if err != nil {
		return nil, fmt.Errorf("ModelProviderConfig.MarshalJSON() [conf.go]: %w", err)
	}

	return data, nil
}

// MarshalYAML implements custom YAML marshaling for ModelProviderConfig.
// It keeps duration fields serialized as strings, same as JSON marshaling.
func (c ModelProviderConfig) MarshalYAML() (any, error) {
	data, err := c.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("ModelProviderConfig.MarshalYAML() [conf.go]: %w", err)
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("ModelProviderConfig.MarshalYAML() [conf.go]: failed to unmarshal marshaled json: %w", err)
	}

	return out, nil
}

// formatDurationForConfig converts duration value to a string with explicit unit suffix.
func formatDurationForConfig(d time.Duration) string {
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", d/time.Second)
	}
	if d%time.Millisecond == 0 {
		return fmt.Sprintf("%dms", d/time.Millisecond)
	}
	if d%time.Microsecond == 0 {
		return fmt.Sprintf("%dus", d/time.Microsecond)
	}

	return fmt.Sprintf("%dns", d)
}

// UnmarshalJSON implements custom JSON unmarshaling for ModelProviderConfig.
// It handles duration fields that are represented as strings in JSON (e.g., "30s", "5m").
func (c *ModelProviderConfig) UnmarshalJSON(data []byte) error {
	// Define a temporary struct with string fields for durations
	type Alias ModelProviderConfig
	aux := &struct {
		ConnectTimeout        string `json:"connect-timeout,omitempty"`
		RequestTimeout        string `json:"request-timeout,omitempty"`
		RateLimitBackoffScale string `json:"rate-limit-backoff-scale,omitempty"`
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
	ModelTags []ModelTagMapping `json:"model-tags,omitempty" yaml:"model-tags,omitempty"`
	// ToolSelection defines model tag based tool selection rules.
	ToolSelection ToolSelectionConfig `json:"tool-selection,omitempty" yaml:"tool-selection,omitempty"`
	// ContextCompactionThreshold defines the ratio of current context length to max context length
	// at which message compaction is triggered. Defaults to 0.95 when unset or invalid.
	ContextCompactionThreshold float64 `json:"context-compaction-threshold,omitempty" yaml:"context-compaction-threshold,omitempty"`
	// LLMRetryMaxAttempts is the maximum number of attempts for temporary LLM API failures.
	// Defaults to 10 when unset or invalid.
	LLMRetryMaxAttempts int `json:"llm-retry-max-attempts,omitempty" yaml:"llm-retry-max-attempts,omitempty"`
	// LLMRetryMaxBackoffSeconds caps exponential backoff delay in seconds.
	// Defaults to 60 when unset or invalid.
	LLMRetryMaxBackoffSeconds int `json:"llm-retry-max-backoff-seconds,omitempty" yaml:"llm-retry-max-backoff-seconds,omitempty"`
	// Defaults defines default values for cli command flags.
	Defaults RunDefaultsConfig `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	// ShadowPaths defines glob patterns redirected to shadow directory when --shadow-dir is enabled.
	ShadowPaths []string `json:"shadow-paths,omitempty" yaml:"shadow-paths,omitempty"`
}

// ConfigStore is an interface for accessing configuration data.
// For single config source, it returns up to date data from it.
// For multiple config sources, implementation behind is responsible for collecting config data
// from all sources and present merged view of it.
type ConfigStore interface {
	// GetModelProviderConfigs returns a map of model provider configurations, keyed by provider name.
	GetModelProviderConfigs() (map[string]*ModelProviderConfig, error)

	// GetAgentRoleConfigs returns a map of agent role configurations, keyed by role name.
	GetAgentRoleConfigs() (map[string]*AgentRoleConfig, error)

	// GetGlobalConfig returns global configuration
	GetGlobalConfig() (*GlobalConfig, error)

	// GetMCPServerConfigs returns MCP server configurations keyed by server name.
	GetMCPServerConfigs() (map[string]*MCPServerConfig, error)

	// GetAgentConfigFile returns file content from agent configuration namespace.
	// The expected virtual location is conf/agent/<subdir>/<filename>.
	GetAgentConfigFile(subdir, filename string) ([]byte, error)

	// GetModelAliases returns model aliases keyed by alias name.
	GetModelAliases() (map[string]ModelAliasValue, error)
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
