package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIContainerFlagsPropagation(t *testing.T) {
	tests := []struct {
		name                     string
		args                     []string
		expectedShadowDir        string
		expectedEnabled          bool
		expectedDisabled         bool
		expectedImage            string
		expectedMounts           []string
		expectedEnv              []string
		expectError              bool
		expectedErrorSubstring   string
	}{
		{
			name:            "container image does not enable mode",
			args:            []string{"--shadow-dir=./", "--container-image=busybox:latest", "prompt"},
			expectedShadowDir: "./",
			expectedEnabled: false,
			expectedImage:   "busybox:latest",
		},
		{
			name:            "container enabled",
			args:            []string{"--container-enabled", "prompt"},
			expectedEnabled: true,
		},
		{
			name:            "container mount enables container mode",
			args:            []string{"--container-mount=/host:/container", "prompt"},
			expectedEnabled: true,
			expectedMounts:  []string{"/host:/container"},
		},
		{
			name:            "container env enables container mode",
			args:            []string{"--container-env=KEY=value", "prompt"},
			expectedEnabled: true,
			expectedEnv:     []string{"KEY=value"},
		},
		{
			name:                   "container flags are mutually exclusive",
			args:                   []string{"--container-enabled", "--container-disabled", "prompt"},
			expectError:            true,
			expectedErrorSubstring: "--container-enabled and --container-disabled cannot be used together",
		},
		{
			name:            "container disabled propagates",
			args:            []string{"--container-disabled", "prompt"},
			expectedEnabled: false,
			expectedDisabled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			captured := ""
			originalRun := runCommandFunc
			t.Cleanup(func() {
				runCommandFunc = originalRun
			})

			if tc.expectError {
				runCommandFunc = originalRun
			} else {
				runCommandFunc = func(params *RunParams) error {
					captured = fmt.Sprintf("shadow=%s,enabled=%t,disabled=%t,image=%s,mounts=%v,env=%v", params.ShadowDir, params.ContainerEnabled, params.ContainerDisabled, params.ContainerImage, params.ContainerMounts, params.ContainerEnv)
					return nil
				}
			}

			cmd := RunCommand()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrorSubstring)
				return
			}

			require.NoError(t, err)
			assert.Contains(t, captured, fmt.Sprintf("shadow=%s", tc.expectedShadowDir))
			assert.Contains(t, captured, fmt.Sprintf("enabled=%t", tc.expectedEnabled))
			assert.Contains(t, captured, fmt.Sprintf("disabled=%t", tc.expectedDisabled))
			assert.Contains(t, captured, fmt.Sprintf("image=%s", tc.expectedImage))
			assert.Contains(t, captured, fmt.Sprintf("mounts=%v", tc.expectedMounts))
			assert.Contains(t, captured, fmt.Sprintf("env=%v", tc.expectedEnv))
		})
	}
}
