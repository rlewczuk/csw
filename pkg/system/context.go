package system

import (
	"fmt"
	"strings"
)

// ParseRunContextEntries parses repeated --context/-c run command values in KEY=VAL format.
func ParseRunContextEntries(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	result := make(map[string]string, len(values))
	for _, entry := range values {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			return nil, fmt.Errorf("ParseRunContextEntries() [context.go]: context entry cannot be empty")
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("ParseRunContextEntries() [context.go]: invalid context entry %q: expected KEY=VAL format", entry)
		}

		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("ParseRunContextEntries() [context.go]: invalid context entry %q: key cannot be empty", entry)
		}

		result[key] = parts[1]
	}

	return result, nil
}
