# Package `pkg/vfs` Overview

Package `pkg/vfs` provides filesystem and VCS abstractions used by tools and core runtime code. It includes interface definitions, local and git-backed implementations, access-control wrappers, search/filter utilities, patch/edit helpers, and in-memory mocks for testing.

## Important files

* `local.go` - local filesystem VFS implementation
* `access.go` - permission-enforcing VFS wrapper
* `glob.go` - glob pattern matching and filtering
* `grep.go` - regex content search over VFS
* `patcher.go` - text patching with unified diff
* `patch.go` - patch parsing for file operations
* `config.go` - hide pattern configuration
* `shadow.go` - shadow filesystem routing
* `mock.go` - in-memory VFS/VCS test doubles

## Important public API objects

### Types

* `LocalVFS` - local filesystem implementation
* `AccessControlVFS` - permission wrapper for VFS
* `PermissionError` - permission error with context
* `GlobFilter` - glob pattern filter interface
* `GrepFilter` - regex search interface
* `GrepMatch` - file match with line numbers
* `FilePatcher` - file patching interface
* `ShadowVFS` - routes paths to shadow filesystem
* `MockVFS` - in-memory VFS for testing
* `MockVCS` - in-memory VCS for testing
* `MockVCSCommitCall` - records commit invocation
* `MockVCSMergeCall` - records merge invocation
* `Patch` - parsed patch with file operations
* `Hunk` - single file operation interface
* `AddFile` - file creation operation
* `DeleteFile` - file deletion operation
* `UpdateFile` - file modification operation
* `UpdateFileChunk` - change block within update

### Constructors

* `NewLocalVFS` - creates local VFS instance
* `NewAccessControlVFS` - creates access-controlled VFS
* `NewGlobFilter` - creates glob filter instance
* `NewGrepFilter` - creates grep filter instance
* `NewFilePatcher` - creates file patcher instance
* `NewShadowVFS` - creates shadow VFS wrapper
* `NewMockVFS` - creates mock VFS instance
* `NewMockVFSFromDir` - creates mock VFS from directory
* `NewMockVCS` - creates mock VCS instance

### Functions

* `BuildHidePatterns` - builds hide patterns from config
* `DefaultShadowPatterns` - returns default shadow patterns
* `ParsePatch` - parses patch string into Patch struct
* `matchGlob` - matches path against glob pattern

### Errors

* `ErrOldStringNotFound` - old string not found in content
* `ErrOldStringMultipleMatch` - ambiguous old string match

# Additional sections

## Major files (legacy)

- `local.go`: Local filesystem `VFS` implementation with root-path enforcement and hide-pattern aware traversal.
- `access.go`: Permission-enforcing VFS wrapper with glob-based rule matching and structured permission errors.
- `glob.go`: Glob-rule parser/filter used for hide and include/exclude path matching.
- `grep.go`: Regex content search over VFS files with optional glob filtering.
- `patcher.go`: Targeted text patching helper with unified diff output and ambiguity handling.
- `patch.go`: Patch parser for Add/Delete/Update file operations with hunk/chunk structure.
- `config.go`: Hide-pattern assembly from role config and ignore files (`.cswignore`/`.gitignore`).
- `shadow.go`: Shadow VFS wrapper that routes configured paths to alternate filesystem.
- `mock.go`: In-memory `VFS`/`VCS`/repo test doubles for deterministic tests.
