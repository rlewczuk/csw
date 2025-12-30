package vfs

import (
	"os"
	"path/filepath"
	"strings"
)

// LocalVFS implements SweVFS interface for local filesystem operations.
// It provides sandboxed access to files within a root directory.
type LocalVFS struct {
	root string
}

// NewLocalVFS creates a new LocalVFS instance rooted at the given directory.
// The root path is converted to an absolute path and all operations are sandboxed within this directory.
func NewLocalVFS(root string) (*LocalVFS, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// Ensure root directory exists
	info, err := os.Stat(absRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, ErrNotADir
	}

	return &LocalVFS{root: absRoot}, nil
}

// validatePath ensures the path is valid and within the sandbox.
// It returns the absolute filesystem path if valid.
func (l *LocalVFS) validatePath(path string) (string, error) {
	if path == "" {
		return "", ErrInvalidPath
	}

	// Clean the path to remove any .. or . components
	cleanPath := filepath.Clean(path)

	// Prevent absolute paths or paths that try to escape
	if filepath.IsAbs(cleanPath) {
		return "", ErrInvalidPath
	}

	// Join with root and ensure it's still within root
	absPath := filepath.Join(l.root, cleanPath)
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", err
	}

	// Ensure the resolved path is still within root
	if !strings.HasPrefix(absPath, l.root+string(filepath.Separator)) && absPath != l.root {
		return "", ErrPermissionDenied
	}

	return absPath, nil
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
			return nil, ErrFileNotFound
		}
		if os.IsPermission(err) {
			return nil, ErrPermissionDenied
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
			return ErrPermissionDenied
		}
		return err
	}

	// Write the file
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
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
			return ErrFileNotFound
		}
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return err
	}

	// If it's a directory and recursive is false, return error
	if info.IsDir() && !recursive {
		return ErrNotAFile
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
			return ErrPermissionDenied
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
			return nil, ErrFileNotFound
		}
		if os.IsPermission(err) {
			return nil, ErrPermissionDenied
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, ErrNotADir
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

			result = append(result, relPath)
			return nil
		})

		if err != nil {
			if os.IsPermission(err) {
				return nil, ErrPermissionDenied
			}
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			if os.IsPermission(err) {
				return nil, ErrPermissionDenied
			}
			return nil, err
		}

		for _, entry := range entries {
			fullPath := filepath.Join(absPath, entry.Name())
			relPath, err := filepath.Rel(l.root, fullPath)
			if err != nil {
				return nil, err
			}
			result = append(result, relPath)
		}
	}

	return result, nil
}

// FindFiles searches for files and directories matching the given query.
// The query is matched using filepath.Match pattern.
// If recursive is true, it searches recursively from the root.
// Returns paths relative to the VFS root.
func (l *LocalVFS) FindFiles(query string, recursive bool) ([]string, error) {
	if query == "" {
		return nil, ErrInvalidPath
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

			// Match against the filename or relative path
			matched, err := filepath.Match(query, filepath.Base(p))
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
				return nil, ErrPermissionDenied
			}
			return nil, err
		}

		for _, entry := range entries {
			matched, err := filepath.Match(query, entry.Name())
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
