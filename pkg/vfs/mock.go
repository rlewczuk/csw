package vfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// fileEntry represents a file or directory in the mock filesystem.
type fileEntry struct {
	isDir    bool
	content  []byte
	children map[string]*fileEntry
}

// MockVFS implements SweVFS interface with an in-memory filesystem.
// It behaves identically to LocalVFS but keeps all files in memory.
type MockVFS struct {
	root  *fileEntry
	mutex sync.RWMutex
}

// NewMockVFS creates a new MockVFS instance with an empty in-memory filesystem.
func NewMockVFS() *MockVFS {
	return &MockVFS{
		root: &fileEntry{
			isDir:    true,
			children: make(map[string]*fileEntry),
		},
	}
}

// NewMockVFSFromDir creates a new MockVFS instance and prepopulates it with files from the given directory.
func NewMockVFSFromDir(dir string) (*MockVFS, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	// Ensure directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, ErrNotADir
	}

	mock := NewMockVFS()

	// Walk the directory and populate the mock
	err = filepath.Walk(absDir, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if p == absDir {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(absDir, p)
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Create directory
			mock.createDir(relPath)
		} else {
			// Read and write file
			content, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if err := mock.WriteFile(relPath, content); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return mock, nil
}

// validatePath ensures the path is valid.
// It returns the cleaned path if valid.
func (m *MockVFS) validatePath(path string) (string, error) {
	if path == "" {
		return "", ErrInvalidPath
	}

	// Clean the path to remove any .. or . components
	cleanPath := filepath.Clean(path)

	// Prevent absolute paths or paths that try to escape
	if filepath.IsAbs(cleanPath) {
		return "", ErrInvalidPath
	}

	// Check for path traversal attempts
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, string(filepath.Separator)+"..") {
		return "", ErrPermissionDenied
	}

	return cleanPath, nil
}

// getEntry navigates to the entry at the given path.
// Returns the entry and its parent, or nil if not found.
func (m *MockVFS) getEntry(path string) (*fileEntry, error) {
	if path == "." || path == "" {
		return m.root, nil
	}

	parts := strings.Split(filepath.ToSlash(path), "/")
	current := m.root

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		if !current.isDir {
			return nil, ErrNotADir
		}

		next, exists := current.children[part]
		if !exists {
			return nil, ErrFileNotFound
		}

		current = next
	}

	return current, nil
}

// createDir creates a directory at the given path, including parent directories.
func (m *MockVFS) createDir(path string) error {
	if path == "." || path == "" {
		return nil
	}

	parts := strings.Split(filepath.ToSlash(path), "/")
	current := m.root

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		if !current.isDir {
			return ErrNotADir
		}

		next, exists := current.children[part]
		if !exists {
			next = &fileEntry{
				isDir:    true,
				children: make(map[string]*fileEntry),
			}
			current.children[part] = next
		} else if !next.isDir {
			return ErrNotADir
		}

		current = next
	}

	return nil
}

