package vfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitRepo implements the VCS interface for git repositories.
// It uses go-git library for git operations and manages worktrees for branches.
type GitRepo struct {
	path          string
	worktreesPath string
	repo          *git.Repository
	worktrees     map[string]*gitWorktree
	mutex         sync.RWMutex
	hidePatterns  []string
}

// gitWorktree represents a worktree for a specific branch
type gitWorktree struct {
	branch string
	path   string
	vfs    VFS
}

// NewGitRepo creates a new GitRepo instance from an existing git repository path.
// The worktreesPath parameter specifies the directory where worktrees will be created.
// The hidePatterns parameter specifies glob patterns for files and directories that should be hidden.
func NewGitRepo(path string, worktreesPath string, hidePatterns []string) (*GitRepo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("NewGitRepo() [git.go]: %w", err)
	}

	// Check if path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("NewGitRepo() [git.go]: %w", ErrFileNotFound)
		}
		return nil, fmt.Errorf("NewGitRepo() [git.go]: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("NewGitRepo() [git.go]: %w", ErrNotADir)
	}

	// Open the git repository
	repo, err := git.PlainOpen(absPath)
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, fmt.Errorf("NewGitRepo() [git.go]: %w", ErrFileNotFound)
		}
		return nil, fmt.Errorf("NewGitRepo() [git.go]: %w", err)
	}

	// Resolve worktreesPath to absolute path
	absWorktreesPath, err := filepath.Abs(worktreesPath)
	if err != nil {
		return nil, fmt.Errorf("NewGitRepo() [git.go]: %w", err)
	}

	return &GitRepo{
		path:          absPath,
		worktreesPath: absWorktreesPath,
		repo:          repo,
		worktrees:     make(map[string]*gitWorktree),
		hidePatterns:  hidePatterns,
	}, nil
}

// GetWorktree extracts the worktree for the given branch and returns a VFS instance for it.
// If worktree is already extracted, it returns the existing VFS instance.
func (g *GitRepo) GetWorktree(branch string) (VFS, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Check if worktree already exists
	if wt, exists := g.worktrees[branch]; exists {
		return wt.vfs, nil
	}

	// Create worktree path using the configured worktreesPath and branch name
	// Branch names with '/' will create nested directories
	worktreePath := filepath.Join(g.worktreesPath, branch)
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		return nil, fmt.Errorf("GitRepo.GetWorktree() [git.go]: %w", err)
	}

	// Get the branch reference
	branchRef := plumbing.NewBranchReferenceName(branch)

	// Check if branch exists
	_, err := g.repo.Reference(branchRef, true)
	if err != nil {
		// Clean up the worktree directory we created
		os.RemoveAll(worktreePath)
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, fmt.Errorf("GitRepo.GetWorktree() [git.go]: branch %q not found: %w", branch, ErrFileNotFound)
		}
		return nil, fmt.Errorf("GitRepo.GetWorktree() [git.go]: %w", err)
	}

	// For go-git, we'll create a worktree by checking out the branch to a separate directory
	// Since go-git doesn't have full worktree support, we'll simulate it by creating a separate VFS
	// that points to the worktree directory and checkout the branch there

	// First, let's check if this is the main worktree (master/main branch)
	var wt *gitWorktree

	if branch == "master" || branch == "main" {
		// For main/master branch, use the repository root directly
		localVFS, err := NewLocalVFS(g.path, g.hidePatterns)
		if err != nil {
			os.RemoveAll(worktreePath)
			return nil, fmt.Errorf("GitRepo.GetWorktree() [git.go]: %w", err)
		}
		// Set the repo reference
		localVFS.repo = g

		wt = &gitWorktree{
			branch: branch,
			path:   g.path,
			vfs:    localVFS,
		}
	} else {
		// For other branches, create a worktree directory and checkout
		// Remove any existing content
		os.RemoveAll(worktreePath)
		if err := os.MkdirAll(worktreePath, 0755); err != nil {
			return nil, fmt.Errorf("GitRepo.GetWorktree() [git.go]: %w", err)
		}

		// Create a new worktree by checking out the branch
		w, err := g.repo.Worktree()
		if err != nil {
			os.RemoveAll(worktreePath)
			return nil, fmt.Errorf("GitRepo.GetWorktree() [git.go]: %w", err)
		}

		// Checkout the branch to the worktree directory
		err = w.Checkout(&git.CheckoutOptions{
			Branch: branchRef,
			Force:  true,
		})
		if err != nil {
			// Try to checkout by creating the branch if it doesn't exist locally
			// but exists remotely
			err = w.Checkout(&git.CheckoutOptions{
				Branch: branchRef,
				Create: false,
				Force:  true,
			})
			if err != nil {
				os.RemoveAll(worktreePath)
				return nil, fmt.Errorf("GitRepo.GetWorktree() [git.go]: %w", err)
			}
		}

		localVFS, err := NewLocalVFS(worktreePath, g.hidePatterns)
		if err != nil {
			os.RemoveAll(worktreePath)
			return nil, fmt.Errorf("GitRepo.GetWorktree() [git.go]: %w", err)
		}
		localVFS.repo = g

		wt = &gitWorktree{
			branch: branch,
			path:   worktreePath,
			vfs:    localVFS,
		}
	}

	g.worktrees[branch] = wt
	return wt.vfs, nil
}

