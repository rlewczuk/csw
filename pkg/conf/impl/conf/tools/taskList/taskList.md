Lists task children (subtasks) for a selected task.

Resolution rules:
- uses `uuid` if provided,
- otherwise uses `name`,
- otherwise uses current session task,
- otherwise lists top-level tasks.

Set `recursive=true` to include nested subtask trees.
