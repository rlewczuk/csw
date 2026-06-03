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

// RunExecution contains parameters and runtime state for a non-TUI run session.
type RunExecution struct {
	Config              *conf.GlobalConfig
	Stdin               stdio.Reader
	Stdout              stdio.Writer
	Stderr              stdio.Writer
	Prompt              string
	CommandName         string
	CommandPath         string
	CommandArgs         []string
	CommandTemplate     string
	CommandTaskMetadata *core.Task
	ContextData         map[string]any
	Task                *core.Task
	InitialTask         *core.Task
	BashRunTimeout      time.Duration
}

// RunCommand runs a non-TUI agent session with the provided execution params.
func RunCommand(params *RunExecution) error {
	if params == nil || params.Config == nil {
		return fmt.Errorf("RunCommand() [run.go]: params cannot be nil")
	}

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

	prep, err := populateRunExecutionParams(params, stdin)
	if err != nil {
		return err
	}

	startTime := time.Now()
	ctx := context.Background()
	defaults := &params.Config.Defaults

	if defaults.NoCommit {
		defaults.Worktree = ""
		defaults.Merge = false
	}

	if params.BashRunTimeout <= 0 {
		params.BashRunTimeout = defaultBashRunTimeout
	}
	if strings.TrimSpace(defaults.OutputFormat) == "" {
		defaults.OutputFormat = "short"
	}
	if defaults.OutputFormat != "short" && defaults.OutputFormat != "full" && defaults.OutputFormat != "jsonl" {
		return fmt.Errorf("RunCommand() [run.go]: invalid --output-format %q (allowed: short, full, jsonl)", defaults.OutputFormat)
	}
	if err := validateMergeRunExecution(params); err != nil {
		return err
	}

	resolvedWorktreeBranch, err := ResolveWorktreeBranchName(ctx, ResolveWorktreeBranchNameParams{Prompt: params.Prompt, ModelName: defaults.Model, WorkDir: defaults.Workdir, ShadowDir: defaults.ShadowDir, ProjectConfig: defaults.ProjectConfig, ConfigPath: defaults.ConfigPath, WorktreeBranch: defaults.Worktree})
	if err != nil {
		return fmt.Errorf("RunCommand() [run.go]: failed to resolve worktree branch: %w", err)
	}
	defaults.Worktree = resolvedWorktreeBranch
	if defaults.Worktree != "" {
		_, _ = fmt.Fprintf(stdout, "[INFO] Worktree branch: %s\n", defaults.Worktree)
	}

	sweSystem, buildResult, err := BuildSystem(BuildSystemParams{WorkDir: defaults.Workdir, ShadowDir: defaults.ShadowDir, ConfigPath: defaults.ConfigPath, ProjectConfig: defaults.ProjectConfig, ModelName: defaults.Model, RoleName: defaults.Role, WorktreeBranch: defaults.Worktree, GitUserName: defaults.GitUserName, GitUserEmail: defaults.GitUserEmail, ContainerEnabled: defaults.Container.Enabled, ContainerDisabled: defaults.ContainerDisabled, ContainerImage: defaults.Container.Image, ContainerMounts: defaults.Container.Mounts, ContainerEnv: defaults.Container.Env, LSPServer: defaults.LSPServer, LogLLMRequests: defaults.LogLLMRequests, LogLLMRequestsRaw: defaults.LogLLMRequestsRaw, NoRefresh: defaults.NoRefresh, Thinking: defaults.Thinking, BashRunTimeout: params.BashRunTimeout, AllowedPaths: defaults.VFSAllow, MaxToolThreads: defaults.MaxThreads, RunBashMaxOutput: defaults.RunBashMax, VFSReadLimit: defaults.VfsReadLimit, AllowAllPermissions: defaults.AllowAllPermissions})
	if err != nil {
		return err
	}
	defer buildResult.Cleanup()
	defer logging.FlushLogs()

	defaults.Workdir = buildResult.WorkDir
	defaults.ShadowDir = buildResult.ShadowDir
	defaults.Model = buildResult.ModelName
	params.ContextData = BuildPromptContextData(params.ContextData, core.AgentState{Info: core.AgentStateCommonInfo{AgentName: "CSW Coding Agent", WorkDir: buildResult.WorkDir, ShadowDir: buildResult.ShadowDir}, Role: buildResult.RoleConfig.Clone(), Task: cloneRunTask(params.Task)})
	if err := renderCommandPrompt(params, buildResult.WorkDir, buildResult.ShellRunner, buildResult.HostShellRunner); err != nil {
		return err
	}
	if err := PreparePromptWithContext(params); err != nil {
		return err
	}

	if defaults.LSPServer != "" {
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
	}(sweSystem, StartRunSessionParams{ModelName: defaults.Model, RoleName: defaults.Role, Task: params.Task, Thinking: defaults.Thinking, ModelOverridden: defaults.ModelOverridden, Prompt: params.Prompt, OutputHandler: sessionOutput})
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

	finalizeResult, finalizeErr := FinalizeWorktreeSession(ctx, buildResult.VCS, buildResult.WorktreeBranch, defaults.Merge, defaults.CommitMessageTemplate, sweSystem, session, stderr, buildResult.WorkDirRoot, buildResult.WorkDir, params.Prompt)
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
	if prep.runInTaskMode && prep.taskManager != nil && strings.TrimSpace(prep.taskIdentifier) != "" {
		if finalStatus, shouldApply := resolveTaskFinalStatusForRun(session, defaults.Merge); shouldApply {
			if _, err := prep.taskManager.UpdateTask(core.TaskUpdateParams{Identifier: prep.taskIdentifier, Status: &finalStatus}); err != nil {
				return err
			}
		}
	}
	return nil
}

