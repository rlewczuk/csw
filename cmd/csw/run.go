package main

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/system"
)

// RunCommand creates the run command.
func RunCommand() *cobra.Command {
	var (
		cliModel             string
		cliRole              string
		cliWorkDir           string
		cliWorktree          string
		cliAllowAllPerms     bool
		cliInteractive       bool
		cliConfigPath        string
		cliProjectConfig     string
		cliSaveSessionTo     string
		cliSaveSession       bool
		cliLogLLMRequests    bool
		cliLogLLMRequestsRaw bool
		cliNoRefresh         bool
		cliLSPServer         string
		cliThinking          string
		cliGitUser           string
		cliGitEmail          string
		cliCommitMessage     string
		cliMerge             bool
		cliNoMerge           bool
		cliNoCommit          bool
		cliContainerImage    string
		cliContainerOn       bool
		cliContainerOff      bool
		cliContainerMount    []string
		cliContainerEnv      []string
		cliBashRunTimeout    string
		cliRunBashMax        int
		cliVFSReadLimit      int
		cliMaxThreads        int
		cliOutputFormat      string
		cliVFSAllow          []string
		cliContext           []string
		cliTaskIdentifier    string
		cliTaskNext          bool
		cliTaskLast          bool
		cliTaskReset         bool
	)

	cmd := &cobra.Command{
		Use:   "run [<task-uuid-or-branch>|--last|--next] [--model <model>] [--role <role>] [--workdir <dir>] [--shadow-dir <path>] [--worktree <feature-branch-name>] [--merge|--no-merge] [--no-commit] [--container-image <image>] [--container-enabled|--container-disabled] [--container-mount <host_path>:<container_path>] [--container-env <key>=<value>] [--commit-message <template>] [--allow-all-permissions] [--interactive] [--save-session-to <file>] [--save-session] [\"prompt\"] [command-args...]",
		Short: "Start a run chat session with an agent",
		Long:  "Start a standard terminal session (no TUI) with a given model and role. The session can be non-interactive or lightly interactive.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			runBashMaxOutput := cliRunBashMax
			vfsReadLimit := cliVFSReadLimit
			configPath, err := system.BuildConfigPath(cliProjectConfig, cliConfigPath)
			if err != nil {
				return err
			}
			configRoot, err := system.ResolveWorkDir(cliWorkDir)
			if err != nil {
				return err
			}
			if strings.TrimSpace(shadowDir) != "" {
				configRoot, err = system.ResolveWorkDir(shadowDir)
				if err != nil {
					return err
				}
			}
			configPath, err = system.ResolveConfigPathForProjectRoot(configPath, configRoot)
			if err != nil {
				return err
			}
			cswConfig, err := conf.CswConfigLoad(configPath)
			if err != nil {
				return err
			}
			globalConfig := cswConfig.GlobalConfig
			if globalConfig == nil {
				globalConfig = &conf.GlobalConfig{}
				cswConfig.GlobalConfig = globalConfig
			}
			parameters := &globalConfig.Parameters
			if cmd.Flags().Changed("model") {
				parameters.Model = cliModel
				parameters.ModelOverridden = true
			}
			if cmd.Flags().Changed("role") || parameters.Role == "" {
				parameters.Role = cliRole
				parameters.RoleOverridden = cmd.Flags().Changed("role")
			}
			if cmd.Flags().Changed("workdir") {
				parameters.Workdir = cliWorkDir
			}
			if cmd.Flags().Changed("worktree") {
				parameters.Worktree = cliWorktree
			}
			if cmd.Flags().Changed("merge") {
				parameters.Merge = cliMerge
			}
			if cmd.Flags().Changed("no-commit") {
				parameters.NoCommit = cliNoCommit
			}
			if parameters.Container == nil {
				parameters.Container = &conf.ContainerConfig{}
			}
			if cmd.Flags().Changed("container-enabled") {
				parameters.ContainerEnabled = cliContainerOn
			}
			if cmd.Flags().Changed("container-disabled") {
				parameters.ContainerDisabled = cliContainerOff
			}
			if cmd.Flags().Changed("container-image") {
				parameters.Container.Image = cliContainerImage
			}
			if cmd.Flags().Changed("container-mount") {
				parameters.Container.Mounts = append([]string(nil), cliContainerMount...)
			}
			if cmd.Flags().Changed("container-env") {
				parameters.Container.Env = append([]string(nil), cliContainerEnv...)
			}
			if cmd.Flags().Changed("allow-all-permissions") {
				parameters.AllowAllPermissions = cliAllowAllPerms
			}
			if cmd.Flags().Changed("log-llm-requests") {
				parameters.LogLLMRequests = cliLogLLMRequests
			}
			if cmd.Flags().Changed("log-llm-requests-raw") {
				parameters.LogLLMRequestsRaw = cliLogLLMRequestsRaw
			}
			if cmd.Flags().Changed("lsp-server") {
				parameters.LSPServer = cliLSPServer
			}
			if cmd.Flags().Changed("thinking") {
				parameters.Thinking = cliThinking
			}
			if cmd.Flags().Changed("git-user") {
				parameters.GitUserName = cliGitUser
			}
			if cmd.Flags().Changed("git-email") {
				parameters.GitUserEmail = cliGitEmail
			}
			if cmd.Flags().Changed("max-threads") {
				parameters.MaxThreads = cliMaxThreads
			}
			if cmd.Flags().Changed("vfs-allow") {
				parameters.VFSAllow = append([]string(nil), cliVFSAllow...)
			}
			if cmd.Flags().Changed("run-bash-max") {
				parameters.RunBashMax = &runBashMaxOutput
			}
			if cmd.Flags().Changed("vfs-read-limit") {
				parameters.VfsReadLimit = &vfsReadLimit
			}
			if shouldApplyRunShadowDir(cmd) {
				parameters.ShadowDir = shadowDir
			}
			parameters.NoMerge = cliNoMerge
			parameters.BashRunTimeout = cliBashRunTimeout
			parameters.CommitMessageTemplate = cliCommitMessage
			parameters.ConfigPath = cliConfigPath
			parameters.ProjectConfig = cliProjectConfig
			parameters.Interactive = cliInteractive
			parameters.SaveSessionTo = cliSaveSessionTo
			parameters.SaveSession = cliSaveSession
			parameters.NoRefresh = cliNoRefresh
			parameters.OutputFormat = cliOutputFormat
			parameters.ContextEntries = append([]string(nil), cliContext...)
			parameters.TaskIdentifier = strings.TrimSpace(cliTaskIdentifier)
			parameters.TaskNext = cliTaskNext
			parameters.TaskLast = cliTaskLast
			parameters.TaskReset = cliTaskReset
			parameters.PositionalArgs = append([]string(nil), args...)
			return system.RunCommand(core.NewRunExecution(cswConfig, os.Stdin, os.Stdout, os.Stderr))
		},
	}

	cmd.Flags().StringVar(&cliModel, "model", "", "Model alias or model spec in provider/model format (single or comma-separated fallback list); if not set, uses defaults")
	cmd.Flags().StringVar(&cliRole, "role", "developer", "Agent role name")
	cmd.Flags().StringVar(&cliWorkDir, "workdir", "", "Working directory (default: current directory)")
	cmd.Flags().StringVar(&cliWorktree, "worktree", "", "Create and use a git worktree for this session on a feature branch")
	cmd.Flags().BoolVar(&cliMerge, "merge", false, "Merge the feature worktree branch into main after commit")
	cmd.Flags().BoolVar(&cliNoMerge, "no-merge", false, "Disable merging after commit, overriding configured defaults")
	cmd.Flags().BoolVar(&cliNoCommit, "no-commit", false, "Work directly in the project directory without creating a worktree or committing changes")
	cmd.Flags().StringVar(&cliContainerImage, "container-image", "", "Container image for running bash commands in container mode")
	cmd.Flags().BoolVar(&cliContainerOn, "container-enabled", false, "Enable running bash commands in container mode")
	cmd.Flags().BoolVar(&cliContainerOff, "container-disabled", false, "Disable running bash commands in container mode")
	cmd.Flags().StringArrayVar(&cliContainerMount, "container-mount", nil, "Additional container mount in host_path:container_path format (repeatable)")
	cmd.Flags().StringArrayVar(&cliContainerEnv, "container-env", nil, "Additional container env var in KEY=VALUE format (repeatable)")
	cmd.Flags().StringVar(&cliCommitMessage, "commit-message", "", "Custom commit message template, e.g. '[{{ .Branch }}] {{ .Message }}'")
	cmd.Flags().BoolVar(&cliAllowAllPerms, "allow-all-permissions", false, "Allow all permissions without asking")
	cmd.Flags().BoolVar(&cliInteractive, "interactive", false, "Enable interactive mode (allows user to respond to agent questions)")
	cmd.Flags().StringVar(&cliConfigPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	cmd.Flags().StringVar(&cliProjectConfig, "project-config", "", "Custom project config directory (default: .csw/config)")
	cmd.Flags().StringVar(&cliSaveSessionTo, "save-session-to", "", "Save session conversation to specified markdown file")
	cmd.Flags().BoolVar(&cliSaveSession, "save-session", false, "Save session conversation")
	cmd.Flags().BoolVar(&cliLogLLMRequests, "log-llm-requests", false, "Log LLM requests and responses")
	cmd.Flags().BoolVar(&cliLogLLMRequestsRaw, "log-llm-requests-raw", false, "Log raw line-based LLM requests and responses")
	cmd.Flags().BoolVar(&cliNoRefresh, "no-refresh", false, "Disable OAuth access-token refresh for this run")
	cmd.Flags().StringVar(&cliLSPServer, "lsp-server", "", "Path to LSP server binary (empty to disable LSP)")
	cmd.Flags().StringVar(&cliThinking, "thinking", "", "Thinking/reasoning mode override: low, medium, high, xhigh (effort-based) or true/false (boolean)")
	cmd.Flags().StringVar(&cliGitUser, "git-user", "", "Git user name for git operations (default: from git config)")
	cmd.Flags().StringVar(&cliGitEmail, "git-email", "", "Git user email for git operations (default: from git config)")
	cmd.Flags().StringVar(&cliBashRunTimeout, "bash-run-timeout", "120", "Default runBash command timeout (duration; plain number means seconds)")
	cmd.Flags().IntVar(&cliRunBashMax, "run-bash-max", 2048, "Default runBash output limit in bytes (0 disables limit)")
	cmd.Flags().IntVar(&cliVFSReadLimit, "vfs-read-limit", 384, "Default vfsRead output limit in lines (0 disables limit)")
	cmd.Flags().IntVar(&cliMaxThreads, "max-threads", 0, "Maximum number of tool calls executed in parallel")
	cmd.Flags().StringVar(&cliOutputFormat, "output-format", "short", "Console output format: short, full, jsonl")
	cmd.Flags().StringArrayVar(&cliVFSAllow, "vfs-allow", nil, "Additional path to allow VFS access outside of worktree (repeatable, or use ':' separated list)")
	cmd.Flags().StringArrayVarP(&cliContext, "context", "c", nil, "Template context value in KEY=VAL format (repeatable)")
	cmd.Flags().BoolVar(&cliTaskLast, "last", false, "Run latest unfinished task in task context")
	cmd.Flags().BoolVar(&cliTaskNext, "next", false, "Run oldest unfinished task in task context")
	cmd.Flags().BoolVar(&cliTaskReset, "reset", false, "Reset task branch before run in task context")

	return cmd
}

// shouldApplyRunShadowDir reports whether resolved shadow-dir should override config.
func shouldApplyRunShadowDir(cmd *cobra.Command) bool {
	if cmd == nil {
		return strings.TrimSpace(shadowDir) != ""
	}
	shadowFlag := cmd.Flag("shadow-dir")
	return (shadowFlag != nil && shadowFlag.Changed) || strings.TrimSpace(shadowDir) != ""
}
