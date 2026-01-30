from:
  type: string
  description: The absolute path to the source file or directory.
  required: true
to:
  type: string
  description: The absolute path to the destination.
  required: true
---

## `vfs.move` tool

Moves or renames a file or directory from one path to another.

Usage:
- Both from and to parameters must be absolute paths
- Can be used to rename files/directories or move them to different locations
- If the destination already exists, it may be overwritten depending on permissions
- Parent directory of the destination must exist
