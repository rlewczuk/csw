package core

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/runner"
	uimock "github.com/rlewczuk/csw/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	require.Len(t, result.FeedbackRequests, 1)
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
	require.Len(t, result.FeedbackRequests, 2)
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
