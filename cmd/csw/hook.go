package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/spf13/cobra"
)

type hookRunOutput struct {
	result          *core.HookExecutionResult
	sessionContext  map[string]string
	feedbackResults []core.HookFeedbackResponse
}

type noopSessionOutputHandler struct{}

func (noopSessionOutputHandler) ShowMessage(string, string)                   {}
func (noopSessionOutputHandler) AddUserMessage(string)                        {}
func (noopSessionOutputHandler) AddAssistantMessage(string, string)           {}
func (noopSessionOutputHandler) AddToolCall(*tool.ToolCall)                   {}
func (noopSessionOutputHandler) AddToolCallResult(*tool.ToolResponse)         {}
func (noopSessionOutputHandler) RunFinished(error)                            {}
func (noopSessionOutputHandler) OnPermissionQuery(*tool.ToolPermissionsQuery) {}
func (noopSessionOutputHandler) OnRateLimitError(int)                         {}
func (noopSessionOutputHandler) ShouldRetryAfterFailure(string) bool          { return false }

// HookCommand creates commands for hook diagnostics and execution.
func HookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage and test hooks",
	}

	cmd.AddCommand(hookListCommand())
	cmd.AddCommand(hookInfoCommand())
	cmd.AddCommand(hookRunCommand())

	return cmd
}

func hookListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			hooks, err := store.GetHookConfigs()
			if err != nil {
				return fmt.Errorf("hookListCommand() [hook.go]: failed to load hook configs: %w", err)
			}

			return outputHookList(cmd.OutOrStdout(), hooks)
		},
	}
}

func hookInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info <hook-name>",
		Short: "Show detailed hook configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hookName := strings.TrimSpace(args[0])
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			hooks, err := store.GetHookConfigs()
			if err != nil {
				return fmt.Errorf("hookInfoCommand() [hook.go]: failed to load hook configs: %w", err)
			}

			hookCfg, ok := hooks[hookName]
			if !ok || hookCfg == nil {
				return fmt.Errorf("hookInfoCommand() [hook.go]: hook not found: %s", hookName)
			}

			return outputHookInfo(cmd.OutOrStdout(), hookCfg)
		},
	}
}

func hookRunCommand() *cobra.Command {
	var (
		contextPairs []string
		contextFiles []string
		runReal      bool
	)

	cmd := &cobra.Command{
		Use:   "run <hook-name>",
		Short: "Run one hook with simulated context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hookName := strings.TrimSpace(args[0])
			if hookName == "" {
				return fmt.Errorf("hookRunCommand() [hook.go]: hook name cannot be empty")
			}

			contextData, err := system.ParseRunContextEntries(contextPairs)
			if err != nil {
				return fmt.Errorf("hookRunCommand() [hook.go]: failed to parse --context values: %w", err)
			}
			if contextData == nil {
				contextData = map[string]string{}
			}
			contextFromData, err := system.ParseRunContextFromEntries(contextFiles)
			if err != nil {
				return fmt.Errorf("hookRunCommand() [hook.go]: failed to parse --context-from values: %w", err)
			}
			for key, value := range contextFromData {
				contextData[key] = value
			}

			output, err := runOneHook(context.Background(), hookName, contextData, runReal)
			if err != nil {
				return err
			}

			return outputHookRunResult(cmd.OutOrStdout(), output)
		},
	}

	cmd.Flags().StringArrayVar(&contextPairs, "context", nil, "Set context value in KEY=VAL format (repeatable)")
	cmd.Flags().StringArrayVar(&contextFiles, "context-from", nil, "Set context value from file in KEY=FILENAME format (repeatable)")
	cmd.Flags().BoolVar(&runReal, "run", false, "Run model-backed hooks against real model/providers")

	return cmd
}

func runOneHook(ctx context.Context, hookName string, contextData map[string]string, runReal bool) (*hookRunOutput, error) {
	store, err := GetCompositeConfigStore()
	if err != nil {
		return nil, err
	}

	hooks, err := store.GetHookConfigs()
	if err != nil {
		return nil, fmt.Errorf("runOneHook() [hook.go]: failed to load hook configs: %w", err)
	}

	hookCfg, ok := hooks[hookName]
	if !ok || hookCfg == nil {
		return nil, fmt.Errorf("runOneHook() [hook.go]: hook not found: %s", hookName)
	}
	if !hookCfg.Enabled {
		return nil, fmt.Errorf("runOneHook() [hook.go]: hook %q is disabled", hookName)
	}

	selectedStore := system.NewRuntimeConfigStore(store, map[string]*conf.HookConfig{hookName: hookCfg.Clone()})

	providers := map[string]models.ModelProvider{}
	var (
		session *core.SweSession
		cleanup func()
	)
	cleanup = func() {}

	if runReal {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return nil, fmt.Errorf("runOneHook() [hook.go]: failed to resolve workdir: %w", cwdErr)
		}
		sweSystem, buildResult, buildErr := system.BuildSystem(system.BuildSystemParams{WorkDir: cwd, RoleName: "developer"})
		if buildErr != nil {
			return nil, fmt.Errorf("runOneHook() [hook.go]: failed to build system: %w", buildErr)
		}
		cleanup = buildResult.Cleanup
		providers = sweSystem.ModelProviders
		session, err = sweSystem.NewSession(buildResult.ModelName, noopSessionOutputHandler{})
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("runOneHook() [hook.go]: failed to create session: %w", err)
		}
	}
	defer cleanup()

	engine := core.NewHookEngine(
		selectedStore,
		core.NewDefaultHookRunner("."),
		core.NewDefaultHookRunner("."),
		providers,
	)
	engine.MergeContext(contextData)

	if !runReal && (hookCfg.Type == conf.HookTypeLLM || hookCfg.Type == conf.HookTypeSubAgent) {
		simulated, simErr := simulateModelHookRun(engine, hookCfg)
		if simErr != nil {
			return nil, simErr
		}

		return &hookRunOutput{result: simulated, sessionContext: engine.ContextData()}, nil
	}

	result, execErr := engine.Execute(ctx, core.HookExecutionRequest{Name: strings.TrimSpace(hookCfg.Hook), Session: session})
	if execErr != nil {
		return &hookRunOutput{result: result, sessionContext: engine.ContextData()}, fmt.Errorf("runOneHook() [hook.go]: hook execution failed: %w", execErr)
	}

	return &hookRunOutput{result: result, sessionContext: engine.ContextData()}, nil
}

