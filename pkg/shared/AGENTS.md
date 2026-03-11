# Package `pkg/shared` Overview

Package `pkg/shared` contains cross-cutting utility code reused across packages, including patch parsing, file copy helpers, and UUIDv7 generation.

## Important files

* `fileutil.go` - Recursive directory and file copy utilities
* `patch.go` - Custom patch format parser for file operations
* `uuid.go` - UUIDv7 generator for time-sortable unique IDs

## Important public API objects

* `CopyDir` - Recursively copies a directory preserving permissions
* `CopyFile` - Copies a file preserving permissions
* `ParsePatch` - Parses custom patch format into structured operations
* `Patch` - Container for parsed patch hunks
* `Hunk` - Interface for file operation types (AddFile, DeleteFile, UpdateFile)
* `AddFile` - File creation operation with path and contents
* `DeleteFile` - File deletion operation with path
* `UpdateFile` - File modification operation with chunks
* `UpdateFileChunk` - Single change block within an update operation
* `GenerateUUIDv7` - Generates time-sortable UUIDv7 string
