package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/ui"
)

// HookSessionStatus describes lifecycle status exposed to hook context.
type HookSessionStatus string

const (
	// HookSessionStatusNone indicates no run status is known yet.
	HookSessionStatusNone HookSessionStatus = "none"
	// HookSessionStatusRunning indicates session is currently running.
	HookSessionStatusRunning HookSessionStatus = "running"
	// HookSessionStatusSuccess indicates session finished successfully.
	HookSessionStatusSuccess HookSessionStatus = "success"
	// HookSessionStatusFailed indicates session finished with error.
	HookSessionStatusFailed HookSessionStatus = "failed"
)

// HookContext stores cumulative hook context values for one session.
type HookContext map[string]string

// HookExecutionRequest defines one hook execution.
type HookExecutionRequest struct {
	Name string
	View ui.IAppView
	VCS  interface{}
}

// HookExecutionResult contains hook command execution details.
type HookExecutionResult struct {
	Config   *conf.HookConfig
	Command  string
	Stdout   string
	Stderr   string
	ExitCode int
}

// HookExecutionError represents non-zero hook exit code.
type HookExecutionError struct {
	HookName string
	ExitCode int
}

// Error returns error message for hook execution failure.
func (e *HookExecutionError) Error() string {
	return fmt.Sprintf("hook %q returned non-zero exit code %d", e.HookName, e.ExitCode)
}

type shellCommandRunner interface {
	RunCommandWithOptionsDetailed(command string, options runner.CommandOptions) (stdout string, stderr string, exitCode int, err error)
}

// HookEngine executes configured hooks against cumulative session context.
type HookEngine struct {
	configStore conf.ConfigStore
	hostRunner  shellCommandRunner
	shellRunner shellCommandRunner
	contextData HookContext
}

// NewHookEngine creates a new hook execution engine.
func NewHookEngine(configStore conf.ConfigStore, hostRunner shellCommandRunner, shellRunner shellCommandRunner) *HookEngine {
	engine := &HookEngine{
		configStore: configStore,
		hostRunner:  hostRunner,
		shellRunner: shellRunner,
		contextData: make(HookContext),
	}
	engine.contextData["status"] = string(HookSessionStatusNone)
	return engine
}

// ContextData returns a copy of current hook context data.
func (e *HookEngine) ContextData() HookContext {
	result := make(HookContext, len(e.contextData))
	for key, value := range e.contextData {
		result[key] = value
	}
	return result
}

// SetContextValue sets one context field.
func (e *HookEngine) SetContextValue(key string, value string) {
	if e == nil || strings.TrimSpace(key) == "" {
		return
	}
	e.contextData[key] = value
}

// MergeContext merges values into cumulative hook context.
func (e *HookEngine) MergeContext(values map[string]string) {
	if e == nil {
		return
	}
	for key, value := range values {
		if strings.TrimSpace(key) == "" {
			continue
		}
		e.contextData[key] = value
	}
}

// SetSessionStatus updates status field in hook context.
func (e *HookEngine) SetSessionStatus(status HookSessionStatus) {
	e.SetContextValue("status", string(status))
}

// FindEnabledHook returns enabled hook for the given extension point.
func (e *HookEngine) FindEnabledHook(hookName string) (*conf.HookConfig, error) {
	if e == nil || e.configStore == nil {
		return nil, nil
	}

	hooks, err := e.configStore.GetHookConfigs()
	if err != nil {
		return nil, fmt.Errorf("HookEngine.FindEnabledHook() [hooks_engine.go]: failed to load hook configs: %w", err)
	}

	for _, cfg := range hooks {
		if cfg == nil {
			continue
		}
		if !cfg.Enabled {
			continue
		}
		if strings.TrimSpace(cfg.Hook) != hookName {
			continue
		}
		return cfg.Clone(), nil
	}

	return nil, nil
}

// Execute runs one hook by extension point name when configured and enabled.
func (e *HookEngine) Execute(ctx context.Context, request HookExecutionRequest) (*HookExecutionResult, error) {
	if e == nil || e.configStore == nil {
		return nil, nil
	}

	hookName := strings.TrimSpace(request.Name)
	if hookName == "" {
		return nil, fmt.Errorf("HookEngine.Execute() [hooks_engine.go]: hook name is empty")
	}

	hookConfig, err := e.FindEnabledHook(hookName)
	if err != nil {
		return nil, err
	}
	if hookConfig == nil {
		return nil, nil
	}

	e.SetContextValue("hook", hookName)

	switch hookConfig.Type {
	case conf.HookTypeShell:
		return e.executeShell(ctx, hookConfig, request)
	case conf.HookTypeLLM, conf.HookTypeSubAgent:
		return nil, fmt.Errorf("HookEngine.Execute() [hooks_engine.go]: hook type %q is not implemented", hookConfig.Type)
	default:
		return nil, fmt.Errorf("HookEngine.Execute() [hooks_engine.go]: unsupported hook type %q", hookConfig.Type)
	}
}

