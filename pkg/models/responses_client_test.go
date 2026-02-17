package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultResponsesTestURL = "https://api.openai.com/v1"
	testResponsesModelName  = "gpt-4o-mini"
	testResponsesTimeout    = 30 * time.Second
	connectResponsesTimeout = 5 * time.Second
)

// responsesTestClient holds either a real or mock client and provides cleanup.
type responsesTestClient struct {
	Client *ResponsesClient
	Mock   *testutil.MockHTTPServer
}

// Close cleans up the test client resources.
func (tc *responsesTestClient) Close() {
	if tc.Mock != nil {
		tc.Mock.Close()
	}
}

func getResponsesModelName() string {
	if model := testutil.IntegCfgReadFile("responses.model"); model != "" {
		return model
	}
	return testResponsesModelName
}

// getResponsesTestClient returns a client for testing - either real or mock based on integration mode.
// For mock mode, it also returns the mock server for adding responses.
func getResponsesTestClient(t *testing.T) *responsesTestClient {
	t.Helper()

	if testutil.IntegTestEnabled("responses") {
		url := testutil.IntegCfgReadFile("responses.url")
		if url == "" {
			url = defaultResponsesTestURL
		}
		apiKey := testutil.IntegCfgReadFile("responses.key")
		if apiKey == "" {
			t.Skip("Skipping test: _integ/responses.key not configured")
		}

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:            url,
			APIKey:         apiKey,
			ConnectTimeout: connectResponsesTimeout,
			RequestTimeout: testResponsesTimeout,
		})
		require.NoError(t, err)

		return &responsesTestClient{Client: client}
	}

	mock := testutil.NewMockHTTPServer()
	client, err := NewResponsesClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	return &responsesTestClient{Client: client, Mock: mock}
}

func TestNewResponsesClient(t *testing.T) {
	t.Run("creates client with valid configuration", func(t *testing.T) {
		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:            defaultResponsesTestURL,
			APIKey:         "test-api-key",
			ConnectTimeout: connectResponsesTimeout,
			RequestTimeout: testResponsesTimeout,
		})

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := NewResponsesClient(nil)

		assert.Error(t, err)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		_, err := NewResponsesClient(&conf.ModelProviderConfig{URL: ""})

		assert.Error(t, err)
	})
}

func TestNewResponsesClientWithHTTPClient(t *testing.T) {
	t.Run("creates client with custom HTTP client", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClientWithHTTPClient(mock.URL(), mock.Client())

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		_, err := NewResponsesClientWithHTTPClient("", mock.Client())

		assert.Error(t, err)
	})

	t.Run("returns error for nil HTTP client", func(t *testing.T) {
		_, err := NewResponsesClientWithHTTPClient(defaultResponsesTestURL, nil)

		assert.Error(t, err)
	})
}

func TestResponsesClient_ListModels(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	if tc.Mock != nil {
		modelsResponse := `{"data":[{"id":"gpt-4o-mini","object":"model","created":1640000000,"owned_by":"openai"},{"id":"gpt-4o","object":"model","created":1640000000,"owned_by":"openai"}]}`
		tc.Mock.AddRestResponse("/models", "GET", modelsResponse)
		tc.Mock.AddRestResponse("/models", "GET", modelsResponse)
	}

	t.Run("lists available models", func(t *testing.T) {
		modelList, err := tc.Client.ListModels()

		require.NoError(t, err)
		assert.NotNil(t, modelList)
		assert.NotEmpty(t, modelList)
	})

	t.Run("finds test model in list", func(t *testing.T) {
		modelList, err := tc.Client.ListModels()

		require.NoError(t, err)

		found := false
		for _, model := range modelList {
			if model.Name == getResponsesModelName() {
				found = true
				break
			}
		}

		assert.True(t, found, "expected test model %s to be available", getResponsesModelName())
	})
}

