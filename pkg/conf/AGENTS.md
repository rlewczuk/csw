# Package `pkg/conf` Overview

Package `pkg/conf` defines the configuration domain for CSW and the config-store abstractions used by the rest of the system. It covers global settings, model providers, agent roles, tool and file access policies, and layered config loading across defaults and local/project scopes.

## Important files

* `conf.go` - Core configuration types and interfaces
* `merge.go` - Config merge and clone operations

## Important public API objects

* `ConfigStore` - Interface for accessing configuration data
* `WritableConfigStore` - Extends ConfigStore with write operations
* `GlobalConfig` - Global configuration file structure
* `ModelProviderConfig` - Common configuration for model providers
* `AgentRoleConfig` - Agent role configuration with privileges
* `ToolSelectionConfig` - Tool availability rules across model tags
* `ContainerConfig` - Default container execution settings
* `CLIDefaultsConfig` - Default values for CLI command flags
* `MCPServerConfig` - MCP server configuration
* `HookConfig` - Hook extension point binding configuration
* `FileAccess` - File access permissions struct
* `ModelTagMapping` - Model-to-tag mapping rule
* `ModelProviderCost` - Token pricing for context tier
* `ModelProviderModalities` - Model input/output modalities
* `ModelVendorFamilyTemplateOverride` - Vendor and family template overrides
* `AccessFlag` - Access control flag type (auto/allow/deny/ask)
* `AuthMode` - Authentication mode for model providers (none/api_key/oauth2)
* `MCPTransportType` - MCP server transport type (stdio/http/https)
* `HookType` - Hook execution backend type (shell/llm/subagent)
* `HookRunOn` - Shell hook execution target (host/sandbox)
