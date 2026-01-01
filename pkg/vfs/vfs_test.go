package vfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFixture provides a temporary directory for testing
type TestFixture struct {
	Root string
	VFS  VFS
}

// Setup creates a temporary directory and VFS instance (either LocalVFS or MockVFS)
func setupFixture(t *testing.T) *TestFixture {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "vfs-test-*")
	require.NoError(t, err, "Failed to create temp directory")

	localVFS, err := NewLocalVFS(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
	}
	require.NoError(t, err, "Failed to create LocalVFS")

	return &TestFixture{
		Root: tempDir,
		VFS:  localVFS,
	}
}

func setupMockFixture(t *testing.T) *TestFixture {
	t.Helper()

	mockVFS := NewMockVFS()
	return &TestFixture{
		Root: "",
		VFS:  mockVFS,
	}
}

// Cleanup removes the temporary directory
func (f *TestFixture) Cleanup() {
	if f.Root != "" {
		os.RemoveAll(f.Root)
	}
}

// runTestWithBothVFS runs a test function with both LocalVFS and MockVFS
func runTestWithBothVFS(t *testing.T, testFunc func(*testing.T, *TestFixture)) {
	t.Run("LocalVFS", func(t *testing.T) {
		fixture := setupFixture(t)
		defer fixture.Cleanup()
		testFunc(t, fixture)
	})

	t.Run("MockVFS", func(t *testing.T) {
		fixture := setupMockFixture(t)
		defer fixture.Cleanup()
		testFunc(t, fixture)
	})
}

func TestNewLocalVFS(t *testing.T) {
	t.Run("ValidRootDirectory", func(t *testing.T) {
		fixture := setupFixture(t)
		defer fixture.Cleanup()

		assert.NotNil(t, fixture.VFS, "Expected non-nil VFS")
	})

	t.Run("NonExistentDirectory", func(t *testing.T) {
		_, err := NewLocalVFS("/path/that/does/not/exist")
		assert.ErrorIs(t, err, ErrFileNotFound)
	})

	t.Run("RootIsFile", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "vfs-test-file-*")
		require.NoError(t, err, "Failed to create temp file")
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		_, err = NewLocalVFS(tempFile.Name())
		assert.ErrorIs(t, err, ErrNotADir)
	})

	t.Run("RelativePath", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "vfs-test-*")
		require.NoError(t, err, "Failed to create temp directory")
		defer os.RemoveAll(tempDir)

		// Get relative path
		cwd, err := os.Getwd()
		require.NoError(t, err, "Failed to get current directory")

		relPath, err := filepath.Rel(cwd, tempDir)
		require.NoError(t, err, "Failed to get relative path")

		localVFS, err := NewLocalVFS(relPath)
		require.NoError(t, err, "Expected success with relative path")

		assert.True(t, filepath.IsAbs(localVFS.root), "Expected absolute root path")
	})
}

func TestReadFile(t *testing.T) {
	t.Run("ReadExistingFile", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testContent := []byte("test content")
			testPath := "test.txt"

			// Write test file using VFS
			require.NoError(t, fixture.VFS.WriteFile(testPath, testContent), "Failed to create test file")

			// Read using VFS
			content, err := fixture.VFS.ReadFile(testPath)
			require.NoError(t, err, "Failed to read file")
			assert.Equal(t, testContent, content, "Content mismatch")
		})
	})
}

