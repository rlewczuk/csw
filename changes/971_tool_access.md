Implement mechanism for controlling access to tools in `pkg/tool/access.go`:
* implement mechanism as `AccessControlTool` struct in `pkg/tool/access.go`, along with constructor function etc.;
* use `AccessFlag` from `pkg/shared/access.go` to define access flags;
* implement object satisfying `Tool` interface that will check access flags before delegating to another `Tool` implementation;
* it should be possible to specify different access flags for different tools (map of tool names to `AccessFlag`);
* it should be possible to use masks for tool names, for example `*` for all tools, `vfs.*` for all VFS tools etc.;
* tool name consist of one or more alphanumeric segments separated by dot, for example `vfs.read` or `custom.tool`, mask `*` is limited to single segment but `**` can be used for multiple segments;
* take into account that tool name mask can contain part of name and `*` or `**` in a single segment, for example `foo.ba*` will match `foo.bar` and `foo.baz` but not `foo.bar.baz`;
* if there are multiple matching masks, the most specific one should be used;
* adding `**` item with given access flag to privileges map should effectively work as default for tools not matched by any other mask;
* it should have default access flags that deny all access if nothing matches and there is no `**` in privileges map;
* be sure that default `**` is not explicitly implemented, it is just feature of implemented matching logic;
* write comprehensive tests for it in `pkg/tool/access_test.go`;
