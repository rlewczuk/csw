# Package `pkg/system` Overview

`pkg/system` provides core system orchestration for building, running, and managing agent sessions. It wires together configuration, models, tools, VFS/VCS, hooks, container runtime, and worktree handling to create a fully initialized `SweSystem` and drive session lifecycle.

## Important files

* `bootstrap.go` - build system and configuration wiring
* `bootstrap_container.go` - container runtime resolution and identity helpers for bootstrap
* `context.go` - run context parsing helpers
* `hooks.go` - hook parsing and runtime config
* `runtime.go` - run session startup flow
* `system.go` - SweSystem lifecycle and sessions
* `worktree.go` - worktree finalize and resume helpers

## Important public API objects

* `SweSystem` - Main runtime system object
* `SessionLoggerFactory` - Creates per-session loggers
* `BuildSystemParams` - Inputs for BuildSystem
* `BuildSystemResult` - Outputs of BuildSystem
* `ResolveRunDefaultsParams` - Inputs for run defaults
* `ResolveWorktreeBranchNameParams` - Inputs for branch resolver
* `ContainerUserIdentity` - Host user identity for containers
* `RuntimeHookConfigStore` - Hook-overriding config store
* `HookOverride` - Parsed hook override entry
* `StartRunSessionParams` - Inputs for run session startup
* `StartRunSessionResult` - Outputs from run session startup
* `WorktreeFinalizeResult` - Worktree finalization output
* `ResumeUUIDPattern` - UUID regex for resume targets
* `ResumeWorktreeNamePattern` - Worktree name regex for resume
* `NewRuntimeConfigStore()` - Creates runtime hook config store
* `BuildSystem()` - Builds configured SweSystem
* `PrepareSessionVFS()` - Creates session VCS and VFS
* `ResolveRunDefaults()` - Resolves run command defaults
* `ResolveWorktreeBranchName()` - Resolves dynamic worktree branch
* `ResolveContainerRuntimeConfig()` - Resolves container runtime settings
* `ParseContainerMountSpec()` - Parses container mount entry
* `ParseContainerEnvSpec()` - Parses container env entry
* `ResolveContainerGitAuthorIdentity()` - Resolves git identity in container
* `BuildConfigPath()` - Builds effective config path
* `ValidateConfigPaths()` - Validates config directories
* `ResolveWorkDir()` - Resolves absolute working directory
* `ResolveModelName()` - Resolves effective model name
* `ResolveModelSpec()` - Resolves model alias or spec
* `CreateProviderMap()` - Builds provider lookup map
* `CreateModelTagRegistry()` - Builds model tag registry
* `ParseRunContextEntries()` - Parses --context entries
* `ParseRunContextFromEntries()` - Parses --context-from entries
* `HandleCommitHookResponse()` - Processes commit hook result
* `ApplyHookDefaults()` - Applies default hook values
* `BuildRuntimeHookConfigStore()` - Builds overridden hook config store
* `ApplyHookOverridesToConfigs()` - Applies hook overrides to configs
* `ParseHookOverride()` - Parses hook override string
* `BuildNewHookConfig()` - Builds new hook config
* `ApplyHookSettings()` - Applies settings to hook config
* `ParseHookTimeout()` - Parses hook timeout value
* `NormalizeResumeTarget()` - Normalizes resume target value
* `StartRunSession()` - Starts run session runtime
* `LoadSession()` - Loads persisted session
* `LoadLastSession()` - Loads latest persisted session
* `NewSession()` - Creates new runtime session
* `ExecuteSubAgentTask()` - Runs delegated subagent task
* `GetSession()` - Gets session by id
* `GetSessionThread()` - Gets session thread by id
* `ListSessions()` - Lists active sessions
* `DeleteSession()` - Deletes session by id
* `Shutdown()` - Stops sessions and closes resources
* `FinalizeWorktreeSession()` - Finalizes worktree and optional merge
* `FindSessionIDByWorkDirName()` - Finds session by work dir name
* `ResolveResumeTargetAsBranchOrWorktree()` - Resolves resume target as branch
* `FindSessionIDByWorkDirPath()` - Finds session by work dir path
* `ResolveResumeTargetToSessionID()` - Resolves resume target to session id
* `ResolveResumeTargetAsPath()` - Resolves resume target as path