func TestMoveFile(t *testing.T) {
	t.Run("MoveFileToSameDirectory", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testContent := []byte("test content")
			srcPath := "old.txt"
			dstPath := "new.txt"

			// Create source file
			require.NoError(t, fixture.VFS.WriteFile(srcPath, testContent), "Failed to create test file")

			// Move file
			err := fixture.VFS.MoveFile(srcPath, dstPath)
			require.NoError(t, err, "Failed to move file")

			// Verify destination file exists with correct content
			content, err := fixture.VFS.ReadFile(dstPath)
			require.NoError(t, err, "Failed to read destination file")
			assert.Equal(t, testContent, content, "Content mismatch")

			// Verify source file no longer exists
			_, err = fixture.VFS.ReadFile(srcPath)
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected source file to be deleted")
		})
	})

	t.Run("RenameFile", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testContent := []byte("rename test")
			srcPath := "original.txt"
			dstPath := "renamed.txt"

			// Create source file
			require.NoError(t, fixture.VFS.WriteFile(srcPath, testContent), "Failed to create test file")

			// Rename file
			err := fixture.VFS.MoveFile(srcPath, dstPath)
			require.NoError(t, err, "Failed to rename file")

			// Verify renamed file exists
			content, err := fixture.VFS.ReadFile(dstPath)
			require.NoError(t, err, "Failed to read renamed file")
			assert.Equal(t, testContent, content, "Content mismatch")

			// Verify original file no longer exists
			_, err = fixture.VFS.ReadFile(srcPath)
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected original file to be deleted")
		})
	})

	t.Run("MoveFileToDifferentDirectory", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testContent := []byte("move test")
			srcPath := "file.txt"
			dstPath := "subdir/file.txt"

			// Create source file
			require.NoError(t, fixture.VFS.WriteFile(srcPath, testContent), "Failed to create test file")

			// Move file to different directory
			err := fixture.VFS.MoveFile(srcPath, dstPath)
			require.NoError(t, err, "Failed to move file")

			// Verify destination file exists
			content, err := fixture.VFS.ReadFile(dstPath)
			require.NoError(t, err, "Failed to read destination file")
			assert.Equal(t, testContent, content, "Content mismatch")

			// Verify source file no longer exists
			_, err = fixture.VFS.ReadFile(srcPath)
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected source file to be deleted")
		})
	})

	t.Run("MoveFileToNestedDirectory", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testContent := []byte("nested move test")
			srcPath := "file.txt"
			dstPath := "a/b/c/file.txt"

			// Create source file
			require.NoError(t, fixture.VFS.WriteFile(srcPath, testContent), "Failed to create test file")

			// Move file to nested directory
			err := fixture.VFS.MoveFile(srcPath, dstPath)
			require.NoError(t, err, "Failed to move file")

			// Verify destination file exists
			content, err := fixture.VFS.ReadFile(dstPath)
			require.NoError(t, err, "Failed to read destination file")
			assert.Equal(t, testContent, content, "Content mismatch")

			// Verify source file no longer exists
			_, err = fixture.VFS.ReadFile(srcPath)
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected source file to be deleted")
		})
	})

	t.Run("MoveDirectory", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create directory with files
			require.NoError(t, fixture.VFS.WriteFile("olddir/file1.txt", []byte("content1")), "Failed to create file1")
			require.NoError(t, fixture.VFS.WriteFile("olddir/file2.txt", []byte("content2")), "Failed to create file2")
			require.NoError(t, fixture.VFS.WriteFile("olddir/subdir/file3.txt", []byte("content3")), "Failed to create file3")

			// Move directory
			err := fixture.VFS.MoveFile("olddir", "newdir")
			require.NoError(t, err, "Failed to move directory")

			// Verify files in new directory
			content1, err := fixture.VFS.ReadFile("newdir/file1.txt")
			require.NoError(t, err, "Failed to read file1 in new location")
			assert.Equal(t, []byte("content1"), content1, "Content mismatch for file1")

			content2, err := fixture.VFS.ReadFile("newdir/file2.txt")
			require.NoError(t, err, "Failed to read file2 in new location")
			assert.Equal(t, []byte("content2"), content2, "Content mismatch for file2")

			content3, err := fixture.VFS.ReadFile("newdir/subdir/file3.txt")
			require.NoError(t, err, "Failed to read file3 in new location")
			assert.Equal(t, []byte("content3"), content3, "Content mismatch for file3")

			// Verify old directory no longer accessible
			_, err = fixture.VFS.ReadFile("olddir/file1.txt")
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected old directory to be moved")
		})
	})

	t.Run("MoveDirectoryToNestedLocation", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create directory with files
			require.NoError(t, fixture.VFS.WriteFile("movedir/file.txt", []byte("content")), "Failed to create file")

			// Move directory to nested location
			err := fixture.VFS.MoveFile("movedir", "parent/child/movedir")
			require.NoError(t, err, "Failed to move directory")

			// Verify file in new location
			content, err := fixture.VFS.ReadFile("parent/child/movedir/file.txt")
			require.NoError(t, err, "Failed to read file in new location")
			assert.Equal(t, []byte("content"), content, "Content mismatch")

			// Verify old directory no longer exists
			_, err = fixture.VFS.ReadFile("movedir/file.txt")
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected old directory to be moved")
		})
	})

	t.Run("MoveNonExistentFile", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.MoveFile("nonexistent.txt", "newname.txt")
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected ErrFileNotFound for non-existent source")
		})
	})

	t.Run("MoveToExistingDestination", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create source and destination files
			require.NoError(t, fixture.VFS.WriteFile("src.txt", []byte("source")), "Failed to create source file")
			require.NoError(t, fixture.VFS.WriteFile("dst.txt", []byte("destination")), "Failed to create destination file")

			// Try to move to existing destination
			err := fixture.VFS.MoveFile("src.txt", "dst.txt")
			assert.ErrorIs(t, err, ErrFileExists, "Expected ErrFileExists for existing destination")
		})
	})

	t.Run("MoveWithInvalidSourcePath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.MoveFile("../../../etc/passwd", "newname.txt")
			assert.ErrorIs(t, err, ErrPermissionDenied, "Expected ErrPermissionDenied for path traversal")
		})
	})

	t.Run("MoveWithInvalidDestinationPath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			require.NoError(t, fixture.VFS.WriteFile("file.txt", []byte("content")), "Failed to create test file")

			err := fixture.VFS.MoveFile("file.txt", "../../../tmp/evil.txt")
			assert.ErrorIs(t, err, ErrPermissionDenied, "Expected ErrPermissionDenied for path traversal")
		})
	})

	t.Run("MoveWithAbsoluteSourcePath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.MoveFile("/etc/passwd", "newname.txt")
			assert.ErrorIs(t, err, ErrInvalidPath, "Expected ErrInvalidPath for absolute source path")
		})
	})

	t.Run("MoveWithAbsoluteDestinationPath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			require.NoError(t, fixture.VFS.WriteFile("file.txt", []byte("content")), "Failed to create test file")

			err := fixture.VFS.MoveFile("file.txt", "/tmp/newfile.txt")
			assert.ErrorIs(t, err, ErrInvalidPath, "Expected ErrInvalidPath for absolute destination path")
		})
	})

	t.Run("MoveWithEmptySourcePath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.MoveFile("", "newname.txt")
			assert.ErrorIs(t, err, ErrInvalidPath, "Expected ErrInvalidPath for empty source path")
		})
	})

	t.Run("MoveWithEmptyDestinationPath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			require.NoError(t, fixture.VFS.WriteFile("file.txt", []byte("content")), "Failed to create test file")

			err := fixture.VFS.MoveFile("file.txt", "")
			assert.ErrorIs(t, err, ErrInvalidPath, "Expected ErrInvalidPath for empty destination path")
		})
	})

	t.Run("MoveFileFromSubdirectory", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testContent := []byte("subdirectory test")
			srcPath := "dir1/dir2/file.txt"
			dstPath := "dir3/file.txt"

			// Create source file in subdirectory
			require.NoError(t, fixture.VFS.WriteFile(srcPath, testContent), "Failed to create test file")

			// Move file
			err := fixture.VFS.MoveFile(srcPath, dstPath)
			require.NoError(t, err, "Failed to move file")

			// Verify destination file exists
			content, err := fixture.VFS.ReadFile(dstPath)
			require.NoError(t, err, "Failed to read destination file")
			assert.Equal(t, testContent, content, "Content mismatch")

			// Verify source file no longer exists
			_, err = fixture.VFS.ReadFile(srcPath)
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected source file to be deleted")
		})
	})
}

