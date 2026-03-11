# Package `pkg/tool` Overview

Package `pkg/tool` is the agent tool execution layer. It defines the common tool interfaces/schema model, tool registry/composition behavior, permission-query integration, and concrete tools for shell execution, todo tracking, subagent delegation, skill loading, web fetching, and VFS-based file operations.

## Important files

* `tool.go` - Core tool API and schema/value types
* `registry.go` - Tool registry for registration and execution
* `access.go` - Access-control wrapper with permission rules
* `permissions.go` - Permission query builders and options
* `custom.go` - Custom user-defined tool execution
* `run_bash.go` - Bash command execution tool
* `todo.go` - Todo list read/write tools
* `subagent.go` - Subagent task delegation tool
* `skill.go` - Skill loading from .agents/skills
* `web_fetch.go` - Web content fetching tool
* `vfs_read.go` - File read with offset/limit
* `vfs_write.go` - File write with LSP validation
* `vfs_edit.go` - String replacement edit tool
* `vfs_patch.go` - Structured patch application
* `vfs_delete.go` - File deletion tool
* `vfs_move.go` - File move/rename tool
* `vfs_list.go` - Directory listing tool
* `vfs_find.go` - File glob search tool
* `vfs_grep.go` - Content regex search tool
* `vfs_helpers.go` - Shared VFS tool helpers

## Important public API objects

* `Tool` - Interface for executable tools
* `ToolCall` - Represents a tool invocation
* `ToolResponse` - Tool execution result
* `ToolValue` - JSON-compatible value wrapper
* `ToolInfo` - Tool metadata and schema
* `ToolSchema` - JSON Schema for tool arguments
* `PropertySchema` - Schema for single property
* `SchemaType` - JSON Schema type constants
* `ToolRegistry` - Tool registration and lookup
* `AccessControlTool` - Permission wrapper for tools
* `ToolPermissionsQuery` - User permission request
* `LoggerSetter` - Interface for logger injection
* `RunBashTool` - Bash execution implementation
* `TodoWriteTool` - Todo list update tool
* `TodoReadTool` - Todo list retrieval tool
* `TodoItem` - Single todo task representation
* `TodoSession` - Interface for todo access
* `SubAgentTool` - Subagent delegation tool
* `SubAgentExecutor` - Interface for subagent execution
* `SubAgentTaskRequest` - Subagent task input
* `SubAgentTaskResult` - Subagent task output
* `SkillTool` - Skill loading tool
* `WebFetchTool` - Web fetching tool
* `VFSReadTool` - File read tool
* `VFSWriteTool` - File write tool
* `VFSEditTool` - File edit tool
* `VFSPatchTool` - Patch application tool
* `VFSDeleteTool` - File delete tool
* `VFSMoveTool` - File move tool
* `VFSListTool` - Directory list tool
* `VFSFindTool` - File find tool
* `VFSGrepTool` - Content search tool
* `CustomCommandTool` - User-defined custom tool
* `RoleRestrictedTool` - Role-based tool restriction
* `NewToolRegistry()` - Creates tool registry
* `NewToolValue()` - Creates tool value
* `NewToolSchema()` - Creates tool schema
* `NewPermissionQuery()` - Creates permission query
* `NewVFSPermissionQuery()` - Creates VFS permission query
* `RegisterVFSTools()` - Registers all VFS tools
* `RegisterRunBashTool()` - Registers bash tool
* `RegisterWebFetchTool()` - Registers web fetch tool
* `RegisterSkillTool()` - Registers skill tool
* `RegisterCustomTools()` - Registers custom tools
