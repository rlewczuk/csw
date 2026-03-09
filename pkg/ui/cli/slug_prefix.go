package cli

import (
	"fmt"
	"strings"
)

const defaultCLISlug = "main"

// normalizeCLISlug returns a normalized CLI slug with fallback.
func normalizeCLISlug(slug string) string {
	normalized := strings.TrimSpace(slug)
	if normalized == "" {
		return defaultCLISlug
	}

	return normalized
}

// addCLISlugPrefix prefixes every non-empty output line with slug marker.
func addCLISlugPrefix(slug string, message string) string {
	prefix := fmt.Sprintf("*%s* ", normalizeCLISlug(slug))
	lines := strings.Split(message, "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = prefix + line
	}

	return strings.Join(lines, "\n")
}
