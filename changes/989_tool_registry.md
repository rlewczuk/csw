Create tool registry in `pkg/tool/registry.go`:
* it should be a map of tool names to tool implementations;
* it should have methods `Register` and `Get` to register and retrieve tools;
* it should have method `List` to list all registered tools;
* it should be possible to register tool under multiple names;
* it should implement `SweTool` interface and delegate execution to appropriate tool;
* it should return error if tool is not found;
* generate function that will accept `VFS` implementation and register all VFS tools;
* write tests for it in `pkg/tool/registry_test.go`, use `testify` library for assertions, mock VFS implemented in `pkg/vfs/mock.go` and existing VFS tools implemented in `pkg/tool/vfs.go`;

