# Model providers

This document describes provider configuration, authentication, and provider-related commands.

## Configuration files

Provider configs are loaded from:

```text
<config-root>/models/<provider>.json|yml|yaml
```

Typical roots are:

- `~/.config/csw`
- `./.csw/config`
- any extra paths from `--config-path`

## Provider config schema reference

Main fields:

- Identity and endpoint:
  - `type` (for example `openai`, `ollama`, `anthropic`, `responses`, `jetbrains`)
  - `name`
  - `description`
  - `url`
- Authentication:
  - `auth_mode`: `none` | `api_key` | `oauth2`
  - `api_key`
  - OAuth fields: `auth_url`, `token_url`, `client_id`, `client_secret`, `refresh_token`
- Request/network:
  - `connect_timeout`, `request_timeout`
  - `headers`, `query_params`
  - `max_retries`, `rate_limit_backoff_scale`
- Generation defaults/capabilities:
  - `default_temperature`, `default_top_p`, `default_top_k`
  - `context_length_limit`, `max_tokens`, `max_input_tokens`, `max_output_tokens`
  - `streaming`, `reasoning`, `reasoning_content`, `tool_call`, `temperature`, `interleaved`
  - `modalities`, `status`, `experimental`
- Model tagging/cost:
  - `model_tags`
  - `cost`
- Extra provider-specific options:
  - `options`

Notes:

- Duration fields are strings (for example `30s`, `120s`, `1m`).
- Provider configs are merged by name across config layers.

## Examples

### Local Ollama

```bash
csw provider add ollama-local \
  --type ollama \
  --url http://localhost:11434 \
  --description "Local Ollama"

csw provider set-default ollama-local
```

### OpenAI-like provider with OAuth

```bash
cat <<'JSON' | csw provider add openai-codex --json
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

csw provider auth openai-codex
```

OAuth callback endpoint used by CLI:

- `http://localhost:1455/auth/callback`

### JetBrains provider

See dedicated guide:

- [docs/JETBRAINS.md](JETBRAINS.md)

## Command reference

`provider` is a top-level command.

### `csw provider list`

List providers from composite configuration.

### `csw provider show <provider-name>`

Show one provider from composite configuration.

### `csw provider add <provider-name> [flags]`

Add or update provider.

Key flags:

- `--type`
- `--url`
- `--family`
- `--vendor`
- `--description`
- `--api-key`
- `--auth`
- `--header key=value` (repeatable)
- `--json` (read config from stdin JSON)

### `csw provider remove <provider-name>`

Remove provider from selected writable config scope.

### `csw provider set-default <provider-name>`

Set `default_provider` in global config for selected scope.

### `csw provider test <provider-name> <model-name>`

Run a provider/model connectivity test chat.

Flags:

- `--streaming`
- `--verbose`

### `csw provider auth <provider-name>`

Run browser OAuth flow for provider.

### `csw provider models [<provider>]`

List models for one provider or all providers.

Flags:

- `--verbose`
- `--json`

## Scope flags for write commands

Provider command supports persistent scope/output flags:

- `--json`
- `--local`
- `--global`
- `--to <config-dir>`
