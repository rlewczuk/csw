Runs task execution session in isolated worktree mode.

Behavior:
- Ensures dependencies are completed.
- Creates/uses task feature branch from parent branch.
- Creates dedicated task branch for run and executes task prompt.
- Persists session summary under task directory.

Options:
- `reset=true`: rebuild task branches from parent state before run.
- `merge=true`: after successful run, merge task feature branch into parent branch.

Returns run result with session ID, summary text, task branch name, and updated task metadata.
