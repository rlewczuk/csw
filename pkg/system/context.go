package system

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
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

// BuildPromptContextData builds prompt template context from CLI context entries and AgentState.
func BuildPromptContextData(cliContext any, agentState core.AgentState) map[string]any {
	contextData := make(map[string]any)

	for key, value := range normalizeContextData(cliContext) {
		contextData[key] = value
	}

	if strings.TrimSpace(agentState.Info.CurrentTime) == "" {
		agentState.Info.CurrentTime = time.Now().Format(time.RFC3339)
	}

	for key, value := range structToTemplateMap(agentState) {
		contextData[key] = value
	}

	return contextData
}

func normalizeContextData(contextData any) map[string]any {
	switch typed := contextData.(type) {
	case nil:
		return nil
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, value := range typed {
			result[key] = value
		}
		return result
	case map[string]string:
		result := make(map[string]any, len(typed))
		for key, value := range typed {
			result[key] = value
		}
		return result
	default:
		return nil
	}
}

func structToTemplateMap(input any) map[string]any {
	value := reflect.ValueOf(input)
	for value.IsValid() && value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return nil
	}

	converted, ok := convertTemplateValue(value).(map[string]any)
	if !ok {
		return nil
	}

	return converted
}

func convertTemplateValue(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}

	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		return convertTemplateValue(value.Elem())
	case reflect.Ptr:
		if value.IsNil() {
			return nil
		}
		return convertTemplateValue(value.Elem())
	case reflect.Struct:
		result := make(map[string]any, value.NumField())
		valueType := value.Type()
		for idx := 0; idx < value.NumField(); idx++ {
			fieldType := valueType.Field(idx)
			if fieldType.PkgPath != "" {
				continue
			}
			result[fieldType.Name] = convertTemplateValue(value.Field(idx))
		}
		return result
	case reflect.Slice, reflect.Array:
		length := value.Len()
		result := make([]any, length)
		for idx := 0; idx < length; idx++ {
			result[idx] = convertTemplateValue(value.Index(idx))
		}
		return result
	case reflect.Map:
		if value.Type().Key().Kind() == reflect.String {
			result := make(map[string]any, value.Len())
			for _, key := range value.MapKeys() {
				result[key.String()] = convertTemplateValue(value.MapIndex(key))
			}
			return result
		}
		return value.Interface()
	default:
		return value.Interface()
	}
}
