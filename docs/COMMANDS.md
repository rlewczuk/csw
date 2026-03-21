# Commands

This document describes **custom slash commands** used with `csw cli`.

Custom commands let you store reusable prompt templates as files and invoke them with:

```text
/<command-name> [arguments...]
```

`csw` loads command files from:

```text
<workdir>/.agents/commands/*.md
```

When `--shadow-dir` is set, commands are loaded from:

```text
<shadow-dir>/.agents/commands/*.md
```

---

## Using custom commands

### Basic usage

Invoke a command as the prompt:

```bash
csw cli "/review api package"
```

or pass arguments as extra positional CLI args:

```bash
csw cli /review api package
```

Both forms resolve to command name `review` with arguments `api`, `package`.

### Quoted arguments

Arguments are shell-like tokenized, including quoted values:

```bash
csw cli '/review src/pkg "naming consistency"'
```

In this case, the second argument is `naming consistency`.

### How rendering works

When a command is invoked, `csw`:

1. Loads `.agents/commands/<name>.md`
2. Applies argument placeholders in template body
3. Expands inline shell expressions (``!`...` ``)
4. Expands script shortcuts (`!script.sh` and `!!script.sh`)
5. Expands file references (`@path`)
6. Sends final rendered text as the session prompt

If the rendered prompt is empty, command execution fails.

### Behavior with `--model` and `--role`

Command metadata may define model/agent defaults (see format below).

- If CLI flags `--model` or `--role` are provided, **CLI flags win**.
- If not provided, command metadata can override defaults.

### Examples

#### Example: argument placeholders

Command file template:

```text
Task: Review $1
Extra context: $2
All args: $ARGUMENTS
```

Invocation:

```bash
csw cli '/review pkg/core "focus on error handling"'
```

Rendered prompt:

```text
Task: Review pkg/core
Extra context: focus on error handling
All args: pkg/core focus on error handling
```

#### Example: file inclusion

Template:

```text
Review this file:
@pkg/core/session.go
```

`@pkg/core/session.go` is replaced with file content (relative to current workdir unless absolute path).

#### Example: shell expansion

Template:

```text
Current branch: !`git branch --show-current`
Changed files:
!`git diff --name-only`
```

Each ``!`...` `` is executed and replaced by command output.

Notes:

- Non-zero exit code fails command rendering.
- If shell expansion is present, container mode is automatically requested unless `--container-disabled` is passed.

#### Example: script shortcuts

Template:

```text
Default runtime script output:
!scripts/context.sh

Always host script output:
!!scripts/host_context.sh
```

Notes:

- `!some_script.sh` runs using the **default runtime** (host or container, depending on CLI/config/runtime settings).
- `!!some_script.sh` always runs on the **host**.
- Script output replaces the token (trailing newline is trimmed).
- Non-zero exit code fails command rendering.

---

## Defining custom commands

Create files in:

```text
.agents/commands/
```

Each command is one Markdown file named:

```text
<command-name>.md
```

So `/review` maps to `.agents/commands/review.md`.

### Command naming rules

- Command name cannot be empty
- Command name cannot contain `/` or path separators
- File must exist and contain non-empty template body

### Minimal command example

`/.agents/commands/review.md`:

```md
Please review the following scope:
$ARGUMENTS

Focus on correctness, clarity, and test coverage.
```

Use it with:

```bash
csw cli "/review pkg/core pkg/tool"
```

### Command with metadata example

`.agents/commands/review.md`:

```md
---
description: Review code changes for quality and safety
agent: reviewer
model: openai-codex/gpt-5-codex
---
Review scope: $ARGUMENTS

Checklist:
- correctness
- error handling
- edge cases
- tests
```

This metadata sets default role/model for this command (unless overridden by CLI flags).

### Command with file + shell expansion example

`.agents/commands/debug.md`:

```md
---
description: Debug a failing test in detail
agent: debugger
---
Investigate failure for: $1

Recent test output:
!`go test ./... -run $1 -count=1`

Relevant file:
@$2
```

Invocation:

```bash
csw cli '/debug TestAgentCoreInitialization pkg/core/session.go'
```

---

## Custom command file format reference

A command file is Markdown with optional YAML frontmatter.

### Structure

```md
---
description: <optional string>
agent: <optional role name>
model: <optional provider/model>
---
<template body>
```

If frontmatter is missing or invalid, file is treated as plain template body.

### Frontmatter fields

- `description` (optional): free-text description
- `agent` (optional): default role for the command (equivalent to CLI role override source)
- `model` (optional): default model in `provider/model` format

### Template placeholders

- `$ARGUMENTS` → all command arguments joined by spaces
- `$1`, `$2`, ... `$N` → positional argument values (1-based)
  - Missing index resolves to empty string

### Inline shell expressions

- Syntax: ``!`<shell command>` ``
- Command output (without trailing newline) replaces expression
- Non-zero exit status fails command rendering

### Script shortcuts

- Syntax: `!path/to/script.sh`
  - Runs script with default runtime shell (host or container).
- Syntax: `!!path/to/script.sh`
  - Runs script with host shell only.
- Script output (without trailing newline) replaces the token
- Non-zero exit status fails command rendering

### File references

- Syntax: `@path/to/file`
- File content replaces the reference
- Relative paths are resolved against the session workdir
- Trailing punctuation in path tokens (`, . ; :`) is trimmed during resolution

### Error cases

Command invocation fails when:

- command file is missing
- template body is empty
- invocation has invalid quoting
- shell expression execution fails
- referenced file cannot be read
- final rendered prompt is empty
