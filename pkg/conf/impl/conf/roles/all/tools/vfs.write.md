path:
  type: string
  description: The absolute path to the file to write.
  required: true
content:
  type: string
  description: The content to write to the file.
  required: true
---

## `vfs.write` tool

Writes content to a file at the specified path. Creates the file if it doesn't exist, or overwrites it if it does.

Usage:
- The path parameter must be an absolute path, not a relative path
- The content parameter contains the complete file content to write
- Parent directories must exist before writing the file
- Be careful as this will overwrite existing files without warning
