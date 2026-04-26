# Package `pkg/vfs` Overview

Package `pkg/vfs` provides filesystem abstractions, local and in-memory VFS implementations, access-control wrappers, glob/grep filtering, patch parsing, text patching, and shadow routing for the project.

## Important files

* `access.go` - Access-control VFS wrapper.
* `access_pattern_test.go` - Access pattern matching and specificity tests.
* `config.go` - Hide-pattern configuration helpers.
* `glob.go` - Glob matching and filtering.
* `grep.go` - Regex file content search.
* `local.go` - Local filesystem VFS implementation.
* `mock.go` - In-memory VFS test double.
* `mock_vcs.go` - In-memory VCS test double.
* `patch.go` - Patch parsing for file operations.
* `patcher.go` - Text patching and unified diff.
* `shadow.go` - Shadow filesystem routing wrapper.
* `vfs_findfiles_test.go` - FindFiles and glob-pattern behavior tests.
* `vfs_movefile_test.go` - MoveFile behavior and edge-case tests for LocalVFS and MockVFS.

## Important public API objects

* `PermissionError` - Permission-required operation error.
* `AccessControlVFS` - VFS wrapper enforcing access rules.
* `GlobFilter` - Interface for glob path matching.
* `GrepMatch` - Path and matching line numbers.
* `GrepFilter` - Interface for regex file search.
* `LocalVFS` - Local filesystem VFS implementation.
* `MockVFS` - In-memory VFS test double.
* `MockVCSCommitCall` - Recorded CommitWorktree invocation.
* `MockVCSMergeCall` - Recorded MergeBranches invocation.
* `MockVCS` - In-memory VCS test double.
* `Patch` - Parsed patch containing hunks.
* `Hunk` - Interface for patch operations.
* `AddFile` - Patch add-file operation.
* `DeleteFile` - Patch delete-file operation.
* `UpdateFile` - Patch update-file operation.
* `UpdateFileChunk` - Single update-file chunk.
* `FilePatcher` - Interface for file text edits.
* `ShadowVFS` - Routes selected paths to shadow VFS.
* `ErrOldStringNotFound` - Error when old text missing.
* `ErrOldStringMultipleMatch` - Error when old text ambiguous.
* `NewAccessControlVFS` - Creates access-controlled VFS wrapper.
* `BuildHidePatterns` - Builds hide patterns from ignore files.
* `DefaultShadowPatterns` - Returns default shadowed path globs.
* `NewGlobFilter` - Creates glob filter instance.
* `NewGrepFilter` - Creates grep filter instance.
* `NewLocalVFS` - Creates LocalVFS from root path.
* `NewMockVFS` - Creates empty in-memory VFS.
* `NewMockVFSFromDir` - Creates MockVFS from directory contents.
* `NewMockVCS` - Creates in-memory VCS double.
* `ParsePatch` - Parses patch text into operations.
* `NewFilePatcher` - Creates file patcher instance.
* `NewShadowVFS` - Creates shadow-routing VFS wrapper.

# Additional sections

## Major files (legacy)

- `local.go`: Local filesystem `VFS` implementation with root-path enforcement and hide-pattern aware traversal.
- `access.go`: Permission-enforcing VFS wrapper with glob-based rule matching and structured permission errors.
- `access_pattern_test.go`: Focused tests for glob matching specificity and access pattern resolution.
- `glob.go`: Glob-rule parser/filter used for hide and include/exclude path matching.
- `grep.go`: Regex content search over VFS files with optional glob filtering.
- `patcher.go`: Targeted text patching helper with unified diff output and ambiguity handling.
- `patch.go`: Patch parser for Add/Delete/Update file operations with hunk/chunk structure.
- `config.go`: Hide-pattern assembly from role config and ignore files (`.cswignore`/`.gitignore`).
- `shadow.go`: Shadow VFS wrapper that routes configured paths to alternate filesystem.
- `mock.go`: In-memory `VFS`/repo test doubles for deterministic tests.
- `mock_vcs.go`: In-memory `VCS` test double for deterministic tests.
- `vfs_movefile_test.go`: Focused MoveFile scenarios split from the general VFS test suite.
