package main

import (
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
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/vcs"
	"gopkg.in/yaml.v3"
)

type RunParams = system.RunParams

const defaultBashRunTimeout = 120 * time.Second

func executeSystemRunCommand(params *RunParams) error {
	return system.RunCommand(params)
}

func runCommand(params *RunParams) error {
	if params == nil {
		return fmt.Errorf("runCommand() [run_compat.go]: params cannot be nil")
	}
	if params.Command == nil {
		return runCommandFunc(params)
	}

	positionalArgs := append([]string(nil), params.PositionalArgs...)
	prompt := ""
	extraPositionalArgs := []string(nil)
	if len(positionalArgs) >= 1 {
		prompt = positionalArgs[0]
		extraPositionalArgs = positionalArgs[1:]
	}

	stdin := params.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	if prompt != "" && strings.HasPrefix(prompt, "@") {
		promptFile := strings.TrimPrefix(prompt, "@")
		data, err := os.ReadFile(promptFile)
		if err != nil {
			return fmt.Errorf("runCommand() [run_compat.go]: failed to read prompt file: %w", err)
		}
		prompt = string(data)
	} else if prompt == "-" {
		data, err := stdio.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("runCommand() [run_compat.go]: failed to read prompt from stdin: %w", err)
		}
		prompt = string(data)
	}
	if prompt != "" {
		prompt = strings.TrimSpace(prompt)
	}

	runInTaskMode := strings.TrimSpace(params.TaskIdentifier) != "" || params.TaskLast || params.TaskNext

	invocation, isCommandInvocation, err := commands.ParseInvocation(prompt, extraPositionalArgs)
	if err != nil {
		return fmt.Errorf("runCommand() [run_compat.go]: %w", err)
	}
	if !isCommandInvocation && len(extraPositionalArgs) > 0 {
		return fmt.Errorf("runCommand() [run_compat.go]: prompt must be a single argument unless using /command invocation")
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
			return fmt.Errorf("runCommand() [run_compat.go]: %w", loadErr)
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
		return fmt.Errorf("runCommand() [run_compat.go]: prompt cannot be empty")
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
		return fmt.Errorf("runCommand() [run_compat.go]: --container-enabled and --container-disabled cannot be used together")
	}

	var taskManager *core.TaskManager
	taskIdentifier := ""
	if runInTaskMode {
		if invocation != nil {
			prompt = "/" + strings.TrimSpace(invocation.Name)
			extraPositionalArgs = append([]string(nil), invocation.Arguments...)
		}
		if loadTaskBackendFunc == nil {
			return fmt.Errorf("runCommand() [run_compat.go]: task manager loader not configured")
		}
		manager, _, loadErr := loadTaskBackendFunc(params.Command)
		if loadErr != nil {
			return loadErr
		}
		taskManager = manager
		identifier, resolveErr := resolveTaskRunIdentifier(manager, params.TaskIdentifier, params.TaskLast, params.TaskNext)
		if resolveErr != nil {
			return resolveErr
		}
		taskIdentifier = identifier
		taskDir, resolvedTask, resolveErr := manager.ResolveTask(core.TaskLookup{Identifier: identifier})
		if resolveErr != nil {
			return resolveErr
		}
		resolvedTask.TaskDir = taskDir
		params.Task = cloneRunTask(resolvedTask)
		params.InitialTask = cloneRunTask(resolvedTask)
		if prompt == "" {
			taskPromptBytes, readErr := os.ReadFile(filepath.Join(taskDir, "task.md"))
			if readErr != nil {
				return fmt.Errorf("runCommand() [run_compat.go]: failed to read task prompt: %w", readErr)
			}
			prompt = strings.TrimSpace(string(taskPromptBytes))
		}
		if prompt == "" {
			return fmt.Errorf("runCommand() [run_compat.go]: prompt cannot be empty")
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

	params.Command = nil

	err = runCommandFunc(params)
	if err != nil {
		return err
	}

	if runInTaskMode && taskManager != nil && strings.TrimSpace(taskIdentifier) != "" {
		finalStatus := core.TaskStatusCompleted
		if params.Merge {
			finalStatus = core.TaskStatusMerged
		}
		if _, updateErr := taskManager.UpdateTask(core.TaskUpdateParams{Identifier: taskIdentifier, Status: &finalStatus}); updateErr != nil {
			return updateErr
		}
	}

	return nil
}

func resolveCommandsRootDir(workDir string, shadowDir string) (string, error) {
	if strings.TrimSpace(shadowDir) != "" {
		resolvedShadowDir, err := system.ResolveWorkDir(shadowDir)
		if err != nil {
			return "", fmt.Errorf("resolveCommandsRootDir() [run_compat.go]: failed to resolve shadow directory: %w", err)
		}
		return resolvedShadowDir, nil
	}
	resolvedWorkDir, err := system.ResolveWorkDir(workDir)
	if err != nil {
		return "", fmt.Errorf("resolveCommandsRootDir() [run_compat.go]: failed to resolve work directory: %w", err)
	}
	return resolvedWorkDir, nil
}

func renderCommandPrompt(params *RunParams, workDir string, shellRunner runner.CommandRunner, hostShellRunner runner.CommandRunner) error {
	if params == nil {
		return fmt.Errorf("renderCommandPrompt() [run_compat.go]: params is nil")
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
		return fmt.Errorf("renderCommandPrompt() [run_compat.go]: failed to render command /%s: %w", params.CommandName, err)
	}

	params.Prompt = strings.TrimSpace(expandedPrompt)
	if params.Prompt == "" {
		return fmt.Errorf("renderCommandPrompt() [run_compat.go]: rendered command /%s prompt is empty", params.CommandName)
	}

	return nil
}

func PreparePromptWithContext(params *RunParams) error {
	return system.PreparePromptWithContext(params)
}

func BuildContainerStartupInfoMessage(buildResult system.BuildSystemResult) string {
	return system.BuildContainerStartupInfoMessage(buildResult)
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

func buildSummaryMessageFunc(output core.SessionThreadOutput) func(string, shared.MessageType) {
	if output == nil {
		return nil
	}
	return func(message string, messageType shared.MessageType) {
		output.ShowMessage(message, string(messageType))
	}
}

func validateMergeRunParams(params *RunParams) error {
	if params == nil {
		return fmt.Errorf("validateMergeRunParams() [run_compat.go]: params cannot be nil")
	}
	if params.Merge && strings.TrimSpace(params.WorktreeBranch) == "" {
		return fmt.Errorf("runCommand() [run_compat.go]: --merge requires --worktree")
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
	defaults, err := resolver(system.ResolveRunDefaultsParams{WorkDir: workDir, ShadowDir: shadowDir, ProjectConfig: projectConfig, ConfigPath: configPath})
	if err != nil {
		return cliMerge
	}
	if defaults.Merge {
		return true
	}
	return cliMerge
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
		return fmt.Errorf("applyCommandTaskMetadata() [run_compat.go]: failed to read task metadata: %w", err)
	}
	var persistedTask core.Task
	if err := yaml.Unmarshal(taskBytes, &persistedTask); err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run_compat.go]: failed to parse task metadata: %w", err)
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
		return fmt.Errorf("applyCommandTaskMetadata() [run_compat.go]: failed to serialize task metadata: %w", err)
	}
	if err := os.WriteFile(taskFilePath, updatedBytes, 0o644); err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run_compat.go]: failed to persist task metadata: %w", err)
	}
	return nil
}
