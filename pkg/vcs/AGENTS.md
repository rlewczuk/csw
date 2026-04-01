# Package `pkg/vcs` Overview

Package `pkg/vcs` contains VCS interface implementations for git repositories and null/no-op version control.

## Important files

* `git_vcs.go` - GitVCS implementation with worktree management
* `git_merge.go` - Git merge utilities and helper functions
* `null_vcs.go` - NullVCS no-op implementation for non-git directories

## Important public API objects

### Structs

* `GitVCS` - Git repository with worktree management
* `NullVCS` - No-op VCS for plain directories

### Constructor Functions

* `NewGitRepo(path, worktreesPath, hidePatterns, allowedPaths, name, email)` - Creates GitVCS instance
* `NewNullVFS(vfs)` - Creates NullVCS instance

### Helper Functions

* `ReadGitConfigValue(key)` - Reads git config value
* `GitWorktreeForBranch(repoDir, branch)` - Finds worktree for branch
* `HardResetWorktree(workDir)` - Hard resets worktree to HEAD
* `DetectMergeBaseBranch(repoDir)` - Detects current branch name
* `GitBranchExists(repoDir, branch)` - Checks if branch exists
* `CreateMergeWorktree(repoDir, branch)` - Creates temporary merge worktree
* `ExtractConflictFilesFromOutput(output)` - Parses conflict files from git output
* `IsMergeConflictError(output)` - Detects merge conflict in output
* `ResolveGitCommitID(workDir, rev)` - Resolves revision to commit ID
* `ResolveHostGitConfigValue(key)` - Reads host git config
* `ResolveGitIdentity(value, gitConfigKey)` - Resolves git identity with fallback
* `CollectEditedFiles(workDirRoot, workDir, baseCommitID, headCommitID)` - Collects changed files
