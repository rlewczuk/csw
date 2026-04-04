# Package `pkg/core` Overview

Package `pkg/core` contains the runtime orchestration layer for agent sessions, managing session lifecycle, prompt/tool assembly, role/model switching, the run loop, tool-call execution flow, permission pauses, and async session threading used by UI layers.

## Important files

* `commit_message.go` - Commit message generation using model-backed templates
* `compact.go` - Context compaction for chat messages to manage token limits
* `hooks_engine.go` - Hook execution engine for lifecycle extension points
* `prompt.go` - Prompt and tool-info generator with role fragment merging
* `role.go` - Role registry with cached config loading and role merging
* `session.go` - Core session engine with chat/tool loops and retries
* `session_agents.go` - AGENTS.md instruction injection helpers for VFS tool results
* `session_persisted.go` - Persisted session state model for loading sessions
* `session_thread.go` - Async thread wrapper for non-blocking UI interaction
* `state.go` - Agent state structures for template processing
* `worktree_branch.go` - Worktree branch name generation using models

## Important public API objects

* `SweSession` - Core session engine managing chat loops and tool execution
* `SweSessionParams` - Parameters for creating a new SweSession
* `SessionThread` - Async wrapper for session with pause/resume/interrupt control
* `SessionThreadInput` - Interface for handling input to the session
* `SessionThreadOutput` - Interface for handling output from the session
* `SessionFactory` - Interface for creating new sessions
* `AgentRoleRegistry` - Registry for agent role configurations with caching
* `PromptGenerator` - Interface for generating prompts and tool info
* `ConfPromptGenerator` - Prompt generator using ConfigStore
* `AgentState` - Agent state structure for template processing
* `AgentStateCommonInfo` - Common agent state information
* `SubAgentTaskRunner` - Interface for executing delegated subagent tasks
* `HookEngine` - Executes configured hooks against session context
* `HookContext` - Cumulative hook context values for one session
* `HookExecutionRequest` - Defines one hook execution
* `HookExecutionResult` - Contains hook command execution details
* `HookExecutionError` - Represents non-zero hook exit code
* `HookSessionStatus` - Lifecycle status exposed to hook context (none, running, success, failed)
* `HookFeedbackRequest` - Defines one feedback command emitted by script
* `HookFeedbackResponse` - Defines processed feedback result
* `HookFeedbackResponseMode` - Response delivery mode for feedback (none, stdin, rerun)
* `PersistedSessionState` - Persisted session state model for loading sessions
* `GenerateCommitMessage()` - Generates commit messages using LLM
* `GenerateWorktreeBranchName()` - Generates worktree branch names using LLM
* `CompactMessages()` - Applies multi-step compaction to chat messages
* `RestoreSessionFromPersistedState()` - Restores session from persisted state
* `NewHookEngine()` - Creates a new hook execution engine
* `NewAgentRoleRegistry()` - Creates a new AgentRoleRegistry
* `NewConfPromptGenerator()` - Creates a new ConfPromptGenerator
* `NewSweSession()` - Creates a new SweSession from parameters
* `NewSessionThread()` - Creates a new SessionThread
* `NewSessionThreadWithSession()` - Creates SessionThread with existing session
* `IsHookExecutionError()` - Reports whether error indicates non-zero hook exit
