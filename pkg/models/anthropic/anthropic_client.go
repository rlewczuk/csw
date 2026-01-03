package anthropic

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
	"github.com/codesnort/codesnort-swe/pkg/tool"
)

// AnthropicClient is a client for interacting with Anthropic API
type AnthropicClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	apiVersion string
}

// AnthropicChatModel is a chat model implementation for Anthropic
type AnthropicChatModel struct {
	client  *AnthropicClient
	model   string
	options *models.ChatOptions
}

// AnthropicEmbeddingModel is a placeholder embedding model (not supported by Anthropic)
type AnthropicEmbeddingModel struct {
	client *AnthropicClient
	model  string
}

// NewAnthropicClient creates a new Anthropic client with the given base URL and options
func NewAnthropicClient(baseURL string, options *models.ModelConnectionOptions) (*AnthropicClient, error) {
	if baseURL == "" {
		return nil, errors.New("baseURL cannot be empty")
	}

	// Default options
	connectTimeout := 10 * time.Second
	requestTimeout := 60 * time.Second
	apiKey := ""
	apiVersion := "2023-06-01" // Default Anthropic API version

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

	return &AnthropicClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
		apiKey:     apiKey,
		apiVersion: apiVersion,
	}, nil
}

// ChatModel returns a ChatModel implementation for the given model and options
func (c *AnthropicClient) ChatModel(model string, options *models.ChatOptions) models.ChatModel {
	return &AnthropicChatModel{
		client:  c,
		model:   model,
		options: options,
	}
}

// EmbeddingModel returns an EmbeddingModel implementation for the given model
// Note: Anthropic doesn't support embeddings, so this will return an error when used
func (c *AnthropicClient) EmbeddingModel(model string) models.EmbeddingModel {
	return &AnthropicEmbeddingModel{
		client: c,
		model:  model,
	}
}

