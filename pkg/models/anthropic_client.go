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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/tool"
)

const anthropicPromptCachingBetaHeaderValue = "prompt-caching-2024-07-31"

// AnthropicClient is a client for interacting with Anthropic API
type AnthropicClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	apiVersion string
	config     *conf.ModelProviderConfig
	// verbose enables logging of HTTP requests and responses.
	verbose bool
	// rawLLMCallback receives raw, line-based LLM communication logs.
	rawLLMCallback func(string)
}

// AnthropicChatModel is a chat model implementation for Anthropic
type AnthropicChatModel struct {
	client  *AnthropicClient
	model   string
	options *ChatOptions
}

// AnthropicEmbeddingModel is a placeholder embedding model (not supported by Anthropic)
type AnthropicEmbeddingModel struct {
	client *AnthropicClient
	model  string
}

// NewAnthropicClient creates a new Anthropic client with the given config
func NewAnthropicClient(config *conf.ModelProviderConfig) (*AnthropicClient, error) {
	if config == nil {
		return nil, fmt.Errorf("NewAnthropicClient() [anthropic_client.go]: config cannot be nil")
	}

	if config.URL == "" {
		return nil, fmt.Errorf("NewAnthropicClient() [anthropic_client.go]: URL cannot be empty")
	}

	// Default options (1h timeouts to accommodate long-running LLM requests)
	connectTimeout := 3600 * time.Second
	requestTimeout := 3600 * time.Second
	apiKey := ""
	apiVersion := "2023-06-01" // Default Anthropic API version

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

	return &AnthropicClient{
		baseURL:    strings.TrimSuffix(config.URL, "/"),
		httpClient: httpClient,
		apiKey:     apiKey,
		apiVersion: apiVersion,
		config:     config,
	}, nil
}

// NewAnthropicClientWithHTTPClient creates a new Anthropic client with the given base URL and custom HTTP client
func NewAnthropicClientWithHTTPClient(baseURL string, httpClient *http.Client) (*AnthropicClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("NewAnthropicClientWithHTTPClient() [anthropic_client.go]: baseURL cannot be empty")
	}

	if httpClient == nil {
		return nil, fmt.Errorf("NewAnthropicClientWithHTTPClient() [anthropic_client.go]: httpClient cannot be nil")
	}

	apiVersion := "2023-06-01" // Default Anthropic API version

	return &AnthropicClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
		apiKey:     "", // API key not required for mock server
		apiVersion: apiVersion,
		config:     nil, // No config for test clients
	}, nil
}

// GetConfig returns the provider configuration for this client.
// Returns nil if client was created without config (e.g., in tests).
func (c *AnthropicClient) GetConfig() *conf.ModelProviderConfig {
	return c.config
}

// SetVerbose enables or disables verbose logging for HTTP requests and responses.
func (c *AnthropicClient) SetVerbose(verbose bool) {
	c.verbose = verbose
}

// SetRawLLMCallback sets callback for raw, line-based LLM communication logs.
func (c *AnthropicClient) SetRawLLMCallback(callback func(string)) {
	c.rawLLMCallback = callback
}

func (c *AnthropicClient) emitRawLLMLine(line string) {
	if c == nil || c.rawLLMCallback == nil {
		return
	}
	c.rawLLMCallback(line)
}

func (c *AnthropicClient) emitRawRequest(req *http.Request, body []byte) {
	if c == nil || c.rawLLMCallback == nil || req == nil {
		return
	}

	c.emitRawLLMLine(">>> REQUEST " + req.Method + " " + req.URL.String())
	obfuscatedHeaders := obfuscateHeaders(req.Header)
	headerKeys := make([]string, 0, len(obfuscatedHeaders))
	for key := range obfuscatedHeaders {
		headerKeys = append(headerKeys, key)
	}
	sort.Strings(headerKeys)
	for _, key := range headerKeys {
		for _, value := range obfuscatedHeaders.Values(key) {
			c.emitRawLLMLine(">>> HEADER " + key + ": " + value)
		}
	}
	if len(body) > 0 {
		c.emitRawLLMLine(">>> BODY " + obfuscateJSONBody(body))
	}
}

