package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/tool"
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
	Name    string
	View    ui.IAppView
	VCS     interface{}
	Session *SweSession
}

// HookExecutionResult contains hook command execution details.
type HookExecutionResult struct {
	Config            *conf.HookConfig
	Command           string
	Stdout            string
	Stderr            string
	ExitCode          int
	FeedbackRequests  []HookFeedbackRequest
	FeedbackResponses []HookFeedbackResponse
}

// HookFeedbackResponseMode defines response delivery mode for feedback command.
type HookFeedbackResponseMode string

const (
	// HookFeedbackResponseNone means no response should be returned.
	HookFeedbackResponseNone HookFeedbackResponseMode = "none"
	// HookFeedbackResponseStdin means response should be passed through stdin as JSON lines.
	HookFeedbackResponseStdin HookFeedbackResponseMode = "stdin"
	// HookFeedbackResponseRerun means script should be rerun with response in CSW_RESPONSE env variable.
	HookFeedbackResponseRerun HookFeedbackResponseMode = "rerun"
)

// HookFeedbackRequest defines one feedback command emitted by script.
type HookFeedbackRequest struct {
	Fn       string                   `json:"fn"`
	Args     map[string]any           `json:"args,omitempty"`
	Response HookFeedbackResponseMode `json:"response,omitempty"`
	ID       string                   `json:"id,omitempty"`
}

// HookFeedbackResponse defines processed feedback result.
type HookFeedbackResponse struct {
	ID     string `json:"id,omitempty"`
	Fn     string `json:"fn"`
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

var cswFeedbackLinePattern = regexp.MustCompile(`(?m)^\s*CSWFEEDBACK:\s*(\{.*\})\s*$`)

const (
	hookFeedbackFnResponse = "response"
	hookStatusOK           = "OK"
	hookStatusError        = "ERROR"
	hookStatusTimeout      = "TIMEOUT"
)

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
	providers   map[string]models.ModelProvider
	mu          sync.RWMutex
}

// NewHookEngine creates a new hook execution engine.
func NewHookEngine(configStore conf.ConfigStore, hostRunner shellCommandRunner, shellRunner shellCommandRunner, providers map[string]models.ModelProvider) *HookEngine {
	engine := &HookEngine{
		configStore: configStore,
		hostRunner:  hostRunner,
		shellRunner: shellRunner,
		contextData: make(HookContext),
		providers:   providers,
	}
	engine.contextData["status"] = string(HookSessionStatusNone)
	return engine
}

// ContextData returns a copy of current hook context data.
func (e *HookEngine) ContextData() HookContext {
	e.mu.RLock()
	defer e.mu.RUnlock()

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

	e.mu.Lock()
	defer e.mu.Unlock()

	e.contextData[key] = value
}

// MergeContext merges values into cumulative hook context.
func (e *HookEngine) MergeContext(values map[string]string) {
	if e == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

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
	hookDir, err := e.resolveAndPrepareHookDir(hookConfig)
	if err != nil {
		return nil, fmt.Errorf("HookEngine.Execute() [hooks_engine.go]: failed to prepare hook directory for hook %q: %w", hookConfig.Name, err)
	}
	e.SetContextValue("hook_dir", hookDir)
	hookConfig.HookDir = hookDir

	switch hookConfig.Type {
	case conf.HookTypeShell:
		return e.executeShell(ctx, hookConfig, request)
	case conf.HookTypeLLM:
		return e.executeLLM(ctx, hookConfig, request)
	case conf.HookTypeSubAgent:
		return e.executeSubAgent(ctx, hookConfig, request)
	default:
		return nil, fmt.Errorf("HookEngine.Execute() [hooks_engine.go]: unsupported hook type %q", hookConfig.Type)
	}
}

