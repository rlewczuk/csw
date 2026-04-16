package main

import (
	"context"
	"fmt"
	stdio "io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/core"
	sessionio "github.com/rlewczuk/csw/pkg/io"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/vcs"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// RunParams holds all parameters for runCommand.
type RunParams struct {
	Command               *cobra.Command
	PositionalArgs        []string
	ContextEntries        []string
	TaskIdentifier        string
	TaskNext              bool
	TaskLast              bool
	TaskReset             bool
	NoMerge               bool
	BashRunTimeoutValue   string
	Prompt                string
	CommandName           string
	CommandArgs           []string
	CommandTemplate       string
	CommandTaskMetadata   *commands.TaskMetadata
	ContextData           map[string]string
	ModelName             string
	RoleName              string
	Task                  *core.Task
	InitialTask           *core.Task
	WorkDir               string
	ShadowDir             string
	WorktreeBranch        string
	GitUserName           string
	GitUserEmail          string
	Merge                 bool
	ContainerEnabled      bool
	ContainerDisabled     bool
	ContainerImage        string
	ContainerMounts       []string
	ContainerEnv          []string
	CommitMessageTemplate string
	ConfigPath            string
	ProjectConfig         string
	AllowAllPerms         bool
	Interactive           bool
	SaveSessionTo         string
	SaveSession           bool
	LogLLMRequests        bool
	LogLLMRequestsRaw     bool
	NoRefresh             bool
	LSPServer             string
	Thinking              string
	ModelOverridden       bool
	BashRunTimeout        time.Duration
	MaxThreads            int
	AllowAllPermissions   bool
	OutputFormat          string
	VFSAllow              []string
	MCPEnable             []string
	MCPDisable            []string
	Stdin                 stdio.Reader
	Stdout                stdio.Writer
	Stderr                stdio.Writer
}

// RunCommandResult stores run command execution result values.
type RunCommandResult struct {
	SessionID   string
	SummaryText string
}

const defaultBashRunTimeout = 120 * time.Second

var runCommandFunc = runCommand
var resolveRunDefaultsFunc = system.ResolveRunDefaults
var resolveWorktreeBranchNameFunc = system.ResolveWorktreeBranchName
var buildSystemFunc = system.BuildSystem
var loadTaskBackendFunc = loadTaskBackend

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
		cliMCPEnable         []string
		cliMCPDisable        []string
		cliContext           []string
		cliTaskIdentifier    string
		cliTaskNext          bool
		cliTaskLast          bool
		cliTaskReset         bool
	)

	cmd := &cobra.Command{
		Use:   "run [--task <name|uuid>|--last|--next] [--model <model>] [--role <role>] [--workdir <dir>] [--shadow-dir <path>] [--worktree <feature-branch-name>] [--merge|--no-merge] [--container-image <image>] [--container-enabled|--container-disabled] [--container-mount <host_path>:<container_path>] [--container-env <key>=<value>] [--commit-message <template>] [--allow-all-permissions] [--interactive] [--save-session-to <file>] [--save-session] [\"prompt\"] [command-args...]",
		Short: "Start a run chat session with an agent",
		Long:  "Start a standard terminal session (no TUI) with a given model and role. The session can be non-interactive or lightly interactive.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Suppress usage for runtime errors from command execution.
			// Argument/flag parsing errors happen before RunE and still show usage.
			cmd.SilenceUsage = true

			return runCommandFunc(&RunParams{
				Command:               cmd,
				PositionalArgs:        append([]string(nil), args...),
				ContextEntries:        append([]string(nil), cliContext...),
				TaskIdentifier:        cliTaskIdentifier,
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
				MCPEnable:             append([]string(nil), cliMCPEnable...),
				MCPDisable:            append([]string(nil), cliMCPDisable...),
			})
		},
	}

	// Define flags
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
	cmd.Flags().StringArrayVar(&cliMCPEnable, "mcp-enable", nil, "Enable MCP server by name (repeatable, accepts comma-separated list)")
	cmd.Flags().StringArrayVar(&cliMCPDisable, "mcp-disable", nil, "Disable MCP server by name (repeatable, accepts comma-separated list)")
	cmd.Flags().StringArrayVarP(&cliContext, "context", "c", nil, "Template context value in KEY=VAL format (repeatable)")
	cmd.Flags().StringVar(&cliTaskIdentifier, "task", "", "Run in task context for specified task name or UUID")
	cmd.Flags().BoolVar(&cliTaskLast, "last", false, "Run latest unfinished task in task context")
	cmd.Flags().BoolVar(&cliTaskNext, "next", false, "Run oldest unfinished task in task context")
	cmd.Flags().BoolVar(&cliTaskReset, "reset", false, "Reset task branch before run in task context")
	return cmd
}

