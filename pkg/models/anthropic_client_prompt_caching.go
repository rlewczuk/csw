package models

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rlewczuk/csw/pkg/tool"
)

// convertToolsToAnthropic converts tool.ToolInfo to Anthropic AnthropicTool format.
func convertToolsToAnthropic(tools []tool.ToolInfo) []AnthropicTool {
	if len(tools) == 0 {
		return nil
	}

	orderedTools := make([]tool.ToolInfo, len(tools))
	copy(orderedTools, tools)
	sort.SliceStable(orderedTools, func(i, j int) bool {
		if orderedTools[i].Name == orderedTools[j].Name {
			return orderedTools[i].Description < orderedTools[j].Description
		}
		return orderedTools[i].Name < orderedTools[j].Name
	})

	anthropicTools := make([]AnthropicTool, len(orderedTools))
	for i, t := range orderedTools {
		normalizedSchema := normalizeAnthropicToolSchemaForPromptCaching(t.Schema)
		schemaJSON, _ := marshalStableAnthropicJSON(normalizedSchema)
		var schemaMap map[string]interface{}
		json.Unmarshal(schemaJSON, &schemaMap)

		anthropicTools[i] = AnthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schemaMap,
		}
	}

	return anthropicTools
}

// marshalAnthropicMessagesRequest marshals Anthropic request in a deterministic format with prompt-caching hints.
func marshalAnthropicMessagesRequest(req AnthropicMessagesRequest) ([]byte, error) {
	body, err := marshalStableAnthropicJSON(req)
	if err != nil {
		return nil, fmt.Errorf("AnthropicChatModel.marshalAnthropicMessagesRequest() [anthropic_client_prompt_caching.go]: failed to marshal base request: %w", err)
	}

	var requestMap map[string]any
	if err := json.Unmarshal(body, &requestMap); err != nil {
		return nil, fmt.Errorf("AnthropicChatModel.marshalAnthropicMessagesRequest() [anthropic_client_prompt_caching.go]: failed to unmarshal base request: %w", err)
	}

	applyAnthropicPromptCachingBreakpoints(requestMap)

	stableBody, err := marshalStableAnthropicJSON(requestMap)
	if err != nil {
		return nil, fmt.Errorf("AnthropicChatModel.marshalAnthropicMessagesRequest() [anthropic_client_prompt_caching.go]: failed to marshal request map: %w", err)
	}

	return stableBody, nil
}

// applyAnthropicPromptCachingBreakpoints adds cache-control breakpoints for stable prompt-prefix caching.
func applyAnthropicPromptCachingBreakpoints(requestMap map[string]any) {
	if requestMap == nil {
		return
	}

	if markLastToolWithCacheControl(requestMap) {
		return
	}

	markSystemPromptWithCacheControl(requestMap)
}

// markLastToolWithCacheControl marks the last tool with cache-control metadata and returns true when a tool was marked.
func markLastToolWithCacheControl(requestMap map[string]any) bool {
	toolsRaw, ok := requestMap["tools"]
	if !ok {
		return false
	}

	tools, ok := toolsRaw.([]any)
	if !ok || len(tools) == 0 {
		return false
	}

	lastIdx := len(tools) - 1
	toolMap, ok := tools[lastIdx].(map[string]any)
	if !ok {
		return false
	}

	toolMap["cache_control"] = map[string]any{"type": "ephemeral"}
	tools[lastIdx] = toolMap
	requestMap["tools"] = tools

	return true
}

// markSystemPromptWithCacheControl marks system prompt blocks with cache-control metadata.
func markSystemPromptWithCacheControl(requestMap map[string]any) {
	systemRaw, exists := requestMap["system"]
	if !exists {
		return
	}

	switch system := systemRaw.(type) {
	case string:
		if strings.TrimSpace(system) == "" {
			return
		}
		requestMap["system"] = []any{
			map[string]any{
				"type":          "text",
				"text":          system,
				"cache_control": map[string]any{"type": "ephemeral"},
			},
		}
	case []any:
		if len(system) == 0 {
			return
		}
		lastIdx := len(system) - 1
		blockMap, ok := system[lastIdx].(map[string]any)
		if !ok {
			return
		}
		blockMap["cache_control"] = map[string]any{"type": "ephemeral"}
		system[lastIdx] = blockMap
		requestMap["system"] = system
	}
}

// normalizeAnthropicToolSchemaForPromptCaching normalizes schema slices to reduce prompt-cache misses.
func normalizeAnthropicToolSchemaForPromptCaching(schema tool.ToolSchema) tool.ToolSchema {
	normalized := schema
	normalized.Required = anthropicSortedStringsCopy(schema.Required)
	normalized.Properties = normalizeAnthropicPropertySchemaMapForPromptCaching(schema.Properties)
	return normalized
}

// normalizeAnthropicPropertySchemaMapForPromptCaching normalizes nested property schemas.
func normalizeAnthropicPropertySchemaMapForPromptCaching(properties map[string]tool.PropertySchema) map[string]tool.PropertySchema {
	if len(properties) == 0 {
		return properties
	}

	normalized := make(map[string]tool.PropertySchema, len(properties))
	for key, property := range properties {
		nested := property
		nested.Enum = anthropicSortedStringsCopy(property.Enum)
		nested.Required = anthropicSortedStringsCopy(property.Required)
		nested.Properties = normalizeAnthropicPropertySchemaMapForPromptCaching(property.Properties)
		if property.Items != nil {
			nestedItem := *property.Items
			nestedItem.Enum = anthropicSortedStringsCopy(property.Items.Enum)
			nestedItem.Required = anthropicSortedStringsCopy(property.Items.Required)
			nestedItem.Properties = normalizeAnthropicPropertySchemaMapForPromptCaching(property.Items.Properties)
			nested.Items = &nestedItem
		}
		normalized[key] = nested
	}

	return normalized
}

// anthropicSortedStringsCopy returns a sorted copy of input strings.
func anthropicSortedStringsCopy(input []string) []string {
	if len(input) == 0 {
		return input
	}

	cloned := make([]string, len(input))
	copy(cloned, input)
	sort.Strings(cloned)
	return cloned
}

// marshalStableAnthropicJSON marshals values deterministically by using encoding/json map key ordering.
func marshalStableAnthropicJSON(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshalStableAnthropicJSON() [anthropic_client_prompt_caching.go]: failed to marshal value: %w", err)
	}

	return body, nil
}