// runExecutionPrepResult holds values resolved by populateRunExecutionParams
// that are required by the rest of RunCommand.
type runExecutionPrepResult struct {
	taskManager    *core.TaskManager
	taskIdentifier string
	runInTaskMode  bool
}

// populateRunExecutionParams resolves prompt, command invocation, context,
// container settings, and task data for a non-TUI run session. It mutates
// both params and its embedded defaults, and returns the task state required
// by the rest of RunCommand.
func populateRunExecutionParams(params *RunExecution, stdin stdio.Reader) (*runExecutionPrepResult, error) {
	if params.Config.Defaults.Role == "" {
		params.Config.Defaults.Role = params.Config.Defaults.DefaultRole
	}
	defaults := &params.Config.Defaults
	if defaults.Container == nil {
		defaults.Container = &conf.ContainerConfig{}
	}
	if defaults.ContainerEnabled {
		defaults.Container.Enabled = true
	}

	var preloadedTaskManager *core.TaskManager
	var taskManager *core.TaskManager
	taskIdentifier := ""
	runInTaskMode := false

	positionalArgs := append([]string(nil), defaults.PositionalArgs...)
	effectiveTaskIdentifier := strings.TrimSpace(defaults.TaskIdentifier)
	if effectiveTaskIdentifier == "" && !defaults.TaskLast && !defaults.TaskNext && len(positionalArgs) > 0 {
		resolvedIdentifier, recognized, err := resolveTaskIdentifierFromPosition(nil, positionalArgs[0])
		if err != nil {
			return nil, err
		}
		if !recognized {
			loadedManager, loadErr := loadRunTaskManager(defaults.TaskDir, defaults.Workdir, defaults.ShadowDir, defaults.ProjectConfig, defaults.ConfigPath)
			if loadErr != nil {
				return nil, loadErr
			}
			preloadedTaskManager = loadedManager
			resolvedIdentifier, recognized, err = resolveTaskIdentifierFromPosition(loadedManager, positionalArgs[0])
			if err != nil {
				return nil, err
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
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: failed to read prompt file: %w", err)
		}
		prompt = string(data)
	} else if prompt == "-" {
		data, err := stdio.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: failed to read prompt from stdin: %w", err)
		}
		prompt = string(data)
	}

	if prompt != "" {
		prompt = strings.TrimSpace(prompt)
	}

	runInTaskMode = strings.TrimSpace(effectiveTaskIdentifier) != "" || defaults.TaskLast || defaults.TaskNext

	invocation, isCommandInvocation, err := commands.ParseInvocation(prompt, extraPositionalArgs)
	if err != nil {
		return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: %w", err)
	}
	if !isCommandInvocation && len(extraPositionalArgs) > 0 {
		return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: prompt must be a single argument unless using /command invocation")
	}

	commandTemplate := ""
	commandName := ""
	commandPath := ""
	commandArgs := []string(nil)
	commandModelOverride := ""
	commandRoleOverride := ""
	var commandRunDefaults *conf.RunDefaultsConfig
	var commandTaskMetadata *core.Task
	commandNeedsShell := false
	if invocation != nil {
		resolvedInvocation, resolveErr := resolveRunCommandInvocation(invocation, defaults.Workdir, defaults.ShadowDir, runInTaskMode)
		if resolveErr != nil {
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: %w", resolveErr)
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
		return nil, err
	}

	if prompt == "" && !runInTaskMode {
		return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: prompt cannot be empty")
	}

	bashRunTimeout, err := parseBashRunTimeout(defaults.BashRunTimeout)
	if err != nil {
		return nil, err
	}

	containerOn := defaults.Container != nil && defaults.Container.Enabled
	containerOff := defaults.ContainerDisabled
	var commandContainerEnabled *bool
	if commandRunDefaults != nil {
		if defaults.Container == nil {
			defaults.Container = &conf.ContainerConfig{}
		}
		defaults.MergeFrom(*commandRunDefaults)
		if defaults.NoCommit {
			defaults.Worktree = ""
			defaults.Merge = false
		}
		if commandRunDefaults.Container != nil && commandRunDefaults.Container.Enabled {
			enabledValue := true
			commandContainerEnabled = &enabledValue
			containerOn = true
			containerOff = false
		}
		if defaults.MaxThreads < 0 {
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: --max-threads must be >= 0")
		}
		if defaults.RunBashMax != nil && *defaults.RunBashMax < 0 {
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: --run-bash-max must be >= 0")
		}
		if defaults.VfsReadLimit != nil && *defaults.VfsReadLimit < 0 {
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: --vfs-read-limit must be >= 0")
		}
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
				return nil, loadErr
			}
		}
		taskManager = manager
		identifier, err := resolveTaskRunIdentifierForRun(manager, effectiveTaskIdentifier, defaults.TaskLast, defaults.TaskNext)
		if err != nil {
			return nil, err
		}
		taskIdentifier = identifier
		taskDir, resolvedTask, err := manager.ResolveTask(core.TaskLookup{Identifier: identifier})
		if err != nil {
			return nil, err
		}
		resolvedTask.TaskDir = taskDir
		params.Task = cloneRunTask(resolvedTask)
		params.InitialTask = cloneRunTask(resolvedTask)
		if !defaults.RoleOverridden && strings.TrimSpace(resolvedTask.Role) != "" {
			defaults.Role = strings.TrimSpace(resolvedTask.Role)
		}
		if prompt == "" {
			taskPromptBytes, readErr := os.ReadFile(filepath.Join(taskDir, "task.md"))
			if readErr != nil {
				return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: failed to read task prompt: %w", readErr)
			}
			prompt = strings.TrimSpace(string(taskPromptBytes))
		}
		if prompt == "" {
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: prompt cannot be empty")
		}
		runningStatus := core.TaskStatusRunning
		if _, updateErr := manager.UpdateTask(core.TaskUpdateParams{Identifier: identifier, Status: &runningStatus}); updateErr != nil {
			return nil, updateErr
		}
		taskRunMerge := resolveTaskRunMerge(defaults.NoMerge, defaults.Merge, defaults.Worktree)
		taskFeatureBranch := ""
		if params.Task != nil {
			taskFeatureBranch = strings.TrimSpace(params.Task.FeatureBranch)
		}
		if shouldDisableTaskWorktreeForRun(commandTaskMetadata, params.Task) || defaults.NoCommit {
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

	params.Prompt = prompt
	params.CommandName = commandName
	params.CommandPath = commandPath
	params.CommandArgs = commandArgs
	params.CommandTemplate = commandTemplate
	params.CommandTaskMetadata = commandTaskMetadata
	params.ContextData = make(map[string]any, len(contextData))
	for key, value := range contextData {
		params.ContextData[key] = value
	}
	params.BashRunTimeout = bashRunTimeout
	defaults.GitUserName = vcs.ResolveGitIdentity(defaults.GitUserName, "user.name")
	defaults.GitUserEmail = vcs.ResolveGitIdentity(defaults.GitUserEmail, "user.email")
	if defaults.Container == nil {
		defaults.Container = &conf.ContainerConfig{}
	}
	defaults.Container.Enabled = containerRequested
	defaults.ContainerDisabled = containerOff
	defaults.VFSAllow = parseVFSAllowPaths(defaults.VFSAllow)

	return &runExecutionPrepResult{
		taskManager:    taskManager,
		taskIdentifier: taskIdentifier,
		runInTaskMode:  runInTaskMode,
	}, nil
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

func buildRunSessionOutput(params *RunExecution, output stdio.Writer) core.SessionThreadOutput {
	if params == nil {
		return sessionio.NewTextSessionOutput(output)
	}
	defaults := &params.Config.Defaults
	if strings.TrimSpace(defaults.OutputFormat) == "jsonl" {
		return sessionio.NewJsonlSessionOutput(output)
	}
	return sessionio.NewTextSessionOutputWithSlug(output, defaults.Worktree)
}
func buildSummaryMessageFunc(output core.SessionThreadOutput) func(string, shared.MessageType) {
	if output == nil {
		return nil
	}
	return func(message string, messageType shared.MessageType) { output.ShowMessage(message, string(messageType)) }
}

type runSessionInput interface{ StartReadingInput() }

func buildRunStdinSessionInput(params *RunExecution, thread core.SessionThreadInput, input stdio.Reader) runSessionInput {
	if params == nil || thread == nil || input == nil {
		return nil
	}
	if params.Config.Defaults.OutputFormat == "jsonl" {
		return sessionio.NewJsonlSessionInput(input, thread)
	}
	return sessionio.NewTextSessionInput(input, thread)
}
func validateMergeRunExecution(params *RunExecution) error {
	if params == nil {
		return fmt.Errorf("validateMergeRunExecution() [run.go]: params cannot be nil")
	}
	defaults := &params.Config.Defaults
	if defaults.Merge && defaults.NoCommit {
		return fmt.Errorf("RunCommand() [run.go]: --merge cannot be used with --no-commit")
	}
	if defaults.Merge && strings.TrimSpace(defaults.Worktree) == "" {
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

func shouldDisableTaskWorktree(taskMetadata *core.Task) bool {
	if taskMetadata == nil || taskMetadata.FieldsPresent&core.TaskFieldFeatureBranch == 0 {
		return false
	}

	return strings.TrimSpace(taskMetadata.FeatureBranch) == ""
}

func shouldDisableTaskWorktreeForRun(taskMetadata *core.Task, taskData *core.Task) bool {
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
func PreparePromptWithContext(params *RunExecution) error {
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
