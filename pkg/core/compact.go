package core

import (
	"os"
	"sort"
	"strings"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
)

const (
	compactedFileMessagePrefix = "<csw_context_compaction_file path=\""
	compactedFileMessageSuffix = "</csw_context_compaction_file>"
	compactDefaultContextLimit = 4000
)

// ChatCompactor compacts chat message history.
type ChatCompactor interface {
	// CompactMessages compacts provided message history.
	CompactMessages(messages []*models.ChatMessage) []*models.ChatMessage
}

// CompactMessagesChatCompactor applies CompactMessages function-based compaction.
type CompactMessagesChatCompactor struct{}

// NewCompactMessagesChatCompactor creates default ChatCompactor implementation.
func NewCompactMessagesChatCompactor() ChatCompactor {
	return &CompactMessagesChatCompactor{}
}

// CompactMessages compacts provided message history using CompactMessages function.
func (c *CompactMessagesChatCompactor) CompactMessages(messages []*models.ChatMessage) []*models.ChatMessage {
	return CompactMessages(messages)
}

// CompactMessages applies multi-step compaction to chat messages.
func CompactMessages(messages []*models.ChatMessage) []*models.ChatMessage {
	return compactMessagesWithLimit(messages, compactDefaultContextLimit)
}

func compactMessagesWithLimit(messages []*models.ChatMessage, contextLimit int) []*models.ChatMessage {
	compacted := compactMessagesStep1ReplaceFileParts(messages)
	compacted = compactMessagesStep2KeepLastTodo(compacted)
	compacted = compactMessagesStep3ClipRunBashResponses(compacted)
	compacted = compactMessagesStep4PruneGrepFindWithLimit(compacted, contextLimit)
	compacted = compactMessagesStep5TrimAssistantThinkingWithLimit(compacted, contextLimit)
	compacted = compactMessagesStep6DropOldMessagesPreservingUserAndCompactedWithLimit(compacted, contextLimit)
	compacted = compactMessagesStep7DropOldCompactedMessagesWithLimit(compacted, contextLimit)
	compacted = compactMessagesStep8EnsureToolCallResponsePairs(compacted)

	return compacted
}

func compactMessagesStep1ReplaceFileParts(messages []*models.ChatMessage) []*models.ChatMessage {
	cloned := cloneMessages(messages)
	if len(cloned) == 0 {
		return cloned
	}

	type partRef struct {
		msgIndex  int
		partIndex int
	}
	type fileInfo struct {
		summaryLength int
		lastMsgIndex  int
		refs          []partRef
	}

	fileToInfo := make(map[string]*fileInfo)
	for msgIndex, msg := range cloned {
		if msg == nil {
			continue
		}

		messagePaths := messageFilePaths(msg)
		for partIndex, part := range msg.Parts {
			paths := filePathsForPart(part)
			if len(paths) == 0 && msg.Role == models.ChatRoleAssistant && (part.Text != "" || part.ReasoningContent != "") {
				paths = messagePaths
			}
			if len(paths) == 0 {
				continue
			}

			pathSet := make(map[string]struct{}, len(paths))
			for _, path := range paths {
				trimmed := strings.TrimSpace(path)
				if trimmed == "" {
					continue
				}
				pathSet[trimmed] = struct{}{}
			}

			if len(pathSet) == 0 {
				continue
			}

			partLength := compactMessagePartSummaryLengthForFile(part)
			for path := range pathSet {
				info := fileToInfo[path]
				if info == nil {
					info = &fileInfo{}
					fileToInfo[path] = info
				}
				info.summaryLength += partLength
				info.lastMsgIndex = msgIndex
				info.refs = append(info.refs, partRef{msgIndex: msgIndex, partIndex: partIndex})
			}
		}
	}

	type insertion struct {
		path    string
		content string
	}
	insertionsByMessage := make(map[int][]insertion)
	removePartsByMessage := make(map[int]map[int]struct{})
	for path, info := range fileToInfo {
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if info.summaryLength <= len(contentBytes) {
			continue
		}

		for _, ref := range info.refs {
			removeSet := removePartsByMessage[ref.msgIndex]
			if removeSet == nil {
				removeSet = make(map[int]struct{})
				removePartsByMessage[ref.msgIndex] = removeSet
			}
			removeSet[ref.partIndex] = struct{}{}
		}

		insertionsByMessage[info.lastMsgIndex] = append(insertionsByMessage[info.lastMsgIndex], insertion{
			path:    path,
			content: string(contentBytes),
		})
	}

	if len(insertionsByMessage) == 0 {
		return cloned
	}

	for msgIndex := range insertionsByMessage {
		sort.Slice(insertionsByMessage[msgIndex], func(i, j int) bool {
			return insertionsByMessage[msgIndex][i].path < insertionsByMessage[msgIndex][j].path
		})
	}

	result := make([]*models.ChatMessage, 0, len(cloned))
	for msgIndex, msg := range cloned {
		if msg == nil {
			continue
		}

		removeSet := removePartsByMessage[msgIndex]
		if len(removeSet) == 0 {
			result = append(result, msg)
		} else {
			filteredParts := make([]models.ChatMessagePart, 0, len(msg.Parts))
			for partIndex, part := range msg.Parts {
				if _, shouldRemove := removeSet[partIndex]; shouldRemove {
					continue
				}
				filteredParts = append(filteredParts, part)
			}
			if len(filteredParts) > 0 {
				updated := cloneMessage(msg)
				updated.Parts = filteredParts
				result = append(result, updated)
			}
		}

		for _, fileInsert := range insertionsByMessage[msgIndex] {
			result = append(result, newCompactedFileContentMessage(fileInsert.path, fileInsert.content))
		}
	}

	return result
}