func runCommand(params *RunParams) error {
	if params == nil {
		return fmt.Errorf("runCommand() [run.go]: params cannot be nil")
	}

	startTime := time.Now()
	ctx := context.Background()
	var stdin stdio.Reader = os.Stdin
	var stdout stdio.Writer = os.Stdout
	var stderr stdio.Writer = os.Stderr
	if params.Stdin != nil {
		stdin = params.Stdin
	}
	if params.Stdout != nil {
		stdout = params.Stdout
	}
	if params.Stderr != nil {
		stderr = params.Stderr
	}

	var taskManager *core.TaskManager
	taskIdentifier := ""
	runInTaskMode := false

	if params.Command != nil {
		positionalArgs := append([]string(nil), params.PositionalArgs...)
		prompt := ""
		extraPositionalArgs := []string(nil)
		if len(positionalArgs) >= 1 {
			prompt = positionalArgs[0]
			extraPositionalArgs = positionalArgs[1:]
		}

		if prompt != "" && strings.HasPrefix(prompt, "@") {
			promptFile := strings.TrimPrefix(prompt, "@")
			data, err := os.ReadFile(promptFile)
			if err != nil {
				return fmt.Errorf("runCommand() [run.go]: failed to read prompt file: %w", err)
			}
			prompt = string(data)
		} else if prompt == "-" {
			data, err := stdio.ReadAll(stdin)
			if err != nil {
				return fmt.Errorf("runCommand() [run.go]: failed to read prompt from stdin: %w", err)
			}
			prompt = string(data)
		}

		if prompt != "" {
			prompt = strings.TrimSpace(prompt)
		}

		runInTaskMode = strings.TrimSpace(params.TaskIdentifier) != "" || params.TaskLast || params.TaskNext

		invocation, isCommandInvocation, err := commands.ParseInvocation(prompt, extraPositionalArgs)
		if err != nil {
			return fmt.Errorf("runCommand() [run.go]: %w", err)
		}
		if !isCommandInvocation && len(extraPositionalArgs) > 0 {
			return fmt.Errorf("runCommand() [run.go]: prompt must be a single argument unless using /command invocation")
		}

		commandTemplate := ""
		commandName := ""
		commandArgs := []string(nil)
		commandModelOverride := ""
		commandRoleOverride := ""
		var commandRunDefaults *commands.RunDefaultsMetadata
		var commandTaskMetadata *commands.TaskMetadata
		commandNeedsShell := false
		if invocation != nil && !runInTaskMode {
			commandsRoot, rootErr := resolveCommandsRootDir(params.WorkDir, params.ShadowDir)
			if rootErr != nil {
				return rootErr
			}
			loadedCommand, loadErr := commands.LoadFromDir(filepath.Join(commandsRoot, ".agents", "commands"), invocation.Name)
			if loadErr != nil {
				return fmt.Errorf("runCommand() [run.go]: %w", loadErr)
			}

			commandTemplate = loadedCommand.Template
			commandName = loadedCommand.Name
			commandArgs = invocation.Arguments

			commandModelOverride = strings.TrimSpace(loadedCommand.Metadata.Model)
			commandRoleOverride = strings.TrimSpace(loadedCommand.Metadata.Agent)
			if loadedCommand.Metadata.CSW != nil {
				commandRunDefaults = loadedCommand.Metadata.CSW.Defaults
				commandTaskMetadata = loadedCommand.Metadata.CSW.Task
			}
			commandNeedsShell = commands.HasDefaultRuntimeShellExpansion(loadedCommand.Template)

			prompt = loadedCommand.Template
		}

		contextData, err := system.ParseRunContextEntries(params.ContextEntries)
		if err != nil {
			return err
		}

		if prompt == "" && !runInTaskMode {
			return fmt.Errorf("runCommand() [run.go]: prompt cannot be empty")
		}

		bashRunTimeout, err := parseBashRunTimeout(params.BashRunTimeoutValue)
		if err != nil {
			return err
		}

		if err := applyRunDefaults(resolveRunDefaultsFunc, params.Command, params.WorkDir, params.ShadowDir, params.ProjectConfig, params.ConfigPath, &params.ModelName, &params.WorktreeBranch, &params.Merge, &params.LogLLMRequests, &params.LogLLMRequestsRaw, &params.Thinking, &params.LSPServer, &params.GitUserName, &params.GitUserEmail, &params.MaxThreads, &params.ShadowDir, &params.AllowAllPerms, &params.VFSAllow); err != nil {
			return err
		}

		containerOn := params.ContainerEnabled
		containerOff := params.ContainerDisabled
		commandContainerEnabled, err := applyCommandRunDefaults(params.Command, commandRunDefaults, &params.ModelName, &params.RoleName, &params.WorktreeBranch, &params.Merge, &params.LogLLMRequests, &params.Thinking, &params.LSPServer, &params.GitUserName, &params.GitUserEmail, &params.MaxThreads, &params.ShadowDir, &params.AllowAllPerms, &params.VFSAllow, &containerOn, &containerOff, &params.ContainerImage, &params.ContainerMounts, &params.ContainerEnv)
		if err != nil {
			return err
		}
		if params.NoMerge {
			params.Merge = false
		}
		params.LogLLMRequests = params.LogLLMRequests || params.LogLLMRequestsRaw
		params.ModelOverridden = params.Command.Flags().Changed("model")

		if invocation != nil {
			if !params.Command.Flags().Changed("model") && commandModelOverride != "" {
				params.ModelName = commandModelOverride
			}
			if !params.Command.Flags().Changed("role") && commandRoleOverride != "" {
				params.RoleName = commandRoleOverride
			}
		}

		containerEnabledChanged := params.Command.Flags().Changed("container-enabled")
		containerDisabledChanged := params.Command.Flags().Changed("container-disabled")
		if containerEnabledChanged && containerDisabledChanged {
			return fmt.Errorf("runCommand() [run.go]: --container-enabled and --container-disabled cannot be used together")
		}

		if runInTaskMode {
			if invocation != nil {
				prompt = "/" + strings.TrimSpace(invocation.Name)
				extraPositionalArgs = append([]string(nil), invocation.Arguments...)
			}

			manager, _, err := loadTaskBackendFunc(params.Command)
			if err != nil {
				return err
			}
			taskManager = manager

			identifier, err := resolveTaskRunIdentifier(manager, params.TaskIdentifier, params.TaskLast, params.TaskNext)
			if err != nil {
				return err
			}
			taskIdentifier = identifier

			taskDir, resolvedTask, err := manager.ResolveTask(core.TaskLookup{Identifier: identifier})
			if err != nil {
				return err
			}
			resolvedTask.TaskDir = taskDir
			params.Task = cloneRunTask(resolvedTask)
			params.InitialTask = cloneRunTask(resolvedTask)

			if prompt == "" {
				taskPromptBytes, readErr := os.ReadFile(filepath.Join(taskDir, "task.md"))
				if readErr != nil {
					return fmt.Errorf("runCommand() [run.go]: failed to read task prompt: %w", readErr)
				}
				prompt = strings.TrimSpace(string(taskPromptBytes))
			}

			if prompt == "" {
				return fmt.Errorf("runCommand() [run.go]: prompt cannot be empty")
			}

			runningStatus := core.TaskStatusRunning
			if _, updateErr := manager.UpdateTask(core.TaskUpdateParams{Identifier: identifier, Status: &runningStatus}); updateErr != nil {
				return updateErr
			}

			taskRunMerge := resolveTaskRunMerge(params.Command.Flags().Changed("merge") || params.NoMerge, params.Merge, params.WorktreeBranch, resolveRunDefaultsFunc, params.WorkDir, params.ShadowDir, params.ProjectConfig, params.ConfigPath)
			if strings.TrimSpace(params.WorktreeBranch) == "" {
				params.WorktreeBranch = strings.TrimSpace(params.Task.FeatureBranch)
			}
			params.Merge = taskRunMerge
			_ = params.TaskReset
		}

		containerRequested := (containerEnabledChanged && containerOn) || len(params.ContainerMounts) > 0 || len(params.ContainerEnv) > 0
		if !containerEnabledChanged && !containerDisabledChanged && commandContainerEnabled != nil {
			containerRequested = *commandContainerEnabled
		}
		if !containerDisabledChanged && invocation != nil && commandNeedsShell {
			containerRequested = true
		}

		params.Prompt = prompt
		params.CommandName = commandName
		params.CommandArgs = commandArgs
		params.CommandTemplate = commandTemplate
		params.CommandTaskMetadata = commandTaskMetadata
		params.ContextData = contextData
		params.BashRunTimeout = bashRunTimeout
		params.GitUserName = vcs.ResolveGitIdentity(params.GitUserName, "user.name")
		params.GitUserEmail = vcs.ResolveGitIdentity(params.GitUserEmail, "user.email")
		params.ContainerEnabled = containerRequested
		params.ContainerDisabled = containerDisabledChanged && containerOff
		params.VFSAllow = parseVFSAllowPaths(params.VFSAllow)
		params.MCPEnable = parseMCPServerFlagValues(params.MCPEnable)
		params.MCPDisable = parseMCPServerFlagValues(params.MCPDisable)
	}

	if params.BashRunTimeout <= 0 {
		params.BashRunTimeout = defaultBashRunTimeout
	}
	if strings.TrimSpace(params.OutputFormat) == "" {
		params.OutputFormat = "short"
	}
	if params.OutputFormat != "short" && params.OutputFormat != "full" && params.OutputFormat != "jsonl" {
		return fmt.Errorf("runCommand() [run.go]: invalid --output-format %q (allowed: short, full, jsonl)", params.OutputFormat)
	}

	if err := validateMergeRunParams(params); err != nil {
		return err
	}

	resolvedWorktreeBranch, err := resolveWorktreeBranchNameFunc(ctx, system.ResolveWorktreeBranchNameParams{
		Prompt:         params.Prompt,
		ModelName:      params.ModelName,
		WorkDir:        params.WorkDir,
		ShadowDir:      params.ShadowDir,
		ProjectConfig:  params.ProjectConfig,
		ConfigPath:     params.ConfigPath,
		WorktreeBranch: params.WorktreeBranch,
	})
	if err != nil {
		return fmt.Errorf("runCommand() [run.go]: failed to resolve worktree branch: %w", err)
	}
	params.WorktreeBranch = resolvedWorktreeBranch
	if params.WorktreeBranch != "" {
		_, _ = fmt.Fprintf(stdout, "[INFO] Worktree branch: %s\n", params.WorktreeBranch)
	}

	sweSystem, buildResult, err := buildSystemFunc(system.BuildSystemParams{
		WorkDir:             params.WorkDir,
		ShadowDir:           params.ShadowDir,
		ConfigPath:          params.ConfigPath,
		ProjectConfig:       params.ProjectConfig,
		ModelName:           params.ModelName,
		RoleName:            params.RoleName,
		WorktreeBranch:      params.WorktreeBranch,
		GitUserName:         params.GitUserName,
		GitUserEmail:        params.GitUserEmail,
		ContainerEnabled:    params.ContainerEnabled,
		ContainerDisabled:   params.ContainerDisabled,
		ContainerImage:      params.ContainerImage,
		ContainerMounts:     params.ContainerMounts,
		ContainerEnv:        params.ContainerEnv,
		LSPServer:           params.LSPServer,
		LogLLMRequests:      params.LogLLMRequests,
		LogLLMRequestsRaw:   params.LogLLMRequestsRaw,
		NoRefresh:           params.NoRefresh,
		Thinking:            params.Thinking,
		BashRunTimeout:      params.BashRunTimeout,
		AllowedPaths:        params.VFSAllow,
		MaxToolThreads:      params.MaxThreads,
		AllowAllPermissions: params.AllowAllPerms,
		MCPEnable:           params.MCPEnable,
		MCPDisable:          params.MCPDisable,
	})
	if err != nil {
		return err
	}
	defer buildResult.Cleanup()
	defer logging.FlushLogs()

	params.WorkDir = buildResult.WorkDir
	params.ShadowDir = buildResult.ShadowDir
	params.ModelName = buildResult.ModelName
	if err := renderCommandPrompt(params, buildResult.WorkDir, buildResult.ShellRunner, buildResult.HostShellRunner); err != nil {
		return err
	}
	if err := PreparePromptWithContext(params); err != nil {
		return err
	}

	if params.LSPServer != "" {
		lspStatus := "disabled"
		if buildResult.LSPStarted {
			lspStatus = "started"
		}
		_, _ = fmt.Fprintf(stdout, "[INFO] LSP %s (workdir: %s)\n", lspStatus, buildResult.LSPWorkDir)
	}
	if strings.TrimSpace(buildResult.ContainerImage) != "" {
		_, _ = fmt.Fprintln(stdout, BuildContainerStartupInfoMessage(buildResult))
	}

	sessionOutput := buildRunSessionOutput(params, stdout)
	runtimeResult, err := sweSystem.StartRunSession(system.StartRunSessionParams{
		ModelName:       params.ModelName,
		RoleName:        params.RoleName,
		Task:            params.Task,
		Thinking:        params.Thinking,
		ModelOverridden: params.ModelOverridden,
		Prompt:          params.Prompt,
		OutputHandler:   sessionOutput,
	})
	if err != nil {
		return fmt.Errorf("runCommand() [run.go]: failed to start run session runtime: %w", err)
	}
	session := runtimeResult.Session
	if sessionInput := buildRunStdinSessionInput(params, runtimeResult.Thread, stdin); sessionInput != nil {
		sessionInput.StartReadingInput()
	}

	finalizeVCS := buildResult.VCS
	finalizeWorktreeBranch := buildResult.WorktreeBranch
	finalizeWorktreeDir := buildResult.WorkDir

	sessionID := session.ID()
	defer func() {
		_, _ = fmt.Fprintf(stdout, "Session ID: %s\n", sessionID)
	}()

	baseCommitID := vcs.ResolveGitCommitID(vcs.ChooseGitDiffDir(buildResult.WorkDirRoot, buildResult.WorkDir), "HEAD")

	var sessionRunErr error

	// Wait for completion or context cancellation
	select {
	case err := <-runtimeResult.Done:
		if err != nil {
			sessionRunErr = fmt.Errorf("runCommand() [run.go]: session error: %w", err)
		}
	case <-ctx.Done():
		sessionRunErr = ctx.Err()
	}

	var finalizeResult system.WorktreeFinalizeResult
	finalizeResult, finalizeErr := system.FinalizeWorktreeSession(ctx, finalizeVCS, finalizeWorktreeBranch, params.Merge, params.CommitMessageTemplate, sweSystem, session, stderr, buildResult.WorkDirRoot, finalizeWorktreeDir, params.Prompt)
	if finalizeErr != nil {
		sessionRunErr = finalizeErr
	}
	endTime := time.Now()

	if err := core.EmitSessionSummary(startTime, endTime, session, core.SessionSummaryBuildResult{
		LogsDir:        buildResult.LogsDir,
		WorkDirRoot:    buildResult.WorkDirRoot,
		WorkDir:        buildResult.WorkDir,
		LSPServer:      buildResult.LSPServer,
		ContainerImage: buildResult.ContainerImage,
	}, buildSummaryMessageFunc(sessionOutput), sessionRunErr, baseCommitID, finalizeResult.HeadCommitID); err != nil {
		return err
	}

	if err := applyCommandTaskMetadata(params); err != nil {
		return err
	}

	if runInTaskMode && taskManager != nil && strings.TrimSpace(taskIdentifier) != "" {
		finalStatus := core.TaskStatusCompleted
		if params.Merge {
			finalStatus = core.TaskStatusMerged
		}
		if _, err := taskManager.UpdateTask(core.TaskUpdateParams{Identifier: taskIdentifier, Status: &finalStatus}); err != nil {
			return err
		}
	}

	return nil
}

