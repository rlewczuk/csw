package main

import (
	"testing"

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
