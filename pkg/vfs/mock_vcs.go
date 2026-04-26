package vfs

import (
	"strings"
	"sync"

	"github.com/rlewczuk/csw/pkg/apis"
)

// MockVCSCommitCall stores information about a CommitWorktree invocation.
type MockVCSCommitCall struct {
	Branch  string
	Message string
}

// MockVCSMergeCall stores information about a MergeBranches invocation.
type MockVCSMergeCall struct {
	Into string
	From string
}

// MockVCS is a lightweight VCS test double backed by MockVFS.
type MockVCS struct {
	mutex       sync.RWMutex
	worktrees   map[string]apis.VFS
	commitCalls []MockVCSCommitCall
	dropCalls   []string
	mergeCalls  []MockVCSMergeCall
	deleteCalls []string
	commitErr   error
	dropErr     error
	mergeErr    error
	deleteErr   error
}

// NewMockVCS creates a new MockVCS with an optional base VFS.
func NewMockVCS(base apis.VFS) *MockVCS {
	if base == nil {
		base = NewMockVFS()
	}

	return &MockVCS{
		worktrees: map[string]apis.VFS{"": base},
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

// SetMergeError configures MergeBranches to return the provided error.
func (m *MockVCS) SetMergeError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.mergeErr = err
}

// SetDeleteError configures DeleteBranch to return the provided error.
func (m *MockVCS) SetDeleteError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.deleteErr = err
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

// GetMergeCalls returns all recorded merge calls.
func (m *MockVCS) GetMergeCalls() []MockVCSMergeCall {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	calls := make([]MockVCSMergeCall, len(m.mergeCalls))
	copy(calls, m.mergeCalls)
	return calls
}

// GetDeleteCalls returns all recorded delete calls.
func (m *MockVCS) GetDeleteCalls() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	calls := make([]string, len(m.deleteCalls))
	copy(calls, m.deleteCalls)
	return calls
}

// GetWorktree returns a worktree VFS for the provided branch.
func (m *MockVCS) GetWorktree(branch string) (apis.VFS, error) {
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
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.deleteCalls = append(m.deleteCalls, name)
	if m.deleteErr != nil {
		return m.deleteErr
	}
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

// ListWorktrees returns the list of worktree branches.
func (m *MockVCS) ListWorktrees() ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make([]string, 0, len(m.worktrees))
	for branch := range m.worktrees {
		if branch != "" { // Skip the empty string entry (base VFS)
			result = append(result, branch)
		}
	}
	return result, nil
}

// MergeBranches is a no-op for MockVCS.
func (m *MockVCS) MergeBranches(into string, from string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.mergeCalls = append(m.mergeCalls, MockVCSMergeCall{Into: into, From: from})
	if m.mergeErr != nil {
		return m.mergeErr
	}
	return nil
}
