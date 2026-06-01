package system

import (
	"context"
	"fmt"
	stdio "io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	sessionio "github.com/rlewczuk/csw/pkg/io"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/vcs"
)

const defaultBashRunTimeout = 120 * time.Second

type runExecution struct {
	config                *conf.GlobalConfig
	stdin                 stdio.Reader
	stdout                stdio.Writer
	stderr                stdio.Writer
	prompt                string
	commandName           string
	commandPath           string
	commandArgs           []string
	commandTemplate       string
	commandTaskMetadata   *commands.TaskMetadata
	contextData           map[string]any
	task                  *core.Task
	initialTask           *core.Task
	modelOverridden       bool
	bashRunTimeout        time.Duration
	Prompt                string
	CommandName           string
	CommandPath           string
	CommandArgs           []string
	CommandTemplate       string
	CommandTaskMetadata   *commands.TaskMetadata
	ContextData           map[string]any
	Task                  *core.Task
	InitialTask           *core.Task
	WorkDir               string
	ShadowDir             string
	WorktreeBranch        string
	GitUserName           string
	GitUserEmail          string
	Merge                 bool
	NoCommit              bool
	ContainerEnabled      bool
	ContainerDisabled     bool
	ContainerImage        string
	ContainerMounts       []string
	ContainerEnv          []string
	ConfigPath            string
	ProjectConfig         string
	AllowAllPerms         bool
	LogLLMRequests        bool
	LogLLMRequestsRaw     bool
	NoRefresh             bool
	Interactive           bool
	LSPServer             string
	Thinking              string
	ModelName             string
	RoleName              string
	ModelOverridden       bool
	BashRunTimeout        time.Duration
	RunBashMaxOutput      *int
	VFSReadLimit          *int
	MaxThreads            int
	OutputFormat          string
	VFSAllow              []string
	CommitMessageTemplate string
}

// RunCommand runs a non-TUI agent session with the provided global config.
func RunCommand(globalConfig *conf.GlobalConfig) error {
	if globalConfig == nil {
		return fmt.Errorf("RunCommand() [run.go]: config cannot be nil")
	}
	if globalConfig.Defaults.Role == "" {
		globalConfig.Defaults.Role = globalConfig.Defaults.DefaultRole
	}
	exec := &runExecution{config: globalConfig, stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr}
	return runCommand(exec)
}