func (c *AnthropicClient) emitRawResponse(resp *http.Response, body []byte) {
	if c == nil || c.rawLLMCallback == nil || resp == nil {
		return
	}

	c.emitRawLLMLine("<<< RESPONSE " + strconv.Itoa(resp.StatusCode))
	obfuscatedHeaders := obfuscateHeaders(resp.Header)
	headerKeys := make([]string, 0, len(obfuscatedHeaders))
	for key := range obfuscatedHeaders {
		headerKeys = append(headerKeys, key)
	}
	sort.Strings(headerKeys)
	for _, key := range headerKeys {
		for _, value := range obfuscatedHeaders.Values(key) {
			c.emitRawLLMLine("<<< HEADER " + key + ": " + value)
		}
	}
	if len(body) > 0 {
		c.emitRawLLMLine("<<< BODY " + obfuscateJSONBody(body))
	}
}

func (c *AnthropicClient) emitRawStreamChunk(line string) {
	if c == nil || c.rawLLMCallback == nil {
		return
	}
	c.emitRawLLMLine("<<< CHUNK " + obfuscateBodyWithRegex(line))
}

func (c *AnthropicClient) applyConfiguredHeaders(req *http.Request) {
	if c == nil || c.config == nil || len(c.config.Headers) == 0 {
		return
	}

	headerNames := make([]string, 0, len(c.config.Headers))
	for name := range c.config.Headers {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)

	for _, name := range headerNames {
		value := c.config.Headers[name]
		if name == "" || value == "" {
			continue
		}
		if req.Header.Get(name) != "" {
			continue
		}
		req.Header.Set(name, value)
	}
}

func (c *AnthropicClient) applyConfiguredQueryParams(req *http.Request) {
	if c == nil || c.config == nil || len(c.config.QueryParams) == 0 || req.URL == nil {
		return
	}

	query := req.URL.Query()
	queryParamKeys := make([]string, 0, len(c.config.QueryParams))
	for key := range c.config.QueryParams {
		queryParamKeys = append(queryParamKeys, key)
	}
	sort.Strings(queryParamKeys)

	for _, key := range queryParamKeys {
		value := c.config.QueryParams[key]
		if key == "" || value == "" {
			continue
		}
		if query.Get(key) != "" {
			continue
		}
		query.Set(key, value)
	}

	req.URL.RawQuery = query.Encode()
}

// ChatModel returns a ChatModel implementation for the given model and options
func (c *AnthropicClient) ChatModel(model string, options *ChatOptions) ChatModel {
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
	return &AnthropicChatModel{
		client:  c,
		model:   model,
		options: mergedOptions,
	}
}

// EmbeddingModel returns an EmbeddingModel implementation for the given model
// Note: Anthropic doesn't support embeddings, so this will return an error when used
func (c *AnthropicClient) EmbeddingModel(model string) EmbeddingModel {
	return &AnthropicEmbeddingModel{
		client: c,
		model:  model,
	}
}

// ListModels lists all available models
func (c *AnthropicClient) ListModels() ([]ModelInfo, error) {
	url := c.baseURL + "/v1/models"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", c.apiVersion)
	setUserAgentHeader(req)
	c.applyConfiguredHeaders(req)
	c.applyConfiguredQueryParams(req)
	applyOptionsHeaders(req, nil)

	logVerboseRequest(req, nil, c.verbose)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, c.handleHTTPError(err)
	}
	defer resp.Body.Close()

	bodyBytes, err := logVerboseResponse(resp, c.verbose)
	if err != nil {
		return nil, err
	}

	if err := c.checkStatusCode(resp); err != nil {
		return nil, err
	}

	var response AnthropicModelsListResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert from Anthropic AnthropicModelInfo to models.AnthropicModelInfo
	result := make([]ModelInfo, len(response.Data))
	for i, model := range response.Data {
		result[i] = ModelInfo{
			Name:       model.ID,
			Model:      model.ID,
			ModifiedAt: model.CreatedAt,
			Size:       0, // Anthropic API doesn't provide size
			Family:     model.DisplayName,
		}
	}

	return result, nil
}

