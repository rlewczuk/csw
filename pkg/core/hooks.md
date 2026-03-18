# Hooks mechanism design (`pkg/core/hooks.md`)

## Goal

Provide configurable extension points in agent lifecycle where external programs can run.
Hooks can observe lifecycle context and, for selected hook points, replace built-in behavior.

Current implementation includes:

- Hook configuration model in `pkg/conf`
- Hook config loading/merging in config stores (`embedded`, `local`, `composite`)
- Shell hook execution engine in `pkg/core` (`HookEngine`)
- Merge hook integration in CLI worktree finalization (`cmd/csw/cli.go`)

## Configuration model

Hook config object (`conf.HookConfig`) fields:

- `enabled` (default: `true`)
- `hook` (extension point name, for example `merge`)
- `name` (user-assigned unique hook id)
- `type` (`shell` default, `llm`, `subagent` reserved for future)
- `command` (for shell hooks, `text/template` command template)
- `timeout` (`time.Duration`, `0` means no timeout)
- `run-on` (`host` or `sandbox`, default `sandbox`)

Config file location:

- `config/hooks/<name>.yaml` (or `.yml` / `.json`)

## Config loading and merging

All config stores expose hooks via `conf.ConfigStore`:

- `GetHookConfigs() (map[string]*HookConfig, error)`
- `LastHookConfigsUpdate() (time.Time, error)`

### Local store (`pkg/conf/impl/local.go`)

- Loads files from `hooks/`
- YAML has precedence over JSON for same base filename
- If `name` is missing, filename base is used

### Embedded store (`pkg/conf/impl/embedded.go`)

- Loads from embedded `conf/hooks/`
- Same precedence and naming behavior as local store

### Composite store (`pkg/conf/impl/composite.go`)

- Merges in source order: less specific -> more specific
- Merge key is `name`
- For same name, only fields explicitly configured in more specific source replace previous ones

This supports `project > user > embedded` overrides without requiring full object duplication.

## Hook context data

`HookEngine` maintains cumulative map `HookContext` over session lifecycle.

Context keys currently used:

- `branch`
- `summary`
- `hook`
- `workdir`
- `rootdir`
- `status` (`none`, `running`, `success`, `failed`)

The map is updated over time in CLI flow and reused for each hook execution.

## Shell execution model

Shell hooks are executed by `HookEngine.Execute()`:

1. Resolve enabled hook by extension point (`hook` field)
2. Render `command` via `text/template` with context map
3. Choose runner by `run-on`:
   - `sandbox` -> sandbox runner if available, otherwise host runner
   - `host` -> host runner if available, otherwise sandbox runner
4. Export context as environment variables:
   - key `branch` -> `CSW_BRANCH`
   - key `rootdir` -> `CSW_ROOTDIR`
   - etc.
5. Run command with optional timeout
6. Capture and display both stdout and stderr through `IAppView.ShowMessage()`

Hook success/failure is based on exit code:

- Exit code `0` => success
- Non-zero exit code => error (`HookExecutionError`)

## Merge hook behavior

Extension point: `merge`

Location: `finalizeWorktreeSession()` in `cmd/csw/cli.go`

Behavior:

- If merge hook exists and is enabled, hook executes instead of built-in merge algorithm.
- If hook succeeds (`exit code 0`), built-in merge is skipped.
- If hook fails (non-zero or execution error), finalize flow treats merge as failed:
  - emits merge failure message
  - keeps worktree and branch for manual investigation

If no merge hook is configured, existing built-in merge logic remains unchanged.

## Output and UI

Hook output is UI-agnostic by sending messages through `ui.IAppView.ShowMessage()`.

Displayed messages include:

- Hook command line
- Captured stdout
- Captured stderr

This works in CLI and any future app view implementation.

## Future extension points

The design keeps execution-type dispatch in `HookEngine.Execute()` to allow adding:

- `llm` hook handler (one-shot query)
- `subagent` hook handler (full delegated run)

without changing hook config shape or merge semantics.
