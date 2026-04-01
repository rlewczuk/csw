package system

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/vcs"
)

var ResumeUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var ResumeWorktreeNamePattern = regexp.MustCompile(`^[0-9]{4}-[a-z0-9][a-z0-9-]*$`)

var executeConflictSubAgentFunc = executeConflictSubAgentTask

type WorktreeFinalizeResult struct {
	HeadCommitID string
}

func FinalizeWorktreeSession(ctx context.Context, gitVcs apis.VCS, worktreeBranch string, merge bool, commitMessageTemplate string, sweSystem *SweSystem, session *core.SweSession, stderr io.Writer, repoDir string, worktreeDir string, originalPrompt string, hookEngine *core.HookEngine, appView ui.IAppView) (WorktreeFinalizeResult, error) {
	result := WorktreeFinalizeResult{}
	if worktreeBranch == "" || gitVcs == nil {
		return result, nil
	}

	baseBranch := vcs.DetectMergeBaseBranch(repoDir)
	commitMessage := ""
	commitHandledByHook := false

	hookCommitMessage, skipBuiltInCommit, commitHookErr := HandleCommitHookResponse(ctx, hookEngine, gitVcs, worktreeBranch, repoDir, worktreeDir, session, appView)
	if commitHookErr != nil {
		_, _ = fmt.Fprintf(stderr, "worktree commit hook failed: %v\n", commitHookErr)
		return result, fmt.Errorf("finalizeWorktreeSession() [cli.go]: commit hook failed: %w", commitHookErr)
	}
	if strings.TrimSpace(hookCommitMessage) != "" {
		commitMessage = hookCommitMessage
	}
	if skipBuiltInCommit {
		commitHandledByHook = true
	}

	if !commitHandledByHook {
		if strings.TrimSpace(commitMessage) == "" {
			generatedMessage, err := core.GenerateCommitMessage(ctx, sweSystem.ModelProviders, sweSystem.ConfigStore, session, worktreeBranch, commitMessageTemplate)
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
		mergeHookExecuted := false
		if hookEngine != nil {
			hookEngine.MergeContext(map[string]string{
				"branch":  strings.TrimSpace(worktreeBranch),
				"workdir": strings.TrimSpace(firstNonEmpty(worktreeDir, repoDir)),
				"rootdir": strings.TrimSpace(repoDir),
			})
			hookResult, hookErr := hookEngine.Execute(ctx, core.HookExecutionRequest{Name: "merge", View: appView, VCS: gitVcs, Session: session})
			if hookResult != nil {
				mergeHookExecuted = true
			}
			if hookErr != nil {
				_, _ = fmt.Fprintf(stderr, "automatic merge failed: %v\n", hookErr)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}
		}

		if mergeHookExecuted {
			result.HeadCommitID = vcs.ResolveGitCommitID(repoDir, baseBranch)
		} else if strings.TrimSpace(repoDir) == "" || strings.TrimSpace(worktreeDir) == "" || sweSystem == nil || session == nil {
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

func mergeWorktreeWithConflictResolution(ctx context.Context, repoDir string, worktreeDir string, worktreeBranch string, baseBranch string, originalPrompt string, sweSystem *SweSystem, session *core.SweSession, stderr io.Writer) (string, error) {
	if sweSystem == nil || sweSystem.ConfigStore == nil {
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

		prompt, err := buildConflictResolutionPrompt(sweSystem.ConfigStore, conflictResolutionPromptData{
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

func buildConflictResolutionPrompt(configStore interface {
	GetAgentConfigFile(subdir, filename string) ([]byte, error)
}, data conflictResolutionPromptData) (string, error) {
	templateBytes, err := configStore.GetAgentConfigFile("conflict", "prompt.md")
	if err != nil {
		return "", fmt.Errorf("buildConflictResolutionPrompt() [cli.go]: failed to read conflict/prompt.md: %w", err)
	}

	tmpl, err := template.New("conflict-prompt").Parse(string(templateBytes))
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

func FindSessionIDByWorkDirName(workDirName string, logsDir string, sessionEntries []os.DirEntry) (string, bool) {
	trimmedName := strings.TrimSpace(workDirName)
	if trimmedName == "" {
		return "", false
	}

	for _, entry := range sessionEntries {
		if !entry.IsDir() {
			continue
		}

		statePath := filepath.Join(logsDir, "sessions", entry.Name(), "session.json")
		stateBytes, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}

		var state core.PersistedSessionState
		if err := json.Unmarshal(stateBytes, &state); err != nil {
			continue
		}

		sessionWorkDir := strings.TrimSpace(state.WorkDir)
		if sessionWorkDir == "" {
			continue
		}

		if filepath.Base(filepath.Clean(sessionWorkDir)) == trimmedName {
			return entry.Name(), true
		}
	}

	return "", false
}

func ResolveResumeTargetAsBranchOrWorktree(target string, workDir string, logsDir string, sessionEntries []os.DirEntry) (string, bool) {
	resolvedWorkDir, err := ResolveWorkDir(workDir)
	if err != nil {
		return "", false
	}

	branchExists, err := vcs.GitBranchExists(resolvedWorkDir, target)
	if err == nil && branchExists {
		worktreeExists, worktreeName := vcs.GitWorktreeForBranch(resolvedWorkDir, target)
		if worktreeExists && strings.TrimSpace(worktreeName) != "" {
			if sessionID, ok := FindSessionIDByWorkDirName(worktreeName, logsDir, sessionEntries); ok {
				return sessionID, true
			}
		}
	}

	return FindSessionIDByWorkDirName(target, logsDir, sessionEntries)
}

func FindSessionIDByWorkDirPath(workDirPath string, logsDir string, sessionEntries []os.DirEntry) (string, bool) {
	expectedPath := filepath.Clean(strings.TrimSpace(workDirPath))
	if expectedPath == "" {
		return "", false
	}

	for _, entry := range sessionEntries {
		if !entry.IsDir() {
			continue
		}

		statePath := filepath.Join(logsDir, "sessions", entry.Name(), "session.json")
		stateBytes, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}

		var state core.PersistedSessionState
		if err := json.Unmarshal(stateBytes, &state); err != nil {
			continue
		}

		sessionWorkDir := filepath.Clean(strings.TrimSpace(state.WorkDir))
		if sessionWorkDir == expectedPath {
			return entry.Name(), true
		}
	}

	return "", false
}

func ResolveResumeTargetToSessionID(resumeTarget string, workDir string, logsDir string) (string, error) {
	trimmedTarget := strings.TrimSpace(resumeTarget)
	if trimmedTarget == "" {
		return "", nil
	}

	if strings.EqualFold(trimmedTarget, "last") {
		return "last", nil
	}

	if strings.TrimSpace(logsDir) == "" {
		return "", fmt.Errorf("ResolveResumeTargetToSessionID() [cli.go]: logs directory is empty")
	}

	if ResumeUUIDPattern.MatchString(trimmedTarget) {
		return strings.ToLower(trimmedTarget), nil
	}

	entries, err := os.ReadDir(filepath.Join(logsDir, "sessions"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("ResolveResumeTargetToSessionID() [cli.go]: no persisted sessions found")
		}
		return "", fmt.Errorf("ResolveResumeTargetToSessionID() [cli.go]: failed to read sessions directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})

	if candidateID, ok := ResolveResumeTargetAsPath(trimmedTarget, logsDir, entries); ok {
		return candidateID, nil
	}

	if candidateID, ok := ResolveResumeTargetAsBranchOrWorktree(trimmedTarget, workDir, logsDir, entries); ok {
		return candidateID, nil
	}

	return "", fmt.Errorf("ResolveResumeTargetToSessionID() [cli.go]: no session found for --resume value %q", resumeTarget)
}

func ResolveResumeTargetAsPath(target string, logsDir string, sessionEntries []os.DirEntry) (string, bool) {
	if !filepath.IsAbs(target) {
		return "", false
	}

	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		return "", false
	}

	if sessionID, ok := FindSessionIDByWorkDirPath(target, logsDir, sessionEntries); ok {
		return sessionID, true
	}

	return "", false
}
