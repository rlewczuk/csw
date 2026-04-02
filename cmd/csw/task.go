package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// TaskCommand creates task command with persistent hierarchical task management.
func TaskCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "task",
		Short: "Manage persistent hierarchical tasks",
	}

	command.AddCommand(taskNewCommand())
	command.AddCommand(taskUpdateCommand())
	command.AddCommand(taskGetCommand())
	command.AddCommand(taskRunCommand())
	command.AddCommand(taskListCommand())
	command.AddCommand(taskMergeCommand())

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
	var run bool

	command := &cobra.Command{
		Use:   "new",
		Short: "Create new persistent task",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, backend, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			created, err := manager.CreateTask(core.TaskCreateParams{
				ParentTaskID:  strings.TrimSpace(parent),
				Name:          strings.TrimSpace(name),
				Description:   strings.TrimSpace(description),
				FeatureBranch: strings.TrimSpace(branch),
				ParentBranch:  strings.TrimSpace(parentBranch),
				Role:          strings.TrimSpace(role),
				Deps:          append([]string(nil), deps...),
				Prompt:        strings.TrimSpace(prompt),
			})
			if err != nil {
				return err
			}

			if !run {
				fmt.Fprintf(os.Stdout, "Task created: %s\n", created.UUID)
				return nil
			}

			outcome, runErr := backend.RunTask(cmd.Context(), strings.TrimSpace(created.UUID), "", false, false)
			printTaskRunOutcome(outcome)
			return runErr
		},
	}

	command.Flags().StringVarP(&name, "name", "n", "", "Task name/slug")
	command.Flags().StringVarP(&description, "description", "d", "", "Task description")
	command.Flags().StringVarP(&branch, "branch", "b", "", "Feature branch name")
	command.Flags().StringVarP(&parentBranch, "parent-branch", "B", "", "Parent branch name")
	command.Flags().StringVarP(&role, "role", "r", "", "Task role")
	command.Flags().StringArrayVarP(&deps, "depends", "D", nil, "Dependency task UUID (repeatable)")
	command.Flags().StringVarP(&prompt, "prompt", "p", "", "Task prompt")
	command.Flags().StringVar(&parent, "parent", "", "Parent task name or UUID")
	command.Flags().BoolVar(&run, "run", false, "Run created task immediately")
	_ = command.MarkFlagRequired("prompt")

	return command
}

func taskUpdateCommand() *cobra.Command {
	var name string
	var description string
	var branch string
	var parentBranch string
	var role string
	var deps []string
	var prompt string
	var run bool

	command := &cobra.Command{
		Use:   "update {name|uuid}",
		Short: "Update existing task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, backend, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			identifier := strings.TrimSpace(args[0])
			params := core.TaskUpdateParams{Identifier: identifier}
			if cmd.Flags().Changed("name") {
				value := strings.TrimSpace(name)
				params.Name = &value
			}
			if cmd.Flags().Changed("description") {
				value := strings.TrimSpace(description)
				params.Description = &value
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
			if cmd.Flags().Changed("prompt") {
				value := strings.TrimSpace(prompt)
				params.Prompt = &value
			}

			updated, err := manager.UpdateTask(params)
			if err != nil {
				return err
			}

			if !run {
				fmt.Fprintf(os.Stdout, "Task updated: %s\n", updated.UUID)
				return nil
			}

			outcome, runErr := backend.RunTask(cmd.Context(), strings.TrimSpace(updated.UUID), "", false, false)
			printTaskRunOutcome(outcome)
			return runErr
		},
	}

	command.Flags().StringVarP(&name, "name", "n", "", "Task name/slug")
	command.Flags().StringVarP(&description, "description", "d", "", "Task description")
	command.Flags().StringVarP(&branch, "branch", "b", "", "Feature branch name")
	command.Flags().StringVarP(&parentBranch, "parent-branch", "B", "", "Parent branch name")
	command.Flags().StringVarP(&role, "role", "r", "", "Task role")
	command.Flags().StringArrayVarP(&deps, "depends", "D", nil, "Dependency task UUID (repeatable)")
	command.Flags().StringVarP(&prompt, "prompt", "p", "", "Task prompt")
	command.Flags().BoolVar(&run, "run", false, "Run task immediately after update")

	return command
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

func taskRunCommand() *cobra.Command {
	var merge bool
	var reset bool

	command := &cobra.Command{
		Use:   "run {name|uuid}",
		Short: "Run task session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, backend, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			outcome, runErr := backend.RunTask(cmd.Context(), strings.TrimSpace(args[0]), "", merge, reset)
			printTaskRunOutcome(outcome)
			return runErr
		},
	}

	command.Flags().BoolVar(&merge, "merge", false, "Merge task into parent branch after successful run")
	command.Flags().BoolVar(&reset, "reset", false, "Reset task branch before run")

	return command
}