func applyCommandTaskMetadata(params *RunParams) error {
	if params == nil || params.Task == nil || params.CommandTaskMetadata == nil {
		return nil
	}
	taskDir := strings.TrimSpace(params.Task.TaskDir)
	if taskDir == "" {
		return nil
	}

	taskFilePath := filepath.Join(taskDir, "task.yml")
	taskBytes, err := os.ReadFile(taskFilePath)
	if err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run.go]: failed to read task metadata: %w", err)
	}

	var persistedTask core.Task
	if err := yaml.Unmarshal(taskBytes, &persistedTask); err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run.go]: failed to parse task metadata: %w", err)
	}

	applyIfUnchanged := func(fieldName string, apply func()) {
		if params.InitialTask == nil {
			apply()
			return
		}
		currentField := reflect.ValueOf(persistedTask).FieldByName(fieldName)
		initialField := reflect.ValueOf(*params.InitialTask).FieldByName(fieldName)
		if !currentField.IsValid() || !initialField.IsValid() {
			return
		}
		if reflect.DeepEqual(currentField.Interface(), initialField.Interface()) {
			apply()
		}
	}

	metadata := params.CommandTaskMetadata
	if metadata.UUID != nil {
		applyIfUnchanged("UUID", func() { persistedTask.UUID = strings.TrimSpace(*metadata.UUID) })
	}
	if metadata.Name != nil {
		applyIfUnchanged("Name", func() { persistedTask.Name = strings.TrimSpace(*metadata.Name) })
	}
	if metadata.Description != nil {
		applyIfUnchanged("Description", func() { persistedTask.Description = strings.TrimSpace(*metadata.Description) })
	}
	if metadata.Status != nil {
		applyIfUnchanged("Status", func() { persistedTask.Status = strings.TrimSpace(*metadata.Status) })
	}
	if metadata.FeatureBranch != nil {
		applyIfUnchanged("FeatureBranch", func() { persistedTask.FeatureBranch = strings.TrimSpace(*metadata.FeatureBranch) })
	}
	if metadata.ParentBranch != nil {
		applyIfUnchanged("ParentBranch", func() { persistedTask.ParentBranch = strings.TrimSpace(*metadata.ParentBranch) })
	}
	if metadata.Role != nil {
		applyIfUnchanged("Role", func() { persistedTask.Role = strings.TrimSpace(*metadata.Role) })
	}
	if metadata.Deps != nil {
		applyIfUnchanged("Deps", func() { persistedTask.Deps = append([]string(nil), (*metadata.Deps)...) })
	}
	if metadata.SessionIDs != nil {
		applyIfUnchanged("SessionIDs", func() { persistedTask.SessionIDs = append([]string(nil), (*metadata.SessionIDs)...) })
	}
	if metadata.SubtaskIDs != nil {
		applyIfUnchanged("SubtaskIDs", func() { persistedTask.SubtaskIDs = append([]string(nil), (*metadata.SubtaskIDs)...) })
	}
	if metadata.ParentTaskID != nil {
		applyIfUnchanged("ParentTaskID", func() { persistedTask.ParentTaskID = strings.TrimSpace(*metadata.ParentTaskID) })
	}
	if metadata.CreatedAt != nil {
		applyIfUnchanged("CreatedAt", func() { persistedTask.CreatedAt = strings.TrimSpace(*metadata.CreatedAt) })
	}
	if metadata.UpdatedAt != nil {
		applyIfUnchanged("UpdatedAt", func() { persistedTask.UpdatedAt = strings.TrimSpace(*metadata.UpdatedAt) })
	}

	updatedBytes, err := yaml.Marshal(&persistedTask)
	if err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run.go]: failed to serialize task metadata: %w", err)
	}
	if err := os.WriteFile(taskFilePath, updatedBytes, 0644); err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run.go]: failed to persist task metadata: %w", err)
	}

	return nil
}

