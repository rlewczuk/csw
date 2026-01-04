Implement mechanism for controlling access to VFS operations in `pkg/vfs/access.go`:
* proceed to finish `AccessControlVFS` struct in `pkg/vfs/access.go`;
* use `FileAccess` struct from `pkg/shared/access.go` to define access flags;
* implement object satisfying `VFS` interface that will check access flags before delegating to another `VFS` implementation;
* it should be possible to specify different access flags for different paths (map of UNIX style globs to `FileAccess`);
* it should be possible to specify default access flags for all paths as `*` in above map;
* it should have default access flags that deny all access if nothing matches;
* if there are multiple matching globs, the most specific one should be used;
* make sure all VFS operations check access before performing operation;
* implement comprehensive set of tests for it in `pkg/vfs/access_test.go`;
* make sure to test glob matching and shadowing logic thoroughly, so that there are no security holes;


---


Check if more nuanced globs are working properly, for example `/foo/bar*` will be more specific than `/foo/*`, 
or `/foo/bar/baz*` will be more specific than `/foo/b*/baz*`.