// DropWorktree closes and removes the worktree for the given branch.
func (g *GitRepo) DropWorktree(branch string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	wt, exists := g.worktrees[branch]
	if !exists {
		return fmt.Errorf("GitRepo.DropWorktree() [git.go]: worktree for branch %q not found: %w", branch, ErrFileNotFound)
	}

	// Remove the worktree directory if it's not the main repository
	if wt.path != g.path {
		if err := os.RemoveAll(wt.path); err != nil {
			return fmt.Errorf("GitRepo.DropWorktree() [git.go]: %w", err)
		}
	}

	delete(g.worktrees, branch)
	return nil
}

// CommitWorktree commits the changes in the worktree for the given branch.
func (g *GitRepo) CommitWorktree(branch string, message string) error {
	g.mutex.RLock()
	wt, exists := g.worktrees[branch]
	g.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("GitRepo.CommitWorktree() [git.go]: worktree for branch %q not found: %w", branch, ErrFileNotFound)
	}

	w, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("GitRepo.CommitWorktree() [git.go]: %w", err)
	}

	// Add all changes
	_, err = w.Add(".")
	if err != nil {
		return fmt.Errorf("GitRepo.CommitWorktree() [git.go]: %w", err)
	}

	// Get the status to check if there are changes
	status, err := w.Status()
	if err != nil {
		return fmt.Errorf("GitRepo.CommitWorktree() [git.go]: %w", err)
	}

	if status.IsClean() {
		return fmt.Errorf("GitRepo.CommitWorktree() [git.go]: no changes to commit")
	}

	// Commit the changes
	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "CodeSnort",
			Email: "codesnort@example.com",
		},
	})
	if err != nil {
		return fmt.Errorf("GitRepo.CommitWorktree() [git.go]: %w", err)
	}

	// Update the worktree reference
	wt.vfs = nil
	localVFS, err := NewLocalVFS(g.path, g.hidePatterns)
	if err != nil {
		return fmt.Errorf("GitRepo.CommitWorktree() [git.go]: %w", err)
	}
	localVFS.repo = g
	wt.vfs = localVFS

	return nil
}

// NewBranch creates a new branch from the given branch.
func (g *GitRepo) NewBranch(name string, from string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Get the reference for the source branch
	fromRef := plumbing.NewBranchReferenceName(from)
	ref, err := g.repo.Reference(fromRef, true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return fmt.Errorf("GitRepo.NewBranch() [git.go]: source branch %q not found: %w", from, ErrFileNotFound)
		}
		return fmt.Errorf("GitRepo.NewBranch() [git.go]: %w", err)
	}

	// Create the new branch reference
	newRef := plumbing.NewBranchReferenceName(name)

	// Check if branch already exists
	_, err = g.repo.Reference(newRef, true)
	if err == nil {
		return fmt.Errorf("GitRepo.NewBranch() [git.go]: branch %q already exists: %w", name, ErrFileExists)
	}

	// Create the new branch
	newBranchRef := plumbing.NewHashReference(newRef, ref.Hash())
	err = g.repo.Storer.SetReference(newBranchRef)
	if err != nil {
		return fmt.Errorf("GitRepo.NewBranch() [git.go]: %w", err)
	}

	return nil
}

// DeleteBranch deletes the given branch.
func (g *GitRepo) DeleteBranch(name string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Cannot delete the current branch
	head, err := g.repo.Head()
	if err != nil {
		return fmt.Errorf("GitRepo.DeleteBranch() [git.go]: %w", err)
	}

	branchRef := plumbing.NewBranchReferenceName(name)
	if head.Name() == branchRef {
		return fmt.Errorf("GitRepo.DeleteBranch() [git.go]: cannot delete current branch: %w", ErrPermissionDenied)
	}

	// Check if branch exists
	_, err = g.repo.Reference(branchRef, true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return fmt.Errorf("GitRepo.DeleteBranch() [git.go]: branch %q not found: %w", name, ErrFileNotFound)
		}
		return fmt.Errorf("GitRepo.DeleteBranch() [git.go]: %w", err)
	}

	// Remove the worktree if it exists
	if wt, exists := g.worktrees[name]; exists {
		if wt.path != g.path {
			os.RemoveAll(wt.path)
		}
		delete(g.worktrees, name)
	}

	// Delete the branch reference
	err = g.repo.Storer.RemoveReference(branchRef)
	if err != nil {
		return fmt.Errorf("GitRepo.DeleteBranch() [git.go]: %w", err)
	}

	return nil
}

