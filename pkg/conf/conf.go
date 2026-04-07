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
	CacheRead float64 `json:"cache_read,omitempty" yaml:"cache_read,omitempty"`
	// CacheWrite is the cost per 1M cache write tokens.
	CacheWrite float64 `json:"cache_write,omitempty" yaml:"cache_write,omitempty"`
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
	APIKey string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
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

	enabledConfigured bool
}

// UnmarshalJSON unmarshals MCPServerConfig and tracks explicitly configured fields.
func (c *MCPServerConfig) UnmarshalJSON(data []byte) error {
	type alias MCPServerConfig
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("MCPServerConfig.UnmarshalJSON() [conf.go]: failed to unmarshal mcp server config: %w", err)
	}

	*c = MCPServerConfig(decoded)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("MCPServerConfig.UnmarshalJSON() [conf.go]: failed to unmarshal mcp server config raw map: %w", err)
	}

	_, c.enabledConfigured = raw["enabled"]

	return nil
}

// UnmarshalYAML unmarshals MCPServerConfig and tracks explicitly configured fields.
func (c *MCPServerConfig) UnmarshalYAML(node *yaml.Node) error {
	type alias MCPServerConfig
	var decoded alias

	if err := node.Decode(&decoded); err != nil {
		return fmt.Errorf("MCPServerConfig.UnmarshalYAML() [conf.go]: failed to decode mcp server config: %w", err)
	}

	*c = MCPServerConfig(decoded)

	if node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "enabled" {
			c.enabledConfigured = true
			break
		}
	}

	return nil
}

// HookConfig defines configuration for one hook extension point binding.
type HookConfig struct {
	// Enabled controls whether hook should be executed. Defaults to true.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Description is optional human-readable hook description.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Hook is extension point identifier (for example: merge, commit, summary).
	Hook string `json:"hook,omitempty" yaml:"hook,omitempty"`
	// Name is user-assigned hook identifier used for matching/overriding.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Type defines hook backend type. Defaults to shell.
	Type HookType `json:"type,omitempty" yaml:"type,omitempty"`
	// Command is shell command template rendered with hook context data.
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
	// Prompt is LLM user prompt template rendered with hook context data.
	Prompt string `json:"prompt,omitempty" yaml:"prompt,omitempty"`
	// SystemPrompt is optional LLM system prompt template rendered with hook context data.
	SystemPrompt string `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`
	// Model is optional provider/model override for LLM hook execution.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`
	// Thinking is optional --thinking-style override for LLM hook execution.
	Thinking string `json:"thinking,omitempty" yaml:"thinking,omitempty"`
	// Role is optional role override for subagent hook execution.
	Role string `json:"role,omitempty" yaml:"role,omitempty"`
	// OutputTo is optional output field name used by hooks for generated output.
	// For LLM hooks it stores model response text in hook context. Defaults to "result".
	// For shell hooks it is exposed in synthetic response feedback and maps to stdout.
	OutputTo string `json:"output_to,omitempty" yaml:"output_to,omitempty"`
	// ErrorTo is optional output field name used by shell hooks in synthetic response
	// feedback and maps to stderr.
	ErrorTo string `json:"error_to,omitempty" yaml:"error_to,omitempty"`
	// Timeout limits hook execution. Zero means no timeout.
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// RunOn defines shell execution target: host or sandbox.
	RunOn HookRunOn `json:"run-on,omitempty" yaml:"run-on,omitempty"`
	// HookDir is absolute directory path containing hook config and additional files.
	// It is populated by configuration store loaders and is not part of on-disk schema.
	HookDir string `json:"-" yaml:"-"`
	// EmbeddedFiles contains additional files bundled with embedded hook configuration.
	// It is populated by embedded config loader and is not part of on-disk schema.
	EmbeddedFiles map[string][]byte `json:"-" yaml:"-"`
	// EmbeddedSource indicates that hook came from embedded config source and may require
	// materialization of EmbeddedFiles before execution.
	EmbeddedSource bool `json:"-" yaml:"-"`

	enabledConfigured  bool
	descriptionConfigured bool
	hookConfigured     bool
	nameConfigured     bool
	typeConfigured     bool
	commandConfigured  bool
	promptConfigured   bool
	systemConfigured   bool
	modelConfigured    bool
	thinkingConfigured bool
	roleConfigured     bool
	outputToConfigured bool
	errorToConfigured  bool
	timeoutConfigured  bool
	runOnConfigured    bool
}

