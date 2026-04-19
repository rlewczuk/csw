package models

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/rlewczuk/csw/pkg/tool"
)

// convertToOpenAIMessage converts a models.ChatMessage to OpenAI OpenaiChatCompletionMessage format
func convertToOpenAIMessage(msg *ChatMessage) OpenaiChatCompletionMessage {
	openaiMsg := OpenaiChatCompletionMessage{
		Role: string(msg.Role),
	}

	toolResponses := msg.GetToolResponses()
	if len(toolResponses) > 0 {
		openaiMsg.Role = "tool"
	}

	var reasoningContent string
	for _, part := range msg.Parts {
		if part.ReasoningContent != "" {
			reasoningContent += part.ReasoningContent
		}
	}
	if reasoningContent != "" {
		openaiMsg.ReasoningContent = reasoningContent
	}

	// Check if message contains only text
	hasOnlyText := true
	for _, part := range msg.Parts {
		if part.ToolCall != nil || part.ToolResponse != nil {
			hasOnlyText = false
			break
		}
	}

	if hasOnlyText {
		// Simple text message
		openaiMsg.Content = msg.GetText()
	} else {
		// Message contains tool calls or tool responses
		for _, part := range msg.Parts {
			if part.Text != "" {
				openaiMsg.Content = part.Text
			} else if part.ToolCall != nil {
				// Add tool call
				argsJSON, _ := marshalStableJSON(part.ToolCall.Arguments.Raw())
				openaiMsg.ToolCalls = append(openaiMsg.ToolCalls, OpenaiToolCall{
					ID:   part.ToolCall.ID,
					Type: "function",
					Function: OpenaiToolCallFunction{
						Name:      part.ToolCall.Function,
						Arguments: string(argsJSON),
					},
				})
			} else if part.ToolResponse != nil {
				// OpenaiTool response - set tool_call_id and content
				// Prefer Call.ID if available, fall back to ID for backward compatibility
				if part.ToolResponse.Call != nil {
					openaiMsg.ToolCallID = part.ToolResponse.Call.ID
				}
				if part.ToolResponse.Error != nil {
					openaiMsg.Content = part.ToolResponse.Error.Error()
				} else {
					resultJSON, _ := marshalStableJSON(part.ToolResponse.Result.Raw())
					openaiMsg.Content = string(resultJSON)
				}
			}
		}
	}

	return openaiMsg
}

// convertFromOpenAIMessage converts OpenAI OpenaiChatCompletionMessage to models.ChatMessage
func convertFromOpenAIMessage(msg *OpenaiChatCompletionMessage) *ChatMessage {
	var parts []ChatMessagePart

	// Add reasoning content if present (for thinking models like GLM-5)
	if msg.ReasoningContent != "" {
		parts = append(parts, ChatMessagePart{ReasoningContent: msg.ReasoningContent})
	}

	// Add text content if present
	if contentStr, ok := msg.Content.(string); ok && contentStr != "" {
		parts = append(parts, ChatMessagePart{Text: contentStr})
	}

	// Add tool calls if present
	for _, tc := range msg.ToolCalls {
		var args map[string]interface{}
		json.Unmarshal([]byte(tc.Function.Arguments), &args)
		parts = append(parts, ChatMessagePart{
			ToolCall: &tool.ToolCall{
				ID:        tc.ID,
				Function:  tc.Function.Name,
				Arguments: tool.NewToolValue(args),
			},
		})
	}

	return &ChatMessage{
		Role:  ChatRole(msg.Role),
		Parts: parts,
	}
}

// mapThinkingToReasoningEffort maps a thinking mode string to OpenAI's reasoning_effort value.
// OpenAI supports "low", "medium", "high" for reasoning effort on o1 models.
// If thinking is "true", defaults to "medium". If "false" or empty, returns empty string.
func mapThinkingToReasoningEffort(thinking string) string {
	if thinking == "" || thinking == "false" {
		return ""
	}

	switch thinking {
	case "low", "medium", "high":
		return thinking
	case "true":
		return "medium"
	case "xhigh":
		// OpenAI doesn't have xhigh, map to high
		return "high"
	default:
		// For unknown values, return as-is and let the API handle it
		return thinking
	}
}

// convertToolsToOpenAI converts tool.ToolInfo to OpenAI OpenaiTool format
func convertToolsToOpenAI(tools []tool.ToolInfo) []OpenaiTool {
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

	openaiTools := make([]OpenaiTool, len(orderedTools))
	for i, t := range orderedTools {
		// Convert ToolSchema to map[string]interface{}
		normalizedSchema := normalizeToolSchemaForPromptCaching(t.Schema)
		schemaJSON, _ := marshalStableJSON(normalizedSchema)
		var schemaMap map[string]interface{}
		json.Unmarshal(schemaJSON, &schemaMap)

		openaiTools[i] = OpenaiTool{
			Type: "function",
			Function: OpenaiToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  schemaMap,
			},
		}
	}
	return openaiTools
}

// normalizeToolSchemaForPromptCaching normalizes schema slices to reduce prompt-cache misses.
func normalizeToolSchemaForPromptCaching(schema tool.ToolSchema) tool.ToolSchema {
	normalized := schema
	normalized.Required = sortedStringsCopy(schema.Required)
	normalized.Properties = normalizePropertySchemaMapForPromptCaching(schema.Properties)
	return normalized
}

// normalizePropertySchemaMapForPromptCaching normalizes nested property schemas.
func normalizePropertySchemaMapForPromptCaching(properties map[string]tool.PropertySchema) map[string]tool.PropertySchema {
	if len(properties) == 0 {
		return properties
	}

	normalized := make(map[string]tool.PropertySchema, len(properties))
	for key, property := range properties {
		nested := property
		nested.Enum = sortedStringsCopy(property.Enum)
		nested.Required = sortedStringsCopy(property.Required)
		nested.Properties = normalizePropertySchemaMapForPromptCaching(property.Properties)
		if property.Items != nil {
			nestedItem := *property.Items
			nestedItem.Enum = sortedStringsCopy(property.Items.Enum)
			nestedItem.Required = sortedStringsCopy(property.Items.Required)
			nestedItem.Properties = normalizePropertySchemaMapForPromptCaching(property.Items.Properties)
			nested.Items = &nestedItem
		}
		normalized[key] = nested
	}

	return normalized
}

// sortedStringsCopy returns a sorted copy of input strings.
func sortedStringsCopy(input []string) []string {
	if len(input) == 0 {
		return input
	}

	cloned := make([]string, len(input))
	copy(cloned, input)
	sort.Strings(cloned)
	return cloned
}

// marshalStableJSON marshals values deterministically by using encoding/json map key ordering.
func marshalStableJSON(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshalStableJSON() [openai_client_conversion.go]: failed to marshal value: %w", err)
	}

	return body, nil
}
