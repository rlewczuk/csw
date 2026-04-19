package models

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/rlewczuk/csw/pkg/tool"
)

const responsesOverloadedRetryAfterSeconds = 60

type responsesToolCallInProgress struct {
	CallID    string
	Name      string
	Arguments string
}

func responsesToolCallFromStream(tc *responsesToolCallInProgress) *tool.ToolCall {
	if tc == nil || tc.CallID == "" || tc.Name == "" {
		return nil
	}

	args := tool.NewToolValue(map[string]any{})
	if tc.Arguments != "" {
		if parsed, err := tool.NewToolValueFromJSON(tc.Arguments); err == nil {
			args = parsed
		} else {
			args = tool.NewToolValue(map[string]any{"raw": tc.Arguments})
		}
	}

	return &tool.ToolCall{
		ID:        tc.CallID,
		Function:  tc.Name,
		Arguments: args,
	}
}

func convertToResponsesItems(messages []*ChatMessage) ([]ResponsesItem, error) {
	var items []ResponsesItem

	for _, msg := range messages {
		if msg == nil {
			continue
		}

		if msg.Role == ChatRoleSystem {
			continue
		}

		var contentParts []ResponsesContent
		var toolCalls []*tool.ToolCall
		var toolResponses []*tool.ToolResponse

		for _, part := range msg.Parts {
			if part.Text != "" {
				if msg.Role == ChatRoleAssistant {
					contentParts = append(contentParts, ResponsesContent{Type: "output_text", Text: part.Text})
				} else {
					contentParts = append(contentParts, ResponsesContent{Type: "input_text", Text: part.Text})
				}
			}
			if part.ToolCall != nil {
				toolCalls = append(toolCalls, part.ToolCall)
			}
			if part.ToolResponse != nil {
				toolResponses = append(toolResponses, part.ToolResponse)
			}
		}

		if len(contentParts) > 0 {
			items = append(items, ResponsesItem{Type: "message", Role: string(msg.Role), Content: contentParts})
		}

		for _, call := range toolCalls {
			if call == nil {
				continue
			}
			argsJSON := "{}"
			if call.Arguments.Raw() != nil {
				normalizedArgs := normalizeMapOrderValue(call.Arguments.Raw())
				if bytes, err := json.Marshal(normalizedArgs); err == nil {
					argsJSON = string(bytes)
				}
			}
			items = append(items, ResponsesItem{Type: "function_call", CallID: call.ID, Name: call.Function, Arguments: argsJSON})
		}

		for _, resp := range toolResponses {
			if resp == nil || resp.Call == nil || resp.Call.ID == "" {
				return nil, fmt.Errorf("convertToResponsesItems() [responses_client_response_conversion.go]: tool response missing call ID")
			}
			output := ""
			if resp.Error != nil {
				output = resp.Error.Error()
			} else if resp.Result.Raw() != nil {
				normalizedResult := normalizeMapOrderValue(resp.Result.Raw())
				if bytes, err := json.Marshal(normalizedResult); err == nil {
					output = string(bytes)
				}
			}
			items = append(items, ResponsesItem{Type: "function_call_output", CallID: resp.Call.ID, Output: output})
		}
	}

	return items, nil
}

// buildResponsesInstructions returns request instructions from system/developer messages.
func buildResponsesInstructions(messages []*ChatMessage) string {
	instructions := make([]string, 0)

	for _, msg := range messages {
		if msg == nil {
			continue
		}
		if msg.Role != ChatRoleSystem && msg.Role != ChatRoleDeveloper {
			continue
		}

		for _, part := range msg.Parts {
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			instructions = append(instructions, part.Text)
		}
	}

	if len(instructions) == 0 {
		return defaultResponsesInstructions
	}

	return strings.Join(instructions, "\n\n")
}

// usesCodexCompatibilityEndpoint returns true when backend uses ChatGPT Codex quirks.
func usesCodexCompatibilityEndpoint(baseURL string) bool {
	return strings.Contains(strings.ToLower(baseURL), "/backend-api/codex")
}

