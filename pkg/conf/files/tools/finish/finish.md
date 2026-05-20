Use this tool to finish current session loop normally.

When called, the session stops processing further LLM turns after current tool handling,
and finalization flow runs as in normal completion (same behavior as when assistant
returns message without tool calls).

Usage:
- You must provide the `summary` field.
- `summary` must contain the LLM-generated final session summary: describe what was done in this session, including completed work, important changes, validation performed, and any notable remaining information.
- The provided summary is displayed on the console with session information and persisted to `summary.json`.
