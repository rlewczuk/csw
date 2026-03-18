package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/tool"
	uimock "github.com/rlewczuk/csw/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSubAgentTaskRunner struct {
	mu       sync.Mutex
	requests []tool.SubAgentTaskRequest
	result   tool.SubAgentTaskResult
	err      error
	delay    time.Duration
	onRun    func(request tool.SubAgentTaskRequest)
}

func (m *mockSubAgentTaskRunner) ExecuteSubAgentTask(_ *SweSession, request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error) {
	if m.onRun != nil {
		m.onRun(request)
	}
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	m.mu.Lock()
	m.requests = append(m.requests, request)
	m.mu.Unlock()
	if m.err != nil {
		return tool.SubAgentTaskResult{}, m.err
	}
	return m.result, nil
}

func (m *mockSubAgentTaskRunner) Requests() []tool.SubAgentTaskRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]tool.SubAgentTaskRequest, len(m.requests))
	copy(result, m.requests)
	return result
}

func TestHookEngineExecuteShell(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:    "merge-hook",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo {{.branch}} {{.hook}}",
			RunOn:   conf.HookRunOnSandbox,
			Timeout: 3 * time.Second,
		},
	})

	hostRunner := runner.NewMockRunner()
	sandboxRunner := runner.NewMockRunner()
	sandboxRunner.SetResponseDetailed("echo feature/one merge", "ok\n", "warn\n", 0, nil)
	view := uimock.NewMockAppView()

	engine := NewHookEngine(configStore, hostRunner, sandboxRunner, nil)
	engine.MergeContext(map[string]string{
		"branch":  "feature/one",
		"workdir": "/repo/work",
		"rootdir": "/repo",
		"status":  string(HookSessionStatusRunning),
	})

	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge", View: view})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "echo feature/one merge", result.Command)
	require.Len(t, sandboxRunner.GetExecutions(), 1)
	assert.Equal(t, "echo feature/one merge", sandboxRunner.GetExecutions()[0].Command)
	require.Empty(t, hostRunner.GetExecutions())
	require.Len(t, view.ShowMessageCalls, 3)
	assert.Contains(t, view.ShowMessageCalls[0].Message, "[hook:merge-hook] command")
	assert.Contains(t, view.ShowMessageCalls[1].Message, "[hook:merge-hook][stdout]")
	assert.Contains(t, view.ShowMessageCalls[2].Message, "[hook:merge-hook][stderr]")
}

func TestHookEngineExecuteShellExitCodeFailure(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:    "merge-hook",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo fail",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed("echo fail", "", "bad\n", 9, nil)

	engine := NewHookEngine(configStore, hostRunner, nil, nil)
	_, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge", View: nil})
	require.Error(t, err)
	assert.True(t, IsHookExecutionError(err))
}

func TestHookEngineSetsEnvironmentAndRestores(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:    "merge-hook",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "env-check",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed("env-check", "", "", 0, nil)

	_ = os.Setenv("CSW_BRANCH", "existing-value")
	t.Cleanup(func() {
		_ = os.Unsetenv("CSW_BRANCH")
	})

	engine := NewHookEngine(configStore, hostRunner, nil, nil)
	engine.MergeContext(map[string]string{
		"branch":  "feature/two",
		"workdir": "/repo/work",
		"rootdir": "/repo",
		"status":  string(HookSessionStatusRunning),
	})

	_, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.NoError(t, err)

	assert.Equal(t, "existing-value", os.Getenv("CSW_BRANCH"))
}

func TestHookEngineFindEnabledHookMergesByName(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	hook := &conf.HookConfig{Name: "m", Hook: "merge", Enabled: true, Type: conf.HookTypeShell, Command: "echo ok"}
	configStore.SetHookConfigs(map[string]*conf.HookConfig{"m": hook})

	engine := NewHookEngine(configStore, runner.NewMockRunner(), nil, nil)
	resolved, err := engine.FindEnabledHook("merge")
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, "m", resolved.Name)
	assert.Equal(t, "echo ok", resolved.Command)

	resolved, err = engine.FindEnabledHook("summary")
	require.NoError(t, err)
	assert.Nil(t, resolved)
}

