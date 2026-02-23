package vfs

import (
	"errors"
	"fmt"
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

// MockVFS implements VFS interface with an in-memory filesystem.
// It behaves identically to LocalVFS but keeps all files in memory.
type MockVFS struct {
	root  *fileEntry
	mutex sync.RWMutex
}

func (m *MockVFS) GetBranch() string {
	return "worktree"
}

func (m *MockVFS) WorktreePath() string {
	return "/path/to/worktree"
}

func (m *MockVFS) GetRepo() VCS {
	return nil // TODO to be implemented
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
			return nil, fmt.Errorf("NewMockVFSFromDir() [mock.go]: %w", ErrFileNotFound)
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("NewMockVFSFromDir() [mock.go]: %w", ErrNotADir)
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
		return "", fmt.Errorf("MockVFS.validatePath() [mock.go]: %w", ErrInvalidPath)
	}

	// Clean the path to remove any .. or . components
	cleanPath := filepath.Clean(path)

	// Prevent absolute paths or paths that try to escape
	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("MockVFS.validatePath() [mock.go]: %w", ErrPermissionDenied)
	}

	// Check for path traversal attempts
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("MockVFS.validatePath() [mock.go]: %w", ErrPermissionDenied)
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
			return nil, fmt.Errorf("MockVFS.getEntry() [mock.go]: %w", ErrNotADir)
		}

		next, exists := current.children[part]
		if !exists {
			return nil, fmt.Errorf("MockVFS.getEntry() [mock.go]: %w", ErrFileNotFound)
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
			return fmt.Errorf("MockVFS.createDir() [mock.go]: %w", ErrNotADir)
		}

		next, exists := current.children[part]
		if !exists {
			next = &fileEntry{
				isDir:    true,
				children: make(map[string]*fileEntry),
			}
			current.children[part] = next
		} else if !next.isDir {
			return fmt.Errorf("MockVFS.createDir() [mock.go]: %w", ErrNotADir)
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
		return nil, fmt.Errorf("MockVFS.ReadFile() [mock.go]: %w", ErrNotAFile)
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
		return fmt.Errorf("MockVFS.WriteFile() [mock.go]: %w", ErrNotADir)
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
		return fmt.Errorf("MockVFS.DeleteFile() [mock.go]: %w", ErrNotAFile)
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
		return nil, fmt.Errorf("MockVFS.ListFiles() [mock.go]: %w", ErrNotADir)
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
// The query supports glob patterns:
//   - * matches any number of characters except /
//   - ? matches any single character except /
//   - [abc] matches any character in the set
//   - [a-z] matches any character in the range
//   - ** matches any number of characters including /
//
// If recursive is true, it searches recursively from the root.
// Returns paths relative to the VFS root.
func (m *MockVFS) FindFiles(query string, recursive bool) ([]string, error) {
	if query == "" {
		return nil, fmt.Errorf("MockVFS.FindFiles() [mock.go]: %w", ErrInvalidPath)
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var result []string

	if recursive {
		// Recursive search
		var walk func(prefix string, e *fileEntry) error
		walk = func(prefix string, e *fileEntry) error {
			for name, child := range e.children {
				childPath := filepath.Join(prefix, name)
				relPath := childPath
				if prefix == "" {
					relPath = name
				}

				matched, err := matchGlob(query, relPath)
				if err != nil {
					return err
				}

				if matched {
					result = append(result, relPath)
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
			matched, err := matchGlob(query, name)
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
		return fmt.Errorf("MockVFS.MoveFile() [mock.go]: %w", ErrFileExists)
	}
	// Check if error is something other than file not found (which we expect when the destination doesn't exist yet)
	// We use errors.Is since we wrapped the error with %w
	if !errors.Is(err, ErrFileNotFound) {
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

// branchState represents the state of a branch in the mock repository
type branchState struct {
	vfs     *MockVFS
	commits []commitInfo
}

// commitInfo represents a commit in the mock repository
type commitInfo struct {
	message string
	files   map[string][]byte
}

// MockVCSCommitCall stores information about a CommitWorktree invocation.
type MockVCSCommitCall struct {
	Branch  string
	Message string
}

// MockVCS is a lightweight VCS test double backed by MockVFS.
type MockVCS struct {
	mutex       sync.RWMutex
	worktrees   map[string]VFS
	commitCalls []MockVCSCommitCall
	dropCalls   []string
	commitErr   error
	dropErr     error
}

// NewMockVCS creates a new MockVCS with an optional base VFS.
func NewMockVCS(base VFS) *MockVCS {
	if base == nil {
		base = NewMockVFS()
	}

	return &MockVCS{
		worktrees: map[string]VFS{"": base},
	}
}

// SetCommitError configures CommitWorktree to return the provided error.
func (m *MockVCS) SetCommitError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.commitErr = err
}

// SetDropError configures DropWorktree to return the provided error.
func (m *MockVCS) SetDropError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.dropErr = err
}

// GetCommitCalls returns all recorded commit calls.
func (m *MockVCS) GetCommitCalls() []MockVCSCommitCall {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	calls := make([]MockVCSCommitCall, len(m.commitCalls))
	copy(calls, m.commitCalls)
	return calls
}

// GetDropCalls returns all recorded drop calls.
func (m *MockVCS) GetDropCalls() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	calls := make([]string, len(m.dropCalls))
	copy(calls, m.dropCalls)
	return calls
}

// GetWorktree returns a worktree VFS for the provided branch.
func (m *MockVCS) GetWorktree(branch string) (VFS, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	v, ok := m.worktrees[branch]
	if ok {
		return v, nil
	}
	v = NewMockVFS()
	m.worktrees[branch] = v
	return v, nil
}

// DropWorktree drops a worktree by branch.
func (m *MockVCS) DropWorktree(branch string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.dropCalls = append(m.dropCalls, branch)
	if m.dropErr != nil {
		return m.dropErr
	}
	delete(m.worktrees, branch)
	return nil
}

// CommitWorktree records commit call and returns configured error if any.
func (m *MockVCS) CommitWorktree(branch string, message string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.commitCalls = append(m.commitCalls, MockVCSCommitCall{Branch: branch, Message: message})
	if m.commitErr != nil {
		return m.commitErr
	}
	return nil
}

// NewBranch is a no-op for MockVCS.
func (m *MockVCS) NewBranch(name string, from string) error {
	return nil
}

// DeleteBranch is a no-op for MockVCS.
func (m *MockVCS) DeleteBranch(name string) error {
	return nil
}

// ListBranches returns known worktree branches.
func (m *MockVCS) ListBranches(prefix string) ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make([]string, 0, len(m.worktrees))
	for branch := range m.worktrees {
		if prefix == "" || strings.HasPrefix(branch, prefix) {
			result = append(result, branch)
		}
	}
	return result, nil
}

// MergeBranches is a no-op for MockVCS.
func (m *MockVCS) MergeBranches(into string, from string) error {
	return nil
}

// MockRepo implements the VCS interface with an in-memory git-like repository.
// It emulates git behavior using branches and worktrees stored in memory.
type MockRepo struct {
	branches  map[string]*branchState
	worktrees map[string]*MockVFS
	mutex     sync.RWMutex
}

// NewMockRepo creates a new MockRepo instance with a default "main" branch.
func NewMockRepo() *MockRepo {
	mainVFS := NewMockVFS()
	return &MockRepo{
		branches: map[string]*branchState{
			"main": {
				vfs:     mainVFS,
				commits: []commitInfo{},
			},
		},
		worktrees: make(map[string]*MockVFS),
	}
}

// GetWorktree extracts the worktree for the given branch and returns a VFS instance for it.
// If worktree is already extracted, it returns the existing VFS instance.
func (m *MockRepo) GetWorktree(branch string) (VFS, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if worktree already exists
	if vfs, exists := m.worktrees[branch]; exists {
		return vfs, nil
	}

	// Check if branch exists
	branchState, exists := m.branches[branch]
	if !exists {
		return nil, fmt.Errorf("MockRepo.GetWorktree() [mock.go]: branch %q not found: %w", branch, ErrFileNotFound)
	}

	// Create a copy of the branch's VFS for the worktree
	worktreeVFS := m.copyVFS(branchState.vfs)
	m.worktrees[branch] = worktreeVFS

	return worktreeVFS, nil
}

// copyVFS creates a deep copy of a MockVFS
func (m *MockRepo) copyVFS(source *MockVFS) *MockVFS {
	dest := NewMockVFS()

	source.mutex.RLock()
	defer source.mutex.RUnlock()

	var copyEntry func(src *fileEntry, dst *fileEntry)
	copyEntry = func(src *fileEntry, dst *fileEntry) {
		dst.isDir = src.isDir
		if src.content != nil {
			dst.content = make([]byte, len(src.content))
			copy(dst.content, src.content)
		}
		if src.children != nil {
			dst.children = make(map[string]*fileEntry)
			for name, child := range src.children {
				dst.children[name] = &fileEntry{}
				copyEntry(child, dst.children[name])
			}
		}
	}

	copyEntry(source.root, dest.root)
	return dest
}

// DropWorktree closes and removes the worktree for the given branch.
func (m *MockRepo) DropWorktree(branch string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.worktrees[branch]; !exists {
		return fmt.Errorf("MockRepo.DropWorktree() [mock.go]: worktree for branch %q not found: %w", branch, ErrFileNotFound)
	}

	delete(m.worktrees, branch)
	return nil
}

// CommitWorktree commits the changes in the worktree for the given branch.
func (m *MockRepo) CommitWorktree(branch string, message string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	worktreeVFS, exists := m.worktrees[branch]
	if !exists {
		return fmt.Errorf("MockRepo.CommitWorktree() [mock.go]: worktree for branch %q not found: %w", branch, ErrFileNotFound)
	}

	branchState, exists := m.branches[branch]
	if !exists {
		return fmt.Errorf("MockRepo.CommitWorktree() [mock.go]: branch %q not found: %w", branch, ErrFileNotFound)
	}

	// Get all files from the worktree
	files, err := worktreeVFS.ListFiles(".", true)
	if err != nil {
		return fmt.Errorf("MockRepo.CommitWorktree() [mock.go]: %w", err)
	}

	// Create a snapshot of all files
	fileSnapshot := make(map[string][]byte)
	for _, file := range files {
		content, err := worktreeVFS.ReadFile(file)
		if err != nil {
			continue // Skip directories
		}
		fileSnapshot[file] = content
	}

	// Add the commit
	commit := commitInfo{
		message: message,
		files:   fileSnapshot,
	}
	branchState.commits = append(branchState.commits, commit)

	// Update the branch's VFS with the committed state
	branchState.vfs = m.copyVFS(worktreeVFS)

	return nil
}

// NewBranch creates a new branch from the given branch.
func (m *MockRepo) NewBranch(name string, from string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if source branch exists
	sourceBranch, exists := m.branches[from]
	if !exists {
		return fmt.Errorf("MockRepo.NewBranch() [mock.go]: source branch %q not found: %w", from, ErrFileNotFound)
	}

	// Check if target branch already exists
	if _, exists := m.branches[name]; exists {
		return fmt.Errorf("MockRepo.NewBranch() [mock.go]: branch %q already exists: %w", name, ErrFileExists)
	}

	// Create new branch as a copy of the source branch
	m.branches[name] = &branchState{
		vfs:     m.copyVFS(sourceBranch.vfs),
		commits: append([]commitInfo{}, sourceBranch.commits...),
	}

	return nil
}

// DeleteBranch deletes the given branch.
func (m *MockRepo) DeleteBranch(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if branch exists first
	if _, exists := m.branches[name]; !exists {
		return fmt.Errorf("MockRepo.DeleteBranch() [mock.go]: branch %q not found: %w", name, ErrFileNotFound)
	}

	// Cannot delete if it's the only branch
	if len(m.branches) == 1 {
		return fmt.Errorf("MockRepo.DeleteBranch() [mock.go]: cannot delete the only branch: %w", ErrPermissionDenied)
	}

	// Remove worktree if it exists
	delete(m.worktrees, name)

	// Delete the branch
	delete(m.branches, name)

	return nil
}

// ListBranches returns a list of all branches.
func (m *MockRepo) ListBranches(prefix string) ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var result []string
	for name := range m.branches {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			result = append(result, name)
		}
	}

	return result, nil
}

// MergeBranches merges the given branch into the current branch.
func (m *MockRepo) MergeBranches(into string, from string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if both branches exist
	intoBranch, exists := m.branches[into]
	if !exists {
		return fmt.Errorf("MockRepo.MergeBranches() [mock.go]: target branch %q not found: %w", into, ErrFileNotFound)
	}

	fromBranch, exists := m.branches[from]
	if !exists {
		return fmt.Errorf("MockRepo.MergeBranches() [mock.go]: source branch %q not found: %w", from, ErrFileNotFound)
	}

	// Merge files from source branch into target branch
	// Get all files from the source branch
	files, err := fromBranch.vfs.ListFiles(".", true)
	if err != nil {
		return fmt.Errorf("MockRepo.MergeBranches() [mock.go]: %w", err)
	}

	for _, file := range files {
		content, err := fromBranch.vfs.ReadFile(file)
		if err != nil {
			continue // Skip directories
		}
		if err := intoBranch.vfs.WriteFile(file, content); err != nil {
			return fmt.Errorf("MockRepo.MergeBranches() [mock.go]: %w", err)
		}
	}

	// Add a merge commit
	intoBranch.commits = append(intoBranch.commits, commitInfo{
		message: fmt.Sprintf("Merge branch '%s' into %s", from, into),
		files:   make(map[string][]byte),
	})

	return nil
}
