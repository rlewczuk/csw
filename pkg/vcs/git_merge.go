package vcs

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var RunGitCommand = runGitCommandX
var ListGitConflictFiles = listGitConflictFiles
var GitLookPath = exec.LookPath
var GitConfigValue = ReadGitConfigValue

// ReadGitConfigValue reads a single git configuration key from host git config.
func ReadGitConfigValue(key string) (string, error) {
	cmd := exec.Command("git", "config", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ReadGitConfigValue() [bootstrap.go]: failed to read git config key %q: %w", key, err)
	}

	return strings.TrimSpace(string(output)), nil
}

func GitWorktreeForBranch(repoDir string, branch string) (bool, string) {
	output, err := RunGitCommand(repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return false, ""
	}

	targetRef := "refs/heads/" + strings.TrimSpace(branch)
	currentPath := ""
	currentBranchRef := ""

	flush := func() (bool, string) {
		if strings.TrimSpace(currentPath) == "" {
			return false, ""
		}
		if strings.TrimSpace(currentBranchRef) != targetRef {
			return false, ""
		}
		worktreePath := filepath.Clean(strings.TrimSpace(currentPath))
		return true, filepath.Base(worktreePath)
	}

	for _, line := range strings.Split(output, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			if ok, name := flush(); ok {
				return true, name
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

	if ok, name := flush(); ok {
		return true, name
	}

	return false, ""
}

func HardResetWorktree(workDir string) error {
	trimmedWorkDir := strings.TrimSpace(workDir)
	if trimmedWorkDir == "" {
		return fmt.Errorf("hardResetWorktree() [git_metge.go]: workDir is empty")
	}

	if _, err := RunGitCommand(trimmedWorkDir, "reset", "--hard", "HEAD"); err != nil {
		return fmt.Errorf("hardResetWorktree() [git_metge.go]: failed to reset worktree: %w", err)
	}
	if _, err := RunGitCommand(trimmedWorkDir, "clean", "-fd"); err != nil {
		return fmt.Errorf("hardResetWorktree() [git_metge.go]: failed to clean worktree: %w", err)
	}

	return nil
}

// DetectMergeBaseBranch resolves the branch currently checked out in repoDir.
func DetectMergeBaseBranch(repoDir string) string {
	if strings.TrimSpace(repoDir) == "" {
		return "main"
	}

	branch, err := RunGitCommand(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "main"
	}

	trimmedBranch := strings.TrimSpace(branch)
	if trimmedBranch == "" || trimmedBranch == "HEAD" {
		return "main"
	}

	return trimmedBranch
}

func GitBranchExists(repoDir string, branch string) (bool, error) {
	_, err := RunGitCommand(repoDir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}

	if strings.Contains(err.Error(), "exit status 1") {
		return false, nil
	}

	return false, err
}

func CreateMergeWorktree(repoDir string, branch string) (string, func(), error) {
	if strings.TrimSpace(branch) == "" {
		return "", func() {}, fmt.Errorf("createMergeWorktree() [git_metge.go]: branch is empty")
	}

	workRoot := filepath.Join(repoDir, ".cswdata", "work")
	if err := os.MkdirAll(workRoot, 0755); err != nil {
		return "", func() {}, fmt.Errorf("createMergeWorktree() [git_metge.go]: failed to create worktree root directory: %w", err)
	}

	mergeWorktreePath, err := os.MkdirTemp(workRoot, ".merge-"+strings.ReplaceAll(branch, "/", "-")+"-")
	if err != nil {
		return "", func() {}, fmt.Errorf("createMergeWorktree() [git_metge.go]: failed to allocate temporary merge worktree path: %w", err)
	}

	if err := os.RemoveAll(mergeWorktreePath); err != nil {
		return "", func() {}, fmt.Errorf("createMergeWorktree() [git_metge.go]: failed to prepare temporary merge worktree path: %w", err)
	}

	if _, err := RunGitCommand(repoDir, "worktree", "add", "--force", mergeWorktreePath, branch); err != nil {
		_ = os.RemoveAll(mergeWorktreePath)
		return "", func() {}, fmt.Errorf("createMergeWorktree() [git_metge.go]: failed to create temporary %q worktree: %w", branch, err)
	}

	cleanup := func() {
		_, _ = RunGitCommand(repoDir, "worktree", "remove", "--force", mergeWorktreePath)
		_ = os.RemoveAll(mergeWorktreePath)
	}

	return mergeWorktreePath, cleanup, nil
}

func runGitCommandX(workDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	trimmedOutput := strings.TrimSpace(string(output))
	if err != nil {
		return "", fmt.Errorf("runGitCommandX() [git_metge.go]: git %s failed: %w: %s", strings.Join(args, " "), err, trimmedOutput)
	}

	return trimmedOutput, nil
}

func listGitConflictFiles(workDir string) []string {
	output, err := RunGitCommand(workDir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	conflicts := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		conflicts = append(conflicts, trimmed)
	}

	return conflicts
}

func ExtractConflictFilesFromOutput(output string) []string {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	pattern := regexp.MustCompile(`(?m)CONFLICT \([^\)]*\): .* in (.+)$`)
	matches := pattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	files := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		files = append(files, name)
	}

	return files
}

func IsMergeConflictError(output string) bool {
	trimmedOutput := strings.TrimSpace(output)
	if trimmedOutput == "" {
		return false
	}

	upperOutput := strings.ToUpper(trimmedOutput)
	if strings.Contains(upperOutput, "CONFLICT") {
		return true
	}

	return strings.Contains(trimmedOutput, "could not apply") ||
		strings.Contains(trimmedOutput, "Resolve all conflicts manually") ||
		strings.Contains(trimmedOutput, "rebase-merge")
}

func ResolveGitCommitID(workDir string, rev string) string {
	if strings.TrimSpace(workDir) == "" || strings.TrimSpace(rev) == "" {
		return ""
	}

	cmd := exec.Command("git", "rev-parse", rev)
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func ResolveHostGitConfigValue(key string) string {
	if _, err := GitLookPath("git"); err != nil {
		return ""
	}

	value, err := GitConfigValue(key)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(value)
}

// resolveGitIdentity returns the provided value if non-empty, otherwise falls back to host git config.
func ResolveGitIdentity(value, gitConfigKey string) string {
	if value != "" {
		return value
	}
	return ResolveHostGitConfigValue(gitConfigKey)
}

func ChooseGitDiffDir(workDirRoot string, workDir string) string {
	if strings.TrimSpace(workDir) != "" {
		return workDir
	}
	if strings.TrimSpace(workDirRoot) != "" {
		return workDirRoot
	}

	return "."
}

func GitDiffNameOnly(workDir string, commitRange string) []string {
	args := []string{"diff", "--name-only"}
	if strings.TrimSpace(commitRange) != "" {
		args = append(args, commitRange)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return ParseGitFileList(output)
}

func GitUntrackedFiles(workDir string) []string {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return ParseGitFileList(output)
}

func ParseGitFileList(output []byte) []string {
	if len(output) == 0 {
		return nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	result := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		result = append(result, line)
	}

	if len(result) == 0 {
		return nil
	}

	sort.Strings(result)
	return result
}

func CollectEditedFiles(workDirRoot string, workDir string, baseCommitID string, headCommitID string) []string {
	diffDir := ChooseGitDiffDir(workDirRoot, workDir)
	if diffDir == "" {
		return nil
	}

	trimmedBase := strings.TrimSpace(baseCommitID)
	trimmedHead := strings.TrimSpace(headCommitID)
	if trimmedBase != "" && trimmedHead != "" && trimmedBase != trimmedHead {
		files := GitDiffNameOnly(diffDir, trimmedBase+".."+trimmedHead)
		if len(files) > 0 {
			return files
		}
	}

	tracked := GitDiffNameOnly(diffDir, "")
	untracked := GitUntrackedFiles(diffDir)
	combined := append(tracked, untracked...)
	if len(combined) == 0 {
		return nil
	}

	unique := make(map[string]struct{}, len(combined))
	for _, file := range combined {
		if strings.TrimSpace(file) == "" {
			continue
		}
		unique[file] = struct{}{}
	}

	result := make([]string, 0, len(unique))
	for file := range unique {
		result = append(result, file)
	}
	sort.Strings(result)

	return result
}
