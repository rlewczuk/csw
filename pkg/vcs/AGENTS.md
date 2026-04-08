# Package `pkg/vcs` Overview

Package `pkg/vcs` implements git and null VCS behavior in package `pkg/vcs`.

## Important files

* `git_vcs.go` - GitVCS repository and worktree operations.
* `git_merge.go` - Git merge and diff helper functions.
* `null_vcs.go` - NullVCS no-op implementation.

## Important public API objects

* `GitVCS` - Git-backed VCS implementation.
* `NullVCS` - No-op VCS implementation.
* `NewGitRepo` - Creates GitVCS from repository path.
* `NewNullVFS` - Creates NullVCS wrapper.
* `ReadGitConfigValue` - Reads git config key value.
* `GitWorktreeForBranch` - Finds worktree name for branch.
* `HardResetWorktree` - Resets and cleans worktree.
* `DetectMergeBaseBranch` - Detects current branch name.
* `GitBranchExists` - Checks branch existence.
* `CreateMergeWorktree` - Creates temporary merge worktree.
* `ExtractConflictFilesFromOutput` - Extracts conflict file paths.
* `IsMergeConflictError` - Detects merge conflict output.
* `ResolveGitCommitID` - Resolves revision to commit hash.
* `ResolveHostGitConfigValue` - Reads host git config value.
* `ResolveGitIdentity` - Resolves configured git identity.
* `CollectEditedFiles` - Collects edited file paths.