// Chat sends a chat request and returns the response
func (m *AnthropicChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("AnthropicChatModel.Chat() [anthropic_client.go]: messages cannot be nil or empty")
	}

	if m.model == "" {
		return nil, fmt.Errorf("AnthropicChatModel.Chat() [anthropic_client.go]: model not set")
	}

	// Use provided options or fall back to model's default options
	effectiveOptions := options
	if effectiveOptions == nil {
		effectiveOptions = m.options
	}

	// Convert messages to Anthropic format
	// Anthropic API requires system messages to be separate
	var systemPrompt string
	var anthropicMessages []AnthropicMessageParam

	for _, msg := range messages {
		if msg.Role == ChatRoleSystem {
			// Accumulate system messages
			if systemPrompt != "" {
				systemPrompt += "\n"
			}
			systemPrompt += msg.GetText()
		} else {
			// Convert message to Anthropic format
			anthropicMsg := convertToAnthropicMessage(msg)
			anthropicMessages = append(anthropicMessages, anthropicMsg)
		}
	}

	// Build request
	chatReq := AnthropicMessagesRequest{
		Model:     m.model,
		Messages:  anthropicMessages,
		MaxTokens: DefaultMaxTokens,
		Stream:    false,
		Tools:     convertToolsToAnthropic(tools),
	}

	// Apply config MaxTokens if set
	if m.client.config != nil && m.client.config.MaxTokens > 0 {
		chatReq.MaxTokens = m.client.config.MaxTokens
	}

	// Add system prompt if present
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
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
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
		return nil, m.client.handleHTTPError(err)
	}
	defer resp.Body.Close()

	// Read response body for logging and processing
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log verbose response if enabled
	if effectiveOptions != nil && effectiveOptions.Verbose {
		logVerboseResponseFromBytes(resp, bodyBytes)
	}
	m.client.emitRawResponse(resp, bodyBytes)

	// Check status code and log error response if needed
	if err := m.client.checkStatusCodeWithBody(resp, bodyBytes); err != nil {
		// Log the error response to structured logger
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
		}
		return nil, err
	}

	var chatResp AnthropicMessagesResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Log response using structured logger if available
	if effectiveOptions != nil && effectiveOptions.Logger != nil {
		logHTTPResponseWithObfuscation(effectiveOptions.Logger, resp, chatResp)
	}

	if len(chatResp.Content) == 0 {
		return nil, fmt.Errorf("AnthropicChatModel.Chat() [anthropic_client.go]: no content in response")
	}

	// Convert response to ChatMessage
	result := convertFromAnthropicResponse(chatResp.Content)
	result.TokenUsage = &TokenUsage{
		InputTokens:          chatResp.Usage.InputTokens,
		InputNonCachedTokens: chatResp.Usage.InputTokens,
		OutputTokens:         chatResp.Usage.OutputTokens,
		TotalTokens:          chatResp.Usage.InputTokens + chatResp.Usage.OutputTokens,
	}
	result.ContextLengthTokens = result.TokenUsage.TotalTokens
	return result, nil
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses
func (m *AnthropicChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		// Validate inputs
		if len(messages) == 0 {
			fmt.Fprintf(os.Stderr, "ERROR: AnthropicChatModel.ChatStream() [anthropic_client.go]: messages cannot be empty\n")
			return
		}

		if m.model == "" {
			fmt.Fprintf(os.Stderr, "ERROR: AnthropicChatModel.ChatStream() [anthropic_client.go]: model cannot be empty\n")
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
			fmt.Fprintf(os.Stderr, "ERROR: AnthropicChatModel.ChatStream() [anthropic_client.go]: failed to create request: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "ERROR: AnthropicChatModel.ChatStream() [anthropic_client.go]: HTTP request failed: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "ERROR: AnthropicChatModel.ChatStream() [anthropic_client.go]: API error (status %d): %v\n", resp.StatusCode, err)
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

// Embed generates embeddings for the given input text
// Note: Anthropic doesn't support embeddings, so this always returns an error
func (m *AnthropicEmbeddingModel) Embed(ctx context.Context, input string) ([]float64, error) {
	return nil, fmt.Errorf("AnthropicEmbeddingModel.Embed() [anthropic_client.go]: not implemented")
}

// handleHTTPError converts HTTP errors to appropriate model errors.
// Network errors that can be retried are wrapped in NetworkError.
func (c *AnthropicClient) handleHTTPError(err error) error {
	if err == nil {
		return nil
	}

	// Check for DNS errors first
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		// Temporary DNS errors (like server misbehaving) should be retried
		if dnsErr.IsTemporary || dnsErr.IsNotFound {
			// IsNotFound means the host doesn't exist - this should NOT be retried
			if dnsErr.IsNotFound && !dnsErr.IsTemporary {
				return fmt.Errorf("%w: %v", ErrEndpointNotFound, err)
			}
			// Temporary DNS issues should be retried
			return &NetworkError{
				Message:     fmt.Sprintf("temporary DNS error: %v", err),
				IsRetryable: true,
			}
		}
		// Other DNS errors (like server misbehaving) should be retried
		return &NetworkError{
			Message:     fmt.Sprintf("DNS error: %v", err),
			IsRetryable: true,
		}
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Timeout and temporary errors can be retried
		if netErr.Timeout() || netErr.Temporary() {
			return &NetworkError{
				Message:     fmt.Sprintf("network timeout: %v", err),
				IsRetryable: true,
			}
		}
		// Connection refused and other network errors can also be retried
		return &NetworkError{
			Message:     fmt.Sprintf("network error: %v", err),
			IsRetryable: true,
		}
	}

	// Check for connection refused errors (can be retried)
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" || opErr.Op == "read" || opErr.Op == "write" {
			return &NetworkError{
				Message:     fmt.Sprintf("connection error: %v", err),
				IsRetryable: true,
			}
		}
	}

	// For other errors, wrap as endpoint unavailable (not retryable by default)
	return fmt.Errorf("%w: %v", ErrEndpointUnavailable, err)
}

