package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var resolveTaskRunDefaultsFunc = system.ResolveRunDefaults
var resolveTaskWorktreeBranchNameFunc = system.ResolveWorktreeBranchName
var buildTaskSystemFunc = system.BuildSystem
var taskEditorLookPathFunc = exec.LookPath
var runTaskEditorFunc = runTaskEditor
var generateTaskDescriptionFunc = generateTaskDescription
var taskDirUUIDPattern = regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

type taskDirectSessionRunner struct {
	baseDir       string
	modelName     string
	configPath    string
	projectConfig string
	thinking      string
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
}

func newTaskDirectSessionRunner(baseDir string, modelName string, configPath string, projectConfig string, thinking string) (*taskDirectSessionRunner, error) {
	trimmedBaseDir := strings.TrimSpace(baseDir)
	if trimmedBaseDir == "" {
		return nil, fmt.Errorf("newTaskDirectSessionRunner() [task.go]: baseDir cannot be empty")
	}

	return &taskDirectSessionRunner{
		baseDir:       trimmedBaseDir,
		modelName:     strings.TrimSpace(modelName),
		configPath:    strings.TrimSpace(configPath),
		projectConfig: strings.TrimSpace(projectConfig),
		thinking:      strings.TrimSpace(thinking),
		stdin:         os.Stdin,
		stdout:        os.Stdout,
		stderr:        os.Stderr,
	}, nil
}

// RunTaskSession runs task session directly without spawning another process.
func (r *taskDirectSessionRunner) RunTaskSession(ctx context.Context, request core.TaskSessionRunRequest) (core.TaskSessionRunResult, error) {
	if r == nil {
		return core.TaskSessionRunResult{}, fmt.Errorf("taskDirectSessionRunner.RunTaskSession() [task.go]: runner is nil")
	}
	result, err := RunInProcessTaskSession(ctx, request, TaskRunDefaults{
		WorkDir:       strings.TrimSpace(r.baseDir),
		ModelName:     strings.TrimSpace(r.modelName),
		ConfigPath:    strings.TrimSpace(r.configPath),
		ProjectConfig: strings.TrimSpace(r.projectConfig),
		Thinking:      strings.TrimSpace(r.thinking),
	}, RunStreams{
		Stdin:  r.stdin,
		Stdout: r.stdout,
		Stderr: r.stderr,
	})
	if err != nil {
		return result, fmt.Errorf("taskDirectSessionRunner.RunTaskSession() [task.go]: direct task run failed: %w", err)
	}

	return result, nil
}

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

func taskNewCommand() *cobra.Command {
	var name string
	var description string
	var branch string
	var parentBranch string
	var role string
	var deps []string
	var prompt string
	var parent string
	var cliModel string
	var cliEditor string
	var cliWorkDir string
	var cliShadowDir string
	var cliConfigPath string
	var cliProjectConfig string

	command := &cobra.Command{
		Use:   "new",
		Short: "Create new persistent task",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedPrompt, shouldCreate, err := resolveTaskNewPrompt(cmd.Context(), taskNewPromptParams{
				Prompt:        strings.TrimSpace(prompt),
				Editor:        strings.TrimSpace(cliEditor),
				WorkDir:       strings.TrimSpace(cliWorkDir),
				ShadowDir:     strings.TrimSpace(cliShadowDir),
				ProjectConfig: strings.TrimSpace(cliProjectConfig),
				ConfigPath:    strings.TrimSpace(cliConfigPath),
			})
			if err != nil {
				return err
			}
			if !shouldCreate {
				fmt.Fprintln(os.Stdout, "Task not created: prompt is empty")
				return nil
			}

			createParams, err := resolveTaskCreateParams(cmd.Context(), taskCreateResolveParams{
				Prompt:        resolvedPrompt,
				Name:          strings.TrimSpace(name),
				Description:   strings.TrimSpace(description),
				Branch:        strings.TrimSpace(branch),
				ParentBranch:  strings.TrimSpace(parentBranch),
				Role:          strings.TrimSpace(role),
				ParentTaskID:  strings.TrimSpace(parent),
				Deps:          append([]string(nil), deps...),
				ModelName:     strings.TrimSpace(cliModel),
				WorkDir:       strings.TrimSpace(cliWorkDir),
				ShadowDir:     strings.TrimSpace(cliShadowDir),
				ProjectConfig: strings.TrimSpace(cliProjectConfig),
				ConfigPath:    strings.TrimSpace(cliConfigPath),
			})
			if err != nil {
				return err
			}

			manager, _, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			created, err := manager.CreateTask(createParams)
			if err != nil {
				return err
			}

			printTaskCreated(created)
			return nil
		},
	}

	command.Flags().StringVarP(&name, "name", "n", "", "Task name/slug")
	command.Flags().StringVarP(&description, "description", "d", "", "Task description")
	command.Flags().StringVarP(&branch, "branch", "b", "", "Feature branch name")
	command.Flags().StringVarP(&parentBranch, "parent-branch", "B", "", "Parent branch name")
	command.Flags().StringVarP(&role, "role", "r", "", "Task role")
	command.Flags().StringArrayVarP(&deps, "depends", "D", nil, "Dependency task UUID (repeatable)")
	command.Flags().StringVarP(&prompt, "prompt", "p", "", "Task prompt")
	command.Flags().StringVar(&cliModel, "model", "", "Model alias or model spec in provider/model format (single or comma-separated fallback list); if not set, uses defaults")
	command.Flags().StringVar(&cliEditor, "editor", "", "Editor command used for interactive prompt creation")
	command.Flags().StringVar(&cliWorkDir, "workdir", "", "Working directory (default: current directory)")
	command.Flags().StringVar(&cliShadowDir, "shadow-dir", "", "Shadow directory for agent files overlay (AGENTS.md, .agents*, .csw*, .cswdata)")
	command.Flags().StringVar(&cliConfigPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	command.Flags().StringVar(&cliProjectConfig, "project-config", "", "Custom project config directory (default: .csw/config)")
	command.Flags().StringVar(&parent, "parent", "", "Parent task name or UUID")

	return command
}

