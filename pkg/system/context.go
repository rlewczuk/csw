package system

import (
	"fmt"
	"os"
	"strings"
)

// ParseCLIContextEntries parses repeated --context/-c CLI values in KEY=VAL format.
func ParseCLIContextEntries(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	result := make(map[string]string, len(values))
	for _, entry := range values {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			return nil, fmt.Errorf("ParseCLIContextEntries() [cli.go]: context entry cannot be empty")
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("ParseCLIContextEntries() [cli.go]: invalid context entry %q: expected KEY=VAL format", entry)
		}

		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("ParseCLIContextEntries() [cli.go]: invalid context entry %q: key cannot be empty", entry)
		}

		result[key] = parts[1]
	}

	return result, nil
}

func ParseCLIContextFromEntries(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	result := make(map[string]string, len(values))
	for _, entry := range values {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			return nil, fmt.Errorf("ParseCLIContextFromEntries() [hook.go]: context-from entry cannot be empty")
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("ParseCLIContextFromEntries() [hook.go]: invalid context-from entry %q: expected KEY=FILENAME format", entry)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("ParseCLIContextFromEntries() [hook.go]: invalid context-from entry %q: key cannot be empty", entry)
		}
		fileName := strings.TrimSpace(parts[1])
		if fileName == "" {
			return nil, fmt.Errorf("ParseCLIContextFromEntries() [hook.go]: invalid context-from entry %q: filename cannot be empty", entry)
		}
		data, err := os.ReadFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("ParseCLIContextFromEntries() [hook.go]: failed to read %q: %w", fileName, err)
		}
		result[key] = string(data)
	}

	return result, nil
}
