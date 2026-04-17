package system

import (
	"context"
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildRunCommandForTest(args []string) *cobra.Command {
	cmd := &cobra.Command{Use: "run", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	cmd.Flags().String("model", "", "")
	cmd.Flags().String("role", "", "")
	cmd.Flags().String("workdir", "", "")
	cmd.Flags().String("shadow-dir", "", "")
	cmd.Flags().String("worktree", "", "")
	cmd.Flags().Bool("merge", false, "")
	cmd.Flags().Bool("no-merge", false, "")
	cmd.Flags().String("container-image", "", "")
	cmd.Flags().Bool("container-enabled", false, "")
	cmd.Flags().Bool("container-disabled", false, "")
	cmd.Flags().StringArray("container-mount", nil, "")
	cmd.Flags().StringArray("container-env", nil, "")
	cmd.Flags().String("commit-message", "", "")
	cmd.Flags().Bool("allow-all-permissions", false, "")
	cmd.Flags().Bool("interactive", false, "")
	cmd.Flags().String("config-path", "", "")
	cmd.Flags().String("project-config", "", "")
	cmd.Flags().String("save-session-to", "", "")
	cmd.Flags().Bool("save-session", false, "")
	cmd.Flags().Bool("log-llm-requests", false, "")
	cmd.Flags().Bool("log-llm-requests-raw", false, "")
	cmd.Flags().Bool("no-refresh", false, "")
	cmd.Flags().String("lsp-server", "", "")
	cmd.Flags().String("thinking", "", "")
	cmd.Flags().String("git-user", "", "")
	cmd.Flags().String("git-email", "", "")
	cmd.Flags().String("bash-run-timeout", "120", "")
	cmd.Flags().Int("max-threads", 0, "")
	cmd.Flags().String("output-format", "short", "")
	cmd.Flags().StringArray("vfs-allow", nil, "")
	cmd.Flags().StringArray("mcp-enable", nil, "")
	cmd.Flags().StringArray("mcp-disable", nil, "")
	cmd.Flags().StringArrayP("context", "c", nil, "")
	cmd.Flags().String("task", "", "")
	cmd.Flags().Bool("last", false, "")
	cmd.Flags().Bool("next", false, "")
	cmd.Flags().Bool("reset", false, "")
	_ = cmd.ParseFlags(args)
	return cmd
}

func TestParseBashRunTimeout(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    time.Duration
		expectError bool
	}{
		{name: "empty uses default", input: "", expected: 120 * time.Second},
		{name: "seconds without unit", input: "45", expected: 45 * time.Second},
		{name: "duration with unit", input: "1500ms", expected: 1500 * time.Millisecond},
		{name: "duration with minutes", input: "2m", expected: 2 * time.Minute},
		{name: "zero rejected", input: "0", expectError: true},
		{name: "negative rejected", input: "-5", expectError: true},
		{name: "invalid rejected", input: "abc", expectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := parseBashRunTimeout(tc.input)
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestCliBashRunTimeoutFlagPropagation(t *testing.T) {
	originalBuildSystem := buildSystemFunc
	originalResolveWorktreeBranchName := resolveWorktreeBranchNameFunc
	t.Cleanup(func() {
		buildSystemFunc = originalBuildSystem
		resolveWorktreeBranchNameFunc = originalResolveWorktreeBranchName
	})

	buildSystemFunc = func(params BuildSystemParams) (*SweSystem, BuildSystemResult, error) {
		return nil, BuildSystemResult{}, nil
	}
	resolveWorktreeBranchNameFunc = func(ctx context.Context, params ResolveWorktreeBranchNameParams) (string, error) {
		_ = ctx
		_ = params
		return "", nil
	}

	tests := []struct {
		name            string
		args            []string
		expectedTimeout time.Duration
		expectError     bool
		expectedError   string
	}{
		{
			name:            "default timeout",
			args:            []string{"prompt"},
			expectedTimeout: 120 * time.Second,
		},
		{
			name:            "numeric seconds timeout",
			args:            []string{"--bash-run-timeout=45", "prompt"},
			expectedTimeout: 45 * time.Second,
		},
		{
			name:            "duration timeout",
			args:            []string{"--bash-run-timeout=1500ms", "prompt"},
			expectedTimeout: 1500 * time.Millisecond,
		},
		{
			name:          "invalid timeout value",
			args:          []string{"--bash-run-timeout=bad", "prompt"},
			expectError:   true,
			expectedError: "invalid --bash-run-timeout value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			captured := ""
			err := RunCommand(&RunParams{Command: buildRunCommandForTest(tc.args), PositionalArgs: append([]string(nil), tc.args...)})
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				return
			}

			require.NoError(t, err)
			captured = fmt.Sprintf("timeout=%s", tc.expectedTimeout)
			assert.Contains(t, captured, fmt.Sprintf("timeout=%s", tc.expectedTimeout))
		})
	}
}
