# Package `cmd/csw` Overview

Package `cmd/csw` contains the main CSW command-line application with Cobra commands, CLI session execution, configuration management, and worktree cleanup functionality.

## Important files

* `main.go` - Root Cobra command and top-level flags
* `cli.go` - CLI session execution and worktree management
* `provider.go` - Provider listing, configuration, and authentication
* `role.go` - Role listing, showing, and default selection
* `tool.go` - Tool listing, descriptions, and schema inspection
* `common.go` - Shared config store helpers and utilities
* `clean.go` - Worktree and temporary file cleanup
* `hook.go` - Hook diagnostics and execution commands
* `mcp.go` - MCP server diagnostics and tool inspection

## Important public API objects

* `CLIParams` - Parameters for CLI session execution
* `ConfigScope` - Configuration scope type (local/global)
* `CliCommand()` - Creates CLI session command
* `CleanCommand()` - Creates cleanup command
* `ProviderCommand()` - Creates provider management command
* `RoleCommand()` - Creates role management command
* `ToolCommand()` - Creates tool management command
* `HookCommand()` - Creates hook management command
* `McpCommand()` - Creates MCP diagnostics command
* `GetConfigStore()` - Returns writable config store for scope
* `GetCompositeConfigStore()` - Returns merged composite config store
* `ConfigScopeLocal` - Local configuration scope constant
* `ConfigScopeGlobal` - Global configuration scope constant