func taskUpdateCommand() *cobra.Command {
	return taskUpdateCommandWithDefaults("update [name|uuid]", "Update existing task", false)
}

func taskEditCommand() *cobra.Command {
	return taskUpdateCommandWithDefaults("edit [name|uuid]", "Edit existing task prompt (shortcut for update --edit)", true)
}

func taskUpdateCommandWithDefaults(use string, short string, defaultEdit bool) *cobra.Command {
	var name string
	var description string
	var status string
	var branch string
	var parentBranch string
	var role string
	var deps []string
	var prompt string
	var last bool
	var next bool
	edit := defaultEdit
	var cliEditor string
	var regen bool
	var regenBranch bool
	var regenName bool
	var regenDescription bool

	command := &cobra.Command{
		Use:   strings.TrimSpace(use),
		Short: strings.TrimSpace(short),
		Args: func(cmd *cobra.Command, args []string) error {
			if last && next {
				return fmt.Errorf("taskUpdateCommand.Args() [task.go]: --last and --next cannot be used together")
			}
			if !last && !next {
				return cobra.ExactArgs(1)(cmd, args)
			}
			if len(args) > 0 {
				return fmt.Errorf("taskUpdateCommand.Args() [task.go]: task identifier cannot be used with --last or --next")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if edit && cmd.Flags().Changed("prompt") {
				return fmt.Errorf("taskUpdateCommand.RunE() [task.go]: --edit and --prompt cannot be used together")
			}

			manager, _, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			argIdentifier := ""
			if len(args) > 0 {
				argIdentifier = strings.TrimSpace(args[0])
			}
			identifier, err := resolveTaskRunIdentifier(manager, argIdentifier, last, next)
			if err != nil {
				return err
			}

			taskDir, taskData, err := manager.ResolveTask(core.TaskLookup{Identifier: identifier})
			if err != nil {
				return err
			}

			currentPrompt, err := readTaskPromptFile(taskDir)
			if err != nil {
				return err
			}

			params := core.TaskUpdateParams{Identifier: identifier}
			if cmd.Flags().Changed("name") {
				value := strings.TrimSpace(name)
				params.Name = &value
			}
			if cmd.Flags().Changed("description") {
				value := strings.TrimSpace(description)
				params.Description = &value
			}
			if cmd.Flags().Changed("status") {
				value := strings.TrimSpace(status)
				params.Status = &value
			}
			if cmd.Flags().Changed("branch") {
				value := strings.TrimSpace(branch)
				params.FeatureBranch = &value
			}
			if cmd.Flags().Changed("parent-branch") {
				value := strings.TrimSpace(parentBranch)
				params.ParentBranch = &value
			}
			if cmd.Flags().Changed("role") {
				value := strings.TrimSpace(role)
				params.Role = &value
			}
			if cmd.Flags().Changed("depends") {
				value := append([]string(nil), deps...)
				params.Deps = &value
			}
			resolvedPrompt := currentPrompt
			if edit {
				editorCommand, editorErr := resolveTaskEditorCommand(taskNewPromptParams{Editor: strings.TrimSpace(cliEditor)})
				if editorErr != nil {
					return editorErr
				}

				editedPrompt, promptChanged, editErr := editTaskPrompt(cmd.Context(), editorCommand, currentPrompt)
				if editErr != nil {
					return editErr
				}
				resolvedPrompt = editedPrompt
				if promptChanged {
					value := strings.TrimSpace(editedPrompt)
					params.Prompt = &value
				}
			} else if cmd.Flags().Changed("prompt") {
				value := strings.TrimSpace(prompt)
				params.Prompt = &value
				resolvedPrompt = value
			}

			if regen {
				regenBranch = true
				regenName = true
				regenDescription = true
			}

			if regenBranch || regenName || regenDescription {
				resolvedCreateParams, resolveErr := resolveTaskCreateParams(cmd.Context(), taskCreateResolveParams{
					Prompt:        resolvedPrompt,
					Name:          pickTaskRegenValue(taskData.Name, "", regenName),
					Description:   pickTaskRegenValue(taskData.Description, "", regenDescription),
					Branch:        pickTaskRegenValue(taskData.FeatureBranch, "", regenBranch),
					ParentBranch:  taskData.ParentBranch,
					Role:          taskData.Role,
					Deps:          append([]string(nil), taskData.Deps...),
					ModelName:     "",
					WorkDir:       "",
					ShadowDir:     "",
					ProjectConfig: "",
					ConfigPath:    "",
				})
				if resolveErr != nil {
					return resolveErr
				}

				if regenBranch {
					value := strings.TrimSpace(resolvedCreateParams.FeatureBranch)
					params.FeatureBranch = &value
				}
				if regenName {
					value := strings.TrimSpace(resolvedCreateParams.Name)
					params.Name = &value
				}
				if regenDescription {
					value := strings.TrimSpace(resolvedCreateParams.Description)
					params.Description = &value
				}
			}

			updated, err := manager.UpdateTask(params)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stdout, "Task updated: %s\n", updated.UUID)
			return nil
		},
	}

	command.Flags().StringVarP(&name, "name", "n", "", "Task name/slug")
	command.Flags().StringVarP(&description, "description", "d", "", "Task description")
	command.Flags().StringVar(&status, "status", "", "Task status (draft, created, open, running, merged)")
	command.Flags().StringVarP(&branch, "branch", "b", "", "Feature branch name")
	command.Flags().StringVarP(&parentBranch, "parent-branch", "B", "", "Parent branch name")
	command.Flags().StringVarP(&role, "role", "r", "", "Task role")
	command.Flags().StringArrayVarP(&deps, "depends", "D", nil, "Dependency task UUID (repeatable)")
	command.Flags().StringVarP(&prompt, "prompt", "p", "", "Task prompt")
	command.Flags().BoolVar(&last, "last", false, "Update latest unfinished task")
	command.Flags().BoolVar(&next, "next", false, "Update oldest unfinished task")
	command.Flags().BoolVar(&edit, "edit", defaultEdit, "Edit task prompt in editor")
	command.Flags().StringVar(&cliEditor, "editor", "", "Editor command used for interactive prompt editing")
	command.Flags().BoolVar(&regen, "regen", false, "Regenerate task name, branch and description")
	command.Flags().BoolVar(&regenBranch, "regen-branch", false, "Regenerate feature branch")
	command.Flags().BoolVar(&regenName, "regen-name", false, "Regenerate task name")
	command.Flags().BoolVar(&regenDescription, "regen-description", false, "Regenerate task description")

	return command
}