// ReadFile reads the content of the file located at the given path and returns its data as a byte slice.
func (m *MockVFS) ReadFile(path string) ([]byte, error) {
	cleanPath, err := m.validatePath(path)
	if err != nil {
		return nil, err
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	entry, err := m.getEntry(cleanPath)
	if err != nil {
		return nil, err
	}

	if entry.isDir {
		return nil, ErrNotAFile
	}

	// Return a copy to prevent external modification
	result := make([]byte, len(entry.content))
	copy(result, entry.content)
	return result, nil
}

// WriteFile writes the given content to the file located at the given path.
// It creates parent directories if they don't exist.
func (m *MockVFS) WriteFile(path string, content []byte) error {
	cleanPath, err := m.validatePath(path)
	if err != nil {
		return err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create parent directories
	dir := filepath.Dir(cleanPath)
	if dir != "." {
		if err := m.createDir(dir); err != nil {
			return err
		}
	}

	// Navigate to parent directory
	parts := strings.Split(filepath.ToSlash(cleanPath), "/")
	fileName := parts[len(parts)-1]

	var parent *fileEntry
	if len(parts) > 1 {
		parentPath := filepath.Join(parts[:len(parts)-1]...)
		parent, err = m.getEntry(parentPath)
		if err != nil {
			return err
		}
	} else {
		parent = m.root
	}

	if !parent.isDir {
		return ErrNotADir
	}

	// Create or update the file
	fileCopy := make([]byte, len(content))
	copy(fileCopy, content)

	parent.children[fileName] = &fileEntry{
		isDir:   false,
		content: fileCopy,
	}

	return nil
}

// DeleteFile deletes the file located at the given path.
// If recursive is true, directories and their contents are deleted.
// If force is true, it's included for API compatibility but doesn't affect in-memory operations.
func (m *MockVFS) DeleteFile(path string, recursive bool, force bool) error {
	cleanPath, err := m.validatePath(path)
	if err != nil {
		return err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if entry exists
	entry, err := m.getEntry(cleanPath)
	if err != nil {
		return err
	}

	// If it's a directory and recursive is false, return error
	if entry.isDir && !recursive {
		return ErrNotAFile
	}

	// Navigate to parent and remove the entry
	parts := strings.Split(filepath.ToSlash(cleanPath), "/")
	entryName := parts[len(parts)-1]

	var parent *fileEntry
	if len(parts) > 1 {
		parentPath := filepath.Join(parts[:len(parts)-1]...)
		parent, err = m.getEntry(parentPath)
		if err != nil {
			return err
		}
	} else {
		parent = m.root
	}

	delete(parent.children, entryName)
	return nil
}

// ListFiles lists all files and directories located at the given path.
// If recursive is true, it lists all files and directories recursively.
// Returns paths relative to the VFS root.
func (m *MockVFS) ListFiles(path string, recursive bool) ([]string, error) {
	cleanPath, err := m.validatePath(path)
	if err != nil {
		return nil, err
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	entry, err := m.getEntry(cleanPath)
	if err != nil {
		return nil, err
	}

	if !entry.isDir {
		return nil, ErrNotADir
	}

	var result []string

	if recursive {
		// Recursive listing
		var walk func(prefix string, e *fileEntry)
		walk = func(prefix string, e *fileEntry) {
			for name, child := range e.children {
				childPath := filepath.Join(prefix, name)
				result = append(result, childPath)

				if child.isDir {
					walk(childPath, child)
				}
			}
		}

		if cleanPath == "." || cleanPath == "" {
			walk("", entry)
		} else {
			walk(cleanPath, entry)
		}
	} else {
		// Non-recursive listing
		for name := range entry.children {
			if cleanPath == "." || cleanPath == "" {
				result = append(result, name)
			} else {
				result = append(result, filepath.Join(cleanPath, name))
			}
		}
	}

	return result, nil
}

// FindFiles searches for files and directories matching the given query.
// The query is matched using filepath.Match pattern.
// If recursive is true, it searches recursively from the root.
// Returns paths relative to the VFS root.
func (m *MockVFS) FindFiles(query string, recursive bool) ([]string, error) {
	if query == "" {
		return nil, ErrInvalidPath
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var result []string

	if recursive {
		// Recursive search
		var walk func(prefix string, e *fileEntry) error
		walk = func(prefix string, e *fileEntry) error {
			for name, child := range e.children {
				matched, err := filepath.Match(query, name)
				if err != nil {
					return err
				}

				childPath := filepath.Join(prefix, name)
				if matched {
					if prefix == "" {
						result = append(result, name)
					} else {
						result = append(result, childPath)
					}
				}

				if child.isDir {
					if err := walk(childPath, child); err != nil {
						return err
					}
				}
			}
			return nil
		}

		if err := walk("", m.root); err != nil {
			return nil, err
		}
	} else {
		// Non-recursive search
		for name := range m.root.children {
			matched, err := filepath.Match(query, name)
			if err != nil {
				return nil, err
			}

			if matched {
				result = append(result, name)
			}
		}
	}

	return result, nil
}

// MoveFile moves or renames a file or directory from src to dst.
// It works for both files and directories.
// Can be used for renaming by providing a different name in dst within the same directory.
func (m *MockVFS) MoveFile(src, dst string) error {
	cleanSrc, err := m.validatePath(src)
	if err != nil {
		return err
	}

	cleanDst, err := m.validatePath(dst)
	if err != nil {
		return err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if source exists
	srcEntry, err := m.getEntry(cleanSrc)
	if err != nil {
		return err
	}

	// Check if destination already exists
	_, err = m.getEntry(cleanDst)
	if err == nil {
		return ErrFileExists
	}
	if err != ErrFileNotFound {
		return err
	}

	// Create parent directory of destination if it doesn't exist
	dstDir := filepath.Dir(cleanDst)
	if dstDir != "." && dstDir != "" {
		if err := m.createDir(dstDir); err != nil {
			return err
		}
	}

	// Get the parent of source
	srcParts := strings.Split(filepath.ToSlash(cleanSrc), "/")
	srcName := srcParts[len(srcParts)-1]

	var srcParent *fileEntry
	if len(srcParts) > 1 {
		srcParentPath := filepath.Join(srcParts[:len(srcParts)-1]...)
		srcParent, err = m.getEntry(srcParentPath)
		if err != nil {
			return err
		}
	} else {
		srcParent = m.root
	}

	// Get the parent of destination
	dstParts := strings.Split(filepath.ToSlash(cleanDst), "/")
	dstName := dstParts[len(dstParts)-1]

	var dstParent *fileEntry
	if len(dstParts) > 1 {
		dstParentPath := filepath.Join(dstParts[:len(dstParts)-1]...)
		dstParent, err = m.getEntry(dstParentPath)
		if err != nil {
			return err
		}
	} else {
		dstParent = m.root
	}

	// Move the entry
	dstParent.children[dstName] = srcEntry
	delete(srcParent.children, srcName)

	return nil
}
