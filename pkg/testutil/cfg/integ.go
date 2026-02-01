// Package integcfg provides integration test configuration helpers.
package cfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Dir returns the absolute path to the _integ directory at project root.
func Dir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "_integ"
	}

	dir := cwd
	for {
		candidate := filepath.Join(dir, "_integ")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			return "_integ"
		}
		dir = parent
	}
}

// ReadFile reads a file from the _integ directory and returns its trimmed content.
// Returns empty string if file doesn't exist or is empty.
func ReadFile(filename string) string {
	path := filepath.Join(Dir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// TestEnabled checks if a feature is enabled by reading the corresponding .enabled file.
// Returns true if the file exists and contains "yes" (case insensitive).
func TestEnabled(name string) bool {
	return strings.EqualFold(ReadFile(name+".enabled"), "yes") || strings.EqualFold(ReadFile("all.enabled"), "yes")
}

// MkTempDir creates a temporary directory inside projectRoot/tmp with the given pattern.
// The directory is automatically cleaned up after the test completes.
// On test failure, the directory path is logged and the directory is preserved for inspection.
// On test success, the directory is removed.
//
// Usage:
//
//	tmpDir := cfg.MkTempDir(t, projectRoot, "mytest_*")
//	// Use tmpDir for test files
func MkTempDir(t *testing.T, projectRoot, pattern string) string {
	t.Helper()

	baseDir := filepath.Join(projectRoot, "tmp")
	tmpDir, err := os.MkdirTemp(baseDir, pattern)
	if err != nil {
		t.Fatalf("MkTempDir: failed to create temporary directory: %v", err)
	}

	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Temporary directory preserved for inspection: %s", tmpDir)
		} else {
			os.RemoveAll(tmpDir)
		}
	})

	return tmpDir
}
