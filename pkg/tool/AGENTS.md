# pkg/tool

`pkg/tool` is the agent tool execution layer. It defines the common tool interfaces/schema model, tool registry/composition behavior, permission-query integration, and concrete tools for shell execution, todo tracking, and VFS-based file operations.

## Major files

- `tool.go`: Core public tool API (`Tool`, `ToolCall`, `ToolResponse`, `ToolInfo`, schema/value types).
- `registry.go`: Tool registry for registration, lookup, execution, logger injection, and model-tag filtering.
- `access.go`: Access-control wrapper for tools based on configured permissions and wildcard rule matching.
- `permissions.go`: Shared permission-query builders and option normalization.
- `run_bash.go`: Bash execution tool implementation with privilege checks and command options.
- `todo.go`: Todo read/write tool implementations and todo item/session interfaces.
- `vfs_patch.go`: Structured patch-application tool for add/update/move/delete operations with diagnostics.
- `vfs_write.go`: Full file write tool with optional post-write LSP validation.
- `vfs_edit.go`: Targeted string-replacement edit tool with optional diagnostics.
- `vfs_read.go`: File read tool with offset/limit slicing and numbered output formatting.
- `vfs_grep.go`: Regex file-content search tool with optional include/path filters.
- `vfs_helpers.go`: Shared helpers for VFS tool output formatting and LSP diagnostic conversion.
