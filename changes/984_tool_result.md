Remove redundancy between `ToolValue`, `ToolArgs` and `ToolResult` by merging them into single `ToolValue` type:
* it should be possible to create `ToolValue` from any JSON-compatible value;
* it should be possible to access underlying value as `any`;
* retain existing convenience accessors for primitive types (string, bool, int, float) and collections (array, object);
* retain existing introspection methods (Has, Keys, Len);
* adapt `Tool` interface to use `ToolValue` for arguments and results;
* adapt existing tools in `vfs.go` to use single `ToolValue` type;
* update unit tests for both code from `tool.go` and `vfs.go` to use `ToolValue`;
* if certain tests are redundant after merge, remove them;
* make sure all tests work after merge;
