package models

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/tool"
)

// DefaultMaxTokens is the default maximum number of tokens to generate in the response.
const DefaultMaxTokens = 32000

const openAIPromptCacheRetentionOptionKey = "prompt_cache_retention"

// OpenAIClient is a client for interacting with OpenAI-compatible API
type OpenAIClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	config     *conf.ModelProviderConfig
	// tokenExpiry is the expiration time of the current access token.
	tokenExpiry time.Time
	// tokenMu protects concurrent access to apiKey and tokenExpiry.
	tokenMu sync.RWMutex
	// configUpdater is an optional callback for persisting configuration changes.
	configUpdater ConfigUpdater
	// verbose enables logging of HTTP requests and responses.
	verbose bool
	// rawLLMCallback receives raw, line-based LLM communication logs.
	rawLLMCallback func(string)
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

	// Default options (1h timeouts to accommodate long-running LLM requests)
	connectTimeout := 3600 * time.Second
	requestTimeout := 3600 * time.Second
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

	client := &OpenAIClient{
		baseURL:    strings.TrimSuffix(config.URL, "/"),
		httpClient: httpClient,
		apiKey:     apiKey,
		config:     config,
	}

	if IsOAuth2Provider(config) && apiKey != "" {
		expiry, err := ExtractJWTExpiry(apiKey)
		if err == nil {
			client.tokenExpiry = expiry
		}
	}

	return client, nil
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

// SetConfigUpdater sets a callback function that will be called to persist
// configuration changes after OAuth2 token renewal.
func (c *OpenAIClient) SetConfigUpdater(updater ConfigUpdater) {
	c.configUpdater = updater
}

// SetVerbose enables or disables verbose logging for HTTP requests and responses.
func (c *OpenAIClient) SetVerbose(verbose bool) {
	c.verbose = verbose
}

// SetRawLLMCallback sets callback for raw, line-based LLM communication logs.
func (c *OpenAIClient) SetRawLLMCallback(callback func(string)) {
	c.rawLLMCallback = callback
}

// emitRawLLMLine emits a single raw line to the callback if set.
func (c *OpenAIClient) emitRawLLMLine(line string) {
	if c == nil || c.rawLLMCallback == nil {
		return
	}
	c.rawLLMCallback(line)
}

