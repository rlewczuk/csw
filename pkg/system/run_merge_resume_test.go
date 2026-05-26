package system

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMergeCLIParams(t *testing.T) {
	tests := []struct {
		name           string
		params         *RunParams
		expectError    bool
		errorSubstring string
	}{
		{
			name: "merge without worktree is rejected",
			params: &RunParams{
				Merge: true,
			},
			expectError:    true,
			errorSubstring: "--merge requires --worktree",
		},
		{
			name: "merge with no commit is rejected",
			params: &RunParams{
				Merge:    true,
				NoCommit: true,
			},
			expectError:    true,
			errorSubstring: "--merge cannot be used with --no-commit",
		},
		{
			name: "no commit without worktree is allowed",
			params: &RunParams{
				NoCommit: true,
			},
		},
		{
			name: "merge with worktree is allowed",
			params: &RunParams{
				Merge:          true,
				WorktreeBranch: "feature/test",
			},
		},
		{
			name: "non merge without worktree is allowed",
			params: &RunParams{
				Merge: false,
			},
		},
		{
			name:           "nil params are rejected",
			params:         nil,
			expectError:    true,
			errorSubstring: "params cannot be nil",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMergeRunParams(tc.params)
			if tc.expectError {
				require.Error(t, err)
				if tc.errorSubstring != "" {
					assert.Contains(t, err.Error(), tc.errorSubstring)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestResolveTaskRunMerge(t *testing.T) {
	tests := []struct {
		name             string
		mergeFlagChanged bool
		cliMerge         bool
		cliWorktree      string
		defaultsMerge    bool
		resolverErr      bool
		expectedMerge    bool
	}{
		{
			name:             "explicit merge flag true has priority",
			mergeFlagChanged: true,
			cliMerge:         true,
			defaultsMerge:    false,
			expectedMerge:    true,
		},
		{
			name:             "explicit merge flag false has priority",
			mergeFlagChanged: true,
			cliMerge:         false,
			defaultsMerge:    true,
			expectedMerge:    false,
		},
		{
			name:          "defaults merge true enables merge for task run",
			cliMerge:      false,
			defaultsMerge: true,
			expectedMerge: true,
		},
		{
			name:          "defaults merge false keeps merge disabled",
			cliMerge:      false,
			defaultsMerge: false,
			expectedMerge: false,
		},
		{
			name:          "explicit worktree skips defaults",
			cliMerge:      false,
			cliWorktree:   "feature/cli",
			defaultsMerge: true,
			expectedMerge: false,
		},
		{
			name:          "resolver error falls back to cli merge",
			cliMerge:      false,
			resolverErr:   true,
			defaultsMerge: true,
			expectedMerge: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := runDefaultsResolver(func(params ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
				_ = params
				if tc.resolverErr {
					return conf.RunDefaultsConfig{}, assert.AnError
				}

				return conf.RunDefaultsConfig{Merge: tc.defaultsMerge}, nil
			})

			actual := resolveTaskRunMerge(tc.mergeFlagChanged, tc.cliMerge, tc.cliWorktree, resolver, "wd", "shadow", "project", "cfg")
			assert.Equal(t, tc.expectedMerge, actual)
		})
	}
}

func TestShouldDisableTaskWorktree(t *testing.T) {
	tests := []struct {
		name     string
		metadata *commands.TaskMetadata
		expected bool
	}{
		{
			name:     "nil metadata does not disable worktree",
			metadata: nil,
			expected: false,
		},
		{
			name: "nil feature branch does not disable worktree",
			metadata: &commands.TaskMetadata{
				FeatureBranch: nil,
			},
			expected: false,
		},
		{
			name: "empty feature branch disables worktree",
			metadata: &commands.TaskMetadata{
				FeatureBranch: strPtr(""),
			},
			expected: true,
		},
		{
			name: "blank feature branch disables worktree",
			metadata: &commands.TaskMetadata{
				FeatureBranch: strPtr("  \t "),
			},
			expected: true,
		},
		{
			name: "non-empty feature branch does not disable worktree",
			metadata: &commands.TaskMetadata{
				FeatureBranch: strPtr("feature/task-123"),
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := shouldDisableTaskWorktree(tc.metadata)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestShouldDisableTaskWorktreeForRun(t *testing.T) {
	tests := []struct {
		name     string
		metadata *commands.TaskMetadata
		taskData *core.Task
		expected bool
	}{
		{
			name:     "command metadata can disable worktree",
			metadata: &commands.TaskMetadata{FeatureBranch: strPtr("")},
			taskData: &core.Task{FeatureBranch: "feature/task"},
			expected: true,
		},
		{
			name:     "empty task feature branch disables worktree",
			metadata: nil,
			taskData: &core.Task{FeatureBranch: ""},
			expected: true,
		},
		{
			name:     "no commit task disables worktree",
			metadata: nil,
			taskData: &core.Task{FeatureBranch: "feature/task", NoCommit: true},
			expected: true,
		},
		{
			name:     "blank task feature branch disables worktree",
			metadata: nil,
			taskData: &core.Task{FeatureBranch: "   "},
			expected: true,
		},
		{
			name:     "non-empty task feature branch keeps worktree",
			metadata: nil,
			taskData: &core.Task{FeatureBranch: "feature/task"},
			expected: false,
		},
		{
			name:     "nil task does not disable worktree by itself",
			metadata: nil,
			taskData: nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := shouldDisableTaskWorktreeForRun(tc.metadata, tc.taskData)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestResolveTaskFinalStatusForRun(t *testing.T) {
	tests := []struct {
		name                 string
		sessionUpdatedStatus bool
		merge                bool
		expectedStatus       string
		expectedShouldApply  bool
	}{
		{
			name:                "merge without in-session update resolves to merged",
			merge:               true,
			expectedStatus:      "merged",
			expectedShouldApply: true,
		},
		{
			name:                "no merge without in-session update resolves to completed",
			merge:               false,
			expectedStatus:      "completed",
			expectedShouldApply: true,
		},
		{
			name:                 "in-session status update suppresses final status override",
			sessionUpdatedStatus: true,
			merge:                true,
			expectedStatus:       "",
			expectedShouldApply:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			session := core.NewSweSession(&core.SweSessionParams{})
			if tc.sessionUpdatedStatus {
				session.SetTaskStatusUpdatedInSession(true)
			}

			status, shouldApply := resolveTaskFinalStatusForRun(session, tc.merge)
			assert.Equal(t, tc.expectedStatus, status)
			assert.Equal(t, tc.expectedShouldApply, shouldApply)
		})
	}
}

func TestApplyRunDefaultsNoCommit(t *testing.T) {
	tests := []struct {
		name                string
		defaultsNoCommit    bool
		noCommitFlagChanged bool
		initialNoCommit     bool
		initialWorktree     string
		initialMerge        bool
		expectedNoCommit    bool
		expectedWorktree    string
		expectedMerge       bool
	}{
		{
			name:             "default no commit clears default worktree and merge",
			defaultsNoCommit: true,
			initialWorktree:  "feature/default",
			initialMerge:     true,
			expectedNoCommit: true,
		},
		{
			name:                "explicit no commit clears explicit worktree and merge",
			initialNoCommit:     true,
			noCommitFlagChanged: true,
			initialWorktree:     "feature/cli",
			initialMerge:        true,
			expectedNoCommit:    true,
		},
		{
			name:             "default false keeps worktree and merge",
			defaultsNoCommit: false,
			initialWorktree:  "feature/default",
			initialMerge:     true,
			expectedWorktree: "feature/default",
			expectedMerge:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "run"}
			cmd.Flags().Bool("no-commit", false, "")
			cmd.Flags().Int("run-bash-max", 0, "")
			if tc.noCommitFlagChanged {
				require.NoError(t, cmd.Flags().Set("no-commit", "true"))
			}

			workDir := "wd"
			model := ""
			worktree := tc.initialWorktree
			merge := tc.initialMerge
			logLLMRequests := false
			logLLMRequestsRaw := false
			thinking := ""
			lspServer := ""
			gitUser := ""
			gitEmail := ""
			maxThreads := 0
			shadowDir := ""
			allowAllPerms := false
			vfsAllow := []string(nil)
			noCommit := tc.initialNoCommit
			runBashMaxOutput := (*int)(nil)
			resolver := runDefaultsResolver(func(params ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
				_ = params
				return conf.RunDefaultsConfig{NoCommit: tc.defaultsNoCommit}, nil
			})

			err := applyRunDefaults(resolver, cmd, workDir, "shadow", "project", "cfg", &workDir, &model, &worktree, &merge, &logLLMRequests, &logLLMRequestsRaw, &thinking, &lspServer, &gitUser, &gitEmail, &maxThreads, &shadowDir, &allowAllPerms, &vfsAllow, &noCommit, &runBashMaxOutput)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedNoCommit, noCommit)
			assert.Equal(t, tc.expectedWorktree, worktree)
			assert.Equal(t, tc.expectedMerge, merge)
		})
	}
}

func strPtr(value string) *string {
	return &value
}
