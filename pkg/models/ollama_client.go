package models

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
	"os"
	"strings"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/tool"
)

// OllamaClient is a client for interacting with Ollama API
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	model      string
	config     *conf.ModelProviderConfig
}

// OllamaChatModel is a chat model implementation for Ollama
type OllamaChatModel struct {
	client  *OllamaClient
	model   string
	options *ChatOptions
}

// OllamaEmbeddingModel is an embedding model implementation for Ollama
type OllamaEmbeddingModel struct {
	client *OllamaClient
	model  string
}

// NewOllamaClient creates a new Ollama client with the given config
func NewOllamaClient(config *conf.ModelProviderConfig) (*OllamaClient, error) {
	if config == nil {
		return nil, fmt.Errorf("NewOllamaClient() [ollama_client.go]: config cannot be nil")
	}

	if config.URL == "" {
		return nil, fmt.Errorf("NewOllamaClient() [ollama_client.go]: URL cannot be empty")
	}

	// Default options
	connectTimeout := 10 * time.Second
	requestTimeout := 60 * time.Second

	if config.ConnectTimeout > 0 {
		connectTimeout = config.ConnectTimeout
	}
	if config.RequestTimeout > 0 {
		requestTimeout = config.RequestTimeout
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
		baseURL:    strings.TrimSuffix(config.URL, "/"),
		httpClient: httpClient,
		config:     config,
	}, nil
}

// NewOllamaClientWithHTTPClient creates a new Ollama client with a custom HTTP client.
// This is useful for testing with mock HTTP servers.
func NewOllamaClientWithHTTPClient(baseURL string, httpClient *http.Client) (*OllamaClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("NewOllamaClientWithHTTPClient() [ollama_client.go]: baseURL cannot be empty")
	}

	if httpClient == nil {
		return nil, fmt.Errorf("NewOllamaClientWithHTTPClient() [ollama_client.go]: httpClient cannot be nil")
	}

	return &OllamaClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
		config:     nil, // No config for test clients
	}, nil
}

// GetConfig returns the provider configuration for this client.
// Returns nil if client was created without config (e.g., in tests).
func (c *OllamaClient) GetConfig() *conf.ModelProviderConfig {
	return c.config
}

// SetModel sets the model to use for chat and embedding operations
func (c *OllamaClient) SetModel(model string) {
	c.model = model
}

// ChatModel returns a ChatModel implementation for the given model and options
func (c *OllamaClient) ChatModel(model string, options *ChatOptions) ChatModel {
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
	return &OllamaChatModel{
		client:  c,
		model:   model,
		options: mergedOptions,
	}
}

// EmbeddingModel returns an EmbeddingModel implementation for the given model
func (c *OllamaClient) EmbeddingModel(model string) EmbeddingModel {
	return &OllamaEmbeddingModel{
		client: c,
		model:  model,
	}
}