func cloneRunTask(task *core.Task) *core.Task {
	if task == nil {
		return nil
	}
	cloned := *task
	cloned.Deps = append([]string(nil), task.Deps...)
	cloned.SessionIDs = append([]string(nil), task.SessionIDs...)
	cloned.SubtaskIDs = append([]string(nil), task.SubtaskIDs...)

	return &cloned
}

func buildRunSessionOutput(params *RunParams, output stdio.Writer) core.SessionThreadOutput {
	if params == nil {
		return sessionio.NewTextSessionOutput(output)
	}

	if strings.TrimSpace(params.OutputFormat) == "jsonl" {
		return sessionio.NewJsonlSessionOutput(output)
	}

	return sessionio.NewTextSessionOutputWithSlug(output, params.WorktreeBranch)
}

func buildSummaryMessageFunc(output core.SessionThreadOutput) func(string, shared.MessageType) {
	if output == nil {
		return nil
	}

	return func(message string, messageType shared.MessageType) {
		output.ShowMessage(message, string(messageType))
	}
}

type runSessionInput interface {
	StartReadingInput()
}

func buildRunStdinSessionInput(params *RunParams, thread core.SessionThreadInput, input stdio.Reader) runSessionInput {
	if params == nil || thread == nil || input == nil {
		return nil
	}

	if params.OutputFormat == "jsonl" {
		return sessionio.NewJsonlSessionInput(input, thread)
	}

	return sessionio.NewTextSessionInput(input, thread)
}

