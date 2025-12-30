Implement mock implementation of `SweVFS` interface in `pkg/vfs/mock.go`:
* it should implement `SweVFS` interface;
* it should behave identically to `LocalVFS` but keep all files in memory;
* it should not write any files to disk;
* it should be possible to prepopulate mock with files from given directory;
* make sure it shares test suite with `LocalVFS` (see `vfs_test.go`), so that we can be sure that both have identical behavior;
* do not create new tests file, modify `vfs_test.go` to run tests against both `LocalVFS` and `MockVFS`;


 