func TestHookEngineFindEnabledHooksReturnsAllMatchingEnabled(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook-a": {
			Name:    "merge-hook-a",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo a",
		},
		"merge-hook-b": {
			Name:    "merge-hook-b",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo b",
		},
		"merge-hook-disabled": {
			Name:    "merge-hook-disabled",
			Hook:    "merge",
			Enabled: false,
			Type:    conf.HookTypeShell,
			Command: "echo disabled",
		},
		"summary-hook": {
			Name:    "summary-hook",
			Hook:    "summary",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo summary",
		},
	})

	engine := NewHookEngine(configStore, runner.NewMockRunner(), nil, nil)
	resolved, err := engine.FindEnabledHooks("merge")
	require.NoError(t, err)
	require.Len(t, resolved, 2)
	names := []string{resolved[0].Name, resolved[1].Name}
	assert.ElementsMatch(t, []string{"merge-hook-a", "merge-hook-b"}, names)
}

func TestHookEngineExecuteRunsAllHooksForSameExtensionPoint(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook-a": {
			Name:    "merge-hook-a",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo hook-a",
			RunOn:   conf.HookRunOnHost,
		},
		"merge-hook-b": {
			Name:    "merge-hook-b",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo hook-b",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed("echo hook-a", "ok-a\n", "", 0, nil)
	hostRunner.SetResponseDetailed("echo hook-b", "ok-b\n", "", 0, nil)

	engine := NewHookEngine(configStore, hostRunner, nil, nil)
	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.NoError(t, err)
	require.NotNil(t, result)

	executions := hostRunner.GetExecutions()
	require.Len(t, executions, 2)
	commands := []string{executions[0].Command, executions[1].Command}
	assert.ElementsMatch(t, []string{"echo hook-a", "echo hook-b"}, commands)
	assert.Contains(t, result.Stdout, "ok-a")
	assert.Contains(t, result.Stdout, "ok-b")
}

func TestHookExecutionErrorMessage(t *testing.T) {
	err := (&HookExecutionError{HookName: "merge-hook", ExitCode: 7}).Error()
	assert.Equal(t, fmt.Sprintf("hook %q returned non-zero exit code %d", "merge-hook", 7), err)
}

func TestHookEngineExecuteShellProcessesContextFeedback(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:    "merge-hook",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo feedback",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed("echo feedback", "CSWFEEDBACK: {\"fn\":\"context\",\"args\":{\"alpha\":\"one\",\"beta\":2},\"response\":\"none\",\"id\":\"ctx-1\"}\n", "", 0, nil)

	engine := NewHookEngine(configStore, hostRunner, nil, nil)
	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.FeedbackRequests, 2)
	require.Len(t, result.FeedbackResponses, 1)

	assert.Equal(t, "context", result.FeedbackResponses[0].Fn)
	assert.True(t, result.FeedbackResponses[0].OK)
	contextData := engine.ContextData()
	assert.Equal(t, "one", contextData["alpha"])
	assert.Equal(t, "2", contextData["beta"])
}

func TestHookEngineExecuteShellProcessesLLMFeedbackWithResponseModes(t *testing.T) {
	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	provider.SetChatResponse("test-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "one")})
	provider.SetChatResponse("test-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "two")})

	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:    "merge-hook",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo feedback-llm",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed(
		"echo feedback-llm",
		strings.Join([]string{
			"CSWFEEDBACK: {\"fn\":\"llm\",\"args\":{\"prompt\":\"first\",\"system-prompt\":\"sys\",\"model\":\"mock/test-model\"},\"response\":\"stdin\",\"id\":\"r1\"}",
			"CSWFEEDBACK: {\"fn\":\"llm\",\"args\":{\"prompt\":\"second\",\"model\":\"mock/test-model\"},\"response\":\"rerun\",\"id\":\"r2\"}",
		}, "\n")+"\n",
		"",
		0,
		nil,
	)

	engine := NewHookEngine(configStore, hostRunner, nil, map[string]models.ModelProvider{"mock": provider})
	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.FeedbackRequests, 3)
	require.Len(t, result.FeedbackResponses, 2)

	ids := []string{result.FeedbackResponses[0].ID, result.FeedbackResponses[1].ID}
	sort.Strings(ids)
	assert.Equal(t, []string{"r1", "r2"}, ids)
	for _, response := range result.FeedbackResponses {
		assert.True(t, response.OK)
		assert.Equal(t, "llm", response.Fn)
	}

	executions := hostRunner.GetExecutions()
	require.Len(t, executions, 3)
	assert.Equal(t, "echo feedback-llm", executions[0].Command)
	assert.Contains(t, executions[1].Command, "| (echo feedback-llm)")
	assert.Contains(t, executions[2].Command, "CSW_RESPONSE=")
	assert.Contains(t, executions[2].Command, "echo feedback-llm")
	assert.Equal(t, "", os.Getenv("CSW_RESPONSE"))
	require.Len(t, provider.RecordedMessages, 2)
}