func readTaskPromptFile(taskDir string) (string, error) {
	taskPromptPath := filepath.Join(strings.TrimSpace(taskDir), "task.md")
	promptBytes, err := os.ReadFile(taskPromptPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("readTaskPromptFile() [task.go]: failed to read task prompt: %w", err)
	}

	return strings.TrimSpace(string(promptBytes)), nil
}

func editTaskPrompt(ctx context.Context, editorCommand string, currentPrompt string) (string, bool, error) {
	temporaryFile, err := os.CreateTemp("", "csw-task-update-*.md")
	if err != nil {
		return "", false, fmt.Errorf("editTaskPrompt() [task.go]: failed to create temporary prompt file: %w", err)
	}
	temporaryFilePath := temporaryFile.Name()

	initialPrompt := strings.TrimSpace(currentPrompt)
	if initialPrompt != "" {
		if _, writeErr := temporaryFile.WriteString(initialPrompt + "\n"); writeErr != nil {
			_ = temporaryFile.Close()
			_ = os.Remove(temporaryFilePath)
			return "", false, fmt.Errorf("editTaskPrompt() [task.go]: failed to write initial prompt: %w", writeErr)
		}
	}

	if closeErr := temporaryFile.Close(); closeErr != nil {
		_ = os.Remove(temporaryFilePath)
		return "", false, fmt.Errorf("editTaskPrompt() [task.go]: failed to close temporary prompt file: %w", closeErr)
	}
	defer func() {
		_ = os.Remove(temporaryFilePath)
	}()

	if err := runTaskEditorFunc(ctx, editorCommand, temporaryFilePath); err != nil {
		return "", false, err
	}

	editedPromptBytes, err := os.ReadFile(temporaryFilePath)
	if err != nil {
		return "", false, fmt.Errorf("editTaskPrompt() [task.go]: failed to read edited prompt: %w", err)
	}
	editedPrompt := strings.TrimSpace(string(editedPromptBytes))

	return editedPrompt, editedPrompt != initialPrompt, nil
}

func pickTaskRegenValue(current string, regenerated string, shouldRegenerate bool) string {
	if shouldRegenerate {
		return regenerated
	}

	return strings.TrimSpace(current)
}

func taskGetCommand() *cobra.Command {
	var asJSON bool
	var asYAML bool
	var includeSummary bool

	command := &cobra.Command{
		Use:   "get {name|uuid}",
		Short: "Get task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, _, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}
			if asJSON && asYAML {
				return fmt.Errorf("taskGetCommand.RunE() [task.go]: --json and --yaml cannot be used together")
			}

			taskData, summaryMeta, summaryText, err := manager.GetTask(core.TaskLookup{Identifier: strings.TrimSpace(args[0])}, includeSummary)
			if err != nil {
				return err
			}

			if asJSON {
				payload := map[string]any{"task": taskData}
				if summaryMeta != nil {
					payload["summary_meta"] = summaryMeta
				}
				if strings.TrimSpace(summaryText) != "" {
					payload["summary"] = strings.TrimSpace(summaryText)
				}
				return outputJSON(payload)
			}

			if asYAML {
				payload := map[string]any{"task": taskData}
				if summaryMeta != nil {
					payload["summary_meta"] = summaryMeta
				}
				if strings.TrimSpace(summaryText) != "" {
					payload["summary"] = strings.TrimSpace(summaryText)
				}
				content, marshalErr := yaml.Marshal(payload)
				if marshalErr != nil {
					return fmt.Errorf("taskGetCommand.RunE() [task.go]: failed to marshal yaml: %w", marshalErr)
				}
				_, _ = fmt.Fprint(os.Stdout, string(content))
				return nil
			}

			printTaskHuman(taskData, summaryMeta, summaryText)
			return nil
		},
	}

	command.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	command.Flags().BoolVar(&asYAML, "yaml", false, "Output as YAML")
	command.Flags().BoolVar(&includeSummary, "summary", false, "Include latest session summary")

	return command
}

