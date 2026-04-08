# Package `cmd/csw` Overview

Package `cmd/csw` implements the CSW command-line application.

## Important files

* `main.go` - Root command and global flags
* `cli.go` - CLI session command implementation
* `provider.go` - Provider management subcommands
* `role.go` - Role management subcommands
* `tool.go` - Tool inspection subcommands
* `common.go` - Config store helper functions
* `clean.go` - Worktree cleanup command
* `hook.go` - Hook diagnostics commands
* `mcp.go` - MCP diagnostics commands
* `task.go` - Persistent task commands

## Important public API objects

* `CLIParams` - Parameters for CLI execution
* `ConfigScope` - Configuration scope enum
* `ConfigScope` values: `local`, `global`
* `CliCommand()` - Creates `cli` command
* `CleanCommand()` - Creates `clean` command
* `ProviderCommand()` - Creates `provider` command
* `RoleCommand()` - Creates `role` command
* `ToolCommand()` - Creates `tool` command
* `HookCommand()` - Creates `hook` command
* `McpCommand()` - Creates `mcp` command
* `TaskCommand()` - Creates `task` command
* `GetConfigStore()` - Returns writable config store
* `GetCompositeConfigStore()` - Returns composite config store
