package vfs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitVCS implements the VCS interface for git repositories.
// It uses go-git library for git operations and manages worktrees for branches.
type GitVCS struct {
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

// NewGitRepo creates a new GitVCS instance from an existing git repository path.
// The worktreesPath parameter specifies the directory where worktrees will be created.
// The hidePatterns parameter specifies glob patterns for files and directories that should be hidden.
func NewGitRepo(path string, worktreesPath string, hidePatterns []string) (*GitVCS, error) {
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

	return &GitVCS{
		path:          absPath,
		worktreesPath: absWorktreesPath,
		repo:          repo,
		worktrees:     make(map[string]*gitWorktree),
		hidePatterns:  hidePatterns,
	}, nil
}

// GetWorktree extracts the worktree for the given branch and returns a VFS instance for it.
// If worktree is already extracted, it returns the existing VFS instance.
func (g *GitVCS) GetWorktree(branch string) (VFS, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Check if worktree already exists
	if wt, exists := g.worktrees[branch]; exists {
		return wt.vfs, nil
	}

	// Create worktree path using the configured worktreesPath and branch name.
	worktreePath := filepath.Join(g.worktreesPath, branch)

	// Check if branch exists
	_, err := g.repo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, fmt.Errorf("GitVCS.GetWorktree() [git.go]: branch %q not found: %w", branch, ErrFileNotFound)
		}
		return nil, fmt.Errorf("GitVCS.GetWorktree() [git.go]: %w", err)
	}

	if err := os.RemoveAll(worktreePath); err != nil {
		return nil, fmt.Errorf("GitVCS.GetWorktree() [git.go]: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return nil, fmt.Errorf("GitVCS.GetWorktree() [git.go]: %w", err)
	}

	if err := g.runGit("worktree", "add", "--force", worktreePath, branch); err != nil {
		_ = os.RemoveAll(worktreePath)
		return nil, fmt.Errorf("GitVCS.GetWorktree() [git.go]: %w", err)
	}

	localVFS, err := NewLocalVFS(worktreePath, g.hidePatterns)
	if err != nil {
		_ = g.runGit("worktree", "remove", "--force", worktreePath)
		_ = os.RemoveAll(worktreePath)
		return nil, fmt.Errorf("GitVCS.GetWorktree() [git.go]: %w", err)
	}
	localVFS.repo = g

	wt := &gitWorktree{
		branch: branch,
		path:   worktreePath,
		vfs:    localVFS,
	}

	g.worktrees[branch] = wt
	return wt.vfs, nil
}

// DropWorktree closes and removes the worktree for the given branch.
func (g *GitVCS) DropWorktree(branch string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	wt, exists := g.worktrees[branch]
	if !exists {
		return fmt.Errorf("GitVCS.DropWorktree() [git.go]: worktree for branch %q not found: %w", branch, ErrFileNotFound)
	}

	if err := g.runGit("worktree", "remove", "--force", wt.path); err != nil {
		return fmt.Errorf("GitVCS.DropWorktree() [git.go]: %w", err)
	}

	if err := os.RemoveAll(wt.path); err != nil {
		return fmt.Errorf("GitVCS.DropWorktree() [git.go]: %w", err)
	}

	delete(g.worktrees, branch)
	return nil
}

// CommitWorktree commits the changes in the worktree for the given branch.
func (g *GitVCS) CommitWorktree(branch string, message string) error {
	g.mutex.RLock()
	wt, exists := g.worktrees[branch]
	g.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("GitVCS.CommitWorktree() [git.go]: worktree for branch %q not found: %w", branch, ErrFileNotFound)
	}

	if err := g.runGitInWorktree(wt.path, "add", "-A"); err != nil {
		return fmt.Errorf("GitVCS.CommitWorktree() [git.go]: %w", err)
	}

	statusOutput, err := g.runGitOutputInWorktree(wt.path, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("GitVCS.CommitWorktree() [git.go]: %w", err)
	}

	if strings.TrimSpace(statusOutput) == "" {
		return fmt.Errorf("GitVCS.CommitWorktree() [git.go]: %w", ErrNoChangesToCommit)
	}

	if err := g.runGitInWorktree(wt.path, "commit", "-m", message); err != nil {
		return fmt.Errorf("GitVCS.CommitWorktree() [git.go]: %w", err)
	}

	return nil
}

