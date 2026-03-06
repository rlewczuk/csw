package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"strings"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/tool"
)

const (
	jetbrainsAccessTokenHeader          string = "jb-access-token"
	jetbrainsAuthenticateJWTHeader      string = "grazie-authenticate-jwt"
	jetbrainsFallbackBrowserTokenHeader string = "x-jetbrains-bearer"
)

// NewJetBrainsClient creates a new JetBrains AI client.
//
// JetBrains AI endpoints are not OpenAI-compatible. This client wraps the
// existing OpenAI-compatible client for transport/auth plumbing and adds
// JetBrains-specific request/response handling.
func NewJetBrainsClient(config *conf.ModelProviderConfig) (*JetBrainsClient, error) {
	openaiClient, err := NewOpenAIClient(config)
	if err != nil {
		return nil, fmt.Errorf("NewJetBrainsClient() [jetbrains_client.go]: failed to create base client: %w", err)
	}

	if strings.TrimSpace(openaiClient.baseURL) == "" {
		return nil, fmt.Errorf("NewJetBrainsClient() [jetbrains_client.go]: URL cannot be empty")
	}

	return &JetBrainsClient{openaiClient: openaiClient}, nil
}

// NewJetBrainsClientWithHTTPClient creates a new JetBrains client with custom HTTP client.
// This helper is intended for tests.
func NewJetBrainsClientWithHTTPClient(baseURL string, httpClient *http.Client) (*JetBrainsClient, error) {
	openaiClient, err := NewOpenAIClientWithHTTPClient(baseURL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("NewJetBrainsClientWithHTTPClient() [jetbrains_client.go]: failed to create base client: %w", err)
	}

	return &JetBrainsClient{openaiClient: openaiClient}, nil
}

// JetBrainsClient is a provider client for JetBrains AI private endpoint.
//
// It exposes the common ModelProvider interface while forwarding list-model
// operations to the OpenAI-compatible /models endpoint (as used by local proxy)
// and chat operations to JetBrains streaming endpoint.
type JetBrainsClient struct {
	openaiClient *OpenAIClient
}

// JetBrainsChatModel implements chat operations for JetBrains AI.
type JetBrainsChatModel struct {
	client  *JetBrainsClient
	model   string
	options *ChatOptions
}

// JetBrainsEmbeddingModel is a placeholder embedding model.
// JetBrains endpoint used here does not support embedding operations.
type JetBrainsEmbeddingModel struct {
	client *JetBrainsClient
	model  string
}

// GetConfig returns provider configuration for this client.
func (c *JetBrainsClient) GetConfig() *conf.ModelProviderConfig {
	if c == nil || c.openaiClient == nil {
		return nil
	}
	return c.openaiClient.GetConfig()
}

// SetConfigUpdater sets callback used to persist refreshed auth state.
func (c *JetBrainsClient) SetConfigUpdater(updater ConfigUpdater) {
	if c == nil || c.openaiClient == nil {
		return
	}
	c.openaiClient.SetConfigUpdater(updater)
}

// SetVerbose enables or disables verbose HTTP logging.
func (c *JetBrainsClient) SetVerbose(verbose bool) {
	if c == nil || c.openaiClient == nil {
		return
	}
	c.openaiClient.SetVerbose(verbose)
}

// ListModels lists available models using OpenAI-compatible /models endpoint.
func (c *JetBrainsClient) ListModels() ([]ModelInfo, error) {
	if c == nil || c.openaiClient == nil {
		return nil, fmt.Errorf("JetBrainsClient.ListModels() [jetbrains_client.go]: client is not initialized")
	}
	return c.openaiClient.ListModels()
}

// ChatModel returns chat model implementation for JetBrains provider.
func (c *JetBrainsClient) ChatModel(model string, options *ChatOptions) ChatModel {
	mergedOptions := options
	if cfg := c.GetConfig(); cfg != nil && cfg.Verbose {
		if mergedOptions == nil {
			mergedOptions = &ChatOptions{}
		}
		if !mergedOptions.Verbose {
			mergedOptions.Verbose = cfg.Verbose
		}
	}

	return &JetBrainsChatModel{
		client:  c,
		model:   model,
		options: mergedOptions,
	}
}

// EmbeddingModel returns embedding model implementation.
func (c *JetBrainsClient) EmbeddingModel(model string) EmbeddingModel {
	return &JetBrainsEmbeddingModel{
		client: c,
		model:  model,
	}
}

