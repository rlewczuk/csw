path:
  type: string
  description: The absolute path to the directory to list.
  required: true
---

## `vfs.list` tool

Lists files and directories in the specified directory.

Usage:
- The path parameter must be an absolute path to a directory
- Returns a list of files and directories with their metadata (name, type, size, modification time)
- Does not recursively list subdirectories - only lists direct children
- Returns an error if the path doesn't exist or is not a directory