func TestHookEngineExecuteLLMHookStoresResultInDefaultField(t *testing.T) {
	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	provider.SetChatResponse("test-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "hook-result")})

	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"summary-llm": {
			Name:    "summary-llm",
			Hook:    "summary",
			Enabled: true,
			Type:    conf.HookTypeLLM,
			Prompt:  "Prompt: {{.user_prompt}}",
		},
	})

	engine := NewHookEngine(configStore, nil, nil, map[string]models.ModelProvider{"mock": provider})
	engine.SetContextValue("user_prompt", "Initial request")
	engine.SetContextValue("rootdir", t.TempDir())

	session := &SweSession{providerName: "mock", model: "test-model", thinking: "medium"}
	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary", Session: session})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "hook-result", engine.ContextData()["result"])
	require.Len(t, provider.RecordedMessages, 1)
	require.Len(t, provider.RecordedMessages[0], 1)
	assert.Equal(t, models.ChatRoleUser, provider.RecordedMessages[0][0].Role)
	assert.Equal(t, "Prompt: Initial request", strings.TrimSpace(provider.RecordedMessages[0][0].GetText()))
}

func TestHookEngineExecuteLLMHookUsesConfiguredOptionsAndOutputTo(t *testing.T) {
	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	provider.SetChatResponse("test-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "custom-result")})

	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"summary-llm": {
			Name:         "summary-llm",
			Hook:         "summary",
			Enabled:      true,
			Type:         conf.HookTypeLLM,
			Prompt:       "Prompt: {{.user_prompt}}",
			SystemPrompt: "System: {{.status}}",
			Model:        "mock/test-model",
			Thinking:     "high",
			OutputTo:     "llm_out",
		},
	})

	engine := NewHookEngine(configStore, nil, nil, map[string]models.ModelProvider{"mock": provider})
	engine.MergeContext(map[string]string{"user_prompt": "Initial request", "status": "running"})
	engine.SetContextValue("rootdir", t.TempDir())

	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "custom-result", engine.ContextData()["llm_out"])
	require.Len(t, provider.RecordedMessages, 1)
	require.Len(t, provider.RecordedMessages[0], 2)
	assert.Equal(t, models.ChatRoleSystem, provider.RecordedMessages[0][0].Role)
	assert.Equal(t, "System: running", strings.TrimSpace(provider.RecordedMessages[0][0].GetText()))
	assert.Equal(t, models.ChatRoleUser, provider.RecordedMessages[0][1].Role)
	assert.Equal(t, "Prompt: Initial request", strings.TrimSpace(provider.RecordedMessages[0][1].GetText()))
}

func TestHookEngineExecuteLLMHookFailsWhenPromptMissing(t *testing.T) {
	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"summary-llm": {
			Name:    "summary-llm",
			Hook:    "summary",
			Enabled: true,
			Type:    conf.HookTypeLLM,
		},
	})

	engine := NewHookEngine(configStore, nil, nil, map[string]models.ModelProvider{"mock": provider})
	session := &SweSession{providerName: "mock", model: "test-model"}
	_, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary", Session: session})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty prompt")
}