func taskListCommand() *cobra.Command {
	var recursive bool
	var verbose bool

	command := &cobra.Command{
		Use:   "list [name|uuid]",
		Short: "List tasks",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, _, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			lookup := core.TaskLookup{}
			if len(args) == 1 {
				lookup.Identifier = strings.TrimSpace(args[0])
			}

			tasks, err := manager.ListTasks(lookup, recursive)
			if err != nil {
				return err
			}

			sort.Slice(tasks, func(i, j int) bool {
				if tasks[i] == nil || tasks[j] == nil {
					return i < j
				}
				if tasks[i].Name == tasks[j].Name {
					return tasks[i].UUID < tasks[j].UUID
				}
				return tasks[i].Name < tasks[j].Name
			})

			for _, item := range tasks {
				if item == nil {
					continue
				}
				if verbose {
					fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\t%s\n", item.UUID, item.Name, item.FeatureBranch, item.Status, item.Description)
				} else {
					fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\n", item.UUID, item.Name, item.FeatureBranch, item.Status)
				}
			}
			return nil
		},
	}

	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "List recursively")
	command.Flags().BoolVar(&verbose, "verbose", false, "Include task description")

	return command
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

func loadTaskBackend(cmd *cobra.Command) (*core.TaskManager, *core.TaskBackendAdapter, error) {
	workDir, err := system.ResolveWorkDir("")
	if err != nil {
		return nil, nil, err
	}

	store, err := GetCompositeConfigStore()
	if err != nil {
		return nil, nil, err
	}

	vcsRepo, _, err := system.PrepareSessionVFS(workDir, workDir, "", false, nil, "", "", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("loadTaskBackend() [task.go]: failed to prepare vcs: %w", err)
	}

	runner, err := core.NewCLITaskSessionRunner(workDir, modelName, configPath, projectConfig, "")
	if err != nil {
		return nil, nil, err
	}

	manager, err := core.NewTaskManager(workDir, store, runner)
	if err != nil {
		return nil, nil, err
	}

	backend, err := core.NewTaskBackendAdapter(manager, vcsRepo, nil)
	if err != nil {
		return nil, nil, err
	}

	return manager, backend, nil
}

func printTaskRunOutcome(outcome tool.TaskRunOutcome) {
	fmt.Fprintf(os.Stdout, "Task run session: %s\n", strings.TrimSpace(outcome.SessionID))
	fmt.Fprintf(os.Stdout, "Task branch: %s\n", strings.TrimSpace(outcome.TaskBranchName))
	if strings.TrimSpace(outcome.SummaryText) != "" {
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintln(os.Stdout, strings.TrimSpace(outcome.SummaryText))
	}
}

func printTaskHuman(taskData *core.Task, summaryMeta *core.TaskSessionSummary, summaryText string) {
	if taskData == nil {
		return
	}
	fmt.Fprintf(os.Stdout, "UUID: %s\n", taskData.UUID)
	fmt.Fprintf(os.Stdout, "Name: %s\n", taskData.Name)
	fmt.Fprintf(os.Stdout, "Description: %s\n", taskData.Description)
	fmt.Fprintf(os.Stdout, "Status: %s\n", taskData.Status)
	fmt.Fprintf(os.Stdout, "State: %s\n", taskData.State)
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
