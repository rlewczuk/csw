package vfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNullVCS(t *testing.T) {
	mockVFS := NewMockVFS()

	nullVCS, err := NewNullVFS(mockVFS)
	require.NoError(t, err)
	assert.NotNil(t, nullVCS)
	assert.Equal(t, mockVFS, nullVCS.vfs)
}

func TestNullVCS_GetWorktree(t *testing.T) {
	mockVFS := NewMockVFS()
	nullVCS, err := NewNullVFS(mockVFS)
	require.NoError(t, err)

	tests := []struct {
		name   string
		branch string
	}{
		{
			name:   "returns vfs for main branch",
			branch: "main",
		},
		{
			name:   "returns vfs for any branch name",
			branch: "feature-branch",
		},
		{
			name:   "returns vfs for empty branch name",
			branch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vfs, err := nullVCS.GetWorktree(tt.branch)
			require.NoError(t, err)
			assert.Equal(t, mockVFS, vfs)
		})
	}
}

func TestNullVCS_DropWorktree(t *testing.T) {
	mockVFS := NewMockVFS()
	nullVCS, err := NewNullVFS(mockVFS)
	require.NoError(t, err)

	tests := []struct {
		name   string
		branch string
	}{
		{
			name:   "no effect for main branch",
			branch: "main",
		},
		{
			name:   "no effect for any branch",
			branch: "some-branch",
		},
		{
			name:   "no effect for empty branch",
			branch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nullVCS.DropWorktree(tt.branch)
			assert.NoError(t, err)
		})
	}
}

func TestNullVCS_CommitWorktree(t *testing.T) {
	mockVFS := NewMockVFS()
	nullVCS, err := NewNullVFS(mockVFS)
	require.NoError(t, err)

	tests := []struct {
		name    string
		branch  string
		message string
	}{
		{
			name:    "no effect for main branch",
			branch:  "main",
			message: "test commit",
		},
		{
			name:    "no effect for any branch",
			branch:  "feature",
			message: "another commit",
		},
		{
			name:    "no effect with empty message",
			branch:  "main",
			message: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nullVCS.CommitWorktree(tt.branch, tt.message)
			assert.NoError(t, err)
		})
	}
}

func TestNullVCS_NewBranch(t *testing.T) {
	mockVFS := NewMockVFS()
	nullVCS, err := NewNullVFS(mockVFS)
	require.NoError(t, err)

	tests := []struct {
		name string
		from string
		to   string
	}{
		{
			name: "no effect for new branch from main",
			from: "main",
			to:   "feature",
		},
		{
			name: "no effect for any branch names",
			from: "feature",
			to:   "another-feature",
		},
		{
			name: "no effect with empty names",
			from: "",
			to:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nullVCS.NewBranch(tt.to, tt.from)
			assert.NoError(t, err)
		})
	}
}

func TestNullVCS_DeleteBranch(t *testing.T) {
	mockVFS := NewMockVFS()
	nullVCS, err := NewNullVFS(mockVFS)
	require.NoError(t, err)

	tests := []struct {
		name   string
		branch string
	}{
		{
			name:   "no effect for main branch",
			branch: "main",
		},
		{
			name:   "no effect for any branch",
			branch: "feature",
		},
		{
			name:   "no effect for empty branch",
			branch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nullVCS.DeleteBranch(tt.branch)
			assert.NoError(t, err)
		})
	}
}

func TestNullVCS_ListBranches(t *testing.T) {
	mockVFS := NewMockVFS()
	nullVCS, err := NewNullVFS(mockVFS)
	require.NoError(t, err)

	tests := []struct {
		name     string
		prefix   string
		expected []string
	}{
		{
			name:     "returns main with empty prefix",
			prefix:   "",
			expected: []string{"main"},
		},
		{
			name:     "returns main with matching prefix",
			prefix:   "ma",
			expected: []string{"main"},
		},
		{
			name:     "returns main with full prefix",
			prefix:   "main",
			expected: []string{"main"},
		},
		{
			name:     "returns main even with non-matching prefix",
			prefix:   "feature",
			expected: []string{"main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branches, err := nullVCS.ListBranches(tt.prefix)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, branches)
		})
	}
}

func TestNullVCS_MergeBranches(t *testing.T) {
	mockVFS := NewMockVFS()
	nullVCS, err := NewNullVFS(mockVFS)
	require.NoError(t, err)

	tests := []struct {
		name string
		into string
		from string
	}{
		{
			name: "no effect for merge into main",
			into: "main",
			from: "feature",
		},
		{
			name: "no effect for merge between any branches",
			into: "develop",
			from: "feature",
		},
		{
			name: "no effect with empty names",
			into: "",
			from: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nullVCS.MergeBranches(tt.into, tt.from)
			assert.NoError(t, err)
		})
	}
}

func TestNullVCS_VCSInterface(t *testing.T) {
	// Verify NullVCS implements VCS interface
	mockVFS := NewMockVFS()
	nullVCS, err := NewNullVFS(mockVFS)
	require.NoError(t, err)

	// This will fail at compile time if NullVCS doesn't implement VCS
	var _ VCS = nullVCS
}

func TestNullVCS_MultipleCalls(t *testing.T) {
	// Test that multiple calls to methods don't cause issues
	mockVFS := NewMockVFS()
	nullVCS, err := NewNullVFS(mockVFS)
	require.NoError(t, err)

	// Call GetWorktree multiple times
	vfs1, err := nullVCS.GetWorktree("main")
	require.NoError(t, err)
	vfs2, err := nullVCS.GetWorktree("main")
	require.NoError(t, err)
	assert.Equal(t, vfs1, vfs2)

	// Call ListBranches multiple times
	branches1, err := nullVCS.ListBranches("")
	require.NoError(t, err)
	branches2, err := nullVCS.ListBranches("")
	require.NoError(t, err)
	assert.Equal(t, branches1, branches2)

	// Call methods that have no effect multiple times
	for i := 0; i < 3; i++ {
		assert.NoError(t, nullVCS.DropWorktree("main"))
		assert.NoError(t, nullVCS.CommitWorktree("main", "msg"))
		assert.NoError(t, nullVCS.NewBranch("new", "main"))
		assert.NoError(t, nullVCS.DeleteBranch("main"))
		assert.NoError(t, nullVCS.MergeBranches("main", "feature"))
	}
}
