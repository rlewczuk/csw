package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWorkDir_UsesExplicitDirOverride(t *testing.T) {
	overrideDir := t.TempDir()

	resolved, err := ResolveWorkDir(overrideDir)
	require.NoError(t, err)

	expected, err := filepath.Abs(overrideDir)
	require.NoError(t, err)
	assert.Equal(t, expected, resolved)
}

func TestResolveWorkDir_FindsNearestProjectDirWithDotCsw(t *testing.T) {
	rootDir := t.TempDir()
	markerDir := filepath.Join(rootDir, ".csw")
	require.NoError(t, os.MkdirAll(markerDir, 0755))

	nestedDir := filepath.Join(rootDir, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})
	require.NoError(t, os.Chdir(nestedDir))

	resolved, err := ResolveWorkDir("")
	require.NoError(t, err)
	assert.Equal(t, rootDir, resolved)
}

func TestResolveWorkDir_FindsNearestProjectDirWithDotCswdata(t *testing.T) {
	rootDir := t.TempDir()
	markerDir := filepath.Join(rootDir, ".cswdata")
	require.NoError(t, os.MkdirAll(markerDir, 0755))

	nestedDir := filepath.Join(rootDir, "deep", "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})
	require.NoError(t, os.Chdir(nestedDir))

	resolved, err := ResolveWorkDir("")
	require.NoError(t, err)
	assert.Equal(t, rootDir, resolved)
}

func TestResolveWorkDir_ReturnsCurrentDirWhenNoProjectMarkerFound(t *testing.T) {
	baseDir := t.TempDir()
	nestedDir := filepath.Join(baseDir, "x", "y")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})
	require.NoError(t, os.Chdir(nestedDir))

	resolved, err := ResolveWorkDir("")
	require.NoError(t, err)
	assert.Equal(t, nestedDir, resolved)
}

func TestResolveConfigPathForProjectRoot(t *testing.T) {
	t.Run("replaces @PROJ entries with project root", func(t *testing.T) {
		projectRoot := filepath.Join(t.TempDir(), "shadow")
		configPath := "@DEFAULTS:~/.config/csw:@PROJ/.csw/config:/custom"

		resolvedConfigPath, err := ResolveConfigPathForProjectRoot(configPath, projectRoot)
		require.NoError(t, err)
		assert.Equal(t, "@DEFAULTS:~/.config/csw:"+filepath.Join(projectRoot, ".csw", "config")+":/custom", resolvedConfigPath)
	})

	t.Run("keeps config path unchanged when project root is empty", func(t *testing.T) {
		configPath := "@DEFAULTS:@PROJ/.csw/config"

		resolvedConfigPath, err := ResolveConfigPathForProjectRoot(configPath, "")
		require.NoError(t, err)
		assert.Equal(t, configPath, resolvedConfigPath)
	})
}
