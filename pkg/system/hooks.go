package system

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/vcs"
)

func HandleCommitHookResponse(ctx context.Context, hookEngine *core.HookEngine, gitVcs apis.VCS, worktreeBranch string, repoDir string, worktreeDir string, session *core.SweSession, appView ui.IAppView) (string, bool, error) {
	if hookEngine == nil {
		return "", false, nil
	}

	hookEngine.MergeContext(map[string]string{
		"branch":  strings.TrimSpace(worktreeBranch),
		"workdir": strings.TrimSpace(firstNonEmpty(worktreeDir, repoDir)),
		"rootdir": strings.TrimSpace(repoDir),
	})
	hookResult, hookErr := hookEngine.Execute(ctx, core.HookExecutionRequest{Name: "commit", View: appView, VCS: gitVcs, Session: session})
	if hookResult == nil {
		if hookErr != nil {
			return "", false, fmt.Errorf("handleCommitHookResponse() [cli.go]: commit hook execution failed: %w", hookErr)
		}
		return "", false, nil
	}

	response := core.FindHookResponseRequest(hookResult)
	if response == nil {
		if hookErr != nil {
			return "", false, fmt.Errorf("handleCommitHookResponse() [cli.go]: commit hook execution failed: %w", hookErr)
		}
		return "", false, nil
	}

	status := strings.ToUpper(strings.TrimSpace(core.HookResponseStatus(response)))
	if status == "" {
		status = "OK"
	}

	switch status {
	case "TIMEOUT", "ERROR":
		return "", false, fmt.Errorf("handleCommitHookResponse() [cli.go]: commit hook returned status %s", status)
	case "COMMITED":
		resetDir := strings.TrimSpace(firstNonEmpty(worktreeDir, repoDir))
		if err := vcs.HardResetWorktree(resetDir); err != nil {
			return "", false, err
		}
		return "", true, nil
	default:
		return core.HookResponseArgString(response, "commit-message"), false, nil
	}
}

func ApplyHookDefaults(target *conf.HookConfig) {
	if target == nil {
		return
	}
	if target.Type == "" {
		target.Type = conf.HookTypeShell
	}
	if target.RunOn == "" {
		target.RunOn = conf.HookRunOnSandbox
	}
	if target.Type == conf.HookTypeLLM && strings.TrimSpace(target.OutputTo) == "" {
		target.OutputTo = "result"
	}
}

type RuntimeHookConfigStore struct {
	base           conf.ConfigStore
	hookConfigs    map[string]*conf.HookConfig
	hookConfigsNow time.Time
}

func NewRuntimeConfigStore(base conf.ConfigStore, hookConfigs map[string]*conf.HookConfig) *RuntimeHookConfigStore {
	return &RuntimeHookConfigStore{
		base:           base,
		hookConfigs:    hookConfigs,
		hookConfigsNow: time.Now(),
	}
}

func (s *RuntimeHookConfigStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	return s.base.GetModelProviderConfigs()
}

func (s *RuntimeHookConfigStore) GetModelAliases() (map[string]conf.ModelAliasValue, error) {
	return s.base.GetModelAliases()
}

func (s *RuntimeHookConfigStore) LastModelAliasesUpdate() (time.Time, error) {
	return s.base.LastModelAliasesUpdate()
}

func (s *RuntimeHookConfigStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	return s.base.LastModelProviderConfigsUpdate()
}

func (s *RuntimeHookConfigStore) GetAgentRoleConfigs() (map[string]*conf.AgentRoleConfig, error) {
	return s.base.GetAgentRoleConfigs()
}

func (s *RuntimeHookConfigStore) LastAgentRoleConfigsUpdate() (time.Time, error) {
	return s.base.LastAgentRoleConfigsUpdate()
}

func (s *RuntimeHookConfigStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	return s.base.GetGlobalConfig()
}

func (s *RuntimeHookConfigStore) LastGlobalConfigUpdate() (time.Time, error) {
	return s.base.LastGlobalConfigUpdate()
}

func (s *RuntimeHookConfigStore) GetMCPServerConfigs() (map[string]*conf.MCPServerConfig, error) {
	return s.base.GetMCPServerConfigs()
}

