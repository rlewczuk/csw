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
	"strings"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/tool"
)

// OpenAIClient is a client for interacting with OpenAI-compatible API
type OpenAIClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	config     *conf.ModelProviderConfig
}

// OpenAIChatModel is a chat model implementation for OpenAI
type OpenAIChatModel struct {
	client  *OpenAIClient
	model   string
	options *ChatOptions
}

// OpenAIEmbeddingModel is an embedding model implementation for OpenAI
type OpenAIEmbeddingModel struct {
	client *OpenAIClient
	model  string
}

// NewOpenAIClient creates a new OpenAI-compatible client with the given config
func NewOpenAIClient(config *conf.ModelProviderConfig) (*OpenAIClient, error) {
	if config == nil {
		return nil, fmt.Errorf("NewOpenAIClient() [openai_client.go]: config cannot be nil")
	}

	if config.URL == "" {
		return nil, fmt.Errorf("NewOpenAIClient() [openai_client.go]: URL cannot be empty")
	}

	// Default options
	connectTimeout := 10 * time.Second
	requestTimeout := 60 * time.Second
	apiKey := "ollama" // Default API key for Ollama

	if config.ConnectTimeout > 0 {
		connectTimeout = config.ConnectTimeout
	}
	if config.RequestTimeout > 0 {
		requestTimeout = config.RequestTimeout
	}
	if config.APIKey != "" {
		apiKey = config.APIKey
	}

	// Create HTTP client with custom transport for connection timeout
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: connectTimeout,
		}).DialContext,
	}

	httpClient := &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}

	return &OpenAIClient{
		baseURL:    strings.TrimSuffix(config.URL, "/"),
		httpClient: httpClient,
		apiKey:     apiKey,
		config:     config,
	}, nil
}

// NewOpenAIClientWithHTTPClient creates a new OpenAI-compatible client with a custom HTTP client.
// This is useful for testing with mock HTTP servers.
func NewOpenAIClientWithHTTPClient(baseURL string, httpClient *http.Client) (*OpenAIClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("NewOpenAIClientWithHTTPClient() [openai_client.go]: baseURL cannot be empty")
	}

	if httpClient == nil {
		return nil, fmt.Errorf("NewOpenAIClientWithHTTPClient() [openai_client.go]: httpClient cannot be nil")
	}

	return &OpenAIClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
		apiKey:     "test", // Default API key for testing
		config:     nil,    // No config for test clients
	}, nil
}

// GetConfig returns the provider configuration for this client.
// Returns nil if client was created without config (e.g., in tests).
func (c *OpenAIClient) GetConfig() *conf.ModelProviderConfig {
	return c.config
}

// ChatModel returns a ChatModel implementation for the given model and options
func (c *OpenAIClient) ChatModel(model string, options *ChatOptions) ChatModel {
	// Merge config verbose flag with provided options
	mergedOptions := options
	if c.config != nil && c.config.Verbose {
		if mergedOptions == nil {
			mergedOptions = &ChatOptions{}
		}
		// Config verbose flag sets default, but explicit option overrides
		if !mergedOptions.Verbose {
			mergedOptions.Verbose = c.config.Verbose
		}
	}
	return &OpenAIChatModel{
		client:  c,
		model:   model,
		options: mergedOptions,
	}
}

// EmbeddingModel returns an EmbeddingModel implementation for the given model
func (c *OpenAIClient) EmbeddingModel(model string) EmbeddingModel {
	return &OpenAIEmbeddingModel{
		client: c,
		model:  model,
	}
}

// ListModels lists all available models
func (c *OpenAIClient) ListModels() ([]ModelInfo, error) {
	url := c.baseURL + "/models"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert from OpenAI OpenaiModelData to models.ModelInfo
	result := make([]ModelInfo, len(response.Data))
	for i, model := range response.Data {
		result[i] = ModelInfo{
			Name:       model.ID,
			Model:      model.ID,
			ModifiedAt: time.Unix(model.Created, 0).Format(time.RFC3339),
			Size:       0, // OpenAI API doesn't provide size
			Family:     model.OwnedBy,
		}
	}

	return result, nil
}

