package models

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/rlewczuk/csw/pkg/tool"
)

// convertToOllamaMessage converts a models.ChatMessage to Ollama OllamaMessage format.
func convertToOllamaMessage(msg *ChatMessage) OllamaMessage {
	ollamaMsg := OllamaMessage{
		Role: string(msg.Role),
	}

	toolCalls := msg.GetToolCalls()
	toolResponses := msg.GetToolResponses()

	if len(toolCalls) > 0 {
		ollamaMsg.Content = msg.GetText()
		for _, tc := range toolCalls {
			var args map[string]interface{}
			if tc.Arguments.Raw() != nil {
				if m, ok := tc.Arguments.Raw().(map[string]interface{}); ok {
					args = m
				}
			}
			if args == nil {
				args = make(map[string]interface{})
			}
			ollamaMsg.ToolCalls = append(ollamaMsg.ToolCalls, OllamaToolCall{
				Function: OllamaToolCallFunction{
					Name:      tc.Function,
					Arguments: args,
				},
			})
		}
	} else if len(toolResponses) > 0 {
		ollamaMsg.Role = "tool"
		for _, tr := range toolResponses {
			if tr.Error != nil {
				ollamaMsg.Content = "Error: " + tr.Error.Error()
			} else {
				resultJSON, _ := json.Marshal(tr.Result.Raw())
				ollamaMsg.Content = string(resultJSON)
			}
			if tr.Call != nil {
				ollamaMsg.ToolName = tr.Call.Function
			}
			break
		}
	} else {
		ollamaMsg.Content = msg.GetText()
	}

	return ollamaMsg
}

// generateOllamaToolCallID generates a unique ID for tool calls since Ollama doesn't provide them.
func generateOllamaToolCallID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "call_" + hex.EncodeToString(b)
}

// convertFromOllamaMessage converts Ollama OllamaMessage to models.ChatMessage.
func convertFromOllamaMessage(msg OllamaMessage) *ChatMessage {
	var parts []ChatMessagePart

	if msg.Content != "" {
		parts = append(parts, ChatMessagePart{Text: msg.Content})
	}

	for _, tc := range msg.ToolCalls {
		parts = append(parts, ChatMessagePart{
			ToolCall: &tool.ToolCall{
				ID:        generateOllamaToolCallID(),
				Function:  tc.Function.Name,
				Arguments: tool.NewToolValue(tc.Function.Arguments),
			},
		})
	}

	return &ChatMessage{
		Role:  ChatRole(msg.Role),
		Parts: parts,
	}
}

// convertToolsToOllama converts tool.ToolInfo to Ollama OllamaTool format.
func convertToolsToOllama(tools []tool.ToolInfo) []OllamaTool {
	if len(tools) == 0 {
		return nil
	}

	ollamaTools := make([]OllamaTool, len(tools))
	for i, t := range tools {
		schemaJSON, _ := json.Marshal(t.Schema)
		var schemaMap map[string]interface{}
		json.Unmarshal(schemaJSON, &schemaMap)

		ollamaTools[i] = OllamaTool{
			Type: "function",
			Function: OllamaToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  schemaMap,
			},
		}
	}
	return ollamaTools
}
