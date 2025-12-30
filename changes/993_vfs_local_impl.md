Implement local filesystem implementation of `SweVFS` interface in `pkg/vfs/local/local.go`:
* it should implement `SweVFS` interface
* it should use standard library for file operations
* it should work on local filesystem in a given root directory passed to `NewLocalVFS` method
* it should return errors defined in `pkg/vfs/vfs.go`
* local vfs should be sandboxed, i.e. it should not be possible to access files outside of root directory
* all paths inside VFS should be relative to root path passed during initialization
* use error constants defined in `pkg/vfs/vfs.go` to return errors
* if certain errors are not defined but will have to be returned, add them to `pkg/vfs/vfs.go`
* write comprehensive tests for it in `pkg/vfs/local/local_test.go`
* make sure that proper cleanup is performed in test fixture

