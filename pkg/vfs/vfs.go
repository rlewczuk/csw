package vfs

import "errors"

var (
	ErrFileNotFound     = errors.New("file not found")
	ErrFileExists       = errors.New("file already exists")
	ErrNotADir          = errors.New("not a directory")
	ErrNotAFile         = errors.New("not a file")
	ErrPermissionDenied = errors.New("permission denied")
	ErrNotImplemented   = errors.New("not implemented")
	ErrInvalidPath      = errors.New("invalid path")
	ErrAskPermission    = errors.New("ask permission")
)

// VFS represents virtual filesystem. It encapsulates access to local and remote files and directories,
// git worktree, in-memory files etc., may implement access control for agent etc.
// Version control, snapshotting etc. is not part of VFS, it is implemented in another layer.
type VFS interface {

	// ReadFile reads the content of the file located at the given path and returns its data as a byte slice.
	ReadFile(path string) ([]byte, error)

	// WriteFile writes the given content to the file located at the given path.
	WriteFile(path string, content []byte) error

	// DeleteFile deletes the file located at the given path.
	DeleteFile(path string, recursive bool, force bool) error

	// ListFiles lists all files and directories located at the given path.
	ListFiles(path string, recursive bool) ([]string, error)

	// FindFiles searches for files and directories matching the given query.
	FindFiles(query string, recursive bool) ([]string, error)

	// MoveFile moves or renames a file or directory from src to dst.
	// It works for both files and directories.
	// Can be used for renaming by providing a different name in dst within the same directory.
	MoveFile(src, dst string) error
}
