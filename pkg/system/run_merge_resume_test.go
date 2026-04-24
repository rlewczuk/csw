package system

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
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

func TestResolveTaskFinalStatusForRun(t *testing.T) {
	tests := []struct {
		name                   string
		sessionUpdatedStatus   bool
		merge                  bool
		expectedStatus         string
		expectedShouldApply    bool
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
