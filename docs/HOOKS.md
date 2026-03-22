# Hooks configuration and usage

Hooks let you extend CSW lifecycle behavior with shell scripts, one-shot LLM calls, and subagent tasks.

## Where hooks are configured

Hook configs are loaded from:

```text
<config-root>/hooks/<hook-name>/
  <hook-name>.json|yml|yaml
  ...additional files (scripts/templates/etc.)
```

Examples:

```text
.csw/config/hooks/
  pre_run/
    pre_run.yml
    prepare_prompt.sh
  summary/
    summary.yml
  merge/
    merge.yml
    push_and_mr.sh
```

### Important naming rule

For local hooks, hook directory name, config filename base, and `name` in config should match. If they do not match, hook is auto-disabled with a warning.

## Hook lifecycle points used by CLI/runtime

Common hook names used in runtime:

- `pre_run` (before session run, prompt can be templated using updated context)
- `commit` (worktree commit finalization flow)
- `merge` (worktree merge finalization flow)
- `branch_name` (worktree branch suffix generation)

You can also define other names for your own flows if integrated by runtime code.

## Hook context data

Hooks render templates with cumulative session context (string map). Common keys:

- `user_prompt`
- `branch`
- `workdir`
- `rootdir`
- `status` (`none`, `running`, `success`, `failed`)
- `summary`
- `hook`
- `hook_dir`

For shell hooks, context is also exported as environment variables with `CSW_` prefix, for example:

- `user_prompt` -> `CSW_USER_PROMPT`
- `rootdir` -> `CSW_ROOTDIR`

## Hook types and examples

## 1) External script hooks (`type: shell`)

Shell hooks render `command` as `text/template` and execute it on selected runner.

Example config (`hooks/pre_run/pre_run.yml`):

```yaml
name: pre_run
hook: pre_run
enabled: true
type: shell
run-on: sandbox
timeout: 30s
command: |
  bash {{ .hook_dir }}/prepare_prompt.sh "{{ .user_prompt }}"
```

Example script (`hooks/pre_run/prepare_prompt.sh`):

```bash
#!/usr/bin/env bash
set -euo pipefail

prompt_input="${1:-}"

# Update hook context
printf 'CSWFEEDBACK: {"fn":"context","args":{"prepared_prompt":"[prepared] %s"}}\n' "$prompt_input"

# Ask LLM from script feedback
printf 'CSWFEEDBACK: {"fn":"llm","id":"draft1","args":{"prompt":"Create concise checklist for: %s"},"response":"stdin"}\n' "$prompt_input"

# Script can read replayed response when response=stdin
if [ ! -t 0 ]; then
  while IFS= read -r line; do
    # line is JSON feedback response
    :
  done
fi
```

Feedback replay modes for script feedback requests:

- `none` (default): no replay
- `stdin`: rerun command with feedback JSON lines piped to stdin
- `rerun`: rerun command once per response with `CSW_RESPONSE` env var

## 2) LLM hooks (`type: llm`)

LLM hooks render `prompt` (and optional `system_prompt`) from hook context and execute one-shot model request.

Example (`hooks/summary/summary.yml`):

```yaml
name: summary
hook: summary
enabled: true
type: llm
model: openai-codex/gpt-5-codex
thinking: medium
prompt: |
  Summarize session result for branch {{ .branch }}.
  User prompt:
  {{ .user_prompt }}
output_to: summary_llm
timeout: 45s
```

Where output is stored:

- Response text is written into hook context under `output_to` key.
- If `output_to` is omitted for `llm`, default is `result`.

## 3) Subagent hooks (`type: subagent`)

Subagent hooks render prompt templates and run delegated subagent execution.

Example (`hooks/merge/merge.yml`):

```yaml
name: merge
hook: merge
enabled: true
type: subagent
role: developer
prompt: |
  Validate merge readiness for branch {{ .branch }}.
  Root dir: {{ .rootdir }}
output_to: merge_summary
error_to: merge_error
timeout: 120s
```

Subagent result behavior:

