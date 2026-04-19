package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ResponsesChatCompactor compacts message history using Responses API compact endpoint.
type ResponsesChatCompactor struct {
	chatModel *ResponsesChatModel
}

// Compactor returns Responses API-based compactor implementation.
func (m *ResponsesChatModel) Compactor() ChatCompator {
	if m == nil {
		return nil
	}

	return &ResponsesChatCompactor{chatModel: m}
}

// CompactMessages compacts chat history using POST /v1/responses/compact endpoint.
func (c *ResponsesChatCompactor) CompactMessages(messages []*ChatMessage) ([]*ChatMessage, error) {
	if c == nil || c.chatModel == nil || c.chatModel.client == nil {
		return nil, fmt.Errorf("ResponsesChatCompactor.CompactMessages() [responses_client_compactor.go]: chat model not initialized")
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("ResponsesChatCompactor.CompactMessages() [responses_client_compactor.go]: messages cannot be nil or empty")
	}
	if c.chatModel.model == "" {
		return nil, fmt.Errorf("ResponsesChatCompactor.CompactMessages() [responses_client_compactor.go]: model not set")
	}

	items, err := convertToResponsesItems(messages)
	if err != nil {
		return nil, err
	}

	compactReq := ResponsesCreateRequest{
		Model: c.chatModel.model,
		Input: items,
	}

	body, err := json.Marshal(compactReq)
	if err != nil {
		return nil, fmt.Errorf("ResponsesChatCompactor.CompactMessages() [responses_client_compactor.go]: failed to marshal request: %w", err)
	}

	url := buildResponsesCompactURL(c.chatModel.client.baseURL)
	executeRequest := func(token string) (*http.Response, []byte, error) {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return nil, nil, fmt.Errorf("ResponsesChatCompactor.CompactMessages() [responses_client_compactor.go]: failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		setUserAgentHeader(req)
		c.chatModel.client.applyConfiguredHeaders(req)
		applyOptionsHeaders(req, c.chatModel.options)

		resp, err := c.chatModel.client.httpClient.Do(req)
		if err != nil {
			return nil, nil, wrapLLMRequestError(c.chatModel.client.handleHTTPError(err), nil, nil)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			readErr := fmt.Errorf("ResponsesChatCompactor.CompactMessages() [responses_client_compactor.go]: failed to read response body: %w", c.chatModel.client.handleHTTPError(err))
			return nil, nil, wrapLLMRequestError(readErr, resp, nil)
		}

		return resp, bodyBytes, nil
	}

	token, err := c.chatModel.client.GetAccessToken()
	if err != nil {
		return nil, err
	}

	resp, bodyBytes, err := executeRequest(token)
	if err != nil {
		return nil, err
	}

	if c.chatModel.client.shouldRefreshAfterUnauthorized(resp, bodyBytes) {
		if err := c.chatModel.client.refreshTokenIfNeeded(true, token); err != nil {
			return nil, err
		}
		token, err = c.chatModel.client.GetAccessToken()
		if err != nil {
			return nil, err
		}
		resp, bodyBytes, err = executeRequest(token)
		if err != nil {
			return nil, err
		}
	}

	if err := c.chatModel.client.checkStatusCodeWithBody(resp, bodyBytes); err != nil {
		return nil, wrapLLMRequestError(err, resp, bodyBytes)
	}

	var compactResp ResponsesCompactedResponse
	if err := json.Unmarshal(bodyBytes, &compactResp); err != nil {
		decodeErr := fmt.Errorf("ResponsesChatCompactor.CompactMessages() [responses_client_compactor.go]: failed to decode response: %w", err)
		return nil, wrapLLMRequestError(decodeErr, resp, bodyBytes)
	}

	compactedMessages, err := convertFromResponsesCompactedOutput(compactResp.Output)
	if err != nil {
		return nil, wrapLLMRequestError(err, resp, bodyBytes)
	}

	return compactedMessages, nil
}

// buildResponsesCompactURL returns compact endpoint URL with ensured /v1 prefix.
func buildResponsesCompactURL(baseURL string) string {
	normalizedBaseURL := strings.TrimSpace(baseURL)
	if strings.HasSuffix(strings.ToLower(normalizedBaseURL), "/v1") {
		return normalizedBaseURL + "/responses/compact"
	}

	return normalizedBaseURL + "/v1/responses/compact"
}
