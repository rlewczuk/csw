package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
)

func cloneMessages(messages []*models.ChatMessage) []*models.ChatMessage {
	if len(messages) == 0 {
		return nil
	}

	result := make([]*models.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		result = append(result, cloneMessage(msg))
	}
	return result
}

func cloneMessage(msg *models.ChatMessage) *models.ChatMessage {
	if msg == nil {
		return nil
	}

	parts := make([]models.ChatMessagePart, len(msg.Parts))
	for i, part := range msg.Parts {
		parts[i] = cloneMessagePart(part)
	}

	var tokenUsage *models.TokenUsage
	if msg.TokenUsage != nil {
		copyValue := *msg.TokenUsage
		tokenUsage = &copyValue
	}

	return &models.ChatMessage{
		Role:                msg.Role,
		Parts:               parts,
		TokenUsage:          tokenUsage,
		ContextLengthTokens: msg.ContextLengthTokens,
	}
}

func cloneMessagePart(part models.ChatMessagePart) models.ChatMessagePart {
	cloned := models.ChatMessagePart{
		Text:             part.Text,
		ReasoningContent: part.ReasoningContent,
	}

	if part.ToolCall != nil {
		callCopy := *part.ToolCall
		callCopy.Arguments = cloneToolValue(callCopy.Arguments)
		cloned.ToolCall = &callCopy
	}

	if part.ToolResponse != nil {
		responseCopy := *part.ToolResponse
		if responseCopy.Call != nil {
			callCopy := *responseCopy.Call
			callCopy.Arguments = cloneToolValue(callCopy.Arguments)
			responseCopy.Call = &callCopy
		}
		responseCopy.Result = cloneToolValue(responseCopy.Result)
		cloned.ToolResponse = &responseCopy
	}

	return cloned
}

func cloneToolValue(value tool.ToolValue) tool.ToolValue {
	data, err := json.Marshal(value.Raw())
	if err != nil {
		return value
	}

	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return value
	}

	return tool.NewToolValue(raw)
}

func compactMessagePartSummaryLength(part models.ChatMessagePart) int {
	size := len(part.Text) + len(part.ReasoningContent)
	if part.ToolCall != nil {
		size += compactToolCallEstimateSize(part.ToolCall)
	}
	if part.ToolResponse != nil {
		size += compactToolResponseEstimateSize(part.ToolResponse)
	}
	return size
}

func compactMessagePartSummaryLengthForFile(part models.ChatMessagePart) int {
	size := len(part.Text) + len(part.ReasoningContent)
	if part.ToolResponse == nil {
		return size
	}

	resultObject := part.ToolResponse.Result.Object()
	if resultObject == nil {
		return size
	}

	if contentValue, ok := resultObject["content"]; ok {
		if content, ok := contentValue.AsStringOK(); ok {
			size += len(content)
			return size
		}
	}

	size += compactToolResponseEstimateSize(part.ToolResponse)
	return size
}

func compactMessagePartIsEmpty(part models.ChatMessagePart) bool {
	return part.Text == "" && part.ReasoningContent == "" && part.ToolCall == nil && part.ToolResponse == nil
}

func compactMessagesIsAboveThreshold(messages []*models.ChatMessage, contextLimit int, threshold float64) bool {
	if contextLimit <= 0 {
		return false
	}
	currentSize := compactMessagesEstimateSize(messages)
	return float64(currentSize) > float64(contextLimit)*threshold
}

func compactMessagesEstimateSize(messages []*models.ChatMessage) int {
	size := 0
	for _, msg := range messages {
		size += compactMessageEstimateSize(msg)
	}
	return size
}

func compactMessageEstimateSize(msg *models.ChatMessage) int {
	if msg == nil {
		return 0
	}

	size := len(msg.Role) + 4
	for _, part := range msg.Parts {
		size += len(part.Text)
		size += len(part.ReasoningContent)
		if part.ToolCall != nil {
			size += compactToolCallEstimateSize(part.ToolCall)
		}
		if part.ToolResponse != nil {
			size += compactToolResponseEstimateSize(part.ToolResponse)
		}
	}
	return size
}

func compactToolCallEstimateSize(call *tool.ToolCall) int {
	if call == nil {
		return 0
	}
	serialized, err := json.Marshal(call)
	if err != nil {
		return len(call.ID) + len(call.Function)
	}
	return len(serialized)
}

