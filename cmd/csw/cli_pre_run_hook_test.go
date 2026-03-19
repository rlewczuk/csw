package main

import (
	"context"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreparePromptWithPreRunHook(t *testing.T) {
	tests := []struct {
		name          string
		params        *CLIParams
		withHook      bool
		runnerSetup   func(*runner.MockRunner)
		expectedPrompt string
		expectError   string
		expectedRuns  int
	}{
		{
			name: "pre_run hook updates context before template rendering",
			params: &CLIParams{
				Prompt:      "Hello {{.NAME}}",
				ContextData: map[string]string{},
			},
			withHook: true,
			runnerSetup: func(mockRunner *runner.MockRunner) {
				mockRunner.SetResponseDetailed("pre-run-cmd", `CSWFEEDBACK: {"fn":"context","args":{"NAME":"FromHook"},"response":"none","id":"ctx-1"}`+"\n", "", 0, nil)
			},
			expectedPrompt: "Hello FromHook",
			expectedRuns:   1,
		},
		{
			name: "without hook engine prompt still renders from cli context",
			params: &CLIParams{
				Prompt:      "Hello {{.NAME}}",
				ContextData: map[string]string{"NAME": "FromCLI"},
			},
			withHook:       false,
			expectedPrompt: "Hello FromCLI",
			expectedRuns:   0,
		},
		{
			name: "pre_run hook failure is returned",
			params: &CLIParams{
				Prompt: "Hello",
			},
			withHook: true,
			runnerSetup: func(mockRunner *runner.MockRunner) {
				mockRunner.SetResponseDetailed("pre-run-cmd", "", "failed\n", 9, nil)
			},
			expectError:  "pre_run hook execution failed",
			expectedRuns: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				hookEngine  *core.HookEngine
				mockRunner  *runner.MockRunner
			)

			if tc.withHook {
				configStore := confimpl.NewMockConfigStore()
				configStore.SetHookConfigs(map[string]*conf.HookConfig{
					"pre-run-hook": {
						Name:    "pre-run-hook",
						Hook:    "pre_run",
						Enabled: true,
						Type:    conf.HookTypeShell,
						Command: "pre-run-cmd",
						RunOn:   conf.HookRunOnHost,
					},
				})

				mockRunner = runner.NewMockRunner()
				if tc.runnerSetup != nil {
					tc.runnerSetup(mockRunner)
				}

				hookEngine = core.NewHookEngine(configStore, mockRunner, nil, nil)
			}

			err := preparePromptWithPreRunHook(context.Background(), tc.params, "/repo", hookEngine)
			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedPrompt, tc.params.Prompt)
			}

			if mockRunner != nil {
				assert.Len(t, mockRunner.GetExecutions(), tc.expectedRuns)
			}
		})
	}
}
