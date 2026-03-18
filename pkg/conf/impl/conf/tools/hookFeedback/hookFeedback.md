
Sends a hook feedback message from a subagent session started by a hook.

This tool accepts the same payload fields as `CSWFEEDBACK: { ... }` JSON objects used in external hook scripts:
- `fn` (required) - action function to execute
- `args` (optional) - argument map specific to selected `fn`
- `id` (optional) - request id copied to returned result
- `response` (optional) - accepted for compatibility but ignored by this tool

Supported `fn` values are equivalent to external script hook feedback handling and currently include:
- `context` - merge values from `args` into hook session context
- `llm` - run one-shot LLM query with fields from `args`:
  - `prompt` (required)
  - `system-prompt` (optional)
  - `model` (optional, format `provider/model`, defaults to session model)
  - `thinking` (optional, defaults to session thinking level)
- `response` - stores explicit response payload used by hook engine output mapping

Notes:
- This tool is available only in hook-started subagent sessions.
- `response` delivery mode used by external scripts is ignored for this tool.
