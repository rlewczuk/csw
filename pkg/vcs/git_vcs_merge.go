package vcs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/apis"
)

// MergeBranches merges the given source branch into the target branch.
func (g *GitVCS) MergeBranches(into string, from string) error {
	intoBranchExists, err := g.branchExists(into)
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
	}
	if !intoBranchExists {
		return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: target branch %q not found: %w", into, apis.ErrFileNotFound)
	}

	fromBranchExists, err := g.branchExists(from)
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
	}
	if !fromBranchExists {
		return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: source branch %q not found: %w", from, apis.ErrFileNotFound)
	}

	g.mutex.RLock()
	intoWorktree, hasIntoWorktree := g.worktrees[into]
	fromWorktree, hasFromWorktree := g.worktrees[from]
	g.mutex.RUnlock()

	currentPrimaryBranch, err := g.currentBranchInWorktree(g.path)
	if err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
	}

	intoMergePath := g.path
	intoCleanup := func() {}
	if hasIntoWorktree {
		intoMergePath = intoWorktree.path
	} else if currentPrimaryBranch != into {
		tempIntoWorktreePath := filepath.Join(g.worktreesPath, ".merge-into-"+strings.ReplaceAll(into, "/", "_"))
		if err := os.RemoveAll(tempIntoWorktreePath); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(tempIntoWorktreePath), 0755); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
		}
		if err := g.runGit("worktree", "add", "--force", tempIntoWorktreePath, into); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
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
			return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(tempFromWorktreePath), 0755); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
		}
		if err := g.runGit("worktree", "add", "--force", tempFromWorktreePath, from); err != nil {
			return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
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
			return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w: %w", err, apis.ErrMergeConflict)
		}
		return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
	}

	if err := g.runGitInWorktree(intoMergePath, "merge", "--ff-only", from); err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
	}

	if err := g.syncCheckedOutBranchWorktrees(into, intoMergePath); err != nil {
		return fmt.Errorf("GitVCS.MergeBranches() [git_vcs_merge.go]: %w", err)
	}

	return nil
}

func (g *GitVCS) syncCheckedOutBranchWorktrees(branch string, skipPath string) error {
	paths, err := g.checkedOutBranchWorktreePaths(branch)
	if err != nil {
		return fmt.Errorf("GitVCS.syncCheckedOutBranchWorktrees() [git_vcs_merge.go]: %w", err)
	}

	for _, worktreePath := range paths {
		if worktreePath == skipPath {
			continue
		}
		if err := g.runGitInWorktree(worktreePath, "reset", "--hard", branch); err != nil {
			return fmt.Errorf("GitVCS.syncCheckedOutBranchWorktrees() [git_vcs_merge.go]: %w", err)
		}
	}

	return nil
}

func (g *GitVCS) checkedOutBranchWorktreePaths(branch string) ([]string, error) {
	output, err := g.runGitOutput("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("GitVCS.checkedOutBranchWorktreePaths() [git_vcs_merge.go]: %w", err)
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
