Returns LLM-readable task information for a single task.

Task can be resolved by `uuid`, `name`, or current session task context.

Use `summary=true` to include latest session summary text and summary metadata.

Use `promptOnly=true` or `promptOnly="yes"` to return only task prompt (`task.md`) content in vfsRead-like `content` format (with line numbers).