// emitRawRequest emits raw request details (method, URL, headers, body) to the callback.
func (c *OpenAIClient) emitRawRequest(req *http.Request, body []byte) {
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

// emitRawResponse emits raw response details (status, headers, body) to the callback.
func (c *OpenAIClient) emitRawResponse(resp *http.Response, body []byte) {
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

// emitRawStreamChunk emits a raw streaming chunk line to the callback.
func (c *OpenAIClient) emitRawStreamChunk(line string) {
	if c == nil || c.rawLLMCallback == nil {
		return
	}
	c.emitRawLLMLine("<<< CHUNK " + obfuscateBodyWithRegex(line))
}

// RefreshTokenIfNeeded checks if the OAuth2 access token needs to be refreshed
// and refreshes it if necessary. It returns an error if the refresh fails.
// For non-OAuth2 providers, this method does nothing and returns nil.
func (c *OpenAIClient) RefreshTokenIfNeeded() error {
	if !IsOAuth2Provider(c.config) {
		return nil
	}

	c.tokenMu.RLock()
	expiry := c.tokenExpiry
	c.tokenMu.RUnlock()

	if !IsTokenExpired(expiry, defaultTokenRefreshMargin) {
		return nil
	}

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if !IsTokenExpired(c.tokenExpiry, defaultTokenRefreshMargin) {
		return nil
	}

	resp, err := RenewToken(c.config, c.httpClient)
	if err != nil {
		return fmt.Errorf("OpenAIClient.RefreshTokenIfNeeded() [openai_client.go]: %w", err)
	}

	c.apiKey = resp.AccessToken
	c.tokenExpiry = CalculateTokenExpiry(resp.ExpiresIn)

	needsPersist := false
	if c.config != nil {
		c.config.APIKey = resp.AccessToken
		needsPersist = true
	}

	if resp.RefreshToken != "" && c.config != nil {
		c.config.RefreshToken = resp.RefreshToken
		needsPersist = true
	}

	if needsPersist && c.configUpdater != nil {
		if err := c.configUpdater(c.config); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: OpenAIClient.RefreshTokenIfNeeded() [openai_client.go]: failed to persist config: %v\n", err)
		}
	}

	return nil
}

// GetAccessToken returns the current access token, refreshing it if necessary.
// For non-OAuth2 providers, it returns the static API key.
func (c *OpenAIClient) GetAccessToken() (string, error) {
	if err := c.RefreshTokenIfNeeded(); err != nil {
		return "", err
	}

	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.apiKey, nil
}

func (c *OpenAIClient) applyConfiguredHeaders(req *http.Request) {
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

func (c *OpenAIClient) applyConfiguredQueryParams(req *http.Request) {
	if c == nil || c.config == nil || len(c.config.QueryParams) == 0 || req.URL == nil {
		return
	}

	query := req.URL.Query()
	queryKeys := make([]string, 0, len(c.config.QueryParams))
	for key := range c.config.QueryParams {
		queryKeys = append(queryKeys, key)
	}
	sort.Strings(queryKeys)

	for _, key := range queryKeys {
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
func (c *OpenAIClient) EmbeddingModel(model string) *OpenAIEmbeddingModel {
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

	token, err := c.GetAccessToken()
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	c.applyConfiguredQueryParams(req)
	setUserAgentHeader(req)
	c.applyConfiguredHeaders(req)
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

	var response OpenaiModelList
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
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
		Model:     m.model,
		Messages:  openaiMessages,
		Stream:    false,
		Tools:     convertToolsToOpenAI(tools),
		MaxTokens: DefaultMaxTokens,
	}
	if len(tools) > 0 {
		chatReq.ToolChoice = "auto"
	}

	// Apply config MaxTokens if set
	if m.client.config != nil && m.client.config.MaxTokens > 0 {
		chatReq.MaxTokens = m.client.config.MaxTokens
	}

	// Apply options if provided
	if effectiveOptions != nil {
		chatReq.Temperature = float64(effectiveOptions.Temperature)
		chatReq.TopP = float64(effectiveOptions.TopP)
		// Note: OpenAI API doesn't have TopK parameter
		// Add reasoning/thinking configuration if set
		if effectiveOptions.Thinking != "" {
			chatReq.ReasoningEffort = mapThinkingToReasoningEffort(effectiveOptions.Thinking)
		}
	}

	url := m.client.baseURL + "/chat/completions"

	body, err := m.client.marshalChatCompletionRequest(chatReq, effectiveOptions)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	token, err := m.client.GetAccessToken()
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	m.client.applyConfiguredQueryParams(req)
	setUserAgentHeader(req)
	m.client.applyConfiguredHeaders(req)
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

	var chatResp OpenaiChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Log response using structured logger if available
	if effectiveOptions != nil && effectiveOptions.Logger != nil {
		logHTTPResponseWithObfuscation(effectiveOptions.Logger, resp, chatResp)
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
	if chatResp.Usage != nil {
		cachedTokens := 0
		if chatResp.Usage.PromptTokensDetails != nil {
			cachedTokens = chatResp.Usage.PromptTokensDetails.CachedTokens
		}
		nonCachedTokens := chatResp.Usage.PromptTokens - cachedTokens
		if nonCachedTokens < 0 {
			nonCachedTokens = 0
		}
		result.TokenUsage = &TokenUsage{
			InputTokens:          chatResp.Usage.PromptTokens,
			InputCachedTokens:    cachedTokens,
			InputNonCachedTokens: nonCachedTokens,
			OutputTokens:         chatResp.Usage.CompletionTokens,
			TotalTokens:          chatResp.Usage.TotalTokens,
		}
		result.ContextLengthTokens = chatResp.Usage.TotalTokens
	}
	return result, nil
}

// Compactor returns nil because OpenAI chat model does not provide session compaction.
func (m *OpenAIChatModel) Compactor() ChatCompator {
	return nil
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
			StreamOptions: &OpenaiStreamOptions{
				IncludeUsage: true,
			},
			Tools:     convertToolsToOpenAI(tools),
			MaxTokens: DefaultMaxTokens,
		}
		if len(tools) > 0 {
			chatReq.ToolChoice = "auto"
		}

		// Apply config MaxTokens if set
		if m.client.config != nil && m.client.config.MaxTokens > 0 {
			chatReq.MaxTokens = m.client.config.MaxTokens
		}

		// Apply options if provided
		if effectiveOptions != nil {
			chatReq.Temperature = float64(effectiveOptions.Temperature)
			chatReq.TopP = float64(effectiveOptions.TopP)
			// Note: OpenAI API doesn't have TopK parameter
			// Add reasoning/thinking configuration if set
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
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client.go]: failed to create request: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		token, err := m.client.GetAccessToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client.go]: failed to get access token: %v\n", err)
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

		// Print verbose request output if enabled
		logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)

		// Log request using structured logger if available
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPRequestWithObfuscation(effectiveOptions.Logger, req, chatReq)
		}
		m.client.emitRawRequest(req, body)

		resp, err := m.client.httpClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client.go]: HTTP request failed: %v\n", err)
			return
		}
		defer resp.Body.Close()

		// Log response headers before checking status (so errors are also logged)
		logVerboseStreamResponseHeaders(resp, effectiveOptions != nil && effectiveOptions.Verbose)
		m.client.emitRawResponse(resp, nil)

		if err := m.client.checkStatusCode(resp); err != nil {
			// Read and log error response body
			bodyBytes, _ := io.ReadAll(resp.Body)
			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
			}
			fmt.Fprintf(os.Stderr, "ERROR: OpenAIChatModel.ChatStream() [openai_client.go]: API error (status %d): %v\n", resp.StatusCode, err)
			return
		}

		// Create scanner for SSE and stream responses
		scanner := bufio.NewScanner(resp.Body)

		// Track accumulated tool calls across chunks
		toolCallsInProgress := make(map[int]*OpenaiToolCall)
		// Track accumulated reasoning content
		var accumulatedReasoningContent string
		usage := TokenUsage{}
		hasUsage := false
		contextLength := 0

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
			m.client.emitRawStreamChunk(line)

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

				// Log each chunk using structured logger if available
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

				// Check finish reason
				if choice.FinishReason != "" {
					// If there's content or tool calls in the delta, yield it before ending
					if choice.Delta != nil {
						// Accumulate any remaining reasoning content
						if choice.Delta.ReasoningContent != "" {
							accumulatedReasoningContent += choice.Delta.ReasoningContent
						}

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
									{Text: content, ReasoningContent: accumulatedReasoningContent},
								},
							}
							accumulatedReasoningContent = ""
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
							// Add reasoning content as first part if any
							if accumulatedReasoningContent != "" {
								result.Parts = append(result.Parts, ChatMessagePart{
									ReasoningContent: accumulatedReasoningContent,
								})
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

				// Process delta
				if choice.Delta != nil {
					// Handle reasoning content (for thinking models like GLM-5)
					// Accumulate it instead of yielding separately
					if choice.Delta.ReasoningContent != "" {
						accumulatedReasoningContent += choice.Delta.ReasoningContent
					}

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
								{Text: content, ReasoningContent: accumulatedReasoningContent},
							},
						}
						accumulatedReasoningContent = ""
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

