package vfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalVFS implements VFS interface for local filesystem operations.
// It provides sandboxed access to files within a root directory.
type LocalVFS struct {
	root   string
	repo   Repo
	filter GlobFilter
}

func (l *LocalVFS) GetBranch() string {
	return l.root
}

func (l *LocalVFS) WorktreePath() string {
	return l.root
}

func (l *LocalVFS) GetRepo() Repo {
	return l.repo
}

// NewLocalVFS creates a new LocalVFS instance rooted at the given directory.
// The root path is converted to an absolute path and all operations are sandboxed within this directory.
// The hidePatterns parameter specifies glob patterns for files and directories that should be ignored
// by all operations (ListFiles, FindFiles). By default, it should be empty (no files are hidden).
func NewLocalVFS(root string, hidePatterns []string) (*LocalVFS, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// Ensure root directory exists
	info, err := os.Stat(absRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("NewLocalVFS() [local.go]: %w", ErrFileNotFound)
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("NewLocalVFS() [local.go]: %w", ErrNotADir)
	}

	// Create filter with defaultMatch=false (don't hide anything by default)
	// and patterns that should be hidden (match=true means hide)
	filter := NewGlobFilter(false, hidePatterns)

	return &LocalVFS{root: absRoot, filter: filter}, nil
}

// validatePath ensures the path is valid and within the sandbox.
// It returns the absolute filesystem path if valid.
func (l *LocalVFS) validatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("LocalVFS.validatePath() [local.go]: %w", ErrInvalidPath)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(l.root, path)
	}

	// Clean the path to remove any .. or . components
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(cleanPath, l.root) {
		return cleanPath, nil
	}

	return "", fmt.Errorf("LocalVFS.validatePath() [local.go]: %w", ErrPermissionDenied)
}

// ReadFile reads the content of the file located at the given path and returns its data as a byte slice.
func (l *LocalVFS) ReadFile(path string) ([]byte, error) {
	absPath, err := l.validatePath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("LocalVFS.ReadFile() [local.go]: %w", ErrFileNotFound)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("LocalVFS.ReadFile() [local.go]: %w", ErrPermissionDenied)
		}
		return nil, err
	}

	return data, nil
}

// WriteFile writes the given content to the file located at the given path.
// It creates parent directories if they don't exist.
func (l *LocalVFS) WriteFile(path string, content []byte) error {
	absPath, err := l.validatePath(path)
	if err != nil {
		return err
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("LocalVFS.WriteFile() [local.go]: %w", ErrPermissionDenied)
		}
		return err
	}

	// Write the file
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("LocalVFS.WriteFile() [local.go]: %w", ErrPermissionDenied)
		}
		return err
	}

	return nil
}

// DeleteFile deletes the file located at the given path.
// If recursive is true, directories and their contents are deleted.
// If force is true, read-only files are also deleted.
func (l *LocalVFS) DeleteFile(path string, recursive bool, force bool) error {
	absPath, err := l.validatePath(path)
	if err != nil {
		return err
	}

	// Check if file/directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("LocalVFS.DeleteFile() [local.go]: %w", ErrFileNotFound)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("LocalVFS.DeleteFile() [local.go]: %w", ErrPermissionDenied)
		}
		return err
	}

	// If it's a directory and recursive is false, return error
	if info.IsDir() && !recursive {
		return fmt.Errorf("LocalVFS.DeleteFile() [local.go]: %w", ErrNotAFile)
	}

	// If force is true, make writable before deletion
	if force {
		if err := os.Chmod(absPath, 0755); err != nil && !os.IsPermission(err) {
			return err
		}
	}

	// Delete the file or directory
	var deleteErr error
	if info.IsDir() {
		deleteErr = os.RemoveAll(absPath)
	} else {
		deleteErr = os.Remove(absPath)
	}

	if deleteErr != nil {
		if os.IsPermission(deleteErr) {
			return fmt.Errorf("LocalVFS.DeleteFile() [local.go]: %w", ErrPermissionDenied)
		}
		return deleteErr
	}

	return nil
}

