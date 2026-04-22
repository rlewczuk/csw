package system

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vcs"
)

var executeConflictSubAgentFunc = executeConflictSubAgentTask

var newGenerationChatModelForWorktreeFunc = core.NewGenerationChatModelFromSpec

type WorktreeFinalizeResult struct {
	HeadCommitID string
}

func FinalizeWorktreeSession(ctx context.Context, gitVcs apis.VCS, worktreeBranch string, merge bool, commitMessageTemplate string, sweSystem *SweSystem, session *core.SweSession, stderr io.Writer, repoDir string, worktreeDir string, originalPrompt string) (WorktreeFinalizeResult, error) {
	result := WorktreeFinalizeResult{}
	if worktreeBranch == "" || gitVcs == nil {
		return result, nil
	}

	baseBranch := vcs.DetectMergeBaseBranch(repoDir)
	stashedLocalChanges := false
	commitMessage := ""
	commitHandledByHook := false

	if !commitHandledByHook {
		if strings.TrimSpace(commitMessage) == "" {
			if sweSystem == nil || sweSystem.Config == nil {
				_, _ = fmt.Fprintln(stderr, "worktree commit message generation failed: system config store is not available")
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}
			if session == nil {
				_, _ = fmt.Fprintln(stderr, "worktree commit message generation failed: session is not available")
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}

			providerName := strings.TrimSpace(session.ProviderName())
			provider, found := sweSystem.ModelProviders[providerName]
			if !found {
				_, _ = fmt.Fprintf(stderr, "worktree commit message generation failed: provider not found: %s\n", providerName)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}

			chatModel, modelErr := newGenerationChatModelForWorktreeFunc(
				session.ModelWithProvider(),
				sweSystem.ModelProviders,
				nil,
				sweSystem.Config,
				provider,
				sweSystem.ModelAliases,
				func(message string, msgType shared.MessageType) {
					if strings.TrimSpace(message) == "" {
						return
					}
					if msgType == shared.MessageTypeError {
						_, _ = fmt.Fprintf(stderr, "worktree commit message generation retry: %s\n", message)
						return
					}
					_, _ = fmt.Fprintf(stderr, "worktree commit message generation: %s\n", message)
				},
			)
			if modelErr != nil {
				_, _ = fmt.Fprintf(stderr, "worktree commit message generation failed: %v\n", modelErr)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}

			generatedMessage, err := core.GenerateCommitMessage(ctx, chatModel, sweSystem.Config, strings.TrimSpace(originalPrompt), worktreeBranch, commitMessageTemplate)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "worktree commit message generation failed: %v\n", err)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}
			commitMessage = generatedMessage
		}

		if commitErr := gitVcs.CommitWorktree(worktreeBranch, commitMessage); commitErr != nil && !errors.Is(commitErr, apis.ErrNoChangesToCommit) {
			_, _ = fmt.Fprintf(stderr, "worktree commit failed: %v\n", commitErr)
			if merge {
				_, _ = fmt.Fprintln(stderr, "merge skipped because commit failed. Resolve issues and merge manually.")
			}
			_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
			return result, nil
		}
	}

	if merge {
		shouldHandleLocalStash := strings.TrimSpace(repoDir) != ""

		var stashErr error
		if shouldHandleLocalStash {
			stashedLocalChanges, stashErr = stashLocalChangesBeforeMerge(repoDir)
			if stashErr != nil {
				_, _ = fmt.Fprintf(stderr, "automatic merge failed: %v\n", stashErr)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}

			defer func() {
				restoreErr := restoreStashedLocalChangesAfterMerge(repoDir, stashedLocalChanges, stderr)
				if restoreErr != nil {
					_, _ = fmt.Fprintf(stderr, "failed to restore stashed local changes after merge: %v\n", restoreErr)
				}
			}()
		}

		if strings.TrimSpace(repoDir) == "" || strings.TrimSpace(worktreeDir) == "" || sweSystem == nil || session == nil {
			mergeErr := gitVcs.MergeBranches(baseBranch, worktreeBranch)
			if mergeErr != nil {
				if errors.Is(mergeErr, apis.ErrMergeConflict) {
					_, _ = fmt.Fprintf(stderr, "automatic merge failed due to conflicts: %v\n", mergeErr)
					_, _ = fmt.Fprintf(stderr, "resolve conflicts manually and merge branch '%s' into %s.\n", worktreeBranch, baseBranch)
					_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual conflict resolution.")
					return result, nil
				}

				_, _ = fmt.Fprintf(stderr, "automatic merge failed: %v\n", mergeErr)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}

			result.HeadCommitID = vcs.ResolveGitCommitID(repoDir, baseBranch)
		} else {
			headCommitID, mergeErr := mergeWorktreeWithConflictResolution(ctx, repoDir, worktreeDir, worktreeBranch, baseBranch, originalPrompt, sweSystem, session, stderr)
			if mergeErr != nil {
				_, _ = fmt.Fprintf(stderr, "automatic merge failed: %v\n", mergeErr)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}
			result.HeadCommitID = headCommitID
		}
	}

	if dropErr := gitVcs.DropWorktree(worktreeBranch); dropErr != nil {
		_, _ = fmt.Fprintf(stderr, "worktree cleanup failed: %v\n", dropErr)
	}

	if merge {
		if deleteErr := gitVcs.DeleteBranch(worktreeBranch); deleteErr != nil {
			_, _ = fmt.Fprintf(stderr, "feature branch cleanup failed: %v\n", deleteErr)
		}
	}

	return result, nil
}