func TestWriteFile(t *testing.T) {
	t.Run("WriteNewFile", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testContent := []byte("new content")
			testPath := "new.txt"

			err := fixture.VFS.WriteFile(testPath, testContent)
			require.NoError(t, err, "Failed to write file")

			// Verify file was written
			content, err := fixture.VFS.ReadFile(testPath)
			require.NoError(t, err, "Failed to read written file")
			assert.Equal(t, testContent, content, "Content mismatch")
		})
	})

	t.Run("OverwriteExistingFile", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testPath := "overwrite.txt"
			originalContent := []byte("original")
			newContent := []byte("overwritten")

			// Write original
			require.NoError(t, fixture.VFS.WriteFile(testPath, originalContent), "Failed to write original file")

			// Overwrite
			require.NoError(t, fixture.VFS.WriteFile(testPath, newContent), "Failed to overwrite file")

			// Verify
			content, err := fixture.VFS.ReadFile(testPath)
			require.NoError(t, err, "Failed to read file")
			assert.Equal(t, newContent, content, "Content mismatch")
		})
	})

	t.Run("WriteFileWithNestedDirectories", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testContent := []byte("nested content")
			testPath := "a/b/c/deep.txt"

			err := fixture.VFS.WriteFile(testPath, testContent)
			require.NoError(t, err, "Failed to write file")

			// Verify file was written
			content, err := fixture.VFS.ReadFile(testPath)
			require.NoError(t, err, "Failed to read written file")
			assert.Equal(t, testContent, content, "Content mismatch")
		})
	})

	t.Run("PathTraversalAttack", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.WriteFile("../../../tmp/evil.txt", []byte("evil"))
			assert.ErrorIs(t, err, ErrPermissionDenied, "Expected ErrPermissionDenied for path traversal")
		})
	})

	t.Run("AbsolutePath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.WriteFile("/tmp/evil.txt", []byte("evil"))
			assert.ErrorIs(t, err, ErrInvalidPath, "Expected ErrInvalidPath for absolute path")
		})
	})

	t.Run("EmptyPath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.WriteFile("", []byte("content"))
			assert.ErrorIs(t, err, ErrInvalidPath, "Expected ErrInvalidPath for empty path")
		})
	})
}

