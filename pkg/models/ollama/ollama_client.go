package ollama

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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
	"github.com/codesnort/codesnort-swe/pkg/tool"
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
func (m *OllamaChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) (*models.ChatMessage, error) {
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
		ollamaMessages[i] = convertToOllamaMessage(msg)
	}

	// Build request
	chatReq := ChatRequest{
		Model:    m.model,
		Messages: ollamaMessages,
		Stream:   false,
		Tools:    convertToolsToOllama(tools),
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

	// Ollama sends multiple JSON objects even with stream=false
	// We need to read all of them and merge the results
	decoder := json.NewDecoder(resp.Body)
	var mergedMessage Message
	mergedMessage.Role = "assistant"

	for {
		var chatResp ChatResponse
		if err := decoder.Decode(&chatResp); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		// Merge content
		if chatResp.Message.Content != "" {
			mergedMessage.Content += chatResp.Message.Content
		}

		// Merge tool calls
		if len(chatResp.Message.ToolCalls) > 0 {
			mergedMessage.ToolCalls = append(mergedMessage.ToolCalls, chatResp.Message.ToolCalls...)
		}

		if chatResp.Done {
			break
		}
	}

	// Convert response to models.ChatMessage
	result := convertFromOllamaMessage(mergedMessage)
	return result, nil
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses
func (m *OllamaChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) iter.Seq[*models.ChatMessage] {
	return func(yield func(*models.ChatMessage) bool) {
		// Validate inputs
		if messages == nil || len(messages) == 0 {
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

		// Convert messages to Ollama format
		ollamaMessages := make([]Message, len(messages))
		for i, msg := range messages {
			ollamaMessages[i] = convertToOllamaMessage(msg)
		}

		// Build request with streaming enabled
		chatReq := ChatRequest{
			Model:    m.model,
			Messages: ollamaMessages,
			Stream:   true,
			Tools:    convertToolsToOllama(tools),
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
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := m.client.httpClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if err := m.client.checkStatusCode(resp); err != nil {
			return
		}

		// Create decoder and stream responses
		decoder := json.NewDecoder(resp.Body)
		for {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			var chatResp ChatResponse
			if err := decoder.Decode(&chatResp); err != nil {
				if err == io.EOF {
					return
				}
				return
			}

			// Convert the streamed message to ChatMessage
			result := convertFromOllamaMessage(chatResp.Message)

			// Check if this is the final message
			if chatResp.Done {
				// Yield the final fragment if it has content or tool calls
				if len(result.Parts) > 0 && (result.GetText() != "" || len(result.GetToolCalls()) > 0) {
					if !yield(result) {
						return
					}
				}
				return
			}

			// Yield the fragment if it has content or tool calls
			if len(result.Parts) > 0 && (result.GetText() != "" || len(result.GetToolCalls()) > 0) {
				if !yield(result) {
					return
				}
			}
		}
	}
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

// convertToOllamaMessage converts a models.ChatMessage to Ollama Message format
func convertToOllamaMessage(msg *models.ChatMessage) Message {
	ollamaMsg := Message{
		Role: string(msg.Role),
	}

	// Check if message contains tool calls or responses
	toolCalls := msg.GetToolCalls()
	toolResponses := msg.GetToolResponses()

	if len(toolCalls) > 0 {
		// Message contains tool calls from assistant
		ollamaMsg.Content = msg.GetText()
		for _, tc := range toolCalls {
			// Safely convert arguments to map
			var args map[string]interface{}
			if tc.Arguments.Raw() != nil {
				if m, ok := tc.Arguments.Raw().(map[string]interface{}); ok {
					args = m
				}
			}
			if args == nil {
				args = make(map[string]interface{})
			}
			ollamaMsg.ToolCalls = append(ollamaMsg.ToolCalls, ToolCall{
				Function: ToolCallFunction{
					Name:      tc.Function,
					Arguments: args,
				},
			})
		}
	} else if len(toolResponses) > 0 {
		// Message contains tool responses
		// In Ollama, tool responses should use role "tool" and include tool_name
		ollamaMsg.Role = "tool"
		for _, tr := range toolResponses {
			if tr.Error != nil {
				ollamaMsg.Content = "Error: " + tr.Error.Error()
			} else {
				resultJSON, _ := json.Marshal(tr.Result.Raw())
				ollamaMsg.Content = string(resultJSON)
			}
			// Set tool_name from the original tool call if available
			if tr.Call != nil {
				ollamaMsg.ToolName = tr.Call.Function
			}
			break // Handle only first response for now
		}
	} else {
		// Simple text message
		ollamaMsg.Content = msg.GetText()
	}

	return ollamaMsg
}

// generateToolCallID generates a unique ID for tool calls since Ollama doesn't provide them.
func generateToolCallID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "call_" + hex.EncodeToString(b)
}

// convertFromOllamaMessage converts Ollama Message to models.ChatMessage
func convertFromOllamaMessage(msg Message) *models.ChatMessage {
	var parts []models.ChatMessagePart

	// Add text content if present
	if msg.Content != "" {
		parts = append(parts, models.ChatMessagePart{Text: msg.Content})
	}

	// Add tool calls if present
	for _, tc := range msg.ToolCalls {
		parts = append(parts, models.ChatMessagePart{
			ToolCall: &tool.ToolCall{
				ID:        generateToolCallID(),
				Function:  tc.Function.Name,
				Arguments: tool.NewToolValue(tc.Function.Arguments),
			},
		})
	}

	return &models.ChatMessage{
		Role:  models.ChatRole(msg.Role),
		Parts: parts,
	}
}

// convertToolsToOllama converts tool.ToolInfo to Ollama Tool format
func convertToolsToOllama(tools []tool.ToolInfo) []Tool {
	if len(tools) == 0 {
		return nil
	}

	ollamaTools := make([]Tool, len(tools))
	for i, t := range tools {
		// Convert ToolSchema to map[string]interface{}
		schemaJSON, _ := json.Marshal(t.Schema)
		var schemaMap map[string]interface{}
		json.Unmarshal(schemaJSON, &schemaMap)

		ollamaTools[i] = Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  schemaMap,
			},
		}
	}
	return ollamaTools
}
