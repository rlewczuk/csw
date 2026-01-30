## `todo.write` tool

Updates the todo list with the provided array of todo items. This replaces the entire list.

Usage:
- The todos parameter must be an array of todo item objects
- Each todo item must have: id, content, status, and priority fields
- Status must be one of: pending, in_progress, completed, cancelled
- Priority must be one of: low, medium, high
- Use this tool to track progress on multi-step tasks
- Mark tasks as in_progress when starting, and completed when done
