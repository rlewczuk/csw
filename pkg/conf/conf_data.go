package conf

import (
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
}

// GlobalConfig represents the global configuration file structure.
type GlobalConfig struct {
	// ModelTags contains global model-to-tag mappings
	ModelTags []ModelTagMapping `json:"model_tags,omitempty"`
}
