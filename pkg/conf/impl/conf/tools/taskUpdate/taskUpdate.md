Updates existing persistent task metadata.

Task can be identified by:
- `uuid`, or
- `name`, or
- current session task when neither is provided.

Use this tool to modify task description, status, branches, role, dependencies, or task prompt before execution.

When `run=true`, task is executed immediately after successful update.