// ListBranches returns a list of all branches.
func (g *GitRepo) ListBranches(prefix string) ([]string, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	branches, err := g.repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("GitRepo.ListBranches() [git.go]: %w", err)
	}

	var result []string
	err = branches.ForEach(func(ref *plumbing.Reference) error {
		branchName := ref.Name().Short()
		if prefix == "" || strings.HasPrefix(branchName, prefix) {
			result = append(result, branchName)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("GitRepo.ListBranches() [git.go]: %w", err)
	}

	return result, nil
}

// MergeBranches merges the given branch into the current branch.
func (g *GitRepo) MergeBranches(into string, from string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Get references for both branches
	intoRef := plumbing.NewBranchReferenceName(into)
	fromRef := plumbing.NewBranchReferenceName(from)

	// Check if branches exist
	intoHash, err := g.repo.ResolveRevision(plumbing.Revision(intoRef))
	if err != nil {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: target branch %q not found: %w", into, ErrFileNotFound)
	}

	fromHash, err := g.repo.ResolveRevision(plumbing.Revision(fromRef))
	if err != nil {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: source branch %q not found: %w", from, ErrFileNotFound)
	}

	// Get the commits
	intoCommit, err := g.repo.CommitObject(*intoHash)
	if err != nil {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: %w", err)
	}

	fromCommit, err := g.repo.CommitObject(*fromHash)
	if err != nil {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: %w", err)
	}

	// Find common ancestor
	intoIter := object.NewCommitPreorderIter(intoCommit, nil, nil)
	fromIter := object.NewCommitPreorderIter(fromCommit, nil, nil)

	intoAncestors := make(map[plumbing.Hash]bool)
	err = intoIter.ForEach(func(c *object.Commit) error {
		intoAncestors[c.Hash] = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: %w", err)
	}

	var commonAncestor *object.Commit
	err = fromIter.ForEach(func(c *object.Commit) error {
		if intoAncestors[c.Hash] {
			commonAncestor = c
			return errors.New("found")
		}
		return nil
	})
	if err != nil && err.Error() != "found" {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: %w", err)
	}

	if commonAncestor == nil {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: no common ancestor found")
	}

	// Create a merge commit
	// For simplicity, we'll do a fast-forward merge if possible
	// Otherwise, create a merge commit

	// Check if fast-forward is possible
	if intoAncestors[fromCommit.Hash] {
		// Already merged
		return nil
	}

	// Check if fast-forward is possible (from is descendant of into)
	fromAncestors := make(map[plumbing.Hash]bool)
	err = fromIter.ForEach(func(c *object.Commit) error {
		fromAncestors[c.Hash] = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: %w", err)
	}

	if fromAncestors[intoCommit.Hash] {
		// Fast-forward: move into to from
		newRef := plumbing.NewHashReference(intoRef, fromCommit.Hash)
		err = g.repo.Storer.SetReference(newRef)
		if err != nil {
			return fmt.Errorf("GitRepo.MergeBranches() [git.go]: %w", err)
		}
		return nil
	}

	// Create a merge commit
	mergeCommit := &object.Commit{
		Author: object.Signature{
			Name:  "CodeSnort",
			Email: "codesnort@example.com",
		},
		Committer: object.Signature{
			Name:  "CodeSnort",
			Email: "codesnort@example.com",
		},
		Message:  fmt.Sprintf("Merge branch '%s' into %s", from, into),
		TreeHash: fromCommit.TreeHash,
		ParentHashes: []plumbing.Hash{
			intoCommit.Hash,
			fromCommit.Hash,
		},
	}

	// Store the commit
	obj := g.repo.Storer.NewEncodedObject()
	err = mergeCommit.Encode(obj)
	if err != nil {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: %w", err)
	}

	hash, err := g.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: %w", err)
	}

	// Update the branch reference
	newRef := plumbing.NewHashReference(intoRef, hash)
	err = g.repo.Storer.SetReference(newRef)
	if err != nil {
		return fmt.Errorf("GitRepo.MergeBranches() [git.go]: %w", err)
	}

	return nil
}

// Path returns the repository path
func (g *GitRepo) Path() string {
	return g.path
}
