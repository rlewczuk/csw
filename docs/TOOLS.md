# Tools

This document describes custom tool configuration and tool inspection commands.

## Tool configuration layout

Custom tools are loaded from:

```text
<config-root>/tools/<tool-name>/<tool-name>.json|yaml|yml
```

Optional companions in the same directory:

- `<tool-name>.md` (tool description)
- `<tool-name>.schema.json` (tool input schema)

## Example custom tool

`tools/jiraSearch/jiraSearch.yaml`:

```yaml
command:
  - "python3"
  - "{{ .tooldir }}/jira_search.py"
  - "{{ .arg.query }}"
cwd: "{{ .workdir }}"
env:
  JIRA_URL: "https://jira.example.com"
  JIRA_TOKEN: "{{ .arg.token }}"
result:
  matches: "{{ .stdout }}"
error: "jiraSearch failed (exit={{ .exitCode }}): {{ .stderr }}"
timeout: "60s"
loglevel: "info"
roles:
  - developer
  - debugger
```

## Role interaction

Tool files define how a tool executes, but role policy still controls accessibility.

Make sure role config allows usage in `tools-access`, for example:

```yaml
tools-access:
  jiraSearch: allow
```

## Tool command reference

`tool` is a top-level command.

### `csw tool list`

List available tools.

Flags:

- `--role <role>` (filter by role access)
- `--json`

### `csw tool info <tool-name>`

Show full tool metadata (schema + description) as exposed to the model.

Flags:

- `--json`

### `csw tool desc <tool-name>`

Show tool description only.

Flags:

- `--json`
