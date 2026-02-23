# cmd/csw

`cmd/csw` implements the main CSW command-line application. It defines root command/flags, configuration and provider management subcommands, role/tool inspection commands, runtime bootstrap wiring, and execution paths for both TUI and `cli` session modes.

## Major files

- `main.go`: Root Cobra command and top-level flags; wires `conf` and `cli` command trees and default execution mode.
- `bootstrap.go`: Runtime bootstrap pipeline that builds system dependencies (config stores, providers, tools, VFS/VCS, optional LSP).
- `cli.go`: `csw cli` command implementation, including prompt input modes, session execution, and optional worktree/merge/commit-message flow.
- `provider.go`: `conf provider` command suite for listing, showing, adding/removing, default selection, auth, and model discovery.
- `role.go`: `conf role` command suite for role listing/showing/default selection and role prompt rendering.
- `tool.go`: `conf tool` command suite for tool listing, descriptions, and schema inspection.
- `common.go`: Shared command helpers for config path resolution, store selection, provider mapping, and model/tag utilities.
- `tui.go`: TUI execution path and app wiring (presenter/view initialization, signal handling, session startup).