func TestDeleteFile(t *testing.T) {
	t.Run("DeleteExistingFile", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testPath := "delete.txt"
			require.NoError(t, fixture.VFS.WriteFile(testPath, []byte("content")), "Failed to create test file")

			err := fixture.VFS.DeleteFile(testPath, false, false)
			require.NoError(t, err, "Failed to delete file")

			// Verify file was deleted
			_, err = fixture.VFS.ReadFile(testPath)
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected ErrFileNotFound after deletion")
		})
	})

	t.Run("DeleteNonExistentFile", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.DeleteFile("nonexistent.txt", false, false)
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})

	t.Run("DeleteDirectoryWithoutRecursive", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testDir := "testdir"
			// Create directory by writing a file in it, then deleting the file
			require.NoError(t, fixture.VFS.WriteFile(filepath.Join(testDir, "dummy.txt"), []byte("dummy")), "Failed to create test directory")
			require.NoError(t, fixture.VFS.DeleteFile(filepath.Join(testDir, "dummy.txt"), false, false), "Failed to delete dummy file")

			err := fixture.VFS.DeleteFile(testDir, false, false)
			assert.ErrorIs(t, err, ErrNotAFile)
		})
	})

	t.Run("DeleteEmptyDirectoryRecursive", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testDir := "emptydir"
			// Create directory by writing a file in it, then deleting the file
			require.NoError(t, fixture.VFS.WriteFile(filepath.Join(testDir, "dummy.txt"), []byte("dummy")), "Failed to create test directory")
			require.NoError(t, fixture.VFS.DeleteFile(filepath.Join(testDir, "dummy.txt"), false, false), "Failed to delete dummy file")

			err := fixture.VFS.DeleteFile(testDir, true, false)
			require.NoError(t, err, "Failed to delete empty directory")

			// Verify directory was deleted
			_, err = fixture.VFS.ListFiles(testDir, false)
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected directory to be deleted")
		})
	})

	t.Run("DeleteDirectoryWithContentsRecursive", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testDir := "dirwithfiles"
			require.NoError(t, fixture.VFS.WriteFile(filepath.Join(testDir, "file1.txt"), []byte("content1")), "Failed to create test file")
			require.NoError(t, fixture.VFS.WriteFile(filepath.Join(testDir, "subdir", "file2.txt"), []byte("content2")), "Failed to create test file")

			err := fixture.VFS.DeleteFile(testDir, true, false)
			require.NoError(t, err, "Failed to delete directory")

			// Verify directory was deleted
			_, err = fixture.VFS.ListFiles(testDir, false)
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected directory to be deleted")
		})
	})

	t.Run("DeleteReadOnlyFileWithForce", func(t *testing.T) {
		// This test only applies to LocalVFS as MockVFS doesn't have file permissions
		t.Run("LocalVFS", func(t *testing.T) {
			fixture := setupFixture(t)
			defer fixture.Cleanup()

			testPath := "readonly.txt"
			absPath := filepath.Join(fixture.Root, testPath)
			require.NoError(t, os.WriteFile(absPath, []byte("content"), 0444), "Failed to create readonly file")

			err := fixture.VFS.DeleteFile(testPath, false, true)
			require.NoError(t, err, "Failed to delete readonly file with force")

			// Verify file was deleted
			_, err = os.Stat(absPath)
			assert.True(t, os.IsNotExist(err), "Expected file to be deleted")
		})

		t.Run("MockVFS", func(t *testing.T) {
			fixture := setupMockFixture(t)
			defer fixture.Cleanup()

			testPath := "readonly.txt"
			require.NoError(t, fixture.VFS.WriteFile(testPath, []byte("content")), "Failed to create test file")

			err := fixture.VFS.DeleteFile(testPath, false, true)
			require.NoError(t, err, "Failed to delete file with force")

			// Verify file was deleted
			_, err = fixture.VFS.ReadFile(testPath)
			assert.ErrorIs(t, err, ErrFileNotFound, "Expected file to be deleted")
		})
	})

	t.Run("PathTraversalAttack", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.DeleteFile("../../../tmp/important.txt", false, false)
			assert.ErrorIs(t, err, ErrPermissionDenied, "Expected ErrPermissionDenied for path traversal")
		})
	})
}

