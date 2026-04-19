package vfs

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			assert.ErrorIs(t, err, apis.ErrInvalidPath, "Expected ErrInvalidPath for empty query")
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

	t.Run("GlobPatternStar", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("foo.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("bar.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("baz.log", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/nested.txt", []byte("content")), "Failed to create file")

			// * should match any characters except /
			result, err := fixture.VFS.FindFiles("*.txt", false)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 2, "Expected 2 matching files")
			assert.Contains(t, result, "foo.txt")
			assert.Contains(t, result, "bar.txt")
		})
	})

	t.Run("GlobPatternStarRecursive", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("foo.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/bar.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/baz.log", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/sub/qux.txt", []byte("content")), "Failed to create file")

			// * should match any characters except / even in recursive mode
			result, err := fixture.VFS.FindFiles("*.txt", true)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 3, "Expected 3 matching files")
			assert.Contains(t, result, "foo.txt")
			assert.Contains(t, result, "dir/bar.txt")
			assert.Contains(t, result, "dir/sub/qux.txt")
		})
	})

	t.Run("GlobPatternQuestionMark", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("file1.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("file2.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("file10.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("filea.txt", []byte("content")), "Failed to create file")

			// ? should match exactly one character
			result, err := fixture.VFS.FindFiles("file?.txt", false)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 3, "Expected 3 matching files (file1, file2, filea)")
			assert.Contains(t, result, "file1.txt")
			assert.Contains(t, result, "file2.txt")
			assert.Contains(t, result, "filea.txt")
			assert.NotContains(t, result, "file10.txt")
		})
	})

	t.Run("GlobPatternCharacterSet", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("file1.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("file2.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("file3.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("filea.txt", []byte("content")), "Failed to create file")

			// [abc] should match any of the characters in the set
			result, err := fixture.VFS.FindFiles("file[12].txt", false)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 2, "Expected 2 matching files")
			assert.Contains(t, result, "file1.txt")
			assert.Contains(t, result, "file2.txt")
			assert.NotContains(t, result, "file3.txt")
		})
	})

	t.Run("GlobPatternCharacterRange", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("file1.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("file2.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("file3.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("file9.txt", []byte("content")), "Failed to create file")

			// [0-9] should match any digit
			result, err := fixture.VFS.FindFiles("file[1-3].txt", false)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 3, "Expected 3 matching files")
			assert.Contains(t, result, "file1.txt")
			assert.Contains(t, result, "file2.txt")
			assert.Contains(t, result, "file3.txt")
			assert.NotContains(t, result, "file9.txt")
		})
	})

	t.Run("GlobPatternDoubleStarRecursive", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("file.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/file.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/sub/file.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/sub/deep/file.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("other.log", []byte("content")), "Failed to create file")

			// **/file.txt should match file.txt at any depth
			result, err := fixture.VFS.FindFiles("**/file.txt", true)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 4, "Expected 4 matching files")
			assert.Contains(t, result, "file.txt")
			assert.Contains(t, result, "dir/file.txt")
			assert.Contains(t, result, "dir/sub/file.txt")
			assert.Contains(t, result, "dir/sub/deep/file.txt")
			assert.NotContains(t, result, "other.log")
		})
	})

	t.Run("GlobPatternDoubleStarPrefix", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("test.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/test.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/sub/test.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/sub/file.txt", []byte("content")), "Failed to create file")

			// **/*.txt should match all .txt files at any depth
			result, err := fixture.VFS.FindFiles("**/*.txt", true)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 4, "Expected 4 matching files")
			assert.Contains(t, result, "test.txt")
			assert.Contains(t, result, "dir/test.txt")
			assert.Contains(t, result, "dir/sub/test.txt")
			assert.Contains(t, result, "dir/sub/file.txt")
		})
	})

	t.Run("GlobPatternDoubleStarMiddle", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("src/main.go", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("src/util/helper.go", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("src/util/sub/deep.go", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("test/main_test.go", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("src/main.txt", []byte("content")), "Failed to create file")

			// src/**/*.go should match all .go files under src at any depth
			result, err := fixture.VFS.FindFiles("src/**/*.go", true)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 3, "Expected 3 matching files")
			assert.Contains(t, result, "src/main.go")
			assert.Contains(t, result, "src/util/helper.go")
			assert.Contains(t, result, "src/util/sub/deep.go")
			assert.NotContains(t, result, "test/main_test.go")
			assert.NotContains(t, result, "src/main.txt")
		})
	})

	t.Run("GlobPatternDoubleStarOnly", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("file1.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/file2.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/sub/file3.txt", []byte("content")), "Failed to create file")

			// ** should match everything at any depth
			result, err := fixture.VFS.FindFiles("**", true)
			require.NoError(t, err, "Failed to find files")
			// Should match all files and directories
			assert.GreaterOrEqual(t, len(result), 3, "Expected at least 3 matching files")
			assert.Contains(t, result, "file1.txt")
			assert.Contains(t, result, "dir/file2.txt")
			assert.Contains(t, result, "dir/sub/file3.txt")
		})
	})

	t.Run("GlobPatternMixedWildcards", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("test1.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("test2.log", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/test3.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/sub/file1.txt", []byte("content")), "Failed to create file")

			// **/test?.txt should match test<single-char>.txt at any depth
			result, err := fixture.VFS.FindFiles("**/test?.txt", true)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 2, "Expected 2 matching files")
			assert.Contains(t, result, "test1.txt")
			assert.Contains(t, result, "dir/test3.txt")
			assert.NotContains(t, result, "test2.log")
			assert.NotContains(t, result, "dir/sub/file1.txt")
		})
	})

	t.Run("GlobPatternNoRecursiveWithDoubleStar", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("file.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("dir/file.txt", []byte("content")), "Failed to create file")

			// When recursive=false, ** should still work but only in root
			result, err := fixture.VFS.FindFiles("**/*.txt", false)
			require.NoError(t, err, "Failed to find files")
			// Should only match files in root that match *.txt
			assert.Len(t, result, 1, "Expected 1 matching file")
			assert.Contains(t, result, "file.txt")
		})
	})

	t.Run("GlobPatternExactMatch", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("exact.txt", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("other.txt", []byte("content")), "Failed to create file")

			// Exact filename should match only that file
			result, err := fixture.VFS.FindFiles("exact.txt", false)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 1, "Expected 1 matching file")
			assert.Contains(t, result, "exact.txt")
		})
	})

	t.Run("GlobPatternPathWithStar", func(t *testing.T) {
		runTestWithBothVFS(t, func(t *testing.T, fixture *TestFixture) {
			// Create test files
			require.NoError(t, fixture.VFS.WriteFile("src/file1.go", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("src/file2.go", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("test/file3.go", []byte("content")), "Failed to create file")
			require.NoError(t, fixture.VFS.WriteFile("src/sub/file4.go", []byte("content")), "Failed to create file")

			// src/*.go should match .go files directly under src
			result, err := fixture.VFS.FindFiles("src/*.go", true)
			require.NoError(t, err, "Failed to find files")
			assert.Len(t, result, 2, "Expected 2 matching files")
			assert.Contains(t, result, "src/file1.go")
			assert.Contains(t, result, "src/file2.go")
			assert.NotContains(t, result, "test/file3.go")
			assert.NotContains(t, result, "src/sub/file4.go")
		})
	})
}
