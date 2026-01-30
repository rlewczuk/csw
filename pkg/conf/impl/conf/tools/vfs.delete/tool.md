## `vfs.delete` tool

Deletes a file or directory at the specified path.

Usage:
- The path parameter must be an absolute path, not a relative path
- For directories, all contents will be recursively deleted
- This operation cannot be undone
- Returns an error if the path doesn't exist