func (e *HookEngine) executeSubAgent(ctx context.Context, hookConfig *conf.HookConfig, request HookExecutionRequest) (*HookExecutionResult, error) {
	promptTemplate := strings.TrimSpace(hookConfig.Prompt)
	if promptTemplate == "" {
		return nil, fmt.Errorf("HookEngine.executeSubAgent() [hooks_engine.go]: hook %q has empty prompt", hookConfig.Name)
	}

	if request.Session == nil {
		return nil, fmt.Errorf("HookEngine.executeSubAgent() [hooks_engine.go]: session is required for hook %q", hookConfig.Name)
	}

	prompt, err := e.renderTemplate("hook-subagent-prompt", promptTemplate)
	if err != nil {
		return nil, fmt.Errorf("HookEngine.executeSubAgent() [hooks_engine.go]: failed to render prompt for hook %q: %w", hookConfig.Name, err)
	}

	if strings.TrimSpace(hookConfig.SystemPrompt) != "" {
		systemPrompt, renderErr := e.renderTemplate("hook-subagent-system-prompt", hookConfig.SystemPrompt)
		if renderErr != nil {
			return nil, fmt.Errorf("HookEngine.executeSubAgent() [hooks_engine.go]: failed to render system prompt for hook %q: %w", hookConfig.Name, renderErr)
		}
		prompt = strings.TrimSpace(strings.Join([]string{systemPrompt, prompt}, "\n\n"))
	}

	modelRef := strings.TrimSpace(hookConfig.Model)
	if modelRef == "" {
		modelRef = request.Session.ModelWithProvider()
	}
	if strings.TrimSpace(modelRef) == "" {
		return nil, fmt.Errorf("HookEngine.executeSubAgent() [hooks_engine.go]: unable to resolve model for hook %q", hookConfig.Name)
	}

	thinking := strings.TrimSpace(hookConfig.Thinking)
	if thinking == "" {
		thinking = request.Session.ThinkingLevel()
	}

	role := strings.TrimSpace(hookConfig.Role)
	if role == "" {
		if parentRole := request.Session.Role(); parentRole != nil {
			role = strings.TrimSpace(parentRole.Name)
		}
	}

	runner := request.Session
	timeout := HookTimeoutOrDefault(hookConfig.Timeout)
	if timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = timeoutCtx
	}

	requestPayload := tool.SubAgentTaskRequest{
		Slug:     buildSubAgentHookSlug(hookConfig),
		Title:    fmt.Sprintf("Hook %s", strings.TrimSpace(hookConfig.Name)),
		Prompt:   prompt,
		Role:     role,
		Model:    modelRef,
		Thinking: thinking,
	}

	result := &HookExecutionResult{Config: hookConfig.Clone(), Command: prompt}
	responseRequest := HookFeedbackRequest{Fn: hookFeedbackFnResponse, Args: map[string]any{}}

	type runResult struct {
		value tool.SubAgentTaskResult
		err   error
	}
	resultCh := make(chan runResult, 1)
	go func() {
		value, runErr := runner.ExecuteSubAgentTask(requestPayload)
		resultCh <- runResult{value: value, err: runErr}
	}()

	select {
	case <-ctx.Done():
		result.ExitCode = 124
		result.Stderr = ctx.Err().Error()
		responseRequest.Args["status"] = hookStatusTimeout
		responseRequest.Args["stderr"] = result.Stderr
		if errorField := strings.TrimSpace(hookConfig.ErrorTo); errorField != "" {
			responseRequest.Args[errorField] = result.Stderr
		}
		result.FeedbackRequests = []HookFeedbackRequest{responseRequest}
		return result, fmt.Errorf("HookEngine.executeSubAgent() [hooks_engine.go]: hook %q timed out: %w", hookConfig.Name, ctx.Err())
	case run := <-resultCh:
		if run.err != nil {
			result.ExitCode = 1
			result.Stderr = run.err.Error()
			responseRequest.Args["status"] = hookStatusError
			responseRequest.Args["stderr"] = result.Stderr
			if errorField := strings.TrimSpace(hookConfig.ErrorTo); errorField != "" {
				responseRequest.Args[errorField] = result.Stderr
			}
			result.FeedbackRequests = []HookFeedbackRequest{responseRequest}
			return result, fmt.Errorf("HookEngine.executeSubAgent() [hooks_engine.go]: subagent execution failed for hook %q: %w", hookConfig.Name, run.err)
		}

		status := strings.ToUpper(strings.TrimSpace(run.value.Status))
		if status == "COMPLETED" {
			status = hookStatusOK
		}
		if status == "ERROR" {
			result.ExitCode = 1
		}
		if status == "" {
			status = hookStatusOK
		}

		result.Stdout = strings.TrimSpace(run.value.Summary)
		if status == hookStatusError && strings.TrimSpace(result.Stderr) == "" {
			result.Stderr = strings.TrimSpace(run.value.Summary)
			if result.Stderr == "" {
				result.Stderr = fmt.Sprintf("subagent hook %q returned error status", hookConfig.Name)
			}
		}
		if status == hookStatusTimeout && strings.TrimSpace(result.Stderr) == "" {
			result.Stderr = fmt.Sprintf("subagent hook %q returned timeout status", hookConfig.Name)
		}
		if status == hookStatusOK {
			result.ExitCode = 0
		}
		if status == hookStatusTimeout {
			result.ExitCode = 124
		}

		toField := strings.TrimSpace(hookConfig.OutputTo)
		if toField == "" {
			toField = "result"
		}
		e.SetContextValue(toField, result.Stdout)

		responseRequest.Args["status"] = status
		responseRequest.Args["stdout"] = result.Stdout
		responseRequest.Args["stdin"] = result.Stdout
		responseRequest.Args["stderr"] = result.Stderr
		if outputField := strings.TrimSpace(hookConfig.OutputTo); outputField != "" {
			responseRequest.Args[outputField] = result.Stdout
		}
		if errorField := strings.TrimSpace(hookConfig.ErrorTo); errorField != "" {
			responseRequest.Args[errorField] = result.Stderr
		}
		result.FeedbackRequests = []HookFeedbackRequest{responseRequest}

		if request.View != nil {
			request.View.ShowMessage(fmt.Sprintf("[hook:%s][subagent] model=%s", hookConfig.Name, modelRef), ui.MessageTypeInfo)
			if result.Stdout != "" {
				request.View.ShowMessage(fmt.Sprintf("[hook:%s][subagent-summary]\n%s", hookConfig.Name, result.Stdout), ui.MessageTypeInfo)
			}
			if result.Stderr != "" {
				request.View.ShowMessage(fmt.Sprintf("[hook:%s][subagent-error]\n%s", hookConfig.Name, result.Stderr), ui.MessageTypeWarning)
			}
		}

		if status == hookStatusError {
			return result, &HookExecutionError{HookName: hookConfig.Name, ExitCode: 1}
		}
		if status == hookStatusTimeout {
			return result, fmt.Errorf("HookEngine.executeSubAgent() [hooks_engine.go]: subagent timed out for hook %q", hookConfig.Name)
		}

		return result, nil
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
	applyHookEnvironment(e.ContextData())
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

	feedbackRequests := parseHookFeedbackRequests(stdout, stderr)
	feedbackRequests = ensureHookResponseFeedbackRequest(feedbackRequests, hookConfig, stdout, stderr, exitCode, runErr)
	result.FeedbackRequests = feedbackRequests
	feedbackExecutions := e.processHookFeedbackRequests(ctx, request, feedbackRequests)
	if len(feedbackExecutions) > 0 {
		result.FeedbackResponses = make([]HookFeedbackResponse, 0, len(feedbackExecutions))
		stdinResponses := make([]string, 0)
		rerunResponses := make([]string, 0)

		for _, execution := range feedbackExecutions {
			result.FeedbackResponses = append(result.FeedbackResponses, execution.Response)
			if execution.Mode == HookFeedbackResponseNone {
				continue
			}
			line, err := marshalHookFeedbackResponse(execution.Response)
			if err != nil {
				continue
			}
			switch execution.Mode {
			case HookFeedbackResponseStdin:
				stdinResponses = append(stdinResponses, line)
			case HookFeedbackResponseRerun:
				rerunResponses = append(rerunResponses, line)
			}
		}

		if len(stdinResponses) > 0 {
			stdinCommand := buildStdinReplayCommand(command, stdinResponses)
			_, _, _, _ = runnerToUse.RunCommandWithOptionsDetailed(stdinCommand, runner.CommandOptions{Timeout: timeout})
		}

		for _, line := range rerunResponses {
			rerunCommand := buildRerunCommand(command, line)
			_, _, _, _ = runnerToUse.RunCommandWithOptionsDetailed(rerunCommand, runner.CommandOptions{Timeout: timeout})
		}
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

func (e *HookEngine) executeLLM(ctx context.Context, hookConfig *conf.HookConfig, request HookExecutionRequest) (*HookExecutionResult, error) {
	promptTemplate := strings.TrimSpace(hookConfig.Prompt)
	if promptTemplate == "" {
		return nil, fmt.Errorf("HookEngine.executeLLM() [hooks_engine.go]: hook %q has empty prompt", hookConfig.Name)
	}

	prompt, err := e.renderTemplate("hook-llm-prompt", promptTemplate)
	if err != nil {
		return nil, fmt.Errorf("HookEngine.executeLLM() [hooks_engine.go]: failed to render prompt for hook %q: %w", hookConfig.Name, err)
	}

	systemPrompt := ""
	if strings.TrimSpace(hookConfig.SystemPrompt) != "" {
		systemPrompt, err = e.renderTemplate("hook-llm-system-prompt", hookConfig.SystemPrompt)
		if err != nil {
			return nil, fmt.Errorf("HookEngine.executeLLM() [hooks_engine.go]: failed to render system prompt for hook %q: %w", hookConfig.Name, err)
		}
	}

	modelRef := strings.TrimSpace(hookConfig.Model)
	if modelRef == "" && request.Session != nil {
		modelRef = request.Session.ModelWithProvider()
	}
	if strings.TrimSpace(modelRef) == "" {
		return nil, fmt.Errorf("HookEngine.executeLLM() [hooks_engine.go]: unable to resolve model for hook %q", hookConfig.Name)
	}

	providerName, modelName, err := parseFeedbackModelRef(modelRef)
	if err != nil {
		return nil, fmt.Errorf("HookEngine.executeLLM() [hooks_engine.go]: failed to parse model for hook %q: %w", hookConfig.Name, err)
	}

	provider, ok := e.providers[providerName]
	if !ok || provider == nil {
		return nil, fmt.Errorf("HookEngine.executeLLM() [hooks_engine.go]: provider not found for hook %q: %s", hookConfig.Name, providerName)
	}

	thinking := strings.TrimSpace(hookConfig.Thinking)
	if thinking == "" && request.Session != nil {
		thinking = request.Session.ThinkingLevel()
	}

	messages := make([]*models.ChatMessage, 0, 2)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, models.NewTextMessage(models.ChatRoleSystem, systemPrompt))
	}
	messages = append(messages, models.NewTextMessage(models.ChatRoleUser, prompt))

	chatCtx := ctx
	var cancel context.CancelFunc
	timeout := HookTimeoutOrDefault(hookConfig.Timeout)
	if timeout > 0 {
		chatCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	chatModel := provider.ChatModel(modelName, nil)
	chatResponse, err := chatModel.Chat(chatCtx, messages, &models.ChatOptions{Thinking: thinking}, nil)
	if err != nil {
		return nil, fmt.Errorf("HookEngine.executeLLM() [hooks_engine.go]: llm request failed for hook %q: %w", hookConfig.Name, err)
	}

	responseText := ""
	if chatResponse != nil {
		responseText = strings.TrimSpace(chatResponse.GetText())
	}

	toField := strings.TrimSpace(hookConfig.OutputTo)
	if toField == "" {
		toField = "result"
	}
	e.SetContextValue(toField, responseText)

	if request.View != nil {
		request.View.ShowMessage(fmt.Sprintf("[hook:%s][llm] model=%s", hookConfig.Name, providerName+"/"+modelName), ui.MessageTypeInfo)
		if responseText != "" {
			request.View.ShowMessage(fmt.Sprintf("[hook:%s][llm-response]\n%s", hookConfig.Name, responseText), ui.MessageTypeInfo)
		}
	}

	return &HookExecutionResult{
		Config:   hookConfig.Clone(),
		Command:  prompt,
		Stdout:   responseText,
		ExitCode: 0,
	}, nil
}

func parseHookFeedbackRequests(stdout string, stderr string) []HookFeedbackRequest {
	matches := cswFeedbackLinePattern.FindAllStringSubmatch(strings.Join([]string{stdout, stderr}, "\n"), -1)
	if len(matches) == 0 {
		return nil
	}

	result := make([]HookFeedbackRequest, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		payload := strings.TrimSpace(match[1])
		if payload == "" {
			continue
		}
		request := HookFeedbackRequest{}
		if err := json.Unmarshal([]byte(payload), &request); err != nil {
			continue
		}
		request.Fn = strings.TrimSpace(request.Fn)
		if request.Fn == "" {
			continue
		}
		request.ID = strings.TrimSpace(request.ID)
		request.Response = normalizeFeedbackResponseMode(request.Response)
		if request.Args == nil {
			request.Args = map[string]any{}
		}
		result = append(result, request)
	}

	return result
}

func normalizeFeedbackResponseMode(value HookFeedbackResponseMode) HookFeedbackResponseMode {
	switch HookFeedbackResponseMode(strings.ToLower(strings.TrimSpace(string(value)))) {
	case HookFeedbackResponseStdin:
		return HookFeedbackResponseStdin
	case HookFeedbackResponseRerun:
		return HookFeedbackResponseRerun
	default:
		return HookFeedbackResponseNone
	}
}

type hookFeedbackExecution struct {
	Mode     HookFeedbackResponseMode
	Response HookFeedbackResponse
}

func (e *HookEngine) processHookFeedbackRequests(ctx context.Context, hookRequest HookExecutionRequest, requests []HookFeedbackRequest) []hookFeedbackExecution {
	if len(requests) == 0 {
		return nil
	}

	processable := make([]HookFeedbackRequest, 0, len(requests))
	for _, request := range requests {
		if strings.EqualFold(strings.TrimSpace(request.Fn), hookFeedbackFnResponse) {
			continue
		}
		processable = append(processable, request)
	}
	if len(processable) == 0 {
		return nil
	}

	results := make(chan hookFeedbackExecution, len(processable))
	var waitGroup sync.WaitGroup

	for _, current := range processable {
		request := current
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			response := e.handleHookFeedbackRequest(ctx, hookRequest, request)
			results <- hookFeedbackExecution{Mode: request.Response, Response: response}
		}()
	}

	waitGroup.Wait()
	close(results)

	collected := make([]hookFeedbackExecution, 0, len(processable))
	for result := range results {
		collected = append(collected, result)
	}

	return collected
}

