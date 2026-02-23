package vfs

// NullVCS represents a plain unversioned directory.
// It satisfies the VCS interface but has no actual version control functionality.
type NullVCS struct {
	vfs VFS
}

// NewNullVCS creates a new NullVCS instance with the given VFS.
// It accepts a VFS and returns a pointer to NullVCS or an error.
func NewNullVFS(vfs VFS) (*NullVCS, error) {
	return &NullVCS{vfs: vfs}, nil
}

// GetWorktree returns the VFS for the given root path.
// For NullVCS, it simply returns the underlying VFS.
func (n *NullVCS) GetWorktree(branch string) (VFS, error) {
	return n.vfs, nil
}

// DropWorktree has no effect for NullVCS.
func (n *NullVCS) DropWorktree(branch string) error {
	return nil
}

// CommitWorktree has no effect for NullVCS.
func (n *NullVCS) CommitWorktree(branch string, message string) error {
	return nil
}

// NewBranch has no effect for NullVCS.
func (n *NullVCS) NewBranch(name string, from string) error {
	return nil
}

// DeleteBranch has no effect for NullVCS.
func (n *NullVCS) DeleteBranch(name string) error {
	return nil
}

// ListBranches returns a list of all branches.
// For NullVCS, it always returns ["main"] as the default branch.
func (n *NullVCS) ListBranches(prefix string) ([]string, error) {
	return []string{"main"}, nil
}

// MergeBranches has no effect for NullVCS.
func (n *NullVCS) MergeBranches(into string, from string) error {
	return nil
}
