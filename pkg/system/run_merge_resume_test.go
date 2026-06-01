package system

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMergeCLIParams(t *testing.T) {
	tests := []struct {
		name           string
		params         *runExecution
		expectError    bool
		errorSubstring string
	}{
		{
			name: "merge without worktree is rejected",
			params: &runExecution{
				Merge: true,
			},
			expectError:    true,
			errorSubstring: "--merge requires --worktree",
		},
		{
			name: "merge with no commit is rejected",
			params: &runExecution{
				Merge:    true,
				NoCommit: true,
			},
			expectError:    true,
			errorSubstring: "--merge cannot be used with --no-commit",
		},
		{
			name: "no commit without worktree is allowed",
			params: &runExecution{
				NoCommit: true,
			},
		},
		{
			name: "merge with worktree is allowed",
			params: &runExecution{
				Merge:          true,
				WorktreeBranch: "feature/test",
			},
		},
		{
			name: "non merge without worktree is allowed",
			params: &runExecution{
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
			err := validateMergeRunExecution(tc.params)
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
		expectedMerge    bool
	}{
		{
			name:             "explicit merge flag true has priority",
			mergeFlagChanged: true,
			cliMerge:         true,
			expectedMerge:    true,
		},
		{
			name:             "explicit merge flag false has priority",
			mergeFlagChanged: true,
			cliMerge:         false,
			expectedMerge:    false,
		},
		{
			name:          "merge value is already resolved before task run",
			cliMerge:      false,
			expectedMerge: false,
		},
		{
			name:          "defaults merge false keeps merge disabled",
			cliMerge:      false,
			expectedMerge: false,
		},
		{
			name:          "explicit worktree skips defaults",
			cliMerge:      false,
			cliWorktree:   "feature/cli",
			expectedMerge: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := resolveTaskRunMerge(tc.mergeFlagChanged, tc.cliMerge, tc.cliWorktree)
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

func TestRunDefaultsNoCommit(t *testing.T) {
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
			defaults := conf.RunDefaultsConfig{Worktree: tc.initialWorktree, Merge: tc.initialMerge, NoCommit: tc.initialNoCommit || tc.defaultsNoCommit}
			if defaults.NoCommit {
				defaults.Worktree = ""
				defaults.Merge = false
			}

			assert.Equal(t, tc.expectedNoCommit, defaults.NoCommit)
			assert.Equal(t, tc.expectedWorktree, defaults.Worktree)
			assert.Equal(t, tc.expectedMerge, defaults.Merge)
		})
	}
}

func strPtr(value string) *string {
	return &value
}
