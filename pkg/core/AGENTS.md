# pkg/core

`pkg/core` is the runtime orchestration layer for agent sessions. It manages session lifecycle, prompt/tool assembly, role/model switching, the run loop, tool-call execution flow, permission pauses, and async session threading used by UI layers.

## Major files

- `system.go`: Main system container (`SweSystem`) for session creation, lookup, listing, and shutdown.
- `session.go`: Core session engine (`SweSession`) that runs chat/tool loops, handles retries, permissions, and model/role changes.
- `session_thread.go`: Async thread wrapper around sessions for non-blocking UI interaction and interruption/pause/resume control.
- `prompt.go`: Prompt and tool-info generator that merges role fragments and builds runtime tool schema metadata.
- `role.go`: Role registry with cached config loading and role merge behavior.
- `commit_message.go`: Commit message generation pipeline using model-backed prompt templates.