// ListModels lists all available models
func (c *AnthropicClient) ListModels() ([]models.ModelInfo, error) {
	url := c.baseURL + "/v1/models"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", c.apiVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, c.handleHTTPError(err)
	}
	defer resp.Body.Close()

	if err := c.checkStatusCode(resp); err != nil {
		return nil, err
	}

	var response ModelsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert from Anthropic ModelInfo to models.ModelInfo
	result := make([]models.ModelInfo, len(response.Data))
	for i, model := range response.Data {
		result[i] = models.ModelInfo{
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
func (m *AnthropicChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) (*models.ChatMessage, error) {
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

	// Convert messages to Anthropic format
	// Anthropic API requires system messages to be separate
	var systemPrompt string
	var anthropicMessages []MessageParam

	for _, msg := range messages {
		if msg.Role == models.ChatRoleSystem {
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
	chatReq := MessagesRequest{
		Model:     m.model,
		Messages:  anthropicMessages,
		MaxTokens: 4096, // Default max tokens
		Stream:    false,
		Tools:     convertToolsToAnthropic(tools),
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
	}

	url := m.client.baseURL + "/v1/messages"

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", m.client.apiKey)
	req.Header.Set("anthropic-version", m.client.apiVersion)

	resp, err := m.client.httpClient.Do(req)
	if err != nil {
		return nil, m.client.handleHTTPError(err)
	}
	defer resp.Body.Close()

	if err := m.client.checkStatusCode(resp); err != nil {
		return nil, err
	}

	var chatResp MessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Content) == 0 {
		return nil, errors.New("no content in response")
	}

	// Convert response to ChatMessage
	result := convertFromAnthropicResponse(chatResp.Content)
	return result, nil
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses
func (m *AnthropicChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) iter.Seq[*models.ChatMessage] {
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

		// Convert messages to Anthropic format
		var systemPrompt string
		var anthropicMessages []MessageParam

		for _, msg := range messages {
			if msg.Role == models.ChatRoleSystem {
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
		chatReq := MessagesRequest{
			Model:     m.model,
			Messages:  anthropicMessages,
			MaxTokens: 4096,
			Stream:    true,
			Tools:     convertToolsToAnthropic(tools),
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
		}

		url := m.client.baseURL + "/v1/messages"

		body, err := json.Marshal(chatReq)
		if err != nil {
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", m.client.apiKey)
		req.Header.Set("anthropic-version", m.client.apiVersion)

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

				var event StreamEvent
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					// Skip invalid JSON
					continue
				}

				// Handle different event types
				switch event.Type {
				case "content_block_start":
					// Handle tool_use blocks
					if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
						result := &models.ChatMessage{
							Role: models.ChatRoleAssistant,
							Parts: []models.ChatMessagePart{
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
						result := &models.ChatMessage{
							Role: models.ChatRoleAssistant,
							Parts: []models.ChatMessagePart{
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
					// Check for stop reason
					if event.Delta != nil && event.Delta.StopReason != "" {
						return
					}
				case "message_stop":
					return
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
// Note: Anthropic doesn't support embeddings, so this always returns an error
func (m *AnthropicEmbeddingModel) Embed(ctx context.Context, input string) ([]float64, error) {
	return nil, errors.New("not implemented")
}

// handleHTTPError converts HTTP errors to appropriate model errors
func (c *AnthropicClient) handleHTTPError(err error) error {
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
func (c *AnthropicClient) checkStatusCode(resp *http.Response) error {
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

// convertToAnthropicMessage converts a models.ChatMessage to Anthropic MessageParam format
func convertToAnthropicMessage(msg *models.ChatMessage) MessageParam {
	// Check if message contains only text
	if len(msg.Parts) > 0 && msg.Parts[0].Text != "" && msg.Parts[0].ToolCall == nil && msg.Parts[0].ToolResponse == nil {
		// Simple text message
		return MessageParam{
			Role:    string(msg.Role),
			Content: msg.GetText(),
		}
	}

	// Message contains tool calls or tool responses - use array of content blocks
	var contentBlocks []ContentBlock
	for _, part := range msg.Parts {
		if part.Text != "" {
			contentBlocks = append(contentBlocks, ContentBlock{
				Type: "text",
				Text: part.Text,
			})
		} else if part.ToolCall != nil {
			// Tool use block
			contentBlocks = append(contentBlocks, ContentBlock{
				Type:  "tool_use",
				ID:    part.ToolCall.ID,
				Name:  part.ToolCall.Function,
				Input: part.ToolCall.Arguments.Raw().(map[string]interface{}),
			})
		} else if part.ToolResponse != nil {
			// Tool result block
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
			contentBlocks = append(contentBlocks, ContentBlock{
				Type:      "tool_result",
				ToolUseID: toolUseID,
				Content:   content,
				IsError:   part.ToolResponse.Error != nil,
			})
		}
	}

	return MessageParam{
		Role:    string(msg.Role),
		Content: contentBlocks,
	}
}

// convertFromAnthropicResponse converts Anthropic response content to models.ChatMessage
func convertFromAnthropicResponse(content []ResponseContent) *models.ChatMessage {
	var parts []models.ChatMessagePart
	for _, c := range content {
		if c.Type == "text" {
			parts = append(parts, models.ChatMessagePart{Text: c.Text})
		} else if c.Type == "tool_use" {
			parts = append(parts, models.ChatMessagePart{
				ToolCall: &tool.ToolCall{
					ID:        c.ID,
					Function:  c.Name,
					Arguments: tool.NewToolValue(c.Input),
				},
			})
		}
	}

	return &models.ChatMessage{
		Role:  models.ChatRoleAssistant,
		Parts: parts,
	}
}

// convertToolsToAnthropic converts tool.ToolInfo to Anthropic Tool format
func convertToolsToAnthropic(tools []tool.ToolInfo) []Tool {
	if len(tools) == 0 {
		return nil
	}

	anthropicTools := make([]Tool, len(tools))
	for i, t := range tools {
		// Convert ToolSchema to map[string]interface{}
		schemaJSON, _ := json.Marshal(t.Schema)
		var schemaMap map[string]interface{}
		json.Unmarshal(schemaJSON, &schemaMap)

		anthropicTools[i] = Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schemaMap,
		}
	}
	return anthropicTools
}