func TestResponsesClient_ChatModel(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("creates chat model with model name and options", func(t *testing.T) {
		options := &ChatOptions{Temperature: 0.7, TopP: 0.9, TopK: 40}

		chatModel := tc.Client.ChatModel(getResponsesModelName(), options)

		assert.NotNil(t, chatModel)
	})

	t.Run("sends chat message and gets response", func(t *testing.T) {
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/responses", "POST", `{"id":"resp_1","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"4"}]}]}`)
		}

		chatModel := tc.Client.ChatModel(getResponsesModelName(), &ChatOptions{Temperature: 0.7})

		messages := []*ChatMessage{
			{Role: ChatRoleUser, Parts: []ChatMessagePart{{Text: "What is 2+2?"}}},
		}

		response, err := chatModel.Chat(ctx, messages, nil, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, ChatRoleAssistant, response.Role)
		assert.NotEmpty(t, response.GetText())
	})

	t.Run("returns error for empty messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(getResponsesModelName(), nil)

		response, err := chatModel.Chat(ctx, []*ChatMessage{}, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})
}

func TestResponsesClient_ChatModelStream(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("streams chat message and gets fragments", func(t *testing.T) {
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/responses", "POST", true,
				`data: {"type":"response.output_text.delta","delta":"1","item_id":"msg_1","output_index":0,"content_index":0}`,
				`data: {"type":"response.output_text.delta","delta":"\n2","item_id":"msg_1","output_index":0,"content_index":0}`,
				"data: [DONE]",
			)
		}

		chatModel := tc.Client.ChatModel(getResponsesModelName(), nil)
		messages := []*ChatMessage{
			{Role: ChatRoleUser, Parts: []ChatMessagePart{{Text: "Count to 2"}}},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Greater(t, len(fragments), 0)
	})
}

func TestResponsesClient_ToolCalling(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	weatherTool := tool.ToolInfo{
		Name:        "get_weather",
		Description: "Get the current weather in a given location",
		Schema: tool.ToolSchema{
			Type: tool.SchemaTypeObject,
			Properties: map[string]tool.PropertySchema{
				"location": {Type: tool.SchemaTypeString, Description: "City"},
			},
			Required:             []string{"location"},
			AdditionalProperties: false,
		},
	}

	t.Run("tool calls are returned in response", func(t *testing.T) {
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/responses", "POST", `{"id":"resp_tool","object":"response","status":"completed","output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"get_weather","arguments":"{\"location\":\"Paris\"}"}]}`)
		}

		chatModel := tc.Client.ChatModel(getResponsesModelName(), nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Check Paris weather")}

		response, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})

		require.NoError(t, err)
		toolCalls := response.GetToolCalls()
		require.NotEmpty(t, toolCalls)
		assert.Equal(t, "get_weather", toolCalls[0].Function)
	})

	t.Run("tool responses are sent as function_call_output", func(t *testing.T) {
		if tc.Mock == nil {
			t.Skip("Skipping test: mock server required")
		}
		tc.Mock.AddRestResponse("/responses", "POST", `{"id":"resp_2","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`)

		call := &tool.ToolCall{
			ID:       "call_2",
			Function: "get_weather",
			Arguments: tool.NewToolValue(map[string]any{
				"location": "Berlin",
			}),
		}
		toolResp := &tool.ToolResponse{
			Call:   call,
			Result: tool.NewToolValue(map[string]any{"temp": 18}),
			Done:   true,
		}

		chatModel := tc.Client.ChatModel(getResponsesModelName(), nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Check Berlin"),
			NewToolCallMessage(call),
			NewToolResponseMessage(toolResp),
		}

		_, err := chatModel.Chat(ctx, messages, nil, []tool.ToolInfo{weatherTool})
		require.NoError(t, err)

		reqs := tc.Mock.GetRequests()
		require.NotEmpty(t, reqs)

		var chatReq ResponsesCreateRequest
		require.NoError(t, json.Unmarshal(reqs[len(reqs)-1].Body, &chatReq))

		found := false
		for _, item := range chatReq.Input {
			if item.Type == "function_call_output" && item.CallID == "call_2" {
				found = true
				break
			}
		}

		assert.True(t, found, "expected function_call_output item in request")
	})
}

func TestResponsesClient_ToolCallingStream(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("streams tool call arguments", func(t *testing.T) {
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/responses", "POST", true,
				`data: {"type":"response.output_item.added","item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"get_weather","arguments":""}}`,
				`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"location\":\"Rome\""}`,
				`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"}"}`,
				`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","arguments":"{\"location\":\"Rome\"}"}`,
				"data: [DONE]",
			)
		}

		chatModel := tc.Client.ChatModel(getResponsesModelName(), nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Weather Rome")}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		require.NotNil(t, iterator)

		var toolCalls []*tool.ToolCall
		for fragment := range iterator {
			toolCalls = append(toolCalls, fragment.GetToolCalls()...)
		}

		assert.NotEmpty(t, toolCalls)
		if len(toolCalls) > 0 {
			assert.Equal(t, "get_weather", toolCalls[0].Function)
		}
	})
}

func TestResponsesClient_Logging(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("logs request and response", func(t *testing.T) {
		if tc.Mock != nil {
			tc.Mock.AddRestResponse("/responses", "POST", `{"id":"resp_log","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Logged"}]}]}`)
		}

		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		logger := slog.New(handler)

		chatModel := tc.Client.ChatModel(getResponsesModelName(), &ChatOptions{Logger: logger})
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Test logging")}

		_, err := chatModel.Chat(ctx, messages, nil, nil)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "llm_request")
		assert.Contains(t, logOutput, "llm_response")
		assert.Contains(t, logOutput, "model")
	})
}

func TestResponsesClient_ContextLengthLimit(t *testing.T) {
	t.Run("Chat method uses ContextLengthLimit as max_output_tokens", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:                mock.URL(),
			APIKey:             "test-key",
			ContextLengthLimit: 2048,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/responses", "POST", `{"id":"resp_limit","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello"}]}]}`)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq ResponsesCreateRequest
		require.NoError(t, json.Unmarshal(reqs[0].Body, &chatReq))
		assert.Equal(t, 2048, chatReq.MaxOutputTokens)
	})
}