// Chat sends a chat request and returns the response
func (m *OpenAIChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("OpenAIChatModel.Chat() [openai_client.go]: messages cannot be nil or empty")
	}

	if m.model == "" {
		return nil, fmt.Errorf("OpenAIChatModel.Chat() [openai_client.go]: model not set")
	}

	// Use provided options or fall back to model's default options
	effectiveOptions := options
	if effectiveOptions == nil {
		effectiveOptions = m.options
	}

	// Convert messages to OpenAI format
	openaiMessages := make([]OpenaiChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = convertToOpenAIMessage(msg)
	}

	// Build request
	chatReq := OpenaiChatCompletionRequest{
		Model:    m.model,
		Messages: openaiMessages,
		Stream:   false,
		Tools:    convertToolsToOpenAI(tools),
	}

	// Apply options if provided
	if effectiveOptions != nil {
		chatReq.Temperature = float64(effectiveOptions.Temperature)
		chatReq.TopP = float64(effectiveOptions.TopP)
		// Note: OpenAI API doesn't have TopK parameter
	}

	url := m.client.baseURL + "/chat/completions"

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.client.apiKey)

	// Print verbose request output if enabled
	logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)

	resp, err := m.client.httpClient.Do(req)
	if err != nil {
		return nil, m.client.handleHTTPError(err)
	}
	defer resp.Body.Close()

	// Log response before checking status (so errors are also logged)
	if err := wrapResponseBodyForLogging(resp, effectiveOptions != nil && effectiveOptions.Verbose); err != nil {
		return nil, err
	}

	if err := m.client.checkStatusCode(resp); err != nil {
		return nil, err
	}

	// Read the response body (already logged if verbose)
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var chatResp OpenaiChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAIChatModel.Chat() [openai_client.go]: no choices in response")
	}

	// Get the first choice
	choice := chatResp.Choices[0]
	if choice.Message == nil {
		return nil, fmt.Errorf("OpenAIChatModel.Chat() [openai_client.go]: no message in choice")
	}

	// Convert response to models.ChatMessage
	result := convertFromOpenAIMessage(choice.Message)
	return result, nil
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses
func (m *OpenAIChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		// Validate inputs
		if len(messages) == 0 {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client.go]: messages cannot be empty\n")
			return
		}

		if m.model == "" {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client.go]: model cannot be empty\n")
			return
		}

		// Use provided options or fall back to model's default options
		effectiveOptions := options
		if effectiveOptions == nil {
			effectiveOptions = m.options
		}

		// Convert messages to OpenAI format
		openaiMessages := make([]OpenaiChatCompletionMessage, len(messages))
		for i, msg := range messages {
			openaiMessages[i] = convertToOpenAIMessage(msg)
		}

		// Build request with streaming enabled
		chatReq := OpenaiChatCompletionRequest{
			Model:    m.model,
			Messages: openaiMessages,
			Stream:   true,
			Tools:    convertToolsToOpenAI(tools),
		}

		// Apply options if provided
		if effectiveOptions != nil {
			chatReq.Temperature = float64(effectiveOptions.Temperature)
			chatReq.TopP = float64(effectiveOptions.TopP)
			// Note: OpenAI API doesn't have TopK parameter
		}

		url := m.client.baseURL + "/chat/completions"

		body, err := json.Marshal(chatReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client.go]: failed to marshal request: %v\n", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client.go]: failed to create request: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+m.client.apiKey)
		req.Header.Set("Accept", "text/event-stream")

		// Print verbose request output if enabled
		logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)

		resp, err := m.client.httpClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client.go]: HTTP request failed: %v\n", err)
			return
		}
		defer resp.Body.Close()

		// Log response headers before checking status (so errors are also logged)
		logVerboseStreamResponseHeaders(resp, effectiveOptions != nil && effectiveOptions.Verbose)

		if err := m.client.checkStatusCode(resp); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client.go]: API error (status %d): %v\n", resp.StatusCode, err)
			return
		}

		// Create scanner for SSE and stream responses
		scanner := bufio.NewScanner(resp.Body)

		// Track accumulated tool calls across chunks
		toolCallsInProgress := make(map[int]*OpenaiToolCall)

		for scanner.Scan() {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()

			// Skip empty lines
			if line == "" {
				continue
			}

			// Print raw line in verbose mode
			if effectiveOptions != nil && effectiveOptions.Verbose {
				fmt.Println(line)
			}

			// Check for [DONE] marker
			if strings.TrimSpace(line) == "data: [DONE]" {
				if effectiveOptions != nil && effectiveOptions.Verbose {
					fmt.Println("=== End of Streaming Response ===")
					fmt.Println()
				}
				return
			}

			// Process SSE data line
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				var chatResp OpenaiChatCompletionResponse
				if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
					// Skip invalid JSON (might be [DONE] or other markers)
					continue
				}

				if len(chatResp.Choices) == 0 {
					continue
				}

				choice := chatResp.Choices[0]

				// Check finish reason
				if choice.FinishReason != "" {
					// If there's content or tool calls in the delta, yield it before ending
					if choice.Delta != nil {
						// Yield any remaining content
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
								Parts: []ChatMessagePart{
									{Text: content},
								},
							}
							if !yield(result) {
								return
							}
						}

						// Yield any tool calls accumulated so far
						if len(toolCallsInProgress) > 0 {
							result := &ChatMessage{
								Role:  ChatRoleAssistant,
								Parts: []ChatMessagePart{},
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
					return
				}

				// Process delta
				if choice.Delta != nil {
					// Handle text content
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
							Parts: []ChatMessagePart{
								{Text: content},
							},
						}
						if !yield(result) {
							return
						}
					}

					// Handle tool calls in delta
					// OpenaiTool calls are streamed incrementally
					if len(choice.Delta.ToolCalls) > 0 {
						for _, tcDelta := range choice.Delta.ToolCalls {
							// Get or create the tool call in progress
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
								// Accumulate the arguments
								if tcDelta.Function.Arguments != "" {
									tc.Function.Arguments += tcDelta.Function.Arguments
								}
								// Update fields if they were sent
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

		// Check for scanner error
		if err := scanner.Err(); err != nil {
			return
		}
	}
}