func (e *HookEngine) handleHookFeedbackRequest(ctx context.Context, hookRequest HookExecutionRequest, request HookFeedbackRequest) HookFeedbackResponse {
	response := HookFeedbackResponse{ID: request.ID, Fn: request.Fn, OK: false}

	switch request.Fn {
	case "context":
		updated, err := e.handleHookFeedbackContext(request.Args)
		if err != nil {
			response.Error = err.Error()
			return response
		}
		response.OK = true
		response.Result = updated
	case "llm":
		result, err := e.handleHookFeedbackLLM(ctx, hookRequest, request.Args)
		if err != nil {
			response.Error = err.Error()
			return response
		}
		response.OK = true
		response.Result = result
	default:
		response.Error = fmt.Sprintf("HookEngine.handleHookFeedbackRequest() [hooks_engine.go]: unsupported feedback function %q", request.Fn)
	}

	return response
}

func (e *HookEngine) handleHookFeedbackContext(args map[string]any) (map[string]string, error) {
	updated := make(map[string]string)
	for key, value := range args {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		updated[trimmedKey] = hookFeedbackValueToString(value)
	}

	e.MergeContext(updated)
	return updated, nil
}

func (e *HookEngine) handleHookFeedbackLLM(ctx context.Context, hookRequest HookExecutionRequest, args map[string]any) (map[string]any, error) {
	prompt := strings.TrimSpace(hookFeedbackArgString(args, "prompt"))
	if prompt == "" {
		return nil, fmt.Errorf("HookEngine.handleHookFeedbackLLM() [hooks_engine.go]: missing args.prompt")
	}

	modelRef := strings.TrimSpace(hookFeedbackArgString(args, "model"))
	if modelRef == "" && hookRequest.Session != nil {
		modelRef = hookRequest.Session.ModelWithProvider()
	}
	if strings.TrimSpace(modelRef) == "" {
		return nil, fmt.Errorf("HookEngine.handleHookFeedbackLLM() [hooks_engine.go]: unable to resolve model")
	}

	providerName, modelName, err := parseFeedbackModelRef(modelRef)
	if err != nil {
		return nil, err
	}

	provider, ok := e.providers[providerName]
	if !ok || provider == nil {
		return nil, fmt.Errorf("HookEngine.handleHookFeedbackLLM() [hooks_engine.go]: provider not found: %s", providerName)
	}

	thinking := strings.TrimSpace(hookFeedbackArgString(args, "thinking"))
	if thinking == "" && hookRequest.Session != nil {
		thinking = hookRequest.Session.ThinkingLevel()
	}

	messages := make([]*models.ChatMessage, 0, 2)
	systemPrompt := strings.TrimSpace(hookFeedbackArgString(args, "system-prompt"))
	if systemPrompt != "" {
		messages = append(messages, models.NewTextMessage(models.ChatRoleSystem, systemPrompt))
	}
	messages = append(messages, models.NewTextMessage(models.ChatRoleUser, prompt))

	chatModel := provider.ChatModel(modelName, nil)
	chatResponse, err := chatModel.Chat(ctx, messages, &models.ChatOptions{Thinking: thinking}, nil)
	if err != nil {
		return nil, fmt.Errorf("HookEngine.handleHookFeedbackLLM() [hooks_engine.go]: llm request failed: %w", err)
	}

	result := map[string]any{
		"model":    providerName + "/" + modelName,
		"thinking": strings.TrimSpace(thinking),
		"text":     "",
	}
	if chatResponse != nil {
		result["text"] = strings.TrimSpace(chatResponse.GetText())
	}

	return result, nil
}

