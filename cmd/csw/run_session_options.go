package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/spf13/cobra"
)

type runDefaultsResolver func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error)

func applyRunDefaults(
	resolver runDefaultsResolver,
	cmd *cobra.Command,
	workDir string,
	shadowDir string,
	projectConfig string,
	configPath string,
	model *string,
	worktree *string,
	merge *bool,
	logLLMRequests *bool,
	thinking *string,
	lspServer *string,
	gitUser *string,
	gitEmail *string,
	maxThreads *int,
	shadowDirOut *string,
	allowAllPerms *bool,
	vfsAllow *[]string,
) error {
	defaults, err := resolver(system.ResolveRunDefaultsParams{
		WorkDir:       workDir,
		ShadowDir:     shadowDir,
		ProjectConfig: projectConfig,
		ConfigPath:    configPath,
	})
	if err != nil {
		return fmt.Errorf("applyRunDefaults() [run_session_options.go]: failed to resolve run defaults: %w", err)
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
		return fmt.Errorf("applyRunDefaults() [run_session_options.go]: --max-threads must be >= 0")
	}

	return nil
}

func applyCommandRunDefaults(
	cmd *cobra.Command,
	defaults *commands.RunDefaultsMetadata,
	model *string,
	role *string,
	worktree *string,
	merge *bool,
	logLLMRequests *bool,
	thinking *string,
	lspServer *string,
	gitUser *string,
	gitEmail *string,
	maxThreads *int,
	shadowDirOut *string,
	allowAllPerms *bool,
	vfsAllow *[]string,
	containerOn *bool,
	containerOff *bool,
	containerImage *string,
	containerMounts *[]string,
	containerEnv *[]string,
) (*bool, error) {
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
		return commandContainerEnabled, fmt.Errorf("applyCommandRunDefaults() [run_session_options.go]: --max-threads must be >= 0")
	}

	return commandContainerEnabled, nil
}

func isThinkingFlagChanged(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}

	return cmd.Flags().Changed("thinking")
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
		return 0, fmt.Errorf("parseBashRunTimeout() [run_session_options.go]: invalid --bash-run-timeout value %q: %w", value, err)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("parseBashRunTimeout() [run_session_options.go]: --bash-run-timeout must be positive, got %q", value)
	}

	return parsed, nil
}

// parseVFSAllowPaths parses the --vfs-allow flag values.
// It handles both repeated flags and colon-separated values.
func parseVFSAllowPaths(values []string) []string {
	var result []string
	for _, v := range values {
		parts := strings.Split(v, ":")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

// parseMCPServerFlagValues parses repeated --mcp-enable/--mcp-disable values.
// It accepts repeated flags and comma-separated names in each value.
func parseMCPServerFlagValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		parts := strings.Split(value, ",")
		for _, part := range parts {
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
