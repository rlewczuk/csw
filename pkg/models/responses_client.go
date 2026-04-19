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
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/tool"
)

const responsesPromptCacheRetentionOptionKey = "prompt_cache_retention"

// defaultResponsesInstructions is used when no explicit instructions are provided.
const defaultResponsesInstructions string = "You are a helpful assistant."

// ResponsesClient is a client for interacting with Open Responses-compatible APIs.
type ResponsesClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	config     *conf.ModelProviderConfig
	// tokenExpiry is the expiration time of the current access token.
	tokenExpiry time.Time
	// tokenMu protects concurrent access to apiKey and tokenExpiry.
	tokenMu sync.RWMutex
	// configUpdater is an optional callback for persisting configuration changes
	// (e.g., updated refresh tokens after OAuth2 token renewal).
	configUpdater ConfigUpdater
	// verbose enables logging of HTTP requests and responses.
	verbose bool
	// rawLLMCallback receives raw, line-based LLM communication logs.
	rawLLMCallback func(string)
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

// responsesRateLimitErrorBody represents a 429 error response payload.
type responsesRateLimitErrorBody struct {
	Error *responsesRateLimitErrorDetails `json:"error"`
}

// responsesRateLimitErrorDetails represents rate-limit metadata returned in error payloads.
type responsesRateLimitErrorDetails struct {
	Type            string `json:"type"`
	Message         string `json:"message"`
	ResetsInSeconds int    `json:"resets_in_seconds"`
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

	client := &ResponsesClient{
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

// SetConfigUpdater sets a callback function that will be called to persist
// configuration changes (e.g., updated API keys or refresh tokens after OAuth2
// token renewal). This method should be called after creating the client if
// configuration persistence is needed.
func (c *ResponsesClient) SetConfigUpdater(updater ConfigUpdater) {
	c.configUpdater = updater
}

// SetVerbose enables or disables verbose logging for HTTP requests and responses.
func (c *ResponsesClient) SetVerbose(verbose bool) {
	c.verbose = verbose
}

// SetRawLLMCallback sets callback for raw, line-based LLM communication logs.
func (c *ResponsesClient) SetRawLLMCallback(callback func(string)) {
	c.rawLLMCallback = callback
}

func (c *ResponsesClient) emitRawLLMLine(line string) {
	if c == nil || c.rawLLMCallback == nil {
		return
	}
	c.rawLLMCallback(line)
}

func (c *ResponsesClient) emitRawRequest(req *http.Request, body []byte) {
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

func (c *ResponsesClient) emitRawResponse(resp *http.Response, body []byte) {
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

func (c *ResponsesClient) emitRawStreamChunk(line string) {
	if c == nil || c.rawLLMCallback == nil {
		return
	}

	c.emitRawLLMLine("<<< CHUNK " + obfuscateBodyWithRegex(line))
}


func (c *ResponsesClient) promptCacheRetention() string {
	if c == nil || c.config == nil || c.config.Options == nil {
		return ""
	}

	retentionRaw, ok := c.config.Options[responsesPromptCacheRetentionOptionKey]
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

func (c *ResponsesClient) applyConfiguredHeaders(req *http.Request) {
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

func (c *ResponsesClient) applyConfiguredQueryParams(req *http.Request) {
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

	executeRequest := func(token string) (*http.Response, []byte, error) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("ResponsesClient.ListModels() [responses_client.go]: failed to create request: %w", err)
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
			return nil, nil, c.handleHTTPError(err)
		}
		defer resp.Body.Close()

		bodyBytes, err := logVerboseResponse(resp, c.verbose)
		if err != nil {
			return nil, nil, err
		}

		return resp, bodyBytes, nil
	}

	token, err := c.GetAccessToken()
	if err != nil {
		return nil, err
	}

	resp, bodyBytes, err := executeRequest(token)
	if err != nil {
		return nil, err
	}

	if c.shouldRefreshAfterUnauthorized(resp, bodyBytes) {
		if err := c.refreshTokenIfNeeded(true, token); err != nil {
			return nil, err
		}
		token, err = c.GetAccessToken()
		if err != nil {
			return nil, err
		}
		resp, bodyBytes, err = executeRequest(token)
		if err != nil {
			return nil, err
		}
	}

	if err := c.checkStatusCodeWithBody(resp, bodyBytes); err != nil {
		return nil, err
	}

	var response ResponsesModelListResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, fmt.Errorf("ResponsesClient.ListModels() [responses_client.go]: failed to decode response: %w", err)
	}

	result := make([]ModelInfo, 0, len(response.Data)+len(response.Models))
	for _, model := range response.Data {
		result = append(result, ModelInfo{
			Name:       model.ID,
			Model:      model.ID,
			ModifiedAt: time.Unix(model.Created, 0).Format(time.RFC3339),
			Size:       0,
			Family:     model.OwnedBy,
		})
	}

	for _, model := range response.Models {
		modelName := model.ID
		if modelName == "" {
			modelName = model.Slug
		}
		if modelName == "" {
			continue
		}

		modifiedAt := ""
		if model.Created > 0 {
			modifiedAt = time.Unix(model.Created, 0).Format(time.RFC3339)
		}

		family := model.OwnedBy
		if family == "" {
			family = model.DisplayName
		}

		result = append(result, ModelInfo{
			Name:       modelName,
			Model:      modelName,
			ModifiedAt: modifiedAt,
			Size:       0,
			Family:     family,
		})
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
		// Add reasoning/thinking configuration if set
		if effectiveOptions.Thinking != "" {
			chatReq.Reasoning = buildResponsesReasoning(effectiveOptions.Thinking)
		}
	}

	url := m.client.baseURL + "/responses"

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client.go]: failed to marshal request: %w", err)
	}

	executeRequest := func(token string) (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return nil, nil, fmt.Errorf("ResponsesChatModel.Chat() [responses_client.go]: failed to create request: %w", err)
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
			readErr := fmt.Errorf("ResponsesChatModel.Chat() [responses_client.go]: failed to read response body: %w", m.client.handleHTTPError(err))
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
		decodeErr := fmt.Errorf("ResponsesChatModel.Chat() [responses_client.go]: failed to decode response: %w", err)
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

// Compactor returns nil because Responses chat model does not provide session compaction.
func (m *ResponsesChatModel) Compactor() ChatCompator {
	return nil
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
			// Add reasoning/thinking configuration if set
			if effectiveOptions.Thinking != "" {
				chatReq.Reasoning = buildResponsesReasoning(effectiveOptions.Thinking)
			}
		}

		url := m.client.baseURL + "/responses"

		body, err := json.Marshal(chatReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: failed to marshal request: %v\n", err)
			return
		}

		executeRequest := func(token string) (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
			if err != nil {
				return nil, fmt.Errorf("ResponsesChatModel.ChatStream() [responses_client.go]: failed to create request: %w", err)
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
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: failed to get access token: %v\n", err)
			return
		}

		resp, err := executeRequest(token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: HTTP request failed: %v\n", err)
			return
		}

		if m.client.shouldRefreshAfterUnauthorized(resp, nil) {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr == nil && m.client.shouldRefreshAfterUnauthorized(resp, bodyBytes) {
				if refreshErr := m.client.refreshTokenIfNeeded(true, token); refreshErr == nil {
					token, err = m.client.GetAccessToken()
					if err != nil {
						fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: failed to get refreshed access token: %v\n", err)
						return
					}

					resp, err = executeRequest(token)
					if err != nil {
						fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: HTTP request failed after token refresh: %v\n", err)
						return
					}
				} else {
					fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: failed to refresh access token: %v\n", refreshErr)
					return
				}
			} else if readErr != nil {
				fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: failed to read unauthorized response body: %v\n", readErr)
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
			fmt.Fprintf(os.Stderr, "ERROR: ResponsesChatModel.ChatStream() [responses_client.go]: API error (status %d): %v\n", resp.StatusCode, err)
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

// Embed generates embeddings for the given input text.
// Note: Responses API doesn't support embeddings, so this always returns an error.
func (m *ResponsesEmbeddingModel) Embed(ctx context.Context, input string) ([]float64, error) {
	return nil, fmt.Errorf("ResponsesEmbeddingModel.Embed() [responses_client.go]: not implemented")
}

// handleHTTPError converts HTTP errors to appropriate model errors.
// Network errors that can be retried are wrapped in NetworkError.
func (c *ResponsesClient) handleHTTPError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return &NetworkError{
			Message:     fmt.Sprintf("unexpected stream EOF: %v", err),
			IsRetryable: true,
		}
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

	codexRetryAfter := codexLimitRetryAfterSeconds(resp.Header)

	// Try to parse error response for retry information
	var usageLimitResp responsesRateLimitErrorBody
	if json.Unmarshal(bodyBytes, &usageLimitResp) == nil && usageLimitResp.Error != nil {
		retryAfter = mergeRateLimitRetryAfter(
			retryAfter,
			codexRetryAfter,
			usageLimitResp.Error.ResetsInSeconds,
			usageLimitResp.Error.Type,
		)
		return &RateLimitError{
			RetryAfterSeconds: retryAfter,
			Message:           usageLimitResp.Error.Message,
		}
	}

	var errResp OpenaiErrorResponse
	if json.Unmarshal(bodyBytes, &errResp) == nil && errResp.Error != nil {
		retryAfter = mergeRateLimitRetryAfter(retryAfter, codexRetryAfter, 0, errResp.Error.Type)
		return &RateLimitError{
			RetryAfterSeconds: retryAfter,
			Message:           errResp.Error.Message,
		}
	}

	retryAfter = mergeRateLimitRetryAfter(retryAfter, codexRetryAfter, 0, "")

	return &RateLimitError{
		RetryAfterSeconds: retryAfter,
		Message:           bodyStr,
	}
}

// mergeRateLimitRetryAfter merges retry-after candidates based on error semantics.
func mergeRateLimitRetryAfter(retryAfter int, codexRetryAfter int, bodyRetryAfter int, errorType string) int {
	errorTypeLower := strings.ToLower(strings.TrimSpace(errorType))
	if errorTypeLower == "usage_limit_reached" {
		if codexRetryAfter > 0 {
			return codexRetryAfter
		}
		if bodyRetryAfter > 0 {
			return bodyRetryAfter
		}
		return retryAfter
	}

	if retryAfter > 0 {
		return retryAfter
	}
	if bodyRetryAfter > 0 {
		return bodyRetryAfter
	}
	if codexRetryAfter > 0 {
		return codexRetryAfter
	}

	return 0
}

// codexLimitRetryAfterSeconds resolves retry-after from Codex-specific limit headers.
func codexLimitRetryAfterSeconds(headers http.Header) int {
	if len(headers) == 0 {
		return 0
	}

	primaryUsedPercent := parseIntHeader(headers, "X-Codex-Primary-Used-Percent")
	secondaryUsedPercent := parseIntHeader(headers, "X-Codex-Secondary-Used-Percent")
	primaryResetAfter := parseIntHeader(headers, "X-Codex-Primary-Reset-After-Seconds")
	secondaryResetAfter := parseIntHeader(headers, "X-Codex-Secondary-Reset-After-Seconds")
	activeLimit := strings.ToLower(strings.TrimSpace(headers.Get("X-Codex-Active-Limit")))

	if strings.Contains(activeLimit, "primary") && primaryResetAfter > 0 {
		return primaryResetAfter
	}
	if strings.Contains(activeLimit, "secondary") && secondaryResetAfter > 0 {
		return secondaryResetAfter
	}

	primaryExceeded := primaryUsedPercent >= 100
	secondaryExceeded := secondaryUsedPercent >= 100

	if primaryExceeded && !secondaryExceeded {
		return primaryResetAfter
	}
	if secondaryExceeded && !primaryExceeded {
		return secondaryResetAfter
	}
	if primaryExceeded && secondaryExceeded {
		return maxInt(primaryResetAfter, secondaryResetAfter)
	}

	return 0
}

// parseIntHeader parses an integer header value and returns zero on parse failure.
func parseIntHeader(headers http.Header, key string) int {
	value := strings.TrimSpace(headers.Get(key))
	if value == "" {
		return 0
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}

	return parsed
}

// maxInt returns the larger of two integers.
func maxInt(a int, b int) int {
	if a > b {
		return a
	}

	return b
}

func wrapLLMRequestError(err error, resp *http.Response, bodyBytes []byte) error {
	if err == nil {
		return nil
	}

	var llmReqErr *LLMRequestError
	if errors.As(err, &llmReqErr) {
		if strings.TrimSpace(llmReqErr.RawResponse) == "" && resp != nil {
			llmReqErr.RawResponse = formatRawHTTPResponse(resp.StatusCode, resp.Header, bodyBytes)
		}
		return err
	}

	rawResponse := ""
	if resp != nil {
		rawResponse = formatRawHTTPResponse(resp.StatusCode, resp.Header, bodyBytes)
	}

	return &LLMRequestError{
		Err:         err,
		RawResponse: rawResponse,
	}
}

func formatRawHTTPResponse(statusCode int, headers http.Header, bodyBytes []byte) string {
	var responseBuilder strings.Builder

	responseBuilder.WriteString(strconv.Itoa(statusCode))
	responseBuilder.WriteString("\n")

	if headers != nil {
		headerKeys := make([]string, 0, len(headers))
		for key := range headers {
			headerKeys = append(headerKeys, key)
		}
		sort.Strings(headerKeys)

		for _, key := range headerKeys {
			for _, value := range headers.Values(key) {
				responseBuilder.WriteString(key)
				responseBuilder.WriteString(": ")
				responseBuilder.WriteString(value)
				responseBuilder.WriteString("\n")
			}
		}
	}

	responseBuilder.WriteString("\n")
	responseBuilder.Write(bodyBytes)

	return responseBuilder.String()
}
