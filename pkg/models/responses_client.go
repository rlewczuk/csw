package models

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/tool"
)

// ResponsesClient is a client for interacting with Open Responses-compatible APIs.
type ResponsesClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	config     *conf.ModelProviderConfig
}

// ResponsesChatModel is a chat model implementation for Responses API.
type ResponsesChatModel struct {
	client  *ResponsesClient
	model   string
	options *ChatOptions
}

// ResponsesEmbeddingModel is a placeholder embedding model (not supported by Responses API).
type ResponsesEmbeddingModel struct {
	client *ResponsesClient
	model  string
}

type responsesToolCallInProgress struct {
	CallID    string
	Name      string
	Arguments string
}

// NewResponsesClient creates a new Responses client with the given config.
func NewResponsesClient(config *conf.ModelProviderConfig) (*ResponsesClient, error) {
	if config == nil {
		return nil, fmt.Errorf("NewResponsesClient() [responses_client.go]: config cannot be nil")
	}

	if config.URL == "" {
		return nil, fmt.Errorf("NewResponsesClient() [responses_client.go]: URL cannot be empty")
	}

	connectTimeout := 3600 * time.Second
	requestTimeout := 3600 * time.Second
	apiKey := ""

	if config.ConnectTimeout > 0 {
		connectTimeout = config.ConnectTimeout
	}
	if config.RequestTimeout > 0 {
		requestTimeout = config.RequestTimeout
	}
	if config.APIKey != "" {
		apiKey = config.APIKey
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: connectTimeout,
		}).DialContext,
	}

	httpClient := &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}

	return &ResponsesClient{
		baseURL:    strings.TrimSuffix(config.URL, "/"),
		httpClient: httpClient,
		apiKey:     apiKey,
		config:     config,
	}, nil
}

// NewResponsesClientWithHTTPClient creates a new Responses client with a custom HTTP client.
// This is useful for testing with mock HTTP servers.
func NewResponsesClientWithHTTPClient(baseURL string, httpClient *http.Client) (*ResponsesClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("NewResponsesClientWithHTTPClient() [responses_client.go]: baseURL cannot be empty")
	}

	if httpClient == nil {
		return nil, fmt.Errorf("NewResponsesClientWithHTTPClient() [responses_client.go]: httpClient cannot be nil")
	}

	return &ResponsesClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
		apiKey:     "test",
		config:     nil,
	}, nil
}

// GetConfig returns the provider configuration for this client.
// Returns nil if client was created without config (e.g., in tests).
func (c *ResponsesClient) GetConfig() *conf.ModelProviderConfig {
	return c.config
}

func (c *ResponsesClient) applyConfiguredHeaders(req *http.Request) {
	if c == nil || c.config == nil || len(c.config.Headers) == 0 {
		return
	}

	for name, value := range c.config.Headers {
		if name == "" || value == "" {
			continue
		}
		if req.Header.Get(name) != "" {
			continue
		}
		req.Header.Set(name, value)
	}
}

// ChatModel returns a ChatModel implementation for the given model and options.
func (c *ResponsesClient) ChatModel(model string, options *ChatOptions) ChatModel {
	mergedOptions := options
	if c.config != nil && c.config.Verbose {
		if mergedOptions == nil {
			mergedOptions = &ChatOptions{}
		}
		if !mergedOptions.Verbose {
			mergedOptions.Verbose = c.config.Verbose
		}
	}
	return &ResponsesChatModel{
		client:  c,
		model:   model,
		options: mergedOptions,
	}
}

// EmbeddingModel returns an EmbeddingModel implementation for the given model.
// Note: Responses API doesn't support embeddings.
func (c *ResponsesClient) EmbeddingModel(model string) EmbeddingModel {
	return &ResponsesEmbeddingModel{
		client: c,
		model:  model,
	}
}