// UnmarshalJSON unmarshals HookConfig, applies defaults and tracks configured fields.
func (c *HookConfig) UnmarshalJSON(data []byte) error {
	aux := struct {
		Enabled  *bool     `json:"enabled,omitempty"`
		Description string `json:"description,omitempty"`
		Hook     string    `json:"hook,omitempty"`
		Name     string    `json:"name,omitempty"`
		Type     HookType  `json:"type,omitempty"`
		Command  string    `json:"command,omitempty"`
		Prompt   string    `json:"prompt,omitempty"`
		System   string    `json:"system_prompt,omitempty"`
		Model    string    `json:"model,omitempty"`
		Thinking string    `json:"thinking,omitempty"`
		Role     string    `json:"role,omitempty"`
		OutputTo string    `json:"output_to,omitempty"`
		ErrorTo  string    `json:"error_to,omitempty"`
		Timeout  string    `json:"timeout,omitempty"`
		RunOn    HookRunOn `json:"run-on,omitempty"`
	}{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("HookConfig.UnmarshalJSON() [conf.go]: failed to unmarshal hook config: %w", err)
	}

	if aux.Enabled != nil {
		c.Enabled = *aux.Enabled
		c.enabledConfigured = true
	}
	c.Description = aux.Description
	c.Hook = aux.Hook
	c.Name = aux.Name
	c.Type = aux.Type
	c.Command = aux.Command
	c.Prompt = aux.Prompt
	c.SystemPrompt = aux.System
	c.Model = aux.Model
	c.Thinking = aux.Thinking
	c.Role = aux.Role
	c.OutputTo = aux.OutputTo
	c.ErrorTo = aux.ErrorTo
	c.RunOn = aux.RunOn

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("HookConfig.UnmarshalJSON() [conf.go]: failed to unmarshal hook config raw map: %w", err)
	}

	_, c.enabledConfigured = raw["enabled"]
	_, c.descriptionConfigured = raw["description"]
	_, c.hookConfigured = raw["hook"]
	_, c.nameConfigured = raw["name"]
	_, c.typeConfigured = raw["type"]
	_, c.commandConfigured = raw["command"]
	_, c.promptConfigured = raw["prompt"]
	_, c.systemConfigured = raw["system_prompt"]
	_, c.modelConfigured = raw["model"]
	_, c.thinkingConfigured = raw["thinking"]
	_, c.roleConfigured = raw["role"]
	_, c.outputToConfigured = raw["output_to"]
	_, c.errorToConfigured = raw["error_to"]
	_, c.timeoutConfigured = raw["timeout"]
	_, c.runOnConfigured = raw["run-on"]

	if strings.TrimSpace(aux.Timeout) != "" {
		d, err := time.ParseDuration(strings.TrimSpace(aux.Timeout))
		if err != nil {
			return fmt.Errorf("HookConfig.UnmarshalJSON() [conf.go]: invalid timeout: %w", err)
		}
		c.Timeout = d
		c.timeoutConfigured = true
	}

	c.applyDefaults()

	return nil
}