func simulateModelHookRun(engine *core.HookEngine, hookCfg *conf.HookConfig) (*core.HookExecutionResult, error) {
	if hookCfg == nil {
		return &core.HookExecutionResult{}, nil
	}

	prompt, err := shared.RenderTextWithContext(hookCfg.Prompt, engine.ContextData())
	if err != nil {
		return nil, fmt.Errorf("simulateModelHookRun() [hook.go]: failed to render prompt: %w", err)
	}
	if strings.TrimSpace(hookCfg.SystemPrompt) != "" {
		renderedSystem, renderErr := shared.RenderTextWithContext(hookCfg.SystemPrompt, engine.ContextData())
		if renderErr != nil {
			return nil, fmt.Errorf("simulateModelHookRun() [hook.go]: failed to render system prompt: %w", renderErr)
		}
		if hookCfg.Type == conf.HookTypeSubAgent {
			prompt = strings.TrimSpace(strings.Join([]string{renderedSystem, prompt}, "\n\n"))
		}
	}

	result := &core.HookExecutionResult{
		Config:   hookCfg.Clone(),
		Command:  prompt,
		ExitCode: 0,
	}

	toField := strings.TrimSpace(hookCfg.OutputTo)
	if toField == "" {
		toField = "result"
	}
	engine.SetContextValue(toField, "")

	return result, nil
}

func outputHookList(output io.Writer, hooks map[string]*conf.HookConfig) error {
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tHOOK\tTYPE\tDESCRIPTION\tSTATUS")
	names := make([]string, 0, len(hooks))
	for name := range hooks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cfg := hooks[name]
		if cfg == nil {
			continue
		}
		status := "disabled"
		if cfg.Enabled {
			status = "enabled"
		}
		description := strings.TrimSpace(cfg.Description)
		if description == "" {
			description = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, strings.TrimSpace(cfg.Hook), string(cfg.Type), description, status)
	}

	return nil
}

func outputHookInfo(output io.Writer, cfg *conf.HookConfig) error {
	formatted, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("outputHookInfo() [hook.go]: failed to marshal hook config: %w", err)
	}
	fmt.Fprintln(output, string(formatted))

	return nil
}

func outputHookRunResult(output io.Writer, payload *hookRunOutput) error {
	if payload == nil || payload.result == nil {
		fmt.Fprintln(output, "No hook result")
		return nil
	}
	result := payload.result

	fmt.Fprintf(output, "Hook: %s\n", hookResultName(result))
	if strings.TrimSpace(result.Command) != "" {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "rendered_prompt_or_command:")
		fmt.Fprintln(output, result.Command)
	}
	if strings.TrimSpace(result.Stdout) != "" {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "stdout:")
		fmt.Fprintln(output, result.Stdout)
	}
	if strings.TrimSpace(result.Stderr) != "" {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "stderr:")
		fmt.Fprintln(output, result.Stderr)
	}

	fmt.Fprintln(output, "")
	fmt.Fprintf(output, "exit_code: %d\n", result.ExitCode)

	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "feedback_requests:")
	if len(result.FeedbackRequests) == 0 {
		fmt.Fprintln(output, "[]")
	} else {
		formatted, err := json.MarshalIndent(result.FeedbackRequests, "", "  ")
		if err != nil {
			return fmt.Errorf("outputHookRunResult() [hook.go]: failed to marshal feedback requests: %w", err)
		}
		fmt.Fprintln(output, string(formatted))
	}

	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "feedback_responses:")
	if len(result.FeedbackResponses) == 0 {
		fmt.Fprintln(output, "[]")
	} else {
		formatted, err := json.MarshalIndent(result.FeedbackResponses, "", "  ")
		if err != nil {
			return fmt.Errorf("outputHookRunResult() [hook.go]: failed to marshal feedback responses: %w", err)
		}
		fmt.Fprintln(output, string(formatted))
	}

	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "session_context:")
	formatted, err := json.MarshalIndent(normalizeContextMap(payload.sessionContext), "", "  ")
	if err != nil {
		return fmt.Errorf("outputHookRunResult() [hook.go]: failed to marshal session context: %w", err)
	}
	fmt.Fprintln(output, string(formatted))

	return nil
}

func normalizeContextMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]string, len(values))
	for _, key := range keys {
		result[key] = values[key]
	}

	return result
}

func hookResultName(result *core.HookExecutionResult) string {
	if result == nil || result.Config == nil {
		return "-"
	}
	name := strings.TrimSpace(result.Config.Name)
	if name == "" {
		return "-"
	}

	return name
}
