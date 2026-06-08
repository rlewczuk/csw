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

// RunCommand runs a non-TUI agent session with the provided execution params.
//
// The caller must pass a non-nil params with a non-nil Config field; the
// function does not guard against nil Config and will panic in that case.
func RunCommand(params *core.RunExecution) error {
	if params == nil {
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
	parameters := &params.Config.GlobalConfig.Parameters

	if parameters.NoCommit {
		parameters.Worktree = ""
		parameters.Merge = false
	}

	if params.BashRunTimeout <= 0 {
		params.BashRunTimeout = defaultBashRunTimeout
	}
	if strings.TrimSpace(parameters.OutputFormat) == "" {
		parameters.OutputFormat = "short"
	}
	if parameters.OutputFormat != "short" && parameters.OutputFormat != "full" && parameters.OutputFormat != "jsonl" {
		return fmt.Errorf("RunCommand() [run.go]: invalid --output-format %q (allowed: short, full, jsonl)", parameters.OutputFormat)
	}
	if err := validateMergeRunExecution(params); err != nil {
		return err
	}
	if err := mergeRunExecutionConfig(params.Config, parameters); err != nil {
		return err
	}
	parameters = &params.Config.GlobalConfig.Parameters

	resolvedWorktreeBranch, err := ResolveWorktreeBranchName(ctx, ResolveWorktreeBranchNameParams{Prompt: params.Prompt, ModelName: parameters.Model, WorkDir: parameters.Workdir, ShadowDir: parameters.ShadowDir, ProjectConfig: parameters.ProjectConfig, ConfigPath: parameters.ConfigPath, WorktreeBranch: parameters.Worktree})
	if err != nil {
		return fmt.Errorf("RunCommand() [run.go]: failed to resolve worktree branch: %w", err)
	}
	parameters.Worktree = resolvedWorktreeBranch
	if parameters.Worktree != "" {
		_, _ = fmt.Fprintf(stdout, "[INFO] Worktree branch: %s\n", parameters.Worktree)
	}

	parameters.BashRunTimeout = params.BashRunTimeout.String()
	sweSystem, err := BuildSystem(params.Config)
	if err != nil {
		return err
	}
	runtimeConfig := params.Config.Runtime
	if runtimeConfig.Cleanup != nil {
		defer runtimeConfig.Cleanup()
	}
	defer logging.FlushLogs()

	roleConfig, _ := core.NewAgentRoleRegistry(params.Config).Get(parameters.Role)
	params.ContextData = BuildPromptContextData(params.ContextData, core.AgentState{Info: core.AgentStateCommonInfo{AgentName: "CSW Coding Agent", WorkDir: parameters.Workdir, ShadowDir: parameters.ShadowDir}, Role: roleConfig.Clone(), Task: cloneRunTask(params.Task), Config: params.Config})
	if err := renderCommandPrompt(params, parameters.Workdir, runtimeConfig.ShellRunner, runtimeConfig.HostShellRunner); err != nil {
		return err
	}
	if err := PreparePromptWithContext(params); err != nil {
		return err
	}

	if parameters.LSPServer != "" {
		lspStatus := "disabled"
		if runtimeConfig.LSPStarted {
			lspStatus = "started"
		}
		_, _ = fmt.Fprintf(stdout, "[INFO] LSP %s (workdir: %s)\n", lspStatus, parameters.Workdir)
	}
	for _, message := range BuildRunAgentStartupInfoMessages(params) {
		_, _ = fmt.Fprintln(stdout, message)
	}
	if parameters.Container != nil && parameters.Container.Enabled && strings.TrimSpace(parameters.Container.Image) != "" {
		_, _ = fmt.Fprintln(stdout, BuildContainerStartupInfoMessage(params.Config))
	}

	sessionOutput := buildRunSessionOutput(params, stdout)
	runtimeResult, err := func(sweSystem *SweSystem, params StartRunSessionParams) (StartRunSessionResult, error) {
		return sweSystem.StartRunSession(params)
	}(sweSystem, StartRunSessionParams{Execution: params, ModelName: parameters.Model, RoleName: parameters.Role, Task: params.Task, Thinking: parameters.Thinking, ModelOverridden: parameters.ModelOverridden, Prompt: params.Prompt, OutputHandler: sessionOutput})
	if err != nil {
		return fmt.Errorf("RunCommand() [run.go]: failed to start run session runtime: %w", err)
	}
	session := runtimeResult.Session
	if sessionInput := buildRunStdinSessionInput(params, runtimeResult.Thread, stdin); sessionInput != nil {
		sessionInput.StartReadingInput()
	}

	baseCommitID := vcs.ResolveGitCommitID(vcs.ChooseGitDiffDir(sweSystem.WorkDirRoot, parameters.Workdir), "HEAD")
	var sessionRunErr error
	select {
	case err := <-runtimeResult.Done:
		if err != nil {
			sessionRunErr = fmt.Errorf("RunCommand() [run.go]: session error: %w", err)
		}
	case <-ctx.Done():
		sessionRunErr = ctx.Err()
	}

	finalizeResult, finalizeErr := FinalizeWorktreeSession(ctx, runtimeConfig.VCS, sweSystem, session, stderr, params.Prompt)
	if finalizeErr != nil {
		sessionRunErr = finalizeErr
	}
	endTime := time.Now()

	containerImage := ""
	if parameters.Container != nil && parameters.Container.Enabled {
		containerImage = parameters.Container.Image
	}
	logsDirRoot := sweSystem.WorkDirRoot
	if strings.TrimSpace(parameters.ShadowDir) != "" {
		logsDirRoot = parameters.ShadowDir
	}
	logsDir := filepath.Join(logsDirRoot, ".cswdata", "logs")
	if err := core.EmitSessionSummary(startTime, endTime, session, core.SessionSummaryBuildResult{LogsDir: logsDir, WorkDirRoot: sweSystem.WorkDirRoot, WorkDir: parameters.Workdir, LSPServer: parameters.LSPServer, ContainerImage: containerImage}, buildSummaryMessageFunc(sessionOutput), sessionRunErr, baseCommitID, finalizeResult.HeadCommitID); err != nil {
		return err
	}
	if err := applyCommandTaskMetadata(params); err != nil {
		return err
	}
	if prep.runInTaskMode && prep.taskManager != nil && strings.TrimSpace(prep.taskIdentifier) != "" {
		if finalStatus, shouldApply := resolveTaskFinalStatusForRun(session, parameters.Merge); shouldApply {
			if _, err := prep.taskManager.UpdateTask(core.TaskUpdateParams{Identifier: prep.taskIdentifier, Status: &finalStatus}); err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeRunExecutionConfig(configStore *conf.CswConfig, parameters *conf.RunParameters) error {
	if configStore == nil || parameters == nil {
		return nil
	}
	configPath, err := BuildConfigPath(parameters.ProjectConfig, parameters.ConfigPath)
	if err != nil {
		return fmt.Errorf("mergeRunExecutionConfig() [run.go]: failed to build config path: %w", err)
	}
	configRoot, err := ResolveWorkDir(parameters.Workdir)
	if err != nil {
		return fmt.Errorf("mergeRunExecutionConfig() [run.go]: failed to resolve work directory: %w", err)
	}
	if strings.TrimSpace(parameters.ShadowDir) != "" {
		resolvedShadowDir, shadowErr := ResolveWorkDir(parameters.ShadowDir)
		if shadowErr != nil {
			return fmt.Errorf("mergeRunExecutionConfig() [run.go]: failed to resolve shadow directory: %w", shadowErr)
		}
		configRoot = resolvedShadowDir
	}
	resolvedConfigPath, err := ResolveConfigPathForProjectRoot(configPath, configRoot)
	if err != nil {
		return fmt.Errorf("mergeRunExecutionConfig() [run.go]: failed to resolve config path for project root: %w", err)
	}
	loadedConfig, err := conf.CswConfigLoad(resolvedConfigPath)
	if err != nil {
		return fmt.Errorf("mergeRunExecutionConfig() [run.go]: failed to load config: %w", err)
	}
	if loadedConfig.GlobalConfig != nil && loadedConfig.GlobalConfig.Parameters.Container != nil && parameters.Container != nil {
		loadedConfig.GlobalConfig.Parameters.Container.Merge(*parameters.Container)
		parameters.Container = nil
	}
	if loadedConfig.GlobalConfig != nil {
		loadedConfig.GlobalConfig.Parameters.MergeFrom(*parameters)
		loadedConfig.GlobalConfig.Parameters.Role = parameters.Role
		loadedConfig.GlobalConfig.Parameters.NoMerge = parameters.NoMerge
		loadedConfig.GlobalConfig.Parameters.BashRunTimeout = parameters.BashRunTimeout
		loadedConfig.GlobalConfig.Parameters.CommitMessageTemplate = parameters.CommitMessageTemplate
		loadedConfig.GlobalConfig.Parameters.ConfigPath = parameters.ConfigPath
		loadedConfig.GlobalConfig.Parameters.ProjectConfig = parameters.ProjectConfig
		loadedConfig.GlobalConfig.Parameters.Interactive = parameters.Interactive
		loadedConfig.GlobalConfig.Parameters.ContainerEnabled = parameters.ContainerEnabled
		loadedConfig.GlobalConfig.Parameters.SaveSessionTo = parameters.SaveSessionTo
		loadedConfig.GlobalConfig.Parameters.SaveSession = parameters.SaveSession
		loadedConfig.GlobalConfig.Parameters.NoRefresh = parameters.NoRefresh
		loadedConfig.GlobalConfig.Parameters.ContainerDisabled = parameters.ContainerDisabled
		loadedConfig.GlobalConfig.Parameters.OutputFormat = parameters.OutputFormat
		loadedConfig.GlobalConfig.Parameters.ContextEntries = append([]string(nil), parameters.ContextEntries...)
		loadedConfig.GlobalConfig.Parameters.TaskIdentifier = parameters.TaskIdentifier
		loadedConfig.GlobalConfig.Parameters.TaskNext = parameters.TaskNext
		loadedConfig.GlobalConfig.Parameters.TaskLast = parameters.TaskLast
		loadedConfig.GlobalConfig.Parameters.TaskReset = parameters.TaskReset
		loadedConfig.GlobalConfig.Parameters.PositionalArgs = append([]string(nil), parameters.PositionalArgs...)
		loadedConfig.GlobalConfig.Parameters.ModelOverridden = parameters.ModelOverridden
		loadedConfig.GlobalConfig.Parameters.RoleOverridden = parameters.RoleOverridden
		loadedConfig.GlobalConfig.Parameters.ShadowDir = parameters.ShadowDir
	}
	if loadedConfig.ModelProviderConfigs != nil {
		configStore.ModelProviderConfigs = loadedConfig.ModelProviderConfigs
	}
	if loadedConfig.AgentRoleConfigs != nil {
		configStore.AgentRoleConfigs = loadedConfig.AgentRoleConfigs
	}
	if loadedConfig.AgentConfigFiles != nil {
		configStore.AgentConfigFiles = loadedConfig.AgentConfigFiles
	}
	if loadedConfig.ModelAliases != nil {
		configStore.ModelAliases = loadedConfig.ModelAliases
	}
	if loadedConfig.GlobalConfig != nil {
		configStore.GlobalConfig = loadedConfig.GlobalConfig
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
// both params and its embedded parameters, and returns the task state required
// by the rest of RunCommand.
func populateRunExecutionParams(params *core.RunExecution, stdin stdio.Reader) (*runExecutionPrepResult, error) {
	parameters := &params.Config.GlobalConfig.Parameters
	if parameters.Role == "" {
		parameters.Role = parameters.DefaultRole
	}
	if parameters.Container == nil {
		parameters.Container = &conf.ContainerConfig{}
	}
	if parameters.ContainerEnabled {
		parameters.Container.Enabled = true
	}

	var preloadedTaskManager *core.TaskManager
	var taskManager *core.TaskManager
	taskIdentifier := ""
	runInTaskMode := false

	positionalArgs := append([]string(nil), parameters.PositionalArgs...)
	effectiveTaskIdentifier := strings.TrimSpace(parameters.TaskIdentifier)
	if effectiveTaskIdentifier == "" && !parameters.TaskLast && !parameters.TaskNext && len(positionalArgs) > 0 {
		resolvedIdentifier, recognized, err := resolveTaskIdentifierFromPosition(nil, positionalArgs[0])
		if err != nil {
			return nil, err
		}
		if !recognized {
			loadedManager, loadErr := loadRunTaskManager(parameters.TaskDir, parameters.Workdir, parameters.ShadowDir, parameters.ProjectConfig, parameters.ConfigPath)
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
	parameters.TaskIdentifier = effectiveTaskIdentifier

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

	runInTaskMode = strings.TrimSpace(effectiveTaskIdentifier) != "" || parameters.TaskLast || parameters.TaskNext

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
	var commandRunParameters *conf.RunParameters
	var commandTaskMetadata *core.Task
	commandNeedsShell := false
	if invocation != nil {
		resolvedInvocation, resolveErr := resolveRunCommandInvocation(invocation, parameters.Workdir, parameters.ShadowDir, runInTaskMode)
		if resolveErr != nil {
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: %w", resolveErr)
		}
		commandTemplate = resolvedInvocation.CommandTemplate
		commandName = resolvedInvocation.CommandName
		commandPath = resolvedInvocation.CommandPath
		commandArgs = resolvedInvocation.CommandArgs
		commandModelOverride = resolvedInvocation.CommandModelOverride
		commandRoleOverride = resolvedInvocation.CommandRoleOverride
		commandRunParameters = resolvedInvocation.CommandRunParameters
		commandTaskMetadata = resolvedInvocation.CommandTaskMetadata
		commandNeedsShell = resolvedInvocation.CommandNeedsShell
		prompt = resolvedInvocation.Prompt
		extraPositionalArgs = resolvedInvocation.ExtraPositionalArgs
	}

	contextData, err := ParseRunContextEntries(parameters.ContextEntries)
	if err != nil {
		return nil, err
	}

	if prompt == "" && !runInTaskMode {
		return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: prompt cannot be empty")
	}

	bashRunTimeout, err := parseBashRunTimeout(parameters.BashRunTimeout)
	if err != nil {
		return nil, err
	}

	containerOn := parameters.Container != nil && parameters.Container.Enabled
	containerOff := parameters.ContainerDisabled
	var commandContainerEnabled *bool
	if commandRunParameters != nil {
		if parameters.Container == nil {
			parameters.Container = &conf.ContainerConfig{}
		}
		parameters.MergeFrom(*commandRunParameters)
		if parameters.NoCommit {
			parameters.Worktree = ""
			parameters.Merge = false
		}
		if commandRunParameters.Container != nil && commandRunParameters.Container.Enabled {
			enabledValue := true
			commandContainerEnabled = &enabledValue
			containerOn = true
			containerOff = false
		}
		if parameters.MaxThreads < 0 {
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: --max-threads must be >= 0")
		}
		if parameters.RunBashMax != nil && *parameters.RunBashMax < 0 {
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: --run-bash-max must be >= 0")
		}
		if parameters.VfsReadLimit != nil && *parameters.VfsReadLimit < 0 {
			return nil, fmt.Errorf("populateRunExecutionParams() [run.go]: --vfs-read-limit must be >= 0")
		}
	}
	if parameters.NoMerge {
		parameters.Merge = false
	}
	if parameters.NoCommit {
		parameters.Worktree = ""
		parameters.Merge = false
	}
	parameters.LogLLMRequests = parameters.LogLLMRequests || parameters.LogLLMRequestsRaw

	if invocation != nil {
		if !parameters.ModelOverridden && commandModelOverride != "" {
			parameters.Model = commandModelOverride
		}
		if !parameters.RoleOverridden && commandRoleOverride != "" {
			parameters.Role = commandRoleOverride
		}
	}

	if runInTaskMode {
		manager := preloadedTaskManager
		if manager == nil {
			var loadErr error
			manager, loadErr = loadRunTaskManager(parameters.TaskDir, parameters.Workdir, parameters.ShadowDir, parameters.ProjectConfig, parameters.ConfigPath)
			if loadErr != nil {
				return nil, loadErr
			}
		}
		taskManager = manager
		identifier, err := resolveTaskRunIdentifierForRun(manager, effectiveTaskIdentifier, parameters.TaskLast, parameters.TaskNext)
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
		if !parameters.RoleOverridden && strings.TrimSpace(resolvedTask.Role) != "" {
			parameters.Role = strings.TrimSpace(resolvedTask.Role)
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
		taskRunMerge := resolveTaskRunMerge(parameters.NoMerge, parameters.Merge, parameters.Worktree)
		taskFeatureBranch := ""
		if params.Task != nil {
			taskFeatureBranch = strings.TrimSpace(params.Task.FeatureBranch)
		}
		if shouldDisableTaskWorktreeForRun(commandTaskMetadata, params.Task) || parameters.NoCommit {
			parameters.Worktree = ""
			taskRunMerge = false
		} else if strings.TrimSpace(parameters.Worktree) == "" {
			parameters.Worktree = taskFeatureBranch
		}
		parameters.Merge = taskRunMerge
		_ = parameters.TaskReset
	}

	containerRequested := containerOn || (parameters.Container != nil && (len(parameters.Container.Mounts) > 0 || len(parameters.Container.Env) > 0))
	if !parameters.ContainerDisabled && commandContainerEnabled != nil {
		containerRequested = *commandContainerEnabled
	}
	if !parameters.ContainerDisabled && invocation != nil && commandNeedsShell {
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
	parameters.GitUserName = vcs.ResolveGitIdentity(parameters.GitUserName, "user.name")
	parameters.GitUserEmail = vcs.ResolveGitIdentity(parameters.GitUserEmail, "user.email")
	if parameters.Container == nil {
		parameters.Container = &conf.ContainerConfig{}
	}
	parameters.Container.Enabled = containerRequested
	parameters.ContainerDisabled = containerOff
	parameters.VFSAllow = parseVFSAllowPaths(parameters.VFSAllow)

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

func buildRunSessionOutput(params *core.RunExecution, output stdio.Writer) core.SessionThreadOutput {
	if params == nil {
		return sessionio.NewTextSessionOutput(output)
	}
	parameters := &params.Config.GlobalConfig.Parameters
	if strings.TrimSpace(parameters.OutputFormat) == "jsonl" {
		return sessionio.NewJsonlSessionOutput(output)
	}
	return sessionio.NewTextSessionOutputWithSlug(output, parameters.Worktree)
}
func buildSummaryMessageFunc(output core.SessionThreadOutput) func(string, shared.MessageType) {
	if output == nil {
		return nil
	}
	return func(message string, messageType shared.MessageType) { output.ShowMessage(message, string(messageType)) }
}

type runSessionInput interface{ StartReadingInput() }

func buildRunStdinSessionInput(params *core.RunExecution, thread core.SessionThreadInput, input stdio.Reader) runSessionInput {
	if params == nil || thread == nil || input == nil {
		return nil
	}
	parameters := &params.Config.GlobalConfig.Parameters
	if parameters.OutputFormat == "jsonl" {
		return sessionio.NewJsonlSessionInput(input, thread)
	}
	return sessionio.NewTextSessionInput(input, thread)
}
func validateMergeRunExecution(params *core.RunExecution) error {
	if params == nil {
		return fmt.Errorf("validateMergeRunExecution() [run.go]: params cannot be nil")
	}
	parameters := &params.Config.GlobalConfig.Parameters
	if parameters.Merge && parameters.NoCommit {
		return fmt.Errorf("RunCommand() [run.go]: --merge cannot be used with --no-commit")
	}
	if parameters.Merge && strings.TrimSpace(parameters.Worktree) == "" {
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
func PreparePromptWithContext(params *core.RunExecution) error {
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
func BuildContainerStartupInfoMessage(configStore *conf.CswConfig) string {
	runtimeConfig := configStore.Runtime
	identity := runtimeConfig.ContainerIdentity
	containerImage := ""
	if configStore.GlobalConfig != nil && configStore.GlobalConfig.Parameters.Container != nil {
		containerImage = configStore.GlobalConfig.Parameters.Container.Image
	}
	containerImageInfo := parseContainerImageInfo(containerImage)
	return fmt.Sprintf("[INFO] Container: image=%s tag=%s version=%s user=%s(uid=%d) group=%s(gid=%d)", shared.NullValue(strings.TrimSpace(containerImageInfo.Name)), shared.NullValue(strings.TrimSpace(containerImageInfo.Tag)), shared.NullValue(strings.TrimSpace(containerImageInfo.Version)), shared.NullValue(strings.TrimSpace(identity.UserName)), identity.UID, shared.NullValue(strings.TrimSpace(identity.GroupName)), identity.GID)
}
