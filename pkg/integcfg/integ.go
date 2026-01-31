// Package integcfg provides integration test configuration helpers.
package integcfg

import (
	"os"
	"path/filepath"
	"strings"
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
