package vfs

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			assert.ErrorIs(t, err, apis.ErrFileNotFound, "Expected source file to be deleted")
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
			assert.ErrorIs(t, err, apis.ErrFileNotFound, "Expected original file to be deleted")
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
			assert.ErrorIs(t, err, apis.ErrFileNotFound, "Expected source file to be deleted")
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
			assert.ErrorIs(t, err, apis.ErrFileNotFound, "Expected source file to be deleted")
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
			assert.ErrorIs(t, err, apis.ErrFileNotFound, "Expected old directory to be moved")
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
			assert.ErrorIs(t, err, apis.ErrFileNotFound, "Expected old directory to be moved")
		})
	})

	t.Run("MoveNonExistentFile", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.MoveFile("nonexistent.txt", "newname.txt")
			assert.ErrorIs(t, err, apis.ErrFileNotFound, "Expected ErrFileNotFound for non-existent source")
		})
	})

	t.Run("MoveToExistingDestination", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create source and destination files
			require.NoError(t, fixture.VFS.WriteFile("src.txt", []byte("source")), "Failed to create source file")
			require.NoError(t, fixture.VFS.WriteFile("dst.txt", []byte("destination")), "Failed to create destination file")

			// Try to move to existing destination
			err := fixture.VFS.MoveFile("src.txt", "dst.txt")
			assert.ErrorIs(t, err, apis.ErrFileExists, "Expected ErrFileExists for existing destination")
		})
	})

	t.Run("MoveWithInvalidSourcePath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.MoveFile("../../../etc/passwd", "newname.txt")
			assert.ErrorIs(t, err, apis.ErrPermissionDenied, "Expected ErrPermissionDenied for path traversal")
		})
	})

	t.Run("MoveWithInvalidDestinationPath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			require.NoError(t, fixture.VFS.WriteFile("file.txt", []byte("content")), "Failed to create test file")

			err := fixture.VFS.MoveFile("file.txt", "../../../tmp/evil.txt")
			assert.ErrorIs(t, err, apis.ErrPermissionDenied, "Expected ErrPermissionDenied for path traversal")
		})
	})

	t.Run("MoveWithAbsoluteSourcePath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.MoveFile("/etc/passwd", "newname.txt")
			assert.ErrorIs(t, err, apis.ErrPermissionDenied, "Expected ErrInvalidPath for absolute source path")
		})
	})

	t.Run("MoveWithAbsoluteDestinationPath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			require.NoError(t, fixture.VFS.WriteFile("file.txt", []byte("content")), "Failed to create test file")

			err := fixture.VFS.MoveFile("file.txt", "/tmp/newfile.txt")
			assert.ErrorIs(t, err, apis.ErrPermissionDenied, "Expected ErrInvalidPath for absolute destination path")
		})
	})

	t.Run("MoveWithEmptySourcePath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			err := fixture.VFS.MoveFile("", "newname.txt")
			assert.ErrorIs(t, err, apis.ErrInvalidPath, "Expected ErrInvalidPath for empty source path")
		})
	})

	t.Run("MoveWithEmptyDestinationPath", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			require.NoError(t, fixture.VFS.WriteFile("file.txt", []byte("content")), "Failed to create test file")

			err := fixture.VFS.MoveFile("file.txt", "")
			assert.ErrorIs(t, err, apis.ErrInvalidPath, "Expected ErrInvalidPath for empty destination path")
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
			assert.ErrorIs(t, err, apis.ErrFileNotFound, "Expected source file to be deleted")
		})
	})
}