func resolveTaskRunIdentifier(manager *core.TaskManager, identifier string, useLast bool, useNext bool) (string, error) {
	if useLast && useNext {
		return "", fmt.Errorf("resolveTaskRunIdentifier() [task.go]: --last and --next cannot be used together")
	}
	if (useLast || useNext) && strings.TrimSpace(identifier) != "" {
		return "", fmt.Errorf("resolveTaskRunIdentifier() [task.go]: task identifier cannot be used with --last or --next")
	}

	if useLast || useNext {
		if manager == nil {
			return "", fmt.Errorf("resolveTaskRunIdentifier() [task.go]: manager cannot be nil")
		}

		taskData, err := findRunnableTaskByModTime(manager, useLast)
		if err != nil {
			return "", err
		}

		return strings.TrimSpace(taskData.UUID), nil
	}

	trimmedIdentifier := strings.TrimSpace(identifier)
	if trimmedIdentifier == "" {
		return "", fmt.Errorf("resolveTaskRunIdentifier() [task.go]: task identifier cannot be empty")
	}

	return trimmedIdentifier, nil
}

func findRunnableTaskByModTime(manager *core.TaskManager, newest bool) (*core.Task, error) {
	tasks, err := listAllCurrentTasks(manager)
	if err != nil {
		return nil, err
	}

	modTimes, err := collectTaskYMLModTimes(manager.TasksRoot())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		modTimes = map[string]int64{}
	}

	var selected *core.Task
	selectedModTime := int64(0)
	for _, taskData := range tasks {
		if !isUnfinishedTaskForRun(taskData) {
			continue
		}

		taskID := strings.TrimSpace(taskData.UUID)
		currentModTime := modTimes[taskID]
		if selected == nil {
			selected = taskData
			selectedModTime = currentModTime
			continue
		}

		isBetter := false
		if newest {
			isBetter = currentModTime > selectedModTime || (currentModTime == selectedModTime && taskID > strings.TrimSpace(selected.UUID))
		} else {
			isBetter = currentModTime < selectedModTime || (currentModTime == selectedModTime && taskID < strings.TrimSpace(selected.UUID))
		}

		if isBetter {
			selected = taskData
			selectedModTime = currentModTime
		}
	}

	if selected == nil {
		return nil, fmt.Errorf("findRunnableTaskByModTime() [task.go]: no unfinished task found")
	}

	return selected, nil
}

func listAllCurrentTasks(manager *core.TaskManager) ([]*core.Task, error) {
	if manager == nil {
		return nil, fmt.Errorf("listAllCurrentTasks() [task.go]: manager cannot be nil")
	}

	topLevelTasks, err := manager.ListTasks(core.TaskLookup{}, false)
	if err != nil {
		return nil, err
	}

	allTasks := make([]*core.Task, 0, len(topLevelTasks))
	for _, topLevelTask := range topLevelTasks {
		if topLevelTask == nil {
			continue
		}
		allTasks = append(allTasks, topLevelTask)

		children, childErr := manager.ListTasks(core.TaskLookup{Identifier: strings.TrimSpace(topLevelTask.UUID)}, true)
		if childErr != nil {
			return nil, childErr
		}
		allTasks = append(allTasks, children...)
	}

	return allTasks, nil
}

func isUnfinishedTaskForRun(taskData *core.Task) bool {
	if taskData == nil {
		return false
	}

	status := strings.TrimSpace(taskData.Status)
	if status == core.TaskStatusMerged || status == core.TaskStatusRunning || status == core.TaskStatusDraft {
		return false
	}

	return true
}

func taskListCommand() *cobra.Command {
	var recursive bool
	var includeArchived bool
	var statusFilter string

	command := &cobra.Command{
		Use:   "list [name|uuid]",
		Short: "List tasks",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, _, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}
			return runTaskList(manager, args, recursive, includeArchived, statusFilter, os.Stdout)
		},
	}

	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "List recursively")
	command.Flags().BoolVar(&includeArchived, "archived", false, "Include archived tasks")
	command.Flags().StringVar(&statusFilter, "status", "", "Filter by status list (e.g. open,merged) or exclude one/more statuses with !status")

	return command
}

