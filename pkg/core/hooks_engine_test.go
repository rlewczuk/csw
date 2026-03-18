package core

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
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

	engine := NewHookEngine(configStore, hostRunner, sandboxRunner)
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

	engine := NewHookEngine(configStore, hostRunner, nil)
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

	engine := NewHookEngine(configStore, hostRunner, nil)
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

	engine := NewHookEngine(configStore, runner.NewMockRunner(), nil)
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