func hookFeedbackArgString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, exists := args[key]
	if !exists {
		return ""
	}
	return hookFeedbackValueToString(value)
}

func hookFeedbackValueToString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func ensureHookResponseFeedbackRequest(requests []HookFeedbackRequest, hookConfig *conf.HookConfig, stdout string, stderr string, exitCode int, runErr error) []HookFeedbackRequest {
	for _, request := range requests {
		if strings.EqualFold(strings.TrimSpace(request.Fn), hookFeedbackFnResponse) {
			return requests
		}
	}

	combinedOutput := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(stdout), strings.TrimSpace(stderr)}, "\n"))
	args := map[string]any{
		"stdin":  combinedOutput,
		"stdout": combinedOutput,
		"stderr": stderr,
	}
	if hookConfig != nil {
		if outputField := strings.TrimSpace(hookConfig.OutputTo); outputField != "" {
			args[outputField] = stdout
		}
		if errorField := strings.TrimSpace(hookConfig.ErrorTo); errorField != "" {
			args[errorField] = stderr
		}
	}
	if runErr != nil && isHookTimeoutError(runErr, exitCode) {
		args["status"] = hookStatusTimeout
	} else if exitCode == 0 {
		args["status"] = hookStatusOK
	} else {
		args["status"] = hookStatusError
		args["code"] = exitCode
	}

	return append(requests, HookFeedbackRequest{Fn: hookFeedbackFnResponse, Args: args})
}