func (s *RuntimeHookConfigStore) LastMCPServerConfigsUpdate() (time.Time, error) {
	return s.base.LastMCPServerConfigsUpdate()
}

func (s *RuntimeHookConfigStore) GetHookConfigs() (map[string]*conf.HookConfig, error) {
	cloned := make(map[string]*conf.HookConfig, len(s.hookConfigs))
	for key, value := range s.hookConfigs {
		cloned[key] = value.Clone()
	}

	return cloned, nil
}

func (s *RuntimeHookConfigStore) LastHookConfigsUpdate() (time.Time, error) {
	return s.hookConfigsNow, nil
}

func (s *RuntimeHookConfigStore) GetAgentConfigFile(subdir, filename string) ([]byte, error) {
	return s.base.GetAgentConfigFile(subdir, filename)
}

func BuildRuntimeHookConfigStore(base conf.ConfigStore, overrides []string) (conf.ConfigStore, error) {
	if base == nil {
		return nil, fmt.Errorf("BuildRuntimeHookConfigStore() [cli.go]: base config store is nil")
	}

	if len(overrides) == 0 {
		return base, nil
	}

	configs, err := base.GetHookConfigs()
	if err != nil {
		return nil, fmt.Errorf("BuildRuntimeHookConfigStore() [cli.go]: failed to load hook configs: %w", err)
	}

	adjusted, err := ApplyHookOverridesToConfigs(configs, overrides)
	if err != nil {
		return nil, err
	}

	return &RuntimeHookConfigStore{
		base:           base,
		hookConfigs:    adjusted,
		hookConfigsNow: time.Now(),
	}, nil
}

type HookOverride struct {
	Name     string
	Disable  bool
	Settings map[string]string
}

func ApplyHookOverridesToConfigs(configs map[string]*conf.HookConfig, overrides []string) (map[string]*conf.HookConfig, error) {
	cloned := make(map[string]*conf.HookConfig, len(configs))
	for key, value := range configs {
		cloned[key] = value.Clone()
	}

	for _, rawOverride := range overrides {
		override, err := ParseHookOverride(rawOverride)
		if err != nil {
			return nil, err
		}

		current, exists := cloned[override.Name]
		if !exists {
			if len(override.Settings) == 0 || override.Disable {
				return nil, fmt.Errorf("ApplyHookOverridesToConfigs() [cli.go]: hook %q is not configured", override.Name)
			}

			created, createErr := BuildNewHookConfig(override.Name, override.Settings)
			if createErr != nil {
				return nil, createErr
			}
			cloned[override.Name] = created
			continue
		}

		wasDisabled := !current.Enabled
		if override.Disable {
			current.Enabled = false
			continue
		}

		if len(override.Settings) == 0 {
			if !current.Enabled {
				current.Enabled = true
			}
			ApplyHookDefaults(current)
			continue
		}

		if err := ApplyHookSettings(current, override.Name, override.Settings); err != nil {
			return nil, err
		}
		if wasDisabled {
			current.Enabled = true
		}
		ApplyHookDefaults(current)
	}

	return cloned, nil
}

func ParseHookOverride(value string) (*HookOverride, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, ":")
	if trimmed == "" {
		return nil, fmt.Errorf("ParseHookOverride() [cli.go]: hook override cannot be empty")
	}

	parts := strings.SplitN(trimmed, ":", 2)
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return nil, fmt.Errorf("ParseHookOverride() [cli.go]: hook name is required")
	}

	if len(parts) == 1 {
		return &HookOverride{Name: name}, nil
	}

	action := strings.TrimSpace(parts[1])
	if action == "" {
		return nil, fmt.Errorf("ParseHookOverride() [cli.go]: hook action for %q is empty", name)
	}

	if strings.EqualFold(action, "disable") {
		return &HookOverride{Name: name, Disable: true}, nil
	}

	settings := make(map[string]string)
	entries := strings.Split(action, ",")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		keyValue := strings.SplitN(entry, "=", 2)
		if len(keyValue) != 2 {
			return nil, fmt.Errorf("ParseHookOverride() [cli.go]: invalid hook setting %q for %q", entry, name)
		}

		key := strings.TrimSpace(keyValue[0])
		val := strings.TrimSpace(keyValue[1])
		if key == "" {
			return nil, fmt.Errorf("ParseHookOverride() [cli.go]: empty setting key in %q", entry)
		}
		settings[key] = val
	}

	if len(settings) == 0 {
		return nil, fmt.Errorf("ParseHookOverride() [cli.go]: no hook settings provided for %q", name)
	}

	return &HookOverride{Name: name, Settings: settings}, nil
}

