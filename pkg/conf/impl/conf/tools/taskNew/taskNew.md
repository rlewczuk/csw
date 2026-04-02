Creates a persistent hierarchical task state entry.

Use this tool when you need to create a new execution task that can be reviewed, updated, and run later.

Behavior:
- Generates a task UUID (v7) and initializes task metadata under `.csw/tasks`.
- Stores provided prompt in task `task.md`.
- Optionally creates task as a subtask when `parent` is provided.
- Optionally runs task immediately when `run=true`.

Returns:
- Created task UUID and full task metadata.
- If `run=true`, includes task run outcome with session ID and summary.