// buildResponsesReasoning creates a ResponsesReasoning struct from a thinking mode string.
func buildResponsesReasoning(thinking string) *ResponsesReasoning {
	if thinking == "" || thinking == "false" {
		return nil
	}

	switch thinking {
	case "low", "medium", "high", "xhigh":
		return &ResponsesReasoning{Effort: thinking}
	case "true":
		return &ResponsesReasoning{Effort: "medium"}
	default:
		return &ResponsesReasoning{Effort: thinking}
	}
}

// scanStreamBodyWithAdaptiveBuffer scans stream body lines and doubles scanner buffer on token-too-long errors.
func scanStreamBodyWithAdaptiveBuffer(bodyBytes []byte, handleLine func(line string) bool) error {
	maxTokenSize := bufio.MaxScanTokenSize

	for {
		scanner := bufio.NewScanner(bytes.NewReader(bodyBytes))
		scanner.Buffer(make([]byte, 0, maxTokenSize), maxTokenSize)

		for scanner.Scan() {
			if handleLine(scanner.Text()) {
				return nil
			}
		}

		if err := scanner.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				maxTokenSize *= 2
				continue
			}

			return fmt.Errorf("scanStreamBodyWithAdaptiveBuffer() [responses_client_response_conversion.go]: failed to scan stream body: %w", err)
		}

		return nil
	}
}

// convertFromResponsesStreamBody converts SSE response body into ChatMessage.
func convertFromResponsesStreamBody(bodyBytes []byte) (*ChatMessage, error) {
	result := &ChatMessage{Role: ChatRoleAssistant, Parts: []ChatMessagePart{}}

	toolCallsInProgress := make(map[string]*responsesToolCallInProgress)
	var streamEventErr error
	err := scanStreamBodyWithAdaptiveBuffer(bodyBytes, func(line string) bool {
		if line == "" || strings.HasPrefix(line, "event: ") {
			return false
		}
		if strings.TrimSpace(line) == "data: [DONE]" {
			return true
		}
		if !strings.HasPrefix(line, "data: ") {
			return false
		}

		data := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(data) == "[DONE]" {
			return true
		}

		var event ResponsesStreamEvent
		if unmarshalErr := json.Unmarshal([]byte(data), &event); unmarshalErr != nil {
			return false
		}

		if event.Type == "error" && event.Error != nil {
			streamEventErr = mapResponsesStreamError(event.Error)
			return true
		}

		if event.Type == "response.incomplete" {
			streamEventErr = mapResponsesIncompleteEventError(event.Response)
			if streamEventErr != nil {
				return true
			}
		}

		switch event.Type {
		case "response.output_text.delta":
			if event.Delta != "" {
				result.Parts = append(result.Parts, ChatMessagePart{Text: event.Delta})
			}
		case "response.output_item.added":
			if event.Item != nil && event.Item.Type == "function_call" {
				toolCallsInProgress[event.Item.ID] = &responsesToolCallInProgress{CallID: event.Item.CallID, Name: event.Item.Name}
			}
		case "response.function_call_arguments.delta":
			if event.ItemID == "" {
				return false
			}
			tc := toolCallsInProgress[event.ItemID]
			if tc == nil {
				return false
			}
			tc.Arguments += event.Delta
		case "response.function_call_arguments.done":
			if event.ItemID == "" {
				return false
			}
			tc := toolCallsInProgress[event.ItemID]
			if tc == nil {
				return false
			}
			if event.Arguments != "" {
				tc.Arguments = event.Arguments
			}
			toolCall := responsesToolCallFromStream(tc)
			if toolCall != nil {
				result.Parts = append(result.Parts, ChatMessagePart{ToolCall: toolCall})
			}
			delete(toolCallsInProgress, event.ItemID)
		}

		return false
	})
	if err != nil {
		return nil, fmt.Errorf("convertFromResponsesStreamBody() [responses_client_response_conversion.go]: failed to scan stream body: %w", err)
	}
	if streamEventErr != nil {
		return nil, streamEventErr
	}

	if len(result.Parts) == 0 {
		return nil, fmt.Errorf("convertFromResponsesStreamBody() [responses_client_response_conversion.go]: no usable output items in response")
	}

	var finalUsage TokenUsage
	contextLength := 0
	err = scanStreamBodyWithAdaptiveBuffer(bodyBytes, func(line string) bool {
		if !strings.HasPrefix(line, "data: ") {
			return false
		}
		data := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(data) == "[DONE]" {
			return true
		}
		var event ResponsesStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return false
		}
		if event.Response != nil && event.Response.Usage != nil {
			finalUsage.InputTokens += event.Response.Usage.InputTokens
			if event.Response.Usage.InputTokensDetails != nil {
				finalUsage.InputCachedTokens += event.Response.Usage.InputTokensDetails.CachedTokens
			}
			finalUsage.InputNonCachedTokens = finalUsage.InputTokens - finalUsage.InputCachedTokens
			if finalUsage.InputNonCachedTokens < 0 {
				finalUsage.InputNonCachedTokens = 0
			}
			finalUsage.OutputTokens += event.Response.Usage.OutputTokens
			if event.Response.Usage.TotalTokens > 0 {
				finalUsage.TotalTokens += event.Response.Usage.TotalTokens
			} else {
				finalUsage.TotalTokens += event.Response.Usage.InputTokens + event.Response.Usage.OutputTokens
			}
			if finalUsage.TotalTokens > 0 {
				contextLength = finalUsage.TotalTokens
			}
		}

		return false
	})
	if err != nil {
		return nil, fmt.Errorf("convertFromResponsesStreamBody() [responses_client_response_conversion.go]: failed to scan stream body: %w", err)
	}
	if finalUsage.TotalTokens > 0 {
		usageCopy := finalUsage
		result.TokenUsage = &usageCopy
		result.ContextLengthTokens = contextLength
	}

	return result, nil
}

