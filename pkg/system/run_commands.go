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
	CommandRunDefaults   *conf.RunDefaultsConfig
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

func applyCommandRunDefaults(commandDefaults *conf.RunDefaultsConfig, defaults *conf.RunDefaultsConfig, containerOn *bool, containerOff *bool) (*bool, error) {
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
	if strings.TrimSpace(commandDefaults.DefaultProvider) != "" {
		defaults.DefaultProvider = strings.TrimSpace(commandDefaults.DefaultProvider)
	}
	if strings.TrimSpace(commandDefaults.Model) != "" {
		defaults.Model = strings.TrimSpace(commandDefaults.Model)
	}
	if strings.TrimSpace(commandDefaults.DefaultRole) != "" {
		defaults.Role = strings.TrimSpace(commandDefaults.DefaultRole)
	}
	if strings.TrimSpace(commandDefaults.Workdir) != "" {
		defaults.Workdir = strings.TrimSpace(commandDefaults.Workdir)
	}
	if strings.TrimSpace(commandDefaults.Worktree) != "" {
		defaults.Worktree = strings.TrimSpace(commandDefaults.Worktree)
	}
	if commandDefaults.NoCommit {
		defaults.NoCommit = true
	}
	if defaults.NoCommit {
		defaults.Worktree = ""
		defaults.Merge = false
	}
	if !defaults.NoCommit && commandDefaults.Merge {
		defaults.Merge = true
	}
	if commandDefaults.LogLLMRequests {
		defaults.LogLLMRequests = true
	}
	if strings.TrimSpace(commandDefaults.Thinking) != "" {
		defaults.Thinking = strings.TrimSpace(commandDefaults.Thinking)
	}
	if strings.TrimSpace(commandDefaults.LSPServer) != "" {
		defaults.LSPServer = strings.TrimSpace(commandDefaults.LSPServer)
	}
	if strings.TrimSpace(commandDefaults.GitUserName) != "" {
		defaults.GitUserName = strings.TrimSpace(commandDefaults.GitUserName)
	}
	if strings.TrimSpace(commandDefaults.GitUserEmail) != "" {
		defaults.GitUserEmail = strings.TrimSpace(commandDefaults.GitUserEmail)
	}
	if commandDefaults.MaxThreads != 0 {
		defaults.MaxThreads = commandDefaults.MaxThreads
	}
	if strings.TrimSpace(commandDefaults.TaskDir) != "" {
		defaults.TaskDir = strings.TrimSpace(commandDefaults.TaskDir)
	}
	if strings.TrimSpace(commandDefaults.ShadowDir) != "" {
		defaults.ShadowDir = strings.TrimSpace(commandDefaults.ShadowDir)
	}
	if commandDefaults.AllowAllPermissions {
		defaults.AllowAllPermissions = true
	}
	if len(commandDefaults.VFSAllow) > 0 {
		defaults.VFSAllow = append([]string(nil), commandDefaults.VFSAllow...)
	}
	if commandDefaults.RunBashMax != nil && *commandDefaults.RunBashMax != 0 {
		value := *commandDefaults.RunBashMax
		defaults.RunBashMax = &value
	}
	if commandDefaults.VfsReadLimit != nil && *commandDefaults.VfsReadLimit != 0 {
		value := *commandDefaults.VfsReadLimit
		defaults.VfsReadLimit = &value
	}
	var commandContainerEnabled *bool
	if commandDefaults.Container != nil {
		if strings.TrimSpace(commandDefaults.Container.Image) != "" {
			defaults.Container.Image = strings.TrimSpace(commandDefaults.Container.Image)
		}
		if len(commandDefaults.Container.Mounts) > 0 {
			defaults.Container.Mounts = append([]string(nil), commandDefaults.Container.Mounts...)
		}
		if len(commandDefaults.Container.Env) > 0 {
			defaults.Container.Env = append([]string(nil), commandDefaults.Container.Env...)
		}
		if commandDefaults.Container.Enabled {
			enabledValue := true
			commandContainerEnabled = &enabledValue
			*containerOn = true
			*containerOff = false
		}
	}
	if defaults.MaxThreads < 0 {
		return commandContainerEnabled, fmt.Errorf("applyCommandRunDefaults() [run_commands.go]: --max-threads must be >= 0")
	}
	if defaults.RunBashMax != nil && *defaults.RunBashMax < 0 {
		return commandContainerEnabled, fmt.Errorf("applyCommandRunDefaults() [run_commands.go]: --run-bash-max must be >= 0")
	}
	if defaults.VfsReadLimit != nil && *defaults.VfsReadLimit < 0 {
		return commandContainerEnabled, fmt.Errorf("applyCommandRunDefaults() [run_commands.go]: --vfs-read-limit must be >= 0")
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
