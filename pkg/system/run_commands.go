package system

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/shared"
)

type resolvedRunCommandInvocation struct {
	CommandTemplate      string
	CommandName          string
	CommandPath          string
	CommandArgs          []string
	CommandModelOverride string
	CommandRoleOverride  string
	CommandRunDefaults   *commands.RunDefaultsMetadata
	CommandTaskMetadata  *commands.TaskMetadata
	CommandNeedsShell    bool
	Prompt               string
	ExtraPositionalArgs  []string
}

func resolveRunCommandInvocation(invocation *commands.Invocation, workDir string, shadowDir string, runInTaskMode bool) (*resolvedRunCommandInvocation, error) {
	if invocation == nil {
		return nil, fmt.Errorf("resolveRunCommandInvocation() [run_commands.go]: invocation cannot be nil")
	}

	commandsRoot, rootErr := resolveCommandsRootDir(workDir, shadowDir)
	if rootErr != nil {
		return nil, rootErr
	}
	loadedCommand, loadErr := commands.LoadFromDir(filepath.Join(commandsRoot, ".agents", "commands"), invocation.Name)
	if loadErr != nil {
		return nil, loadErr
	}

	result := &resolvedRunCommandInvocation{
		CommandTemplate:      loadedCommand.Template,
		CommandName:          loadedCommand.Name,
		CommandPath:          loadedCommand.Path,
		CommandArgs:          append([]string(nil), invocation.Arguments...),
		CommandModelOverride: strings.TrimSpace(loadedCommand.Metadata.Model),
		CommandRoleOverride:  strings.TrimSpace(loadedCommand.Metadata.Agent),
		CommandNeedsShell:    commands.HasDefaultRuntimeShellExpansion(loadedCommand.Template),
		Prompt:               loadedCommand.Template,
	}
	if loadedCommand.Metadata.CSW != nil {
		result.CommandRunDefaults = loadedCommand.Metadata.CSW.Defaults
		result.CommandTaskMetadata = loadedCommand.Metadata.CSW.Task
	}
	if runInTaskMode {
		result.Prompt = "/" + strings.TrimSpace(invocation.Name)
		result.ExtraPositionalArgs = append([]string(nil), invocation.Arguments...)
	}

	return result, nil
}

// BuildRunAgentStartupInfoMessages builds startup info lines for model, thinking, role and command.
func BuildRunAgentStartupInfoMessages(params *runExecution, buildResult BuildSystemResult) []string {
	if params == nil {
		return nil
	}

	messages := make([]string, 0, 4)
	messages = append(messages, fmt.Sprintf("[INFO] Model: %s", shared.NullValue(strings.TrimSpace(buildResult.ModelName))))
	messages = append(messages, fmt.Sprintf("[INFO] Thinking: %s", shared.NullValue(strings.TrimSpace(params.Thinking))))

	roleName := strings.TrimSpace(buildResult.RoleConfig.Name)
	if roleName == "" {
		roleName = strings.TrimSpace(params.RoleName)
	}
	messages = append(messages, fmt.Sprintf("[INFO] Role: %s", shared.NullValue(roleName)))

	commandLine := BuildRunCommandStartupInfoMessage(params.CommandName, params.CommandPath)
	if commandLine != "" {
		messages = append(messages, commandLine)
	}

	return messages
}

// BuildRunCommandStartupInfoMessage builds startup info line for detected slash command.
func BuildRunCommandStartupInfoMessage(commandName string, commandPath string) string {
	trimmedCommandName := strings.TrimSpace(commandName)
	if trimmedCommandName == "" {
		return ""
	}

	commandSource := shared.NullValue(resolveRunCommandSource(commandPath))
	return fmt.Sprintf("[INFO] Command: /%s source=%s", trimmedCommandName, commandSource)
}

func resolveRunCommandSource(commandPath string) string {
	trimmedPath := strings.TrimSpace(commandPath)
	if trimmedPath == "" {
		return ""
	}
	if strings.HasPrefix(trimmedPath, "embedded:") {
		return "embedded"
	}

	normalizedPath := filepath.ToSlash(trimmedPath)
	if strings.Contains(normalizedPath, "/.agents/commands/") {
		return ".agents/commands"
	}

	return "custom"
}

