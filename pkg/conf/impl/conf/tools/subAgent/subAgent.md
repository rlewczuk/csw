
Starts a focused subagent task in a separate child session and waits for it to finish.

Use this tool when a task is large enough to delegate independently, while keeping parent context clean.

The child session:
- reuses parent VFS/workdir/LSP,
- has its own session state and todo list,
- runs synchronously (parent waits until it finishes),
- returns only the child agent summary output.

## Parameters

- `slug` (required): very short symbolic subagent session name used in UI/log output.
  - Generate slug from task title the same way as worktree branch symbolic names:
    - lowercase letters, digits, dashes only
    - max 20 characters
    - 2-4 concise words joined with dashes
    - no spaces, slashes, underscores, or punctuation
  - Must be unique among subagent calls in the current parent session.
- `title` (required): short user-facing status title shown while subagent runs.
- `prompt` (required): full initial prompt for the child session.
- `role` (optional): child role override; defaults to parent role.
- `model` (optional): child model override in `provider/model` format; defaults to parent model.
- `thinking` (optional): child thinking override; defaults to parent thinking.

## Output

Returns child session summary result:
- `status`: `completed` or `error`
- `summary`: summary text from child model when completed
- `final_todo_list`: child final todo list (if any)

On failure, returns diagnostic error details instead of summary.