// marshalChatCompletionRequest marshals chat-completions request with optional prompt cache fields.
func (c *OpenAIClient) marshalChatCompletionRequest(req OpenaiChatCompletionRequest, options *ChatOptions) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAIClient.marshalChatCompletionRequest() [openai_client.go]: failed to marshal base request: %w", err)
	}

	retention := c.promptCacheRetention()
	promptCacheKey := ""
	if options != nil {
		promptCacheKey = strings.TrimSpace(options.SessionID)
	}

	if retention == "" && promptCacheKey == "" {
		return body, nil
	}

	var requestMap map[string]any
	if err := json.Unmarshal(body, &requestMap); err != nil {
		return nil, fmt.Errorf("OpenAIClient.marshalChatCompletionRequest() [openai_client.go]: failed to unmarshal base request: %w", err)
	}

	if promptCacheKey != "" {
		requestMap["prompt_cache_key"] = promptCacheKey
	}
	if retention != "" {
		requestMap["prompt_cache_retention"] = retention
	}

	stableBody, err := json.Marshal(requestMap)
	if err != nil {
		return nil, fmt.Errorf("OpenAIClient.marshalChatCompletionRequest() [openai_client.go]: failed to marshal request map: %w", err)
	}

	return stableBody, nil
}

// promptCacheRetention resolves prompt cache retention policy from provider options.
func (c *OpenAIClient) promptCacheRetention() string {
	if c == nil || c.config == nil || c.config.Options == nil {
		return ""
	}

	retentionRaw, ok := c.config.Options[openAIPromptCacheRetentionOptionKey]
	if !ok || retentionRaw == nil {
		return ""
	}

	retention := strings.TrimSpace(fmt.Sprint(retentionRaw))
	if retention == "" {
		return ""
	}

	switch retention {
	case "in_memory", "24h":
		return retention
	default:
		return ""
	}
}