// checkStatusCode checks the HTTP status code and returns appropriate errors
func (c *AnthropicClient) checkStatusCode(resp *http.Response) error {
	return c.checkStatusCodeWithBody(resp, nil)
}

// checkStatusCodeWithBody checks the HTTP status code and returns appropriate errors.
// bodyBytes can be provided if the body has already been read (for error message extraction).
func (c *AnthropicClient) checkStatusCodeWithBody(resp *http.Response, bodyBytes []byte) error {
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

		// Try to parse error response
		var errResp AnthropicErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error != nil {
			// Check for context length errors
			msgLower := strings.ToLower(errResp.Error.Message)
			if strings.Contains(msgLower, "context length") ||
				strings.Contains(msgLower, "too many tokens") ||
				strings.Contains(msgLower, "maximum context length") ||
				strings.Contains(msgLower, "exceeded model token limit") ||
				strings.Contains(msgLower, "token limit") {
				return fmt.Errorf("%w: %s", ErrTooManyInputTokens, errResp.Error.Message)
			}
			return fmt.Errorf("bad request: %s", errResp.Error.Message)
		}

		// Fallback to raw body
		bodyLower := strings.ToLower(bodyStr)
		if strings.Contains(bodyLower, "context length") ||
			strings.Contains(bodyLower, "too many tokens") ||
			strings.Contains(bodyLower, "exceeded model token limit") ||
			strings.Contains(bodyLower, "token limit") {
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
func (c *AnthropicClient) handleRateLimitError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return c.handleRateLimitErrorWithBody(resp, body)
}

// handleRateLimitErrorWithBody handles rate limit (429) errors and extracts retry information.
func (c *AnthropicClient) handleRateLimitErrorWithBody(resp *http.Response, bodyBytes []byte) error {
	bodyStr := string(bodyBytes)

	retryAfter := 0

	// Try to parse Retry-After header
	if retryAfterHeader := resp.Header.Get("Retry-After"); retryAfterHeader != "" {
		if seconds, err := strconv.Atoi(retryAfterHeader); err == nil {
			retryAfter = seconds
		}
	}

	// Try to parse error response for retry information
	var errResp AnthropicErrorResponse
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

// convertToAnthropicMessage converts a models.ChatMessage to Anthropic AnthropicMessageParam format
func convertToAnthropicMessage(msg *ChatMessage) AnthropicMessageParam {
	// Check if message contains only plain text (no reasoning, tool calls or tool responses).
	hasOnlyText := true
	for _, part := range msg.Parts {
		if part.ReasoningContent != "" || part.ToolCall != nil || part.ToolResponse != nil {
			hasOnlyText = false
			break
		}
	}

	if hasOnlyText && len(msg.Parts) > 0 {
		// Simple text message
		return AnthropicMessageParam{
			Role:    string(msg.Role),
			Content: msg.GetText(),
		}
	}

	// Message contains tool calls or tool responses - use array of content blocks
	var contentBlocks []AnthropicContentBlock
	for _, part := range msg.Parts {
		if part.Text != "" {
			contentBlocks = append(contentBlocks, AnthropicContentBlock{
				Type: "text",
				Text: part.Text,
			})
		} else if part.ReasoningContent != "" {
			contentBlocks = append(contentBlocks, AnthropicContentBlock{
				Type:      "thinking",
				Thinking:  part.ReasoningContent,
				Signature: part.ReasoningSignature,
			})
		} else if part.ToolCall != nil {
			toolInput := map[string]interface{}{}
			if rawInput, ok := part.ToolCall.Arguments.Raw().(map[string]interface{}); ok {
				toolInput = rawInput
			}
			// AnthropicTool use block
			contentBlocks = append(contentBlocks, AnthropicContentBlock{
				Type:  "tool_use",
				ID:    part.ToolCall.ID,
				Name:  part.ToolCall.Function,
				Input: toolInput,
			})
		} else if part.ToolResponse != nil {
			// AnthropicTool result block
			var content interface{}
			if part.ToolResponse.Error != nil {
				content = part.ToolResponse.Error.Error()
			} else {
				// Anthropic requires content to be a string or array of content blocks
				// Convert result to JSON string
				resultJSON, err := json.Marshal(part.ToolResponse.Result)
				if err != nil {
					content = fmt.Sprintf("Error serializing result: %v", err)
				} else {
					content = string(resultJSON)
				}
			}
			// Prefer Call.ID if available, fall back to ID for backward compatibility
			toolUseID := part.ToolResponse.Call.ID
			if part.ToolResponse.Call != nil {
				toolUseID = part.ToolResponse.Call.ID
			}
			contentBlocks = append(contentBlocks, AnthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: toolUseID,
				Content:   content,
				IsError:   part.ToolResponse.Error != nil,
			})
		}
	}

	return AnthropicMessageParam{
		Role:    string(msg.Role),
		Content: contentBlocks,
	}
}

// convertFromAnthropicResponse converts Anthropic response content to models.ChatMessage
func convertFromAnthropicResponse(content []AnthropicResponseContent) *ChatMessage {
	var parts []ChatMessagePart
	for _, c := range content {
		if c.Type == "text" {
			parts = append(parts, ChatMessagePart{Text: c.Text})
		} else if c.Type == "thinking" {
			parts = append(parts, ChatMessagePart{ReasoningContent: c.Thinking, ReasoningSignature: c.Signature})
		} else if c.Type == "tool_use" {
			parts = append(parts, ChatMessagePart{
				ToolCall: &tool.ToolCall{
					ID:        c.ID,
					Function:  c.Name,
					Arguments: tool.NewToolValue(c.Input),
				},
			})
		}
	}

	return &ChatMessage{
		Role:  ChatRoleAssistant,
		Parts: parts,
	}
}

// buildAnthropicThinking creates an AnthropicThinking struct from a thinking mode string.
// Anthropic Claude 3.7+ supports thinking with a budget of tokens.
// For effort-based values (low, medium, high, xhigh), maps to appropriate budget ratios.
// For boolean values: "true" enables thinking, "false" or empty returns nil.
func buildAnthropicThinking(thinking string, maxTokens int) *AnthropicThinking {
	if thinking == "" || thinking == "false" {
		return nil
	}

	// Calculate budget based on effort level
	// Budget is the number of tokens allocated for thinking
	var budgetRatio float64
	switch thinking {
	case "low":
		budgetRatio = 0.1 // 10% of max tokens
	case "medium":
		budgetRatio = 0.2 // 20% of max tokens
	case "high":
		budgetRatio = 0.3 // 30% of max tokens
	case "xhigh":
		budgetRatio = 0.4 // 40% of max tokens
	case "true":
		budgetRatio = 0.2 // Default to medium (20%)
	default:
		// For unknown values, default to medium
		budgetRatio = 0.2
	}

	budgetTokens := int(float64(maxTokens) * budgetRatio)
	// Ensure minimum budget of 1024 tokens as per Anthropic API requirements
	if budgetTokens < 1024 {
		budgetTokens = 1024
	}
	// Ensure budget doesn't exceed max tokens
	if budgetTokens >= maxTokens {
		budgetTokens = maxTokens - 1
	}

	return &AnthropicThinking{
		Type:         "enabled",
		BudgetTokens: budgetTokens,
	}
}

// convertToolsToAnthropic converts tool.ToolInfo to Anthropic AnthropicTool format
func convertToolsToAnthropic(tools []tool.ToolInfo) []AnthropicTool {
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

	anthropicTools := make([]AnthropicTool, len(orderedTools))
	for i, t := range orderedTools {
		// Convert ToolSchema to map[string]interface{}
		normalizedSchema := normalizeAnthropicToolSchemaForPromptCaching(t.Schema)
		schemaJSON, _ := marshalStableAnthropicJSON(normalizedSchema)
		var schemaMap map[string]interface{}
		json.Unmarshal(schemaJSON, &schemaMap)

		anthropicTools[i] = AnthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schemaMap,
		}
	}
	return anthropicTools
}

