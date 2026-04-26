package system

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/spf13/cobra"
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
func BuildRunAgentStartupInfoMessages(params *RunParams, buildResult BuildSystemResult) []string {
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
		return commandContainerEnabled, fmt.Errorf("applyCommandRunDefaults() [run_commands.go]: --max-threads must be >= 0")
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

func renderCommandPrompt(params *RunParams, workDir string, shellRunner runner.CommandRunner, hostShellRunner runner.CommandRunner) error {
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