// NewBranch creates a new branch from the given branch.
func (g *GitVCS) NewBranch(name string, from string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if from == "" {
		from = "HEAD"
	}

	if _, err := g.repo.ResolveRevision(plumbing.Revision(from)); err != nil {
		return fmt.Errorf("GitVCS.NewBranch() [git.go]: source branch %q not found: %w", from, ErrFileNotFound)
	}

	if err := g.runGit("show-ref", "--verify", "--quiet", "refs/heads/"+name); err == nil {
		return fmt.Errorf("GitVCS.NewBranch() [git.go]: branch %q already exists: %w", name, ErrFileExists)
	}

	if err := g.runGit("branch", name, from); err != nil {
		return fmt.Errorf("GitVCS.NewBranch() [git.go]: %w", err)
	}

	return nil
}

// DeleteBranch deletes the given branch.
func (g *GitVCS) DeleteBranch(name string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Cannot delete the current branch
	head, err := g.repo.Head()
	if err != nil {
		return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: %w", err)
	}

	branchRef := plumbing.NewBranchReferenceName(name)
	if head.Name() == branchRef {
		return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: cannot delete current branch: %w", ErrPermissionDenied)
	}

	// Check if branch exists
	_, err = g.repo.Reference(branchRef, true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: branch %q not found: %w", name, ErrFileNotFound)
		}
		return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: %w", err)
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
		return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: %w", err)
	}

	return nil
}

// ListBranches returns a list of all branches.
func (g *GitVCS) ListBranches(prefix string) ([]string, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	branches, err := g.repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("GitVCS.ListBranches() [git.go]: %w", err)
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
		return nil, fmt.Errorf("GitVCS.ListBranches() [git.go]: %w", err)
	}

	return result, nil
}

// MergeBranches merges the given branch into the current branch.
func (g *GitVCS) MergeBranches(into string, from string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Get references for both branches
	intoRef := plumbing.NewBranchReferenceName(into)
	fromRef := plumbing.NewBranchReferenceName(from)

	// Check if branches exist
	intoHash, err := g.repo.ResolveRevision(plumbing.Revision(intoRef))
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: target branch %q not found: %w", into, ErrFileNotFound)
	}

	fromHash, err := g.repo.ResolveRevision(plumbing.Revision(fromRef))
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: source branch %q not found: %w", from, ErrFileNotFound)
	}

	// Get the commits
	intoCommit, err := g.repo.CommitObject(*intoHash)
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}

	fromCommit, err := g.repo.CommitObject(*fromHash)
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
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
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
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
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}

	if commonAncestor == nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: no common ancestor found")
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
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}

	if fromAncestors[intoCommit.Hash] {
		// Fast-forward: move into to from
		newRef := plumbing.NewHashReference(intoRef, fromCommit.Hash)
		err = g.repo.Storer.SetReference(newRef)
		if err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
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
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}

	hash, err := g.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}

	// Update the branch reference
	newRef := plumbing.NewHashReference(intoRef, hash)
	err = g.repo.Storer.SetReference(newRef)
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}

	return nil
}

// Path returns the repository path
func (g *GitVCS) Path() string {
	return g.path
}

func (g *GitVCS) runGit(args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", g.path}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("GitVCS.runGit() [git.go]: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (g *GitVCS) runGitInWorktree(worktreePath string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", worktreePath}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("GitVCS.runGitInWorktree() [git.go]: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (g *GitVCS) runGitOutputInWorktree(worktreePath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", worktreePath}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("GitVCS.runGitOutputInWorktree() [git.go]: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
