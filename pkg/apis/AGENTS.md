# Package `pkg/apis` Overview

Package `pkg/apis` contains core abstraction interfaces for virtual filesystem (VFS) and version control system (VCS) operations used throughout the codebase.

## Important files

* `vfs.go` - VFS interface and filesystem error definitions
* `vcs.go` - VCS interface for repository operations

## Important public API objects

### Interfaces

* `VFS` - Virtual filesystem abstraction for file operations
* `VCS` - Version control system abstraction for repository operations

### Errors

* `ErrFileNotFound` - File not found error
* `ErrFileExists` - File already exists error
* `ErrNotADir` - Not a directory error
* `ErrNotAFile` - Not a file error
* `ErrPermissionDenied` - Permission denied error
* `ErrNotImplemented` - Not implemented error
* `ErrInvalidPath` - Invalid path error
* `ErrAskPermission` - Ask permission error
* `ErrNoChangesToCommit` - No changes to commit error
* `ErrMergeConflict` - Merge conflict error
