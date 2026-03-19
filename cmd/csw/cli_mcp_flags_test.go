package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIMCPFlagsPropagation(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		expectedEnable   []string
		expectedDisable  []string
	}{
		{
			name:            "parses repeated and comma-separated values",
			args:            []string{"--mcp-enable=srv-a,srv-b", "--mcp-enable=srv-b", "--mcp-disable=srv-c", "--mcp-disable=srv-d,srv-c", "prompt"},
			expectedEnable:  []string{"srv-a", "srv-b"},
			expectedDisable: []string{"srv-c", "srv-d"},
		},
		{
			name:            "ignores empty values",
			args:            []string{"--mcp-enable=,srv-a,,", "--mcp-disable=", "--mcp-disable=,srv-b", "prompt"},
			expectedEnable:  []string{"srv-a"},
			expectedDisable: []string{"srv-b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			captured := ""
			originalRun := runCLIFunc
			t.Cleanup(func() {
				runCLIFunc = originalRun
			})

			runCLIFunc = func(params *CLIParams) error {
				captured = fmt.Sprintf("enable=%v,disable=%v", params.MCPEnable, params.MCPDisable)
				return nil
			}

			cmd := CliCommand()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			require.NoError(t, err)
			assert.Contains(t, captured, fmt.Sprintf("enable=%v", tc.expectedEnable))
			assert.Contains(t, captured, fmt.Sprintf("disable=%v", tc.expectedDisable))
		})
	}
}
