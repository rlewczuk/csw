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
	gitUserName   string
	gitUserEmail  string
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
func NewGitRepo(path string, worktreesPath string, hidePatterns []string, name string, email string) (*GitVCS, error) {
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
		gitUserName:   name,
		gitUserEmail:  email,
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
	worktreePath := filepath.Join(g.worktreesPath, branch)
	if exists {
		worktreePath = wt.path
	} else {
		worktreeInfo, err := os.Stat(worktreePath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("GitVCS.DropWorktree() [git.go]: worktree for branch %q not found: %w", branch, ErrFileNotFound)
			}
			return fmt.Errorf("GitVCS.DropWorktree() [git.go]: %w", err)
		}
		if !worktreeInfo.IsDir() {
			return fmt.Errorf("GitVCS.DropWorktree() [git.go]: worktree for branch %q not found: %w", branch, ErrFileNotFound)
		}
	}

	if err := g.runGit("worktree", "remove", "--force", worktreePath); err != nil {
		return fmt.Errorf("GitVCS.DropWorktree() [git.go]: %w", err)
	}

	if err := os.RemoveAll(worktreePath); err != nil {
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

// ListWorktrees returns a list of all worktree branch names that are currently extracted.
func (g *GitVCS) ListWorktrees() ([]string, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	// Read the worktrees directory to find extracted worktrees
	entries, err := os.ReadDir(g.worktreesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("GitVCS.ListWorktrees() [git.go]: %w", err)
	}

	var result []string
	for _, entry := range entries {
		if entry.IsDir() {
			result = append(result, entry.Name())
		}
	}

	return result, nil
}

// MergeBranches merges the given branch into the current branch.
func (g *GitVCS) MergeBranches(into string, from string) error {
	if _, err := g.repo.ResolveRevision(plumbing.Revision(plumbing.NewBranchReferenceName(into))); err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: target branch %q not found: %w", into, ErrFileNotFound)
	}

	if _, err := g.repo.ResolveRevision(plumbing.Revision(plumbing.NewBranchReferenceName(from))); err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: source branch %q not found: %w", from, ErrFileNotFound)
	}

	g.mutex.RLock()
	intoWorktree, hasIntoWorktree := g.worktrees[into]
	fromWorktree, hasFromWorktree := g.worktrees[from]
	g.mutex.RUnlock()

	currentPrimaryBranch, err := g.currentBranchInWorktree(g.path)
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}

	intoMergePath := g.path
	intoCleanup := func() {}
	if hasIntoWorktree {
		intoMergePath = intoWorktree.path
	} else if currentPrimaryBranch != into {
		tempIntoWorktreePath := filepath.Join(g.worktreesPath, ".merge-into-"+strings.ReplaceAll(into, "/", "_"))
		if err := os.RemoveAll(tempIntoWorktreePath); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(tempIntoWorktreePath), 0755); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
		}
		if err := g.runGit("worktree", "add", "--force", tempIntoWorktreePath, into); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
		}
		intoMergePath = tempIntoWorktreePath
		intoCleanup = func() {
			_ = g.runGit("worktree", "remove", "--force", tempIntoWorktreePath)
			_ = os.RemoveAll(tempIntoWorktreePath)
		}
	}
	defer intoCleanup()

	fromMergePath := g.path
	fromCleanup := func() {}
	if hasFromWorktree {
		fromMergePath = fromWorktree.path
	} else if currentPrimaryBranch != from {
		tempFromWorktreePath := filepath.Join(g.worktreesPath, ".merge-from-"+strings.ReplaceAll(from, "/", "_"))
		if err := os.RemoveAll(tempFromWorktreePath); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(tempFromWorktreePath), 0755); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
		}
		if err := g.runGit("worktree", "add", "--force", tempFromWorktreePath, from); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
		}
		fromMergePath = tempFromWorktreePath
		fromCleanup = func() {
			_ = g.runGit("worktree", "remove", "--force", tempFromWorktreePath)
			_ = os.RemoveAll(tempFromWorktreePath)
		}
	}
	defer fromCleanup()

	if err := g.runGitInWorktree(fromMergePath, "rebase", into); err != nil {
		errText := err.Error()
		if strings.Contains(errText, "CONFLICT") || strings.Contains(errText, "would be overwritten by merge") {
			_ = g.runGitInWorktree(fromMergePath, "rebase", "--abort")
			return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w: %w", err, ErrMergeConflict)
		}
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}

	if err := g.runGitInWorktree(intoMergePath, "merge", "--ff-only", from); err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}

	if intoMergePath != g.path && currentPrimaryBranch == into {
		if err := g.runGitInWorktree(g.path, "reset", "--hard", into); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
		}
	}

	return nil
}

func (g *GitVCS) currentBranchInWorktree(worktreePath string) (string, error) {
	branchOutput, err := g.runGitOutputInWorktree(worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("GitVCS.currentBranchInWorktree() [git.go]: %w", err)
	}

	return strings.TrimSpace(branchOutput), nil
}

// Path returns the repository path
func (g *GitVCS) Path() string {
	return g.path
}

func (g *GitVCS) runGit(args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", g.path}, args...)...)
	cmd.Env = g.gitCommandEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("GitVCS.runGit() [git.go]: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (g *GitVCS) runGitInWorktree(worktreePath string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", worktreePath}, args...)...)
	cmd.Env = g.gitCommandEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("GitVCS.runGitInWorktree() [git.go]: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (g *GitVCS) runGitOutputInWorktree(worktreePath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", worktreePath}, args...)...)
	cmd.Env = g.gitCommandEnv()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("GitVCS.runGitOutputInWorktree() [git.go]: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// gitCommandEnv returns environment variables with default identity for git commits.
func (g *GitVCS) gitCommandEnv() []string {
	gitUserName := g.gitUserName
	if gitUserName == "" {
		gitUserName = "CSW"
	}

	gitUserEmail := g.gitUserEmail
	if gitUserEmail == "" {
		gitUserEmail = "csw@example.com"
	}

	env := os.Environ()
	env = upsertEnvValue(env, "GIT_AUTHOR_NAME", gitUserName)
	env = upsertEnvValue(env, "GIT_AUTHOR_EMAIL", gitUserEmail)
	env = upsertEnvValue(env, "GIT_COMMITTER_NAME", gitUserName)
	env = upsertEnvValue(env, "GIT_COMMITTER_EMAIL", gitUserEmail)
	return env
}

// upsertEnvValue sets a key-value env pair, replacing any existing key entry.
func upsertEnvValue(env []string, key string, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
