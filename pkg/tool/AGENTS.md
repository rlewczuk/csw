# Package `pkg/tool` Overview

Package `pkg/tool` defines tool contracts, registry, and built-ins in `pkg/tool`.

## Important files

* `tool.go` - Core tool types and interfaces.
* `registry.go` - Tool registry and execution orchestration.
* `access.go` - Access control wrapper and role checks.
* `permissions.go` - Permission query helpers and defaults.
* `run_bash.go` - Bash tool and command validation.
* `todo.go` - Todo tools and item validation.
* `subagent.go` - Subagent request/response tool adapter.
* `skill.go` - Skill loading tool implementation.
* `web_fetch.go` - Web fetch tool implementation.
* `hook_feedback.go` - Hook feedback tool bridge.
* `custom.go` - Config-driven custom command tools.
* `task_new.go` - Task creation tool.
* `task_update.go` - Task update tool.
* `task_run.go` - Task run tool.
* `task_merge.go` - Task merge tool.
* `task_get.go` - Task metadata query tool.
* `task_list.go` - Task listing tool.
* `vfs_read.go` - VFS file read tool.
* `vfs_write.go` - VFS file write tool.
* `vfs_edit.go` - VFS text edit tool.
* `vfs_patch.go` - VFS patch apply tool.
* `vfs_delete.go` - VFS delete tool.
* `vfs_move.go` - VFS move/rename tool.
* `vfs_list.go` - VFS directory listing tool.
* `vfs_find.go` - VFS glob search tool.
* `vfs_grep.go` - VFS content grep tool.

## Important public API objects

* `Tool` - Base interface for executable tools.
* `ToolCall` - Tool invocation payload.
* `ToolResponse` - Tool execution result payload.
* `ToolValue` - JSON-compatible dynamic value wrapper.
* `ToolInfo` - Tool metadata with schema.
* `ToolSchema` - JSON Schema root object.
* `PropertySchema` - JSON Schema property definition.
* `SchemaType` - Enum: string, number, integer, boolean, array, object.
* `ToolRegistry` - Concurrent tool registry and executor.
* `AccessControlTool` - Permission-aware tool wrapper.
* `ToolPermissionsQuery` - Permission request payload.
* `LoggerSetter` - Interface for logger injection.
* `RoleRestrictedTool` - Interface for role-limited tools.
* `RunBashTool` - Bash command execution tool.
* `RunCommandError` - Run-bash policy violation error.
* `TodoItem` - Todo item payload model.
* `TodoSession` - Todo storage session interface.
* `TodoWriteTool` - Todo list write tool.
* `TodoReadTool` - Todo list read tool.
* `SubAgentExecutor` - Subagent execution backend interface.
* `SubAgentTaskRequest` - Subagent task request payload.
* `SubAgentTaskResult` - Subagent task result payload.
* `SubAgentTool` - Subagent delegation tool.
* `HookFeedbackExecutor` - Hook feedback backend interface.
* `HookFeedbackRequest` - Hook feedback request payload.
* `HookFeedbackResponse` - Hook feedback response payload.
* `HookFeedbackTool` - Hook feedback forwarding tool.
* `SkillTool` - Skill loading tool.
* `WebFetchTool` - Web fetching tool.
* `VFSReadTool` - VFS read tool.
* `VFSWriteTool` - VFS write tool.
* `VFSEditTool` - VFS edit tool.
* `VFSPatchTool` - VFS patch tool.
* `VFSDeleteTool` - VFS delete tool.
* `VFSMoveTool` - VFS move tool.
* `VFSListTool` - VFS list tool.
* `VFSFindTool` - VFS file-find tool.
* `VFSGrepTool` - VFS content-grep tool.
* `CustomCommandTool` - Configured custom command tool.
* `TaskBackend` - Persistent task backend interface.
* `TaskSessionRef` - Session task reference interface.
* `TaskRecord` - Persistent task metadata payload.
* `TaskSessionSummary` - Task session summary payload.
* `TaskRunOutcome` - Task run result payload.
* `TaskNewTool` - Task creation tool.
* `TaskUpdateTool` - Task update tool.
* `TaskRunTool` - Task run tool.
* `TaskMergeTool` - Task merge tool.
* `TaskGetTool` - Task lookup tool.
* `TaskListTool` - Task listing tool.
* `NewToolRegistry()` - Creates empty tool registry.
* `NewToolValue()` - Creates `ToolValue` from raw value.
* `NewToolSchema()` - Creates empty tool schema.
* `NewPermissionQuery()` - Creates permission query response.
* `NewVFSPermissionQuery()` - Creates VFS permission query response.
* `NewAccessControlTool()` - Wraps tool with access checks.
* `NewRunBashTool()` - Creates run-bash tool.
* `NewRunBashToolWithSessionWorkdir()` - Creates run-bash tool using session workdir.
* `NewTodoWriteTool()` - Creates todo-write tool.
* `NewTodoReadTool()` - Creates todo-read tool.
* `NewSubAgentTool()` - Creates subagent tool.
* `NewHookFeedbackTool()` - Creates hook-feedback tool.
* `NewSkillTool()` - Creates skill tool.
* `NewWebFetchTool()` - Creates web-fetch tool.
* `NewVFSReadTool()` - Creates VFS-read tool.
* `NewVFSWriteTool()` - Creates VFS-write tool.
* `NewVFSEditTool()` - Creates VFS-edit tool.
* `NewVFSPatchTool()` - Creates VFS-patch tool.
* `NewVFSDeleteTool()` - Creates VFS-delete tool.
* `NewVFSMoveTool()` - Creates VFS-move tool.
* `NewVFSListTool()` - Creates VFS-list tool.
* `NewVFSFindTool()` - Creates VFS-find tool.
* `NewVFSGrepTool()` - Creates VFS-grep tool.
* `NewTaskNewTool()` - Creates task-new tool.
* `NewTaskUpdateTool()` - Creates task-update tool.
* `NewTaskRunTool()` - Creates task-run tool.
* `NewTaskMergeTool()` - Creates task-merge tool.
* `NewTaskGetTool()` - Creates task-get tool.
* `NewTaskListTool()` - Creates task-list tool.
* `RegisterVFSTools()` - Registers all VFS tools.
* `RegisterRunBashTool()` - Registers run-bash tool.
* `RegisterWebFetchTool()` - Registers web-fetch tool.
* `RegisterSkillTool()` - Registers skill tool.
* `RegisterCustomTools()` - Registers configured custom tools.
* `PermissionOptions()` - Returns standard permission options.
