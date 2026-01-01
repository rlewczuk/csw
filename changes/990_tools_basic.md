Implement tools for basic VFS operations in `pkg/tool/vfs.go`:
* tools should implement `SweTool` interface;
* tools should have following names: `vfs.read`, `vfs.write`, `vfs.delete`, `vfs.list`, `vfs.move`;
* do not implement `vfs.find` tool;
* tool function `vfs.read` should accept argument `path` (string) and return `content` (string);
* tool function `vfs.write` should accept arguments `path` (string) and `content` (string) and return nothing;
* tool function `vfs.delete` should accept argument `path` (string) and return nothing;
* tool function `vfs.list` should accept argument `path` (string) and return `files` (array of string);
* tool function `vfs.move` should accept arguments `path` (string) and `destination` (string) and return nothing;
* implement tests for all tools in `pkg/tool/vfs_test.go`, use `testify` library for assertions and mock `VFS` implementation from `pkg/vfs/mock.go`;
