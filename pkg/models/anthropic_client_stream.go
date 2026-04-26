package models

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"os"
	"strings"

	"github.com/rlewczuk/csw/pkg/tool"
)

func (c *AnthropicClient) emitRawStreamChunk(line string) {
	if c == nil || c.rawLLMCallback == nil {
		return
	}
	c.emitRawLLMLine("<<< CHUNK " + obfuscateBodyWithRegex(line))
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses.
func (m *AnthropicChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		// Validate inputs
		if len(messages) == 0 {
			fmt.Fprintf(os.Stderr, "ERROR: AnthropicChatModel.ChatStream() [anthropic_client_stream.go]: messages cannot be empty\n")
			return
		}

		if m.model == "" {
			fmt.Fprintf(os.Stderr, "ERROR: AnthropicChatModel.ChatStream() [anthropic_client_stream.go]: model cannot be empty\n")
			return
		}

		// Use provided options or fall back to model's default options
		effectiveOptions := options
		if effectiveOptions == nil {
			effectiveOptions = m.options
		}

		// Convert messages to Anthropic format
		var systemPrompt string
		var anthropicMessages []AnthropicMessageParam

		for _, msg := range messages {
			if msg.Role == ChatRoleSystem {
				if systemPrompt != "" {
					systemPrompt += "\n"
				}
				systemPrompt += msg.GetText()
			} else {
				anthropicMsg := convertToAnthropicMessage(msg)
				anthropicMessages = append(anthropicMessages, anthropicMsg)
			}
		}

		// Build request with streaming enabled
		chatReq := AnthropicMessagesRequest{
			Model:     m.model,
			Messages:  anthropicMessages,
			MaxTokens: DefaultMaxTokens,
			Stream:    true,
			Tools:     convertToolsToAnthropic(tools),
		}

		// Apply config MaxTokens if set
		if m.client.config != nil && m.client.config.MaxTokens > 0 {
			chatReq.MaxTokens = m.client.config.MaxTokens
		}

		if systemPrompt != "" {
			chatReq.System = systemPrompt
		}

		// Apply options if provided
		if effectiveOptions != nil {
			// Anthropic API does not allow both temperature and top_p to be set
			// Prefer temperature if both are provided
			if effectiveOptions.Temperature > 0 {
				chatReq.Temperature = float64(effectiveOptions.Temperature)
			} else if effectiveOptions.TopP > 0 {
				chatReq.TopP = float64(effectiveOptions.TopP)
			}
			if effectiveOptions.TopK > 0 {
				chatReq.TopK = effectiveOptions.TopK
			}
			// Add thinking configuration if set
			if effectiveOptions.Thinking != "" {
				chatReq.Thinking = buildAnthropicThinking(effectiveOptions.Thinking, chatReq.MaxTokens)
			}
		}

		url := m.client.baseURL + "/v1/messages"

		body, err := marshalAnthropicMessagesRequest(chatReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: AnthropicChatModel.ChatStream() [anthropic_client_stream.go]: failed to create request: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", m.client.apiKey)
		req.Header.Set("anthropic-version", m.client.apiVersion)
		req.Header.Set("anthropic-beta", anthropicPromptCachingBetaHeaderValue)
		setUserAgentHeader(req)
		m.client.applyConfiguredHeaders(req)
		m.client.applyConfiguredQueryParams(req)
		applyOptionsHeaders(req, effectiveOptions)

		// Print verbose request output if enabled
		logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)

		// Log request using structured logger if available
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPRequestWithObfuscation(effectiveOptions.Logger, req, chatReq)
		}
		m.client.emitRawRequest(req, body)

		resp, err := m.client.httpClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: AnthropicChatModel.ChatStream() [anthropic_client_stream.go]: HTTP request failed: %v\n", err)
			return
		}
		defer resp.Body.Close()
		m.client.emitRawResponse(resp, nil)

		// Log response headers before checking status (so errors are also logged)
		logVerboseStreamResponseHeaders(resp, effectiveOptions != nil && effectiveOptions.Verbose)

		if err := m.client.checkStatusCode(resp); err != nil {
			// Read and log error response body
			bodyBytes, _ := io.ReadAll(resp.Body)
			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
			}
			fmt.Fprintf(os.Stderr, "ERROR: AnthropicChatModel.ChatStream() [anthropic_client_stream.go]: API error (status %d): %v\n", resp.StatusCode, err)
			return
		}

		// Create scanner for SSE and stream responses
		scanner := bufio.NewScanner(resp.Body)
		usage := TokenUsage{}
		contextLength := 0

		for scanner.Scan() {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			m.client.emitRawStreamChunk(line)

			// Print raw line in verbose mode
			if effectiveOptions != nil && effectiveOptions.Verbose {
				fmt.Println(line)
			}

			// Skip empty lines
			if line == "" {
				continue
			}

			// Parse event type line (we don't need to track it as the type is in the data JSON)
			if strings.HasPrefix(line, "event: ") {
				continue
			}

			// Process SSE data line
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				// Check for [DONE] or similar markers
				if strings.TrimSpace(data) == "[DONE]" {
					return
				}

				var event AnthropicStreamEvent
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					// Skip invalid JSON
					continue
				}

				// Log each chunk using structured logger if available
				if effectiveOptions != nil && effectiveOptions.Logger != nil {
					logHTTPResponseChunk(effectiveOptions.Logger, event)
				}

				// Handle different event types
				switch event.Type {
				case "content_block_start":
					// Handle tool_use blocks
					if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
						result := &ChatMessage{
							Role: ChatRoleAssistant,
							Parts: []ChatMessagePart{
								{
									ToolCall: &tool.ToolCall{
										ID:        event.ContentBlock.ID,
										Function:  event.ContentBlock.Name,
										Arguments: tool.NewToolValue(event.ContentBlock.Input),
									},
								},
							},
						}
						if !yield(result) {
							return
						}
					}
				case "content_block_delta":
					if event.Delta != nil && event.Delta.Text != "" {
						result := &ChatMessage{
							Role: ChatRoleAssistant,
							Parts: []ChatMessagePart{
								{Text: event.Delta.Text},
							},
						}
						if !yield(result) {
							return
						}
					}
					// Handle partial JSON for tool_use streaming
					// Note: Anthropic may stream tool input as partial_json
					// For now, we only return complete tool_use blocks from content_block_start
				case "message_delta":
					if event.Usage != nil {
						usage.InputTokens += event.Usage.InputTokens
						usage.InputNonCachedTokens = usage.InputTokens
						usage.OutputTokens += event.Usage.OutputTokens
						usage.TotalTokens += event.Usage.InputTokens + event.Usage.OutputTokens
						if usage.TotalTokens > 0 {
							contextLength = usage.TotalTokens
						}
					}
					// Check for stop reason
					if event.Delta != nil && event.Delta.StopReason != "" {
						if usage.TotalTokens > 0 {
							usageMsg := &ChatMessage{Role: ChatRoleAssistant}
							usageCopy := usage
							usageMsg.TokenUsage = &usageCopy
							usageMsg.ContextLengthTokens = contextLength
							if !yield(usageMsg) {
								return
							}
						}
						return
					}
				case "message_stop":
					if usage.TotalTokens > 0 {
						usageMsg := &ChatMessage{Role: ChatRoleAssistant}
						usageCopy := usage
						usageMsg.TokenUsage = &usageCopy
						usageMsg.ContextLengthTokens = contextLength
						if !yield(usageMsg) {
							return
						}
					}
					if effectiveOptions != nil && effectiveOptions.Verbose {
						fmt.Println("=== End of Streaming Response ===")
						fmt.Println()
					}
					return
				}
			}
		}

		// Check for scanner error
		if err := scanner.Err(); err != nil {
			if effectiveOptions != nil && effectiveOptions.Verbose {
				fmt.Println("=== End of Streaming Response ===")
				fmt.Println()
			}
			return
		}
	}
}