// Embed is not implemented for JetBrains endpoint.
func (m *JetBrainsEmbeddingModel) Embed(ctx context.Context, input string) ([]float64, error) {
	_ = ctx
	_ = input
	return nil, fmt.Errorf("JetBrainsEmbeddingModel.Embed() [jetbrains_client.go]: not implemented")
}

// Chat sends non-streaming chat request and returns accumulated final answer.
//
// JetBrains endpoint emits SSE stream. This method requests stream=true and
// aggregates text/tool chunks into one response.
func (m *JetBrainsChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("JetBrainsChatModel.Chat() [jetbrains_client.go]: messages cannot be nil or empty")
	}
	if strings.TrimSpace(m.model) == "" {
		return nil, fmt.Errorf("JetBrainsChatModel.Chat() [jetbrains_client.go]: model not set")
	}

	effectiveOptions := options
	if effectiveOptions == nil {
		effectiveOptions = m.options
	}

	chatReq, err := m.buildRequest(messages, tools)
	if err != nil {
		return nil, err
	}

	req, body, err := m.buildHTTPRequest(ctx, chatReq, true, effectiveOptions)
	if err != nil {
		return nil, err
	}

	logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)
	if effectiveOptions != nil && effectiveOptions.Logger != nil {
		logHTTPRequestWithObfuscation(effectiveOptions.Logger, req, chatReq)
	}

	resp, err := m.client.openaiClient.httpClient.Do(req)
	if err != nil {
		return nil, m.client.openaiClient.handleHTTPError(err)
	}
	defer resp.Body.Close()

	bodyBytes, err := logVerboseResponse(resp, effectiveOptions != nil && effectiveOptions.Verbose)
	if err != nil {
		return nil, fmt.Errorf("JetBrainsChatModel.Chat() [jetbrains_client.go]: failed to read response body: %w", err)
	}

	if err := m.client.openaiClient.checkStatusCodeWithBody(resp, bodyBytes); err != nil {
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
		}
		return nil, err
	}

	result, err := convertFromResponsesStreamBody(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("JetBrainsChatModel.Chat() [jetbrains_client.go]: failed to parse stream response: %w", err)
	}

	if effectiveOptions != nil && effectiveOptions.Logger != nil {
		logHTTPResponseWithObfuscation(effectiveOptions.Logger, resp, result)
	}

	return result, nil
}

// ChatStream sends chat request and yields stream fragments from JetBrains SSE.
func (m *JetBrainsChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		if len(messages) == 0 {
			fmt.Fprintf(os.Stderr, "ERROR: JetBrainsChatModel.ChatStream() [jetbrains_client.go]: messages cannot be empty\n")
			return
		}
		if strings.TrimSpace(m.model) == "" {
			fmt.Fprintf(os.Stderr, "ERROR: JetBrainsChatModel.ChatStream() [jetbrains_client.go]: model cannot be empty\n")
			return
		}

		effectiveOptions := options
		if effectiveOptions == nil {
			effectiveOptions = m.options
		}

		chatReq, err := m.buildRequest(messages, tools)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: JetBrainsChatModel.ChatStream() [jetbrains_client.go]: %v\n", err)
			return
		}

		req, body, err := m.buildHTTPRequest(ctx, chatReq, true, effectiveOptions)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: JetBrainsChatModel.ChatStream() [jetbrains_client.go]: %v\n", err)
			return
		}
		req.Header.Set("Accept", "text/event-stream")

		logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)
		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPRequestWithObfuscation(effectiveOptions.Logger, req, chatReq)
		}

		resp, err := m.client.openaiClient.httpClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: JetBrainsChatModel.ChatStream() [jetbrains_client.go]: HTTP request failed: %v\n", err)
			return
		}
		defer resp.Body.Close()

		logVerboseStreamResponseHeaders(resp, effectiveOptions != nil && effectiveOptions.Verbose)

		if err := m.client.openaiClient.checkStatusCode(resp); err != nil {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
			}
			fmt.Fprintf(os.Stderr, "ERROR: JetBrainsChatModel.ChatStream() [jetbrains_client.go]: API error (status %d): %v\n", resp.StatusCode, err)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		toolCallsInProgress := make(map[string]*responsesToolCallInProgress)
		usage := TokenUsage{}
		hasUsage := false
		contextLength := 0

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			if effectiveOptions != nil && effectiveOptions.Verbose {
				fmt.Println(line)
			}
			if line == "" || strings.HasPrefix(line, "event: ") {
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

			switch event.Type {
			case "response.output_text.delta":
				if event.Delta == "" {
					continue
				}
				if !yield(&ChatMessage{Role: ChatRoleAssistant, Parts: []ChatMessagePart{{Text: event.Delta}}}) {
					return
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
				if tc := toolCallsInProgress[event.ItemID]; tc != nil {
					tc.Arguments += event.Delta
				}
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
					if !yield(&ChatMessage{Role: ChatRoleAssistant, Parts: []ChatMessagePart{{ToolCall: toolCall}}}) {
						return
					}
				}
				delete(toolCallsInProgress, event.ItemID)
			case "response.completed":
				if event.Response != nil && event.Response.Usage != nil {
					hasUsage = true
					cachedTokens := 0
					if event.Response.Usage.InputTokensDetails != nil {
						cachedTokens = event.Response.Usage.InputTokensDetails.CachedTokens
					}
					nonCachedTokens := event.Response.Usage.InputTokens - cachedTokens
					if nonCachedTokens < 0 {
						nonCachedTokens = 0
					}

					usage = TokenUsage{
						InputTokens:          event.Response.Usage.InputTokens,
						InputCachedTokens:    cachedTokens,
						InputNonCachedTokens: nonCachedTokens,
						OutputTokens:         event.Response.Usage.OutputTokens,
						TotalTokens:          event.Response.Usage.TotalTokens,
					}
					if usage.TotalTokens <= 0 {
						usage.TotalTokens = usage.InputTokens + usage.OutputTokens
					}
					contextLength = usage.TotalTokens
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return
		}

		if hasUsage {
			usageCopy := usage
			if !yield(&ChatMessage{Role: ChatRoleAssistant, TokenUsage: &usageCopy, ContextLengthTokens: contextLength}) {
				return
			}
		}
	}
}