func validateMergeRunParams(params *RunParams) error {
	if params == nil {
		return fmt.Errorf("validateMergeRunParams() [run.go]: params cannot be nil")
	}

	if params.Merge && strings.TrimSpace(params.WorktreeBranch) == "" {
		return fmt.Errorf("runCommand() [run.go]: --merge requires --worktree")
	}

	return nil
}

func resolveTaskRunMerge(mergeFlagChanged bool, cliMerge bool, cliWorktree string, resolver runDefaultsResolver, workDir string, shadowDir string, projectConfig string, configPath string) bool {
	if mergeFlagChanged {
		return cliMerge
	}

	if strings.TrimSpace(cliWorktree) != "" {
		return cliMerge
	}

	if resolver == nil {
		return cliMerge
	}

	defaults, err := resolver(system.ResolveRunDefaultsParams{
		WorkDir:       workDir,
		ShadowDir:     shadowDir,
		ProjectConfig: projectConfig,
		ConfigPath:    configPath,
	})
	if err != nil {
		return cliMerge
	}

	if defaults.Merge {
		return true
	}

	return cliMerge
}

func resolveCommandsRootDir(workDir string, shadowDir string) (string, error) {
	if strings.TrimSpace(shadowDir) != "" {
		resolvedShadowDir, err := system.ResolveWorkDir(shadowDir)
		if err != nil {
			return "", fmt.Errorf("resolveCommandsRootDir() [run.go]: failed to resolve shadow directory: %w", err)
		}
		return resolvedShadowDir, nil
	}

	resolvedWorkDir, err := system.ResolveWorkDir(workDir)
	if err != nil {
		return "", fmt.Errorf("resolveCommandsRootDir() [run.go]: failed to resolve work directory: %w", err)
	}

	return resolvedWorkDir, nil
}