func TestListFiles(t *testing.T) {
	t.Run("ListEmptyDirectory", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			files, err := fixture.VFS.ListFiles(".", false)
			require.NoError(t, err, "Failed to list files")
			assert.Empty(t, files, "Expected empty list")
		})
	})

	t.Run("ListDirectoryWithFiles", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			expectedFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
			for _, f := range expectedFiles {
				require.NoError(t, fixture.VFS.WriteFile(f, []byte("content")), "Failed to create test file")
			}

			result, err := fixture.VFS.ListFiles(".", false)
			require.NoError(t, err, "Failed to list files")
			assert.Len(t, result, len(expectedFiles), "File count mismatch")

			// Verify all files are present
			for _, f := range expectedFiles {
				assert.Contains(t, result, f, "Expected file not found in result")
			}
		})
	})

	t.Run("ListNonRecursive", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create nested structure
			require.NoError(t, fixture.VFS.WriteFile("file1.txt", []byte("content")), "Failed to create test file")
			require.NoError(t, fixture.VFS.WriteFile("subdir/file2.txt", []byte("content")), "Failed to create test file")

			result, err := fixture.VFS.ListFiles(".", false)
			require.NoError(t, err, "Failed to list files")
			assert.Len(t, result, 2, "Expected 2 entries")

			// Should contain file1.txt and subdir, but not subdir/file2.txt
			assert.Contains(t, result, "file1.txt", "Expected file1.txt in result")
			assert.Contains(t, result, "subdir", "Expected subdir in result")
		})
	})

	t.Run("ListRecursive", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create nested structure
			require.NoError(t, fixture.VFS.WriteFile("file1.txt", []byte("content")), "Failed to create test file")
			require.NoError(t, fixture.VFS.WriteFile("subdir/file2.txt", []byte("content")), "Failed to create test file")
			require.NoError(t, fixture.VFS.WriteFile("subdir/nested/file3.txt", []byte("content")), "Failed to create test file")

			result, err := fixture.VFS.ListFiles(".", true)
			require.NoError(t, err, "Failed to list files")

			// Should contain all files and directories
			expected := []string{"file1.txt", "subdir", "subdir/file2.txt", "subdir/nested", "subdir/nested/file3.txt"}
			assert.Len(t, result, len(expected), "Entry count mismatch")
		})
	})

	t.Run("ListNonExistentDirectory", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			_, err := fixture.VFS.ListFiles("nonexistent", false)
			assert.ErrorIs(t, err, ErrFileNotFound)
		})
	})

	t.Run("ListFile", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testPath := "file.txt"
			require.NoError(t, fixture.VFS.WriteFile(testPath, []byte("content")), "Failed to create test file")

			_, err := fixture.VFS.ListFiles(testPath, false)
			assert.ErrorIs(t, err, ErrNotADir)
		})
	})

	t.Run("PathTraversalAttack", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			_, err := fixture.VFS.ListFiles("../../../etc", false)
			assert.ErrorIs(t, err, ErrPermissionDenied, "Expected ErrPermissionDenied for path traversal")
		})
	})
}