func (e *HookEngine) resolveAndPrepareHookDir(hookConfig *conf.HookConfig) (string, error) {
	if hookConfig == nil {
		return "", nil
	}

	configuredDir := strings.TrimSpace(hookConfig.HookDir)
	if !hookConfig.EmbeddedSource {
		return configuredDir, nil
	}

	rootDir := strings.TrimSpace(e.ContextData()["rootdir"])
	if rootDir == "" {
		rootDir = strings.TrimSpace(e.ContextData()["workdir"])
	}
	if rootDir == "" {
		return "", fmt.Errorf("HookEngine.resolveAndPrepareHookDir() [hooks_engine.go]: rootdir/workdir hook context is empty")
	}

	hookName := strings.TrimSpace(hookConfig.Name)
	if hookName == "" {
		return "", fmt.Errorf("HookEngine.resolveAndPrepareHookDir() [hooks_engine.go]: hook name is empty")
	}

	targetDir := filepath.Join(rootDir, ".cswdata", "hooks", hookName)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("HookEngine.resolveAndPrepareHookDir() [hooks_engine.go]: failed to create %s: %w", targetDir, err)
	}

	for filename, data := range hookConfig.EmbeddedFiles {
		trimmedName := strings.TrimSpace(filename)
		if trimmedName == "" {
			continue
		}
		if filepath.Base(trimmedName) != trimmedName {
			return "", fmt.Errorf("HookEngine.resolveAndPrepareHookDir() [hooks_engine.go]: invalid embedded hook file name %q", filename)
		}

		outputPath := filepath.Join(targetDir, trimmedName)
		existing, readErr := os.ReadFile(outputPath)
		if readErr == nil && bytes.Equal(existing, data) {
			continue
		}
		if readErr != nil && !os.IsNotExist(readErr) {
			return "", fmt.Errorf("HookEngine.resolveAndPrepareHookDir() [hooks_engine.go]: failed to read %s: %w", outputPath, readErr)
		}

		if writeErr := os.WriteFile(outputPath, data, hookEmbeddedFileMode(trimmedName)); writeErr != nil {
			return "", fmt.Errorf("HookEngine.resolveAndPrepareHookDir() [hooks_engine.go]: failed to write %s: %w", outputPath, writeErr)
		}
	}

	return targetDir, nil
}

