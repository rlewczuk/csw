package vfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAllowedPathFixture creates a test fixture with allowed paths
func setupAllowedPathFixture(t *testing.T, allowedPaths []string) (*LocalVFS, string, []string) {
	t.Helper()

	// Create root directory
	rootDir, err := os.MkdirTemp("", "vfs-allowed-test-root-*")
	require.NoError(t, err, "Failed to create root temp directory")

	// Create allowed directories
	var allowedDirs []string
	for i, path := range allowedPaths {
		if path == "" {
			// Create a temp directory for empty path
			tempDir, err := os.MkdirTemp("", "vfs-allowed-test-allowed-*")
			require.NoError(t, err, "Failed to create allowed temp directory")
			allowedDirs = append(allowedDirs, tempDir)
		} else {
			// Use the provided path (will be resolved to absolute)
			absPath, err := filepath.Abs(path)
			require.NoError(t, err)
			// Create the directory if it doesn't exist
			err = os.MkdirAll(absPath, 0755)
			if err != nil && !os.IsExist(err) {
				require.NoError(t, err, "Failed to create allowed directory")
			}
			allowedDirs = append(allowedDirs, absPath)
		}
		// Replace the original path with the actual directory for reference
		if i < len(allowedPaths) {
			allowedPaths[i] = allowedDirs[len(allowedDirs)-1]
		}
	}

	localVFS, err := NewLocalVFS(rootDir, nil, allowedDirs)
	if err != nil {
		os.RemoveAll(rootDir)
		for _, dir := range allowedDirs {
			os.RemoveAll(dir)
		}
	}
	require.NoError(t, err, "Failed to create LocalVFS")

	return localVFS, rootDir, allowedDirs
}

func cleanupAllowedPathFixture(rootDir string, allowedDirs []string) {
	os.RemoveAll(rootDir)
	for _, dir := range allowedDirs {
		os.RemoveAll(dir)
	}
}

func TestNewLocalVFS_WithAllowedPaths(t *testing.T) {
	t.Run("ValidAllowedPaths", func(t *testing.T) {
		// Create allowed directories
		allowedDir1, err := os.MkdirTemp("", "vfs-allowed-1-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir1)

		allowedDir2, err := os.MkdirTemp("", "vfs-allowed-2-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir2)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir1, allowedDir2})
		require.NoError(t, err)
		assert.NotNil(t, localVFS)
		assert.Len(t, localVFS.allowedPaths, 2)
	})

	t.Run("EmptyAllowedPaths", func(t *testing.T) {
		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, localVFS)
		assert.Empty(t, localVFS.allowedPaths)
	})

	t.Run("RelativeAllowedPath", func(t *testing.T) {
		// Create allowed directory
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		// Get relative path
		cwd, err := os.Getwd()
		require.NoError(t, err)
		relPath, err := filepath.Rel(cwd, allowedDir)
		require.NoError(t, err)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{relPath})
		require.NoError(t, err)
		assert.NotNil(t, localVFS)
		assert.Len(t, localVFS.allowedPaths, 1)
		// Path should be converted to absolute
		assert.True(t, filepath.IsAbs(localVFS.allowedPaths[0]))
	})

	t.Run("NonExistentAllowedPath", func(t *testing.T) {
		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		// Non-existent path should be created if it's a valid path
		// or return error if it can't be resolved
		_, err = NewLocalVFS(rootDir, nil, []string{"/path/that/does/not/exist/12345"})
		// This may or may not error depending on whether the path can be resolved
		// The behavior is to try to convert to absolute path
		assert.NoError(t, err) // filepath.Abs doesn't fail for non-existent paths
	})
}