func runTaskList(manager *core.TaskManager, args []string, recursive bool, includeArchived bool, statusFilter string, output io.Writer) error {
	if manager == nil {
		return fmt.Errorf("runTaskList() [task.go]: manager cannot be nil")
	}
	if output == nil {
		return fmt.Errorf("runTaskList() [task.go]: output cannot be nil")
	}

	lookup := core.TaskLookup{}
	if len(args) == 1 {
		lookup.Identifier = strings.TrimSpace(args[0])
	}

	statuses, exclude, err := parseTaskStatusFilter(statusFilter)
	if err != nil {
		return err
	}

	tasks, err := manager.ListTasks(lookup, recursive)
	if err != nil {
		if !(includeArchived && isTaskNotFoundError(err)) {
			return err
		}
		tasks = []*core.Task{}
	}

	modTimes := map[string]int64{}
	currentTimes, err := collectTaskYMLModTimes(manager.TasksRoot())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		for key, value := range currentTimes {
			modTimes[key] = value
		}
	}

	if includeArchived {
		archivedManager, createErr := core.NewTaskManagerWithTasksDir(manager.TasksRoot(), manager.ArchivedTasksRoot(), nil, nil)
		if createErr != nil {
			return fmt.Errorf("runTaskList() [task.go]: failed to prepare archived task manager: %w", createErr)
		}
		archivedTasks, archivedErr := archivedManager.ListTasks(lookup, recursive)
		if archivedErr != nil {
			if !isTaskNotFoundError(archivedErr) {
				return archivedErr
			}
		} else {
			tasks = append(tasks, archivedTasks...)
		}

		archivedTimes, archivedTimesErr := collectTaskYMLModTimes(manager.ArchivedTasksRoot())
		if archivedTimesErr != nil {
			if !errors.Is(archivedTimesErr, os.ErrNotExist) {
				return archivedTimesErr
			}
		} else {
			for key, value := range archivedTimes {
				modTimes[key] = value
			}
		}
	}

	filtered := make([]*core.Task, 0, len(tasks))
	for _, item := range tasks {
		if item == nil {
			continue
		}
		if !matchesStatusFilter(strings.TrimSpace(item.Status), statuses, exclude) {
			continue
		}
		filtered = append(filtered, item)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i]
		right := filtered[j]
		if left == nil || right == nil {
			return i < j
		}

		leftTime := modTimes[strings.TrimSpace(left.UUID)]
		rightTime := modTimes[strings.TrimSpace(right.UUID)]
		if leftTime == rightTime {
			return strings.TrimSpace(left.UUID) < strings.TrimSpace(right.UUID)
		}

		return leftTime < rightTime
	})

	for _, item := range filtered {
		if item == nil {
			continue
		}
		_, _ = fmt.Fprintf(output, "%s\t%s\t%s\n", item.UUID, item.Status, item.Description)
	}

	return nil
}

func collectTaskYMLModTimes(tasksRoot string) (map[string]int64, error) {
	result := map[string]int64{}
	trimmedRoot := strings.TrimSpace(tasksRoot)
	if trimmedRoot == "" {
		return result, fmt.Errorf("collectTaskYMLModTimes() [task.go]: tasks root cannot be empty")
	}

	if _, err := os.Stat(trimmedRoot); err != nil {
		return nil, fmt.Errorf("collectTaskYMLModTimes() [task.go]: failed to stat tasks root: %w", err)
	}

	walkErr := filepath.WalkDir(trimmedRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry != nil && entry.IsDir() {
			if path != trimmedRoot && !taskDirUUIDPattern.MatchString(strings.TrimSpace(entry.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry == nil || entry.IsDir() || strings.TrimSpace(entry.Name()) != "task.yml" {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}

		taskUUID := strings.TrimSpace(filepath.Base(filepath.Dir(path)))
		if taskUUID == "" {
			return nil
		}
		result[taskUUID] = info.ModTime().UnixNano()
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("collectTaskYMLModTimes() [task.go]: failed to collect task modification times: %w", walkErr)
	}

	return result, nil
}

func parseTaskStatusFilter(rawFilter string) (map[string]struct{}, bool, error) {
	trimmedFilter := strings.TrimSpace(rawFilter)
	if trimmedFilter == "" {
		return nil, false, nil
	}

	exclude := false
	if strings.HasPrefix(trimmedFilter, "!") {
		exclude = true
		trimmedFilter = strings.TrimSpace(strings.TrimPrefix(trimmedFilter, "!"))
	}
	if trimmedFilter == "" {
		return nil, false, fmt.Errorf("parseTaskStatusFilter() [task.go]: status filter cannot be empty")
	}

	parts := strings.Split(trimmedFilter, ",")
	statuses := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		status := strings.TrimSpace(part)
		if status == "" {
			return nil, false, fmt.Errorf("parseTaskStatusFilter() [task.go]: status filter contains empty value")
		}
		statuses[status] = struct{}{}
	}

	return statuses, exclude, nil
}

func matchesStatusFilter(taskStatus string, statuses map[string]struct{}, exclude bool) bool {
	if len(statuses) == 0 {
		return true
	}

	_, exists := statuses[strings.TrimSpace(taskStatus)]
	if exclude {
		return !exists
	}

	return exists
}

func isTaskNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

func taskMergeCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "merge {name|uuid}",
		Short: "Merge task feature branch to parent branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, backend, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			merged, err := backend.MergeTask(cmd.Context(), strings.TrimSpace(args[0]), "")
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Task merged: %s (%s -> %s)\n", merged.UUID, merged.FeatureBranch, merged.ParentBranch)
			return nil
		},
	}

	return command
}

func taskArchiveCommand() *cobra.Command {
	var status string

	command := &cobra.Command{
		Use:   "archive [name|uuid]",
		Short: "Archive tasks",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, _, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			return runTaskArchive(manager, args, status, os.Stdout)
		},
	}

	command.Flags().StringVar(&status, "status", "", "Archive all tasks by status")

	return command
}

