# CSW User Manual (CLI)

## Quick start

This section shows the fastest way to build and use `csw` for common daily tasks.

### 1) Build `csw`

```bash
go build -o ./bin/csw ./cmd/csw
```

### 2) Run agent in normal project mode (no worktree)

```bash
./bin/csw run "Summarize this repository and suggest first improvements"
```

Optional: set working directory explicitly.

```bash
./bin/csw run --workdir /path/to/project "Fix failing tests"
```

### 3) Run agent in worktree mode

Create and use an isolated git worktree branch for the session:

```bash
./bin/csw run --worktree feature/docs-update "Update user docs"
```

If you also want branch merge at the end:

```bash
./bin/csw run --worktree feature/docs-update --merge "Update user docs"
```

Continue work on an existing worktree branch (reuses extracted worktree if present,
or extracts it if missing):

```bash
./bin/csw run --continue feature/docs-update "Apply requested review fixes"
```

### 4) Run with containerized command execution

```bash
./bin/csw run \
  --container-enabled \
  --container-image ghcr.io/codesnort/csw-runner:latest \
  "Run tests and fix failures"
```

With additional mounts/env:

```bash
./bin/csw run \
  --container-enabled \
  --container-image ghcr.io/codesnort/csw-runner:latest \
  --container-mount /host/cache:/workspace/.cache \
  --container-env CI=true \
  "Build project"
```

### 5) Add LSP to the agent

```bash
./bin/csw run --lsp-server /usr/local/bin/gopls "Refactor this package safely"
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
./bin/csw run --verbose "Investigate why command output is truncated"
```

### 8) Logs: where to find them and how to increase detail

Logs are stored under your project:

- `.cswdata/logs/csw.jsonl` — process-level events
- `.cswdata/logs/sessions/<session-id>/logs.json` — session events
- `.cswdata/logs/sessions/<session-id>/llm.jsonl` — request/response payload logs

Useful flags:

- `--verbose` (on `csw run`) for fuller tool output in terminal
- `--log-llm-requests` (on `csw run`) to include detailed LLM request/response logs

Provider diagnostics (outside session runtime):

- `csw provider test ... --verbose`
- `csw provider models ... --verbose`

---

## Configuration and customization

`csw` is designed to be heavily configurable. You can customize:

- provider definitions and authentication
- default CLI behavior
- roles (prompt fragments, permissions, tool access)
- custom tools and their runtime behavior
- MCP server integration
- lifecycle hooks and automation

Detailed references:

- [docs/PROVIDERS.md](PROVIDERS.md)
- [docs/ROLES.md](ROLES.md)
- [docs/TOOLS.md](TOOLS.md)
- [docs/MCP.md](MCP.md)
- [docs/HOOKS.md](HOOKS.md)

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
  mcp/
    local-filesystem.yml
    remote-http.yml
  hooks/
    pre_run/
      pre_run.yml
      prompt_builder.sh
    summary/
      summary.yml
```

### Providers

Provider setup, OAuth flow, file reference, and provider command reference moved to:

- [docs/PROVIDERS.md](PROVIDERS.md)
- [docs/JETBRAINS.md](JETBRAINS.md) (JetBrains-specific setup)

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

### Roles

Role configuration (permissions, prompt fragments, `mcp-servers`) and command reference moved to:

- [docs/ROLES.md](ROLES.md)

### Tools

Custom tool configuration and `csw tool` command reference moved to:

- [docs/TOOLS.md](TOOLS.md)

### MCP

MCP server configuration, role integration, runtime flags, and `csw mcp` diagnostics are documented in:

- [docs/MCP.md](MCP.md)

### Hooks

Hook configuration, hook types (`shell`, `llm`, `subagent`), `--hook` runtime overrides, and feedback mechanism are documented in:

- [docs/HOOKS.md](HOOKS.md)

---

## Full command reference

### `csw run` reference

Usage:

```text
csw run [flags] ["prompt"]
```

Prompt input modes:

- inline argument: `csw run "Fix lint errors"`
- file input: `csw run @prompt.txt`
- stdin: `echo "Fix docs" | csw run -`

Session and runtime options:

- `--model <provider/model>`: explicit model
- `--role <name>`: role name or configured role alias (default: `developer`)
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

### Provider/Role/Tool/MCP reference

### `csw provider`

See: [docs/PROVIDERS.md](PROVIDERS.md)

### `csw role`

See: [docs/ROLES.md](ROLES.md)

### `csw tool`

See: [docs/TOOLS.md](TOOLS.md)

### `csw mcp`

See: [docs/MCP.md](MCP.md)

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

See provider schema reference: [docs/PROVIDERS.md](PROVIDERS.md)

### `roles/<role>/config.json|yml`

See role schema reference: [docs/ROLES.md](ROLES.md)

### `tools/<tool>/...`

See custom tools schema reference: [docs/TOOLS.md](TOOLS.md)

### `mcp/<server>.json|yml`

See MCP schema reference: [docs/MCP.md](MCP.md)

### `hooks/<hook-name>/<hook-name>.json|yml`

See hooks schema reference: [docs/HOOKS.md](HOOKS.md)

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
go run ./cmd/csw run "Explain this codebase structure"
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
