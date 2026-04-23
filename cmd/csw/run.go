package main

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/rlewczuk/csw/pkg/system"
)

// RunCommand creates the run command.
func RunCommand() *cobra.Command {
	var (
		cliModel             string
		cliRole              string
		cliWorkDir           string
		cliWorktree          string
		cliShadowDir         string
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
		cliContainerImage    string
		cliContainerOn       bool
		cliContainerOff      bool
		cliContainerMount    []string
		cliContainerEnv      []string
		cliBashRunTimeout    string
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
		Use:   "run [<task-uuid-or-branch>|--last|--next] [--model <model>] [--role <role>] [--workdir <dir>] [--shadow-dir <path>] [--worktree <feature-branch-name>] [--merge|--no-merge] [--container-image <image>] [--container-enabled|--container-disabled] [--container-mount <host_path>:<container_path>] [--container-env <key>=<value>] [--commit-message <template>] [--allow-all-permissions] [--interactive] [--save-session-to <file>] [--save-session] [\"prompt\"] [command-args...]",
		Short: "Start a run chat session with an agent",
		Long:  "Start a standard terminal session (no TUI) with a given model and role. The session can be non-interactive or lightly interactive.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return system.RunCommand(&system.RunParams{
				Command:               cmd,
				PositionalArgs:        append([]string(nil), args...),
				ContextEntries:        append([]string(nil), cliContext...),
				TaskIdentifier:        strings.TrimSpace(cliTaskIdentifier),
				TaskNext:              cliTaskNext,
				TaskLast:              cliTaskLast,
				TaskReset:             cliTaskReset,
				NoMerge:               cliNoMerge,
				BashRunTimeoutValue:   cliBashRunTimeout,
				ModelName:             cliModel,
				RoleName:              cliRole,
				WorkDir:               cliWorkDir,
				ShadowDir:             cliShadowDir,
				WorktreeBranch:        cliWorktree,
				GitUserName:           cliGitUser,
				GitUserEmail:          cliGitEmail,
				Merge:                 cliMerge,
				ContainerEnabled:      cliContainerOn,
				ContainerDisabled:     cliContainerOff,
				ContainerImage:        cliContainerImage,
				ContainerMounts:       append([]string(nil), cliContainerMount...),
				ContainerEnv:          append([]string(nil), cliContainerEnv...),
				CommitMessageTemplate: cliCommitMessage,
				ConfigPath:            cliConfigPath,
				ProjectConfig:         cliProjectConfig,
				AllowAllPerms:         cliAllowAllPerms,
				Interactive:           cliInteractive,
				SaveSessionTo:         cliSaveSessionTo,
				SaveSession:           cliSaveSession,
				LogLLMRequests:        cliLogLLMRequests,
				LogLLMRequestsRaw:     cliLogLLMRequestsRaw,
				NoRefresh:             cliNoRefresh,
				LSPServer:             cliLSPServer,
				Thinking:              cliThinking,
				MaxThreads:            cliMaxThreads,
				OutputFormat:          cliOutputFormat,
				VFSAllow:              append([]string(nil), cliVFSAllow...),
			})
		},
	}

	cmd.Flags().StringVar(&cliModel, "model", "", "Model alias or model spec in provider/model format (single or comma-separated fallback list); if not set, uses defaults")
	cmd.Flags().StringVar(&cliRole, "role", "developer", "Agent role name")
	cmd.Flags().StringVar(&cliWorkDir, "workdir", "", "Working directory (default: current directory)")
	cmd.Flags().StringVar(&cliShadowDir, "shadow-dir", "", "Shadow directory for agent files overlay (AGENTS.md, .agents*, .csw*, .cswdata)")
	cmd.Flags().StringVar(&cliWorktree, "worktree", "", "Create and use a git worktree for this session on a feature branch")
	cmd.Flags().BoolVar(&cliMerge, "merge", false, "Merge the feature worktree branch into main after commit")
	cmd.Flags().BoolVar(&cliNoMerge, "no-merge", false, "Disable merging after commit, overriding configured defaults")
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
	cmd.Flags().IntVar(&cliMaxThreads, "max-threads", 0, "Maximum number of tool calls executed in parallel")
	cmd.Flags().StringVar(&cliOutputFormat, "output-format", "short", "Console output format: short, full, jsonl")
	cmd.Flags().StringArrayVar(&cliVFSAllow, "vfs-allow", nil, "Additional path to allow VFS access outside of worktree (repeatable, or use ':' separated list)")
	cmd.Flags().StringArrayVarP(&cliContext, "context", "c", nil, "Template context value in KEY=VAL format (repeatable)")
	cmd.Flags().BoolVar(&cliTaskLast, "last", false, "Run latest unfinished task in task context")
	cmd.Flags().BoolVar(&cliTaskNext, "next", false, "Run oldest unfinished task in task context")
	cmd.Flags().BoolVar(&cliTaskReset, "reset", false, "Reset task branch before run in task context")

	return cmd
}