func runTaskArchive(manager *core.TaskManager, args []string, status string, output io.Writer) error {
	if manager == nil {
		return fmt.Errorf("runTaskArchive() [task.go]: manager cannot be nil")
	}
	if output == nil {
		return fmt.Errorf("runTaskArchive() [task.go]: output cannot be nil")
	}

	trimmedStatus := strings.TrimSpace(status)
	if len(args) == 1 && trimmedStatus != "" {
		return fmt.Errorf("runTaskArchive() [task.go]: task identifier and --status cannot be used together")
	}

	if len(args) == 1 {
		archivedTask, archiveErr := manager.ArchiveTask(core.TaskLookup{Identifier: strings.TrimSpace(args[0])})
		if archiveErr != nil {
			return archiveErr
		}
		_, _ = fmt.Fprintf(output, "Task archived: %s\t%s\n", archivedTask.UUID, archivedTask.Name)
		return nil
	}

	if trimmedStatus == "" {
		trimmedStatus = core.TaskStatusMerged
	}
	archivedTasks, archiveErr := manager.ArchiveTasksByStatus(trimmedStatus)
	if archiveErr != nil {
		return archiveErr
	}

	sort.Slice(archivedTasks, func(i, j int) bool {
		if archivedTasks[i] == nil || archivedTasks[j] == nil {
			return i < j
		}
		if archivedTasks[i].Name == archivedTasks[j].Name {
			return archivedTasks[i].UUID < archivedTasks[j].UUID
		}
		return archivedTasks[i].Name < archivedTasks[j].Name
	})
	for _, archivedTask := range archivedTasks {
		if archivedTask == nil {
			continue
		}
		_, _ = fmt.Fprintf(output, "Task archived: %s\t%s\n", archivedTask.UUID, archivedTask.Name)
	}

	return nil
}

func loadTaskBackend(cmd *cobra.Command) (*core.TaskManager, *core.TaskBackendAdapter, error) {
	workDir, err := system.ResolveWorkDir("")
	if err != nil {
		return nil, nil, err
	}

	resolvedTaskDir, err := resolveTaskDirPath(cmd, workDir)
	if err != nil {
		return nil, nil, err
	}

	store, err := GetCompositeConfigStore()
	if err != nil {
		return nil, nil, err
	}

	vcsRepo, _, err := system.PrepareSessionVFS(workDir, workDir, "", nil, "", "", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("loadTaskBackend() [task.go]: failed to prepare vcs: %w", err)
	}

	runner, err := newTaskDirectSessionRunner(workDir, modelName, configPath, projectConfig, "")
	if err != nil {
		return nil, nil, err
	}

	manager, err := core.NewTaskManagerWithTasksDir(workDir, resolvedTaskDir, store, runner)
	if err != nil {
		return nil, nil, err
	}

	backend, err := core.NewTaskBackendAdapter(manager, vcsRepo, nil)
	if err != nil {
		return nil, nil, err
	}

	return manager, backend, nil
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
	if resolvedTaskDir == "" {
		defaults, defaultsErr := resolveTaskRunDefaultsFunc(system.ResolveRunDefaultsParams{
			WorkDir:       workDir,
			ProjectConfig: projectConfig,
			ConfigPath:    configPath,
		})
		if defaultsErr != nil {
			return "", fmt.Errorf("resolveTaskDirPath() [task.go]: failed to resolve CLI defaults: %w", defaultsErr)
		}
		resolvedTaskDir = strings.TrimSpace(defaults.TaskDir)
	}

	if resolvedTaskDir == "" {
		resolvedTaskDir = ".cswdata/tasks"
	}
	if !filepath.IsAbs(resolvedTaskDir) {
		resolvedTaskDir = filepath.Join(workDir, resolvedTaskDir)
	}

	return filepath.Clean(resolvedTaskDir), nil
}

