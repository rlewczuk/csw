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

// Default token refresh safety margin - refresh token 5 minutes before expiry.
const defaultTokenRefreshMargin = 5 * time.Minute

const oauthRefreshMarginOptionKey = "oauth_refresh_margin"
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

type responsesToolCallInProgress struct {
	CallID    string
	Name      string
	Arguments string
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

// RefreshTokenIfNeeded checks if the OAuth2 access token needs to be refreshed
// and refreshes it if necessary. It returns an error if the refresh fails.
// For non-OAuth2 providers, this method does nothing and returns nil.
// If a ConfigUpdater is set and the token is refreshed successfully, the
// updated configuration (including new refresh token if provided) will be
// persisted using the ConfigUpdater callback.
func (c *ResponsesClient) RefreshTokenIfNeeded() error {
	return c.refreshTokenIfNeeded(false, "")
}

func (c *ResponsesClient) refreshTokenIfNeeded(force bool, previousToken string) error {
	if !IsOAuth2Provider(c.config) {
		return nil
	}

	if !force {
		c.tokenMu.RLock()
		currentToken := c.apiKey
		expiry := c.tokenExpiry
		c.tokenMu.RUnlock()

		// When token expiry cannot be determined (opaque access token), avoid eager
		// refresh and rely on 401-based refresh path.
		if currentToken != "" && expiry.IsZero() {
			return nil
		}

		if !IsTokenExpired(expiry, c.tokenRefreshMargin()) {
			return nil
		}
	}

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if force {
		if previousToken != "" && c.apiKey != "" && c.apiKey != previousToken {
			return nil
		}
	} else {
		if c.apiKey != "" && c.tokenExpiry.IsZero() {
			return nil
		}
		if !IsTokenExpired(c.tokenExpiry, c.tokenRefreshMargin()) {
			return nil
		}
	}

	// Refresh the token
	resp, err := RenewToken(c.config, c.httpClient)
	if err != nil {
		return fmt.Errorf("ResponsesClient.RefreshTokenIfNeeded() [responses_client.go]: %w", err)
	}

	// Update the API key and expiry
	c.apiKey = resp.AccessToken
	c.tokenExpiry = CalculateTokenExpiry(resp.ExpiresIn)

	// Update the stored access token
	needsPersist := false
	if c.config != nil {
		c.config.APIKey = resp.AccessToken
		needsPersist = true
	}

	// Update the refresh token if a new one was provided
	if resp.RefreshToken != "" && c.config != nil {
		c.config.RefreshToken = resp.RefreshToken
		needsPersist = true
	}

	// Persist the configuration if a ConfigUpdater is set and changes were made
	if needsPersist && c.configUpdater != nil {
		if err := c.configUpdater(c.config); err != nil {
			// Log the error but don't fail the token refresh
			// The in-memory config is still updated correctly
			fmt.Fprintf(os.Stderr, "WARNING: ResponsesClient.RefreshTokenIfNeeded() [responses_client.go]: failed to persist config: %v\n", err)
		}
	}

	return nil
}

func (c *ResponsesClient) tokenRefreshMargin() time.Duration {
	if c == nil || c.config == nil || c.config.Options == nil {
		return defaultTokenRefreshMargin
	}

	marginRaw, ok := c.config.Options[oauthRefreshMarginOptionKey]
	if !ok || marginRaw == nil {
		return defaultTokenRefreshMargin
	}

	switch value := marginRaw.(type) {
	case string:
		parsed, err := time.ParseDuration(strings.TrimSpace(value))
		if err == nil && parsed >= 0 {
			return parsed
		}
	case float64:
		if value >= 0 {
			return time.Duration(value * float64(time.Second))
		}
	case int:
		if value >= 0 {
			return time.Duration(value) * time.Second
		}
	case int64:
		if value >= 0 {
			return time.Duration(value) * time.Second
		}
	}

	return defaultTokenRefreshMargin
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

func (c *ResponsesClient) shouldRefreshAfterUnauthorized(resp *http.Response, bodyBytes []byte) bool {
	if !IsOAuth2Provider(c.config) || c.config == nil || c.config.RefreshToken == "" || resp == nil {
		return false
	}

	return isExpiredTokenUnauthorized(resp, bodyBytes)
}

func isExpiredTokenUnauthorized(resp *http.Response, bodyBytes []byte) bool {
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		return false
	}

	wwwAuthenticate := strings.ToLower(resp.Header.Get("WWW-Authenticate"))
	if strings.Contains(wwwAuthenticate, "invalid_token") && strings.Contains(wwwAuthenticate, "expired") {
		return true
	}

	if len(bodyBytes) == 0 {
		return false
	}

	var errResp OpenaiErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil || errResp.Error == nil {
		return false
	}

	message := strings.ToLower(errResp.Error.Message)
	code := strings.ToLower(fmt.Sprintf("%v", errResp.Error.Code))

	if strings.Contains(code, "expired") {
		return true
	}

	if strings.Contains(code, "invalid_token") && strings.Contains(message, "expired") {
		return true
	}

	return strings.Contains(message, "token") && strings.Contains(message, "expired")
}

// GetAccessToken returns the current access token, refreshing it if necessary.
// For non-OAuth2 providers, it returns the static API key.
func (c *ResponsesClient) GetAccessToken() (string, error) {
	if err := c.RefreshTokenIfNeeded(); err != nil {
		return "", err
	}

	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.apiKey, nil
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
		Model:           m.model,
		Input:           items,
		Store:           &store,
		Instructions:    buildResponsesInstructions(messages),
		Tools:           convertToolsToResponses(tools),
		Stream:          stream,
		MaxOutputTokens: maxOutputTokens,
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
			Model:           m.model,
			Input:           items,
			Store:           &store,
			Instructions:    buildResponsesInstructions(messages),
			Tools:           convertToolsToResponses(tools),
			Stream:          true,
			MaxOutputTokens: maxOutputTokens,
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

func convertToResponsesItems(messages []*ChatMessage) ([]ResponsesItem, error) {
	var items []ResponsesItem

	for _, msg := range messages {
		if msg == nil {
			continue
		}

		// Skip system messages - they are extracted into Instructions field instead
		// The codex API does not allow system messages in the input array
		if msg.Role == ChatRoleSystem {
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
				normalizedArgs := normalizeMapOrderValue(call.Arguments.Raw())
				if bytes, err := json.Marshal(normalizedArgs); err == nil {
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
				normalizedResult := normalizeMapOrderValue(resp.Result.Raw())
				if bytes, err := json.Marshal(normalizedResult); err == nil {
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

// buildResponsesInstructions returns request instructions from system/developer messages.
func buildResponsesInstructions(messages []*ChatMessage) string {
	instructions := make([]string, 0)

	for _, msg := range messages {
		if msg == nil {
			continue
		}
		if msg.Role != ChatRoleSystem && msg.Role != ChatRoleDeveloper {
			continue
		}

		for _, part := range msg.Parts {
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			instructions = append(instructions, part.Text)
		}
	}

	if len(instructions) == 0 {
		return defaultResponsesInstructions
	}

	return strings.Join(instructions, "\n\n")
}

// usesCodexCompatibilityEndpoint returns true when backend uses ChatGPT Codex quirks.
func usesCodexCompatibilityEndpoint(baseURL string) bool {
	return strings.Contains(strings.ToLower(baseURL), "/backend-api/codex")
}

// buildResponsesReasoning creates a ResponsesReasoning struct from a thinking mode string.
// For effort-based thinking (low, medium, high, xhigh), it sets the Effort field.
// For boolean thinking (true), it sets a default effort level.
// If thinking is empty or "false", returns nil.
func buildResponsesReasoning(thinking string) *ResponsesReasoning {
	if thinking == "" || thinking == "false" {
		return nil
	}

	// Map of valid effort values for OpenAI Responses API
	switch thinking {
	case "low", "medium", "high", "xhigh":
		return &ResponsesReasoning{
			Effort: thinking,
		}
	case "true":
		// Default to medium effort when boolean true is specified
		return &ResponsesReasoning{
			Effort: "medium",
		}
	default:
		// For any other value, try to use it as an effort level
		// This allows for future extensibility
		return &ResponsesReasoning{
			Effort: thinking,
		}
	}
}

// scanStreamBodyWithAdaptiveBuffer scans stream body lines and doubles scanner buffer on token-too-long errors.
func scanStreamBodyWithAdaptiveBuffer(bodyBytes []byte, handleLine func(line string) bool) error {
	maxTokenSize := bufio.MaxScanTokenSize

	for {
		scanner := bufio.NewScanner(bytes.NewReader(bodyBytes))
		scanner.Buffer(make([]byte, 0, maxTokenSize), maxTokenSize)

		for scanner.Scan() {
			if handleLine(scanner.Text()) {
				return nil
			}
		}

		if err := scanner.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				maxTokenSize *= 2
				continue
			}

			return fmt.Errorf("scanStreamBodyWithAdaptiveBuffer() [responses_client.go]: failed to scan stream body: %w", err)
		}

		return nil
	}
}

// convertFromResponsesStreamBody converts SSE response body into ChatMessage.
func convertFromResponsesStreamBody(bodyBytes []byte) (*ChatMessage, error) {
	result := &ChatMessage{
		Role:  ChatRoleAssistant,
		Parts: []ChatMessagePart{},
	}

	toolCallsInProgress := make(map[string]*responsesToolCallInProgress)
	var streamEventErr error
	err := scanStreamBodyWithAdaptiveBuffer(bodyBytes, func(line string) bool {
		if line == "" || strings.HasPrefix(line, "event: ") {
			return false
		}
		if strings.TrimSpace(line) == "data: [DONE]" {
			return true
		}
		if !strings.HasPrefix(line, "data: ") {
			return false
		}

		data := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(data) == "[DONE]" {
			return true
		}

		var event ResponsesStreamEvent
		if unmarshalErr := json.Unmarshal([]byte(data), &event); unmarshalErr != nil {
			return false
		}

		if event.Type == "error" && event.Error != nil {
			streamEventErr = mapResponsesStreamError(event.Error)
			return true
		}

		switch event.Type {
		case "response.output_text.delta":
			if event.Delta != "" {
				result.Parts = append(result.Parts, ChatMessagePart{Text: event.Delta})
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
				return false
			}
			tc := toolCallsInProgress[event.ItemID]
			if tc == nil {
				return false
			}
			tc.Arguments += event.Delta
		case "response.function_call_arguments.done":
			if event.ItemID == "" {
				return false
			}
			tc := toolCallsInProgress[event.ItemID]
			if tc == nil {
				return false
			}
			if event.Arguments != "" {
				tc.Arguments = event.Arguments
			}
			toolCall := responsesToolCallFromStream(tc)
			if toolCall != nil {
				result.Parts = append(result.Parts, ChatMessagePart{ToolCall: toolCall})
			}
			delete(toolCallsInProgress, event.ItemID)
		}

		return false
	})
	if err != nil {
		return nil, fmt.Errorf("convertFromResponsesStreamBody() [responses_client.go]: failed to scan stream body: %w", err)
	}
	if streamEventErr != nil {
		return nil, streamEventErr
	}

	if len(result.Parts) == 0 {
		return nil, fmt.Errorf("convertFromResponsesStreamBody() [responses_client.go]: no usable output items in response")
	}

	var finalUsage TokenUsage
	contextLength := 0
	err = scanStreamBodyWithAdaptiveBuffer(bodyBytes, func(line string) bool {
		if !strings.HasPrefix(line, "data: ") {
			return false
		}
		data := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(data) == "[DONE]" {
			return true
		}
		var event ResponsesStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return false
		}
		if event.Response != nil && event.Response.Usage != nil {
			finalUsage.InputTokens += event.Response.Usage.InputTokens
			if event.Response.Usage.InputTokensDetails != nil {
				finalUsage.InputCachedTokens += event.Response.Usage.InputTokensDetails.CachedTokens
			}
			finalUsage.InputNonCachedTokens = finalUsage.InputTokens - finalUsage.InputCachedTokens
			if finalUsage.InputNonCachedTokens < 0 {
				finalUsage.InputNonCachedTokens = 0
			}
			finalUsage.OutputTokens += event.Response.Usage.OutputTokens
			if event.Response.Usage.TotalTokens > 0 {
				finalUsage.TotalTokens += event.Response.Usage.TotalTokens
			} else {
				finalUsage.TotalTokens += event.Response.Usage.InputTokens + event.Response.Usage.OutputTokens
			}
			if finalUsage.TotalTokens > 0 {
				contextLength = finalUsage.TotalTokens
			}
		}

		return false
	})
	if err != nil {
		return nil, fmt.Errorf("convertFromResponsesStreamBody() [responses_client.go]: failed to scan stream body: %w", err)
	}
	if finalUsage.TotalTokens > 0 {
		usageCopy := finalUsage
		result.TokenUsage = &usageCopy
		result.ContextLengthTokens = contextLength
	}

	return result, nil
}

func mapResponsesStreamError(apiErr *OpenaiAPIError) error {
	if apiErr == nil {
		return fmt.Errorf("mapResponsesStreamError() [responses_client.go]: empty stream error")
	}

	errorCode := fmt.Sprint(apiErr.Code)
	if errorCode == "context_length_exceeded" {
		return fmt.Errorf("%w: %s", ErrTooManyInputTokens, apiErr.Message)
	}

	return &APIRequestError{
		ErrorType: apiErr.Type,
		Code:      errorCode,
		Param:     fmt.Sprint(apiErr.Param),
		Message:   apiErr.Message,
	}
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
		normalizedSchema := normalizeToolSchema(t.Schema)
		schemaJSON, _ := json.Marshal(normalizedSchema)
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

// normalizeMapOrderValue returns a recursively normalized representation where
// map keys are emitted in lexical order during JSON marshaling.
func normalizeMapOrderValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		normalized := make(map[string]any, len(value))
		for _, key := range keys {
			normalized[key] = normalizeMapOrderValue(value[key])
		}
		return normalized
	case []any:
		normalized := make([]any, len(value))
		for i := range value {
			normalized[i] = normalizeMapOrderValue(value[i])
		}
		return normalized
	default:
		return value
	}
}

// normalizeToolSchema returns schema with sorted object keys and required fields.
func normalizeToolSchema(schema tool.ToolSchema) tool.ToolSchema {
	normalized := schema
	if len(normalized.Required) > 0 {
		normalized.Required = append([]string(nil), normalized.Required...)
		sort.Strings(normalized.Required)
	}

	normalized.Properties = normalizePropertySchemaMap(normalized.Properties)
	if normalized.Properties == nil {
		normalized.Properties = map[string]tool.PropertySchema{}
	}

	return normalized
}

// normalizePropertySchemaMap returns map copy with recursively normalized property schemas.
func normalizePropertySchemaMap(props map[string]tool.PropertySchema) map[string]tool.PropertySchema {
	if len(props) == 0 {
		return nil
	}

	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	normalized := make(map[string]tool.PropertySchema, len(props))
	for _, key := range keys {
		normalized[key] = normalizePropertySchema(props[key])
	}

	return normalized
}

// normalizePropertySchema returns recursively normalized property schema.
func normalizePropertySchema(schema tool.PropertySchema) tool.PropertySchema {
	normalized := schema

	if len(normalized.Required) > 0 {
		normalized.Required = append([]string(nil), normalized.Required...)
		sort.Strings(normalized.Required)
	}

	if normalized.Items != nil {
		itemsCopy := normalizePropertySchema(*normalized.Items)
		normalized.Items = &itemsCopy
	}

	normalized.Properties = normalizePropertySchemaMap(normalized.Properties)

	return normalized
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