// stashLocalChangesBeforeMerge stashes local repository changes before automatic merge.
func stashLocalChangesBeforeMerge(repoDir string) (bool, error) {
	trimmedRepoDir := strings.TrimSpace(repoDir)
	if trimmedRepoDir == "" {
		return false, nil
	}

	repoInfo, statErr := os.Stat(trimmedRepoDir)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return false, nil
		}
		return false, fmt.Errorf("stashLocalChangesBeforeMerge() [cli.go]: failed to stat repository directory: %w", statErr)
	}
	if !repoInfo.IsDir() {
		return false, nil
	}

	statusOutput, err := vcs.RunGitCommand(trimmedRepoDir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("stashLocalChangesBeforeMerge() [cli.go]: failed to read repository status: %w", err)
	}

	if strings.TrimSpace(statusOutput) == "" {
		return false, nil
	}

	if _, err := vcs.RunGitCommand(trimmedRepoDir, "stash", "push", "--include-untracked", "-m", "csw: automatic stash before merge"); err != nil {
		return false, fmt.Errorf("stashLocalChangesBeforeMerge() [cli.go]: failed to stash local changes: %w", err)
	}

	return true, nil
}

// restoreStashedLocalChangesAfterMerge restores stashed local changes after automatic merge.
func restoreStashedLocalChangesAfterMerge(repoDir string, stashed bool, stderr io.Writer) error {
	if !stashed {
		return nil
	}

	trimmedRepoDir := strings.TrimSpace(repoDir)
	if trimmedRepoDir == "" {
		return nil
	}

	if _, err := vcs.RunGitCommand(trimmedRepoDir, "stash", "apply", "--index", "stash@{0}"); err != nil {
		if vcs.IsMergeConflictError(err.Error()) {
			if _, resetErr := vcs.RunGitCommand(trimmedRepoDir, "reset", "--hard", "HEAD"); resetErr != nil {
				return fmt.Errorf("restoreStashedLocalChangesAfterMerge() [cli.go]: failed to reset repository after unstash conflict: %w", resetErr)
			}

			if _, cleanErr := vcs.RunGitCommand(trimmedRepoDir, "clean", "-fd"); cleanErr != nil {
				return fmt.Errorf("restoreStashedLocalChangesAfterMerge() [cli.go]: failed to clean repository after unstash conflict: %w", cleanErr)
			}

			_, _ = fmt.Fprintln(stderr, "automatic unstash failed due to conflicts; local changes remain stashed and repository was restored to a clean state.")
			_, _ = fmt.Fprintln(stderr, "please unstash manually when ready (for example: git stash pop stash@{0}).")
			return nil
		}

		return fmt.Errorf("restoreStashedLocalChangesAfterMerge() [cli.go]: failed to apply stashed local changes: %w", err)
	}

	if _, err := vcs.RunGitCommand(trimmedRepoDir, "stash", "drop", "stash@{0}"); err != nil {
		return fmt.Errorf("restoreStashedLocalChangesAfterMerge() [cli.go]: failed to drop temporary stash: %w", err)
	}

	return nil
}