// ListFiles lists all files and directories located at the given path.
// If recursive is true, it lists all files and directories recursively.
// Returns paths relative to the VFS root.
func (l *LocalVFS) ListFiles(path string, recursive bool) ([]string, error) {
	absPath, err := l.validatePath(path)
	if err != nil {
		return nil, err
	}

	// Check if path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("LocalVFS.ListFiles() [local.go]: %w", ErrFileNotFound)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("LocalVFS.ListFiles() [local.go]: %w", ErrPermissionDenied)
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("LocalVFS.ListFiles() [local.go]: %w", ErrNotADir)
	}

	var result []string

	if recursive {
		err = filepath.Walk(absPath, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip the root directory itself
			if p == absPath {
				return nil
			}

			// Get relative path from VFS root
			relPath, err := filepath.Rel(l.root, p)
			if err != nil {
				return err
			}

			// Skip if path matches the filter (is hidden)
			if l.filter.Matches(relPath) {
				// If it's a directory, skip the entire subtree
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			result = append(result, relPath)
			return nil
		})

		if err != nil {
			if os.IsPermission(err) {
				return nil, fmt.Errorf("LocalVFS.ListFiles() [local.go]: %w", ErrPermissionDenied)
			}
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			if os.IsPermission(err) {
				return nil, fmt.Errorf("LocalVFS.ListFiles() [local.go]: %w", ErrPermissionDenied)
			}
			return nil, err
		}

		for _, entry := range entries {
			fullPath := filepath.Join(absPath, entry.Name())
			relPath, err := filepath.Rel(l.root, fullPath)
			if err != nil {
				return nil, err
			}

			// Skip if path matches the filter (is hidden)
			if l.filter.Matches(relPath) {
				continue
			}

			result = append(result, relPath)
		}
	}

	return result, nil
}

// FindFiles searches for files and directories matching the given query.
// The query supports glob patterns:
//   - * matches any number of characters except /
//   - ? matches any single character except /
//   - [abc] matches any character in the set
//   - [a-z] matches any character in the range
//   - ** matches any number of characters including /
//
// If recursive is true, it searches recursively from the root.
// Returns paths relative to the VFS root.
func (l *LocalVFS) FindFiles(query string, recursive bool) ([]string, error) {
	if query == "" {
		return nil, fmt.Errorf("LocalVFS.FindFiles() [local.go]: %w", ErrInvalidPath)
	}

	var result []string

	searchRoot := l.root

	if recursive {
		err := filepath.Walk(searchRoot, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				// Skip permission errors during walk
				if os.IsPermission(err) {
					return nil
				}
				return err
			}

			// Get relative path from VFS root
			relPath, err := filepath.Rel(l.root, p)
			if err != nil {
				return err
			}

			// Skip the root directory itself
			if relPath == "." {
				return nil
			}

			// Skip if path matches the filter (is hidden)
			if l.filter.Matches(relPath) {
				// If it's a directory, skip the entire subtree
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Match against the relative path using glob pattern
			matched, err := matchGlob(query, relPath)
			if err != nil {
				return err
			}

			if matched {
				result = append(result, relPath)
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(searchRoot)
		if err != nil {
			if os.IsPermission(err) {
				return nil, fmt.Errorf("LocalVFS.FindFiles() [local.go]: %w", ErrPermissionDenied)
			}
			return nil, err
		}

		for _, entry := range entries {
			// Skip if path matches the filter (is hidden)
			if l.filter.Matches(entry.Name()) {
				continue
			}

			matched, err := matchGlob(query, entry.Name())
			if err != nil {
				return nil, err
			}

			if matched {
				result = append(result, entry.Name())
			}
		}
	}

	return result, nil
}

// MoveFile moves or renames a file or directory from src to dst.
// It works for both files and directories.
// Can be used for renaming by providing a different name in dst within the same directory.
func (l *LocalVFS) MoveFile(src, dst string) error {
	absSrc, err := l.validatePath(src)
	if err != nil {
		return err
	}

	absDst, err := l.validatePath(dst)
	if err != nil {
		return err
	}

	// Check if source exists
	_, err = os.Stat(absSrc)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("LocalVFS.MoveFile() [local.go]: %w", ErrFileNotFound)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("LocalVFS.MoveFile() [local.go]: %w", ErrPermissionDenied)
		}
		return err
	}

	// Check if destination already exists
	_, err = os.Stat(absDst)
	if err == nil {
		return fmt.Errorf("LocalVFS.MoveFile() [local.go]: %w", ErrFileExists)
	}
	if !os.IsNotExist(err) {
		if os.IsPermission(err) {
			return fmt.Errorf("LocalVFS.MoveFile() [local.go]: %w", ErrPermissionDenied)
		}
		return err
	}

	// Create parent directory of destination if it doesn't exist
	dstDir := filepath.Dir(absDst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("LocalVFS.MoveFile() [local.go]: %w", ErrPermissionDenied)
		}
		return err
	}

	// Perform the move
	if err := os.Rename(absSrc, absDst); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("LocalVFS.MoveFile() [local.go]: %w", ErrPermissionDenied)
		}
		return err
	}

	return nil
}