// ListModels lists all available models
func (c *OllamaClient) ListModels() ([]ModelInfo, error) {
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

	var response OllamaListModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert from Ollama OllamaModelInfo to models.OllamaModelInfo
	result := make([]ModelInfo, len(response.Models))
	for i, model := range response.Models {
		result[i] = ModelInfo{
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
func (m *OllamaChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	if messages == nil || len(messages) == 0 {
		return nil, fmt.Errorf("OllamaChatModel.Chat() [ollama_client.go]: messages cannot be nil or empty")
	}

	if m.model == "" {
		return nil, fmt.Errorf("OllamaChatModel.Chat() [ollama_client.go]: model not set")
	}

	// Use provided options or fall back to model's default options
	effectiveOptions := options
	if effectiveOptions == nil {
		effectiveOptions = m.options
	}

	// Convert messages to Ollama format
	ollamaMessages := make([]OllamaMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = convertToOllamaMessage(msg)
	}

	// Build request
	chatReq := OllamaChatRequest{
		Model:    m.model,
		Messages: ollamaMessages,
		Stream:   false,
		Tools:    convertToolsToOllama(tools),
	}

	// Apply options if provided
	if effectiveOptions != nil {
		chatReq.Options = &OllamaModelOptions{
			Temperature: float64(effectiveOptions.Temperature),
			TopP:        float64(effectiveOptions.TopP),
			TopK:        effectiveOptions.TopK,
		}
	}

	// Apply default ContextLengthLimit as NumPredict, or override with config if set
	if m.client.config != nil && m.client.config.ContextLengthLimit > 0 {
		if chatReq.Options == nil {
			chatReq.Options = &OllamaModelOptions{}
		}
		chatReq.Options.NumPredict = m.client.config.ContextLengthLimit
	} else {
		if chatReq.Options == nil {
			chatReq.Options = &OllamaModelOptions{}
		}
		chatReq.Options.NumPredict = DefaultContextLengthLimit
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

	// Print verbose request output if enabled
	logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)

	// Log request using structured logger if available
	if effectiveOptions != nil && effectiveOptions.Logger != nil {
		logHTTPRequestWithObfuscation(effectiveOptions.Logger, req, chatReq)
	}

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

	// Check status code and log error response if needed
	if err := m.client.checkStatusCode(resp); err != nil {
		// Log the error response to structured logger
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
		}
		return nil, err
	}

	// Ollama sends multiple JSON objects even with stream=false
	// We need to read all of them and merge the results
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	var mergedMessage OllamaMessage
	mergedMessage.Role = "assistant"
	var lastChatResp OllamaChatResponse

	for {
		var chatResp OllamaChatResponse
		if err := decoder.Decode(&chatResp); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		lastChatResp = chatResp

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

	// Log response using structured logger if available
	// We log the last response which contains the final state
	if effectiveOptions != nil && effectiveOptions.Logger != nil {
		logHTTPResponseWithObfuscation(effectiveOptions.Logger, resp, lastChatResp)
	}

	// Convert response to models.ChatMessage
	result := convertFromOllamaMessage(mergedMessage)
	return result, nil
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses
func (m *OllamaChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		// Validate inputs
		if messages == nil || len(messages) == 0 {
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client.go]: messages cannot be empty\n")
			return
		}

		if m.model == "" {
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client.go]: model cannot be empty\n")
			return
		}

		// Use provided options or fall back to model's default options
		effectiveOptions := options
		if effectiveOptions == nil {
			effectiveOptions = m.options
		}

		// Convert messages to Ollama format
		ollamaMessages := make([]OllamaMessage, len(messages))
		for i, msg := range messages {
			ollamaMessages[i] = convertToOllamaMessage(msg)
		}

		// Build request with streaming enabled
		chatReq := OllamaChatRequest{
			Model:    m.model,
			Messages: ollamaMessages,
			Stream:   true,
			Tools:    convertToolsToOllama(tools),
		}

		// Apply options if provided
		if effectiveOptions != nil {
			chatReq.Options = &OllamaModelOptions{
				Temperature: float64(effectiveOptions.Temperature),
				TopP:        float64(effectiveOptions.TopP),
				TopK:        effectiveOptions.TopK,
			}
		}

		// Apply default ContextLengthLimit as NumPredict, or override with config if set
		if m.client.config != nil && m.client.config.ContextLengthLimit > 0 {
			if chatReq.Options == nil {
				chatReq.Options = &OllamaModelOptions{}
			}
			chatReq.Options.NumPredict = m.client.config.ContextLengthLimit
		} else {
			if chatReq.Options == nil {
				chatReq.Options = &OllamaModelOptions{}
			}
			chatReq.Options.NumPredict = DefaultContextLengthLimit
		}

		url := m.client.baseURL + "/api/chat"

		body, err := json.Marshal(chatReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client.go]: failed to marshal request: %v\n", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client.go]: failed to create request: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		// Print verbose request output if enabled
		logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)

		// Log request using structured logger if available
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPRequestWithObfuscation(effectiveOptions.Logger, req, chatReq)
		}

		resp, err := m.client.httpClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client.go]: HTTP request failed: %v\n", err)
			return
		}
		defer resp.Body.Close()

		// Log response headers before checking status (so errors are also logged)
		logVerboseStreamResponseHeaders(resp, effectiveOptions != nil && effectiveOptions.Verbose)

		if err := m.client.checkStatusCode(resp); err != nil {
			// Read and log error response body
			bodyBytes, _ := io.ReadAll(resp.Body)
			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
			}
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client.go]: API error (status %d): %v\n", resp.StatusCode, err)
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

			var chatResp OllamaChatResponse
			if err := decoder.Decode(&chatResp); err != nil {
				if err == io.EOF {
					if effectiveOptions != nil && effectiveOptions.Verbose {
						fmt.Println("=== End of Streaming Response ===")
						fmt.Println()
					}
					return
				}
				if effectiveOptions != nil && effectiveOptions.Verbose {
					fmt.Println("=== End of Streaming Response ===")
					fmt.Println()
				}
				return
			}

			// Print verbose JSON chunk
			if effectiveOptions != nil && effectiveOptions.Verbose {
				jsonBytes, _ := json.Marshal(chatResp)
				fmt.Println(string(jsonBytes))
			}

			// Log each chunk using structured logger if available
			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPResponseChunk(effectiveOptions.Logger, chatResp)
			}

			// Convert the streamed message to ChatMessage
			result := convertFromOllamaMessage(chatResp.Message)

			// Check if this is the final message
			if chatResp.Done {
				if effectiveOptions != nil && effectiveOptions.Verbose {
					fmt.Println("=== End of Streaming Response ===")
					fmt.Println()
				}
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
		return nil, fmt.Errorf("OllamaEmbeddingModel.Embed() [ollama_client.go]: input cannot be empty")
	}

	if m.model == "" {
		return nil, fmt.Errorf("OllamaEmbeddingModel.Embed() [ollama_client.go]: model not set")
	}

	embedReq := OllamaEmbedRequest{
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

	var embedResp OllamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embedResp.Embeddings) == 0 {
		return nil, fmt.Errorf("OllamaEmbeddingModel.Embed() [ollama_client.go]: no embeddings returned")
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
func (c *OllamaClient) checkStatusCode(resp *http.Response) error {
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
		// Check for context length errors
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

// convertToOllamaMessage converts a models.ChatMessage to Ollama OllamaMessage format
func convertToOllamaMessage(msg *ChatMessage) OllamaMessage {
	ollamaMsg := OllamaMessage{
		Role: string(msg.Role),
	}

	// Check if message contains tool calls or responses
	toolCalls := msg.GetToolCalls()
	toolResponses := msg.GetToolResponses()

	if len(toolCalls) > 0 {
		// OllamaMessage contains tool calls from assistant
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
			ollamaMsg.ToolCalls = append(ollamaMsg.ToolCalls, OllamaToolCall{
				Function: OllamaToolCallFunction{
					Name:      tc.Function,
					Arguments: args,
				},
			})
		}
	} else if len(toolResponses) > 0 {
		// OllamaMessage contains tool responses
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

// generateOllamaToolCallID generates a unique ID for tool calls since Ollama doesn't provide them.
func generateOllamaToolCallID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "call_" + hex.EncodeToString(b)
}

// convertFromOllamaMessage converts Ollama OllamaMessage to models.ChatMessage
func convertFromOllamaMessage(msg OllamaMessage) *ChatMessage {
	var parts []ChatMessagePart

	// Add text content if present
	if msg.Content != "" {
		parts = append(parts, ChatMessagePart{Text: msg.Content})
	}

	// Add tool calls if present
	for _, tc := range msg.ToolCalls {
		parts = append(parts, ChatMessagePart{
			ToolCall: &tool.ToolCall{
				ID:        generateOllamaToolCallID(),
				Function:  tc.Function.Name,
				Arguments: tool.NewToolValue(tc.Function.Arguments),
			},
		})
	}

	return &ChatMessage{
		Role:  ChatRole(msg.Role),
		Parts: parts,
	}
}

// convertToolsToOllama converts tool.ToolInfo to Ollama OllamaTool format
func convertToolsToOllama(tools []tool.ToolInfo) []OllamaTool {
	if len(tools) == 0 {
		return nil
	}

	ollamaTools := make([]OllamaTool, len(tools))
	for i, t := range tools {
		// Convert ToolSchema to map[string]interface{}
		schemaJSON, _ := json.Marshal(t.Schema)
		var schemaMap map[string]interface{}
		json.Unmarshal(schemaJSON, &schemaMap)

		ollamaTools[i] = OllamaTool{
			Type: "function",
			Function: OllamaToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  schemaMap,
			},
		}
	}
	return ollamaTools
}
