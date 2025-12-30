Add method `MoveFile(src, dst string) error` to `SweVFS` interface in `pkg/vfs/vfs.go`:
* Implement it for `LocalVFS` and `MockVFS` in `pkg/vfs/local.go` and `pkg/vfs/mock.go`.
* Add appropriate tests for it.
* Make sure both files and directories are moved correctly.
* Make sure method can be used also for renaming files;