func compactToolResponseEstimateSize(response *tool.ToolResponse) int {
	if response == nil {
		return 0
	}

	type responseProjection struct {
		Call   *tool.ToolCall `json:"call,omitempty"`
		Error  string         `json:"error,omitempty"`
		Result tool.ToolValue `json:"result"`
		Done   bool           `json:"done"`
	}

	projection := responseProjection{
		Call:   response.Call,
		Result: response.Result,
		Done:   response.Done,
	}
	if response.Error != nil {
		projection.Error = response.Error.Error()
	}

	serialized, err := json.Marshal(projection)
	if err != nil {
		return 0
	}
	return len(serialized)
}

func isFileOperationTool(functionName string) bool {
	switch functionName {
	case "vfsRead", "vfsWrite", "vfsEdit":
		return true
	default:
		return false
	}
}

func isTodoTool(functionName string) bool {
	return functionName == "todoRead" || functionName == "todoWrite"
}

func filePathsForPart(part models.ChatMessagePart) []string {
	paths := make([]string, 0, 1)

	if part.ToolCall != nil && isFileOperationTool(part.ToolCall.Function) {
		if path, ok := part.ToolCall.Arguments.StringOK("path"); ok {
			paths = append(paths, path)
		}
	}

	if part.ToolResponse != nil && part.ToolResponse.Call != nil && isFileOperationTool(part.ToolResponse.Call.Function) {
		if path, ok := part.ToolResponse.Call.Arguments.StringOK("path"); ok {
			paths = append(paths, path)
		}
	}

	return paths
}

func messageFilePaths(msg *models.ChatMessage) []string {
	if msg == nil {
		return nil
	}

	pathSet := map[string]struct{}{}
	for _, part := range msg.Parts {
		for _, path := range filePathsForPart(part) {
			trimmed := strings.TrimSpace(path)
			if trimmed == "" {
				continue
			}
			pathSet[trimmed] = struct{}{}
		}
	}

	if len(pathSet) == 0 {
		return nil
	}

	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func newCompactedFileContentMessage(path string, content string) *models.ChatMessage {
	text := fmt.Sprintf("%s%s\">\n%s\n%s", compactedFileMessagePrefix, path, content, compactedFileMessageSuffix)
	return models.NewTextMessage(models.ChatRoleUser, text)
}

func compactMessageIsCompactedFileContent(msg *models.ChatMessage) bool {
	if msg == nil || msg.Role != models.ChatRoleUser || len(msg.Parts) != 1 {
		return false
	}
	text := strings.TrimSpace(msg.Parts[0].Text)
	return strings.HasPrefix(text, compactedFileMessagePrefix) && strings.HasSuffix(text, compactedFileMessageSuffix)
}

func compactMessageIsExplicitUserText(msg *models.ChatMessage) bool {
	if msg == nil || msg.Role != models.ChatRoleUser {
		return false
	}

	if compactMessageIsCompactedFileContent(msg) {
		return false
	}

	if len(msg.GetToolCalls()) > 0 || len(msg.GetToolResponses()) > 0 {
		return false
	}

	text := strings.TrimSpace(msg.GetText())
	if text == "" {
		return false
	}

	if strings.Contains(text, "<system>") {
		return true
	}

	return true
}

func clipTextToLines(text string, limit int) string {
	if limit <= 0 {
		return ""
	}

	lines := strings.Split(text, "\n")
	if len(lines) <= limit {
		return text
	}

	return strings.Join(lines[:limit], "\n")
}

func collectToolInteractionIDs(messages []*models.ChatMessage, functionName string) []string {
	ids := make([]string, 0)
	seen := make(map[string]struct{})

	for _, msg := range messages {
		if msg == nil {
			continue
		}

		for _, part := range msg.Parts {
			if part.ToolCall != nil && part.ToolCall.Function == functionName {
				id := strings.TrimSpace(part.ToolCall.ID)
				if id != "" {
					if _, exists := seen[id]; !exists {
						seen[id] = struct{}{}
						ids = append(ids, id)
					}
				}
			}
			if part.ToolResponse != nil && part.ToolResponse.Call != nil && part.ToolResponse.Call.Function == functionName {
				id := strings.TrimSpace(part.ToolResponse.Call.ID)
				if id != "" {
					if _, exists := seen[id]; !exists {
						seen[id] = struct{}{}
						ids = append(ids, id)
					}
				}
			}
		}
	}

	return ids
}
