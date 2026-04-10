# MCP configuration and usage

This document explains how to configure MCP servers for CSW, how MCP config files work, and how to test/inspect MCP integrations with `csw mcp` commands.

## Where to put MCP configuration files

MCP server configs are loaded from:

```text
<config-root>/mcp/<server-name>.json|yml|yaml
```

Common config roots in CLI hierarchy:

1. `@DEFAULTS` (embedded)
2. `~/.config/csw`
3. `./.csw/config`
4. extra `--config-path` entries

Example project layout:

```text
.csw/config/
  mcp/
    local-files.yml
    remote-api.yml
```

## Quick start examples

### Local MCP server over stdio

`mcp/local-files.yml`:

```yaml
description: Local filesystem MCP server
transport: stdio
cmd: npx
args:
  - -y
  - @modelcontextprotocol/server-filesystem
  - /path/to/project
enabled: true
env:
  LOG_LEVEL: info
tools:
  - read_file
  - list_directory
```

### Remote MCP server over HTTP

`mcp/remote-api.yml`:

```yaml
description: Remote MCP API
transport: https
url: https://mcp.example.com
api_key: YOUR_BEARER_TOKEN
enabled: true
tools:
  - "search_*"
  - get_item
```

Notes:

- `http` and `https` transports are supported.
- For HTTP(S), `api_key` is used as bearer auth token.

## Enabling MCP servers for agent sessions

There are 3 ways to control enabled MCP servers at runtime:

1. **Server file setting** (`enabled: true|false`) in `mcp/<server>.yml`.
2. **Role-level default selection** in `roles/<role>/config.yml`:

```yaml
mcp-servers:
  - local-files
  - remote-api
```

When role `mcp-servers` is present and no CLI MCP flags are passed, only listed servers are enabled; others are disabled.

3. **CLI runtime overrides** on `csw run`:
   - `--mcp-enable <name>[,<name>...]` (repeatable)
   - `--mcp-disable <name>[,<name>...]` (repeatable)

If `--mcp-enable/--mcp-disable` are used, they override role selection behavior for that run.

## MCP configuration file reference

`mcp/<server>.json|yml` fields:

- `description` (string, optional)
- `transport` (string, optional):
  - `stdio` (default)
  - `http`
  - `https`
- `url` (string, required for HTTP/S)
- `api_key` (string, optional; bearer token for HTTP/S)
- `cmd` (string, used for stdio server command)
- `args` (array of strings, optional; appended to command args)
- `env` (map string->string, optional; process env for stdio)
- `enabled` (bool, optional)
- `tools` (array of strings, optional):
  - exact tool names (for example `list_directory`)
  - glob patterns (for example `search_*`)
  - if omitted/empty: all server tools are enabled

### Merge semantics across config layers

- MCP configs merge by server name (filename key).
- Field merge highlights:
  - `enabled` only overrides when explicitly set in higher-priority layer.
  - non-empty scalar fields override lower layer.
  - non-empty `args`/`env` replace lower layer values.
  - `tools` replaces when explicitly provided (including empty list).

## Testing and listing MCP resources

Use `csw mcp` diagnostics commands.

### List configured servers

```bash
csw mcp list
```

Columns:

- `NAME`
- `DESCRIPTION`
- `TRANSPORT`

### Probe server availability

```bash
csw mcp list --status
```

Adds `STATUS`:

- `available`
- `unavailable`
- `disabled`

### List tools exposed by one server

```bash
csw mcp tool list <server-name>
```

Columns:

- `NAME`
- `DESCRIPTION`
- `AVAILABLE` (`yes`/`no` after applying `tools` filters)

### Show details for one tool

```bash
csw mcp tool info <server-name> <tool-name>
```

Outputs:

- server + tool metadata
- parameter summary
- raw input JSON schema

### List resources exposed by one server

```bash
csw mcp resource list <server-name>
```

Columns:

- `NAME`
- `URI`
- `MIME TYPE`
- `DESCRIPTION`

### Read one resource by URI

```bash
csw mcp resource read <server-name> <resource-uri>
```

Outputs:

- server + resource metadata
- content blocks returned by the MCP server
- text content directly (or decoded from base64 `blob` when it contains UTF-8 text)

## `csw mcp` command reference

### `csw mcp`

MCP diagnostics command group.

### `csw mcp list [--status]`

List configured MCP servers, optionally probing live status.

### `csw mcp tool list <server-name>`

List tools from one MCP server and mark which are available after local filters.

### `csw mcp tool info <server-name> <tool-name>`

Show detailed information and input schema for one remote MCP tool.

### `csw mcp resource list <server-name>`

List resources available on one MCP server.

### `csw mcp resource read <server-name> <resource-uri>`

Read one resource by URI from MCP server and print returned content.
