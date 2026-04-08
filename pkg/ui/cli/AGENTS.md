# Package `pkg/ui/cli` Overview

Package `pkg/ui/cli` provides terminal UI implementations for app and chat flows. It handles prefixed output, interaction modes, and tool rendering in `pkg/ui/cli`.

## Important files

* `cli_app_view.go` - App diagnostics output and retry prompt behavior
* `cli_chat_view.go` - Chat rendering, tool updates, input handling
* `slug_prefix.go` - Slug normalization and per-line prefix formatting

## Important public API objects

* `CliAppView` - CLI app view with diagnostic logging.
* `NewAppView` - Creates app view with optional slug.
* `NewCliAppView` - Creates CliAppView writing to output.
* `CliChatView` - CLI chat view for rendering and input.
* `NewCliChatView` - Creates chat view with runtime options.
