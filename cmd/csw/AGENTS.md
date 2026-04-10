# Package `cmd/csw` Overview

Package `cmd/csw` implements the CSW command-line application. It defines the root command, global flags, and subcommands for running agent sessions, managing providers, roles, tools, hooks, MCP servers, persistent tasks, and cleaning up worktrees. It also exposes helper functions for configuration store access and path resolution.

## Important files

* `main.go` - Root command and global flags
* `run.go` - Run session command implementation
* `provider.go` - Provider management subcommands
* `role.go` - Role management subcommands
* `tool.go` - Tool inspection subcommands
* `common.go` - Config store helper functions
* `clean.go` - Worktree cleanup command
* `hook.go` - Hook diagnostics commands
* `mcp.go` - MCP diagnostics commands
* `task.go` - Persistent task commands

## Important public API objects

* `RunParams` - Parameters for run execution
* `ConfigScope` - Configuration scope enum
* `ConfigScope` values: `ConfigScopeLocal`, `ConfigScopeGlobal`
* `RunCommand()` - Creates run command
* `CleanCommand()` - Creates clean command
* `ProviderCommand()` - Creates provider command
* `RoleCommand()` - Creates role command
* `ToolCommand()` - Creates tool command
* `HookCommand()` - Creates hook command
* `McpCommand()` - Creates mcp command
* `TaskCommand()` - Creates task command
* `GetConfigStore()` - Returns writable config store
* `GetCompositeConfigStore()` - Returns composite config store
* `BuildConfigPath()` - Builds config path hierarchy string
* `ValidateConfigPaths()` - Validates colon-separated config paths
* `ResolveWorkDir()` - Resolves working directory path
* `ResolveModelName()` - Resolves model name from config
* `CreateProviderMap()` - Builds provider to model map
* `CreateModelTagRegistry()` - Creates model tag registry
