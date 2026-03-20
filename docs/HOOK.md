# `csw hook` command reference

`csw hook` is a diagnostic/testing command group for configured hooks.

## Overview

```bash
csw hook <command> [args]
```

Available subcommands:

- `list` — list all configured hooks
- `info <hook-name>` — print full hook configuration as JSON
- `run <hook-name>` — execute one hook with optional context values

---

## `csw hook list`

Lists hooks loaded from the active configuration hierarchy.

### Usage

```bash
csw hook list
```

### Options

This command has no options.

### Output

Table columns:

- `NAME`
- `HOOK` (lifecycle point, for example `pre_run`, `commit`, `merge`)
- `TYPE` (`shell`, `llm`, `subagent`)
- `DESCRIPTION`
- `STATUS` (`enabled` or `disabled`)

### Example

```bash
csw hook list
```

---

## `csw hook info <hook-name>`

Prints detailed config for one hook in JSON format.

### Usage

```bash
csw hook info <hook-name>
```

### Arguments

- `<hook-name>` (required): configured hook name

### Options

This command has no options.

### Example

```bash
csw hook info pre_run
```

Typical fields in output include:

- `name`, `description`, `hook`, `enabled`, `type`
- shell fields: `command`, `run-on`
- model fields: `prompt`, `system_prompt`, `model`, `thinking`
- routing fields: `output_to`, `error_to`, `timeout`

---

## `csw hook run <hook-name>`

Runs a single hook and prints execution details (rendered prompt/command, stdout/stderr, exit code, feedback, and final session context).

### Usage

```bash
csw hook run <hook-name> [flags]
```

### Arguments

- `<hook-name>` (required): hook to execute

### Options

- `--context KEY=VAL` (repeatable)
  - Injects context key/value for template rendering and hook execution.
- `--context-from KEY=FILENAME` (repeatable)
  - Reads file content and stores it under `KEY` in hook context.
  - Value is raw file content (not trimmed by this command).
- `--run`
  - Enables real model-backed execution for `llm` / `subagent` hooks.
  - Without `--run`, `llm` and `subagent` hooks are simulated (prompt rendered, no provider call).

### Behavior notes

- Disabled hooks cannot be run.
- `shell` hooks execute immediately (host/sandbox behavior depends on hook config).
- `llm` / `subagent` hooks:
  - default mode (without `--run`): preview/simulated execution
  - with `--run`: real provider-backed execution

### Examples

Run a shell hook with inline context:

```bash
csw hook run pre_run --context ticket=ABC-123 --context env=dev
```

Run with context loaded from a file:

```bash
csw hook run pre_run --context ticket=ABC-123 --context-from notes=./tmp/notes.txt
```

Preview an LLM hook (render only, no real model call):

```bash
csw hook run summary --context branch=feature/docs --context user_prompt="Update docs"
```

Execute an LLM hook with real provider/model calls:

```bash
csw hook run summary --run --context branch=feature/docs --context user_prompt="Update docs"
```

---

## Common errors

- `hook not found: <name>` — no hook with that name in loaded config
- `hook "<name>" is disabled` — hook exists but `enabled: false`
- `invalid context entry ... expected KEY=VAL format`
- `invalid context-from entry ... expected KEY=FILENAME format`

---

## Related docs

- `docs/HOOKS.md` — hook system concepts, hook types, lifecycle, feedback protocol, and config details