func BuildNewHookConfig(name string, settings map[string]string) (*conf.HookConfig, error) {
	created := &conf.HookConfig{Name: name, Enabled: true}
	if err := ApplyHookSettings(created, name, settings); err != nil {
		return nil, err
	}
	if strings.TrimSpace(created.Hook) == "" {
		return nil, fmt.Errorf("BuildNewHookConfig() [cli.go]: hook %q requires setting \"hook\"", name)
	}
	if created.Type == conf.HookTypeLLM {
		if strings.TrimSpace(created.Prompt) == "" {
			return nil, fmt.Errorf("BuildNewHookConfig() [cli.go]: hook %q requires setting \"prompt\"", name)
		}
	} else {
		if strings.TrimSpace(created.Command) == "" {
			return nil, fmt.Errorf("BuildNewHookConfig() [cli.go]: hook %q requires setting \"command\"", name)
		}
	}
	ApplyHookDefaults(created)

	return created, nil
}

func ApplyHookSettings(target *conf.HookConfig, name string, settings map[string]string) error {
	if target == nil {
		return fmt.Errorf("ApplyHookSettings() [cli.go]: target hook is nil")
	}

	for key, value := range settings {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "enabled":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("ApplyHookSettings() [cli.go]: invalid enabled value for hook %q: %w", name, err)
			}
			target.Enabled = parsed
		case "hook":
			target.Hook = strings.TrimSpace(value)
		case "name":
			if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != name {
				return fmt.Errorf("ApplyHookSettings() [cli.go]: name override must match hook selector %q", name)
			}
			target.Name = name
		case "type":
			hookType := conf.HookType(strings.TrimSpace(value))
			switch hookType {
			case conf.HookTypeShell, conf.HookTypeLLM, conf.HookTypeSubAgent:
				target.Type = hookType
			default:
				return fmt.Errorf("ApplyHookSettings() [cli.go]: unsupported hook type %q for %q", value, name)
			}
		case "command":
			target.Command = value
		case "description":
			target.Description = value
		case "prompt":
			target.Prompt = value
		case "system_prompt", "system-prompt":
			target.SystemPrompt = value
		case "model":
			target.Model = value
		case "thinking":
			target.Thinking = value
		case "output_to", "output-to", "outputto", "to_field", "to-field", "tofield":
			target.OutputTo = value
		case "error_to", "error-to", "errorto":
			target.ErrorTo = value
		case "timeout":
			parsed, err := ParseHookTimeout(value)
			if err != nil {
				return fmt.Errorf("ApplyHookSettings() [cli.go]: invalid timeout for hook %q: %w", name, err)
			}
			target.Timeout = parsed
		case "run-on", "runon":
			runOn := conf.HookRunOn(strings.TrimSpace(value))
			switch runOn {
			case conf.HookRunOnHost, conf.HookRunOnSandbox:
				target.RunOn = runOn
			default:
				return fmt.Errorf("ApplyHookSettings() [cli.go]: unsupported run-on value %q for %q", value, name)
			}
		default:
			return fmt.Errorf("ApplyHookSettings() [cli.go]: unsupported hook setting %q for %q", key, name)
		}
	}

	if strings.TrimSpace(target.Name) == "" {
		target.Name = name
	}

	return nil
}

func ParseHookTimeout(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	if _, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		trimmed += "s"
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, err
	}
	if parsed < 0 {
		return 0, fmt.Errorf("duration must not be negative")
	}

	return parsed, nil
}

func NormalizeResumeTarget(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}

	if strings.EqualFold(value, "last") {
		return "last", nil
	}

	if ResumeUUIDPattern.MatchString(value) {
		return strings.ToLower(value), nil
	}

	return value, nil
}
