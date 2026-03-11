package cli

import (
	"fmt"
	"regexp"
	"strings"
)

const defaultCLISlug = "main"
const cliGrayPrefixColor = "\x1b[90m"
const cliColorReset = "\x1b[0m"

var subAgentLinePrefixPattern = regexp.MustCompile(`^\*([a-z0-9]+(?:-[a-z0-9]+)*)\*\s+(.*)$`)

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
	lines := strings.Split(message, "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		lineSlug := normalizeCLISlug(slug)
		lineText := line
		if matches := subAgentLinePrefixPattern.FindStringSubmatch(line); len(matches) == 3 {
			lineSlug = normalizeCLISlug(matches[1])
			lineText = matches[2]
		}

		prefix := fmt.Sprintf("%s[%s]%s ", cliGrayPrefixColor, lineSlug, cliColorReset)
		lines[i] = prefix + lineText
	}

	return strings.Join(lines, "\n")
}
