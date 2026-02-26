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
		name                   string
		args                   []string
		expectedEnabled        bool
		expectedImage          string
		expectedMounts         []string
		expectedEnv            []string
		expectError            bool
		expectedErrorSubstring string
	}{
		{
			name:            "container with explicit image",
			args:            []string{"--container=busybox:latest", "prompt"},
			expectedEnabled: true,
			expectedImage:   "busybox:latest",
		},
		{
			name:            "container without image uses default sentinel",
			args:            []string{"--container", "prompt"},
			expectedEnabled: true,
			expectedImage:   "",
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
			name:                   "container rejected with resume",
			args:                   []string{"--container=busybox:latest", "--resume=last", "--continue", "prompt"},
			expectError:            true,
			expectedErrorSubstring: "--container is not supported with --resume",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			captured := ""
			originalRun := runCLIFunc
			t.Cleanup(func() {
				runCLIFunc = originalRun
			})

			if tc.expectError {
				runCLIFunc = originalRun
			} else {
				runCLIFunc = func(params *CLIParams) error {
					captured = fmt.Sprintf("enabled=%t,image=%s,mounts=%v,env=%v", params.ContainerEnabled, params.ContainerImage, params.ContainerMounts, params.ContainerEnv)
					return nil
				}
			}

			cmd := CliCommand()
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
			assert.Contains(t, captured, fmt.Sprintf("enabled=%t", tc.expectedEnabled))
			assert.Contains(t, captured, fmt.Sprintf("image=%s", tc.expectedImage))
			assert.Contains(t, captured, fmt.Sprintf("mounts=%v", tc.expectedMounts))
			assert.Contains(t, captured, fmt.Sprintf("env=%v", tc.expectedEnv))
		})
	}
}