func TestLocalVFS_ReadFile_AllowedPaths(t *testing.T) {
	t.Run("ReadFileInAllowedPath", func(t *testing.T) {
		// Create allowed directory with a file
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		testFile := filepath.Join(allowedDir, "test.txt")
		testContent := []byte("allowed content")
		err = os.WriteFile(testFile, testContent, 0644)
		require.NoError(t, err)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Read file using absolute path
		content, err := localVFS.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, testContent, content)
	})

	t.Run("ReadFileInNestedAllowedPath", func(t *testing.T) {
		// Create allowed directory with nested structure
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		nestedDir := filepath.Join(allowedDir, "nested", "deep")
		err = os.MkdirAll(nestedDir, 0755)
		require.NoError(t, err)

		testFile := filepath.Join(nestedDir, "test.txt")
		testContent := []byte("nested content")
		err = os.WriteFile(testFile, testContent, 0644)
		require.NoError(t, err)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Read file using absolute path
		content, err := localVFS.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, testContent, content)
	})

	t.Run("ReadFileOutsideAllowedPath", func(t *testing.T) {
		// Create allowed directory
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		// Create a file outside allowed path
		outsideDir, err := os.MkdirTemp("", "vfs-outside-*")
		require.NoError(t, err)
		defer os.RemoveAll(outsideDir)

		testFile := filepath.Join(outsideDir, "test.txt")
		err = os.WriteFile(testFile, []byte("outside content"), 0644)
		require.NoError(t, err)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Try to read file outside allowed path
		_, err = localVFS.ReadFile(testFile)
		assert.ErrorIs(t, err, apis.ErrPermissionDenied)
	})

	t.Run("ReadFileWithMultipleAllowedPaths", func(t *testing.T) {
		// Create two allowed directories
		allowedDir1, err := os.MkdirTemp("", "vfs-allowed-1-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir1)

		allowedDir2, err := os.MkdirTemp("", "vfs-allowed-2-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir2)

		testFile1 := filepath.Join(allowedDir1, "test1.txt")
		err = os.WriteFile(testFile1, []byte("content1"), 0644)
		require.NoError(t, err)

		testFile2 := filepath.Join(allowedDir2, "test2.txt")
		err = os.WriteFile(testFile2, []byte("content2"), 0644)
		require.NoError(t, err)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir1, allowedDir2})
		require.NoError(t, err)

		// Read files from both allowed paths
		content1, err := localVFS.ReadFile(testFile1)
		require.NoError(t, err)
		assert.Equal(t, []byte("content1"), content1)

		content2, err := localVFS.ReadFile(testFile2)
		require.NoError(t, err)
		assert.Equal(t, []byte("content2"), content2)
	})
}

func TestLocalVFS_WriteFile_AllowedPaths(t *testing.T) {
	t.Run("WriteFileInAllowedPath", func(t *testing.T) {
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Write file using absolute path
		testFile := filepath.Join(allowedDir, "test.txt")
		testContent := []byte("written content")
		err = localVFS.WriteFile(testFile, testContent)
		require.NoError(t, err)

		// Verify file was written
		content, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, testContent, content)
	})

	t.Run("WriteFileOutsideAllowedPath", func(t *testing.T) {
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		outsideDir, err := os.MkdirTemp("", "vfs-outside-*")
		require.NoError(t, err)
		defer os.RemoveAll(outsideDir)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Try to write file outside allowed path
		testFile := filepath.Join(outsideDir, "test.txt")
		err = localVFS.WriteFile(testFile, []byte("content"))
		assert.ErrorIs(t, err, apis.ErrPermissionDenied)
	})
}