func applyCommandRunDefaults(commandDefaults *commands.RunDefaultsMetadata, defaults *conf.RunDefaultsConfig, containerOn *bool, containerOff *bool) (*bool, error) {
	if commandDefaults == nil {
		return nil, nil
	}
	if defaults == nil {
		return nil, fmt.Errorf("applyCommandRunDefaults() [run_commands.go]: defaults cannot be nil")
	}
	if containerOn == nil || containerOff == nil {
		return nil, fmt.Errorf("applyCommandRunDefaults() [run_commands.go]: container flags cannot be nil")
	}
	if defaults.Container == nil {
		defaults.Container = &conf.ContainerConfig{}
	}
	if commandDefaults.Model != nil {
		defaults.Model = strings.TrimSpace(*commandDefaults.Model)
	}
	if commandDefaults.DefaultRole != nil {
		defaults.Role = strings.TrimSpace(*commandDefaults.DefaultRole)
	}
	if commandDefaults.Worktree != nil {
		defaults.Worktree = strings.TrimSpace(*commandDefaults.Worktree)
	}
	if commandDefaults.NoCommit != nil {
		defaults.NoCommit = *commandDefaults.NoCommit
	}
	if defaults.NoCommit {
		defaults.Worktree = ""
		defaults.Merge = false
	}
	if !defaults.NoCommit && commandDefaults.Merge != nil {
		defaults.Merge = *commandDefaults.Merge
	}
	if commandDefaults.LogLLMRequests != nil {
		defaults.LogLLMRequests = *commandDefaults.LogLLMRequests
	}
	if commandDefaults.Thinking != nil {
		defaults.Thinking = strings.TrimSpace(*commandDefaults.Thinking)
	}
	if commandDefaults.LSPServer != nil {
		defaults.LSPServer = strings.TrimSpace(*commandDefaults.LSPServer)
	}
	if commandDefaults.GitUserName != nil {
		defaults.GitUserName = strings.TrimSpace(*commandDefaults.GitUserName)
	}
	if commandDefaults.GitUserEmail != nil {
		defaults.GitUserEmail = strings.TrimSpace(*commandDefaults.GitUserEmail)
	}
	if commandDefaults.MaxThreads != nil {
		defaults.MaxThreads = *commandDefaults.MaxThreads
	}
	if commandDefaults.ShadowDir != nil {
		defaults.ShadowDir = strings.TrimSpace(*commandDefaults.ShadowDir)
	}
	if commandDefaults.AllowAllPermissions != nil {
		defaults.AllowAllPermissions = *commandDefaults.AllowAllPermissions
	}
	if commandDefaults.VFSAllow != nil {
		defaults.VFSAllow = append([]string(nil), *commandDefaults.VFSAllow...)
	}
	if commandDefaults.RunBashMax != nil {
		value := *commandDefaults.RunBashMax
		defaults.RunBashMax = &value
	}
	var commandContainerEnabled *bool
	if commandDefaults.Container != nil {
		if commandDefaults.Container.Image != nil {
			defaults.Container.Image = strings.TrimSpace(*commandDefaults.Container.Image)
		}
		if commandDefaults.Container.Mounts != nil {
			defaults.Container.Mounts = append([]string(nil), *commandDefaults.Container.Mounts...)
		}
		if commandDefaults.Container.Env != nil {
			defaults.Container.Env = append([]string(nil), *commandDefaults.Container.Env...)
		}
		if commandDefaults.Container.Enabled != nil {
			enabledValue := *commandDefaults.Container.Enabled
			commandContainerEnabled = &enabledValue
			*containerOn = enabledValue
			*containerOff = !enabledValue
		}
	}
	if defaults.MaxThreads < 0 {
		return commandContainerEnabled, fmt.Errorf("applyCommandRunDefaults() [run_commands.go]: --max-threads must be >= 0")
	}
	if defaults.RunBashMax != nil && *defaults.RunBashMax < 0 {
		return commandContainerEnabled, fmt.Errorf("applyCommandRunDefaults() [run_commands.go]: --run-bash-max must be >= 0")
	}
	return commandContainerEnabled, nil
}

func resolveCommandsRootDir(workDir string, shadowDir string) (string, error) {
	if strings.TrimSpace(shadowDir) != "" {
		resolvedShadowDir, err := ResolveWorkDir(shadowDir)
		if err != nil {
			return "", fmt.Errorf("resolveCommandsRootDir() [run_commands.go]: failed to resolve shadow directory: %w", err)
		}
		return resolvedShadowDir, nil
	}
	resolvedWorkDir, err := ResolveWorkDir(workDir)
	if err != nil {
		return "", fmt.Errorf("resolveCommandsRootDir() [run_commands.go]: failed to resolve work directory: %w", err)
	}
	return resolvedWorkDir, nil
}

func renderCommandPrompt(params *runExecution, workDir string, shellRunner runner.CommandRunner, hostShellRunner runner.CommandRunner) error {
	if params == nil {
		return fmt.Errorf("renderCommandPrompt() [run_commands.go]: params is nil")
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
		return fmt.Errorf("renderCommandPrompt() [run_commands.go]: failed to render command /%s: %w", params.CommandName, err)
	}
	params.Prompt = strings.TrimSpace(expandedPrompt)
	if params.Prompt == "" {
		return fmt.Errorf("renderCommandPrompt() [run_commands.go]: rendered command /%s prompt is empty", params.CommandName)
	}
	return nil
}