// ListModels lists all available models.
func (c *ResponsesClient) ListModels() ([]ModelInfo, error) {
	url := c.baseURL + "/models"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ResponsesClient.ListModels() [responses_client.go]: failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	setUserAgentHeader(req)
	c.applyConfiguredHeaders(req)
	applyOptionsHeaders(req, nil)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, c.handleHTTPError(err)
	}
	defer resp.Body.Close()

	if err := c.checkStatusCode(resp); err != nil {
		return nil, err
	}

	var response OpenaiModelList
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("ResponsesClient.ListModels() [responses_client.go]: failed to decode response: %w", err)
	}

	result := make([]ModelInfo, len(response.Data))
	for i, model := range response.Data {
		result[i] = ModelInfo{
			Name:       model.ID,
			Model:      model.ID,
			ModifiedAt: time.Unix(model.Created, 0).Format(time.RFC3339),
			Size:       0,
			Family:     model.OwnedBy,
		}
	}

	return result, nil
}

// Chat sends a chat request and returns the response.
func (m *ResponsesChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client.go]: messages cannot be nil or empty")
	}

	if m.model == "" {
		return nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client.go]: model not set")
	}

	effectiveOptions := options
	if effectiveOptions == nil {
		effectiveOptions = m.options
	}

	items, err := convertToResponsesItems(messages)
	if err != nil {
		return nil, err
	}

	chatReq := ResponsesCreateRequest{
		Model:           m.model,
		Input:           items,
		Tools:           convertToolsToResponses(tools),
		Stream:          false,
		MaxOutputTokens: DefaultMaxTokens,
	}

	if m.client.config != nil && m.client.config.MaxTokens > 0 {
		chatReq.MaxOutputTokens = m.client.config.MaxTokens
	}

	if effectiveOptions != nil {
		chatReq.Temperature = float64(effectiveOptions.Temperature)
		chatReq.TopP = float64(effectiveOptions.TopP)
		if effectiveOptions.SessionID != "" {
			chatReq.PromptCacheKey = effectiveOptions.SessionID
		}
	}

	url := m.client.baseURL + "/responses"

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client.go]: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client.go]: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if m.client.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.client.apiKey)
	}
	setUserAgentHeader(req)
	m.client.applyConfiguredHeaders(req)
	applyOptionsHeaders(req, effectiveOptions)

	logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)
	if effectiveOptions != nil && effectiveOptions.Logger != nil {
		logHTTPRequestWithObfuscation(effectiveOptions.Logger, req, chatReq)
	}

	resp, err := m.client.httpClient.Do(req)
	if err != nil {
		return nil, m.client.handleHTTPError(err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client.go]: failed to read response body: %w", err)
	}

	if effectiveOptions != nil && effectiveOptions.Verbose {
		logVerboseResponseFromBytes(resp, bodyBytes)
	}

	if err := m.client.checkStatusCodeWithBody(resp, bodyBytes); err != nil {
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
		}
		return nil, err
	}

	var chatResp ResponsesResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client.go]: failed to decode response: %w", err)
	}

	if effectiveOptions != nil && effectiveOptions.Logger != nil {
		logHTTPResponseWithObfuscation(effectiveOptions.Logger, resp, chatResp)
	}

	result, err := convertFromResponsesOutput(chatResp.Output)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses.