type taskNewPromptParams struct {
	Prompt        string
	Editor        string
	WorkDir       string
	ShadowDir     string
	ProjectConfig string
	ConfigPath    string
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

func resolveTaskNewPrompt(ctx context.Context, params taskNewPromptParams) (string, bool, error) {
	prompt := strings.TrimSpace(params.Prompt)
	if prompt != "" {
		return prompt, true, nil
	}

	workDir, err := system.ResolveWorkDir(strings.TrimSpace(params.WorkDir))
	if err != nil {
		return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to resolve work directory: %w", err)
	}

	editorCommand, err := resolveTaskEditorCommand(taskNewPromptParams{
		Editor:        strings.TrimSpace(params.Editor),
		WorkDir:       workDir,
		ShadowDir:     strings.TrimSpace(params.ShadowDir),
		ProjectConfig: strings.TrimSpace(params.ProjectConfig),
		ConfigPath:    strings.TrimSpace(params.ConfigPath),
	})
	if err != nil {
		return "", false, err
	}

	temporaryDir, err := resolveTaskTempDir(workDir, strings.TrimSpace(params.ShadowDir))
	if err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(temporaryDir, 0o755); err != nil {
		return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to create temporary directory: %w", err)
	}

	temporaryFile, err := os.CreateTemp(temporaryDir, "csw-task-new-*.md")
	if err != nil {
		return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to create temporary prompt file: %w", err)
	}
	temporaryFilePath := temporaryFile.Name()
	if closeErr := temporaryFile.Close(); closeErr != nil {
		_ = os.Remove(temporaryFilePath)
		return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to close temporary prompt file: %w", closeErr)
	}
	defer func() {
		_ = os.Remove(temporaryFilePath)
	}()

	if err := runTaskEditorFunc(ctx, editorCommand, temporaryFilePath); err != nil {
		return "", false, err
	}

	promptBytes, err := os.ReadFile(temporaryFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to read temporary prompt file: %w", err)
	}

	resolvedPrompt := strings.TrimSpace(string(promptBytes))
	if resolvedPrompt == "" {
		return "", false, nil
	}

	return resolvedPrompt, true, nil
}

func resolveTaskTempDir(workDir string, shadowDir string) (string, error) {
	resolvedWorkDir, err := system.ResolveWorkDir(strings.TrimSpace(workDir))
	if err != nil {
		return "", fmt.Errorf("resolveTaskTempDir() [task.go]: failed to resolve work directory: %w", err)
	}

	configRoot := resolvedWorkDir
	if strings.TrimSpace(shadowDir) != "" {
		resolvedShadowDir, shadowErr := system.ResolveWorkDir(strings.TrimSpace(shadowDir))
		if shadowErr != nil {
			return "", fmt.Errorf("resolveTaskTempDir() [task.go]: failed to resolve shadow directory: %w", shadowErr)
		}
		configRoot = resolvedShadowDir
	}

	return filepath.Join(configRoot, ".cswdata", "tmp"), nil
}

func resolveTaskEditorCommand(params taskNewPromptParams) (string, error) {
	if trimmedEditor := strings.TrimSpace(params.Editor); trimmedEditor != "" {
		return trimmedEditor, nil
	}

	if envEditor := strings.TrimSpace(os.Getenv("EDITOR")); envEditor != "" {
		return envEditor, nil
	}

	defaults, err := resolveTaskRunDefaultsFunc(system.ResolveRunDefaultsParams{
		WorkDir:       strings.TrimSpace(params.WorkDir),
		ShadowDir:     strings.TrimSpace(params.ShadowDir),
		ProjectConfig: strings.TrimSpace(params.ProjectConfig),
		ConfigPath:    strings.TrimSpace(params.ConfigPath),
	})
	if err != nil {
		return "", fmt.Errorf("resolveTaskEditorCommand() [task.go]: failed to resolve CLI defaults: %w", err)
	}

	for _, candidate := range defaults.Editors {
		trimmedCandidate := strings.TrimSpace(candidate)
		if trimmedCandidate == "" {
			continue
		}
		if isTaskEditorAvailable(trimmedCandidate) {
			return trimmedCandidate, nil
		}
	}

	return "", fmt.Errorf("resolveTaskEditorCommand() [task.go]: no editor command found")
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

	_, err := taskEditorLookPathFunc(executable)
	return err == nil
}

func runTaskEditor(ctx context.Context, editorCommand string, promptFilePath string) error {
	editorTokens := strings.Fields(strings.TrimSpace(editorCommand))
	if len(editorTokens) == 0 {
		return fmt.Errorf("runTaskEditor() [task.go]: editor command cannot be empty")
	}

	commandArgs := append([]string(nil), editorTokens[1:]...)
	commandArgs = append(commandArgs, promptFilePath)
	editorProcess := exec.CommandContext(ctx, editorTokens[0], commandArgs...)
	editorProcess.Stdin = os.Stdin
	editorProcess.Stdout = os.Stdout
	editorProcess.Stderr = os.Stderr

	if err := editorProcess.Run(); err != nil {
		return fmt.Errorf("runTaskEditor() [task.go]: failed to run editor: %w", err)
	}

	return nil
}

func resolveTaskCreateParams(ctx context.Context, params taskCreateResolveParams) (core.TaskCreateParams, error) {
	resolvedPrompt := strings.TrimSpace(params.Prompt)
	if resolvedPrompt == "" {
		return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: prompt cannot be empty")
	}

	workDir, err := system.ResolveWorkDir(strings.TrimSpace(params.WorkDir))
	if err != nil {
		return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: failed to resolve work directory: %w", err)
	}

	defaults, err := resolveTaskRunDefaultsFunc(system.ResolveRunDefaultsParams{
		WorkDir:       workDir,
		ShadowDir:     strings.TrimSpace(params.ShadowDir),
		ProjectConfig: strings.TrimSpace(params.ProjectConfig),
		ConfigPath:    strings.TrimSpace(params.ConfigPath),
	})
	if err != nil {
		return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: failed to resolve CLI defaults: %w", err)
	}

	modelName := strings.TrimSpace(params.ModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(defaults.Model)
	}

	resolvedBranch := strings.TrimSpace(params.Branch)
	if resolvedBranch == "" {
		worktreeTemplate := strings.TrimSpace(defaults.Worktree)
		if worktreeTemplate == "" {
			worktreeTemplate = "%"
		}
		if !strings.HasSuffix(worktreeTemplate, "%") {
			worktreeTemplate += "-%"
		}

		generatedBranch, err := resolveTaskWorktreeBranchNameFunc(ctx, system.ResolveWorktreeBranchNameParams{
			Prompt:         resolvedPrompt,
			ModelName:      modelName,
			WorkDir:        workDir,
			ShadowDir:      strings.TrimSpace(params.ShadowDir),
			ProjectConfig:  strings.TrimSpace(params.ProjectConfig),
			ConfigPath:     strings.TrimSpace(params.ConfigPath),
			WorktreeBranch: worktreeTemplate,
		})
		if err != nil {
			return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: failed to resolve task branch: %w", err)
		}
		resolvedBranch = strings.TrimSpace(generatedBranch)
	}

	if resolvedBranch == "" {
		return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: resolved task branch cannot be empty")
	}

	resolvedName := strings.TrimSpace(params.Name)
	if resolvedName == "" {
		resolvedName = resolvedBranch
	}

	resolvedDescription := strings.TrimSpace(params.Description)
	if resolvedDescription == "" {
		generatedDescription, err := generateTaskDescriptionFunc(ctx, taskCreateResolveParams{
			Prompt:        resolvedPrompt,
			Branch:        resolvedBranch,
			Role:          strings.TrimSpace(params.Role),
			ModelName:     modelName,
			WorkDir:       workDir,
			ShadowDir:     strings.TrimSpace(params.ShadowDir),
			ProjectConfig: strings.TrimSpace(params.ProjectConfig),
			ConfigPath:    strings.TrimSpace(params.ConfigPath),
		})
		if err != nil {
			return core.TaskCreateParams{}, err
		}
		resolvedDescription = strings.TrimSpace(generatedDescription)
	}

	return core.TaskCreateParams{
		ParentTaskID:  strings.TrimSpace(params.ParentTaskID),
		Name:          resolvedName,
		Description:   resolvedDescription,
		FeatureBranch: resolvedBranch,
		ParentBranch:  strings.TrimSpace(params.ParentBranch),
		Role:          strings.TrimSpace(params.Role),
		Deps:          append([]string(nil), params.Deps...),
		Prompt:        resolvedPrompt,
	}, nil
}

func generateTaskDescription(ctx context.Context, params taskCreateResolveParams) (string, error) {
	buildParams := system.BuildSystemParams{
		WorkDir:       strings.TrimSpace(params.WorkDir),
		ShadowDir:     strings.TrimSpace(params.ShadowDir),
		ConfigPath:    strings.TrimSpace(params.ConfigPath),
		ProjectConfig: strings.TrimSpace(params.ProjectConfig),
		ModelName:     strings.TrimSpace(params.ModelName),
		RoleName:      strings.TrimSpace(pickTaskRoleName(params.Role)),
	}

	sweSystem, buildResult, err := buildTaskSystemFunc(buildParams)
	if err != nil {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to build system: %w", err)
	}
	defer buildResult.Cleanup()

	modelRefs, err := models.ParseProviderModelChain(strings.TrimSpace(buildResult.ModelName))
	if err != nil || len(modelRefs) == 0 {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to parse resolved model name: %w", err)
	}

	seedSession := core.NewSweSession(&core.SweSessionParams{
		ProviderName: modelRefs[0].Provider,
		Model:        modelRefs[0].Model,
		ModelSpec:    strings.TrimSpace(buildResult.ModelName),
		Messages: []*models.ChatMessage{
			models.NewTextMessage(models.ChatRoleUser, strings.TrimSpace(params.Prompt)),
		},
	})

	generatedDescription, err := core.GenerateCommitMessage(ctx, sweSystem.ModelProviders, sweSystem.ConfigStore, seedSession, strings.TrimSpace(params.Branch), "")
	if err != nil {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to generate description: %w", err)
	}

	return strings.TrimSpace(generatedDescription), nil
}

func pickTaskRoleName(roleName string) string {
	trimmedRoleName := strings.TrimSpace(roleName)
	if trimmedRoleName == "" {
		return "developer"
	}

	return trimmedRoleName
}

func printTaskRunOutcome(outcome tool.TaskRunOutcome) {
	fmt.Fprintf(os.Stdout, "Task run session: %s\n", strings.TrimSpace(outcome.SessionID))
	fmt.Fprintf(os.Stdout, "Task branch: %s\n", strings.TrimSpace(outcome.TaskBranchName))
	if strings.TrimSpace(outcome.SummaryText) != "" {
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintln(os.Stdout, strings.TrimSpace(outcome.SummaryText))
	}
}

func printTaskCreated(taskData *core.Task) {
	if taskData == nil {
		return
	}

	fmt.Fprintf(os.Stdout, "Task created: %s\n", strings.TrimSpace(taskData.UUID))
	fmt.Fprintf(os.Stdout, "Description: %s\n", strings.TrimSpace(taskData.Description))
}

func printTaskHuman(taskData *core.Task, summaryMeta *core.TaskSessionSummary, summaryText string) {
	if taskData == nil {
		return
	}
	fmt.Fprintf(os.Stdout, "UUID: %s\n", taskData.UUID)
	fmt.Fprintf(os.Stdout, "Name: %s\n", taskData.Name)
	fmt.Fprintf(os.Stdout, "Description: %s\n", taskData.Description)
	fmt.Fprintf(os.Stdout, "Status: %s\n", taskData.Status)
	fmt.Fprintf(os.Stdout, "Feature branch: %s\n", taskData.FeatureBranch)
	fmt.Fprintf(os.Stdout, "Parent branch: %s\n", taskData.ParentBranch)
	fmt.Fprintf(os.Stdout, "Role: %s\n", taskData.Role)
	fmt.Fprintf(os.Stdout, "Parent task: %s\n", taskData.ParentTaskID)
	fmt.Fprintf(os.Stdout, "Deps: %s\n", strings.Join(taskData.Deps, ","))
	fmt.Fprintf(os.Stdout, "Sessions: %s\n", strings.Join(taskData.SessionIDs, ","))
	fmt.Fprintf(os.Stdout, "Subtasks: %s\n", strings.Join(taskData.SubtaskIDs, ","))
	fmt.Fprintf(os.Stdout, "Created: %s\n", taskData.CreatedAt)
	fmt.Fprintf(os.Stdout, "Updated: %s\n", taskData.UpdatedAt)
	if summaryMeta != nil {
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintf(os.Stdout, "Last summary session: %s (%s)\n", summaryMeta.SessionID, summaryMeta.Status)
		if strings.TrimSpace(summaryText) != "" {
			fmt.Fprintln(os.Stdout, strings.TrimSpace(summaryText))
		}
	}
}
