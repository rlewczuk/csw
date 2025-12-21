package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/models"
)

// OllamaClient is a client for interacting with Ollama API
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	model      string
}

// OllamaChatModel is a chat model implementation for Ollama
type OllamaChatModel struct {
	client  *OllamaClient
	model   string
	options *models.ChatOptions
}

// OllamaEmbeddingModel is an embedding model implementation for Ollama
type OllamaEmbeddingModel struct {
	client *OllamaClient
	model  string
}

// NewOllamaClient creates a new Ollama client with the given base URL and options
func NewOllamaClient(baseURL string, options *models.ModelConnectionOptions) (*OllamaClient, error) {
	if baseURL == "" {
		return nil, errors.New("baseURL cannot be empty")
	}

	// Default options
	connectTimeout := 10 * time.Second
	requestTimeout := 60 * time.Second

	if options != nil {
		if options.ConnectTimeout > 0 {
			connectTimeout = options.ConnectTimeout
		}
		if options.RequestTimeout > 0 {
			requestTimeout = options.RequestTimeout
		}
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

	return &OllamaClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
	}, nil
}

// SetModel sets the model to use for chat and embedding operations
func (c *OllamaClient) SetModel(model string) {
	c.model = model
}

// ChatModel returns a ChatModel implementation for the given model and options
func (c *OllamaClient) ChatModel(model string, options *models.ChatOptions) models.ChatModel {
	return &OllamaChatModel{
		client:  c,
		model:   model,
		options: options,
	}
}

// EmbeddingModel returns an EmbeddingModel implementation for the given model
func (c *OllamaClient) EmbeddingModel(model string) models.EmbeddingModel {
	return &OllamaEmbeddingModel{
		client: c,
		model:  model,
	}
}

// ListModels lists all available models
func (c *OllamaClient) ListModels() ([]models.ModelInfo, error) {
	url := c.baseURL + "/api/tags"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, c.handleHTTPError(err)
	}
	defer resp.Body.Close()

	if err := c.checkStatusCode(resp); err != nil {
		return nil, err
	}

	var response ListModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert from Ollama ModelInfo to models.ModelInfo
	result := make([]models.ModelInfo, len(response.Models))
	for i, model := range response.Models {
		result[i] = models.ModelInfo{
			Name:       model.Name,
			Model:      model.Model,
			ModifiedAt: model.ModifiedAt,
			Size:       model.Size,
			Family:     model.Details.Family,
		}
	}

	return result, nil
}

