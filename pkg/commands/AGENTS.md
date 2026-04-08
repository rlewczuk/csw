# Package `pkg/commands` Overview

Package `pkg/commands` parses and expands slash commands.

## Important files

* `commands.go` - Command parsing and prompt expansion
* `commands_test.go` - Command behavior tests

## Important public API objects

* `Metadata` - Command frontmatter fields
* `Command` - Loaded command definition
* `Invocation` - Parsed command invocation
* `ParseInvocation()` - Parses slash command invocation
* `LoadFromDir()` - Loads command from directory
* `ApplyArguments()` - Replaces command argument placeholders
* `ExpandPrompt()` - Expands shell and file references
* `HasDefaultRuntimeShellExpansion()` - Detects default shell expansion
