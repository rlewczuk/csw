# Package `pkg/apis` Overview

Package `pkg/apis` defines VFS and VCS abstractions.

## Important files

* `vfs.go` - VFS interface and common errors
* `vcs.go` - VCS repository interface

## Important public API objects

* `VFS` - Virtual filesystem interface
* `VCS` - Version control interface
* `ErrFileNotFound` - File not found
* `ErrFileExists` - File already exists
* `ErrNotADir` - Not a directory
* `ErrNotAFile` - Not a file
* `ErrPermissionDenied` - Permission denied
* `ErrNotImplemented` - Not implemented
* `ErrInvalidPath` - Invalid path
* `ErrAskPermission` - Ask permission
* `ErrNoChangesToCommit` - No changes to commit
* `ErrMergeConflict` - Merge conflict