func mergeWorktreeWithConflictResolution(ctx context.Context, repoDir string, worktreeDir string, worktreeBranch string, baseBranch string, originalPrompt string, sweSystem *SweSystem, session *core.SweSession, stderr io.Writer) (string, error) {
	if sweSystem == nil || sweSystem.Config == nil {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: system config store is not available")
	}
	if session == nil {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: session is nil")
	}
	if strings.TrimSpace(baseBranch) == "" {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: baseBranch is empty")
	}

	attempt := 1
	failedRebaseOutput := ""
	conflictFiles := []string{}

	for {
		command := []string{"rebase", baseBranch}
		_, rebaseErr := vcs.RunGitCommand(worktreeDir, command...)
		if rebaseErr == nil {
			_, _ = fmt.Fprintf(stderr, "[conflict] rebase step succeeded (attempt %d, command: git %s)\n", attempt, strings.Join(command, " "))
			break
		}

		rebaseOutput := rebaseErr.Error()
		if !vcs.IsMergeConflictError(rebaseOutput) {
			return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: rebase failed: %w", rebaseErr)
		}

		failedRebaseOutput = rebaseOutput
		conflictFiles = vcs.ListGitConflictFiles(worktreeDir)
		if len(conflictFiles) == 0 {
			conflictFiles = vcs.ExtractConflictFilesFromOutput(rebaseOutput)
		}

		_, _ = fmt.Fprintf(stderr, "[conflict] merge/rebase conflict detected on attempt %d\n", attempt)
		if len(conflictFiles) > 0 {
			_, _ = fmt.Fprintf(stderr, "[conflict] conflicted files: %s\n", strings.Join(conflictFiles, ", "))
		}

		prompt, err := buildConflictResolutionPrompt(sweSystem.Config, conflictResolutionPromptData{
			Branch:         worktreeBranch,
			OriginalPrompt: originalPrompt,
			ConflictFiles:  strings.Join(conflictFiles, "\n"),
			ConflictOutput: failedRebaseOutput,
		})
		if err != nil {
			return "", err
		}

		request := tool.SubAgentTaskRequest{
			Slug:   fmt.Sprintf("conflict-resolution-%d", attempt),
			Title:  fmt.Sprintf("Resolve merge conflicts (%d)", attempt),
			Role:   "developer",
			Prompt: prompt,
		}
		_, _ = fmt.Fprintf(stderr, "[conflict] starting conflict-resolution sub-session: %s\n", request.Slug)
		subAgentResult, subAgentErr := executeConflictSubAgentFunc(session, request)
		if subAgentErr != nil {
			return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: sub-session failed: %w", subAgentErr)
		}
		if subAgentResult.Status != "completed" {
			return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: conflict-resolution sub-session ended with status %q: %s", subAgentResult.Status, strings.TrimSpace(subAgentResult.Summary))
		}
		_, _ = fmt.Fprintf(stderr, "[conflict] conflict-resolution sub-session completed: %s\n", request.Slug)
		attempt++
	}

	mergeWorktreePath, cleanup, err := vcs.CreateMergeWorktree(repoDir, baseBranch)
	if err != nil {
		return "", err
	}
	defer cleanup()

	if _, err := vcs.RunGitCommand(mergeWorktreePath, "merge", "--ff-only", worktreeBranch); err != nil {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: fast-forward merge into %q failed: %w", baseBranch, err)
	}

	headCommitID, err := vcs.RunGitCommand(mergeWorktreePath, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: failed to resolve %q HEAD commit: %w", baseBranch, err)
	}

	if err := syncCheckedOutBranchWorktrees(repoDir, baseBranch, mergeWorktreePath); err != nil {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: %w", err)
	}

	return strings.TrimSpace(headCommitID), nil
}

func buildConflictResolutionPrompt(config *conf.CswConfig, data conflictResolutionPromptData) (string, error) {
	if config == nil {
		return "", fmt.Errorf("buildConflictResolutionPrompt() [cli.go]: config cannot be nil")
	}
	conflictFiles, ok := config.AgentConfigFiles["conflict"]
	if !ok {
		return "", fmt.Errorf("buildConflictResolutionPrompt() [cli.go]: failed to read conflict/prompt.md: conflict files not found")
	}
	templateText, ok := conflictFiles["prompt.md"]
	if !ok {
		return "", fmt.Errorf("buildConflictResolutionPrompt() [cli.go]: failed to read conflict/prompt.md: file not found")
	}

	tmpl, err := template.New("conflict-prompt").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("buildConflictResolutionPrompt() [cli.go]: failed to parse prompt template: %w", err)
	}

	var promptBuffer bytes.Buffer
	if err := tmpl.Execute(&promptBuffer, data); err != nil {
		return "", fmt.Errorf("buildConflictResolutionPrompt() [cli.go]: failed to render prompt template: %w", err)
	}

	return promptBuffer.String(), nil
}

func syncCheckedOutBranchWorktrees(repoDir string, branch string, skipPath string) error {
	worktreesOutput, err := vcs.RunGitCommand(repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("syncCheckedOutBranchWorktrees() [cli.go]: failed to list git worktrees: %w", err)
	}

	branchRef := "refs/heads/" + branch
	skipPath = filepath.Clean(strings.TrimSpace(skipPath))

	resetWorktree := func(worktreePath string, worktreeBranchRef string) error {
		trimmedPath := filepath.Clean(strings.TrimSpace(worktreePath))
		if trimmedPath == "" || worktreeBranchRef != branchRef || trimmedPath == skipPath {
			return nil
		}

		if _, resetErr := vcs.RunGitCommand(trimmedPath, "reset", "--hard", branch); resetErr != nil {
			return fmt.Errorf("syncCheckedOutBranchWorktrees() [cli.go]: failed to update checked-out %q worktree at %q: %w", branch, trimmedPath, resetErr)
		}

		return nil
	}

	currentPath := ""
	currentBranchRef := ""
	for _, line := range strings.Split(worktreesOutput, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			if err := resetWorktree(currentPath, currentBranchRef); err != nil {
				return err
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

	if err := resetWorktree(currentPath, currentBranchRef); err != nil {
		return err
	}

	return nil
}

type conflictResolutionPromptData struct {
	Branch         string
	OriginalPrompt string
	ConflictFiles  string
	ConflictOutput string
}

func executeConflictSubAgentTask(session *core.SweSession, request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error) {
	if session == nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("executeConflictSubAgentTask() [cli.go]: session is nil")
	}

	return session.ExecuteSubAgentTask(request)
}