// marshalAnthropicMessagesRequest marshals Anthropic request in a deterministic format with prompt-caching hints.
func marshalAnthropicMessagesRequest(req AnthropicMessagesRequest) ([]byte, error) {
	body, err := marshalStableAnthropicJSON(req)
	if err != nil {
		return nil, fmt.Errorf("AnthropicChatModel.marshalAnthropicMessagesRequest() [anthropic_client.go]: failed to marshal base request: %w", err)
	}

	var requestMap map[string]any
	if err := json.Unmarshal(body, &requestMap); err != nil {
		return nil, fmt.Errorf("AnthropicChatModel.marshalAnthropicMessagesRequest() [anthropic_client.go]: failed to unmarshal base request: %w", err)
	}

	applyAnthropicPromptCachingBreakpoints(requestMap)

	stableBody, err := marshalStableAnthropicJSON(requestMap)
	if err != nil {
		return nil, fmt.Errorf("AnthropicChatModel.marshalAnthropicMessagesRequest() [anthropic_client.go]: failed to marshal request map: %w", err)
	}

	return stableBody, nil
}

// applyAnthropicPromptCachingBreakpoints adds cache-control breakpoints for stable prompt-prefix caching.
func applyAnthropicPromptCachingBreakpoints(requestMap map[string]any) {
	if requestMap == nil {
		return
	}

	if markLastToolWithCacheControl(requestMap) {
		return
	}

	markSystemPromptWithCacheControl(requestMap)
}

