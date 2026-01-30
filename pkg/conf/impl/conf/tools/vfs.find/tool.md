## `vfs.find` tool

Searches for files matching a glob pattern within a directory tree.

Usage:
- The path parameter must be an absolute path to a directory
- The pattern parameter uses glob syntax (*, ?, [abc], etc.)
- Recursively searches all subdirectories
- Returns a list of absolute paths to matching files
- Examples: "*.go" finds all Go files, "test_*.py" finds all test Python files
