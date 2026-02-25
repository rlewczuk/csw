package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/spf13/cobra"
)

// CleanCommand creates the clean command.
func CleanCommand() *cobra.Command {
	var (
		cleanWorktree bool
		cleanAll      bool
		cleanWorkDir  string
	)

	cmd := &cobra.Command{
		Use:   "clean [--worktree [branch-name]] [--all] [--workdir <dir>]",
		Short: "Clean up stale worktrees and other temporary files",
		Long: `Clean up stale worktrees and other temporary files.

Currently supports cleaning up worktrees:
  - csw clean --worktree <branch-name>  Clean up a specific worktree
  - csw clean --worktree --all          Clean up all stale worktrees
  - csw clean --all                     Clean up all temporary files (equivalent to --worktree --all)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			// Resolve working directory
			workDir, err := ResolveWorkDir(cleanWorkDir)
			if err != nil {
				return fmt.Errorf("CleanCommand.RunE() [clean.go]: %w", err)
			}

			// Determine what to clean
			if !cleanWorktree && !cleanAll {
				return fmt.Errorf("CleanCommand.RunE() [clean.go]: must specify --worktree or --all")
			}

			// If --all is set without --worktree, it implies --worktree --all
			if cleanAll {
				cleanWorktree = true
			}

			// Get branch name from args if provided
			var branchName string
			if len(args) > 0 {
				branchName = args[0]
			}

			// Validate flags
			if cleanWorktree && !cleanAll && branchName == "" {
				return fmt.Errorf("CleanCommand.RunE() [clean.go]: --worktree requires a branch name or --all")
			}

			if cleanAll && branchName != "" {
				return fmt.Errorf("CleanCommand.RunE() [clean.go]: cannot specify branch name with --all")
			}

			// Create VCS for cleaning
			vcs, err := createCleanVCS(workDir)
			if err != nil {
				return fmt.Errorf("CleanCommand.RunE() [clean.go]: %w", err)
			}

			// Perform cleanup
			if cleanAll {
				return cleanAllWorktrees(vcs, os.Stdout)
			}
			return cleanSingleWorktree(vcs, branchName, os.Stdout)
		},
	}

	cmd.Flags().BoolVar(&cleanWorktree, "worktree", false, "Clean up worktree(s)")
	cmd.Flags().BoolVar(&cleanAll, "all", false, "Clean up all stale items")
	cmd.Flags().StringVar(&cleanWorkDir, "workdir", "", "Working directory (default: current directory)")

	return cmd
}

// createCleanVCS creates a VCS instance for cleaning worktrees.
func createCleanVCS(workDir string) (vfs.VCS, error) {
	// Check if this is a git repository
	worktreesRoot := workDir + "/.cswdata/work"
	gitRepo, err := vfs.NewGitRepo(workDir, worktreesRoot, nil)
	if err != nil {
		return nil, fmt.Errorf("createCleanVCS() [clean.go]: not a git repository: %w", err)
	}
	return gitRepo, nil
}

// cleanSingleWorktree cleans up a specific worktree.
func cleanSingleWorktree(vcs vfs.VCS, branchName string, output io.Writer) error {
	// Check if worktree exists
	worktrees, err := vcs.ListWorktrees()
	if err != nil {
		return fmt.Errorf("cleanSingleWorktree() [clean.go]: failed to list worktrees: %w", err)
	}

	found := false
	for _, wt := range worktrees {
		if wt == branchName {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("cleanSingleWorktree() [clean.go]: worktree %q not found", branchName)
	}

	// Drop the worktree
	if err := vcs.DropWorktree(branchName); err != nil {
		return fmt.Errorf("cleanSingleWorktree() [clean.go]: failed to drop worktree: %w", err)
	}

	// Delete the branch
	if err := vcs.DeleteBranch(branchName); err != nil {
		fmt.Fprintf(output, "Warning: failed to delete branch %q: %v\n", branchName, err)
	} else {
		fmt.Fprintf(output, "Deleted branch: %s\n", branchName)
	}

	fmt.Fprintf(output, "Cleaned up worktree: %s\n", branchName)
	return nil
}

// cleanAllWorktrees cleans up all worktrees.
func cleanAllWorktrees(vcs vfs.VCS, output io.Writer) error {
	worktrees, err := vcs.ListWorktrees()
	if err != nil {
		return fmt.Errorf("cleanAllWorktrees() [clean.go]: failed to list worktrees: %w", err)
	}

	if len(worktrees) == 0 {
		fmt.Fprintln(output, "No worktrees to clean up.")
		return nil
	}

	cleaned := 0
	for _, branchName := range worktrees {
		if err := vcs.DropWorktree(branchName); err != nil {
			fmt.Fprintf(output, "Warning: failed to clean up worktree %q: %v\n", branchName, err)
			continue
		}
		fmt.Fprintf(output, "Cleaned up worktree: %s\n", branchName)

		if err := vcs.DeleteBranch(branchName); err != nil {
			fmt.Fprintf(output, "Warning: failed to delete branch %q: %v\n", branchName, err)
		} else {
			fmt.Fprintf(output, "Deleted branch: %s\n", branchName)
		}
		cleaned++
	}

	fmt.Fprintf(output, "Cleaned up %d worktree(s).\n", cleaned)
	return nil
}
