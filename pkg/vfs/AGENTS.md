# Package `pkg/vfs` Overview

Package `pkg/vfs` provides filesystem and VCS abstractions used by tools and core runtime code. It includes interface definitions, local and git-backed implementations, access-control wrappers, search/filter utilities, patch/edit helpers, and in-memory mocks for testing.

## Important files

* `vfs.go` - VFS interface for file operations
* `vcs.go` - VCS interface for repository operations
* `local.go` - local filesystem VFS implementation
* `git_vcs.go` - Git-backed VCS implementation
* `null_vcs.go` - no-op VCS implementation
* `access.go` - permission-enforcing VFS wrapper
* `glob.go` - glob pattern matching and filtering
* `grep.go` - regex content search over VFS
* `patcher.go` - text patching with unified diff
* `config.go` - hide pattern configuration
* `shadow.go` - shadow filesystem routing
* `mock.go` - in-memory VFS/VCS test doubles

## Important public API objects

* `VFS` - virtual filesystem interface
* `VCS` - version control system interface
* `LocalVFS` - local filesystem implementation
* `NewLocalVFS` - creates local VFS instance
* `GitVCS` - Git repository implementation
* `NewGitRepo` - creates Git VCS instance
* `NullVCS` - no-op VCS implementation
* `NewNullVFS` - creates null VCS instance
* `AccessControlVFS` - permission wrapper for VFS
* `NewAccessControlVFS` - creates access-controlled VFS
* `PermissionError` - permission error with context
* `GlobFilter` - glob pattern filter interface
* `NewGlobFilter` - creates glob filter instance
* `GrepFilter` - regex search interface
* `NewGrepFilter` - creates grep filter instance
* `GrepMatch` - file match with line numbers
* `FilePatcher` - file patching interface
* `NewFilePatcher` - creates file patcher instance
* `ShadowVFS` - routes paths to shadow filesystem
* `NewShadowVFS` - creates shadow VFS wrapper
* `BuildHidePatterns` - builds hide patterns from config
* `DefaultShadowPatterns` - returns default shadow patterns
* `MockVFS` - in-memory VFS for testing
* `NewMockVFS` - creates mock VFS instance
* `NewMockVFSFromDir` - creates mock VFS from directory
* `MockVCS` - in-memory VCS for testing
* `NewMockVCS` - creates mock VCS instance

# Additional sections

## Major files (legacy)

- `vfs.go`: Core public VFS API (`VFS` interface) for file operations, listing/search, and repo metadata access.
- `vcs.go`: Core VCS API (`VCS` interface) for worktree/branch/merge/commit lifecycle operations.
- `local.go`: Local filesystem `VFS` implementation with root-path enforcement and hide-pattern aware traversal.
- `git_vcs.go`: Git-backed `VCS` implementation for worktree management, branch operations, commit, and merge flows.
- `access.go`: Permission-enforcing VFS wrapper with glob-based rule matching and structured permission errors.
- `glob.go`: Glob-rule parser/filter used for hide and include/exclude path matching.
- `grep.go`: Regex content search over VFS files with optional glob filtering.
- `patcher.go`: Targeted text patching helper with unified diff output and ambiguity handling.
- `config.go`: Hide-pattern assembly from role config and ignore files (`.cswignore`/`.gitignore`).
- `mock.go`: In-memory `VFS`/`VCS`/repo test doubles for deterministic tests.