func (m *ResponsesChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		if len(messages) == 0 {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: messages cannot be empty\n")
			return
		}

		if m.model == "" {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: model cannot be empty\n")
			return
		}

		effectiveOptions := options
		if effectiveOptions == nil {
			effectiveOptions = m.options
		}

		items, err := convertToResponsesItems(messages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: %v\n", err)
			return
		}

		chatReq := ResponsesCreateRequest{
			Model:           m.model,
			Input:           items,
			Tools:           convertToolsToResponses(tools),
			Stream:          true,
			MaxOutputTokens: DefaultMaxTokens,
		}

		if m.client.config != nil && m.client.config.MaxTokens > 0 {
			chatReq.MaxOutputTokens = m.client.config.MaxTokens
		}

		if effectiveOptions != nil {
			chatReq.Temperature = float64(effectiveOptions.Temperature)
			chatReq.TopP = float64(effectiveOptions.TopP)
			if effectiveOptions.SessionID != "" {
				chatReq.PromptCacheKey = effectiveOptions.SessionID
			}
		}

		url := m.client.baseURL + "/responses"

		body, err := json.Marshal(chatReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: failed to marshal request: %v\n", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: failed to create request: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		if m.client.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+m.client.apiKey)
		}
		setUserAgentHeader(req)
		m.client.applyConfiguredHeaders(req)
		applyOptionsHeaders(req, effectiveOptions)

		logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPRequestWithObfuscation(effectiveOptions.Logger, req, chatReq)
		}

		resp, err := m.client.httpClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: HTTP request failed: %v\n", err)
			return
		}
		defer resp.Body.Close()

		logVerboseStreamResponseHeaders(resp, effectiveOptions != nil && effectiveOptions.Verbose)

		if err := m.client.checkStatusCode(resp); err != nil {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
			}
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: API error (status %d): %v\n", resp.StatusCode, err)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		toolCallsInProgress := make(map[string]*responsesToolCallInProgress)

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
			}
		}
	}
}

// Embed generates embeddings for the given input text.
// Note: Responses API doesn't support embeddings, so this always returns an error.
func (m *ResponsesEmbeddingModel) Embed(ctx context.Context, input string) ([]float64, error) {
	return nil, fmt.Errorf("ResponsesEmbeddingModel.Embed() [responses_client.go]: not implemented")
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

// handleHTTPError converts HTTP errors to appropriate model errors.
func (c *ResponsesClient) handleHTTPError(err error) error {
	if err == nil {
		return nil
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return fmt.Errorf("%w: %v", ErrEndpointUnavailable, err)
		}
		return fmt.Errorf("%w: %v", ErrEndpointUnavailable, err)
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Errorf("%w: %v", ErrEndpointNotFound, err)
	}

	return fmt.Errorf("%w: %v", ErrEndpointUnavailable, err)
}

// checkStatusCode checks the HTTP status code and returns appropriate errors.
func (c *ResponsesClient) checkStatusCode(resp *http.Response) error {
	return c.checkStatusCodeWithBody(resp, nil)
}

// checkStatusCodeWithBody checks the HTTP status code and returns appropriate errors.
// bodyBytes can be provided if the body has already been read (for error message extraction).
func (c *ResponsesClient) checkStatusCodeWithBody(resp *http.Response, bodyBytes []byte) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return fmt.Errorf("%w: %s", ErrEndpointNotFound, string(bodyBytes))
	case http.StatusUnauthorized, http.StatusForbidden:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return fmt.Errorf("%w: %s", ErrPermissionDenied, string(bodyBytes))
	case http.StatusTooManyRequests:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return c.handleRateLimitErrorWithBody(resp, bodyBytes)
	case http.StatusBadRequest:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		bodyStr := string(bodyBytes)

		var errResp OpenaiErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error != nil {
			if strings.Contains(strings.ToLower(errResp.Error.Message), "context length") ||
				strings.Contains(strings.ToLower(errResp.Error.Message), "too many tokens") ||
				strings.Contains(strings.ToLower(errResp.Error.Message), "maximum context length") {
				return fmt.Errorf("%w: %s", ErrTooManyInputTokens, errResp.Error.Message)
			}
			return fmt.Errorf("bad request: %s", errResp.Error.Message)
		}

		if strings.Contains(strings.ToLower(bodyStr), "context length") ||
			strings.Contains(strings.ToLower(bodyStr), "too many tokens") {
			return fmt.Errorf("%w: %s", ErrTooManyInputTokens, bodyStr)
		}
		return fmt.Errorf("bad request: %s", bodyStr)
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return fmt.Errorf("%w: %s", ErrEndpointUnavailable, string(bodyBytes))
	default:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

