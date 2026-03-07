# CSW User Manual (CLI)

## Quick start

This section shows the fastest way to build and use `csw` for common daily tasks.

### 1) Build `csw`

```bash
go build -o ./bin/csw ./cmd/csw
```

### 2) Run agent in normal project mode (no worktree)

```bash
./bin/csw cli "Summarize this repository and suggest first improvements"
```

Optional: set working directory explicitly.

```bash
./bin/csw cli --workdir /path/to/project "Fix failing tests"
```

### 3) Run agent in worktree mode

Create and use an isolated git worktree branch for the session:

```bash
./bin/csw cli --worktree feature/docs-update "Update user docs"
```

If you also want branch merge at the end:

```bash
./bin/csw cli --worktree feature/docs-update --merge "Update user docs"
```

Continue work on an existing worktree branch (reuses extracted worktree if present,
or extracts it if missing):

```bash
./bin/csw cli --continue feature/docs-update "Apply requested review fixes"
```

### 4) Run with containerized command execution

```bash
./bin/csw cli \
  --container-enabled \
  --container-image ghcr.io/codesnort/csw-runner:latest \
  "Run tests and fix failures"
```

With additional mounts/env:

```bash
./bin/csw cli \
  --container-enabled \
  --container-image ghcr.io/codesnort/csw-runner:latest \
  --container-mount /host/cache:/workspace/.cache \
  --container-env CI=true \
  "Build project"
```

### 5) Add LSP to the agent

```bash
./bin/csw cli --lsp-server /usr/local/bin/gopls "Refactor this package safely"
```

If `--lsp-server` is empty, LSP is disabled.

### 6) Clean worktrees

Clean one worktree:

```bash
./bin/csw clean --worktree feature/docs-update
```

Clean all stale worktrees:

```bash
./bin/csw clean --worktree --all
```

Equivalent full cleanup shortcut:

```bash
./bin/csw clean --all
```

### 7) Use verbose mode for debugging session behavior

```bash
./bin/csw cli --verbose "Investigate why command output is truncated"
```

### 8) Logs: where to find them and how to increase detail

Logs are stored under your project:

- `.cswdata/logs/csw.jsonl` — process-level events
- `.cswdata/logs/sessions/<session-id>/logs.json` — session events
- `.cswdata/logs/sessions/<session-id>/llm.jsonl` — request/response payload logs

Useful flags:

- `--verbose` (on `csw cli`) for fuller tool output in terminal
- `--log-llm-requests` (on `csw cli`) to include detailed LLM request/response logs

Provider diagnostics (outside session runtime):

- `csw conf provider test ... --verbose`
- `csw conf provider models ... --verbose`

---

## Configuration and customization

`csw` is designed to be heavily configurable. You can customize:

- provider definitions and authentication
- default CLI behavior
- roles (prompt fragments, permissions, tool access)
- custom tools and their runtime behavior

### Configuration hierarchy and file locations

`csw` loads configuration from layered sources (later sources override earlier ones):

1. Embedded defaults (`@DEFAULTS`)
2. Global config (`~/.config/csw`)
3. Project config (`./.csw/config`)
4. Optional extra paths from `--config-path`

Default hierarchy used by CLI:

```text
@DEFAULTS:~/.config/csw:./.csw/config
```

You can customize this with:

- `--project-config <dir>`
- `--config-path <dir1:dir2:...>`

Typical config layout:

```text
.csw/config/
  global.json
  models/
    ollama-local.yml
    openai-codex.json
  roles/
    all/
      config.yml
      10-summary.md
    developer/
      config.yml
      10-system.md
    debugger/
      config.yml
      10-system.md
  tools/
    jiraSearch/
      jiraSearch.yaml
      jiraSearch.md
      jiraSearch.schema.json
```

### Configure providers (including OAuth)

#### Basic provider management commands

```bash
csw conf provider list
csw conf provider show <provider-name>
csw conf provider set-default <provider-name>
csw conf provider remove <provider-name>
```

Scopes for write commands:

- `--local` (default, project config)
- `--global`
- `--to <custom-config-dir>`

#### Example: local Ollama provider

```bash
csw conf provider add ollama-local \
  --type ollama \
  --url http://localhost:11434 \
  --description "Local Ollama"

csw conf provider set-default ollama-local
```