func TestParseHookFeedbackRequestsIgnoresInvalidLines(t *testing.T) {
	requests := parseHookFeedbackRequests(
		"hello\nCSWFEEDBACK: {invalid}\nCSWFEEDBACK: {\"fn\":\"context\",\"args\":{\"x\":\"1\"}}\n",
		"CSWFEEDBACK: {\"fn\":\"\"}\n",
	)

	require.Len(t, requests, 1)
	assert.Equal(t, "context", requests[0].Fn)
	assert.Equal(t, HookFeedbackResponseNone, requests[0].Response)
	assert.Equal(t, "1", requests[0].Args["x"])
}

func TestHookEngineExecuteShellSynthesizesResponseFeedback(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:    "merge-hook",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo synthetic",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed("echo synthetic", "hello-out\n", "hello-err\n", 0, nil)

	engine := NewHookEngine(configStore, hostRunner, nil, nil)
	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.NoError(t, err)
	require.NotNil(t, result)

	response := FindHookResponseRequest(result)
	require.NotNil(t, response)
	assert.Equal(t, hookFeedbackFnResponse, response.Fn)
	assert.Equal(t, hookStatusOK, HookResponseStatus(response))
	assert.Contains(t, HookResponseArgString(response, "stdout"), "hello-out")
	assert.Contains(t, HookResponseArgString(response, "stdout"), "hello-err")
	assert.Contains(t, HookResponseArgString(response, "stdin"), "hello-out")
	assert.Contains(t, HookResponseArgString(response, "stdin"), "hello-err")
	assert.Equal(t, "hello-err", strings.TrimSpace(HookResponseArgString(response, "stderr")))
}

func TestHookEngineExecuteShellMapsOutputToAndErrorToInResponseFeedback(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:     "merge-hook",
			Hook:     "merge",
			Enabled:  true,
			Type:     conf.HookTypeShell,
			Command:  "echo synthetic",
			RunOn:    conf.HookRunOnHost,
			OutputTo: "custom_output",
			ErrorTo:  "custom_error",
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed("echo synthetic", "hello-out\n", "hello-err\n", 0, nil)

	engine := NewHookEngine(configStore, hostRunner, nil, nil)
	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.NoError(t, err)
	require.NotNil(t, result)

	response := FindHookResponseRequest(result)
	require.NotNil(t, response)
	assert.Equal(t, "hello-out", HookResponseArgString(response, "custom_output"))
	assert.Equal(t, "hello-err", HookResponseArgString(response, "custom_error"))
}

func TestHookEngineExecuteShellSetsHookDirForLocalHooks(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:    "merge-hook",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo hookdir={{.hook_dir}}",
			RunOn:   conf.HookRunOnHost,
			HookDir: "/cfg/hooks/merge-hook",
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed("echo hookdir=/cfg/hooks/merge-hook", "", "", 0, nil)

	engine := NewHookEngine(configStore, hostRunner, nil, nil)
	_, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.NoError(t, err)

	executions := hostRunner.GetExecutions()
	require.Len(t, executions, 1)
	assert.Equal(t, "echo hookdir=/cfg/hooks/merge-hook", executions[0].Command)
	assert.Equal(t, "/cfg/hooks/merge-hook", engine.ContextData()["hook_dir"])
}

func TestHookEngineExecuteShellMaterializesEmbeddedHookFiles(t *testing.T) {
	tmpRoot := t.TempDir()
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:           "merge-hook",
			Hook:           "merge",
			Enabled:        true,
			Type:           conf.HookTypeShell,
			Command:        "echo embedded={{.hook_dir}}",
			RunOn:          conf.HookRunOnHost,
			EmbeddedSource: true,
			EmbeddedFiles: map[string][]byte{
				"script.sh": []byte("#!/bin/sh\necho one\n"),
			},
		},
	})

	targetDir := filepath.Join(tmpRoot, ".cswdata", "hooks", "merge-hook")
	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed("echo embedded="+targetDir, "", "", 0, nil)

	engine := NewHookEngine(configStore, hostRunner, nil, nil)
	engine.MergeContext(map[string]string{"rootdir": tmpRoot})

	_, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.NoError(t, err)

	content, readErr := os.ReadFile(filepath.Join(targetDir, "script.sh"))
	require.NoError(t, readErr)
	assert.Equal(t, "#!/bin/sh\necho one\n", string(content))
	assert.Equal(t, targetDir, engine.ContextData()["hook_dir"])

	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:           "merge-hook",
			Hook:           "merge",
			Enabled:        true,
			Type:           conf.HookTypeShell,
			Command:        "echo embedded={{.hook_dir}}",
			RunOn:          conf.HookRunOnHost,
			EmbeddedSource: true,
			EmbeddedFiles: map[string][]byte{
				"script.sh": []byte("#!/bin/sh\necho two\n"),
			},
		},
	})

	_, err = engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.NoError(t, err)

	updated, readUpdatedErr := os.ReadFile(filepath.Join(targetDir, "script.sh"))
	require.NoError(t, readUpdatedErr)
	assert.Equal(t, "#!/bin/sh\necho two\n", string(updated))
}