// UnmarshalYAML unmarshals HookConfig, applies defaults and tracks configured fields.
func (c *HookConfig) UnmarshalYAML(node *yaml.Node) error {
	aux := struct {
		Enabled  *bool     `yaml:"enabled,omitempty"`
		Description string `yaml:"description,omitempty"`
		Hook     string    `yaml:"hook,omitempty"`
		Name     string    `yaml:"name,omitempty"`
		Type     HookType  `yaml:"type,omitempty"`
		Command  string    `yaml:"command,omitempty"`
		Prompt   string    `yaml:"prompt,omitempty"`
		System   string    `yaml:"system_prompt,omitempty"`
		Model    string    `yaml:"model,omitempty"`
		Thinking string    `yaml:"thinking,omitempty"`
		Role     string    `yaml:"role,omitempty"`
		OutputTo string    `yaml:"output_to,omitempty"`
		ErrorTo  string    `yaml:"error_to,omitempty"`
		Timeout  string    `yaml:"timeout,omitempty"`
		RunOn    HookRunOn `yaml:"run-on,omitempty"`
	}{}

	if err := node.Decode(&aux); err != nil {
		return fmt.Errorf("HookConfig.UnmarshalYAML() [conf.go]: failed to decode hook config: %w", err)
	}

	if aux.Enabled != nil {
		c.Enabled = *aux.Enabled
		c.enabledConfigured = true
	}
	c.Description = aux.Description
	c.Hook = aux.Hook
	c.Name = aux.Name
	c.Type = aux.Type
	c.Command = aux.Command
	c.Prompt = aux.Prompt
	c.SystemPrompt = aux.System
	c.Model = aux.Model
	c.Thinking = aux.Thinking
	c.Role = aux.Role
	c.OutputTo = aux.OutputTo
	c.ErrorTo = aux.ErrorTo
	c.RunOn = aux.RunOn

	if strings.TrimSpace(aux.Timeout) != "" {
		d, err := time.ParseDuration(strings.TrimSpace(aux.Timeout))
		if err != nil {
			return fmt.Errorf("HookConfig.UnmarshalYAML() [conf.go]: invalid timeout: %w", err)
		}
		c.Timeout = d
		c.timeoutConfigured = true
	}

	if node.Kind != yaml.MappingNode {
		c.applyDefaults()
		return nil
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "enabled":
			c.enabledConfigured = true
		case "hook":
			c.hookConfigured = true
		case "description":
			c.descriptionConfigured = true
		case "name":
			c.nameConfigured = true
		case "type":
			c.typeConfigured = true
		case "command":
			c.commandConfigured = true
		case "prompt":
			c.promptConfigured = true
		case "system_prompt":
			c.systemConfigured = true
		case "model":
			c.modelConfigured = true
		case "thinking":
			c.thinkingConfigured = true
		case "role":
			c.roleConfigured = true
		case "output_to":
			c.outputToConfigured = true
		case "error_to":
			c.errorToConfigured = true
		case "timeout":
			c.timeoutConfigured = true
		case "run-on":
			c.runOnConfigured = true
		}
	}
	c.applyDefaults()

	return nil
}

func (c *HookConfig) applyDefaults() {
	if c.Enabled == false && !c.enabledConfigured {
		c.Enabled = true
	}
	if c.Type == "" {
		c.Type = HookTypeShell
	}
	if c.RunOn == "" {
		c.RunOn = HookRunOnSandbox
	}
	if c.Type == HookTypeLLM && strings.TrimSpace(c.OutputTo) == "" {
		c.OutputTo = "result"
	}
}

// CLIDefaultsConfig defines default values for the cli command flags.
type CLIDefaultsConfig struct {
	// DefaultProvider is the default model provider to use.
	DefaultProvider string `json:"default_provider,omitempty" yaml:"default_provider,omitempty"`
	// DefaultRole is the default agent role to use.
	DefaultRole string `json:"default_role,omitempty" yaml:"default_role,omitempty"`
	// MaxToolThreads is the default --max-threads value.
	MaxToolThreads int `json:"max_tool_threads,omitempty" yaml:"max_tool_threads,omitempty"`
	// Container defines default container execution settings.
	Container ContainerConfig `json:"container,omitempty" yaml:"container,omitempty"`
	// Model is the default --model value.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`
	// Worktree is the default --worktree value.
	Worktree string `json:"worktree,omitempty" yaml:"worktree,omitempty"`
	// Merge is the default --merge value.
	Merge bool `json:"merge,omitempty" yaml:"merge,omitempty"`
	// LogLLMRequests is the default --log-llm-requests value.
	LogLLMRequests bool `json:"log-llm-requests,omitempty" yaml:"log-llm-requests,omitempty"`
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
}

