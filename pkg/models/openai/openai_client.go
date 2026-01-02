package openai

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
	"strings"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/models"
)

// OpenAIClient is a client for interacting with OpenAI-compatible API
type OpenAIClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// OpenAIChatModel is a chat model implementation for OpenAI
type OpenAIChatModel struct {
	client  *OpenAIClient
	model   string
	options *models.ChatOptions
}

// OpenAIEmbeddingModel is an embedding model implementation for OpenAI
type OpenAIEmbeddingModel struct {
	client *OpenAIClient
	model  string
}

// NewOpenAIClient creates a new OpenAI-compatible client with the given base URL and options
func NewOpenAIClient(baseURL string, options *models.ModelConnectionOptions) (*OpenAIClient, error) {
	if baseURL == "" {
		return nil, errors.New("baseURL cannot be empty")
	}

	// Default options
	connectTimeout := 10 * time.Second
	requestTimeout := 60 * time.Second
	apiKey := "ollama" // Default API key for Ollama

	if options != nil {
		if options.ConnectTimeout > 0 {
			connectTimeout = options.ConnectTimeout
		}
		if options.RequestTimeout > 0 {
			requestTimeout = options.RequestTimeout
		}
		if options.APIKey != "" {
			apiKey = options.APIKey
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

	return &OpenAIClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
		apiKey:     apiKey,
	}, nil
}

// ChatModel returns a ChatModel implementation for the given model and options
func (c *OpenAIClient) ChatModel(model string, options *models.ChatOptions) models.ChatModel {
	return &OpenAIChatModel{
		client:  c,
		model:   model,
		options: options,
	}
}

// EmbeddingModel returns an EmbeddingModel implementation for the given model
func (c *OpenAIClient) EmbeddingModel(model string) models.EmbeddingModel {
	return &OpenAIEmbeddingModel{
		client: c,
		model:  model,
	}
}

// ListModels lists all available models
func (c *OpenAIClient) ListModels() ([]models.ModelInfo, error) {
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

	var response ModelList
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert from OpenAI ModelData to models.ModelInfo
	result := make([]models.ModelInfo, len(response.Data))
	for i, model := range response.Data {
		result[i] = models.ModelInfo{
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
func (m *OpenAIChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions) (*models.ChatMessage, error) {
	if len(messages) == 0 {
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

	// Convert messages to OpenAI format
	openaiMessages := make([]ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		// Concatenate parts into a single content string
		content := strings.Join(msg.Parts, "")
		openaiMessages[i] = ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: content,
		}
	}

	// Build request
	chatReq := ChatCompletionRequest{
		Model:    m.model,
		Messages: openaiMessages,
		Stream:   false,
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

	resp, err := m.client.httpClient.Do(req)
	if err != nil {
		return nil, m.client.handleHTTPError(err)
	}
	defer resp.Body.Close()

	if err := m.client.checkStatusCode(resp); err != nil {
		return nil, err
	}

	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	// Get the first choice
	choice := chatResp.Choices[0]
	if choice.Message == nil {
		return nil, errors.New("no message in choice")
	}

	// Convert response to models.ChatMessage
	var content string
	switch v := choice.Message.Content.(type) {
	case string:
		content = v
	case []interface{}:
		// Handle array of content parts
		for _, part := range v {
			if partMap, ok := part.(map[string]interface{}); ok {
				if text, ok := partMap["text"].(string); ok {
					content += text
				}
			}
		}
	default:
		content = fmt.Sprintf("%v", v)
	}

	result := &models.ChatMessage{
		Role:  models.ChatRole(choice.Message.Role),
		Parts: []string{content},
	}

	return result, nil
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses
func (m *OpenAIChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions) iter.Seq[*models.ChatMessage] {
	return func(yield func(*models.ChatMessage) bool) {
		// Validate inputs
		if len(messages) == 0 {
			return
		}

		if m.model == "" {
			return
		}

		// Use provided options or fall back to model's default options
		effectiveOptions := options
		if effectiveOptions == nil {
			effectiveOptions = m.options
		}

		// Convert messages to OpenAI format
		openaiMessages := make([]ChatCompletionMessage, len(messages))
		for i, msg := range messages {
			// Concatenate parts into a single content string
			content := strings.Join(msg.Parts, "")
			openaiMessages[i] = ChatCompletionMessage{
				Role:    string(msg.Role),
				Content: content,
			}
		}

		// Build request with streaming enabled
		chatReq := ChatCompletionRequest{
			Model:    m.model,
			Messages: openaiMessages,
			Stream:   true,
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
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+m.client.apiKey)
		req.Header.Set("Accept", "text/event-stream")

		resp, err := m.client.httpClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if err := m.client.checkStatusCode(resp); err != nil {
			return
		}

		// Create scanner for SSE and stream responses
		scanner := bufio.NewScanner(resp.Body)
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

			// Check for [DONE] marker
			if strings.TrimSpace(line) == "data: [DONE]" {
				return
			}

			// Process SSE data line
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				var chatResp ChatCompletionResponse
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
					// If there's content in the delta, yield it before ending
					if choice.Delta != nil {
						var content string
						switch v := choice.Delta.Content.(type) {
						case string:
							content = v
						default:
							content = fmt.Sprintf("%v", v)
						}

						if content != "" {
							result := &models.ChatMessage{
								Role:  models.ChatRoleAssistant,
								Parts: []string{content},
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
					var content string
					switch v := choice.Delta.Content.(type) {
					case string:
						content = v
					default:
						content = fmt.Sprintf("%v", v)
					}

					if content != "" {
						result := &models.ChatMessage{
							Role:  models.ChatRoleAssistant,
							Parts: []string{content},
						}
						if !yield(result) {
							return
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
		return nil, errors.New("input cannot be empty")
	}

	if m.model == "" {
		return nil, errors.New("model not set")
	}

	embedReq := EmbeddingRequest{
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

	var embedResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embedResp.Data) == 0 {
		return nil, errors.New("no embeddings returned")
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
func (c *OpenAIClient) checkStatusCode(resp *http.Response) error {
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

		// Try to parse error response
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
			// Check for context length errors
			if strings.Contains(strings.ToLower(errResp.Error.Message), "context length") ||
				strings.Contains(strings.ToLower(errResp.Error.Message), "too many tokens") ||
				strings.Contains(strings.ToLower(errResp.Error.Message), "maximum context length") {
				return fmt.Errorf("%w: %s", models.ErrTooManyInputTokens, errResp.Error.Message)
			}
			return fmt.Errorf("bad request: %s", errResp.Error.Message)
		}

		// Fallback to raw body
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