func TestLocalVFS_DeleteFile_AllowedPaths(t *testing.T) {
	t.Run("DeleteFileInAllowedPath", func(t *testing.T) {
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		testFile := filepath.Join(allowedDir, "test.txt")
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Delete file using absolute path
		err = localVFS.DeleteFile(testFile, false, false)
		require.NoError(t, err)

		// Verify file was deleted
		_, err = os.Stat(testFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("DeleteFileOutsideAllowedPath", func(t *testing.T) {
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		outsideDir, err := os.MkdirTemp("", "vfs-outside-*")
		require.NoError(t, err)
		defer os.RemoveAll(outsideDir)

		testFile := filepath.Join(outsideDir, "test.txt")
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Try to delete file outside allowed path
		err = localVFS.DeleteFile(testFile, false, false)
		assert.ErrorIs(t, err, apis.ErrPermissionDenied)
	})
}

func TestLocalVFS_ListFiles_AllowedPaths(t *testing.T) {
	t.Run("ListFilesInAllowedPath", func(t *testing.T) {
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		// Create files in allowed directory
		err = os.WriteFile(filepath.Join(allowedDir, "file1.txt"), []byte("content1"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(allowedDir, "file2.txt"), []byte("content2"), 0644)
		require.NoError(t, err)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// List files using absolute path
		files, err := localVFS.ListFiles(allowedDir, false)
		require.NoError(t, err)
		assert.Len(t, files, 2)
	})

	t.Run("ListFilesOutsideAllowedPath", func(t *testing.T) {
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		outsideDir, err := os.MkdirTemp("", "vfs-outside-*")
		require.NoError(t, err)
		defer os.RemoveAll(outsideDir)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Try to list files outside allowed path
		_, err = localVFS.ListFiles(outsideDir, false)
		assert.ErrorIs(t, err, apis.ErrPermissionDenied)
	})
}

func TestLocalVFS_MoveFile_AllowedPaths(t *testing.T) {
	t.Run("MoveFileWithinAllowedPath", func(t *testing.T) {
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		srcFile := filepath.Join(allowedDir, "src.txt")
		dstFile := filepath.Join(allowedDir, "dst.txt")
		err = os.WriteFile(srcFile, []byte("content"), 0644)
		require.NoError(t, err)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Move file within allowed path
		err = localVFS.MoveFile(srcFile, dstFile)
		require.NoError(t, err)

		// Verify move
		_, err = os.Stat(srcFile)
		assert.True(t, os.IsNotExist(err))
		content, err := os.ReadFile(dstFile)
		require.NoError(t, err)
		assert.Equal(t, []byte("content"), content)
	})

	t.Run("MoveFileFromAllowedToOutside", func(t *testing.T) {
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		outsideDir, err := os.MkdirTemp("", "vfs-outside-*")
		require.NoError(t, err)
		defer os.RemoveAll(outsideDir)

		srcFile := filepath.Join(allowedDir, "src.txt")
		dstFile := filepath.Join(outsideDir, "dst.txt")
		err = os.WriteFile(srcFile, []byte("content"), 0644)
		require.NoError(t, err)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Try to move file from allowed to outside
		err = localVFS.MoveFile(srcFile, dstFile)
		assert.ErrorIs(t, err, apis.ErrPermissionDenied)
	})

	t.Run("MoveFileFromOutsideToAllowed", func(t *testing.T) {
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		outsideDir, err := os.MkdirTemp("", "vfs-outside-*")
		require.NoError(t, err)
		defer os.RemoveAll(outsideDir)

		srcFile := filepath.Join(outsideDir, "src.txt")
		dstFile := filepath.Join(allowedDir, "dst.txt")
		err = os.WriteFile(srcFile, []byte("content"), 0644)
		require.NoError(t, err)

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Try to move file from outside to allowed
		err = localVFS.MoveFile(srcFile, dstFile)
		assert.ErrorIs(t, err, apis.ErrPermissionDenied)
	})
}

func TestLocalVFS_RootAndAllowedPaths(t *testing.T) {
	t.Run("AccessRootAndAllowedPath", func(t *testing.T) {
		// Create root directory with a file
		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		rootFile := filepath.Join(rootDir, "root.txt")
		err = os.WriteFile(rootFile, []byte("root content"), 0644)
		require.NoError(t, err)

		// Create allowed directory with a file
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		allowedFile := filepath.Join(allowedDir, "allowed.txt")
		err = os.WriteFile(allowedFile, []byte("allowed content"), 0644)
		require.NoError(t, err)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Access file in root (relative path)
		content, err := localVFS.ReadFile("root.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("root content"), content)

		// Access file in allowed path (absolute path)
		content, err = localVFS.ReadFile(allowedFile)
		require.NoError(t, err)
		assert.Equal(t, []byte("allowed content"), content)
	})

	t.Run("RelativePathResolvesToRoot", func(t *testing.T) {
		// Create root directory
		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		// Create allowed directory
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		// Create a file in root
		rootFile := filepath.Join(rootDir, "test.txt")
		err = os.WriteFile(rootFile, []byte("root content"), 0644)
		require.NoError(t, err)

		// Create a file with same name in allowed
		allowedFile := filepath.Join(allowedDir, "test.txt")
		err = os.WriteFile(allowedFile, []byte("allowed content"), 0644)
		require.NoError(t, err)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Relative path should resolve to root
		content, err := localVFS.ReadFile("test.txt")
		require.NoError(t, err)
		assert.Equal(t, []byte("root content"), content)

		// Absolute path should access allowed
		content, err = localVFS.ReadFile(allowedFile)
		require.NoError(t, err)
		assert.Equal(t, []byte("allowed content"), content)
	})
}

func TestLocalVFS_PathTraversalWithAllowedPaths(t *testing.T) {
	t.Run("PathTraversalEscapeAllowedPath", func(t *testing.T) {
		// Create allowed directory
		allowedDir, err := os.MkdirTemp("", "vfs-allowed-*")
		require.NoError(t, err)
		defer os.RemoveAll(allowedDir)

		// Create a parent directory with a file (outside allowed)
		parentDir := filepath.Dir(allowedDir)
		outsideFile := filepath.Join(parentDir, "outside.txt")
		err = os.WriteFile(outsideFile, []byte("outside content"), 0644)
		// This might fail due to permissions, but that's OK for the test

		rootDir, err := os.MkdirTemp("", "vfs-root-*")
		require.NoError(t, err)
		defer os.RemoveAll(rootDir)

		localVFS, err := NewLocalVFS(rootDir, nil, []string{allowedDir})
		require.NoError(t, err)

		// Try path traversal from within allowed path
		traversalPath := filepath.Join(allowedDir, "..", "outside.txt")
		_, err = localVFS.ReadFile(traversalPath)
		// Should be denied because the resolved path is outside allowed paths
		assert.ErrorIs(t, err, apis.ErrPermissionDenied)
	})
}
