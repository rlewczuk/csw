# Roles

This document describes role configuration, role prompts, permissions, and role commands.

## Role directory layout

Roles are loaded from:

```text
<config-root>/roles/<role-name>/
  config.json|config.yml|config.yaml
  *.md
```

Special role:

- `roles/all/` provides shared prompt/tool fragments merged into other roles.

## Role config reference

Main fields in `roles/<role>/config.*`:

- `name`
- `aliases` (optional alternate names for the role; usable anywhere a role is selected)
- `description`
- `vfs-privileges` (path pattern -> `{read,write,delete,list,find,move}`)
- `tools-access` (tool name/pattern -> `auto|allow|deny|ask`)
- `run-privileges` (command regex -> `auto|allow|deny|ask`)
- `hidden-patterns` (glob-style patterns hidden from VFS operations)
- `mcp-servers` (list of MCP server names for this role)

Prompt fragments:

- `roles/<role>/*.md` are merged into the effective system prompt.

## Example role config

```yaml
name: debugger
aliases:
  - debug
  - triage
description: Debug-focused role with strict execution style
vfs-privileges:
  "**":
    read: allow
    write: allow
    delete: ask
    list: allow
    find: allow
    move: ask
tools-access:
  "**": allow
run-privileges:
  "rm*": deny
  ".*": allow
hidden-patterns:
  - "**/.secrets/**"
mcp-servers:
  - local-filesystem
```

## Merge behavior (multi-layer config)

Across config layers (`@DEFAULTS` -> global -> project -> extra paths):

- Roles merge by role name.
- Scalar maps are overridden by more specific layers.
- Prompt/tool fragments merge by fragment key.
- `hidden-patterns` are additive.
- `aliases` from a more specific layer replace previous list when provided.
- `mcp-servers` from a more specific layer replace previous list when provided.

## Role aliases

Role aliases are alternative role names resolved to the canonical role.

Example:

- role `developer` with aliases `dev`, `build`
- `csw run --role dev ...` behaves like `--role developer`
- command frontmatter `agent: build` behaves like `agent: developer`
- subagent role fields (for example in hooks) accept aliases as well

## Role command reference

`role` is a top-level command.

### `csw role list`

List available roles from composite configuration.

Flags:

- `--json`

### `csw role show <role>`

Show role config details.

Flags:

- `--json`
- `--system-prompt` (render effective system prompt)
- `--model <provider/model>` (used with `--system-prompt`)

### `csw role set-default <role>`

Set `default_role` in global config.

Scope flags:

- `--local`
- `--global`
- `--to <config-dir>`

### `csw role get-default`

Print current default role.

Flags:

- `--json`