// ModelProviderConfig represents common configuration for model providers.
type ModelProviderConfig struct {
	// Type specifies the provider type (e.g., "ollama", "openai", "anthropic")
	Type string `json:"type" yaml:"type"`
	// Name is a short computer-friendly name for the provider instance (eg. `ollama-local`)
	Name string `json:"name" yaml:"name"`
	// Family is the model family identifier.
	Family string `json:"family,omitempty" yaml:"family,omitempty"`
	// ReleaseDate is the model/template release date in string form.
	ReleaseDate string `json:"release_date,omitempty" yaml:"release_date,omitempty"`
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
	// MaxInputTokens is the maximum number of input tokens accepted by the model.
	MaxInputTokens int `json:"max_input_tokens,omitempty" yaml:"max_input_tokens,omitempty"`
	// MaxOutputTokens is the maximum number of output tokens generated by the model.
	MaxOutputTokens int `json:"max_output_tokens,omitempty" yaml:"max_output_tokens,omitempty"`
	// ModelTags contains model-to-tag mappings specific to this provider.
	// Each mapping has a regexp pattern for model names and a tag to assign.
	ModelTags []ModelTagMapping `json:"model_tags,omitempty" yaml:"model_tags,omitempty"`
	// Reasoning maps effort levels (none, low, medium, high, xhigh) to provider-specific reasoning mode names.
	Reasoning map[string]string `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
	// ReasoningContent defines provider reasoning content field name.
	ReasoningContent string `json:"reasoning_content,omitempty" yaml:"reasoning_content,omitempty"`
	// Temperature indicates whether model/template supports temperature control.
	Temperature *bool `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	// ToolCall indicates whether model/template supports tool calling.
	ToolCall *bool `json:"tool_call,omitempty" yaml:"tool_call,omitempty"`
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

// MarshalJSON implements custom JSON marshaling for ModelProviderConfig.
// It serializes duration fields as strings (e.g., "30s", "5m", "1h0m0s").
func (c ModelProviderConfig) MarshalJSON() ([]byte, error) {
	type Alias ModelProviderConfig
	aux := &struct {
		ConnectTimeout        string `json:"connect_timeout,omitempty"`
		RequestTimeout        string `json:"request_timeout,omitempty"`
		RateLimitBackoffScale string `json:"rate_limit_backoff_scale,omitempty"`
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
	// LLMRetryMaxAttempts is the maximum number of attempts for temporary LLM API failures.
	// Defaults to 10 when unset or invalid.
	LLMRetryMaxAttempts int `json:"llm_retry_max_attempts,omitempty" yaml:"llm_retry_max_attempts,omitempty"`
	// LLMRetryMaxBackoffSeconds caps exponential backoff delay in seconds.
	// Defaults to 60 when unset or invalid.
	LLMRetryMaxBackoffSeconds int `json:"llm_retry_max_backoff_seconds,omitempty" yaml:"llm_retry_max_backoff_seconds,omitempty"`
	// Defaults defines default values for cli command flags.
	Defaults CLIDefaultsConfig `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	// ShadowPaths defines glob patterns redirected to shadow directory when --shadow-dir is enabled.
	ShadowPaths []string `json:"shadow_paths,omitempty" yaml:"shadow_paths,omitempty"`
	// ModelFamilies contains model family templates.
	ModelFamilies map[string]ModelProviderConfig `json:"model_families,omitempty" yaml:"model_families,omitempty"`
	// ModelVendors contains inference vendor templates.
	ModelVendors map[string]ModelProviderConfig `json:"model_vendors,omitempty" yaml:"model_vendors,omitempty"`
	// ModelTemplates contains model templates grouped by training lab.
	ModelTemplates map[string]map[string]ModelProviderConfig `json:"model_templates,omitempty" yaml:"model_templates,omitempty"`
	// VendorFamilyOverrides contains per-provider vendor+family template overrides.
	VendorFamilyOverrides map[string]ModelVendorFamilyTemplateOverride `json:"vendor_family_overrides,omitempty" yaml:"vendor_family_overrides,omitempty"`

	containerConfigured        bool
	containerMountsConfigured  bool
	containerEnvConfigured     bool
	containerImageConfigured   bool
	containerEnabledConfigured bool
}

// UnmarshalJSON unmarshals GlobalConfig and tracks presence of container fields.
func (c *GlobalConfig) UnmarshalJSON(data []byte) error {
	type globalConfigAlias GlobalConfig

	var alias globalConfigAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("GlobalConfig.UnmarshalJSON() [conf.go]: failed to unmarshal global config: %w", err)
	}

	*c = GlobalConfig(alias)
	c.resetContainerPresenceFlags()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("GlobalConfig.UnmarshalJSON() [conf.go]: failed to unmarshal global config raw map: %w", err)
	}

	rawDefaults, ok := raw["defaults"]
	if !ok {
		return nil
	}

	var defaultsMap map[string]json.RawMessage
	if err := json.Unmarshal(rawDefaults, &defaultsMap); err != nil {
		return nil
	}

	rawContainer, ok := defaultsMap["container"]
	if !ok {
		return nil
	}

	c.containerConfigured = true

	var containerMap map[string]json.RawMessage
	if err := json.Unmarshal(rawContainer, &containerMap); err != nil {
		return nil
	}

	_, c.containerMountsConfigured = containerMap["mounts"]
	_, c.containerEnvConfigured = containerMap["env"]
	_, c.containerImageConfigured = containerMap["image"]
	_, c.containerEnabledConfigured = containerMap["enabled"]

	return nil
}

// UnmarshalYAML unmarshals GlobalConfig and tracks presence of container fields.
func (c *GlobalConfig) UnmarshalYAML(node *yaml.Node) error {
	type globalConfigAlias GlobalConfig

	var alias globalConfigAlias
	if err := node.Decode(&alias); err != nil {
		return fmt.Errorf("GlobalConfig.UnmarshalYAML() [conf.go]: failed to decode global config: %w", err)
	}

	*c = GlobalConfig(alias)
	c.resetContainerPresenceFlags()

	if node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		if keyNode.Value != "defaults" {
			continue
		}
		if valueNode.Kind != yaml.MappingNode {
			return nil
		}

		for j := 0; j+1 < len(valueNode.Content); j += 2 {
			subKeyNode := valueNode.Content[j]
			subValueNode := valueNode.Content[j+1]
			if subKeyNode.Value != "container" {
				continue
			}

			c.containerConfigured = true
			if subValueNode.Kind != yaml.MappingNode {
				return nil
			}

			for k := 0; k+1 < len(subValueNode.Content); k += 2 {
				nestedKey := subValueNode.Content[k].Value
				switch nestedKey {
				case "mounts":
					c.containerMountsConfigured = true
				case "env":
					c.containerEnvConfigured = true
				case "image":
					c.containerImageConfigured = true
				case "enabled":
					c.containerEnabledConfigured = true
				}
			}

			return nil
		}
	}

	return nil
}

func (c *GlobalConfig) resetContainerPresenceFlags() {
	c.containerConfigured = false
	c.containerMountsConfigured = false
	c.containerEnvConfigured = false
	c.containerImageConfigured = false
	c.containerEnabledConfigured = false
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

	// GetMCPServerConfigs returns MCP server configurations keyed by server name.
	GetMCPServerConfigs() (map[string]*MCPServerConfig, error)

	// LastMCPServerConfigsUpdate returns timestamp of last update of MCP server configs.
	LastMCPServerConfigsUpdate() (time.Time, error)

	// GetHookConfigs returns hook configurations keyed by hook name.
	GetHookConfigs() (map[string]*HookConfig, error)

	// LastHookConfigsUpdate returns timestamp of last update of hook configs.
	LastHookConfigsUpdate() (time.Time, error)

	// GetAgentConfigFile returns file content from agent configuration namespace.
	// The expected virtual location is conf/agent/<subdir>/<filename>.
	GetAgentConfigFile(subdir, filename string) ([]byte, error)

	// GetModelAliases returns model aliases keyed by alias name.
	GetModelAliases() (map[string]ModelAliasValue, error)

	// LastModelAliasesUpdate returns timestamp of last model alias update.
	LastModelAliasesUpdate() (time.Time, error)
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
