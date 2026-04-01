# Package `pkg/commands` Overview

Package `pkg/commands` contains slash command parsing and expansion for agent prompts with YAML frontmatter support.

## Important files

* `commands.go` - slash command parsing and template expansion
* `commands_test.go` - unit tests for command functionality

## Important public API objects

* `Metadata` - YAML frontmatter fields for commands
* `Command` - loaded command definition with template
* `Invocation` - parsed slash command with arguments
* `ParseInvocation` - parses slash command from prompt
* `LoadFromDir` - loads command from .md file in directory
* `ApplyArguments` - replaces $ARGUMENTS and $N placeholders
* `ExpandPrompt` - expands shell and file references
* `HasDefaultRuntimeShellExpansion` - checks for default runtime shell usage
