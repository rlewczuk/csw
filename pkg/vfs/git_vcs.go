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
)

// GitVCS implements the VCS interface for git repositories.
// It uses git command-line calls and manages worktrees for branches.
type GitVCS struct {
	path          string
	worktreesPath string
	worktrees     map[string]*gitWorktree
	mutex         sync.RWMutex
	hidePatterns  []string
	allowedPaths  []string
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
// The allowedPaths parameter specifies additional absolute paths allowed outside of each worktree.
func NewGitRepo(path string, worktreesPath string, hidePatterns []string, allowedPaths []string, name string, email string) (*GitVCS, error) {
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

	// Resolve worktreesPath to absolute path
	absWorktreesPath, err := filepath.Abs(worktreesPath)
	if err != nil {
		return nil, fmt.Errorf("NewGitRepo() [git.go]: %w", err)
	}

	g := &GitVCS{
		path:          absPath,
		worktreesPath: absWorktreesPath,
		worktrees:     make(map[string]*gitWorktree),
		hidePatterns:  hidePatterns,
		allowedPaths:  append([]string(nil), allowedPaths...),
		gitUserName:   name,
		gitUserEmail:  email,
	}

	showTopLevelOutput, err := g.runGitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		if isExitCode(err, 128) {
			return nil, fmt.Errorf("NewGitRepo() [git.go]: %w", ErrFileNotFound)
		}
		return nil, fmt.Errorf("NewGitRepo() [git.go]: %w", err)
	}

	repoTopLevelPath := filepath.Clean(strings.TrimSpace(showTopLevelOutput))
	if repoTopLevelPath != absPath {
		return nil, fmt.Errorf("NewGitRepo() [git.go]: path %q is not a git repository root: %w", absPath, ErrFileNotFound)
	}

	return g, nil
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
	branchExists, err := g.branchExists(branch)
	if err != nil {
		return nil, fmt.Errorf("GitVCS.GetWorktree() [git.go]: %w", err)
	}
	if !branchExists {
		return nil, fmt.Errorf("GitVCS.GetWorktree() [git.go]: branch %q not found: %w", branch, ErrFileNotFound)
	}

	worktreeInfo, statErr := os.Stat(worktreePath)
	if statErr == nil && worktreeInfo.IsDir() {
		currentBranch, branchErr := g.currentBranchInWorktree(worktreePath)
		if branchErr == nil && currentBranch == branch {
			localVFS, localErr := NewLocalVFS(worktreePath, g.hidePatterns, g.allowedPaths)
			if localErr != nil {
				return nil, fmt.Errorf("GitVCS.GetWorktree() [git.go]: %w", localErr)
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

	localVFS, err := NewLocalVFS(worktreePath, g.hidePatterns, g.allowedPaths)
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

	revisionExists, err := g.revisionExists(from)
	if err != nil {
		return fmt.Errorf("GitVCS.NewBranch() [git.go]: %w", err)
	}
	if !revisionExists {
		return fmt.Errorf("GitVCS.NewBranch() [git.go]: source branch %q not found: %w", from, ErrFileNotFound)
	}

	branchExists, err := g.branchExists(name)
	if err != nil {
		return fmt.Errorf("GitVCS.NewBranch() [git.go]: %w", err)
	}
	if branchExists {
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
	currentBranch, err := g.currentBranchInWorktree(g.path)
	if err != nil {
		return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: %w", err)
	}
	if currentBranch == name {
		return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: cannot delete current branch: %w", ErrPermissionDenied)
	}

	// Check if branch exists
	branchExists, err := g.branchExists(name)
	if err != nil {
		return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: %w", err)
	}
	if !branchExists {
		return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: branch %q not found: %w", name, ErrFileNotFound)
	}

	// Remove the worktree if it exists
	if wt, exists := g.worktrees[name]; exists {
		if wt.path != g.path {
			if err := g.runGit("worktree", "remove", "--force", wt.path); err != nil {
				return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: %w", err)
			}
			if err := os.RemoveAll(wt.path); err != nil {
				return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: %w", err)
			}
		}
		delete(g.worktrees, name)
	}

	if err := g.runGit("branch", "-D", name); err != nil {
		return fmt.Errorf("GitVCS.DeleteBranch() [git.go]: %w", err)
	}

	return nil
}

// ListBranches returns a list of all branches.
func (g *GitVCS) ListBranches(prefix string) ([]string, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	output, err := g.runGitOutput("for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("GitVCS.ListBranches() [git.go]: %w", err)
	}

	var result []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		branchName := strings.TrimSpace(line)
		if branchName == "" {
			continue
		}
		if prefix == "" || strings.HasPrefix(branchName, prefix) {
			result = append(result, branchName)
		}
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
	intoBranchExists, err := g.branchExists(into)
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}
	if !intoBranchExists {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: target branch %q not found: %w", into, ErrFileNotFound)
	}

	fromBranchExists, err := g.branchExists(from)
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}
	if !fromBranchExists {
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

	if err := g.syncCheckedOutBranchWorktrees(into, intoMergePath); err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git.go]: %w", err)
	}

	return nil
}

func (g *GitVCS) syncCheckedOutBranchWorktrees(branch string, skipPath string) error {
	paths, err := g.checkedOutBranchWorktreePaths(branch)
	if err != nil {
		return fmt.Errorf("GitVCS.syncCheckedOutBranchWorktrees() [git.go]: %w", err)
	}

	for _, worktreePath := range paths {
		if worktreePath == skipPath {
			continue
		}
		if err := g.runGitInWorktree(worktreePath, "reset", "--hard", branch); err != nil {
			return fmt.Errorf("GitVCS.syncCheckedOutBranchWorktrees() [git.go]: %w", err)
		}
	}

	return nil
}

func (g *GitVCS) checkedOutBranchWorktreePaths(branch string) ([]string, error) {
	output, err := g.runGitOutput("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("GitVCS.checkedOutBranchWorktreePaths() [git.go]: %w", err)
	}

	branchRef := "refs/heads/" + branch
	var paths []string
	var currentPath string
	var currentBranchRef string

	for _, line := range strings.Split(output, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			if currentPath != "" && currentBranchRef == branchRef {
				paths = append(paths, currentPath)
			}
			currentPath = ""
			currentBranchRef = ""
			continue
		}

		if strings.HasPrefix(trimmedLine, "worktree ") {
			currentPath = strings.TrimPrefix(trimmedLine, "worktree ")
			continue
		}

		if strings.HasPrefix(trimmedLine, "branch ") {
			currentBranchRef = strings.TrimPrefix(trimmedLine, "branch ")
		}
	}

	if currentPath != "" && currentBranchRef == branchRef {
		paths = append(paths, currentPath)
	}

	return paths, nil
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

func (g *GitVCS) runGitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", g.path}, args...)...)
	cmd.Env = g.gitCommandEnv()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("GitVCS.runGitOutput() [git.go]: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
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

	env := sanitizedGitEnv(os.Environ())
	env = upsertEnvValue(env, "GIT_AUTHOR_NAME", gitUserName)
	env = upsertEnvValue(env, "GIT_AUTHOR_EMAIL", gitUserEmail)
	env = upsertEnvValue(env, "GIT_COMMITTER_NAME", gitUserName)
	env = upsertEnvValue(env, "GIT_COMMITTER_EMAIL", gitUserEmail)
	return env
}

// sanitizedGitEnv removes git environment variables that can override repository discovery.
func sanitizedGitEnv(env []string) []string {
	filtered := make([]string, 0, len(env))
	for _, item := range env {
		if strings.HasPrefix(item, "GIT_DIR=") ||
			strings.HasPrefix(item, "GIT_WORK_TREE=") ||
			strings.HasPrefix(item, "GIT_COMMON_DIR=") ||
			strings.HasPrefix(item, "GIT_INDEX_FILE=") ||
			strings.HasPrefix(item, "GIT_OBJECT_DIRECTORY=") ||
			strings.HasPrefix(item, "GIT_ALTERNATE_OBJECT_DIRECTORIES=") {
			continue
		}
		filtered = append(filtered, item)
	}

	return filtered
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

func isExitCode(err error, code int) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	return exitErr.ExitCode() == code
}

func (g *GitVCS) branchExists(branch string) (bool, error) {
	_, err := g.runGitOutput("show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}
	if isExitCode(err, 1) {
		return false, nil
	}
	return false, fmt.Errorf("GitVCS.branchExists() [git.go]: %w", err)
}

func (g *GitVCS) revisionExists(revision string) (bool, error) {
	_, err := g.runGitOutput("rev-parse", "--verify", "--quiet", revision)
	if err == nil {
		return true, nil
	}
	if isExitCode(err, 1) {
		return false, nil
	}
	return false, fmt.Errorf("GitVCS.revisionExists() [git.go]: %w", err)
}
