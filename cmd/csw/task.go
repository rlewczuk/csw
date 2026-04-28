package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/spf13/cobra"
)

var resolveTaskRunDefaultsFunc = system.ResolveRunDefaults
var resolveTaskWorktreeBranchNameFunc = system.ResolveWorktreeBranchName
var generateTaskDescriptionFunc = generateTaskDescription
var buildTaskDescriptionSystemFunc = system.BuildSystem
var newGenerationChatModelFromSpecFunc = core.NewGenerationChatModelFromSpec
var prepareTaskSessionVFSFunc = system.PrepareSessionVFS

// TaskCommand creates task command with persistent hierarchical task management.
func TaskCommand() *cobra.Command {
	var taskDir string

	command := &cobra.Command{
		Use:   "task",
		Short: "Manage persistent hierarchical tasks",
	}

	command.PersistentFlags().StringVar(&taskDir, "task-dir", "", "Task directory path (relative paths are resolved from project directory)")

	command.AddCommand(taskNewCommand())
	command.AddCommand(taskUpdateCommand())
	command.AddCommand(taskEditCommand())
	command.AddCommand(taskGetCommand())
	command.AddCommand(taskListCommand())
	command.AddCommand(taskMergeCommand())
	command.AddCommand(taskArchiveCommand())

	return command
}

func loadTaskManager(cmd *cobra.Command) (*core.TaskManager, apis.VCS, error) {
	workDir, err := system.ResolveWorkDir("")
	if err != nil {
		return nil, nil, err
	}

	resolvedShadowDir := strings.TrimSpace(resolveShadowDir(cmd))
	shadowRoot := ""
	if resolvedShadowDir != "" {
		shadowRoot, err = system.ResolveWorkDir(resolvedShadowDir)
		if err != nil {
			return nil, nil, fmt.Errorf("loadTaskManager() [task.go]: failed to resolve shadow directory: %w", err)
		}
	}

	worktreesBaseDir := workDir
	if shadowRoot != "" {
		worktreesBaseDir = shadowRoot
	}

	resolvedTaskDir, err := resolveTaskDirPath(cmd, workDir)
	if err != nil {
		return nil, nil, err
	}

	store, err := GetCompositeConfigStore()
	if err != nil {
		return nil, nil, err
	}

	allowedPaths := []string(nil)
	if shadowRoot != "" {
		allowedPaths = append(allowedPaths, shadowRoot)
	}

	vcsRepo, _, err := prepareTaskSessionVFSFunc(workDir, worktreesBaseDir, "", nil, "", "", allowedPaths)
	if err != nil {
		return nil, nil, fmt.Errorf("loadTaskManager() [task.go]: failed to prepare vcs: %w", err)
	}

	manager, err := core.NewTaskManagerWithTasksDir(workDir, resolvedTaskDir, store)
	if err != nil {
		return nil, nil, err
	}

	return manager, vcsRepo, nil
}

func resolveTaskDirPath(cmd *cobra.Command, workDir string) (string, error) {
	if cmd == nil {
		return "", fmt.Errorf("resolveTaskDirPath() [task.go]: command cannot be nil")
	}

	flagTaskDir := ""
	flag := cmd.Flag("task-dir")
	if flag != nil {
		flagTaskDir = strings.TrimSpace(flag.Value.String())
	}

	resolvedTaskDir := strings.TrimSpace(flagTaskDir)
	resolvedShadowDir := strings.TrimSpace(resolveShadowDir(cmd))
	if resolvedTaskDir == "" {
		defaults, defaultsErr := resolveTaskRunDefaultsFunc(system.ResolveRunDefaultsParams{
			WorkDir:       workDir,
			ShadowDir:     resolvedShadowDir,
			ProjectConfig: projectConfig,
			ConfigPath:    configPath,
		})
		if defaultsErr != nil {
			return "", fmt.Errorf("resolveTaskDirPath() [task.go]: failed to resolve CLI defaults: %w", defaultsErr)
		}
		resolvedTaskDir = strings.TrimSpace(defaults.TaskDir)
	}

	if resolvedTaskDir == "" {
		if resolvedShadowDir != "" {
			shadowRoot, shadowErr := system.ResolveWorkDir(resolvedShadowDir)
			if shadowErr != nil {
				return "", fmt.Errorf("resolveTaskDirPath() [task.go]: failed to resolve shadow directory: %w", shadowErr)
			}
			resolvedTaskDir = filepath.Join(shadowRoot, ".cswdata", "tasks")
		} else {
			resolvedTaskDir = ".cswdata/tasks"
		}
	}
	if !filepath.IsAbs(resolvedTaskDir) {
		resolvedTaskDir = filepath.Join(workDir, resolvedTaskDir)
	}

	return filepath.Clean(resolvedTaskDir), nil
}

type taskCreateResolveParams struct {
	Prompt        string
	Name          string
	Description   string
	Branch        string
	ParentBranch  string
	Role          string
	ParentTaskID  string
	Deps          []string
	ModelName     string
	WorkDir       string
	ShadowDir     string
	ProjectConfig string
	ConfigPath    string
}

func isTaskEditorAvailable(command string) bool {
	tokens := strings.Fields(strings.TrimSpace(command))
	if len(tokens) == 0 {
		return false
	}
	executable := strings.TrimSpace(tokens[0])
	if executable == "" {
		return false
	}

	if filepath.IsAbs(executable) {
		info, err := os.Stat(executable)
		if err != nil {
			return false
		}
		return !info.IsDir()
	}

	_, err := exec.LookPath(executable)
	return err == nil
}

// shellQuote returns a shell-safe single-quoted value.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func printTaskRunOutcome(outcome tool.TaskRunOutcome) {
	fmt.Fprintf(os.Stdout, "Task run session: %s\n", strings.TrimSpace(outcome.SessionID))
	fmt.Fprintf(os.Stdout, "Task branch: %s\n", strings.TrimSpace(outcome.TaskBranchName))
	if strings.TrimSpace(outcome.SummaryText) != "" {
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintln(os.Stdout, strings.TrimSpace(outcome.SummaryText))
	}
}