// Chat sends a chat request and returns the response
func (m *OllamaChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions) (*models.ChatMessage, error) {
	if messages == nil || len(messages) == 0 {
		return nil, errors.New("messages cannot be nil or empty")
	}

	if m.model == "" {
		return nil, errors.New("model not set")
	}

	// Use provided options or fall back to model's default options
	effectiveOptions := options
	if effectiveOptions == nil {
		effectiveOptions = m.options
	}

	// Convert messages to Ollama format
	ollamaMessages := make([]Message, len(messages))
	for i, msg := range messages {
		// Concatenate parts into a single content string
		content := strings.Join(msg.Parts, "")
		ollamaMessages[i] = Message{
			Role:    string(msg.Role),
			Content: content,
		}
	}

	// Build request
	chatReq := ChatRequest{
		Model:    m.model,
		Messages: ollamaMessages,
		Stream:   false,
	}

	// Apply options if provided
	if effectiveOptions != nil {
		chatReq.Options = &ModelOptions{
			Temperature: float64(effectiveOptions.Temperature),
			TopP:        float64(effectiveOptions.TopP),
			TopK:        effectiveOptions.TopK,
		}
	}

	url := m.client.baseURL + "/api/chat"

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.httpClient.Do(req)
	if err != nil {
		return nil, m.client.handleHTTPError(err)
	}
	defer resp.Body.Close()

	if err := m.client.checkStatusCode(resp); err != nil {
		return nil, err
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert response to models.ChatMessage
	result := &models.ChatMessage{
		Role:  models.ChatRole(chatResp.Message.Role),
		Parts: []string{chatResp.Message.Content},
	}

	return result, nil
}

// ollamaStreamIterator implements ChatStreamIterator for Ollama streaming responses
type ollamaStreamIterator struct {
	ctx       context.Context
	resp      *http.Response
	decoder   *json.Decoder
	client    *OllamaClient
	done      bool
	lastError error
}

// Next returns the next fragment of the chat response
func (it *ollamaStreamIterator) Next() (*models.ChatMessage, error) {
	if it.done {
		return nil, models.ErrEndOfStream
	}

	if it.lastError != nil {
		return nil, it.lastError
	}

	// Check if context is cancelled
	select {
	case <-it.ctx.Done():
		it.done = true
		it.lastError = it.ctx.Err()
		return nil, it.lastError
	default:
	}

	var chatResp ChatResponse
	if err := it.decoder.Decode(&chatResp); err != nil {
		it.done = true
		if err == io.EOF {
			it.lastError = models.ErrEndOfStream
			return nil, models.ErrEndOfStream
		}
		it.lastError = fmt.Errorf("failed to decode streaming response: %w", err)
		return nil, it.lastError
	}

	// Check if this is the final message
	if chatResp.Done {
		it.done = true
		// Return the final fragment if it has content
		if chatResp.Message.Content != "" {
			result := &models.ChatMessage{
				Role:  models.ChatRole(chatResp.Message.Role),
				Parts: []string{chatResp.Message.Content},
			}
			return result, nil
		}
		return nil, models.ErrEndOfStream
	}

	// Return the fragment
	result := &models.ChatMessage{
		Role:  models.ChatRole(chatResp.Message.Role),
		Parts: []string{chatResp.Message.Content},
	}

	return result, nil
}

// Close releases resources associated with the iterator
func (it *ollamaStreamIterator) Close() error {
	if it.resp != nil && it.resp.Body != nil {
		return it.resp.Body.Close()
	}
	return nil
}

// ChatStream sends a chat request and returns an iterator for streaming responses
func (m *OllamaChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions) (models.ChatStreamIterator, error) {
	if messages == nil || len(messages) == 0 {
		return nil, errors.New("messages cannot be nil or empty")
	}

	if m.model == "" {
		return nil, errors.New("model not set")
	}

	// Use provided options or fall back to model's default options
	effectiveOptions := options
	if effectiveOptions == nil {
		effectiveOptions = m.options
	}

	// Convert messages to Ollama format
	ollamaMessages := make([]Message, len(messages))
	for i, msg := range messages {
		// Concatenate parts into a single content string
		content := strings.Join(msg.Parts, "")
		ollamaMessages[i] = Message{
			Role:    string(msg.Role),
			Content: content,
		}
	}

	// Build request with streaming enabled
	chatReq := ChatRequest{
		Model:    m.model,
		Messages: ollamaMessages,
		Stream:   true,
	}

	// Apply options if provided
	if effectiveOptions != nil {
		chatReq.Options = &ModelOptions{
			Temperature: float64(effectiveOptions.Temperature),
			TopP:        float64(effectiveOptions.TopP),
			TopK:        effectiveOptions.TopK,
		}
	}

	url := m.client.baseURL + "/api/chat"

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.httpClient.Do(req)
	if err != nil {
		return nil, m.client.handleHTTPError(err)
	}

	if err := m.client.checkStatusCode(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	// Create iterator
	iterator := &ollamaStreamIterator{
		ctx:     ctx,
		resp:    resp,
		decoder: json.NewDecoder(resp.Body),
		client:  m.client,
		done:    false,
	}

	return iterator, nil
}

// Embed generates embeddings for the given input text
func (m *OllamaEmbeddingModel) Embed(ctx context.Context, input string) ([]float64, error) {
	if input == "" {
		return nil, errors.New("input cannot be empty")
	}

	if m.model == "" {
		return nil, errors.New("model not set")
	}

	embedReq := EmbedRequest{
		Model: m.model,
		Input: []string{input},
	}

	url := m.client.baseURL + "/api/embed"

	body, err := json.Marshal(embedReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.httpClient.Do(req)
	if err != nil {
		return nil, m.client.handleHTTPError(err)
	}
	defer resp.Body.Close()

	if err := m.client.checkStatusCode(resp); err != nil {
		return nil, err
	}

	var embedResp EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embedResp.Embeddings) == 0 {
		return nil, errors.New("no embeddings returned")
	}

	return embedResp.Embeddings[0], nil
}

// handleHTTPError converts HTTP errors to appropriate model errors
func (c *OllamaClient) handleHTTPError(err error) error {
	if err == nil {
		return nil
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return fmt.Errorf("%w: %v", models.ErrEndpointUnavailable, err)
		}
		return fmt.Errorf("%w: %v", models.ErrEndpointUnavailable, err)
	}

	// Check for DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Errorf("%w: %v", models.ErrEndpointNotFound, err)
	}

	return fmt.Errorf("%w: %v", models.ErrEndpointUnavailable, err)
}

// checkStatusCode checks the HTTP status code and returns appropriate errors
func (c *OllamaClient) checkStatusCode(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %s", models.ErrEndpointNotFound, string(body))
	case http.StatusUnauthorized, http.StatusForbidden:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %s", models.ErrPermissionDenied, string(body))
	case http.StatusTooManyRequests:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %s", models.ErrRateExceeded, string(body))
	case http.StatusBadRequest:
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		// Check for context length errors
		if strings.Contains(strings.ToLower(bodyStr), "context length") ||
			strings.Contains(strings.ToLower(bodyStr), "too many tokens") {
			return fmt.Errorf("%w: %s", models.ErrTooManyInputTokens, bodyStr)
		}
		return fmt.Errorf("bad request: %s", bodyStr)
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %s", models.ErrEndpointUnavailable, string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}
}