// Embed generates embeddings for the given input text
func (m *OpenAIEmbeddingModel) Embed(ctx context.Context, input string) ([]float64, error) {
	if input == "" {
		return nil, fmt.Errorf("OpenAIEmbeddingModel.Embed() [openai_client.go]: input cannot be empty")
	}

	if m.model == "" {
		return nil, fmt.Errorf("OpenAIEmbeddingModel.Embed() [openai_client.go]: model not set")
	}

	embedReq := OpenaiEmbeddingRequest{
		Model: m.model,
		Input: input,
	}

	url := m.client.baseURL + "/embeddings"

	body, err := json.Marshal(embedReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.client.apiKey)

	resp, err := m.client.httpClient.Do(req)
	if err != nil {
		return nil, m.client.handleHTTPError(err)
	}
	defer resp.Body.Close()

	if err := m.client.checkStatusCode(resp); err != nil {
		return nil, err
	}

	var embedResp OpenaiEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embedResp.Data) == 0 {
		return nil, fmt.Errorf("OpenAIEmbeddingModel.Embed() [openai_client.go]: no embeddings returned")
	}

	return embedResp.Data[0].Embedding, nil
}

// handleHTTPError converts HTTP errors to appropriate model errors
func (c *OpenAIClient) handleHTTPError(err error) error {
	if err == nil {
		return nil
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return fmt.Errorf("%w: %v", ErrEndpointUnavailable, err)
		}
		return fmt.Errorf("%w: %v", ErrEndpointUnavailable, err)
	}

	// Check for DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Errorf("%w: %v", ErrEndpointNotFound, err)
	}

	return fmt.Errorf("%w: %v", ErrEndpointUnavailable, err)
}

// checkStatusCode checks the HTTP status code and returns appropriate errors
func (c *OpenAIClient) checkStatusCode(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %s", ErrEndpointNotFound, string(body))
	case http.StatusUnauthorized, http.StatusForbidden:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %s", ErrPermissionDenied, string(body))
	case http.StatusTooManyRequests:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %s", ErrRateExceeded, string(body))
	case http.StatusBadRequest:
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Try to parse error response
		var errResp OpenaiErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
			// Check for context length errors
			if strings.Contains(strings.ToLower(errResp.Error.Message), "context length") ||
				strings.Contains(strings.ToLower(errResp.Error.Message), "too many tokens") ||
				strings.Contains(strings.ToLower(errResp.Error.Message), "maximum context length") {
				return fmt.Errorf("%w: %s", ErrTooManyInputTokens, errResp.Error.Message)
			}
			return fmt.Errorf("bad request: %s", errResp.Error.Message)
		}

		// Fallback to raw body
		if strings.Contains(strings.ToLower(bodyStr), "context length") ||
			strings.Contains(strings.ToLower(bodyStr), "too many tokens") {
			return fmt.Errorf("%w: %s", ErrTooManyInputTokens, bodyStr)
		}
		return fmt.Errorf("bad request: %s", bodyStr)
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %s", ErrEndpointUnavailable, string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}
}

// convertToOpenAIMessage converts a models.ChatMessage to OpenAI OpenaiChatCompletionMessage format
func convertToOpenAIMessage(msg *ChatMessage) OpenaiChatCompletionMessage {
	openaiMsg := OpenaiChatCompletionMessage{
		Role: string(msg.Role),
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
				argsJSON, _ := json.Marshal(part.ToolCall.Arguments.Raw())
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
				} else {
					openaiMsg.ToolCallID = part.ToolResponse.Call.ID
				}
				if part.ToolResponse.Error != nil {
					openaiMsg.Content = part.ToolResponse.Error.Error()
				} else {
					resultJSON, _ := json.Marshal(part.ToolResponse.Result.Raw())
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

// convertToolsToOpenAI converts tool.ToolInfo to OpenAI OpenaiTool format
func convertToolsToOpenAI(tools []tool.ToolInfo) []OpenaiTool {
	if len(tools) == 0 {
		return nil
	}

	openaiTools := make([]OpenaiTool, len(tools))
	for i, t := range tools {
		// Convert ToolSchema to map[string]interface{}
		schemaJSON, _ := json.Marshal(t.Schema)
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
