package shared

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

func FormatList(values []string) string {
	if len(values) == 0 {
		return "-"
	}

	copyValues := append([]string(nil), values...)
	sort.Strings(copyValues)
	return strings.Join(copyValues, ", ")
}

func SortedList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	copyValues := append([]string(nil), values...)
	sort.Strings(copyValues)

	return copyValues
}

func NullValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}

	return value
}

// RenderTextWithContext renders prompt as text/template with provided context map.
func RenderTextWithContext(prompt string, contextData map[string]string) (string, error) {
	tmpl, err := template.New("cli-prompt").Option("missingkey=error").Parse(prompt)
	if err != nil {
		return "", fmt.Errorf("RenderTextWithContext() [cli.go]: failed to parse prompt template: %w", err)
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, contextData); err != nil {
		return "", fmt.Errorf("RenderTextWithContext() [cli.go]: failed to render prompt template: %w", err)
	}

	return rendered.String(), nil
}
