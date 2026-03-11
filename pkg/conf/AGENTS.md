# Package `pkg/conf` Overview

Package `pkg/conf` defines the configuration domain for CSW and the config-store abstractions used by the rest of the system. It covers global settings, model providers, agent roles, tool and file access policies, and layered config loading across defaults and local/project scopes.

## Important files

* `conf.go` - Core public configuration API and interfaces
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
* `AccessFlag` - Access control flag type (auto/allow/deny/ask)
* `AuthMode` - Authentication mode for model providers
* `ModelTagMapping` - Model-to-tag mapping rule
* `FileAccess` - File access permissions struct
* `ModelProviderCost` - Token pricing for context tier
* `ModelProviderModalities` - Model input/output modalities
* `ModelVendorFamilyTemplateOverride` - Vendor and family template overrides