func hookEmbeddedFileMode(filename string) os.FileMode {
	if strings.EqualFold(filepath.Ext(filename), ".sh") {
		return 0o755
	}
	return 0o644
}

func isHookTimeoutError(err error, exitCode int) bool {
	if err == nil {
		return false
	}
	if exitCode == 124 {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "timed out")
}

// FindHookResponseRequest returns the first hook feedback request with fn="response".
func FindHookResponseRequest(result *HookExecutionResult) *HookFeedbackRequest {
	if result == nil {
		return nil
	}

	for _, request := range result.FeedbackRequests {
		if !strings.EqualFold(strings.TrimSpace(request.Fn), hookFeedbackFnResponse) {
			continue
		}
		copied := request
		if copied.Args == nil {
			copied.Args = map[string]any{}
		}
		return &copied
	}

	return nil
}

// HookResponseStatus returns normalized hook response status. Missing status means OK.
func HookResponseStatus(request *HookFeedbackRequest) string {
	if request == nil || request.Args == nil {
		return hookStatusOK
	}

	rawStatus, exists := request.Args["status"]
	if !exists {
		return hookStatusOK
	}
	status := strings.ToUpper(strings.TrimSpace(hookFeedbackValueToString(rawStatus)))
	if status == "" {
		return hookStatusOK
	}

	return status
}

