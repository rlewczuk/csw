package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildConfigPath_DefaultProjectConfig(t *testing.T) {
	// When projectConfig is empty, should use default @PROJ/.csw/config
	path, err := system.BuildConfigPath("", "")
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	expectedSuffix := "@DEFAULTS:" + filepath.Join(homeDir, ".config", "csw") + ":@PROJ/.csw/config"
	assert.Equal(t, expectedSuffix, path)
}

func TestBuildConfigPath_CustomProjectConfig(t *testing.T) {
	// Create a temporary directory to use as project config
	tmpDir, err := os.MkdirTemp("", "csw-project-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// When projectConfig is provided, should use it instead of default
	path, err := system.BuildConfigPath(tmpDir, "")
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	expectedSuffix := "@DEFAULTS:" + filepath.Join(homeDir, ".config", "csw") + ":" + tmpDir
	assert.Equal(t, expectedSuffix, path)
}

func TestBuildConfigPath_ProjectConfigNotExist(t *testing.T) {
	// When projectConfig doesn't exist, should return error
	nonExistentPath := "/non/existent/path/that/does/not/exist"
	_, err := system.BuildConfigPath(nonExistentPath, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project config directory does not exist")
}

func TestBuildConfigPath_ProjectConfigIsFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "csw-project-config-*")
	require.NoError(t, err)
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpFilePath)

	// When projectConfig is a file, should return error
	_, err = system.BuildConfigPath(tmpFilePath, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project config path is not a directory")
}

func TestBuildConfigPath_WithCustomConfigPath(t *testing.T) {
	// Create temporary directories for both project config and custom config
	tmpProjectDir, err := os.MkdirTemp("", "csw-project-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpProjectDir)

	tmpCustomDir, err := os.MkdirTemp("", "csw-custom-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpCustomDir)

	// When both projectConfig and customConfigPath are provided
	path, err := system.BuildConfigPath(tmpProjectDir, tmpCustomDir)
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	expectedSuffix := "@DEFAULTS:" + filepath.Join(homeDir, ".config", "csw") + ":" + tmpProjectDir + ":" + tmpCustomDir
	assert.Equal(t, expectedSuffix, path)
}

func TestBuildConfigPath_CustomConfigPathNotExist(t *testing.T) {
	// Create project config directory
	tmpProjectDir, err := os.MkdirTemp("", "csw-project-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpProjectDir)

	// When customConfigPath doesn't exist, should return error
	nonExistentPath := "/non/existent/path/that/does/not/exist"
	_, err = system.BuildConfigPath(tmpProjectDir, nonExistentPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config path does not exist")
}

func TestValidateConfigPaths_ValidPath(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = system.ValidateConfigPaths(tmpDir)
	assert.NoError(t, err)
}

func TestValidateConfigPaths_InvalidPath(t *testing.T) {
	// Test with non-existent path
	err := system.ValidateConfigPaths("/non/existent/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config path does not exist")
}

func TestValidateConfigPaths_FileNotDirectory(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "csw-config-*")
	require.NoError(t, err)
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpFilePath)

	err = system.ValidateConfigPaths(tmpFilePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config path is not a directory")
}

func TestValidateConfigPaths_EmptyPath(t *testing.T) {
	// Empty paths should be skipped
	err := system.ValidateConfigPaths("")
	assert.NoError(t, err)
}

func TestValidateConfigPaths_MultiplePaths(t *testing.T) {
	// Create multiple temporary directories
	tmpDir1, err := os.MkdirTemp("", "csw-config-1-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := os.MkdirTemp("", "csw-config-2-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir2)

	// Test with multiple paths (colon-separated on Unix)
	pathList := tmpDir1 + string(filepath.ListSeparator) + tmpDir2
	err = system.ValidateConfigPaths(pathList)
	assert.NoError(t, err)
}