func TestHookEngineExecuteShellPreservesExplicitResponseFeedback(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:    "merge-hook",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo explicit",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed("echo explicit", "CSWFEEDBACK: {\"fn\":\"response\",\"args\":{\"status\":\"COMMITED\",\"commit-message\":\"msg\"}}\n", "ignored\n", 0, nil)

	engine := NewHookEngine(configStore, hostRunner, nil, nil)
	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.NoError(t, err)
	require.NotNil(t, result)

	requests := result.FeedbackRequests
	require.Len(t, requests, 1)
	assert.Equal(t, hookFeedbackFnResponse, requests[0].Fn)
	assert.Equal(t, "COMMITED", HookResponseStatus(&requests[0]))
	assert.Equal(t, "msg", HookResponseArgString(&requests[0], "commit-message"))
}

func TestHookEngineExecuteShellSynthesizedResponseContainsErrorStatusAndCode(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-hook": {
			Name:    "merge-hook",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo failing",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed("echo failing", "", "bad\n", 12, nil)

	engine := NewHookEngine(configStore, hostRunner, nil, nil)
	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "merge"})
	require.Error(t, err)
	require.NotNil(t, result)

	response := FindHookResponseRequest(result)
	require.NotNil(t, response)
	assert.Equal(t, hookStatusError, HookResponseStatus(response))
	code, exists := response.Args["code"]
	require.True(t, exists)
	assert.EqualValues(t, 12, code)
}

func TestHookEngineExecuteSubAgentHookUsesConfiguredAndInheritedFields(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"summary-subagent": {
			Name:         "summary-subagent",
			Hook:         "summary",
			Enabled:      true,
			Type:         conf.HookTypeSubAgent,
			Prompt:       "Prompt: {{.user_prompt}}",
			SystemPrompt: "System: {{.status}}",
			Model:        "mock/child-model",
			Thinking:     "high",
			Role:         "reviewer",
			OutputTo:     "sub_out",
		},
	})

	runner := &mockSubAgentTaskRunner{result: tool.SubAgentTaskResult{Status: "completed", Summary: "child summary"}}
	session := &SweSession{subAgentRunner: runner, providerName: "mock", model: "parent-model", thinking: "medium", role: &conf.AgentRoleConfig{Name: "developer"}}

	engine := NewHookEngine(configStore, nil, nil, nil)
	engine.MergeContext(map[string]string{"user_prompt": "Initial request", "status": "running"})

	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary", Session: session})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "child summary", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "child summary", engine.ContextData()["sub_out"])

	requests := runner.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "reviewer", requests[0].Role)
	assert.Equal(t, "mock/child-model", requests[0].Model)
	assert.Equal(t, "high", requests[0].Thinking)
	assert.Equal(t, "System: running\n\nPrompt: Initial request", requests[0].Prompt)
	require.NotNil(t, requests[0].HookFeedbackExecutor)

	response := FindHookResponseRequest(result)
	require.NotNil(t, response)
	assert.Equal(t, hookStatusOK, HookResponseStatus(response))
	assert.Equal(t, "child summary", HookResponseArgString(response, "stdout"))
	assert.Equal(t, "child summary", HookResponseArgString(response, "sub_out"))
}