func TestResponsesClient_RequestHeadersAndSessionID(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewResponsesClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "test-key",
		Headers: map[string]string{
			"chatgpt-account-id": "acct_123",
			"originator":         "opencode",
			"session-id":         "ses_header",
			"Authorization":      "Bearer override",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/responses", "POST", `{"id":"resp_headers","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleDeveloper, "Developer instruction"),
		NewTextMessage(ChatRoleUser, "Hello"),
	}

	_, err = chatModel.Chat(context.Background(), messages, &ChatOptions{SessionID: "ses_body"}, nil)
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "Bearer test-key", request.Header.Get("Authorization"))
	assert.Equal(t, "acct_123", request.Header.Get("chatgpt-account-id"))
	assert.Equal(t, "opencode", request.Header.Get("originator"))
	assert.Equal(t, "ses_header", request.Header.Get("session-id"))

	var chatReq ResponsesCreateRequest
	require.NoError(t, json.Unmarshal(request.Body, &chatReq))
	assert.Equal(t, "ses_body", chatReq.PromptCacheKey)

	foundDeveloper := false
	for _, item := range chatReq.Input {
		if item.Role == "developer" {
			foundDeveloper = true
			break
		}
	}
	assert.True(t, foundDeveloper, "expected developer role in request input")
}

func TestResponsesClient_EmbeddingModel(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("returns error for embedding model", func(t *testing.T) {
		embedModel := tc.Client.EmbeddingModel("any-model")

		assert.NotNil(t, embedModel)

		_, err := embedModel.Embed(ctx, "Hello")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented")
	})
}

func TestResponsesClient_StreamLogging(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("logs request and chunks in stream", func(t *testing.T) {
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/responses", "POST", true,
				`data: {"type":"response.output_text.delta","delta":"Chunk1","item_id":"msg_1","output_index":0,"content_index":0}`,
				"data: [DONE]",
			)
		}

		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		logger := slog.New(handler)

		chatModel := tc.Client.ChatModel(getResponsesModelName(), &ChatOptions{Logger: logger})
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Test")}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		for range iterator {
		}

		logOutput := buf.String()
		assert.Contains(t, logOutput, "llm_request")
		assert.Contains(t, logOutput, "llm_response")

		responseCount := strings.Count(logOutput, `"msg":"llm_response"`)
		assert.GreaterOrEqual(t, responseCount, 1)
	})
}

func TestResponsesClient_OptionsHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewResponsesClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "config-api-key",
		Headers: map[string]string{
			"X-Config-Header": "config-value",
			"X-Shared-Header": "config-shared",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/responses", "POST", `{"id":"resp_opts","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello"}]}]}`)

	options := &ChatOptions{
		Headers: map[string]string{
			"X-Options-Header": "options-value",
			"X-Shared-Header":  "options-shared",
			"Authorization":    "Bearer options-auth",
		},
	}

	chatModel := client.ChatModel("test-model", options)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hello"),
	}

	_, err = chatModel.Chat(context.Background(), messages, nil, nil)
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "config-value", request.Header.Get("X-Config-Header"))
	assert.Equal(t, "options-value", request.Header.Get("X-Options-Header"))
	assert.Equal(t, "options-shared", request.Header.Get("X-Shared-Header"), "options headers should override config headers")
	assert.Equal(t, "Bearer config-api-key", request.Header.Get("Authorization"), "authorization header should NOT be overridden by options")
}

func TestResponsesClient_OptionsHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewResponsesClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "config-api-key",
		Headers: map[string]string{
			"X-Config-Header": "config-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/responses", "POST", true,
		`data: {"type":"response.output_text.delta","delta":"Hi","item_id":"msg_1","output_index":0,"content_index":0}`,
		"data: [DONE]",
	)

	options := &ChatOptions{
		Headers: map[string]string{
			"X-Options-Header": "options-value",
			"X-Api-Key":        "should-not-override",
		},
	}

	chatModel := client.ChatModel("test-model", options)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hi"),
	}

	iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
	for range iterator {
	}

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "config-value", request.Header.Get("X-Config-Header"))
	assert.Equal(t, "options-value", request.Header.Get("X-Options-Header"))
	assert.Empty(t, request.Header.Get("X-Api-Key"), "api-key header should NOT be set from options")
}

func TestResponsesClient_RateLimitError(t *testing.T) {
	t.Run("returns rate limit error with retry-after header", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		headers := http.Header{}
		headers.Set("Retry-After", "60")
		mock.AddRestResponseWithStatusAndHeaders("/responses", "POST", `{"error":{"message":"Rate limit exceeded","type":"rate_limit_error"}}`, http.StatusTooManyRequests, headers)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		assert.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 60, rateLimitErr.RetryAfterSeconds)
		assert.Contains(t, rateLimitErr.Error(), "Rate limit exceeded")
	})

	t.Run("returns rate limit error without retry-after header", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		mock.AddRestResponseWithStatus("/responses", "POST", `rate limit exceeded`, http.StatusTooManyRequests)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		assert.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 0, rateLimitErr.RetryAfterSeconds)
	})

	t.Run("wraps underlying error with ErrRateExceeded", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		mock.AddRestResponseWithStatus("/responses", "POST", "rate limit exceeded", http.StatusTooManyRequests)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrRateExceeded))
	})
}
