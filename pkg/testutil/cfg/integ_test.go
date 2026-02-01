package cfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMkTempDir tests the MkTempDir function.
func TestMkTempDir(t *testing.T) {
	// Create a temporary project root for testing
	projectRoot := t.TempDir()

	// Create tmp directory in project root
	err := os.MkdirAll(filepath.Join(projectRoot, "tmp"), 0755)
	require.NoError(t, err)

	// Test creating a temporary directory
	tmpDir := MkTempDir(t, projectRoot, "test_mktemp_*")

	// Verify the directory exists
	info, err := os.Stat(tmpDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify the directory is inside projectRoot/tmp
	assert.True(t, strings.HasPrefix(tmpDir, filepath.Join(projectRoot, "tmp")))

	// Verify the pattern is applied
	assert.Contains(t, tmpDir, "test_mktemp_")

	// Create a test file in the directory
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Verify the file exists
	_, err = os.Stat(testFile)
	require.NoError(t, err)
}

// TestMkTempDirCleanupOnSuccess verifies cleanup happens on test success.
func TestMkTempDirCleanupOnSuccess(t *testing.T) {
	// Create a temporary project root for testing
	projectRoot := t.TempDir()

	// Create tmp directory in project root
	err := os.MkdirAll(filepath.Join(projectRoot, "tmp"), 0755)
	require.NoError(t, err)

	var createdDir string

	// Run a sub-test that will succeed
	t.Run("SubTestThatSucceeds", func(t *testing.T) {
		tmpDir := MkTempDir(t, projectRoot, "test_cleanup_*")
		createdDir = tmpDir

		// Create a test file
		testFile := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		// Directory should exist during test
		_, err = os.Stat(tmpDir)
		require.NoError(t, err)
	})

	// After successful sub-test, directory should be cleaned up
	_, err = os.Stat(createdDir)
	assert.True(t, os.IsNotExist(err), "Expected directory to be removed after successful test")
}
