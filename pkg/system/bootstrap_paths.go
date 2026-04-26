package system

import (
	"fmt"
	"os"
	"path/filepath"
)

// BuildConfigPath builds a config path hierarchy string from base and optional custom paths.
func BuildConfigPath(projectConfig, customConfigPath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("BuildConfigPath() [bootstrap_paths.go]: failed to get user home directory: %w", err)
	}

	projectConfigPath := "@PROJ/.csw/config"
	if projectConfig != "" {
		info, err := os.Stat(projectConfig)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("BuildConfigPath() [bootstrap_paths.go]: project config directory does not exist: %s", projectConfig)
			}
			return "", fmt.Errorf("BuildConfigPath() [bootstrap_paths.go]: failed to access project config directory %s: %w", projectConfig, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("BuildConfigPath() [bootstrap_paths.go]: project config path is not a directory: %s", projectConfig)
		}
		projectConfigPath = projectConfig
	}

	configPathStr := "@DEFAULTS:" + filepath.Join(homeDir, ".config", "csw") + ":" + projectConfigPath

	if customConfigPath != "" {
		if err := ValidateConfigPaths(customConfigPath); err != nil {
			return "", err
		}
		configPathStr = configPathStr + ":" + customConfigPath
	}

	return configPathStr, nil
}

// ValidateConfigPaths validates that all paths in a colon-separated string exist and are directories.
func ValidateConfigPaths(configPath string) error {
	pathComponents := filepath.SplitList(configPath)
	for _, pathComponent := range pathComponents {
		if pathComponent == "" {
			continue
		}
		info, err := os.Stat(pathComponent)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("ValidateConfigPaths() [bootstrap_paths.go]: config path does not exist: %s", pathComponent)
			}
			return fmt.Errorf("ValidateConfigPaths() [bootstrap_paths.go]: failed to access config path %s: %w", pathComponent, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("ValidateConfigPaths() [bootstrap_paths.go]: config path is not a directory: %s", pathComponent)
		}
	}
	return nil
}

// ResolveWorkDir resolves the working directory from an optional path argument.
func ResolveWorkDir(dirPath string) (string, error) {
	if dirPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("ResolveWorkDir() [bootstrap_paths.go]: failed to get current working directory: %w", err)
		}
		projectDir := findNearestProjectDir(wd)
		return projectDir, nil
	}

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", fmt.Errorf("ResolveWorkDir() [bootstrap_paths.go]: failed to resolve directory path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("ResolveWorkDir() [bootstrap_paths.go]: failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("ResolveWorkDir() [bootstrap_paths.go]: path is not a directory: %s", dirPath)
	}
	return absPath, nil
}

// findNearestProjectDir searches upwards from startDir for a directory containing .csw or .cswdata.
func findNearestProjectDir(startDir string) string {
	currentDir := startDir
	for {
		if hasProjectMarker(currentDir) {
			return currentDir
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return startDir
		}

		currentDir = parentDir
	}
}

// hasProjectMarker returns true when dirPath contains .csw or .cswdata directory.
func hasProjectMarker(dirPath string) bool {
	projectMarkers := []string{".csw", ".cswdata"}
	for _, marker := range projectMarkers {
		markerPath := filepath.Join(dirPath, marker)
		markerInfo, err := os.Stat(markerPath)
		if err != nil {
			continue
		}
		if markerInfo.IsDir() {
			return true
		}
	}

	return false
}