func renderCommandPrompt(params *RunParams, workDir string, shellRunner runner.CommandRunner, hostShellRunner runner.CommandRunner) error {
	if params == nil {
		return fmt.Errorf("renderCommandPrompt() [run.go]: params is nil")
	}
	if strings.TrimSpace(params.CommandName) == "" {
		return nil
	}

	template := params.CommandTemplate
	if strings.TrimSpace(template) == "" {
		template = params.Prompt
	}

	withArguments := commands.ApplyArguments(template, params.CommandArgs)
	expandedPrompt, err := commands.ExpandPrompt(withArguments, workDir, shellRunner, hostShellRunner)
	if err != nil {
		return fmt.Errorf("renderCommandPrompt() [run.go]: failed to render command /%s: %w", params.CommandName, err)
	}

	params.Prompt = strings.TrimSpace(expandedPrompt)
	if params.Prompt == "" {
		return fmt.Errorf("renderCommandPrompt() [run.go]: rendered command /%s prompt is empty", params.CommandName)
	}

	return nil
}

func PreparePromptWithContext(params *RunParams) error {
	if params == nil {
		return fmt.Errorf("PreparePromptWithContext() [run.go]: params is nil")
	}

	if strings.TrimSpace(params.Prompt) == "" {
		return nil
	}

	renderedPrompt, err := shared.RenderTextWithContext(params.Prompt, params.ContextData)
	if err != nil {
		return err
	}
	params.Prompt = strings.TrimSpace(renderedPrompt)

	return nil
}

func BuildContainerStartupInfoMessage(buildResult system.BuildSystemResult) string {
	identity := buildResult.ContainerIdentity
	return fmt.Sprintf(
		"[INFO] Container: image=%s tag=%s version=%s user=%s(uid=%d) group=%s(gid=%d)",
		shared.NullValue(strings.TrimSpace(buildResult.ContainerImageName)),
		shared.NullValue(strings.TrimSpace(buildResult.ContainerImageTag)),
		shared.NullValue(strings.TrimSpace(buildResult.ContainerImageVersion)),
		shared.NullValue(strings.TrimSpace(identity.UserName)),
		identity.UID,
		shared.NullValue(strings.TrimSpace(identity.GroupName)),
		identity.GID,
	)
}