func (m *JetBrainsChatModel) buildRequest(messages []*ChatMessage, tools []tool.ToolInfo) (*ResponsesCreateRequest, error) {
	items, err := convertToResponsesItems(messages)
	if err != nil {
		return nil, fmt.Errorf("JetBrainsChatModel.buildRequest() [jetbrains_client.go]: failed to convert messages: %w", err)
	}

	store := false
	chatReq := &ResponsesCreateRequest{
		Model:           m.model,
		Input:           items,
		Store:           &store,
		Instructions:    buildResponsesInstructions(messages),
		Tools:           convertToolsToResponses(tools),
		Stream:          true,
		MaxOutputTokens: 0,
	}

	if cfg := m.client.GetConfig(); cfg != nil {
		if cfg.MaxTokens > 0 {
			chatReq.MaxOutputTokens = cfg.MaxTokens
		}
	}

	return chatReq, nil
}

func (m *JetBrainsChatModel) buildHTTPRequest(ctx context.Context, chatReq *ResponsesCreateRequest, stream bool, options *ChatOptions) (*http.Request, []byte, error) {
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, nil, fmt.Errorf("JetBrainsChatModel.buildHTTPRequest() [jetbrains_client.go]: failed to marshal request: %w", err)
	}

	url := strings.TrimSuffix(m.client.openaiClient.baseURL, "/") + "/user/v5/llm/chat/stream/v8"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, nil, fmt.Errorf("JetBrainsChatModel.buildHTTPRequest() [jetbrains_client.go]: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}

	token, err := m.client.openaiClient.GetAccessToken()
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if cfg := m.client.GetConfig(); cfg != nil {
		setJetBrainsAuthHeaders(req, cfg)
	}

	setUserAgentHeader(req)
	m.client.openaiClient.applyConfiguredQueryParams(req)
	m.client.openaiClient.applyConfiguredHeaders(req)
	applyOptionsHeaders(req, options)

	return req, body, nil
}

func setJetBrainsAuthHeaders(req *http.Request, cfg *conf.ModelProviderConfig) {
	if req == nil || cfg == nil {
		return
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	if req.Header.Get(jetbrainsAuthenticateJWTHeader) == "" && apiKey != "" {
		req.Header.Set(jetbrainsAuthenticateJWTHeader, apiKey)
	}

	bearerToken := strings.TrimSpace(cfg.RefreshToken)
	if bearerToken == "" {
		for _, headerName := range []string{jetbrainsAccessTokenHeader, jetbrainsFallbackBrowserTokenHeader} {
			if cfg.Headers == nil {
				continue
			}
			if token := strings.TrimSpace(cfg.Headers[headerName]); token != "" {
				bearerToken = token
				break
			}
		}
	}

	if req.Header.Get(jetbrainsAccessTokenHeader) == "" && bearerToken != "" {
		req.Header.Set(jetbrainsAccessTokenHeader, bearerToken)
	}
}
