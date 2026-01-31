package vfs

import (
	"os"
	"path/filepath"
	"strings"
)

// BuildHidePatterns builds a list of hide patterns by merging:
// 1. basePatterns (from role configuration)
// 2. Patterns from .cswignore or .gitignore file in the project root
//
// The function reads .cswignore first, and if it doesn't exist, falls back to .gitignore.
// If neither file exists, only basePatterns are returned.
func BuildHidePatterns(projectRoot string, basePatterns []string) ([]string, error) {
	result := make([]string, 0, len(basePatterns)+50)

	// Add base patterns first
	result = append(result, basePatterns...)

	// Try to read .cswignore first
	cswignorePath := filepath.Join(projectRoot, ".cswignore")
	content, err := os.ReadFile(cswignorePath)

	if err != nil {
		if !os.IsNotExist(err) {
			// Error reading file (not just missing)
			return nil, err
		}

		// .cswignore doesn't exist, try .gitignore
		gitignorePath := filepath.Join(projectRoot, ".gitignore")
		content, err = os.ReadFile(gitignorePath)

		if err != nil {
			if os.IsNotExist(err) {
				// Neither file exists, return base patterns only
				return result, nil
			}
			// Error reading .gitignore
			return nil, err
		}
	}

	// Parse the ignore file content and extract patterns
	patterns := parseIgnoreFile(string(content))
	result = append(result, patterns...)

	return result, nil
}

// parseIgnoreFile parses a .gitignore/.cswignore file and returns a list of patterns
func parseIgnoreFile(content string) []string {
	// Split content into lines and parse each line
	var patterns []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		// Trim whitespace (including \r for Windows line endings)
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || line[0] == '#' {
			continue
		}

		patterns = append(patterns, line)
	}

	return patterns
}