func (e *HookEngine) executeShell(ctx context.Context, hookConfig *conf.HookConfig, request HookExecutionRequest) (*HookExecutionResult, error) {
	if strings.TrimSpace(hookConfig.Command) == "" {
		return nil, fmt.Errorf("HookEngine.executeShell() [hooks_engine.go]: hook %q has empty command", hookConfig.Name)
	}

	command, err := e.renderCommand(hookConfig.Command)
	if err != nil {
		return nil, fmt.Errorf("HookEngine.executeShell() [hooks_engine.go]: failed to render command for hook %q: %w", hookConfig.Name, err)
	}

	runnerToUse := e.selectRunner(hookConfig.RunOn)
	if runnerToUse == nil {
		return nil, fmt.Errorf("HookEngine.executeShell() [hooks_engine.go]: no shell runner available for hook %q", hookConfig.Name)
	}

	originalEnv := snapshotEnvironment()
	applyHookEnvironment(e.contextData)
	defer restoreEnvironment(originalEnv)

	timeout := hookConfig.Timeout
	if timeout < 0 {
		timeout = 0
	}

	stdout, stderr, exitCode, runErr := runnerToUse.RunCommandWithOptionsDetailed(command, runner.CommandOptions{Timeout: timeout})

	result := &HookExecutionResult{
		Config:   hookConfig.Clone(),
		Command:  command,
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}

	e.showHookOutput(request.View, hookConfig, command, stdout, stderr)

	if runErr != nil {
		return result, fmt.Errorf("HookEngine.executeShell() [hooks_engine.go]: hook %q execution failed: %w", hookConfig.Name, runErr)
	}
	if exitCode != 0 {
		return result, &HookExecutionError{HookName: hookConfig.Name, ExitCode: exitCode}
	}

	return result, nil
}

func (e *HookEngine) selectRunner(runOn conf.HookRunOn) shellCommandRunner {
	switch runOn {
	case conf.HookRunOnHost:
		if e.hostRunner != nil {
			return e.hostRunner
		}
		return e.shellRunner
	case conf.HookRunOnSandbox:
		if e.shellRunner != nil {
			return e.shellRunner
		}
		return e.hostRunner
	default:
		if e.shellRunner != nil {
			return e.shellRunner
		}
		return e.hostRunner
	}
}

func (e *HookEngine) renderCommand(commandTemplate string) (string, error) {
	tmpl, err := template.New("hook-command").Parse(commandTemplate)
	if err != nil {
		return "", fmt.Errorf("HookEngine.renderCommand() [hooks_engine.go]: failed to parse command template: %w", err)
	}

	data := make(map[string]string, len(e.contextData))
	for key, value := range e.contextData {
		data[key] = value
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("HookEngine.renderCommand() [hooks_engine.go]: failed to execute command template: %w", err)
	}

	return strings.TrimSpace(buf.String()), nil
}

func (e *HookEngine) showHookOutput(view ui.IAppView, hookConfig *conf.HookConfig, command string, stdout string, stderr string) {
	if view == nil || hookConfig == nil {
		return
	}

	view.ShowMessage(fmt.Sprintf("[hook:%s] command: %s", hookConfig.Name, command), ui.MessageTypeInfo)
	if strings.TrimSpace(stdout) != "" {
		view.ShowMessage(fmt.Sprintf("[hook:%s][stdout]\n%s", hookConfig.Name, strings.TrimRight(stdout, "\n")), ui.MessageTypeInfo)
	}
	if strings.TrimSpace(stderr) != "" {
		view.ShowMessage(fmt.Sprintf("[hook:%s][stderr]\n%s", hookConfig.Name, strings.TrimRight(stderr, "\n")), ui.MessageTypeWarning)
	}
}

func applyHookEnvironment(contextData HookContext) {
	keys := make([]string, 0, len(contextData))
	for key := range contextData {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		envKey := "CSW_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		_ = os.Setenv(envKey, contextData[key])
	}
}

func snapshotEnvironment() map[string]string {
	env := os.Environ()
	result := make(map[string]string, len(env))
	for _, item := range env {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[parts[0]] = parts[1]
	}
	return result
}

func restoreEnvironment(snapshot map[string]string) {
	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.HasPrefix(parts[0], "CSW_") {
			_ = os.Unsetenv(parts[0])
		}
	}

	for key, value := range snapshot {
		if strings.HasPrefix(key, "CSW_") {
			_ = os.Setenv(key, value)
		}
	}
}

// IsHookExecutionError reports whether err indicates non-zero hook exit code.
func IsHookExecutionError(err error) bool {
	var hookErr *HookExecutionError
	return errors.As(err, &hookErr)
}

// NewDefaultHookRunner creates default host shell runner.
func NewDefaultHookRunner(workDir string) shellCommandRunner {
	return runner.NewBashRunner(workDir, 0)
}

// HookTimeoutOrDefault normalizes hook timeout configuration.
func HookTimeoutOrDefault(value time.Duration) time.Duration {
	if value < 0 {
		return 0
	}
	return value
}