func compactMessagesStep2KeepLastTodo(messages []*models.ChatMessage) []*models.ChatMessage {
	cloned := cloneMessages(messages)
	if len(cloned) == 0 {
		return cloned
	}

	lastTodoCallID := ""
	for _, msg := range cloned {
		if msg == nil {
			continue
		}
		for _, part := range msg.Parts {
			if part.ToolCall != nil && isTodoTool(part.ToolCall.Function) {
				if strings.TrimSpace(part.ToolCall.ID) != "" {
					lastTodoCallID = part.ToolCall.ID
				}
			}
			if part.ToolResponse != nil && part.ToolResponse.Call != nil && isTodoTool(part.ToolResponse.Call.Function) {
				if strings.TrimSpace(part.ToolResponse.Call.ID) != "" {
					lastTodoCallID = part.ToolResponse.Call.ID
				}
			}
		}
	}

	if lastTodoCallID == "" {
		return cloned
	}

	result := make([]*models.ChatMessage, 0, len(cloned))
	for _, msg := range cloned {
		if msg == nil {
			continue
		}

		updatedParts := make([]models.ChatMessagePart, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			updatedPart := part

			if part.ToolCall != nil && isTodoTool(part.ToolCall.Function) && part.ToolCall.ID != lastTodoCallID {
				updatedPart.ToolCall = nil
			}
			if part.ToolResponse != nil && part.ToolResponse.Call != nil && isTodoTool(part.ToolResponse.Call.Function) && part.ToolResponse.Call.ID != lastTodoCallID {
				updatedPart.ToolResponse = nil
			}

			if compactMessagePartIsEmpty(updatedPart) {
				continue
			}
			updatedParts = append(updatedParts, updatedPart)
		}

		if len(updatedParts) == 0 {
			continue
		}

		updatedMsg := cloneMessage(msg)
		updatedMsg.Parts = updatedParts
		result = append(result, updatedMsg)
	}

	return result
}

func compactMessagesStep3ClipRunBashResponses(messages []*models.ChatMessage) []*models.ChatMessage {
	cloned := cloneMessages(messages)
	for msgIndex, msg := range cloned {
		if msg == nil {
			continue
		}
		for partIndex, part := range msg.Parts {
			if part.ToolResponse == nil || part.ToolResponse.Call == nil || part.ToolResponse.Call.Function != "runBash" {
				continue
			}

			resultObject := part.ToolResponse.Result.Object()
			if resultObject == nil {
				continue
			}

			outputValue, hasOutput := resultObject["output"]
			if !hasOutput {
				continue
			}
			output, ok := outputValue.AsStringOK()
			if !ok {
				continue
			}

			resultObject["output"] = tool.NewToolValue(clipTextToLines(output, 16))
			updatedPart := msg.Parts[partIndex]
			updatedPart.ToolResponse.Result = tool.NewToolValue(resultObject)
			cloned[msgIndex].Parts[partIndex] = updatedPart
		}
	}

	return cloned
}

