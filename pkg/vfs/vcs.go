package vfs

// VCS represents a repository for VFS. It can be a local filesystem, a remote filesystem, a git repository etc.
// This interface is responsible only for repository operations (cloning, pulling, pushing, status, extracting worktree etc.)
type VCS interface {
	// GetWorktree extracts the worktree for the given branch and returns a VFS instance for it.
	// if worktree is already extracted, it returns the existing VFS instance.
	GetWorktree(branch string) (VFS, error)

	// DropWorktree closes and removes the worktree for the given branch.
	DropWorktree(branch string) error

	// CommitWorktree commits the changes in the worktree for the given branch.
	CommitWorktree(branch string, message string) error

	// NewBranch creates a new branch from the given branch.
	NewBranch(name string, from string) error

	// DeleteBranch deletes the given branch.
	DeleteBranch(name string) error

	// ListBranches returns a list of all branches. Default branch is first in the list.
	ListBranches(prefix string) ([]string, error)

	// ListWorktrees returns a list of all worktree branch names that are currently extracted.
	ListWorktrees() ([]string, error)

	// MergeBranches merges the given branch into the current branch.
	MergeBranches(into string, from string) error
}
