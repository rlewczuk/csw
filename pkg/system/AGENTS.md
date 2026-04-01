# Package `pkg/system` Overview

Package `pkg/system` contains the core system orchestration for managing sessions, models, tools, and CLI runtime initialization.

## Important files

* `system.go` - Core SweSystem struct and session management
* `bootstrap.go` - System bootstrapping, config resolution, and setup
* `runtime.go` - CLI session runtime initialization and wiring
* `hooks.go` - Hook handling and runtime hook config store
* `worktree.go` - Worktree session finalization and conflict resolution
* `summary.go` - Session summary building and persistence
* `context.go` - CLI context entry parsing utilities

## Important public API objects

* `SweSystem` - Core system managing sessions, tools, models, and providers
* `SessionLoggerFactory` - Function type for creating session loggers
* `BuildSystemParams` - Parameters for constructing a SweSystem
* `BuildSystemResult` - Outputs from building a SweSystem
* `ResolveCLIDefaultsParams` - Parameters for resolving CLI defaults
* `ResolveWorktreeBranchNameParams` - Parameters for resolving worktree branch names
* `StartCLISessionParams` - Parameters for creating and starting CLI session runtime
* `StartCLISessionResult` - Initialized CLI runtime components
* `SessionLoggerAppView` - App view supporting session logger binding
* `ChatPresenter` - Runtime presenter contract for system wiring
* `ChatView` - Runtime chat view contract for system wiring
* `AppViewFactory` - Builds app view for CLI runtime
* `ChatPresenterFactory` - Builds presenter for session thread
* `ChatViewFactory` - Builds chat view bound to presenter
* `ContainerUserIdentity` - Host user identity mirrored in container mode
* `RuntimeHookConfigStore` - Config store with runtime hook overrides
* `HookOverride` - Parsed hook override with name, disable flag, settings
* `WorktreeFinalizeResult` - Result from finalizing worktree session
* `LoadSession` - Loads a persisted session from disk
* `LoadLastSession` - Loads the most recently updated persisted session
* `NewSession` - Creates a new session for selected model
* `ExecuteSubAgentTask` - Executes delegated child-session task synchronously
* `GetSession` - Returns the session with the given ID
* `GetSessionThread` - Returns the SessionThread for the given session ID
* `ListSessions` - Returns a list of all active sessions
* `DeleteSession` - Deletes the session with the given ID
* `Shutdown` - Interrupts all running sessions and cleans up
* `StartCLISession` - Creates app view, thread, presenter and starts flow
* `ResolveCLIDefaults` - Resolves CLI defaults from effective global config
* `ResolveWorktreeBranchName` - Resolves a worktree branch placeholder
* `PrepareSessionVFS` - Creates session VCS/VFS with worktree handling
* `BuildSystem` - Builds a SweSystem and related setup for CLI and TUI
* `ResolveContainerRuntimeConfig` - Resolves effective container runtime setup
* `ParseContainerMountSpec` - Parses mount in host_path:container_path format
* `ParseContainerEnvSpec` - Parses env var in KEY=VALUE format
* `ResolveContainerGitAuthorIdentity` - Returns git author identity for container mode
* `BuildConfigPath` - Builds a config path hierarchy string
* `ValidateConfigPaths` - Validates that all paths exist and are directories
* `ResolveWorkDir` - Resolves the working directory from an optional path
* `ResolveModelName` - Determines the model name to use
* `CreateProviderMap` - Creates a map of provider names to ModelProvider instances
* `CreateModelTagRegistry` - Creates and populates a model tag registry
* `HandleCommitHookResponse` - Handles commit hook execution and response
* `ApplyHookDefaults` - Applies default values to hook configuration
* `BuildRuntimeHookConfigStore` - Builds config store with hook overrides
* `ParseHookOverride` - Parses hook override string into struct
* `ParseHookTimeout` - Parses hook timeout duration string
* `FinalizeWorktreeSession` - Finalizes worktree with commit and merge
* `NormalizeResumeTarget` - Normalizes resume target to session ID or "last"
* `ResolveResumeTargetToSessionID` - Resolves resume target to session ID
* `BuildSessionSummaryJSON` - Builds session summary JSON structure
* `SaveSessionSummaryJSON` - Saves session summary to JSON file
* `EmitSessionSummary` - Emits and saves session summary on completion
* `ParseCLIContextEntries` - Parses CLI context entries in KEY=VAL format
* `ParseCLIContextFromEntries` - Parses context-from entries from files
