package vfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildHidePatterns(t *testing.T) {
	t.Run("NoIgnoreFile", func(t *testing.T) {
		// Create a temporary directory
		tempDir, err := os.MkdirTemp("", "build-hide-patterns-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		basePatterns := []string{"node_modules/", ".git/"}
		patterns, err := BuildHidePatterns(tempDir, basePatterns)
		require.NoError(t, err)

		// Should only return base patterns
		assert.Equal(t, basePatterns, patterns)
	})

	t.Run("WithCswignore", func(t *testing.T) {
		// Create a temporary directory
		tempDir, err := os.MkdirTemp("", "build-hide-patterns-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create .cswignore file
		cswignoreContent := `# Comment
*.log
tmp/
dist/
`
		cswignorePath := filepath.Join(tempDir, ".cswignore")
		err = os.WriteFile(cswignorePath, []byte(cswignoreContent), 0644)
		require.NoError(t, err)

		basePatterns := []string{"node_modules/", ".git/"}
		patterns, err := BuildHidePatterns(tempDir, basePatterns)
		require.NoError(t, err)

		// Should contain base patterns + patterns from .cswignore
		expected := []string{"node_modules/", ".git/", "*.log", "tmp/", "dist/"}
		assert.Equal(t, expected, patterns)
	})

	t.Run("WithGitignore", func(t *testing.T) {
		// Create a temporary directory
		tempDir, err := os.MkdirTemp("", "build-hide-patterns-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create .gitignore file (no .cswignore)
		gitignoreContent := `# Generated files
build/
*.pyc
`
		gitignorePath := filepath.Join(tempDir, ".gitignore")
		err = os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
		require.NoError(t, err)

		basePatterns := []string{"node_modules/"}
		patterns, err := BuildHidePatterns(tempDir, basePatterns)
		require.NoError(t, err)

		// Should contain base patterns + patterns from .gitignore
		expected := []string{"node_modules/", "build/", "*.pyc"}
		assert.Equal(t, expected, patterns)
	})

	t.Run("PrefersCswignoreOverGitignore", func(t *testing.T) {
		// Create a temporary directory
		tempDir, err := os.MkdirTemp("", "build-hide-patterns-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create both .cswignore and .gitignore
		cswignoreContent := `*.csw`
		cswignorePath := filepath.Join(tempDir, ".cswignore")
		err = os.WriteFile(cswignorePath, []byte(cswignoreContent), 0644)
		require.NoError(t, err)

		gitignoreContent := `*.git`
		gitignorePath := filepath.Join(tempDir, ".gitignore")
		err = os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
		require.NoError(t, err)

		basePatterns := []string{"base/"}
		patterns, err := BuildHidePatterns(tempDir, basePatterns)
		require.NoError(t, err)

		// Should use .cswignore, not .gitignore
		expected := []string{"base/", "*.csw"}
		assert.Equal(t, expected, patterns)
	})

	t.Run("EmptyBasePatterns", func(t *testing.T) {
		// Create a temporary directory
		tempDir, err := os.MkdirTemp("", "build-hide-patterns-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create .cswignore file
		cswignoreContent := `tmp/`
		cswignorePath := filepath.Join(tempDir, ".cswignore")
		err = os.WriteFile(cswignorePath, []byte(cswignoreContent), 0644)
		require.NoError(t, err)

		patterns, err := BuildHidePatterns(tempDir, nil)
		require.NoError(t, err)

		// Should only contain patterns from .cswignore
		expected := []string{"tmp/"}
		assert.Equal(t, expected, patterns)
	})

	t.Run("IgnoreEmptyLinesAndComments", func(t *testing.T) {
		// Create a temporary directory
		tempDir, err := os.MkdirTemp("", "build-hide-patterns-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create .cswignore with empty lines and comments
		cswignoreContent := `
# This is a comment
tmp/

# Another comment
*.log

`
		cswignorePath := filepath.Join(tempDir, ".cswignore")
		err = os.WriteFile(cswignorePath, []byte(cswignoreContent), 0644)
		require.NoError(t, err)

		patterns, err := BuildHidePatterns(tempDir, nil)
		require.NoError(t, err)

		// Should only contain actual patterns, not comments or empty lines
		expected := []string{"tmp/", "*.log"}
		assert.Equal(t, expected, patterns)
	})
}

func TestBuildHidePatternsIntegrationWithLocalVFS(t *testing.T) {
	t.Run("HideFilesBasedOnCswignore", func(t *testing.T) {
		// Create a temporary directory
		tempDir, err := os.MkdirTemp("", "hide-patterns-integration-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create .cswignore
		cswignoreContent := `*.log
tmp/
`
		cswignorePath := filepath.Join(tempDir, ".cswignore")
		err = os.WriteFile(cswignorePath, []byte(cswignoreContent), 0644)
		require.NoError(t, err)

		// Create some test files
		err = os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("content"), 0644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(tempDir, "debug.log"), []byte("log"), 0644)
		require.NoError(t, err)

		err = os.MkdirAll(filepath.Join(tempDir, "tmp"), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "tmp", "temp.txt"), []byte("temp"), 0644)
		require.NoError(t, err)

		// Build hide patterns and create VFS
		basePatterns := []string{"node_modules/"}
		hidePatterns, err := BuildHidePatterns(tempDir, basePatterns)
		require.NoError(t, err)

		vfs, err := NewLocalVFS(tempDir, hidePatterns)
		require.NoError(t, err)

		// List files
		files, err := vfs.ListFiles(".", false)
		require.NoError(t, err)

		// Should only see file.txt and .cswignore, not debug.log or tmp/
		assert.Contains(t, files, "file.txt")
		assert.Contains(t, files, ".cswignore") // .cswignore is not hidden by default
		assert.NotContains(t, files, "debug.log")
		assert.NotContains(t, files, "tmp")
	})
}