func markLastToolWithCacheControl(requestMap map[string]any) bool {
	toolsRaw, ok := requestMap["tools"]
	if !ok {
		return false
	}

	tools, ok := toolsRaw.([]any)
	if !ok || len(tools) == 0 {
		return false
	}

	lastIdx := len(tools) - 1
	toolMap, ok := tools[lastIdx].(map[string]any)
	if !ok {
		return false
	}

	toolMap["cache_control"] = map[string]any{"type": "ephemeral"}
	tools[lastIdx] = toolMap
	requestMap["tools"] = tools
	return true
}

func markSystemPromptWithCacheControl(requestMap map[string]any) {
	systemRaw, exists := requestMap["system"]
	if !exists {
		return
	}

	switch system := systemRaw.(type) {
	case string:
		if strings.TrimSpace(system) == "" {
			return
		}
		requestMap["system"] = []any{
			map[string]any{
				"type":          "text",
				"text":          system,
				"cache_control": map[string]any{"type": "ephemeral"},
			},
		}
	case []any:
		if len(system) == 0 {
			return
		}
		lastIdx := len(system) - 1
		blockMap, ok := system[lastIdx].(map[string]any)
		if !ok {
			return
		}
		blockMap["cache_control"] = map[string]any{"type": "ephemeral"}
		system[lastIdx] = blockMap
		requestMap["system"] = system
	}
}

