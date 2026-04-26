package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/spf13/cobra"
)

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

			manager, _, err := loadTaskManager(cmd)
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
				editorCommand := strings.TrimSpace(cliEditor)
				if editorCommand == "" {
					editorCommand = strings.TrimSpace(os.Getenv("EDITOR"))
				}
				if editorCommand == "" {
					for _, candidate := range []string{"editor", "vim", "mcedit", "nano"} {
						if isTaskEditorAvailable(candidate) {
							editorCommand = candidate
							break
						}
					}
				}
				if editorCommand == "" {
					return fmt.Errorf("taskUpdateCommand.RunE() [task.go]: no editor command found")
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
				resolvedShadowDir := strings.TrimSpace(shadowDir)
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
					ShadowDir:     resolvedShadowDir,
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

	editorProcess := exec.CommandContext(ctx, "sh", "-c", editorCommand+" "+shellQuote(temporaryFilePath))
	editorProcess.Stdin = os.Stdin
	editorProcess.Stdout = os.Stdout
	editorProcess.Stderr = os.Stderr

	if err := editorProcess.Run(); err != nil {
		return "", false, fmt.Errorf("editTaskPrompt() [task.go]: failed to run editor: %w", err)
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
