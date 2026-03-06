# JetBrains AI provider integration

CSW includes a dedicated `jetbrains` provider type for JetBrains AI private API.

## Protocol and endpoint

- Protocol: **JetBrains private SSE stream** (not OpenAI Chat Completions).
- Chat endpoint used by CSW:
  - `POST /user/v5/llm/chat/stream/v8`
- Request/response payload format in this integration is aligned with the **Responses-style event schema** used in the codebase.

Model listing uses `GET /models` from your configured base URL, which is useful when you run a local proxy exposing that endpoint.

## Required auth values

From observed JetBrains IDE traffic, these values are needed:

- `grazie-authenticate-jwt` (JWT token)
- `jb-access-token` (Bearer/session token)

In CSW config:

- `api_key` → used as `grazie-authenticate-jwt`
- `refresh_token` → used as `jb-access-token` and `Authorization: Bearer ...`

## How to obtain tokens

Use your own JetBrains account/session and inspect AI chat traffic (for example with `mitmproxy`), then extract headers from requests to JetBrains AI endpoints.

Common flow:

1. Open JetBrains IDE with AI Assistant enabled.
2. Trigger an AI chat action.
3. Inspect request headers for the AI chat stream request.
4. Copy:
   - `grazie-authenticate-jwt`
   - `jb-access-token` (or equivalent bearer token)

> Tokens are sensitive and may expire. Store them securely and rotate as needed.

## Sample provider config

`~/.config/csw/models/jetbrains.yml` (or project `.csw/config/models/jetbrains.yml`):

```yaml
name: jetbrains
type: jetbrains
description: JetBrains AI private API
url: https://api.jetbrains.ai

# JWT from `grazie-authenticate-jwt` header
api_key: "YOUR_GRAZIE_AUTHENTICATE_JWT"

# Bearer/session token from `jb-access-token` header
refresh_token: "YOUR_JB_ACCESS_TOKEN"

# Optional request tuning
request_timeout: 120s
connect_timeout: 20s
max_tokens: 8192

# Optional extra headers if needed in your environment
headers:
  originator: csw
```

## CLI setup example

```bash
csw conf provider add jetbrains \
  --type jetbrains \
  --url https://api.jetbrains.ai \
  --api-key "YOUR_GRAZIE_AUTHENTICATE_JWT" \
  --description "JetBrains AI private API"
```

Then set `refresh_token` in the provider config file (or via JSON-based add/edit flow).

## Notes

- This is an integration with a private/undocumented API surface; behavior may change.
- Without real credentials at development time, runtime validation against live API is not performed in this repository.