#### Example: OpenAI Codex-style provider with OAuth

Create provider config using JSON mode:

```bash
cat <<'JSON' | csw conf provider add openai-codex --json
{
  "type": "openai",
  "name": "openai-codex",
  "description": "OpenAI Codex via OAuth",
  "url": "https://api.openai.com/v1",
  "auth_mode": "oauth2",
  "auth_url": "https://auth.openai.com/oauth/authorize",
  "token_url": "https://auth.openai.com/oauth/token",
  "client_id": "YOUR_CLIENT_ID"
}
JSON
```

Run OAuth flow:

```bash
csw conf provider auth openai-codex
```

`csw` opens a localhost callback flow (`http://localhost:1455/auth/callback`) and stores access/refresh tokens in provider config.

#### JetBrains AI provider

CSW supports `jetbrains` provider type for JetBrains private AI endpoint integration.

See full setup guide, token acquisition, and sample config in:

- [docs/JETBRAINS.md](JETBRAINS.md)

#### Validate provider setup

List models:

```bash
csw conf provider models
csw conf provider models openai-codex
```

Test one provider/model pair:

```bash
csw conf provider test openai-codex gpt-5-codex
```

With protocol-level diagnostics:

```bash
csw conf provider test openai-codex gpt-5-codex --verbose
csw conf provider models openai-codex --verbose
```

### Configure defaults for CLI options

Set defaults in `global.json` (or `global.yml`):

```json
{
  "default_provider": "openai-codex",
  "default_role": "developer",
  "defaults": {
    "model": "openai-codex/gpt-5-codex",
    "worktree": "feature/%",
    "merge": false,
    "log-llm-requests": true,
    "thinking": "medium",
    "lsp-server": "/usr/local/bin/gopls"
  }
}
```

`worktree` values ending with `%` are expanded to an auto-generated branch suffix from the prompt.

Supported CLI defaults:

- `defaults.model`
- `defaults.worktree`
- `defaults.merge`
- `defaults.log-llm-requests`
- `defaults.thinking`
- `defaults.lsp-server`

### Configure custom roles

Use role commands for inspection/debugging:

```bash
csw conf role list
csw conf role show developer
csw conf role show developer --system-prompt --model openai-codex/gpt-5-codex
csw conf role set-default developer
csw conf role get-default
```

Define a custom role by adding files under `roles/<role-name>/`.

Example `roles/debugger/config.yml`:

```yaml
name: debugger
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
```

Add prompt fragments such as `roles/debugger/10-system.md`, `20-style.md`, etc.

### Configure custom tools

Custom tools are loaded from:

```text
<config-root>/tools/<tool-name>/<tool-name>.json|yaml|yml
```

Optional companion files:

- `<tool-name>.md` (tool description)
- `<tool-name>.schema.json` (tool schema)

Example custom tool config `tools/jiraSearch/jiraSearch.yaml`:

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

Then inspect/debug via:

```bash
csw conf tool list
csw conf tool list --role debugger
csw conf tool info jiraSearch
csw conf tool desc jiraSearch
```

Also ensure role access allows the tool in `roles/<role>/config.yml` (`tools-access`).

---

## Full command reference

### `csw cli` reference

Usage:

```text
csw cli [flags] ["prompt"]
```

Prompt input modes:

- inline argument: `csw cli "Fix lint errors"`
- file input: `csw cli @prompt.txt`
- stdin: `echo "Fix docs" | csw cli -`

Session and runtime options:

- `--model <provider/model>`: explicit model
- `--role <name>`: role name (default: `developer`)
- `--workdir <dir>`: working directory
- `--interactive`: allow interactive follow-up input
- `--allow-all-permissions`: auto-allow permission prompts
- `--save-session`: save conversation
- `--save-session-to <file>`: save to specific markdown file

Resume options:

- `--resume [last|<session-id>]`: resume previous session
- `--resume-continue`: continue resumed session with new user message
- `--force`: force resume even with no pending work

Worktree and merge options:

- `--worktree <branch>`: run in dedicated git worktree
- `--continue <branch>`: continue work in existing worktree branch (branch must exist)
- `--merge`: merge worktree branch after commit (requires `--worktree`)
- `--commit-message <template>`: custom commit template

Container options:

- `--container-enabled`
- `--container-disabled`
- `--container-image <image>`
- `--container-mount <host:container>` (repeatable)
- `--container-env <KEY=VALUE>` (repeatable)

Diagnostics and behavior:

- `--verbose`: show full tool output
- `--log-llm-requests`: write detailed LLM logs
- `--thinking <low|medium|high|xhigh|true|false>`
- `--lsp-server <path>`: enable LSP server
- `--bash-run-timeout <duration-or-seconds>`

Config path controls:

- `--project-config <dir>`
- `--config-path <dir1:dir2:...>`

### `csw conf` reference

### `csw conf provider`

Subcommands:

- `list`
- `show <provider-name>`
- `add <provider-name> [--type --url --description --api-key --header key=value]`
- `remove <provider-name>`
- `set-default <provider-name>`
- `test <provider-name> <model-name> [--streaming] [--verbose]`
- `auth <provider-name>`
- `models [provider] [--verbose]`

Persistent flags:

- `--json`
- `--local` / `--global` / `--to <path>`

### `csw conf role`

Subcommands:

- `list [--json]`
- `show <role> [--json] [--system-prompt] [--model <provider/model>]`
- `set-default <role> [--local|--global|--to]`
- `get-default [--json]`

### `csw conf tool`

Subcommands:

- `list [--role <role>] [--json]`
- `info <tool-name> [--json]`
- `desc <tool-name> [--json]`

### `csw clean` reference

Usage:

```text
csw clean [--worktree [branch-name]] [--all] [--workdir <dir>]
```

Options:

- `--worktree <branch>`: clean one worktree
- `--worktree --all`: clean all worktrees
- `--all`: full cleanup shortcut
- `--workdir <dir>`: target repository

### Configuration files (detailed)

### `global.json` / `global.yml`

Top-level keys:

- `default_provider`
- `default_role`
- `defaults` (CLI default flags)
- `container` (default container behavior)
- `model_tags`
- `tool_selection`
- `llm_retry_max_attempts`
- `llm_retry_max_backoff_seconds`
- `context_compaction_threshold`

### `models/<provider>.json|yml`

Main keys:

- `type`, `name`, `description`, `url`
- `api_key`, `headers`, `query_params`
- `connect_timeout`, `request_timeout`
- generation defaults (`default_temperature`, etc.)
- `model_tags`
- OAuth keys: `auth_mode`, `auth_url`, `token_url`, `client_id`, `client_secret`, `refresh_token`

### `roles/<role>/config.json|yml`

Main keys:

- `name`, `description`
- `vfs-privileges`
- `tools-access`
- `run-privileges`
- `hidden-patterns`

Role prompt files:

- `roles/<role>/*.md` are prompt fragments.
- `roles/all/` acts as common role layer.

### `tools/<tool>/...`

Custom tool definition file:

- `<tool>.json`, `<tool>.yaml`, or `<tool>.yml`

Tool fragment helpers:

- `<tool>.md` (description)
- `<tool>.schema.json` (schema)

---

## Build and run from source

### Build from source

```bash
git clone https://github.com/rlewczuk/csw.git
cd csw
go build -o ./bin/csw ./cmd/csw
```

### Run from source

```bash
go run ./cmd/csw cli "Explain this codebase structure"
```

### Run tests

Before tests:

```bash
./tmpclean.sh
```

Run all tests:

```bash
go test ./... -timeout 60s
```

Run a single package:

```bash
go test ./test -v -timeout 60s
```

### Run integration tests

Integration tests are feature-gated via `_integ/*.enabled` files (`yes` value).

Examples:

- `_integ/all.enabled` with `yes` enables all gated integration tests.
- `_integ/runc.enabled` with `yes` enables container-related tests.

Run integration tests with longer timeout:

```bash
go test ./... -timeout 300s
```

### Contribute to `csw`

Typical workflow:

1. Fork `rlewczuk/csw`
2. Create feature branch
3. Implement focused changes
4. Run tests (`./tmpclean.sh` then `go test ./... -timeout 60s`)
5. Open pull request with clear description and rationale

### Report bugs and ask for help

Project repository:

- https://github.com/rlewczuk/csw

When reporting issues, include:

- `csw` command used
- exact error/output
- environment details (OS, Go version, provider)
- relevant logs from `.cswdata/logs/...`
- reproducible steps
