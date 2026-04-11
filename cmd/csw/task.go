package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var resolveTaskRunDefaultsFunc = system.ResolveRunDefaults

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
	var cliModel string
	var cliRole string
	var cliWorkDir string
	var cliWorktree string
	var cliShadowDir string
	var cliAllowAllPerms bool
	var cliInteractive bool
	var cliConfigPath string
	var cliProjectConfig string
	var cliSaveSessionTo string
	var cliSaveSession bool
	var cliLogLLMRequests bool
	var cliLogLLMRequestsRaw bool
	var cliNoRefresh bool
	var cliLSPServer string
	var cliThinking string
	var cliGitUser string
	var cliGitEmail string
	var cliContainerImage string
	var cliContainerOn bool
	var cliContainerOff bool
	var cliContainerMount []string
	var cliContainerEnv []string
	var cliForceCompact bool
	var cliBashRunTimeout string
	var cliMaxThreads int
	var cliOutputFormat string
	var cliVFSAllow []string
	var cliMCPEnable []string
	var cliMCPDisable []string
	var cliHooks []string
	var cliContext []string

	command := &cobra.Command{
		Use:   "run {name|uuid}",
		Short: "Run task session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := applyRunDefaults(resolveTaskRunDefaultsFunc, cmd, cliWorkDir, cliShadowDir, cliProjectConfig, cliConfigPath, &cliModel, &cliWorktree, &merge, &cliLogLLMRequests, &cliThinking, &cliLSPServer, &cliGitUser, &cliGitEmail, &cliMaxThreads, &cliShadowDir, &cliAllowAllPerms, &cliVFSAllow); err != nil {
				return err
			}
			cliLogLLMRequests = cliLogLLMRequests || cliLogLLMRequestsRaw

			containerEnabledChanged := cmd.Flags().Changed("container-enabled")
			containerDisabledChanged := cmd.Flags().Changed("container-disabled")
			if containerEnabledChanged && containerDisabledChanged {
				return fmt.Errorf("taskRunCommand.RunE() [task.go]: --container-enabled and --container-disabled cannot be used together")
			}

			bashRunTimeout, err := parseBashRunTimeout(cliBashRunTimeout)
			if err != nil {
				return err
			}

			_, backend, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			outcome, runErr := backend.RunTaskWithParams(cmd.Context(), strings.TrimSpace(args[0]), "", core.TaskRunParams{
				Merge: merge,
				Reset: reset,
				RunOptions: core.TaskSessionRunOptions{
					Model:             cliModel,
					Role:              cliRole,
					WorkDir:           cliWorkDir,
					ShadowDir:         cliShadowDir,
					ContainerImage:    cliContainerImage,
					ContainerEnabled:  containerEnabledChanged && cliContainerOn,
					ContainerDisabled: containerDisabledChanged && cliContainerOff,
					ContainerMounts:   append([]string(nil), cliContainerMount...),
					ContainerEnv:      append([]string(nil), cliContainerEnv...),
					AllowAllPerms:     cliAllowAllPerms,
					Interactive:       cliInteractive,
					ConfigPath:        cliConfigPath,
					ProjectConfig:     cliProjectConfig,
					SaveSessionTo:     cliSaveSessionTo,
					SaveSession:       cliSaveSession,
					LogLLMRequests:    cliLogLLMRequests,
					LogLLMRequestsRaw: cliLogLLMRequestsRaw,
					NoRefresh:         cliNoRefresh,
					LSPServer:         cliLSPServer,
					Thinking:          cliThinking,
					ForceCompact:      cliForceCompact,
					BashRunTimeout:    bashRunTimeout.String(),
					MaxThreads:        cliMaxThreads,
					OutputFormat:      cliOutputFormat,
					VFSAllow:          parseVFSAllowPaths(cliVFSAllow),
					MCPEnable:         parseMCPServerFlagValues(cliMCPEnable),
					MCPDisable:        parseMCPServerFlagValues(cliMCPDisable),
					HookOverrides:     append([]string(nil), cliHooks...),
					ContextEntries:    append([]string(nil), cliContext...),
					GitUserName:       strings.TrimSpace(cliGitUser),
					GitUserEmail:      strings.TrimSpace(cliGitEmail),
				},
			})
			printTaskRunOutcome(outcome)
			return runErr
		},
	}

	command.Flags().BoolVar(&merge, "merge", false, "Merge task into parent branch after successful run")
	command.Flags().BoolVar(&reset, "reset", false, "Reset task branch before run")
	command.Flags().StringVar(&cliModel, "model", "", "Model alias or model spec in provider/model format (single or comma-separated fallback list); if not set, uses defaults")
	command.Flags().StringVar(&cliRole, "role", "developer", "Agent role name")
	command.Flags().StringVar(&cliWorkDir, "workdir", "", "Working directory (default: current directory)")
	command.Flags().StringVar(&cliShadowDir, "shadow-dir", "", "Shadow directory for agent files overlay (AGENTS.md, .agents*, .csw*, .cswdata)")
	command.Flags().StringVar(&cliWorktree, "worktree", "", "Create and use a git worktree for this session on a feature branch")
	command.Flags().StringVar(&cliContainerImage, "container-image", "", "Container image for running bash commands in container mode")
	command.Flags().BoolVar(&cliContainerOn, "container-enabled", false, "Enable running bash commands in container mode")
	command.Flags().BoolVar(&cliContainerOff, "container-disabled", false, "Disable running bash commands in container mode")
	command.Flags().StringArrayVar(&cliContainerMount, "container-mount", nil, "Additional container mount in host_path:container_path format (repeatable)")
	command.Flags().StringArrayVar(&cliContainerEnv, "container-env", nil, "Additional container env var in KEY=VALUE format (repeatable)")
	command.Flags().BoolVar(&cliAllowAllPerms, "allow-all-permissions", false, "Allow all permissions without asking")
	command.Flags().BoolVar(&cliInteractive, "interactive", false, "Enable interactive mode (allows user to respond to agent questions)")
	command.Flags().StringVar(&cliConfigPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	command.Flags().StringVar(&cliProjectConfig, "project-config", "", "Custom project config directory (default: .csw/config)")
	command.Flags().StringVar(&cliSaveSessionTo, "save-session-to", "", "Save session conversation to specified markdown file")
	command.Flags().BoolVar(&cliSaveSession, "save-session", false, "Save session conversation")
	command.Flags().BoolVar(&cliLogLLMRequests, "log-llm-requests", false, "Log LLM requests and responses")
	command.Flags().BoolVar(&cliLogLLMRequestsRaw, "log-llm-requests-raw", false, "Log raw line-based LLM requests and responses")
	command.Flags().BoolVar(&cliNoRefresh, "no-refresh", false, "Disable OAuth access-token refresh for this run")
	command.Flags().StringVar(&cliLSPServer, "lsp-server", "", "Path to LSP server binary (empty to disable LSP)")
	command.Flags().StringVar(&cliThinking, "thinking", "", "Thinking/reasoning mode: low, medium, high, xhigh (effort-based) or true/false (boolean)")
	command.Flags().StringVar(&cliThinking, "thinking-mode", "", "Thinking/reasoning mode override when starting or resuming a session")
	command.Flags().BoolVar(&cliForceCompact, "force-compact", false, "Force context compaction after loading a resumed session")
	command.Flags().StringVar(&cliGitUser, "git-user", "", "Git user name for git operations (default: from git config)")
	command.Flags().StringVar(&cliGitEmail, "git-email", "", "Git user email for git operations (default: from git config)")
	command.Flags().StringVar(&cliBashRunTimeout, "bash-run-timeout", "120", "Default runBash command timeout (duration; plain number means seconds)")
	command.Flags().IntVar(&cliMaxThreads, "max-threads", 0, "Maximum number of tool calls executed in parallel")
	command.Flags().StringVar(&cliOutputFormat, "output-format", "short", "Console output format: short, full, jsonl")
	command.Flags().StringArrayVar(&cliVFSAllow, "vfs-allow", nil, "Additional path to allow VFS access outside of worktree (repeatable, or use ':' separated list)")
	command.Flags().StringArrayVar(&cliMCPEnable, "mcp-enable", nil, "Enable MCP server by name (repeatable, accepts comma-separated list)")
	command.Flags().StringArrayVar(&cliMCPDisable, "mcp-disable", nil, "Disable MCP server by name (repeatable, accepts comma-separated list)")
	command.Flags().StringArrayVar(&cliHooks, "hook", nil, "Ephemeral hook override: --hook name | --hook name:disable | --hook name:key=value,key2=value2")
	command.Flags().StringArrayVarP(&cliContext, "context", "c", nil, "Template context value in KEY=VAL format (repeatable)")

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

	resolvedTaskDir, err := resolveTaskDirPath(cmd, workDir)
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

	flagTaskDir, err := cmd.Flags().GetString("task-dir")
	if err != nil {
		flag := cmd.PersistentFlags().Lookup("task-dir")
		if flag == nil {
			return "", fmt.Errorf("resolveTaskDirPath() [task.go]: failed to read --task-dir flag: %w", err)
		}
		flagTaskDir = flag.Value.String()
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