func TestHookEngineExecuteSubAgentHookUsesCustomResponseFeedback(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"summary-subagent": {
			Name:     "summary-subagent",
			Hook:     "summary",
			Enabled:  true,
			Type:     conf.HookTypeSubAgent,
			Prompt:   "Prompt",
			OutputTo: "sub_out",
		},
	})

	runner := &mockSubAgentTaskRunner{result: tool.SubAgentTaskResult{Status: "completed", Summary: "default summary"}}
	feedbackCalled := false
	feedbackOK := false
	runner.onRun = func(request tool.SubAgentTaskRequest) {
		if request.HookFeedbackExecutor == nil {
			return
		}
		feedbackCalled = true
		feedbackResp := request.HookFeedbackExecutor.ExecuteHookFeedback(tool.HookFeedbackRequest{
			Fn: "response",
			Args: map[string]any{
				"status":  "OK",
				"stdout":  "custom stdout",
				"sub_out": "custom context value",
			},
			ID: "custom-1",
		})
		feedbackOK = feedbackResp.OK
	}
	session := &SweSession{subAgentRunner: runner, providerName: "mock", model: "parent-model", thinking: "medium", role: &conf.AgentRoleConfig{Name: "developer"}}

	engine := NewHookEngine(configStore, nil, nil, map[string]models.ModelProvider{})
	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary", Session: session})
	require.NoError(t, err)
	require.NotNil(t, result)

	requests := runner.Requests()
	require.Len(t, requests, 1)
	require.NotNil(t, requests[0].HookFeedbackExecutor)
	assert.True(t, feedbackCalled)
	assert.True(t, feedbackOK)
	response := FindHookResponseRequest(result)
	require.NotNil(t, response)
	assert.Equal(t, "custom stdout", HookResponseArgString(response, "stdout"))
	assert.Equal(t, "custom context value", engine.ContextData()["sub_out"])
}

func TestHookEngineExecuteSubAgentHookFailsWithoutSession(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"summary-subagent": {
			Name:    "summary-subagent",
			Hook:    "summary",
			Enabled: true,
			Type:    conf.HookTypeSubAgent,
			Prompt:  "Prompt",
		},
	})

	engine := NewHookEngine(configStore, nil, nil, nil)
	_, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session is required")
}

func TestHookEngineExecuteSubAgentHookFailsWhenPromptMissing(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"summary-subagent": {
			Name:    "summary-subagent",
			Hook:    "summary",
			Enabled: true,
			Type:    conf.HookTypeSubAgent,
		},
	})

	session := &SweSession{subAgentRunner: &mockSubAgentTaskRunner{}}
	engine := NewHookEngine(configStore, nil, nil, nil)
	_, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary", Session: session})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty prompt")
}

func TestHookEngineExecuteSubAgentHookUsesSessionDefaults(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"summary-subagent": {
			Name:    "summary-subagent",
			Hook:    "summary",
			Enabled: true,
			Type:    conf.HookTypeSubAgent,
			Prompt:  "Prompt: {{.user_prompt}}",
		},
	})

	runner := &mockSubAgentTaskRunner{result: tool.SubAgentTaskResult{Status: "completed", Summary: "ok"}}
	session := &SweSession{subAgentRunner: runner, providerName: "mock", model: "parent-model", thinking: "medium", role: &conf.AgentRoleConfig{Name: "developer"}}

	engine := NewHookEngine(configStore, nil, nil, nil)
	engine.MergeContext(map[string]string{"user_prompt": "Initial request"})

	result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary", Session: session})
	require.NoError(t, err)
	require.NotNil(t, result)

	requests := runner.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "developer", requests[0].Role)
	assert.Equal(t, "mock/parent-model", requests[0].Model)
	assert.Equal(t, "medium", requests[0].Thinking)
	assert.Equal(t, "Prompt: Initial request", requests[0].Prompt)
}

