package system

import (
	"context"
	"fmt"
	stdio "io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	sessionio "github.com/rlewczuk/csw/pkg/io"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/vcs"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// RunParams holds all parameters for RunCommand.
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

type runDefaultsResolver func(params ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error)

// TaskManagerLoader loads task manager for run command task-mode.
type TaskManagerLoader func(cmd *cobra.Command) (*core.TaskManager, error)

// TaskRunIdentifierResolver resolves task identifier for run command task-mode.
type TaskRunIdentifierResolver func(manager *core.TaskManager, identifier string, useLast bool, useNext bool) (string, error)

const defaultBashRunTimeout = 120 * time.Second

var resolveRunDefaultsFunc = ResolveRunDefaults
var resolveWorktreeBranchNameFunc = ResolveWorktreeBranchName
var buildSystemFunc = BuildSystem
var startRunSessionFunc = func(sweSystem *SweSystem, params StartRunSessionParams) (StartRunSessionResult, error) {
	return sweSystem.StartRunSession(params)
}
var emitSessionSummaryFunc = core.EmitSessionSummary
var loadTaskManagerFunc TaskManagerLoader
var resolveTaskRunIdentifierFunc TaskRunIdentifierResolver
var runTaskDirUUIDPattern = regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

// SetRunCommandTaskManagerLoader sets task manager loader used by RunCommand.
func SetRunCommandTaskManagerLoader(loader TaskManagerLoader) {
	loadTaskManagerFunc = loader
}

// SetRunCommandTaskRunIdentifierResolver sets task identifier resolver used by RunCommand.
func SetRunCommandTaskRunIdentifierResolver(resolver TaskRunIdentifierResolver) {
	resolveTaskRunIdentifierFunc = resolver
}

// RunCommand runs a non-TUI agent session.
func RunCommand(params *RunParams) error {
	if params == nil {
		return fmt.Errorf("RunCommand() [run.go]: params cannot be nil")
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
	var preloadedTaskManager *core.TaskManager
	taskIdentifier := ""
	runInTaskMode := false

	if params.Command != nil {
		positionalArgs := append([]string(nil), params.PositionalArgs...)
		effectiveTaskIdentifier := strings.TrimSpace(params.TaskIdentifier)
		if effectiveTaskIdentifier == "" && !params.TaskLast && !params.TaskNext && len(positionalArgs) > 0 {
			resolvedIdentifier, recognized, err := resolveTaskIdentifierFromPosition(nil, positionalArgs[0])
			if err != nil {
				return err
			}
			if !recognized && loadTaskManagerFunc != nil {
				loadedManager, loadErr := loadTaskManagerFunc(params.Command)
				if loadErr != nil {
					return loadErr
				}
				preloadedTaskManager = loadedManager
				resolvedIdentifier, recognized, err = resolveTaskIdentifierFromPosition(loadedManager, positionalArgs[0])
				if err != nil {
					return err
				}
			}
			if recognized {
				effectiveTaskIdentifier = resolvedIdentifier
				positionalArgs = positionalArgs[1:]
			}
		}
		params.TaskIdentifier = effectiveTaskIdentifier

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
				return fmt.Errorf("RunCommand() [run.go]: failed to read prompt file: %w", err)
			}
			prompt = string(data)
		} else if prompt == "-" {
			data, err := stdio.ReadAll(stdin)
			if err != nil {
				return fmt.Errorf("RunCommand() [run.go]: failed to read prompt from stdin: %w", err)
			}
			prompt = string(data)
		}

		if prompt != "" {
			prompt = strings.TrimSpace(prompt)
		}

		runInTaskMode = strings.TrimSpace(effectiveTaskIdentifier) != "" || params.TaskLast || params.TaskNext

		invocation, isCommandInvocation, err := commands.ParseInvocation(prompt, extraPositionalArgs)
		if err != nil {
			return fmt.Errorf("RunCommand() [run.go]: %w", err)
		}
		if !isCommandInvocation && len(extraPositionalArgs) > 0 {
			return fmt.Errorf("RunCommand() [run.go]: prompt must be a single argument unless using /command invocation")
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
				return fmt.Errorf("RunCommand() [run.go]: %w", loadErr)
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

		contextData, err := ParseRunContextEntries(params.ContextEntries)
		if err != nil {
			return err
		}

		if prompt == "" && !runInTaskMode {
			return fmt.Errorf("RunCommand() [run.go]: prompt cannot be empty")
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
			return fmt.Errorf("RunCommand() [run.go]: --container-enabled and --container-disabled cannot be used together")
		}

		if runInTaskMode {
			if invocation != nil {
				prompt = "/" + strings.TrimSpace(invocation.Name)
				extraPositionalArgs = append([]string(nil), invocation.Arguments...)
			}
			if loadTaskManagerFunc == nil {
				return fmt.Errorf("RunCommand() [run.go]: task manager loader not configured")
			}
			if resolveTaskRunIdentifierFunc == nil {
				return fmt.Errorf("RunCommand() [run.go]: task run identifier resolver not configured")
			}
			manager := preloadedTaskManager
			if manager == nil {
				var loadErr error
				manager, loadErr = loadTaskManagerFunc(params.Command)
				if loadErr != nil {
					return loadErr
				}
			}
			taskManager = manager
			identifier, err := resolveTaskRunIdentifierFunc(manager, effectiveTaskIdentifier, params.TaskLast, params.TaskNext)
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
					return fmt.Errorf("RunCommand() [run.go]: failed to read task prompt: %w", readErr)
				}
				prompt = strings.TrimSpace(string(taskPromptBytes))
			}
			if prompt == "" {
				return fmt.Errorf("RunCommand() [run.go]: prompt cannot be empty")
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
		return fmt.Errorf("RunCommand() [run.go]: invalid --output-format %q (allowed: short, full, jsonl)", params.OutputFormat)
	}
	if err := validateMergeRunParams(params); err != nil {
		return err
	}

	resolvedWorktreeBranch, err := resolveWorktreeBranchNameFunc(ctx, ResolveWorktreeBranchNameParams{Prompt: params.Prompt, ModelName: params.ModelName, WorkDir: params.WorkDir, ShadowDir: params.ShadowDir, ProjectConfig: params.ProjectConfig, ConfigPath: params.ConfigPath, WorktreeBranch: params.WorktreeBranch})
	if err != nil {
		return fmt.Errorf("RunCommand() [run.go]: failed to resolve worktree branch: %w", err)
	}
	params.WorktreeBranch = resolvedWorktreeBranch
	if params.WorktreeBranch != "" {
		_, _ = fmt.Fprintf(stdout, "[INFO] Worktree branch: %s\n", params.WorktreeBranch)
	}

	sweSystem, buildResult, err := buildSystemFunc(BuildSystemParams{WorkDir: params.WorkDir, ShadowDir: params.ShadowDir, ConfigPath: params.ConfigPath, ProjectConfig: params.ProjectConfig, ModelName: params.ModelName, RoleName: params.RoleName, WorktreeBranch: params.WorktreeBranch, GitUserName: params.GitUserName, GitUserEmail: params.GitUserEmail, ContainerEnabled: params.ContainerEnabled, ContainerDisabled: params.ContainerDisabled, ContainerImage: params.ContainerImage, ContainerMounts: params.ContainerMounts, ContainerEnv: params.ContainerEnv, LSPServer: params.LSPServer, LogLLMRequests: params.LogLLMRequests, LogLLMRequestsRaw: params.LogLLMRequestsRaw, NoRefresh: params.NoRefresh, Thinking: params.Thinking, BashRunTimeout: params.BashRunTimeout, AllowedPaths: params.VFSAllow, MaxToolThreads: params.MaxThreads, AllowAllPermissions: params.AllowAllPerms, MCPEnable: params.MCPEnable, MCPDisable: params.MCPDisable})
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
	runtimeResult, err := startRunSessionFunc(sweSystem, StartRunSessionParams{ModelName: params.ModelName, RoleName: params.RoleName, Task: params.Task, Thinking: params.Thinking, ModelOverridden: params.ModelOverridden, Prompt: params.Prompt, OutputHandler: sessionOutput})
	if err != nil {
		return fmt.Errorf("RunCommand() [run.go]: failed to start run session runtime: %w", err)
	}
	session := runtimeResult.Session
	if sessionInput := buildRunStdinSessionInput(params, runtimeResult.Thread, stdin); sessionInput != nil {
		sessionInput.StartReadingInput()
	}

	baseCommitID := vcs.ResolveGitCommitID(vcs.ChooseGitDiffDir(buildResult.WorkDirRoot, buildResult.WorkDir), "HEAD")
	var sessionRunErr error
	select {
	case err := <-runtimeResult.Done:
		if err != nil {
			sessionRunErr = fmt.Errorf("RunCommand() [run.go]: session error: %w", err)
		}
	case <-ctx.Done():
		sessionRunErr = ctx.Err()
	}

	finalizeResult, finalizeErr := FinalizeWorktreeSession(ctx, buildResult.VCS, buildResult.WorktreeBranch, params.Merge, params.CommitMessageTemplate, sweSystem, session, stderr, buildResult.WorkDirRoot, buildResult.WorkDir, params.Prompt)
	if finalizeErr != nil {
		sessionRunErr = finalizeErr
	}
	endTime := time.Now()

	if err := emitSessionSummaryFunc(startTime, endTime, session, core.SessionSummaryBuildResult{LogsDir: buildResult.LogsDir, WorkDirRoot: buildResult.WorkDirRoot, WorkDir: buildResult.WorkDir, LSPServer: buildResult.LSPServer, ContainerImage: buildResult.ContainerImage}, buildSummaryMessageFunc(sessionOutput), sessionRunErr, baseCommitID, finalizeResult.HeadCommitID); err != nil {
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

func resolveTaskIdentifierFromPosition(manager *core.TaskManager, candidate string) (string, bool, error) {
	trimmedCandidate := strings.TrimSpace(candidate)
	if trimmedCandidate == "" {
		return "", false, nil
	}
	if runTaskDirUUIDPattern.MatchString(trimmedCandidate) {
		return trimmedCandidate, true, nil
	}
	if manager == nil {
		return "", false, nil
	}

	tasks, err := listAllCurrentTasksForRun(manager)
	if err != nil {
		return "", false, err
	}

	matchedUUID := ""
	for _, taskData := range tasks {
		if taskData == nil || strings.TrimSpace(taskData.FeatureBranch) != trimmedCandidate {
			continue
		}
		if matchedUUID != "" && matchedUUID != strings.TrimSpace(taskData.UUID) {
			return "", false, fmt.Errorf("resolveTaskIdentifierFromPosition() [run.go]: multiple tasks match feature branch %q", trimmedCandidate)
		}
		matchedUUID = strings.TrimSpace(taskData.UUID)
	}

	if matchedUUID == "" {
		return "", false, nil
	}

	return matchedUUID, true, nil
}

func listAllCurrentTasksForRun(manager *core.TaskManager) ([]*core.Task, error) {
	if manager == nil {
		return nil, fmt.Errorf("listAllCurrentTasksForRun() [run.go]: manager cannot be nil")
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

func applyRunDefaults(resolver runDefaultsResolver, cmd *cobra.Command, workDir string, shadowDir string, projectConfig string, configPath string, model *string, worktree *string, merge *bool, logLLMRequests *bool, logLLMRequestsRaw *bool, thinking *string, lspServer *string, gitUser *string, gitEmail *string, maxThreads *int, shadowDirOut *string, allowAllPerms *bool, vfsAllow *[]string) error {
	defaults, err := resolver(ResolveRunDefaultsParams{WorkDir: workDir, ShadowDir: shadowDir, ProjectConfig: projectConfig, ConfigPath: configPath})
	if err != nil {
		return fmt.Errorf("applyRunDefaults() [run.go]: failed to resolve run defaults: %w", err)
	}
	if !cmd.Flags().Changed("model") && defaults.Model != "" {
		*model = defaults.Model
	}
	if !cmd.Flags().Changed("worktree") && defaults.Worktree != "" {
		*worktree = defaults.Worktree
	}
	if !cmd.Flags().Changed("merge") && defaults.Merge {
		*merge = true
	}
	if !cmd.Flags().Changed("log-llm-requests") && defaults.LogLLMRequests {
		*logLLMRequests = true
	}
	if !cmd.Flags().Changed("log-llm-requests-raw") && defaults.LogLLMRequestsRaw {
		*logLLMRequestsRaw = true
	}
	if !isThinkingFlagChanged(cmd) && defaults.Thinking != "" {
		*thinking = defaults.Thinking
	}
	if !cmd.Flags().Changed("lsp-server") && defaults.LSPServer != "" {
		*lspServer = defaults.LSPServer
	}
	if !cmd.Flags().Changed("git-user") && defaults.GitUserName != "" {
		*gitUser = defaults.GitUserName
	}
	if !cmd.Flags().Changed("git-email") && defaults.GitUserEmail != "" {
		*gitEmail = defaults.GitUserEmail
	}
	if !cmd.Flags().Changed("max-threads") && defaults.MaxThreads > 0 {
		*maxThreads = defaults.MaxThreads
	}
	if !cmd.Flags().Changed("shadow-dir") && defaults.ShadowDir != "" {
		*shadowDirOut = defaults.ShadowDir
	}
	if !cmd.Flags().Changed("allow-all-permissions") && defaults.AllowAllPermissions {
		*allowAllPerms = true
	}
	if !cmd.Flags().Changed("vfs-allow") && len(defaults.VFSAllow) > 0 {
		*vfsAllow = append([]string(nil), defaults.VFSAllow...)
	}
	if *maxThreads < 0 {
		return fmt.Errorf("applyRunDefaults() [run.go]: --max-threads must be >= 0")
	}
	return nil
}

func applyCommandRunDefaults(cmd *cobra.Command, defaults *commands.RunDefaultsMetadata, model *string, role *string, worktree *string, merge *bool, logLLMRequests *bool, thinking *string, lspServer *string, gitUser *string, gitEmail *string, maxThreads *int, shadowDirOut *string, allowAllPerms *bool, vfsAllow *[]string, containerOn *bool, containerOff *bool, containerImage *string, containerMounts *[]string, containerEnv *[]string) (*bool, error) {
	if defaults == nil {
		return nil, nil
	}
	if !cmd.Flags().Changed("model") && defaults.Model != nil {
		*model = strings.TrimSpace(*defaults.Model)
	}
	if !cmd.Flags().Changed("role") && defaults.DefaultRole != nil {
		*role = strings.TrimSpace(*defaults.DefaultRole)
	}
	if !cmd.Flags().Changed("worktree") && defaults.Worktree != nil {
		*worktree = strings.TrimSpace(*defaults.Worktree)
	}
	if !cmd.Flags().Changed("merge") && defaults.Merge != nil {
		*merge = *defaults.Merge
	}
	if !cmd.Flags().Changed("log-llm-requests") && defaults.LogLLMRequests != nil {
		*logLLMRequests = *defaults.LogLLMRequests
	}
	if !isThinkingFlagChanged(cmd) && defaults.Thinking != nil {
		*thinking = strings.TrimSpace(*defaults.Thinking)
	}
	if !cmd.Flags().Changed("lsp-server") && defaults.LSPServer != nil {
		*lspServer = strings.TrimSpace(*defaults.LSPServer)
	}
	if !cmd.Flags().Changed("git-user") && defaults.GitUserName != nil {
		*gitUser = strings.TrimSpace(*defaults.GitUserName)
	}
	if !cmd.Flags().Changed("git-email") && defaults.GitUserEmail != nil {
		*gitEmail = strings.TrimSpace(*defaults.GitUserEmail)
	}
	if !cmd.Flags().Changed("max-threads") && defaults.MaxThreads != nil {
		*maxThreads = *defaults.MaxThreads
	}
	if !cmd.Flags().Changed("shadow-dir") && defaults.ShadowDir != nil {
		*shadowDirOut = strings.TrimSpace(*defaults.ShadowDir)
	}
	if !cmd.Flags().Changed("allow-all-permissions") && defaults.AllowAllPermissions != nil {
		*allowAllPerms = *defaults.AllowAllPermissions
	}
	if !cmd.Flags().Changed("vfs-allow") && defaults.VFSAllow != nil {
		*vfsAllow = append([]string(nil), *defaults.VFSAllow...)
	}
	var commandContainerEnabled *bool
	if defaults.Container != nil {
		if !cmd.Flags().Changed("container-image") && defaults.Container.Image != nil {
			*containerImage = strings.TrimSpace(*defaults.Container.Image)
		}
		if !cmd.Flags().Changed("container-mount") && defaults.Container.Mounts != nil {
			*containerMounts = append([]string(nil), *defaults.Container.Mounts...)
		}
		if !cmd.Flags().Changed("container-env") && defaults.Container.Env != nil {
			*containerEnv = append([]string(nil), *defaults.Container.Env...)
		}
		if defaults.Container.Enabled != nil && !cmd.Flags().Changed("container-enabled") && !cmd.Flags().Changed("container-disabled") {
			enabledValue := *defaults.Container.Enabled
			commandContainerEnabled = &enabledValue
			*containerOn = enabledValue
			*containerOff = !enabledValue
		}
	}
	if *maxThreads < 0 {
		return commandContainerEnabled, fmt.Errorf("applyCommandRunDefaults() [run.go]: --max-threads must be >= 0")
	}
	return commandContainerEnabled, nil
}

func isThinkingFlagChanged(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Flags().Changed("thinking")
}
func parseBashRunTimeout(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultBashRunTimeout, nil
	}
	if _, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		trimmed += "s"
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parseBashRunTimeout() [run.go]: invalid --bash-run-timeout value %q: %w", value, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("parseBashRunTimeout() [run.go]: --bash-run-timeout must be positive, got %q", value)
	}
	return parsed, nil
}
func parseVFSAllowPaths(values []string) []string {
	var r []string
	for _, v := range values {
		for _, p := range strings.Split(v, ":") {
			if p = strings.TrimSpace(p); p != "" {
				r = append(r, p)
			}
		}
	}
	return r
}
func parseMCPServerFlagValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}
	return result
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
	if err := os.WriteFile(taskFilePath, updatedBytes, 0o644); err != nil {
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
	return func(message string, messageType shared.MessageType) { output.ShowMessage(message, string(messageType)) }
}

type runSessionInput interface{ StartReadingInput() }

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
		return fmt.Errorf("RunCommand() [run.go]: --merge requires --worktree")
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
	defaults, err := resolver(ResolveRunDefaultsParams{WorkDir: workDir, ShadowDir: shadowDir, ProjectConfig: projectConfig, ConfigPath: configPath})
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
		resolvedShadowDir, err := ResolveWorkDir(shadowDir)
		if err != nil {
			return "", fmt.Errorf("resolveCommandsRootDir() [run.go]: failed to resolve shadow directory: %w", err)
		}
		return resolvedShadowDir, nil
	}
	resolvedWorkDir, err := ResolveWorkDir(workDir)
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

// PreparePromptWithContext renders prompt with template context data.
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

// BuildContainerStartupInfoMessage builds container runtime info line.
func BuildContainerStartupInfoMessage(buildResult BuildSystemResult) string {
	identity := buildResult.ContainerIdentity
	return fmt.Sprintf("[INFO] Container: image=%s tag=%s version=%s user=%s(uid=%d) group=%s(gid=%d)", shared.NullValue(strings.TrimSpace(buildResult.ContainerImageName)), shared.NullValue(strings.TrimSpace(buildResult.ContainerImageTag)), shared.NullValue(strings.TrimSpace(buildResult.ContainerImageVersion)), shared.NullValue(strings.TrimSpace(identity.UserName)), identity.UID, shared.NullValue(strings.TrimSpace(identity.GroupName)), identity.GID)
}