func compactMessagesStep4PruneGrepFind(messages []*models.ChatMessage) []*models.ChatMessage {
	return compactMessagesStep4PruneGrepFindWithLimit(messages, compactDefaultContextLimit)
}

func compactMessagesStep4PruneGrepFindWithLimit(messages []*models.ChatMessage, contextLimit int) []*models.ChatMessage {
	if !compactMessagesIsAboveThreshold(messages, contextLimit, 0.80) {
		return cloneMessages(messages)
	}

	cloned := cloneMessages(messages)
	targetFunctions := []string{"vfsGrep", "vfsFind"}

	keepIDs := make(map[string]map[string]struct{})
	for _, functionName := range targetFunctions {
		ids := collectToolInteractionIDs(cloned, functionName)
		if len(ids) > 3 {
			ids = ids[len(ids)-3:]
		}

		idSet := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			idSet[id] = struct{}{}
		}
		keepIDs[functionName] = idSet
	}

	result := make([]*models.ChatMessage, 0, len(cloned))
	for _, msg := range cloned {
		if msg == nil {
			continue
		}

		updatedParts := make([]models.ChatMessagePart, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			updatedPart := part

			if part.ToolCall != nil {
				if idSet, tracked := keepIDs[part.ToolCall.Function]; tracked {
					if _, keep := idSet[part.ToolCall.ID]; !keep {
						updatedPart.ToolCall = nil
					}
				}
			}

			if part.ToolResponse != nil && part.ToolResponse.Call != nil {
				if idSet, tracked := keepIDs[part.ToolResponse.Call.Function]; tracked {
					if _, keep := idSet[part.ToolResponse.Call.ID]; !keep {
						updatedPart.ToolResponse = nil
					}
				}
			}

			if compactMessagePartIsEmpty(updatedPart) {
				continue
			}
			updatedParts = append(updatedParts, updatedPart)
		}

		if len(updatedParts) == 0 {
			continue
		}

		updatedMsg := cloneMessage(msg)
		updatedMsg.Parts = updatedParts
		result = append(result, updatedMsg)
	}

	return result
}

func compactMessagesStep5TrimAssistantThinking(messages []*models.ChatMessage) []*models.ChatMessage {
	return compactMessagesStep5TrimAssistantThinkingWithLimit(messages, compactDefaultContextLimit)
}

func compactMessagesStep5TrimAssistantThinkingWithLimit(messages []*models.ChatMessage, contextLimit int) []*models.ChatMessage {
	if !compactMessagesIsAboveThreshold(messages, contextLimit, 0.80) {
		return cloneMessages(messages)
	}

	cloned := cloneMessages(messages)
	currentSize := compactMessagesEstimateSize(cloned)
	targetSize := int(0.50 * float64(contextLimit))

	if targetSize < 0 {
		targetSize = 0
	}

	for msgIndex, msg := range cloned {
		if currentSize <= targetSize {
			break
		}
		if msg == nil || msg.Role != models.ChatRoleAssistant {
			continue
		}

		for partIndex, part := range msg.Parts {
			if currentSize <= targetSize {
				break
			}
			if part.ReasoningContent == "" {
				continue
			}

			removedLength := len(part.ReasoningContent)
			updatedPart := part
			updatedPart.ReasoningContent = ""
			cloned[msgIndex].Parts[partIndex] = updatedPart
			currentSize -= removedLength
		}
	}

	return cloned
}

func compactMessagesStep6DropOldMessagesPreservingUserAndCompacted(messages []*models.ChatMessage) []*models.ChatMessage {
	return compactMessagesStep6DropOldMessagesPreservingUserAndCompactedWithLimit(messages, compactDefaultContextLimit)
}

