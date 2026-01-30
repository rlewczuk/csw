todos:
  type: array
  description: Array of todo items to set as the complete todo list.
  required: true
  items:
    type: object
    properties:
      id:
        type: string
        description: Unique identifier for the todo item (UUIDv7 string).
        required: true
      content:
        type: string
        description: Brief description of the task.
        required: true
      status:
        type: string
        description: Status of the task.
        required: true
        enum: [pending, in_progress, completed, cancelled]
      priority:
        type: string
        description: Priority level of the task.
        required: true
        enum: [low, medium, high]
---

## `todo.write` tool

Updates the todo list with the provided array of todo items. This replaces the entire list.

Usage:
- The todos parameter must be an array of todo item objects
- Each todo item must have: id, content, status, and priority fields
- Status must be one of: pending, in_progress, completed, cancelled
- Priority must be one of: low, medium, high
- Use this tool to track progress on multi-step tasks
- Mark tasks as in_progress when starting, and completed when done