func runCommand(params *runExecution) error {
	if params == nil || params.config == nil {
		return fmt.Errorf("runCommand() [run.go]: params cannot be nil")
	}
	defaults := &params.config.Defaults
	if defaults.Container == nil {
		defaults.Container = &conf.ContainerConfig{}
	}
	if defaults.ContainerEnabled {
		defaults.Container.Enabled = true
	}

	startTime := time.Now()
	ctx := context.Background()
	var stdin stdio.Reader = os.Stdin
	var stdout stdio.Writer = os.Stdout
	var stderr stdio.Writer = os.Stderr
	if params.stdin != nil {
		stdin = params.stdin
	}
	if params.stdout != nil {
		stdout = params.stdout
	}
	if params.stderr != nil {
		stderr = params.stderr
	}

	var taskManager *core.TaskManager
	var preloadedTaskManager *core.TaskManager
	taskIdentifier := ""
	runInTaskMode := false

	{
		positionalArgs := append([]string(nil), defaults.PositionalArgs...)
		effectiveTaskIdentifier := strings.TrimSpace(defaults.TaskIdentifier)
		if effectiveTaskIdentifier == "" && !defaults.TaskLast && !defaults.TaskNext && len(positionalArgs) > 0 {
			resolvedIdentifier, recognized, err := resolveTaskIdentifierFromPosition(nil, positionalArgs[0])
			if err != nil {
				return err
			}
			if !recognized {
				loadedManager, loadErr := loadRunTaskManager(defaults.TaskDir, defaults.Workdir, defaults.ShadowDir, defaults.ProjectConfig, defaults.ConfigPath)
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
		defaults.TaskIdentifier = effectiveTaskIdentifier

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

		runInTaskMode = strings.TrimSpace(effectiveTaskIdentifier) != "" || defaults.TaskLast || defaults.TaskNext

		invocation, isCommandInvocation, err := commands.ParseInvocation(prompt, extraPositionalArgs)
		if err != nil {
			return fmt.Errorf("RunCommand() [run.go]: %w", err)
		}
		if !isCommandInvocation && len(extraPositionalArgs) > 0 {
			return fmt.Errorf("RunCommand() [run.go]: prompt must be a single argument unless using /command invocation")
		}

		commandTemplate := ""
		commandName := ""
		commandPath := ""
		commandArgs := []string(nil)
		commandModelOverride := ""
		commandRoleOverride := ""
		var commandRunDefaults *commands.RunDefaultsMetadata
		var commandTaskMetadata *commands.TaskMetadata
		commandNeedsShell := false
		if invocation != nil {
			resolvedInvocation, resolveErr := resolveRunCommandInvocation(invocation, defaults.Workdir, defaults.ShadowDir, runInTaskMode)
			if resolveErr != nil {
				return fmt.Errorf("RunCommand() [run.go]: %w", resolveErr)
			}
			commandTemplate = resolvedInvocation.CommandTemplate
			commandName = resolvedInvocation.CommandName
			commandPath = resolvedInvocation.CommandPath
			commandArgs = resolvedInvocation.CommandArgs
			commandModelOverride = resolvedInvocation.CommandModelOverride
			commandRoleOverride = resolvedInvocation.CommandRoleOverride
			commandRunDefaults = resolvedInvocation.CommandRunDefaults
			commandTaskMetadata = resolvedInvocation.CommandTaskMetadata
			commandNeedsShell = resolvedInvocation.CommandNeedsShell
			prompt = resolvedInvocation.Prompt
			extraPositionalArgs = resolvedInvocation.ExtraPositionalArgs
		}

		contextData, err := ParseRunContextEntries(defaults.ContextEntries)
		if err != nil {
			return err
		}

		if prompt == "" && !runInTaskMode {
			return fmt.Errorf("RunCommand() [run.go]: prompt cannot be empty")
		}

		bashRunTimeout, err := parseBashRunTimeout(defaults.BashRunTimeout)
		if err != nil {
			return err
		}

		containerOn := defaults.Container != nil && defaults.Container.Enabled
		containerOff := defaults.ContainerDisabled
		commandContainerEnabled, err := applyCommandRunDefaults(commandRunDefaults, defaults, &containerOn, &containerOff)
		if err != nil {
			return err
		}
		if defaults.NoMerge {
			defaults.Merge = false
		}
		if defaults.NoCommit {
			defaults.Worktree = ""
			defaults.Merge = false
		}
		defaults.LogLLMRequests = defaults.LogLLMRequests || defaults.LogLLMRequestsRaw

		if invocation != nil {
			if !defaults.ModelOverridden && commandModelOverride != "" {
				defaults.Model = commandModelOverride
			}
			if !defaults.RoleOverridden && commandRoleOverride != "" {
				defaults.Role = commandRoleOverride
			}
		}

		if runInTaskMode {
			manager := preloadedTaskManager
			if manager == nil {
				var loadErr error
				manager, loadErr = loadRunTaskManager(defaults.TaskDir, defaults.Workdir, defaults.ShadowDir, defaults.ProjectConfig, defaults.ConfigPath)
				if loadErr != nil {
					return loadErr
				}
			}
			taskManager = manager
			identifier, err := resolveTaskRunIdentifierForRun(manager, effectiveTaskIdentifier, defaults.TaskLast, defaults.TaskNext)
			if err != nil {
				return err
			}
			taskIdentifier = identifier
			taskDir, resolvedTask, err := manager.ResolveTask(core.TaskLookup{Identifier: identifier})
			if err != nil {
				return err
			}
			resolvedTask.TaskDir = taskDir
			params.task = cloneRunTask(resolvedTask)
			params.initialTask = cloneRunTask(resolvedTask)
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
			taskRunMerge := resolveTaskRunMerge(defaults.NoMerge, defaults.Merge, defaults.Worktree)
			taskFeatureBranch := ""
			if params.task != nil {
				taskFeatureBranch = strings.TrimSpace(params.task.FeatureBranch)
			}
			if shouldDisableTaskWorktreeForRun(commandTaskMetadata, params.task) || defaults.NoCommit {
				defaults.Worktree = ""
				taskRunMerge = false
			} else if strings.TrimSpace(defaults.Worktree) == "" {
				defaults.Worktree = taskFeatureBranch
			}
			defaults.Merge = taskRunMerge
			_ = defaults.TaskReset
		}

		containerRequested := containerOn || (defaults.Container != nil && (len(defaults.Container.Mounts) > 0 || len(defaults.Container.Env) > 0))
		if !defaults.ContainerDisabled && commandContainerEnabled != nil {
			containerRequested = *commandContainerEnabled
		}
		if !defaults.ContainerDisabled && invocation != nil && commandNeedsShell {
			containerRequested = true
		}

		params.prompt = prompt
		params.commandName = commandName
		params.commandPath = commandPath
		params.commandArgs = commandArgs
		params.commandTemplate = commandTemplate
		params.commandTaskMetadata = commandTaskMetadata
		params.contextData = make(map[string]any, len(contextData))
		for key, value := range contextData {
			params.contextData[key] = value
		}
		params.bashRunTimeout = bashRunTimeout
		defaults.GitUserName = vcs.ResolveGitIdentity(defaults.GitUserName, "user.name")
		defaults.GitUserEmail = vcs.ResolveGitIdentity(defaults.GitUserEmail, "user.email")
		if defaults.Container == nil {
			defaults.Container = &conf.ContainerConfig{}
		}
		defaults.Container.Enabled = containerRequested
		defaults.ContainerDisabled = containerOff
		defaults.VFSAllow = parseVFSAllowPaths(defaults.VFSAllow)
	}

	if defaults.NoCommit {
		defaults.Worktree = ""
		defaults.Merge = false
	}

	if params.bashRunTimeout <= 0 {
		params.bashRunTimeout = defaultBashRunTimeout
	}
	if strings.TrimSpace(defaults.OutputFormat) == "" {
		defaults.OutputFormat = "short"
	}
	if defaults.OutputFormat != "short" && defaults.OutputFormat != "full" && defaults.OutputFormat != "jsonl" {
		return fmt.Errorf("RunCommand() [run.go]: invalid --output-format %q (allowed: short, full, jsonl)", defaults.OutputFormat)
	}
	params.Prompt = params.prompt
	params.CommandName = params.commandName
	params.CommandPath = params.commandPath
	params.CommandArgs = params.commandArgs
	params.CommandTemplate = params.commandTemplate
	params.CommandTaskMetadata = params.commandTaskMetadata
	params.ContextData = params.contextData
	params.Task = params.task
	params.InitialTask = params.initialTask
	params.WorkDir = defaults.Workdir
	params.ShadowDir = defaults.ShadowDir
	params.WorktreeBranch = defaults.Worktree
	params.GitUserName = defaults.GitUserName
	params.GitUserEmail = defaults.GitUserEmail
	params.Merge = defaults.Merge
	params.NoCommit = defaults.NoCommit
	params.ContainerEnabled = defaults.Container.Enabled
	params.ContainerDisabled = defaults.ContainerDisabled
	params.ContainerImage = defaults.Container.Image
	params.ContainerMounts = defaults.Container.Mounts
	params.ContainerEnv = defaults.Container.Env
	params.ConfigPath = defaults.ConfigPath
	params.ProjectConfig = defaults.ProjectConfig
	params.AllowAllPerms = defaults.AllowAllPermissions
	params.LogLLMRequests = defaults.LogLLMRequests
	params.LogLLMRequestsRaw = defaults.LogLLMRequestsRaw
	params.NoRefresh = defaults.NoRefresh
	params.LSPServer = defaults.LSPServer
	params.Thinking = defaults.Thinking
	params.ModelName = defaults.Model
	params.RoleName = defaults.Role
	params.ModelOverridden = defaults.ModelOverridden
	params.BashRunTimeout = params.bashRunTimeout
	params.RunBashMaxOutput = defaults.RunBashMax
	params.VFSReadLimit = defaults.VfsReadLimit
	params.MaxThreads = defaults.MaxThreads
	params.OutputFormat = defaults.OutputFormat
	params.VFSAllow = defaults.VFSAllow
	params.CommitMessageTemplate = defaults.CommitMessageTemplate

	if err := validateMergeRunExecution(params); err != nil {
		return err
	}

	resolvedWorktreeBranch, err := ResolveWorktreeBranchName(ctx, ResolveWorktreeBranchNameParams{Prompt: params.Prompt, ModelName: params.ModelName, WorkDir: params.WorkDir, ShadowDir: params.ShadowDir, ProjectConfig: params.ProjectConfig, ConfigPath: params.ConfigPath, WorktreeBranch: params.WorktreeBranch})
	if err != nil {
		return fmt.Errorf("RunCommand() [run.go]: failed to resolve worktree branch: %w", err)
	}
	params.WorktreeBranch = resolvedWorktreeBranch
	if params.WorktreeBranch != "" {
		_, _ = fmt.Fprintf(stdout, "[INFO] Worktree branch: %s\n", params.WorktreeBranch)
	}

	sweSystem, buildResult, err := BuildSystem(BuildSystemParams{WorkDir: params.WorkDir, ShadowDir: params.ShadowDir, ConfigPath: params.ConfigPath, ProjectConfig: params.ProjectConfig, ModelName: params.ModelName, RoleName: params.RoleName, WorktreeBranch: params.WorktreeBranch, GitUserName: params.GitUserName, GitUserEmail: params.GitUserEmail, ContainerEnabled: params.ContainerEnabled, ContainerDisabled: params.ContainerDisabled, ContainerImage: params.ContainerImage, ContainerMounts: params.ContainerMounts, ContainerEnv: params.ContainerEnv, LSPServer: params.LSPServer, LogLLMRequests: params.LogLLMRequests, LogLLMRequestsRaw: params.LogLLMRequestsRaw, NoRefresh: params.NoRefresh, Thinking: params.Thinking, BashRunTimeout: params.BashRunTimeout, AllowedPaths: params.VFSAllow, MaxToolThreads: params.MaxThreads, RunBashMaxOutput: params.RunBashMaxOutput, VFSReadLimit: params.VFSReadLimit, AllowAllPermissions: params.AllowAllPerms})
	if err != nil {
		return err
	}
	defer buildResult.Cleanup()
	defer logging.FlushLogs()

	params.WorkDir = buildResult.WorkDir
	params.ShadowDir = buildResult.ShadowDir
	params.ModelName = buildResult.ModelName
	params.ContextData = BuildPromptContextData(params.ContextData, core.AgentState{Info: core.AgentStateCommonInfo{AgentName: "CSW Coding Agent", WorkDir: buildResult.WorkDir, ShadowDir: buildResult.ShadowDir}, Role: buildResult.RoleConfig.Clone(), Task: cloneRunTask(params.Task)})
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
	for _, message := range BuildRunAgentStartupInfoMessages(params, buildResult) {
		_, _ = fmt.Fprintln(stdout, message)
	}
	if strings.TrimSpace(buildResult.ContainerImage) != "" {
		_, _ = fmt.Fprintln(stdout, BuildContainerStartupInfoMessage(buildResult))
	}

	sessionOutput := buildRunSessionOutput(params, stdout)
	runtimeResult, err := func(sweSystem *SweSystem, params StartRunSessionParams) (StartRunSessionResult, error) {
		return sweSystem.StartRunSession(params)
	}(sweSystem, StartRunSessionParams{ModelName: params.ModelName, RoleName: params.RoleName, Task: params.Task, Thinking: params.Thinking, ModelOverridden: params.ModelOverridden, Prompt: params.Prompt, OutputHandler: sessionOutput})
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

	if err := core.EmitSessionSummary(startTime, endTime, session, core.SessionSummaryBuildResult{LogsDir: buildResult.LogsDir, WorkDirRoot: buildResult.WorkDirRoot, WorkDir: buildResult.WorkDir, LSPServer: buildResult.LSPServer, ContainerImage: buildResult.ContainerImage}, buildSummaryMessageFunc(sessionOutput), sessionRunErr, baseCommitID, finalizeResult.HeadCommitID); err != nil {
		return err
	}
	if err := applyCommandTaskMetadata(params); err != nil {
		return err
	}
	if runInTaskMode && taskManager != nil && strings.TrimSpace(taskIdentifier) != "" {
		if finalStatus, shouldApply := resolveTaskFinalStatusForRun(session, params.Merge); shouldApply {
			if _, err := taskManager.UpdateTask(core.TaskUpdateParams{Identifier: taskIdentifier, Status: &finalStatus}); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveTaskFinalStatusForRun(session *core.SweSession, merge bool) (string, bool) {
	if session != nil && session.TaskStatusUpdatedInSession() {
		return "", false
	}

	if merge {
		return core.TaskStatusMerged, true
	}

	return core.TaskStatusCompleted, true
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

func buildRunSessionOutput(params *runExecution, output stdio.Writer) core.SessionThreadOutput {
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

func buildRunStdinSessionInput(params *runExecution, thread core.SessionThreadInput, input stdio.Reader) runSessionInput {
	if params == nil || thread == nil || input == nil {
		return nil
	}
	if params.OutputFormat == "jsonl" {
		return sessionio.NewJsonlSessionInput(input, thread)
	}
	return sessionio.NewTextSessionInput(input, thread)
}
func validateMergeRunExecution(params *runExecution) error {
	if params == nil {
		return fmt.Errorf("validateMergeRunExecution() [run.go]: params cannot be nil")
	}
	if params.Merge && params.NoCommit {
		return fmt.Errorf("RunCommand() [run.go]: --merge cannot be used with --no-commit")
	}
	if params.Merge && strings.TrimSpace(params.WorktreeBranch) == "" {
		return fmt.Errorf("RunCommand() [run.go]: --merge requires --worktree")
	}
	return nil
}
func resolveTaskRunMerge(mergeFlagChanged bool, cliMerge bool, cliWorktree string) bool {
	if mergeFlagChanged {
		return cliMerge
	}
	if strings.TrimSpace(cliWorktree) != "" {
		return cliMerge
	}
	return cliMerge
}

func shouldDisableTaskWorktree(taskMetadata *commands.TaskMetadata) bool {
	if taskMetadata == nil || taskMetadata.FeatureBranch == nil {
		return false
	}

	return strings.TrimSpace(*taskMetadata.FeatureBranch) == ""
}

func shouldDisableTaskWorktreeForRun(taskMetadata *commands.TaskMetadata, taskData *core.Task) bool {
	if shouldDisableTaskWorktree(taskMetadata) {
		return true
	}
	if taskData == nil {
		return false
	}
	if taskData.NoCommit {
		return true
	}

	return strings.TrimSpace(taskData.FeatureBranch) == ""
}

// PreparePromptWithContext renders prompt with template context data.
func PreparePromptWithContext(params *runExecution) error {
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