func mapResponsesStreamError(apiErr *OpenaiAPIError) error {
	if apiErr == nil {
		return fmt.Errorf("mapResponsesStreamError() [responses_client_response_conversion.go]: empty stream error")
	}

	errorCode := fmt.Sprint(apiErr.Code)
	errorTypeLower := strings.ToLower(strings.TrimSpace(apiErr.Type))
	errorCodeLower := strings.ToLower(strings.TrimSpace(errorCode))
	if errorCode == "context_length_exceeded" {
		return fmt.Errorf("%w: %s", ErrTooManyInputTokens, apiErr.Message)
	}

	if errorTypeLower == "service_unavailable_error" && errorCodeLower == "server_is_overloaded" {
		return &RateLimitError{RetryAfterSeconds: responsesOverloadedRetryAfterSeconds, Message: apiErr.Message}
	}

	if errorTypeLower == "server_error" && errorCodeLower == "server_error" {
		return &NetworkError{Message: apiErr.Message, IsRetryable: true}
	}

	return &APIRequestError{ErrorType: apiErr.Type, Code: errorCode, Param: fmt.Sprint(apiErr.Param), Message: apiErr.Message}
}

// mapResponsesIncompleteEventError maps response.incomplete stream events to retriable errors.
func mapResponsesIncompleteEventError(response *ResponsesResponse) error {
	if response == nil {
		return nil
	}

	reason := responsesIncompleteReason(response.IncompleteDetails)
	if reason != "max_output_tokens" {
		return nil
	}

	return &NetworkError{Message: "response incomplete: max_output_tokens", IsRetryable: true}
}

// responsesIncompleteReason extracts incomplete reason from response.incomplete_details.
func responsesIncompleteReason(incompleteDetails any) string {
	detailsMap, ok := incompleteDetails.(map[string]any)
	if !ok {
		return ""
	}

	reasonValue, ok := detailsMap["reason"]
	if !ok {
		return ""
	}

	reason := strings.ToLower(strings.TrimSpace(fmt.Sprint(reasonValue)))
	if reason == "<nil>" {
		return ""
	}

	return reason
}

