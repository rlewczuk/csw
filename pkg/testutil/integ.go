// Package testutil provides integration test configuration helpers.
package testutil

import (
	"github.com/rlewczuk/csw/pkg/testutil/cfg"
)

// IntegCfgDir returns the absolute path to the _integ directory at project root.
func IntegCfgDir() string {
	return cfg.Dir()
}

// IntegCfgReadFile reads a file from the _integ directory and returns its trimmed content.
// Returns empty string if file doesn't exist or is empty.
func IntegCfgReadFile(filename string) string {
	return cfg.ReadFile(filename)
}

// IntegTestEnabled checks if a feature is enabled by reading the corresponding .enabled file.
// Returns true if the file exists and contains "yes" (case insensitive).
func IntegTestEnabled(name string) bool {
	return cfg.TestEnabled(name)
}
