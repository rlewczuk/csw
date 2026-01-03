// Package integ provides integration test configuration helpers.
package testutil

import (
	"os"
	"path/filepath"
	"strings"
)

// IntegCfgDir returns the absolute path to the _integ directory at project root.
func IntegCfgDir() string {
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

// IntegCfgReadFile reads a file from the _integ directory and returns its trimmed content.
// Returns empty string if file doesn't exist or is empty.
func IntegCfgReadFile(filename string) string {
	path := filepath.Join(IntegCfgDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// IntegTestEnabled checks if a feature is enabled by reading the corresponding .enabled file.
// Returns true if the file exists and contains "yes" (case insensitive).
func IntegTestEnabled(name string) bool {
	return strings.EqualFold(IntegCfgReadFile(name+".enabled"), "yes") || strings.EqualFold(IntegCfgReadFile("all.enabled"), "yes")
}
