# Package `pkg/core` Overview

Package `pkg/core` provides runtime orchestration for agent sessions and tasks. It manages the core session loop, async threading, prompt generation, role caching, lifecycle hooks, persistent tasks, state persistence, and session summarization.

## Important files

* `session.go` - Main session loop and tool execution
* `session_state.go` - Session state accessors, metadata, and bookkeeping helpers
* `session_runtime.go` - Retry, compaction, and runtime helper logic for sessions
* `session_model_role.go` - Session model/role switching and tool registry selection
* `session_thread.go` - Thread-safe async session controller
* `session_agents.go` - Injects AGENTS.md into context
* `session_core_integ_retry_test.go` - Integration tests for SweSession retry behavior
* `prompt.go` - Prompt fragments and tool info builder
* `role.go` - Cached agent role registry
* `hooks_engine.go` - Hook execution and feedback handling
* `task.go` - Persistent task management backend
* `task_storage.go` - Task metadata file persistence and task tree scanning helpers
* `task_summary.go` - Task session summary/output persistence helpers
* `session_persistence.go` - Session state persistence and restore
* `session_summary.go` - Session summary JSON and markdown output
* `commit_message.go` - LLM-based commit message generation
* `worktree_branch.go` - LLM-based branch suffix generation
* `compact.go` - Conversation compaction pipeline
* `state.go` - Shared agent state types

## Important public API objects

* `SweSession` - In-memory session runtime object
* `SweSessionParams` - SweSession construction inputs
* `SessionThread` - Async session execution wrapper
* `SessionThreadInput` - Input operations for session threads
* `SessionThreadOutput` - Output callbacks for session threads
* `SessionFactory` - Creates sessions for threads
* `SubAgentTaskRunner` - Runs delegated subagent tasks
* `PromptGenerator` - Builds prompts and tool docs
* `ConfPromptGenerator` - ConfigStore-backed prompt generator
* `AgentRoleRegistry` - Cached role lookup service
* `AgentState` - Runtime prompt template state
* `AgentStateCommonInfo` - Shared runtime metadata fields
* `HookEngine` - Executes configured lifecycle hooks
* `HookOutputView` - Hook output sink interface
* `HookContext` - Hook context key-value map
* `HookExecutionRequest` - Hook invocation request payload
* `HookExecutionResult` - Hook execution output data
* `HookExecutionError` - Hook non-zero exit error
* `HookSessionStatus` - none, running, success, failed
* `HookFeedbackResponseMode` - none, stdin, rerun
* `Task` - Persistent task metadata record
* `TaskManager` - Persistent task lifecycle manager
* `TaskBackendAdapter` - Tool backend adapter for tasks
* `TaskSessionRunner` - Runs a single task session
* `SessionSummaryBuildResult` - Runtime fields for summary build
* `PersistedSessionState` - Serialized session state alias
* `SessionSummaryJSON` - Persisted session summary schema
* `SubAgentSummaryJSON` - Persisted subagent summary schema
* `NewSweSession()` - Create new session instance
* `NewSessionThread()` - Create thread for new session
* `NewSessionThreadWithSession()` - Create thread for existing session
* `NewConfPromptGenerator()` - Create config prompt generator
* `NewAgentRoleRegistry()` - Create role registry
* `NewHookEngine()` - Create hook engine
* `NewTaskManager()` - Create task manager
* `NewTaskManagerWithTasksDir()` - Create task manager with dir
* `NewTaskBackendAdapter()` - Create task backend adapter
* `NewCLITaskSessionRunner()` - Create CLI task runner
* `CompactMessages()` - Compact chat history messages
* `GenerateCommitMessage()` - Produce commit message via model
* `GenerateWorktreeBranchName()` - Produce worktree branch suffix
* `RestoreSessionFromPersistedState()` - Rebuild session from persisted state
* `BuildSessionSummaryJSON()` - Build session summary payload
* `SaveSessionSummaryJSON()` - Save session summary JSON file
* `EmitSessionSummary()` - Emit and persist session summary
* `WriteSubAgentSummary()` - Persist subagent summary files
