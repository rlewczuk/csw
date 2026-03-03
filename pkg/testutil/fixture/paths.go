// Package fixture provides reusable test fixture helpers.
package fixture

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/testutil/cfg"
)

// ProjectRoot returns the absolute path to the repository root by locating go.mod.
func ProjectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	dir := cwd
	for {
		candidate := filepath.Join(dir, "go.mod")
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}

		dir = parent
	}
}

// ProjectPath returns an absolute path under the repository root.
func ProjectPath(pathParts ...string) string {
	parts := make([]string, 0, len(pathParts)+1)
	parts = append(parts, ProjectRoot())
	parts = append(parts, pathParts...)
	return filepath.Join(parts...)
}

// ProjectTmpDir returns projectRoot/tmp and ensures it exists.
func ProjectTmpDir(t *testing.T) string {
	t.Helper()

	tmpDir := ProjectPath("tmp")
	err := os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		t.Fatalf("fixture.ProjectTmpDir(): failed to create tmp dir: %v", err)
	}

	return tmpDir
}

// MkProjectTempDir creates a temporary directory inside projectRoot/tmp.
func MkProjectTempDir(t *testing.T, pattern string) string {
	t.Helper()
	_ = ProjectTmpDir(t)
	return cfg.MkTempDir(t, ProjectRoot(), pattern)
}