func TestHookEngineExecuteSubAgentHookMapsErrorAndTimeoutStatus(t *testing.T) {
	t.Run("subagent execution error is mapped to ERROR and stderr", func(t *testing.T) {
		configStore := confimpl.NewMockConfigStore()
		configStore.SetHookConfigs(map[string]*conf.HookConfig{
			"summary-subagent": {
				Name:    "summary-subagent",
				Hook:    "summary",
				Enabled: true,
				Type:    conf.HookTypeSubAgent,
				Prompt:  "Prompt",
				ErrorTo: "sub_err",
			},
		})

		runner := &mockSubAgentTaskRunner{err: fmt.Errorf("child failed")}
		session := &SweSession{subAgentRunner: runner, providerName: "mock", model: "parent-model"}

		engine := NewHookEngine(configStore, nil, nil, nil)
		result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary", Session: session})
		require.Error(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.ExitCode)
		assert.Contains(t, result.Stderr, "child failed")

		response := FindHookResponseRequest(result)
		require.NotNil(t, response)
		assert.Equal(t, hookStatusError, HookResponseStatus(response))
		assert.Contains(t, HookResponseArgString(response, "stderr"), "child failed")
		assert.Contains(t, HookResponseArgString(response, "sub_err"), "child failed")
	})

	t.Run("subagent timeout is mapped to TIMEOUT", func(t *testing.T) {
		configStore := confimpl.NewMockConfigStore()
		configStore.SetHookConfigs(map[string]*conf.HookConfig{
			"summary-subagent": {
				Name:    "summary-subagent",
				Hook:    "summary",
				Enabled: true,
				Type:    conf.HookTypeSubAgent,
				Prompt:  "Prompt",
				Timeout: 10 * time.Millisecond,
			},
		})

		runner := &mockSubAgentTaskRunner{delay: 80 * time.Millisecond, result: tool.SubAgentTaskResult{Status: "completed", Summary: "late"}}
		session := &SweSession{subAgentRunner: runner, providerName: "mock", model: "parent-model"}

		engine := NewHookEngine(configStore, nil, nil, nil)
		result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary", Session: session})
		require.Error(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 124, result.ExitCode)

		response := FindHookResponseRequest(result)
		require.NotNil(t, response)
		assert.Equal(t, hookStatusTimeout, HookResponseStatus(response))
	})

	t.Run("subagent result status ERROR is mapped to error outcome", func(t *testing.T) {
		configStore := confimpl.NewMockConfigStore()
		configStore.SetHookConfigs(map[string]*conf.HookConfig{
			"summary-subagent": {
				Name:    "summary-subagent",
				Hook:    "summary",
				Enabled: true,
				Type:    conf.HookTypeSubAgent,
				Prompt:  "Prompt",
			},
		})

		runner := &mockSubAgentTaskRunner{result: tool.SubAgentTaskResult{Status: "error", Summary: "child error details"}}
		session := &SweSession{subAgentRunner: runner, providerName: "mock", model: "parent-model"}

		engine := NewHookEngine(configStore, nil, nil, nil)
		result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary", Session: session})
		require.Error(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.ExitCode)
		assert.Equal(t, "child error details", result.Stderr)

		response := FindHookResponseRequest(result)
		require.NotNil(t, response)
		assert.Equal(t, hookStatusError, HookResponseStatus(response))
		assert.Equal(t, "child error details", HookResponseArgString(response, "stderr"))
	})

	t.Run("subagent result status TIMEOUT is mapped to timeout outcome", func(t *testing.T) {
		configStore := confimpl.NewMockConfigStore()
		configStore.SetHookConfigs(map[string]*conf.HookConfig{
			"summary-subagent": {
				Name:    "summary-subagent",
				Hook:    "summary",
				Enabled: true,
				Type:    conf.HookTypeSubAgent,
				Prompt:  "Prompt",
			},
		})

		runner := &mockSubAgentTaskRunner{result: tool.SubAgentTaskResult{Status: "timeout", Summary: "child timed out"}}
		session := &SweSession{subAgentRunner: runner, providerName: "mock", model: "parent-model"}

		engine := NewHookEngine(configStore, nil, nil, nil)
		result, err := engine.Execute(context.Background(), HookExecutionRequest{Name: "summary", Session: session})
		require.Error(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 124, result.ExitCode)

		response := FindHookResponseRequest(result)
		require.NotNil(t, response)
		assert.Equal(t, hookStatusTimeout, HookResponseStatus(response))
	})
}
