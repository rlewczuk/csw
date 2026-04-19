package models

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
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

// ResponsesEmbeddingModel is a placeholder embedding model (not supported by Responses API).
type ResponsesEmbeddingModel struct {
	client *ResponsesClient
	model  string
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
func (c *ResponsesClient) EmbeddingModel(model string) *ResponsesEmbeddingModel {
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