func TestFindFiles(t *testing.T) {
	t.Run("FindWithSimplePattern", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			files := []string{"test1.txt", "test2.txt", "other.log"}
			for _, f := range files {
				require.NoError(t, fixture.VFS.WriteFile(f, []byte("content")), "Failed to create test file")
			}

			result, err := fixture.VFS.FindFiles("*.txt", false)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 2, "Expected 2 matching files")
		})
	})

	t.Run("FindRecursive", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create nested structure
			require.NoError(t, fixture.VFS.WriteFile("test1.txt", []byte("content")), "Failed to create test file")
			require.NoError(t, fixture.VFS.WriteFile("subdir/test2.txt", []byte("content")), "Failed to create test file")
			require.NoError(t, fixture.VFS.WriteFile("subdir/nested/test3.txt", []byte("content")), "Failed to create test file")
			require.NoError(t, fixture.VFS.WriteFile("other.log", []byte("content")), "Failed to create test file")

			result, err := fixture.VFS.FindFiles("test*.txt", true)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 3, "Expected 3 matching files")
		})
	})

	t.Run("FindWithWildcard", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			files := []string{"file1.go", "file2.go", "test.txt"}
			for _, f := range files {
				require.NoError(t, fixture.VFS.WriteFile(f, []byte("content")), "Failed to create test file")
			}

			result, err := fixture.VFS.FindFiles("*.go", false)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 2, "Expected 2 matching files")
		})
	})

	t.Run("FindNoMatches", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("file.txt", []byte("content")), "Failed to create test file")

			result, err := fixture.VFS.FindFiles("*.go", false)
			require.NoError(t, err, "Failed to find files")
			assert.Empty(t, result, "Expected no matching files")
		})
	})

	t.Run("FindEmptyQuery", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			_, err := fixture.VFS.FindFiles("", false)
			assert.ErrorIs(t, err, ErrInvalidPath, "Expected ErrInvalidPath for empty query")
		})
	})

	t.Run("FindWithCharacterClass", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			files := []string{"file1.txt", "file2.txt", "filea.txt"}
			for _, f := range files {
				require.NoError(t, fixture.VFS.WriteFile(f, []byte("content")), "Failed to create test file")
			}

			result, err := fixture.VFS.FindFiles("file[0-9].txt", false)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 2, "Expected 2 matching files")
		})
	})
}

func TestInterfaceCompliance(t *testing.T) {
	// This test ensures LocalVFS and MockVFS implement the VFS interface
	var _ VFS = (*LocalVFS)(nil)
	var _ VFS = (*MockVFS)(nil)
}

