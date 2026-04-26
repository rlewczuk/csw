# Package `pkg/vcs` Overview

Package `pkg/vcs` provides git-backed and no-op VCS implementations for repository operations, worktree management, branching, merging, and diff utilities.

## Important files

* `git_vcs.go` - Git-backed repository and worktree operations.
* `git_merge.go` - Git merge helpers and diff utilities.
* `null_vcs.go` - No-op VCS implementation.
* `git_vcs_fixture_test.go` - Shared GitVCS test fixture helpers.

## Important public API objects

* `GitVCS` - Git-backed VCS implementation.
* `NullVCS` - No-op VCS implementation.
* `NewGitRepo` - Creates GitVCS from repository path.
* `NewNullVFS` - Creates NullVCS instance.
* `RunGitCommand` - Variable pointing to git command runner.
* `ListGitConflictFiles` - Variable pointing to conflict file lister.
* `GitLookPath` - Variable wrapping exec LookPath for git.
* `GitConfigValue` - Variable wrapping ReadGitConfigValue.
* `ReadGitConfigValue` - Reads a single git configuration key.
* `GitWorktreeForBranch` - Finds worktree name for a branch.
* `HardResetWorktree` - Resets and cleans a worktree.
* `DetectMergeBaseBranch` - Resolves the currently checked out branch.
* `GitBranchExists` - Checks if a branch exists.
* `CreateMergeWorktree` - Creates a temporary merge worktree.
* `ExtractConflictFilesFromOutput` - Extracts conflict file paths from output.
* `IsMergeConflictError` - Detects merge conflict output.
* `ResolveGitCommitID` - Resolves a revision to a commit hash.
* `ResolveHostGitConfigValue` - Reads host git config value.
* `ResolveGitIdentity` - Resolves configured git identity.
* `ChooseGitDiffDir` - Chooses the directory for git diff.
* `GitDiffNameOnly` - Returns changed file names for a range.
* `GitUntrackedFiles` - Returns untracked file paths.
* `ParseGitFileList` - Parses git command output into file list.
* `CollectEditedFiles` - Collects edited files across worktrees.