- subagent summary is captured as hook stdout-like result
- stored in context under `output_to` (default `result` for subagent response mapping)
- status maps to `OK` / `ERROR` / `TIMEOUT`

### Returning data from subagent via feedback tool

Inside hook-started subagent session, use `hookFeedback` tool.

Example payloads:

- set context:
  - `fn=context`, `args={...}`
- run one-shot llm:
  - `fn=llm`, `args.prompt=...`
- override final response payload used by hook:
  - `fn=response`, `args.status/stdout/stderr/...`

This supports returning different data shapes through feedback `args` fields.

## Runtime control with `--hook`

`csw cli` accepts repeatable `--hook` overrides:

- enable existing hook:
  - `--hook commit`
- disable existing hook:
  - `--hook merge:disable`
- change settings:
  - `--hook summary:type=llm,prompt=...,model=provider/model,output_to=mykey`
- create new ephemeral hook (must include required fields):
  - `--hook summary:hook=summary,command=echo hi`

Supported override setting keys:

- `enabled`
- `hook`
- `name` (must match selector name)
- `type` (`shell|llm|subagent`)
- `command`
- `prompt`
- `system_prompt` / `system-prompt`
- `model`
- `thinking`
- `output_to` aliases (`output-to`, `to_field`, ...)
- `error_to` aliases (`error-to`)
- `timeout`
- `run-on` / `runon` (`host|sandbox`)

`--hook` overrides are runtime-only (ephemeral) and do not modify files.

## Hook configuration file reference (full)

Config object fields (`hooks/<name>/<name>.yml`):

- `enabled` (bool, default `true`)
- `hook` (extension point name)
- `name` (hook identifier)
- `type` (`shell` default, `llm`, `subagent`)
- `command` (for shell)
- `prompt` (for llm/subagent)
- `system_prompt` (optional for llm/subagent)
- `model` (optional provider/model override)
- `thinking` (optional thinking override)
- `role` (optional for subagent)
  - accepts canonical role name or any configured role alias
- `output_to` (context/output mapping key)
  - default for llm: `result`
- `error_to` (error field mapping key)
- `timeout` (duration; `0` means no timeout)
- `run-on` (`host` or `sandbox`, default `sandbox`; shell hooks)

### Merge behavior across config layers

- Hooks merge by hook key/name.
- For hook fields, higher-priority layer overrides only fields explicitly configured there.
- Defaults are then applied (for example `type=shell`, `run-on=sandbox`, llm `output_to=result`).

## Feedback mechanism reference (detailed)

Shell hooks can emit feedback commands via output lines:

```text
CSWFEEDBACK: { ...json... }
```

Feedback request JSON fields:

- `fn` (required):
  - `context`
  - `llm`
  - `response` (handled as synthetic/final response in hook engine)
- `args` (object, optional)
- `response` (optional replay mode): `none|stdin|rerun`
- `id` (optional correlation id)

### `fn=context`

- merges `args` key/values into hook context as strings.

### `fn=llm`

`args` fields:

- `prompt` (required)
- `system-prompt` (optional)
- `model` (optional, `provider/model`; defaults to session model)
- `thinking` (optional; defaults to session thinking)

Result payload shape:

- `model`
- `thinking`
- `text`

### `fn=response`

Used to provide or override final response payload consumed by runtime hook handlers.

Common `args` fields:

- `status` (`OK|ERROR|TIMEOUT|COMPLETED` style)
- `stdout`
- `stderr`
- custom mapped keys (`output_to` / `error_to`)

### Replay modes

- `none`: process feedback only, no command replay.
- `stdin`: rerun command with JSON responses on stdin.
- `rerun`: rerun command with `CSW_RESPONSE='<json-line>'` environment variable.

### Automatic synthetic response

If no `fn=response` request is emitted by shell hook, runtime synthesizes one from stdout/stderr/exit status.

### Response/error semantics

- Hook execution success is based on exit code (0 success, non-zero failure).
- Feedback requests are processed in parallel; completion order may differ.