// HookResponseArgString returns one response arg as string.
func HookResponseArgString(request *HookFeedbackRequest, key string) string {
	if request == nil || request.Args == nil {
		return ""
	}
	return strings.TrimSpace(hookFeedbackArgString(request.Args, key))
}

func marshalHookFeedbackResponse(response HookFeedbackResponse) (string, error) {
	encoded, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("marshalHookFeedbackResponse() [hooks_engine.go]: failed to marshal response: %w", err)
	}
	return string(encoded), nil
}

func buildStdinReplayCommand(command string, lines []string) string {
	if len(lines) == 0 {
		return command
	}

	quotedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		quotedLines = append(quotedLines, singleQuoteShellValue(line))
	}

	return fmt.Sprintf("printf '%%s\\n' %s | (%s)", strings.Join(quotedLines, " "), command)
}

func singleQuoteShellValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func buildRerunCommand(command string, responseLine string) string {
	return fmt.Sprintf("CSW_RESPONSE=%s %s", singleQuoteShellValue(responseLine), command)
}

func parseFeedbackModelRef(model string) (string, string, error) {
	trimmed := strings.TrimSpace(model)
	for index, char := range trimmed {
		if char != '/' {
			continue
		}
		provider := strings.TrimSpace(trimmed[:index])
		modelName := strings.TrimSpace(trimmed[index+1:])
		if provider == "" || modelName == "" {
			break
		}
		return provider, modelName, nil
	}

	return "", "", fmt.Errorf("parseFeedbackModelRef() [hooks_engine.go]: invalid model format, expected provider/model: %q", model)
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
	return e.renderTemplate("hook-command", commandTemplate)
}

func (e *HookEngine) renderTemplate(templateName string, templateText string) (string, error) {
	tmpl, err := template.New(templateName).Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("HookEngine.renderTemplate() [hooks_engine.go]: failed to parse template: %w", err)
	}

	contextData := e.ContextData()
	data := make(map[string]string, len(contextData))
	for key, value := range contextData {
		data[key] = value
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("HookEngine.renderTemplate() [hooks_engine.go]: failed to execute template: %w", err)
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

func buildSubAgentHookSlug(hookConfig *conf.HookConfig) string {
	base := "hook"
	if hookConfig != nil {
		name := strings.TrimSpace(hookConfig.Name)
		if name == "" {
			name = strings.TrimSpace(hookConfig.Hook)
		}
		if name != "" {
			name = strings.ToLower(name)
			name = strings.ReplaceAll(name, "_", "-")
			name = strings.ReplaceAll(name, " ", "-")
			name = strings.ReplaceAll(name, "/", "-")
			name = strings.ReplaceAll(name, ".", "-")
			name = strings.Trim(name, "-")
			if name != "" {
				base = name
			}
		}
	}

	return fmt.Sprintf("hook-%s-%d", base, time.Now().UnixNano())
}
