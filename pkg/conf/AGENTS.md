# Package `pkg/conf` Overview

Package `pkg/conf` defines configuration models and store interfaces.

## Important files

* `conf.go` - Configuration types and interfaces
* `merge.go` - Clone and merge helpers

## Important public API objects

* `ConfigStore` - Configuration read interface
* `WritableConfigStore` - Configuration read-write interface
* `GlobalConfig` - Global configuration model
* `ModelProviderConfig` - Model provider configuration
* `AgentRoleConfig` - Agent role configuration
* `ToolSelectionConfig` - Tool selection rules
* `ContainerConfig` - Container defaults configuration
* `RunDefaultsConfig` - CLI run defaults configuration
* `MCPServerConfig` - MCP server configuration
* `HookConfig` - Hook configuration model
* `ModelAliasValue` - Model alias value container
* `FileAccess` - File access permissions
* `ModelTagMapping` - Model tag mapping
* `ModelProviderCost` - Model pricing tier
* `ModelProviderModalities` - Model modality configuration
* `ModelVendorFamilyTemplateOverride` - Vendor family template overrides
* `AccessFlag` values: `auto`, `allow`, `deny`, `ask`
* `AuthMode` values: `none`, `api_key`, `oauth2`
* `MCPTransportType` values: `stdio`, `http`, `https`
* `HookType` values: `shell`, `llm`, `subagent`
* `HookRunOn` values: `host`, `sandbox`