func TestMockVFSPrepopulation(t *testing.T) {
	t.Run("PrepopulateFromDirectory", func(t *testing.T) {
		// Create a temporary directory with test files
		tempDir, err := os.MkdirTemp("", "vfs-prepopulate-test-*")
		require.NoError(t, err, "Failed to create temp directory")
		defer os.RemoveAll(tempDir)

		// Create test structure
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0644), "Failed to create file1")
		require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755), "Failed to create subdir")
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "subdir", "file2.txt"), []byte("content2"), 0644), "Failed to create file2")

		// Create MockVFS from directory
		mockVFS, err := NewMockVFSFromDir(tempDir)
		require.NoError(t, err, "Failed to create MockVFS from directory")

		// Verify files were prepopulated
		content1, err := mockVFS.ReadFile("file1.txt")
		require.NoError(t, err, "Failed to read file1.txt")
		assert.Equal(t, []byte("content1"), content1, "Content mismatch for file1.txt")

		content2, err := mockVFS.ReadFile("subdir/file2.txt")
		require.NoError(t, err, "Failed to read file2.txt")
		assert.Equal(t, []byte("content2"), content2, "Content mismatch for file2.txt")

		// Verify directory structure
		files, err := mockVFS.ListFiles(".", true)
		require.NoError(t, err, "Failed to list files")
		assert.Contains(t, files, "file1.txt", "Expected file1.txt in listing")
		assert.Contains(t, files, "subdir", "Expected subdir in listing")
		assert.Contains(t, files, "subdir/file2.txt", "Expected subdir/file2.txt in listing")
	})

	t.Run("PrepopulateFromNonExistentDirectory", func(t *testing.T) {
		_, err := NewMockVFSFromDir("/path/that/does/not/exist")
		assert.ErrorIs(t, err, ErrFileNotFound, "Expected ErrFileNotFound for non-existent directory")
	})

	t.Run("PrepopulateFromFile", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "vfs-prepopulate-file-*")
		require.NoError(t, err, "Failed to create temp file")
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		_, err = NewMockVFSFromDir(tempFile.Name())
		assert.ErrorIs(t, err, ErrNotADir, "Expected ErrNotADir when prepopulating from a file")
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("ConcurrentReads", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testPath := "concurrent.txt"
			testContent := []byte("concurrent content")
			require.NoError(t, fixture.VFS.WriteFile(testPath, testContent), "Failed to create test file")

			// Launch multiple concurrent reads
			done := make(chan bool)
			for i := 0; i < 10; i++ {
				go func() {
					content, err := fixture.VFS.ReadFile(testPath)
					assert.NoError(t, err, "Concurrent read failed")
					assert.Equal(t, testContent, content, "Concurrent read got wrong content")
					done <- true
				}()
			}

			// Wait for all reads to complete
			for i := 0; i < 10; i++ {
				<-done
			}
		})
	})

	t.Run("ConcurrentWrites", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Launch multiple concurrent writes to different files
			done := make(chan bool)
			for i := 0; i < 10; i++ {
				i := i // capture loop variable
				go func() {
					testPath := filepath.Join("concurrent", string(rune('a'+i))+".txt")
					testContent := []byte("content " + string(rune('0'+i)))
					err := fixture.VFS.WriteFile(testPath, testContent)
					assert.NoError(t, err, "Concurrent write failed")
					done <- true
				}()
			}

			// Wait for all writes to complete
			for i := 0; i < 10; i++ {
				<-done
			}

			// Verify all files were written
			files, err := fixture.VFS.ListFiles("concurrent", false)
			require.NoError(t, err, "Failed to list files")
			assert.Len(t, files, 10, "Expected 10 files")
		})
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("DotPath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create a file
			require.NoError(t, fixture.VFS.WriteFile("test.txt", []byte("content")), "Failed to create test file")

			// List using "." path
			files, err := fixture.VFS.ListFiles(".", false)
			require.NoError(t, err, "Failed to list files")
			assert.Len(t, files, 1, "Expected 1 file")
		})
	})

	t.Run("PathWithDotComponents", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create nested structure
			require.NoError(t, fixture.VFS.WriteFile("dir/file.txt", []byte("content")), "Failed to create test file")

			// Read using path with . components
			content, err := fixture.VFS.ReadFile("dir/./file.txt")
			require.NoError(t, err, "Failed to read file")
			assert.Equal(t, []byte("content"), content, "Content mismatch")
		})
	})

	t.Run("EmptyFileContent", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			testPath := "empty.txt"
			require.NoError(t, fixture.VFS.WriteFile(testPath, []byte{}), "Failed to write empty file")

			content, err := fixture.VFS.ReadFile(testPath)
			require.NoError(t, err, "Failed to read empty file")
			assert.Empty(t, content, "Expected empty content")
		})
	})

	t.Run("LargeFileContent", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create 1MB of data
			largeContent := make([]byte, 1024*1024)
			for i := range largeContent {
				largeContent[i] = byte(i % 256)
			}

			testPath := "large.bin"
			require.NoError(t, fixture.VFS.WriteFile(testPath, largeContent), "Failed to write large file")

			content, err := fixture.VFS.ReadFile(testPath)
			require.NoError(t, err, "Failed to read large file")
			assert.Len(t, content, len(largeContent), "Content length mismatch")
		})
	})
}
