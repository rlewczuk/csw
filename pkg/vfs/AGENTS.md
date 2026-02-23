# pkg/vfs

`pkg/vfs` provides filesystem and VCS abstractions used by tools and core runtime code. It includes interface definitions, local and git-backed implementations, access-control wrappers, search/filter utilities, patch/edit helpers, and in-memory mocks for testing.

## Major files

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
