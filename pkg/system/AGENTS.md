# Package `pkg/system` Overview

Package `pkg/system` contains the core system orchestration for managing sessions, models, tools, and CLI runtime initialization.

## Important files

* `system.go` - Core SweSystem struct and session management
* `runtime.go` - CLI session runtime initialization and wiring
* `bootstrap.go` - System bootstrapping, config resolution, and setup

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
* `ReadGitConfigValue` - Reads a single git configuration key from host git config
* `BuildConfigPath` - Builds a config path hierarchy string
* `ValidateConfigPaths` - Validates that all paths exist and are directories
* `ResolveWorkDir` - Resolves the working directory from an optional path
* `ResolveModelName` - Determines the model name to use
* `CreateProviderMap` - Creates a map of provider names to ModelProvider instances
* `CreateModelTagRegistry` - Creates and populates a model tag registry