// handleRateLimitError handles rate limit (429) errors and extracts retry information.
func (c *ResponsesClient) handleRateLimitError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return c.handleRateLimitErrorWithBody(resp, body)
}

// handleRateLimitErrorWithBody handles rate limit (429) errors and extracts retry information.
func (c *ResponsesClient) handleRateLimitErrorWithBody(resp *http.Response, bodyBytes []byte) error {
	bodyStr := string(bodyBytes)

	retryAfter := 0

	// Try to parse Retry-After header
	if retryAfterHeader := resp.Header.Get("Retry-After"); retryAfterHeader != "" {
		if seconds, err := strconv.Atoi(retryAfterHeader); err == nil {
			retryAfter = seconds
		}
	}

	// Try to parse error response for retry information
	var errResp OpenaiErrorResponse
	if json.Unmarshal(bodyBytes, &errResp) == nil && errResp.Error != nil {
		return &RateLimitError{
			RetryAfterSeconds: retryAfter,
			Message:           errResp.Error.Message,
		}
	}

	return &RateLimitError{
		RetryAfterSeconds: retryAfter,
		Message:           bodyStr,
	}
}

func convertToResponsesItems(messages []*ChatMessage) ([]ResponsesItem, error) {
	var items []ResponsesItem

	for _, msg := range messages {
		if msg == nil {
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
			items = append(items, ResponsesItem{
				Type:    "message",
				Role:    string(msg.Role),
				Content: contentParts,
			})
		}

		for _, call := range toolCalls {
			if call == nil {
				continue
			}
			argsJSON := "{}"
			if call.Arguments.Raw() != nil {
				if bytes, err := json.Marshal(call.Arguments.Raw()); err == nil {
					argsJSON = string(bytes)
				}
			}
			items = append(items, ResponsesItem{
				Type:      "function_call",
				CallID:    call.ID,
				Name:      call.Function,
				Arguments: argsJSON,
			})
		}

		for _, resp := range toolResponses {
			if resp == nil || resp.Call == nil || resp.Call.ID == "" {
				return nil, fmt.Errorf("convertToResponsesItems() [responses_client.go]: tool response missing call ID")
			}
			output := ""
			if resp.Error != nil {
				output = resp.Error.Error()
			} else if resp.Result.Raw() != nil {
				if bytes, err := json.Marshal(resp.Result.Raw()); err == nil {
					output = string(bytes)
				}
			}
			items = append(items, ResponsesItem{
				Type:   "function_call_output",
				CallID: resp.Call.ID,
				Output: output,
			})
		}
	}

	return items, nil
}

func convertFromResponsesOutput(items []ResponsesItem) (*ChatMessage, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("convertFromResponsesOutput() [responses_client.go]: no output items in response")
	}

	result := &ChatMessage{
		Role:  ChatRoleAssistant,
		Parts: []ChatMessagePart{},
	}

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
			result.Parts = append(result.Parts, ChatMessagePart{
				ToolCall: &tool.ToolCall{
					ID:        item.CallID,
					Function:  item.Name,
					Arguments: args,
				},
			})
		}
	}

	if len(result.Parts) == 0 {
		return nil, fmt.Errorf("convertFromResponsesOutput() [responses_client.go]: no usable output items in response")
	}

	return result, nil
}

func convertToolsToResponses(tools []tool.ToolInfo) []ResponsesTool {
	if len(tools) == 0 {
		return nil
	}

	converted := make([]ResponsesTool, len(tools))
	for i, t := range tools {
		schemaJSON, _ := json.Marshal(t.Schema)
		var schemaMap map[string]interface{}
		json.Unmarshal(schemaJSON, &schemaMap)

		converted[i] = ResponsesTool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schemaMap,
		}
	}

	return converted
}
