# Package `pkg/ui/cli` Overview

Package `pkg/ui/cli` contains CLI implementations of UI interfaces for terminal-based interaction. It provides plain-text chat and app views for non-interactive and interactive command-line modes.

## Important files

* `cli_app_view.go` - CLI app-level view implementation
* `cli_chat_view.go` - CLI chat view with input/output handling
* `slug_prefix.go` - CLI output prefix formatting utilities

## Important public API objects

* `CliAppView` - CLI implementation of app-level view interface
* `NewAppView` - Creates CLI app view with optional slug prefix
* `NewCliAppView` - Creates new CLI app view writing to output
* `CliChatView` - Text-based chat view for stdout/stdin
* `NewCliChatView` - Creates CLI chat view with interactive options
