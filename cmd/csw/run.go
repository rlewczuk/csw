package main

import (
	"context"
	"encoding/json"
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
	OutputFormat          string
	VFSAllow              []string
	MCPEnable             []string
	MCPDisable            []string
	HookOverrides         []string
}

const defaultBashRunTimeout = 120 * time.Second

var runFunc = runCommand
var resolveRunDefaultsFunc = system.ResolveRunDefaults
var resolveWorktreeBranchNameFunc = system.ResolveWorktreeBranchName
var buildSystemFunc = system.BuildSystem

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
		cliHooks             []string
		cliContext           []string
		cliTaskJSON          string
		cliTaskDir           string
	)

	cmd := &cobra.Command{
		Use:   "run [--model <model>] [--role <role>] [--workdir <dir>] [--shadow-dir <path>] [--worktree <feature-branch-name>] [--merge] [--container-image <image>] [--container-enabled|--container-disabled] [--container-mount <host_path>:<container_path>] [--container-env <key>=<value>] [--commit-message <template>] [--allow-all-permissions] [--interactive] [--save-session-to <file>] [--save-session] [\"prompt\"] [command-args...]",
		Short: "Start a run chat session with an agent",
		Long:  "Start a standard terminal session (no TUI) with a given model and role. The session can be non-interactive or lightly interactive.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Suppress usage for runtime errors from command execution.
			// Argument/flag parsing errors happen before RunE and still show usage.
			cmd.SilenceUsage = true

			positionalArgs := append([]string(nil), args...)

			prompt := ""
			extraPositionalArgs := []string(nil)
			if len(positionalArgs) >= 1 {
				prompt = positionalArgs[0]
				extraPositionalArgs = positionalArgs[1:]
			}

			// Read prompt from file if it starts with @
			if prompt != "" && strings.HasPrefix(prompt, "@") {
				promptFile := strings.TrimPrefix(prompt, "@")
				data, err := os.ReadFile(promptFile)
				if err != nil {
					return fmt.Errorf("RunCommand.RunE() [run.go]: failed to read prompt file: %w", err)
				}
				prompt = string(data)
			} else if prompt == "-" {
				// Read prompt from stdin
				data, err := stdio.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("RunCommand.RunE() [run.go]: failed to read prompt from stdin: %w", err)
				}
				prompt = string(data)
			}

			if prompt != "" {
				prompt = strings.TrimSpace(prompt)
			}

			invocation, isCommandInvocation, err := commands.ParseInvocation(prompt, extraPositionalArgs)
			if err != nil {
				return fmt.Errorf("RunCommand.RunE() [run.go]: %w", err)
			}
			if !isCommandInvocation && len(extraPositionalArgs) > 0 {
				return fmt.Errorf("RunCommand.RunE() [run.go]: prompt must be a single argument unless using /command invocation")
			}

			commandTemplate := ""
			commandName := ""
			commandArgs := []string(nil)
			commandModelOverride := ""
			commandRoleOverride := ""
			var commandRunDefaults *commands.RunDefaultsMetadata
			var commandTaskMetadata *commands.TaskMetadata
			commandNeedsShell := false
			if invocation != nil {
				commandsRoot, rootErr := resolveCommandsRootDir(cliWorkDir, cliShadowDir)
				if rootErr != nil {
					return rootErr
				}
				loadedCommand, loadErr := commands.LoadFromDir(filepath.Join(commandsRoot, ".agents", "commands"), invocation.Name)
				if loadErr != nil {
					return fmt.Errorf("RunCommand.RunE() [run.go]: %w", loadErr)
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

			contextData, err := system.ParseRunContextEntries(cliContext)
			if err != nil {
				return err
			}

			var taskData *core.Task
			var initialTaskData *core.Task
			if strings.TrimSpace(cliTaskJSON) != "" {
				var task core.Task
				if unmarshalErr := json.Unmarshal([]byte(cliTaskJSON), &task); unmarshalErr != nil {
					return fmt.Errorf("RunCommand.RunE() [run.go]: failed to parse --task-json: %w", unmarshalErr)
				}
				task.TaskDir = strings.TrimSpace(cliTaskDir)
				taskData = &task
				initialTaskData = cloneRunTask(taskData)
			}

			if prompt == "" {
				return fmt.Errorf("RunCommand.RunE() [run.go]: prompt cannot be empty")
			}

			bashRunTimeout, err := parseBashRunTimeout(cliBashRunTimeout)
			if err != nil {
				return err
			}

			if err := applyRunDefaults(resolveRunDefaultsFunc, cmd, cliWorkDir, cliShadowDir, cliProjectConfig, cliConfigPath, &cliModel, &cliWorktree, &cliMerge, &cliLogLLMRequests, &cliLogLLMRequestsRaw, &cliThinking, &cliLSPServer, &cliGitUser, &cliGitEmail, &cliMaxThreads, &cliShadowDir, &cliAllowAllPerms, &cliVFSAllow); err != nil {
				return err
			}
			commandContainerEnabled, err := applyCommandRunDefaults(cmd, commandRunDefaults, &cliModel, &cliRole, &cliWorktree, &cliMerge, &cliLogLLMRequests, &cliThinking, &cliLSPServer, &cliGitUser, &cliGitEmail, &cliMaxThreads, &cliShadowDir, &cliAllowAllPerms, &cliVFSAllow, &cliContainerOn, &cliContainerOff, &cliContainerImage, &cliContainerMount, &cliContainerEnv)
			if err != nil {
				return err
			}
			cliLogLLMRequests = cliLogLLMRequests || cliLogLLMRequestsRaw
			modelOverridden := cmd.Flags().Changed("model")

			if invocation != nil {
				if !cmd.Flags().Changed("model") && commandModelOverride != "" {
					cliModel = commandModelOverride
				}
				if !cmd.Flags().Changed("role") && commandRoleOverride != "" {
					cliRole = commandRoleOverride
				}
			}

			containerEnabledChanged := cmd.Flags().Changed("container-enabled")
			containerDisabledChanged := cmd.Flags().Changed("container-disabled")
			if containerEnabledChanged && containerDisabledChanged {
				return fmt.Errorf("RunCommand.RunE() [run.go]: --container-enabled and --container-disabled cannot be used together")
			}

			containerRequested := (containerEnabledChanged && cliContainerOn) || len(cliContainerMount) > 0 || len(cliContainerEnv) > 0
			if !containerEnabledChanged && !containerDisabledChanged && commandContainerEnabled != nil {
				containerRequested = *commandContainerEnabled
			}
			if !containerDisabledChanged && invocation != nil && commandNeedsShell {
				containerRequested = true
			}
			// Parse vfs-allow paths, handling both repeated flags and colon-separated values
			vfsAllowPaths := parseVFSAllowPaths(cliVFSAllow)
			mcpEnableNames := parseMCPServerFlagValues(cliMCPEnable)
			mcpDisableNames := parseMCPServerFlagValues(cliMCPDisable)

			return runFunc(&RunParams{
				Prompt:                prompt,
				CommandName:           commandName,
				CommandArgs:           commandArgs,
				CommandTemplate:       commandTemplate,
				CommandTaskMetadata:   commandTaskMetadata,
				ContextData:           contextData,
				ModelName:             cliModel,
				RoleName:              cliRole,
				Task:                  taskData,
				InitialTask:           initialTaskData,
				WorkDir:               cliWorkDir,
				ShadowDir:             cliShadowDir,
				WorktreeBranch:        cliWorktree,
				GitUserName:           vcs.ResolveGitIdentity(cliGitUser, "user.name"),
				GitUserEmail:          vcs.ResolveGitIdentity(cliGitEmail, "user.email"),
				Merge:                 cliMerge,
				ContainerEnabled:      containerRequested,
				ContainerDisabled:     containerDisabledChanged && cliContainerOff,
				ContainerImage:        cliContainerImage,
				ContainerMounts:       cliContainerMount,
				ContainerEnv:          cliContainerEnv,
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
				ModelOverridden:       modelOverridden,
				BashRunTimeout:        bashRunTimeout,
				MaxThreads:            cliMaxThreads,
				OutputFormat:          cliOutputFormat,
				VFSAllow:              vfsAllowPaths,
				MCPEnable:             mcpEnableNames,
				MCPDisable:            mcpDisableNames,
				HookOverrides:         cliHooks,
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
	cmd.Flags().StringArrayVar(&cliHooks, "hook", nil, "Ephemeral hook override: --hook name | --hook name:disable | --hook name:key=value,key2=value2")
	cmd.Flags().StringArrayVarP(&cliContext, "context", "c", nil, "Template context value in KEY=VAL format (repeatable)")
	cmd.Flags().StringVar(&cliTaskJSON, "task-json", "", "Task metadata payload used for task session state")
	cmd.Flags().StringVar(&cliTaskDir, "task-dir", "", "Task directory path used for task session state")
	_ = cmd.Flags().MarkHidden("task-json")
	_ = cmd.Flags().MarkHidden("task-dir")
	return cmd
}

func runCommand(params *RunParams) error {
	startTime := time.Now()
	ctx := context.Background()
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
		_, _ = fmt.Fprintf(os.Stdout, "[INFO] Worktree branch: %s\n", params.WorktreeBranch)
	}

		sweSystem, buildResult, err := buildSystemFunc(system.BuildSystemParams{
		WorkDir:           params.WorkDir,
		ShadowDir:         params.ShadowDir,
		ConfigPath:        params.ConfigPath,
		ProjectConfig:     params.ProjectConfig,
		ModelName:         params.ModelName,
		RoleName:          params.RoleName,
		WorktreeBranch:    params.WorktreeBranch,
		GitUserName:       params.GitUserName,
		GitUserEmail:      params.GitUserEmail,
		ContainerEnabled:  params.ContainerEnabled,
		ContainerDisabled: params.ContainerDisabled,
		ContainerImage:    params.ContainerImage,
		ContainerMounts:   params.ContainerMounts,
		ContainerEnv:      params.ContainerEnv,
		LSPServer:         params.LSPServer,
		LogLLMRequests:    params.LogLLMRequests,
		LogLLMRequestsRaw: params.LogLLMRequestsRaw,
		NoRefresh:         params.NoRefresh,
		Thinking:          params.Thinking,
		BashRunTimeout:    params.BashRunTimeout,
		AllowedPaths:      params.VFSAllow,
		MaxToolThreads:    params.MaxThreads,
		MCPEnable:         params.MCPEnable,
		MCPDisable:        params.MCPDisable,
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
	hookConfigStore, err := system.BuildRuntimeHookConfigStore(sweSystem.ConfigStore, params.HookOverrides)
	if err != nil {
		return err
	}
	hookEngine := core.NewHookEngine(
		hookConfigStore,
		core.NewDefaultHookRunner(vcs.ChooseGitDiffDir(buildResult.WorkDirRoot, buildResult.WorkDir)),
		buildResult.ShellRunner,
		sweSystem.ModelProviders,
	)
	if err := PreparePromptWithPreRunHook(ctx, params, buildResult.WorkDirRoot, hookEngine); err != nil {
		return err
	}

	if params.LSPServer != "" {
		lspStatus := "disabled"
		if buildResult.LSPStarted {
			lspStatus = "started"
		}
		_, _ = fmt.Fprintf(os.Stdout, "[INFO] LSP %s (workdir: %s)\n", lspStatus, buildResult.LSPWorkDir)
	}
	if strings.TrimSpace(buildResult.ContainerImage) != "" {
		_, _ = fmt.Fprintln(os.Stdout, BuildContainerStartupInfoMessage(buildResult))
	}

	sessionOutput := buildRunSessionOutput(params, os.Stdout)
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
	if sessionInput := buildRunStdinSessionInput(params, runtimeResult.Thread, os.Stdin); sessionInput != nil {
		sessionInput.StartReadingInput()
	}

	finalizeVCS := buildResult.VCS
	finalizeWorktreeBranch := buildResult.WorktreeBranch
	finalizeWorktreeDir := buildResult.WorkDir

	sessionID := session.ID()
	defer func() {
		_, _ = fmt.Fprintf(os.Stdout, "Session ID: %s\n", sessionID)
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
	finalizeResult, finalizeErr := system.FinalizeWorktreeSession(ctx, finalizeVCS, finalizeWorktreeBranch, params.Merge, params.CommitMessageTemplate, sweSystem, session, os.Stderr, buildResult.WorkDirRoot, finalizeWorktreeDir, params.Prompt, hookEngine, buildHookOutputView(sessionOutput))
	if finalizeErr != nil {
		sessionRunErr = finalizeErr
	}
	endTime := time.Now()
	hookEngine.SetContextValue("summary", strings.TrimSpace(core.LastAssistantMessageText(session)))
	if sessionRunErr != nil {
		hookEngine.SetSessionStatus(core.HookSessionStatusFailed)
	} else {
		hookEngine.SetSessionStatus(core.HookSessionStatusSuccess)
	}

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

type runHookOutputView struct {
	output core.SessionThreadOutput
}

func (v *runHookOutputView) ShowMessage(message string, messageType shared.MessageType) {
	if v == nil || v.output == nil {
		return
	}
	v.output.ShowMessage(message, string(messageType))
}

func buildHookOutputView(output core.SessionThreadOutput) core.HookOutputView {
	if output == nil {
		return nil
	}

	return &runHookOutputView{output: output}
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

func PreparePromptWithPreRunHook(ctx context.Context, params *RunParams, workDirRoot string, hookEngine *core.HookEngine) error {
	if params == nil {
		return fmt.Errorf("PreparePromptWithPreRunHook() [run.go]: params is nil")
	}
	if hookEngine == nil {
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

	hookEngine.MergeContext(map[string]string{
		"branch":      strings.TrimSpace(params.WorktreeBranch),
		"workdir":     strings.TrimSpace(params.WorkDir),
		"rootdir":     strings.TrimSpace(workDirRoot),
		"status":      string(core.HookSessionStatusRunning),
		"user_prompt": strings.TrimSpace(params.Prompt),
	})
	hookEngine.MergeContext(params.ContextData)

	if _, err := hookEngine.Execute(ctx, core.HookExecutionRequest{Name: "pre_run"}); err != nil {
		return fmt.Errorf("PreparePromptWithPreRunHook() [run.go]: pre_run hook execution failed: %w", err)
	}

	if strings.TrimSpace(params.Prompt) == "" {
		return nil
	}

	renderedPrompt, err := shared.RenderTextWithContext(params.Prompt, hookEngine.ContextData())
	if err != nil {
		return err
	}
	params.Prompt = strings.TrimSpace(renderedPrompt)
	hookEngine.SetContextValue("user_prompt", params.Prompt)

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
