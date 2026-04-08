# Package `pkg/system` Overview

Package `pkg/system` contains orchestration and runtime wiring for `pkg/system`.

## Important files

* `system.go` - SweSystem lifecycle and session registry
* `bootstrap.go` - Build system dependencies and config
* `runtime.go` - CLI runtime session startup flow
* `hooks.go` - Hook override parsing and runtime store
* `worktree.go` - Worktree finalize and resume helpers
* `context.go` - CLI context parsing helpers

## Important public API objects

* `SweSystem` - Main runtime system object
* `SessionLoggerFactory` - Creates per-session loggers
* `BuildSystemParams` - Inputs for BuildSystem
* `BuildSystemResult` - Outputs of BuildSystem
* `ResolveCLIDefaultsParams` - Inputs for CLI defaults
* `ResolveWorktreeBranchNameParams` - Inputs for branch resolver
* `StartCLISessionParams` - Inputs for CLI session startup
* `StartCLISessionResult` - CLI runtime startup outputs
* `SessionLoggerAppView` - App view with logger binding
* `ChatPresenter` - Runtime presenter contract
* `ChatView` - Runtime chat view contract
* `AppViewFactory` - App view constructor
* `ChatPresenterFactory` - Chat presenter constructor
* `ChatViewFactory` - Chat view constructor
* `RuntimeHookConfigStore` - Hook-overriding config store
* `HookOverride` - Parsed hook override entry
* `WorktreeFinalizeResult` - Worktree finalization output
* `LoadSession()` - Load persisted session
* `LoadLastSession()` - Load latest persisted session
* `NewSession()` - Create new runtime session
* `ExecuteSubAgentTask()` - Run delegated subagent task
* `GetSession()` - Get session by id
* `GetSessionThread()` - Get session thread by id
* `ListSessions()` - List active sessions
* `DeleteSession()` - Delete session by id
* `Shutdown()` - Stop sessions and close resources
* `StartCLISession()` - Start CLI session runtime
* `ResolveCLIDefaults()` - Load CLI default values
* `ResolveWorktreeBranchName()` - Resolve dynamic worktree branch
* `PrepareSessionVFS()` - Build session VCS and VFS
* `BuildSystem()` - Build configured SweSystem
* `ResolveContainerRuntimeConfig()` - Resolve container runtime settings
* `ParseContainerMountSpec()` - Parse container mount entry
* `ParseContainerEnvSpec()` - Parse container env entry
* `ResolveContainerGitAuthorIdentity()` - Resolve git identity in container
* `BuildConfigPath()` - Build effective config path
* `ValidateConfigPaths()` - Validate config directories
* `ResolveWorkDir()` - Resolve absolute working directory
* `ResolveModelName()` - Resolve effective model name
* `CreateProviderMap()` - Build provider lookup map
* `CreateModelTagRegistry()` - Build model tag registry
* `HandleCommitHookResponse()` - Process commit hook result
* `ApplyHookDefaults()` - Apply default hook values
* `BuildRuntimeHookConfigStore()` - Build overridden hook config store
* `ParseHookOverride()` - Parse hook override string
* `ParseHookTimeout()` - Parse hook timeout value
* `FinalizeWorktreeSession()` - Finalize worktree and optional merge
* `ResolveResumeTargetToSessionID()` - Resolve resume target to session id
* `BuildSessionSummaryJSON()` - Build session summary JSON
* `SaveSessionSummaryJSON()` - Save session summary JSON
* `EmitSessionSummary()` - Emit and save session summary
* `ParseCLIContextEntries()` - Parse --context entries
* `ParseCLIContextFromEntries()` - Parse --context-from entries
