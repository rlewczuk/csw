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

// ResponsesChatModel is a chat model implementation for Responses API.
type ResponsesChatModel struct {
	client  *ResponsesClient
	model   string
	options *ChatOptions
}

// Chat sends a chat request and returns the response.
func (m *ResponsesChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client_chat.go]: messages cannot be nil or empty")
	}

	if m.model == "" {
		return nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client_chat.go]: model not set")
	}

	effectiveOptions := options
	if effectiveOptions == nil {
		effectiveOptions = m.options
	}

	items, err := convertToResponsesItems(messages)
	if err != nil {
		return nil, err
	}

	store := false
	useCodexCompatibility := usesCodexCompatibilityEndpoint(m.client.baseURL)
	maxOutputTokens := DefaultMaxTokens
	stream := false
	if useCodexCompatibility {
		maxOutputTokens = 0
		stream = true
	}

	chatReq := ResponsesCreateRequest{
		Model:                m.model,
		Input:                items,
		Store:                &store,
		Instructions:         buildResponsesInstructions(messages),
		Tools:                convertToolsToResponses(tools),
		Stream:               stream,
		MaxOutputTokens:      maxOutputTokens,
		PromptCacheRetention: m.client.promptCacheRetention(),
	}

	if !useCodexCompatibility && m.client.config != nil && m.client.config.MaxTokens > 0 {
		chatReq.MaxOutputTokens = m.client.config.MaxTokens
	}

	if effectiveOptions != nil {
		chatReq.Temperature = float64(effectiveOptions.Temperature)
		chatReq.TopP = float64(effectiveOptions.TopP)
		if effectiveOptions.SessionID != "" {
			chatReq.PromptCacheKey = effectiveOptions.SessionID
		}
		if effectiveOptions.Thinking != "" {
			chatReq.Reasoning = buildResponsesReasoning(effectiveOptions.Thinking)
		}
	}

	url := m.client.baseURL + "/responses"

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client_chat.go]: failed to marshal request: %w", err)
	}

	executeRequest := func(token string) (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return nil, nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client_chat.go]: failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
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
			return nil, nil, wrapLLMRequestError(m.client.handleHTTPError(err), nil, nil)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			readErr := fmt.Errorf("ResponsesChatModel.Chat() [responses_client_chat.go]: failed to read response body: %w", m.client.handleHTTPError(err))
			return nil, nil, wrapLLMRequestError(readErr, resp, nil)
		}

		if effectiveOptions != nil && effectiveOptions.Verbose {
			logVerboseResponseFromBytes(resp, bodyBytes)
		}
		m.client.emitRawResponse(resp, bodyBytes)

		return resp, bodyBytes, nil
	}

	token, err := m.client.GetAccessToken()
	if err != nil {
		return nil, err
	}

	resp, bodyBytes, err := executeRequest(token)
	if err != nil {
		return nil, err
	}

	if m.client.shouldRefreshAfterUnauthorized(resp, bodyBytes) {
		if err := m.client.refreshTokenIfNeeded(true, token); err != nil {
			return nil, err
		}
		token, err = m.client.GetAccessToken()
		if err != nil {
			return nil, err
		}
		resp, bodyBytes, err = executeRequest(token)
		if err != nil {
			return nil, err
		}
	}

	if err := m.client.checkStatusCodeWithBody(resp, bodyBytes); err != nil {
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
		}
		return nil, wrapLLMRequestError(err, resp, bodyBytes)
	}

	if chatReq.Stream {
		result, err := convertFromResponsesStreamBody(bodyBytes)
		if err != nil {
			return nil, wrapLLMRequestError(err, resp, bodyBytes)
		}
		return result, nil
	}

	var chatResp ResponsesResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		decodeErr := fmt.Errorf("ResponsesChatModel.Chat() [responses_client_chat.go]: failed to decode response: %w", err)
		return nil, wrapLLMRequestError(decodeErr, resp, bodyBytes)
	}

	if effectiveOptions != nil && effectiveOptions.Logger != nil {
		logHTTPResponseWithObfuscation(effectiveOptions.Logger, resp, chatResp)
	}

	result, err := convertFromResponsesOutput(chatResp.Output)
	if err != nil {
		return nil, wrapLLMRequestError(err, resp, bodyBytes)
	}
	if chatResp.Usage != nil {
		total := chatResp.Usage.TotalTokens
		if total <= 0 {
			total = chatResp.Usage.InputTokens + chatResp.Usage.OutputTokens
		}
		cachedTokens := 0
		if chatResp.Usage.InputTokensDetails != nil {
			cachedTokens = chatResp.Usage.InputTokensDetails.CachedTokens
		}
		nonCachedTokens := chatResp.Usage.InputTokens - cachedTokens
		if nonCachedTokens < 0 {
			nonCachedTokens = 0
		}
		result.TokenUsage = &TokenUsage{
			InputTokens:          chatResp.Usage.InputTokens,
			InputCachedTokens:    cachedTokens,
			InputNonCachedTokens: nonCachedTokens,
			OutputTokens:         chatResp.Usage.OutputTokens,
			TotalTokens:          total,
		}
		result.ContextLengthTokens = total
	}

	return result, nil
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses.
func (m *ResponsesChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		if len(messages) == 0 {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: messages cannot be empty\n")
			return
		}

		if m.model == "" {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: model cannot be empty\n")
			return
		}

		effectiveOptions := options
		if effectiveOptions == nil {
			effectiveOptions = m.options
		}

		items, err := convertToResponsesItems(messages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: %v\n", err)
			return
		}

		store := false
		useCodexCompatibility := usesCodexCompatibilityEndpoint(m.client.baseURL)
		maxOutputTokens := DefaultMaxTokens
		if useCodexCompatibility {
			maxOutputTokens = 0
		}

		chatReq := ResponsesCreateRequest{
			Model:                m.model,
			Input:                items,
			Store:                &store,
			Instructions:         buildResponsesInstructions(messages),
			Tools:                convertToolsToResponses(tools),
			Stream:               true,
			MaxOutputTokens:      maxOutputTokens,
			PromptCacheRetention: m.client.promptCacheRetention(),
		}

		if !useCodexCompatibility && m.client.config != nil && m.client.config.MaxTokens > 0 {
			chatReq.MaxOutputTokens = m.client.config.MaxTokens
		}

		if effectiveOptions != nil {
			chatReq.Temperature = float64(effectiveOptions.Temperature)
			chatReq.TopP = float64(effectiveOptions.TopP)
			if effectiveOptions.SessionID != "" {
				chatReq.PromptCacheKey = effectiveOptions.SessionID
			}
			if effectiveOptions.Thinking != "" {
				chatReq.Reasoning = buildResponsesReasoning(effectiveOptions.Thinking)
			}
		}

		url := m.client.baseURL + "/responses"

		body, err := json.Marshal(chatReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: failed to marshal request: %v\n", err)
			return
		}

		executeRequest := func(token string) (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
			if err != nil {
				return nil, fmt.Errorf("ResponsesChatModel.ChatStream() [responses_client_chat.go]: failed to create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
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
				return nil, m.client.handleHTTPError(err)
			}
			return resp, nil
		}

		var token string
		token, err = m.client.GetAccessToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: failed to get access token: %v\n", err)
			return
		}

		resp, err := executeRequest(token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: HTTP request failed: %v\n", err)
			return
		}

		if m.client.shouldRefreshAfterUnauthorized(resp, nil) {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr == nil && m.client.shouldRefreshAfterUnauthorized(resp, bodyBytes) {
				if refreshErr := m.client.refreshTokenIfNeeded(true, token); refreshErr == nil {
					token, err = m.client.GetAccessToken()
					if err != nil {
						fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: failed to get refreshed access token: %v\n", err)
						return
					}

					resp, err = executeRequest(token)
					if err != nil {
						fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: HTTP request failed after token refresh: %v\n", err)
						return
					}
				} else {
					fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: failed to refresh access token: %v\n", refreshErr)
					return
				}
			} else if readErr != nil {
				fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: failed to read unauthorized response body: %v\n", readErr)
				return
			}
		}

		defer resp.Body.Close()

		logVerboseStreamResponseHeaders(resp, effectiveOptions != nil && effectiveOptions.Verbose)
		m.client.emitRawResponse(resp, nil)

		if err := m.client.checkStatusCode(resp); err != nil {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
			}
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client_chat.go]: API error (status %d): %v\n", resp.StatusCode, err)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		toolCallsInProgress := make(map[string]*responsesToolCallInProgress)
		usage := TokenUsage{}
		contextLength := 0

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			m.client.emitRawStreamChunk(line)
			if line == "" {
				continue
			}

			if effectiveOptions != nil && effectiveOptions.Verbose {
				fmt.Println(line)
			}

			if strings.HasPrefix(line, "event: ") {
				continue
			}

			if strings.TrimSpace(line) == "data: [DONE]" {
				if effectiveOptions != nil && effectiveOptions.Verbose {
					fmt.Println("=== End of Streaming Response ===")
					fmt.Println()
				}
				return
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if strings.TrimSpace(data) == "[DONE]" {
				if effectiveOptions != nil && effectiveOptions.Verbose {
					fmt.Println("=== End of Streaming Response ===")
					fmt.Println()
				}
				return
			}

			var event ResponsesStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPResponseChunk(effectiveOptions.Logger, event)
			}

			if event.Response != nil && event.Response.Usage != nil {
				usage.InputTokens += event.Response.Usage.InputTokens
				if event.Response.Usage.InputTokensDetails != nil {
					usage.InputCachedTokens += event.Response.Usage.InputTokensDetails.CachedTokens
				}
				usage.InputNonCachedTokens = usage.InputTokens - usage.InputCachedTokens
				if usage.InputNonCachedTokens < 0 {
					usage.InputNonCachedTokens = 0
				}
				usage.OutputTokens += event.Response.Usage.OutputTokens
				if event.Response.Usage.TotalTokens > 0 {
					usage.TotalTokens += event.Response.Usage.TotalTokens
				} else {
					usage.TotalTokens += event.Response.Usage.InputTokens + event.Response.Usage.OutputTokens
				}
				if usage.TotalTokens > 0 {
					contextLength = usage.TotalTokens
				}
			}

			switch event.Type {
			case "response.output_text.delta":
				if event.Delta != "" {
					result := &ChatMessage{
						Role: ChatRoleAssistant,
						Parts: []ChatMessagePart{
							{Text: event.Delta},
						},
					}
					if !yield(result) {
						return
					}
				}
			case "response.output_item.added":
				if event.Item != nil && event.Item.Type == "function_call" {
					toolCallsInProgress[event.Item.ID] = &responsesToolCallInProgress{
						CallID: event.Item.CallID,
						Name:   event.Item.Name,
					}
				}
			case "response.function_call_arguments.delta":
				if event.ItemID == "" {
					continue
				}
				tc := toolCallsInProgress[event.ItemID]
				if tc == nil {
					continue
				}
				tc.Arguments += event.Delta
			case "response.function_call_arguments.done":
				if event.ItemID == "" {
					continue
				}
				tc := toolCallsInProgress[event.ItemID]
				if tc == nil {
					continue
				}
				if event.Arguments != "" {
					tc.Arguments = event.Arguments
				}
				toolCall := responsesToolCallFromStream(tc)
				if toolCall != nil {
					result := &ChatMessage{
						Role:  ChatRoleAssistant,
						Parts: []ChatMessagePart{{ToolCall: toolCall}},
					}
					if !yield(result) {
						return
					}
				}
				delete(toolCallsInProgress, event.ItemID)
			case "response.completed":
				if usage.TotalTokens > 0 {
					usageMsg := &ChatMessage{Role: ChatRoleAssistant}
					usageCopy := usage
					usageMsg.TokenUsage = &usageCopy
					usageMsg.ContextLengthTokens = contextLength
					if !yield(usageMsg) {
						return
					}
				}
			}
		}
	}
}
