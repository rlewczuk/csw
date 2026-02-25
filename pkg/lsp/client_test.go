package lsp

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientResolvePathUsesWorkingDirForRelativePaths verifies relative paths are
// resolved against the LSP client's configured working directory.
func TestClientResolvePathUsesWorkingDirForRelativePaths(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")

	client, err := NewClient("/bin/true", workspaceDir)
	require.NoError(t, err)

	absPath, err := client.resolvePath("pkg/file.go")
	require.NoError(t, err)

	expected := filepath.Join(workspaceDir, "pkg", "file.go")
	assert.Equal(t, expected, absPath)
}
