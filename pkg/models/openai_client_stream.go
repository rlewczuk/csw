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

// emitRawStreamChunk emits a raw streaming chunk line to the callback.
func (c *OpenAIClient) emitRawStreamChunk(line string) {
	if c == nil || c.rawLLMCallback == nil {
		return
	}
	c.emitRawLLMLine("<<< CHUNK " + obfuscateBodyWithRegex(line))
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses.
func (m *OpenAIChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		if len(messages) == 0 {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client_stream.go]: messages cannot be empty\n")
			return
		}

		if m.model == "" {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client_stream.go]: model cannot be empty\n")
			return
		}

		effectiveOptions := options
		if effectiveOptions == nil {
			effectiveOptions = m.options
		}

		openaiMessages := make([]OpenaiChatCompletionMessage, len(messages))
		for i, msg := range messages {
			openaiMessages[i] = convertToOpenAIMessage(msg)
		}

		chatReq := OpenaiChatCompletionRequest{
			Model:    m.model,
			Messages: openaiMessages,
			Stream:   true,
			StreamOptions: &OpenaiStreamOptions{
				IncludeUsage: true,
			},
			Tools:     convertToolsToOpenAI(tools),
			MaxTokens: DefaultMaxTokens,
		}
		if len(tools) > 0 {
			chatReq.ToolChoice = "auto"
		}
		if m.client.config != nil && m.client.config.MaxTokens > 0 {
			chatReq.MaxTokens = m.client.config.MaxTokens
		}
		if effectiveOptions != nil {
			chatReq.Temperature = float64(effectiveOptions.Temperature)
			chatReq.TopP = float64(effectiveOptions.TopP)
			if effectiveOptions.Thinking != "" {
				chatReq.ReasoningEffort = mapThinkingToReasoningEffort(effectiveOptions.Thinking)
			}
		}

		url := m.client.baseURL + "/chat/completions"
		body, err := m.client.marshalChatCompletionRequest(chatReq, effectiveOptions)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client_stream.go]: failed to create request: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		token, err := m.client.GetAccessToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client_stream.go]: failed to get access token: %v\n", err)
			return
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Accept", "text/event-stream")
		m.client.applyConfiguredQueryParams(req)
		setUserAgentHeader(req)
		m.client.applyConfiguredHeaders(req)
		applyOptionsHeaders(req, effectiveOptions)

		logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPRequestWithObfuscation(effectiveOptions.Logger, req, chatReq)
		}
		m.client.emitRawRequest(req, body)

		resp, err := m.client.httpClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client_stream.go]: HTTP request failed: %v\n", err)
			return
		}
		defer resp.Body.Close()

		logVerboseStreamResponseHeaders(resp, effectiveOptions != nil && effectiveOptions.Verbose)
		m.client.emitRawResponse(resp, nil)

		if err := m.client.checkStatusCode(resp); err != nil {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
			}
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client_stream.go]: API error (status %d): %v\n", resp.StatusCode, err)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		toolCallsInProgress := make(map[int]*OpenaiToolCall)
		var accumulatedReasoningContent string
		usage := TokenUsage{}
		hasUsage := false
		contextLength := 0

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			if line == "" {
				continue
			}
			m.client.emitRawStreamChunk(line)

			if effectiveOptions != nil && effectiveOptions.Verbose {
				fmt.Println(line)
			}

			if strings.TrimSpace(line) == "data: [DONE]" {
				if effectiveOptions != nil && effectiveOptions.Verbose {
					fmt.Println("=== End of Streaming Response ===")
					fmt.Println()
				}
				return
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				var chatResp OpenaiChatCompletionResponse
				if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
					continue
				}

				if effectiveOptions != nil && effectiveOptions.Logger != nil {
					logHTTPResponseChunk(effectiveOptions.Logger, chatResp)
				}
				if chatResp.Usage != nil {
					hasUsage = true
					usage.InputTokens += chatResp.Usage.PromptTokens
					if chatResp.Usage.PromptTokensDetails != nil {
						usage.InputCachedTokens += chatResp.Usage.PromptTokensDetails.CachedTokens
					}
					usage.InputNonCachedTokens = usage.InputTokens - usage.InputCachedTokens
					if usage.InputNonCachedTokens < 0 {
						usage.InputNonCachedTokens = 0
					}
					usage.OutputTokens += chatResp.Usage.CompletionTokens
					usage.TotalTokens += chatResp.Usage.TotalTokens
					if chatResp.Usage.TotalTokens > 0 {
						contextLength = chatResp.Usage.TotalTokens
					}
				}

				if len(chatResp.Choices) == 0 {
					continue
				}

				choice := chatResp.Choices[0]
				if choice.FinishReason != "" {
					if choice.Delta != nil {
						if choice.Delta.ReasoningContent != "" {
							accumulatedReasoningContent += choice.Delta.ReasoningContent
						}

						var content string
						switch v := choice.Delta.Content.(type) {
						case string:
							content = v
						default:
							if v != nil {
								content = fmt.Sprintf("%v", v)
							}
						}

						if content != "" {
							result := &ChatMessage{
								Role: ChatRoleAssistant,
								Parts: []ChatMessagePart{{
									Text:             content,
									ReasoningContent: accumulatedReasoningContent,
								}},
							}
							accumulatedReasoningContent = ""
							if !yield(result) {
								return
							}
						}

						if len(toolCallsInProgress) > 0 {
							result := &ChatMessage{Role: ChatRoleAssistant, Parts: []ChatMessagePart{}}
							if accumulatedReasoningContent != "" {
								result.Parts = append(result.Parts, ChatMessagePart{ReasoningContent: accumulatedReasoningContent})
								accumulatedReasoningContent = ""
							}
							for _, tc := range toolCallsInProgress {
								var args map[string]interface{}
								json.Unmarshal([]byte(tc.Function.Arguments), &args)
								result.Parts = append(result.Parts, ChatMessagePart{
									ToolCall: &tool.ToolCall{
										ID:        tc.ID,
										Function:  tc.Function.Name,
										Arguments: tool.NewToolValue(args),
									},
								})
							}
							if !yield(result) {
								return
							}
						}
					}
					if hasUsage {
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

				if choice.Delta != nil {
					if choice.Delta.ReasoningContent != "" {
						accumulatedReasoningContent += choice.Delta.ReasoningContent
					}

					var content string
					switch v := choice.Delta.Content.(type) {
					case string:
						content = v
					default:
						if v != nil {
							content = fmt.Sprintf("%v", v)
						}
					}

					if content != "" {
						result := &ChatMessage{
							Role: ChatRoleAssistant,
							Parts: []ChatMessagePart{{
								Text:             content,
								ReasoningContent: accumulatedReasoningContent,
							}},
						}
						accumulatedReasoningContent = ""
						if !yield(result) {
							return
						}
					}

					if len(choice.Delta.ToolCalls) > 0 {
						for _, tcDelta := range choice.Delta.ToolCalls {
							tc, exists := toolCallsInProgress[tcDelta.Index]
							if !exists {
								tc = &OpenaiToolCall{
									ID:   tcDelta.ID,
									Type: tcDelta.Type,
									Function: OpenaiToolCallFunction{
										Name:      tcDelta.Function.Name,
										Arguments: tcDelta.Function.Arguments,
									},
								}
								toolCallsInProgress[tcDelta.Index] = tc
							} else {
								if tcDelta.Function.Arguments != "" {
									tc.Function.Arguments += tcDelta.Function.Arguments
								}
								if tcDelta.ID != "" {
									tc.ID = tcDelta.ID
								}
								if tcDelta.Type != "" {
									tc.Type = tcDelta.Type
								}
								if tcDelta.Function.Name != "" {
									tc.Function.Name = tcDelta.Function.Name
								}
							}
						}
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return
		}
	}
}