func compactMessagesStep6DropOldMessagesPreservingUserAndCompactedWithLimit(messages []*models.ChatMessage, contextLimit int) []*models.ChatMessage {
	if !compactMessagesIsAboveThreshold(messages, contextLimit, 0.80) {
		return cloneMessages(messages)
	}

	cloned := cloneMessages(messages)
	currentSize := compactMessagesEstimateSize(cloned)
	targetSize := int(0.50 * float64(contextLimit))
	if targetSize < 0 {
		targetSize = 0
	}

	keepFlags := make([]bool, len(cloned))
	for i := range keepFlags {
		keepFlags[i] = true
	}

	for msgIndex := 1; msgIndex < len(cloned) && currentSize > targetSize; msgIndex++ {
		msg := cloned[msgIndex]
		if msg == nil {
			continue
		}
		if msg.Role == models.ChatRoleSystem {
			continue
		}
		if compactMessageIsCompactedFileContent(msg) {
			continue
		}
		if compactMessageIsExplicitUserText(msg) {
			continue
		}

		keepFlags[msgIndex] = false
		currentSize -= compactMessageEstimateSize(msg)
	}

	result := make([]*models.ChatMessage, 0, len(cloned))
	for i, msg := range cloned {
		if !keepFlags[i] || msg == nil {
			continue
		}
		result = append(result, msg)
	}

	return result
}

func compactMessagesStep7DropOldCompactedMessages(messages []*models.ChatMessage) []*models.ChatMessage {
	return compactMessagesStep7DropOldCompactedMessagesWithLimit(messages, compactDefaultContextLimit)
}

func compactMessagesStep7DropOldCompactedMessagesWithLimit(messages []*models.ChatMessage, contextLimit int) []*models.ChatMessage {
	if !compactMessagesIsAboveThreshold(messages, contextLimit, 0.80) {
		return cloneMessages(messages)
	}

	cloned := cloneMessages(messages)
	currentSize := compactMessagesEstimateSize(cloned)
	targetSize := int(0.65 * float64(contextLimit))
	if targetSize < 0 {
		targetSize = 0
	}

	keepFlags := make([]bool, len(cloned))
	for i := range keepFlags {
		keepFlags[i] = true
	}

	latestCompactedIndex := -1
	for i, msg := range cloned {
		if msg != nil && compactMessageIsCompactedFileContent(msg) {
			latestCompactedIndex = i
		}
	}

	for msgIndex, msg := range cloned {
		if currentSize <= targetSize {
			break
		}
		if msg == nil || !compactMessageIsCompactedFileContent(msg) {
			continue
		}
		if msgIndex == latestCompactedIndex {
			continue
		}

		keepFlags[msgIndex] = false
		currentSize -= compactMessageEstimateSize(msg)
	}

	result := make([]*models.ChatMessage, 0, len(cloned))
	for i, msg := range cloned {
		if !keepFlags[i] || msg == nil {
			continue
		}
		result = append(result, msg)
	}

	return result
}

func compactMessagesStep8EnsureToolCallResponsePairs(messages []*models.ChatMessage) []*models.ChatMessage {
	cloned := cloneMessages(messages)
	if len(cloned) == 0 {
		return cloned
	}

	toolCallIDs := make(map[string]int)
	toolResponseIDs := make(map[string]int)
	for _, msg := range cloned {
		if msg == nil {
			continue
		}
		for _, part := range msg.Parts {
			if part.ToolCall != nil && strings.TrimSpace(part.ToolCall.ID) != "" {
				toolCallIDs[part.ToolCall.ID]++
			}
			if part.ToolResponse != nil && part.ToolResponse.Call != nil && strings.TrimSpace(part.ToolResponse.Call.ID) != "" {
				toolResponseIDs[part.ToolResponse.Call.ID]++
			}
		}
	}

	result := make([]*models.ChatMessage, 0, len(cloned))
	for _, msg := range cloned {
		if msg == nil {
			continue
		}

		updatedParts := make([]models.ChatMessagePart, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			updatedPart := part

			if part.ToolCall != nil {
				callID := strings.TrimSpace(part.ToolCall.ID)
				if callID == "" || toolResponseIDs[callID] == 0 {
					updatedPart.ToolCall = nil
				}
			}

			if part.ToolResponse != nil {
				responseID := ""
				if part.ToolResponse.Call != nil {
					responseID = strings.TrimSpace(part.ToolResponse.Call.ID)
				}
				if responseID == "" || toolCallIDs[responseID] == 0 {
					updatedPart.ToolResponse = nil
				}
			}

			if compactMessagePartIsEmpty(updatedPart) {
				continue
			}
			updatedParts = append(updatedParts, updatedPart)
		}

		if len(updatedParts) == 0 {
			continue
		}

		updatedMsg := cloneMessage(msg)
		updatedMsg.Parts = updatedParts
		result = append(result, updatedMsg)
	}

	return result
}