func convertFromResponsesOutput(items []ResponsesItem) (*ChatMessage, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("convertFromResponsesOutput() [responses_client_response_conversion.go]: no output items in response")
	}

	result := &ChatMessage{Role: ChatRoleAssistant, Parts: []ChatMessagePart{}}

	for _, item := range items {
		switch item.Type {
		case "message":
			if item.Role != "assistant" {
				continue
			}
			for _, content := range item.Content {
				switch content.Type {
				case "output_text":
					if content.Text != "" {
						result.Parts = append(result.Parts, ChatMessagePart{Text: content.Text})
					}
				case "refusal":
					if content.Refusal != "" {
						result.Parts = append(result.Parts, ChatMessagePart{Text: content.Refusal})
					}
				}
			}
		case "function_call":
			if item.CallID == "" || item.Name == "" {
				continue
			}
			args := tool.NewToolValue(map[string]any{})
			if item.Arguments != "" {
				if parsed, err := tool.NewToolValueFromJSON(item.Arguments); err == nil {
					args = parsed
				} else {
					args = tool.NewToolValue(map[string]any{"raw": item.Arguments})
				}
			}
			result.Parts = append(result.Parts, ChatMessagePart{ToolCall: &tool.ToolCall{ID: item.CallID, Function: item.Name, Arguments: args}})
		}
	}

	if len(result.Parts) == 0 {
		return nil, fmt.Errorf("convertFromResponsesOutput() [responses_client_response_conversion.go]: no usable output items in response")
	}

	return result, nil
}

func convertToolsToResponses(tools []tool.ToolInfo) []ResponsesTool {
	if len(tools) == 0 {
		return nil
	}

	converted := make([]ResponsesTool, len(tools))
	for i, t := range tools {
		normalizedSchema := normalizeToolSchema(t.Schema)
		schemaJSON, _ := json.Marshal(normalizedSchema)
		var schemaMap map[string]any
		json.Unmarshal(schemaJSON, &schemaMap)

		converted[i] = ResponsesTool{Type: "function", Name: t.Name, Description: t.Description, Parameters: schemaMap}
	}

	return converted
}

// normalizeMapOrderValue returns a recursively normalized representation where map keys are emitted in lexical order during JSON marshaling.
func normalizeMapOrderValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		normalized := make(map[string]any, len(value))
		for _, key := range keys {
			normalized[key] = normalizeMapOrderValue(value[key])
		}
		return normalized
	case []any:
		normalized := make([]any, len(value))
		for i := range value {
			normalized[i] = normalizeMapOrderValue(value[i])
		}
		return normalized
	default:
		return value
	}
}

// normalizeToolSchema returns schema with sorted object keys and required fields.
func normalizeToolSchema(schema tool.ToolSchema) tool.ToolSchema {
	normalized := schema
	if len(normalized.Required) > 0 {
		normalized.Required = append([]string(nil), normalized.Required...)
		sort.Strings(normalized.Required)
	}

	normalized.Properties = normalizePropertySchemaMap(normalized.Properties)
	if normalized.Properties == nil {
		normalized.Properties = map[string]tool.PropertySchema{}
	}

	return normalized
}

// normalizePropertySchemaMap returns map copy with recursively normalized property schemas.
func normalizePropertySchemaMap(props map[string]tool.PropertySchema) map[string]tool.PropertySchema {
	if len(props) == 0 {
		return nil
	}

	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	normalized := make(map[string]tool.PropertySchema, len(props))
	for _, key := range keys {
		normalized[key] = normalizePropertySchema(props[key])
	}

	return normalized
}

// normalizePropertySchema returns recursively normalized property schema.
func normalizePropertySchema(schema tool.PropertySchema) tool.PropertySchema {
	normalized := schema

	if len(normalized.Required) > 0 {
		normalized.Required = append([]string(nil), normalized.Required...)
		sort.Strings(normalized.Required)
	}

	if normalized.Items != nil {
		itemsCopy := normalizePropertySchema(*normalized.Items)
		normalized.Items = &itemsCopy
	}

	normalized.Properties = normalizePropertySchemaMap(normalized.Properties)

	return normalized
}