// normalizeAnthropicToolSchemaForPromptCaching normalizes schema slices to reduce prompt-cache misses.
func normalizeAnthropicToolSchemaForPromptCaching(schema tool.ToolSchema) tool.ToolSchema {
	normalized := schema
	normalized.Required = anthropicSortedStringsCopy(schema.Required)
	normalized.Properties = normalizeAnthropicPropertySchemaMapForPromptCaching(schema.Properties)
	return normalized
}

// normalizeAnthropicPropertySchemaMapForPromptCaching normalizes nested property schemas.
func normalizeAnthropicPropertySchemaMapForPromptCaching(properties map[string]tool.PropertySchema) map[string]tool.PropertySchema {
	if len(properties) == 0 {
		return properties
	}

	normalized := make(map[string]tool.PropertySchema, len(properties))
	for key, property := range properties {
		nested := property
		nested.Enum = anthropicSortedStringsCopy(property.Enum)
		nested.Required = anthropicSortedStringsCopy(property.Required)
		nested.Properties = normalizeAnthropicPropertySchemaMapForPromptCaching(property.Properties)
		if property.Items != nil {
			nestedItem := *property.Items
			nestedItem.Enum = anthropicSortedStringsCopy(property.Items.Enum)
			nestedItem.Required = anthropicSortedStringsCopy(property.Items.Required)
			nestedItem.Properties = normalizeAnthropicPropertySchemaMapForPromptCaching(property.Items.Properties)
			nested.Items = &nestedItem
		}
		normalized[key] = nested
	}

	return normalized
}

// anthropicSortedStringsCopy returns a sorted copy of input strings.
func anthropicSortedStringsCopy(input []string) []string {
	if len(input) == 0 {
		return input
	}

	cloned := make([]string, len(input))
	copy(cloned, input)
	sort.Strings(cloned)
	return cloned
}

// marshalStableAnthropicJSON marshals values deterministically by using encoding/json map key ordering.
func marshalStableAnthropicJSON(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshalStableAnthropicJSON() [anthropic_client.go]: failed to marshal value: %w", err)
	}

	return body, nil
}
