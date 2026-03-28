package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type unexpectedEOFReadCloser struct{}

func (r *unexpectedEOFReadCloser) Read(p []byte) (int, error) {
	_ = p
	return 0, io.ErrUnexpectedEOF
}

func (r *unexpectedEOFReadCloser) Close() error {
	return nil
}

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

func TestResponsesClient_ListModels_ResponsesModelsPayload(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	if tc.Mock == nil {
		t.Skip("Skipping responses payload assertions against real provider")
	}

	tc.Mock.AddRestResponse("/models", "GET", `{"models":[{"slug":"gpt-5.2-codex","display_name":"gpt-5.2-codex"},{"slug":"gpt-5.2","display_name":"gpt-5.2"}]}`)

	modelList, err := tc.Client.ListModels()
	require.NoError(t, err)
	require.Len(t, modelList, 2)

	assert.Equal(t, "gpt-5.2-codex", modelList[0].Name)
	assert.Equal(t, "gpt-5.2-codex", modelList[0].Model)
	assert.Equal(t, "gpt-5.2", modelList[1].Name)
	assert.Equal(t, "gpt-5.2", modelList[1].Model)
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

func TestResponsesClient_ChatTokenUsage(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	if tc.Mock == nil {
		t.Skip("Skipping token usage assertions against real provider")
	}

	tc.Mock.AddRestResponse("/responses", "POST", `{"id":"resp_usage","object":"response","status":"completed","usage":{"input_tokens":14,"output_tokens":10,"total_tokens":24,"input_tokens_details":{"cached_tokens":4}},"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`)

	chatModel := tc.Client.ChatModel(getResponsesModelName(), nil)
	resp, err := chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.TokenUsage)
	assert.Equal(t, 14, resp.TokenUsage.InputTokens)
	assert.Equal(t, 4, resp.TokenUsage.InputCachedTokens)
	assert.Equal(t, 10, resp.TokenUsage.InputNonCachedTokens)
	assert.Equal(t, 10, resp.TokenUsage.OutputTokens)
	assert.Equal(t, 24, resp.TokenUsage.TotalTokens)
	assert.Equal(t, 24, resp.ContextLengthTokens)
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

func TestResponsesClient_MaxTokens(t *testing.T) {
	t.Run("Chat method uses MaxTokens as max_output_tokens", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			APIKey:    "test-key",
			MaxTokens: 2048,
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

func TestResponsesClient_ChatRequiresInstructions(t *testing.T) {
	t.Run("chat request includes instructions when only user message is provided", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/responses", r.URL.Path)

			var chatReq ResponsesCreateRequest
			err := json.NewDecoder(r.Body).Decode(&chatReq)
			require.NoError(t, err)

			if strings.TrimSpace(chatReq.Instructions) == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, writeErr := w.Write([]byte(`{"detail":"Instructions are required"}`))
				require.NoError(t, writeErr)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{"id":"resp_ok","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
			require.NoError(t, writeErr)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL,
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)
	})
}

func TestResponsesClient_ChatSetsStoreFalse(t *testing.T) {
	t.Run("chat request sets store to false", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/responses", r.URL.Path)

			var chatReq ResponsesCreateRequest
			err := json.NewDecoder(r.Body).Decode(&chatReq)
			require.NoError(t, err)

			if chatReq.Store == nil || *chatReq.Store {
				w.WriteHeader(http.StatusBadRequest)
				_, writeErr := w.Write([]byte(`{"detail":"Store must be set to false"}`))
				require.NoError(t, writeErr)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{"id":"resp_ok","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
			require.NoError(t, writeErr)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL,
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)
	})
}

func TestResponsesClient_CodexCompatibility(t *testing.T) {
	t.Run("Chat uses streaming and omits max_output_tokens for codex endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/backend-api/codex/responses", r.URL.Path)

			var payload map[string]any
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)

			streamValue, ok := payload["stream"].(bool)
			if !ok || !streamValue {
				w.WriteHeader(http.StatusBadRequest)
				_, writeErr := w.Write([]byte(`{"detail":"Stream must be set to true"}`))
				require.NoError(t, writeErr)
				return
			}

			if _, exists := payload["max_output_tokens"]; exists {
				w.WriteHeader(http.StatusBadRequest)
				_, writeErr := w.Write([]byte(`{"detail":"Unsupported parameter: max_output_tokens"}`))
				require.NoError(t, writeErr)
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			_, writeErr := w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"))
			require.NoError(t, writeErr)
			_, writeErr = w.Write([]byte("data: [DONE]\n\n"))
			require.NoError(t, writeErr)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL + "/backend-api/codex",
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("gpt-5.2-codex", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		response, err := chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, "ok", response.GetText())
	})

	t.Run("ChatStream omits max_output_tokens for codex endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/backend-api/codex/responses", r.URL.Path)

			var payload map[string]any
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)

			streamValue, ok := payload["stream"].(bool)
			require.True(t, ok)
			assert.True(t, streamValue)
			_, exists := payload["max_output_tokens"]
			assert.False(t, exists)

			w.Header().Set("Content-Type", "text/event-stream")
			_, writeErr := w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"))
			require.NoError(t, writeErr)
			_, writeErr = w.Write([]byte("data: [DONE]\n\n"))
			require.NoError(t, writeErr)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL + "/backend-api/codex",
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("gpt-5.2-codex", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		var gotText strings.Builder
		for fragment := range chatModel.ChatStream(context.Background(), messages, nil, nil) {
			gotText.WriteString(fragment.GetText())
		}

		assert.Equal(t, "ok", gotText.String())
	})
}

func TestConvertFromResponsesStreamBody_ExtendsScannerBufferForLargeTokens(t *testing.T) {
	longDelta := strings.Repeat("a", 70*1024)
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"" + longDelta + "\"}\n\n" +
		"data: [DONE]\n\n"

	message, err := convertFromResponsesStreamBody([]byte(body))
	require.NoError(t, err)
	require.NotNil(t, message)
	assert.Equal(t, longDelta, message.GetText())
}

func TestConvertFromResponsesStreamBody_HandlesStreamErrorEvent(t *testing.T) {
	t.Run("returns ErrTooManyInputTokens for context_length_exceeded", func(t *testing.T) {
		body := strings.Join([]string{
			"event: error",
			"data: {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"code\":\"context_length_exceeded\",\"message\":\"Your input exceeds the context window of this model. Please adjust your input and try again.\",\"param\":\"input\"},\"sequence_number\":2}",
			"",
		}, "\n")

		_, err := convertFromResponsesStreamBody([]byte(body))
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTooManyInputTokens)
	})

	t.Run("returns APIRequestError with details for non-context stream errors", func(t *testing.T) {
		body := strings.Join([]string{
			"event: error",
			"data: {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"code\":\"unsupported_parameter\",\"message\":\"Unsupported parameter: foo\",\"param\":\"foo\"},\"sequence_number\":2}",
			"",
		}, "\n")

		_, err := convertFromResponsesStreamBody([]byte(body))
		require.Error(t, err)

		var reqErr *APIRequestError
		require.ErrorAs(t, err, &reqErr)
		assert.Equal(t, "invalid_request_error", reqErr.ErrorType)
		assert.Equal(t, "unsupported_parameter", reqErr.Code)
		assert.Equal(t, "foo", reqErr.Param)
		assert.Equal(t, "Unsupported parameter: foo", reqErr.Message)
		assert.Contains(t, err.Error(), "invalid_request_error")
		assert.Contains(t, err.Error(), "unsupported_parameter")
		assert.Contains(t, err.Error(), "foo")
	})
}

func TestFormatRawHTTPResponse(t *testing.T) {
	t.Run("formats status headers and body as multiline string", func(t *testing.T) {
		headers := http.Header{}
		headers.Add("X-Zeta", "z")
		headers.Add("X-Alpha", "a1")
		headers.Add("X-Alpha", "a2")

		raw := formatRawHTTPResponse(http.StatusBadRequest, headers, []byte("body-line"))

		assert.Equal(t, "400\nX-Alpha: a1\nX-Alpha: a2\nX-Zeta: z\n\nbody-line", raw)
	})

	t.Run("formats response with empty headers and empty body", func(t *testing.T) {
		raw := formatRawHTTPResponse(http.StatusOK, nil, nil)
		assert.Equal(t, "200\n\n", raw)
	})
}

func TestResponsesClient_ChatWrapsLLMRequestError(t *testing.T) {
	t.Run("wraps codex stream conversion error with raw response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, writeErr := w.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
			require.NoError(t, writeErr)
			_, writeErr = w.Write([]byte("data: [DONE]\n\n"))
			require.NoError(t, writeErr)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL + "/backend-api/codex",
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("gpt-5.2-codex", nil)
		_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}, nil, nil)
		require.Error(t, err)

		var llmErr *LLMRequestError
		require.True(t, errors.As(err, &llmErr))
		assert.Contains(t, err.Error(), "convertFromResponsesStreamBody() [responses_client.go]: no usable output items in response")
		assert.Contains(t, llmErr.RawResponse, "200\n")
		assert.Contains(t, llmErr.RawResponse, "Content-Type: text/event-stream")
		assert.Contains(t, llmErr.RawResponse, "\n\n")
		assert.Contains(t, llmErr.RawResponse, "response.completed")
	})

	t.Run("wraps network request failure with empty raw response", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return nil, io.EOF
			}),
		}

		client, err := NewResponsesClientWithHTTPClient("https://example.test", httpClient)
		require.NoError(t, err)

		chatModel := client.ChatModel("test-model", nil)
		_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}, nil, nil)
		require.Error(t, err)

		var llmErr *LLMRequestError
		require.True(t, errors.As(err, &llmErr))
		assert.Empty(t, llmErr.RawResponse)
	})

	t.Run("wraps non-stream decode error with raw response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte("not-json"))
			require.NoError(t, writeErr)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL,
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("test-model", nil)
		_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}, nil, nil)
		require.Error(t, err)

		var llmErr *LLMRequestError
		require.True(t, errors.As(err, &llmErr))
		assert.Contains(t, err.Error(), "failed to decode response")
		assert.Contains(t, llmErr.RawResponse, "200\n")
		assert.Contains(t, llmErr.RawResponse, "Content-Type: application/json")
		assert.True(t, strings.HasSuffix(llmErr.RawResponse, "not-json"))
	})

	t.Run("wraps response body unexpected EOF as retryable network error", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       &unexpectedEOFReadCloser{},
					Request:    req,
				}, nil
			}),
		}

		client, err := NewResponsesClientWithHTTPClient("https://example.test", httpClient)
		require.NoError(t, err)

		chatModel := client.ChatModel("test-model", nil)
		_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}, nil, nil)
		require.Error(t, err)

		var networkErr *NetworkError
		require.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
		assert.Contains(t, networkErr.Message, "unexpected stream EOF")
		assert.Contains(t, err.Error(), "failed to read response body")
	})
}

func TestResponsesClient_SystemMessageInInstructions(t *testing.T) {
	t.Run("system message is placed in instructions and not in input items", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/responses", r.URL.Path)

			var chatReq ResponsesCreateRequest
			err := json.NewDecoder(r.Body).Decode(&chatReq)
			require.NoError(t, err)

			// Verify instructions contain the system message
			assert.Contains(t, chatReq.Instructions, "You are a helpful coding assistant")

			// Verify no system role in input items
			for _, item := range chatReq.Input {
				assert.NotEqual(t, "system", item.Role, "system messages should not be in input items")
			}

			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{"id":"resp_ok","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
			require.NoError(t, writeErr)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL,
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleSystem, "You are a helpful coding assistant"),
			NewTextMessage(ChatRoleUser, "Hello"),
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)
	})

	t.Run("system message is placed in instructions for codex endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/backend-api/codex/responses", r.URL.Path)

			var chatReq ResponsesCreateRequest
			err := json.NewDecoder(r.Body).Decode(&chatReq)
			require.NoError(t, err)

			// Verify instructions contain the system message
			assert.Contains(t, chatReq.Instructions, "You are a helpful coding assistant")

			// Verify no system role in input items
			for _, item := range chatReq.Input {
				assert.NotEqual(t, "system", item.Role, "system messages should not be in input items")
			}

			w.Header().Set("Content-Type", "text/event-stream")
			_, writeErr := w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"))
			require.NoError(t, writeErr)
			_, writeErr = w.Write([]byte("data: [DONE]\n\n"))
			require.NoError(t, writeErr)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL + "/backend-api/codex",
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("gpt-5.2-codex", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleSystem, "You are a helpful coding assistant"),
			NewTextMessage(ChatRoleUser, "Hello"),
		}

		response, err := chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, "ok", response.GetText())
	})

	t.Run("system message is placed in instructions for ChatStream", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/backend-api/codex/responses", r.URL.Path)

			var chatReq ResponsesCreateRequest
			err := json.NewDecoder(r.Body).Decode(&chatReq)
			require.NoError(t, err)

			// Verify instructions contain the system message
			assert.Contains(t, chatReq.Instructions, "You are a helpful coding assistant")

			// Verify no system role in input items
			for _, item := range chatReq.Input {
				assert.NotEqual(t, "system", item.Role, "system messages should not be in input items")
			}

			w.Header().Set("Content-Type", "text/event-stream")
			_, writeErr := w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"))
			require.NoError(t, writeErr)
			_, writeErr = w.Write([]byte("data: [DONE]\n\n"))
			require.NoError(t, writeErr)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL + "/backend-api/codex",
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("gpt-5.2-codex", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleSystem, "You are a helpful coding assistant"),
			NewTextMessage(ChatRoleUser, "Hello"),
		}

		var gotText strings.Builder
		for fragment := range chatModel.ChatStream(context.Background(), messages, nil, nil) {
			gotText.WriteString(fragment.GetText())
		}

		assert.Equal(t, "ok", gotText.String())
	})
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

func TestResponsesClient_RawLLMCallback_ObfuscatesRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Authorization", "Bearer response-secret-token")
		_, err := w.Write([]byte(`{"id":"resp_raw","object":"response","status":"completed","api_key":"response-secret-key","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewResponsesClient(&conf.ModelProviderConfig{
		URL:    server.URL,
		APIKey: "request-secret-api-key",
	})
	require.NoError(t, err)

	rawLines := make([]string, 0)
	client.SetRawLLMCallback(func(line string) {
		rawLines = append(rawLines, line)
	})

	chatModel := client.ChatModel("test-model", nil)
	_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hello")}, nil, nil)
	require.NoError(t, err)

	joined := strings.Join(rawLines, "\n")
	assert.Contains(t, joined, ">>> REQUEST POST ")
	assert.Contains(t, joined, ">>> HEADER Authorization: Bear...-key")
	assert.NotContains(t, joined, "request-secret-api-key")
	assert.Contains(t, joined, "<<< RESPONSE 200")
	assert.Contains(t, joined, "<<< HEADER Authorization: Bear...oken")
	assert.NotContains(t, joined, "response-secret-token")
	assert.Contains(t, joined, "api_key")
	assert.NotContains(t, joined, "response-secret-key")
}

func TestResponsesClient_RawLLMCallback_LogsStreamingChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, err := w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"A\"}\n\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"B\"}\n\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("data: [DONE]\n\n"))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewResponsesClient(&conf.ModelProviderConfig{
		URL:    server.URL,
		APIKey: "stream-secret-key",
	})
	require.NoError(t, err)

	rawLines := make([]string, 0)
	client.SetRawLLMCallback(func(line string) {
		rawLines = append(rawLines, line)
	})

	chatModel := client.ChatModel("test-model", nil)
	for range chatModel.ChatStream(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hello")}, nil, nil) {
	}

	joined := strings.Join(rawLines, "\n")
	assert.Contains(t, joined, "<<< CHUNK data: {\"type\":\"response.output_text.delta\",\"delta\":\"A\"}")
	assert.Contains(t, joined, "<<< CHUNK data: {\"type\":\"response.output_text.delta\",\"delta\":\"B\"}")
	assert.Contains(t, joined, "<<< CHUNK data: [DONE]")
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
	t.Run("returns codex primary reset seconds for usage limit reached", func(t *testing.T) {
		tc := getResponsesTestClient(t)
		defer tc.Close()

		if tc.Mock == nil {
			t.Skip("Skipping codex usage-limit header assertions against real provider")
		}

		headers := http.Header{}
		headers.Set("X-Codex-Primary-Used-Percent", "100")
		headers.Set("X-Codex-Primary-Reset-After-Seconds", "4348")
		headers.Set("X-Codex-Secondary-Used-Percent", "77")
		headers.Set("X-Codex-Secondary-Reset-After-Seconds", "68891")
		tc.Mock.AddRestResponseWithStatusAndHeaders(
			"/responses",
			"POST",
			`{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached","resets_in_seconds":4347}}`,
			http.StatusTooManyRequests,
			headers,
		)

		chatModel := tc.Client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err := chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 4348, rateLimitErr.RetryAfterSeconds)
		assert.Equal(t, "The usage limit has been reached", rateLimitErr.Message)
	})

	t.Run("returns codex secondary reset seconds when secondary limit is exceeded", func(t *testing.T) {
		tc := getResponsesTestClient(t)
		defer tc.Close()

		if tc.Mock == nil {
			t.Skip("Skipping codex usage-limit header assertions against real provider")
		}

		headers := http.Header{}
		headers.Set("X-Codex-Primary-Used-Percent", "95")
		headers.Set("X-Codex-Primary-Reset-After-Seconds", "12")
		headers.Set("X-Codex-Secondary-Used-Percent", "100")
		headers.Set("X-Codex-Secondary-Reset-After-Seconds", "68891")
		tc.Mock.AddRestResponseWithStatusAndHeaders(
			"/responses",
			"POST",
			`{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached","resets_in_seconds":68890}}`,
			http.StatusTooManyRequests,
			headers,
		)

		chatModel := tc.Client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err := chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 68891, rateLimitErr.RetryAfterSeconds)
	})

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

func TestResponsesClient_OAuthTokenRenewal(t *testing.T) {
	t.Run("RefreshTokenIfNeeded returns nil for non-OAuth2 provider", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:      mock.URL(),
			APIKey:   "test-key",
			AuthMode: conf.AuthModeAPIKey,
		})
		require.NoError(t, err)

		err = client.RefreshTokenIfNeeded()
		assert.NoError(t, err)
	})

	t.Run("RefreshTokenIfNeeded refreshes expired token", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		// Mock the token refresh endpoint
		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		err = client.RefreshTokenIfNeeded()
		require.NoError(t, err)

		// Verify token was updated
		token, err := client.GetAccessToken()
		require.NoError(t, err)
		assert.Equal(t, "new-access-token", token)
		assert.Equal(t, "new-refresh-token", client.GetConfig().RefreshToken)
	})

	t.Run("RefreshTokenIfNeeded does not refresh valid token", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as valid for a long time
		client.tokenExpiry = time.Now().Add(1 * time.Hour)

		err = client.RefreshTokenIfNeeded()
		require.NoError(t, err)

		// No request should have been made
		reqs := mock.GetRequests()
		assert.Empty(t, reqs)
	})

	t.Run("RefreshTokenIfNeeded returns error when refresh fails", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		// Mock the token refresh endpoint with error
		mock.AddRestResponseWithStatus("/oauth/token", "POST", `{"error":"invalid_grant"}`, http.StatusBadRequest)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "invalid-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		err = client.RefreshTokenIfNeeded()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token renewal failed")
	})

	t.Run("GetAccessToken returns current token for non-OAuth2", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "static-api-key",
		})
		require.NoError(t, err)

		token, err := client.GetAccessToken()
		require.NoError(t, err)
		assert.Equal(t, "static-api-key", token)
	})

	t.Run("GetAccessToken refreshes token when expired", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"refreshed-token","expires_in":3600}`)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		token, err := client.GetAccessToken()
		require.NoError(t, err)
		assert.Equal(t, "refreshed-token", token)
	})

	t.Run("RefreshTokenIfNeeded persists config via ConfigUpdater", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		// Mock the token refresh endpoint
		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			Name:         "test-provider",
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		// Track if config updater was called
		var updaterCalled bool
		var savedConfig *conf.ModelProviderConfig
		client.SetConfigUpdater(func(config *conf.ModelProviderConfig) error {
			updaterCalled = true
			savedConfig = config
			return nil
		})

		err = client.RefreshTokenIfNeeded()
		require.NoError(t, err)

		// Verify config updater was called with updated config
		assert.True(t, updaterCalled)
		require.NotNil(t, savedConfig)
		assert.Equal(t, "new-access-token", savedConfig.APIKey)
		assert.Equal(t, "new-refresh-token", savedConfig.RefreshToken)
		assert.Equal(t, "test-provider", savedConfig.Name)
	})

	t.Run("RefreshTokenIfNeeded persists access token when refresh token missing", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		// Mock the token refresh endpoint (no refresh_token in response)
		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","expires_in":3600}`)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			Name:         "test-provider",
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		// Track if config updater was called
		var updaterCalled bool
		client.SetConfigUpdater(func(config *conf.ModelProviderConfig) error {
			updaterCalled = true
			return nil
		})

		err = client.RefreshTokenIfNeeded()
		require.NoError(t, err)

		// Config updater should be called to persist new access token
		assert.True(t, updaterCalled)
	})

	t.Run("RefreshTokenIfNeeded continues when ConfigUpdater fails", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		// Mock the token refresh endpoint
		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			Name:         "test-provider",
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		// Set a failing config updater
		var updaterCalled bool
		client.SetConfigUpdater(func(config *conf.ModelProviderConfig) error {
			updaterCalled = true
			return assert.AnError
		})

		err = client.RefreshTokenIfNeeded()
		// Token refresh should still succeed even if config persistence fails
		require.NoError(t, err)

		// Verify config updater was called
		assert.True(t, updaterCalled)

		// Verify in-memory config was still updated
		token, err := client.GetAccessToken()
		require.NoError(t, err)
		assert.Equal(t, "new-access-token", token)
		assert.Equal(t, "new-access-token", client.GetConfig().APIKey)
		assert.Equal(t, "new-refresh-token", client.GetConfig().RefreshToken)
	})

	t.Run("RefreshTokenIfNeeded does not eagerly renew opaque non-expiring token", func(t *testing.T) {
		var refreshCalls int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/oauth/token" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			atomic.AddInt32(&refreshCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"access_token":"should-not-be-used","expires_in":3600}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          server.URL,
			APIKey:       "opaque-token",
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     server.URL + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Unknown expiry (zero) with existing token should not cause eager refresh.
		client.tokenExpiry = time.Time{}

		err = client.RefreshTokenIfNeeded()
		require.NoError(t, err)

		assert.Equal(t, int32(0), atomic.LoadInt32(&refreshCalls))
	})

	t.Run("Chat retries on 401 expired token and refreshes once", func(t *testing.T) {
		var tokenRefreshCalls int32
		var responsesCalls int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/oauth/token":
				atomic.AddInt32(&tokenRefreshCalls, 1)
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`))
				require.NoError(t, err)
			case "/responses":
				atomic.AddInt32(&responsesCalls, 1)
				authHeader := r.Header.Get("Authorization")
				if authHeader == "Bearer old-access-token" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Header().Set("Content-Type", "application/json")
					_, err := w.Write([]byte(`{"error":{"message":"access token expired","code":"invalid_token"}}`))
					require.NoError(t, err)
					return
				}

				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"id":"resp_ok","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          server.URL,
			APIKey:       "old-access-token",
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     server.URL + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Token appears fresh locally, refresh should happen only after expired-token 401.
		client.tokenExpiry = time.Now().Add(1 * time.Hour)

		chatModel := client.ChatModel("test-model", nil)
		resp, err := chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "ok", resp.GetText())
		assert.Equal(t, int32(1), atomic.LoadInt32(&tokenRefreshCalls))
		assert.Equal(t, int32(2), atomic.LoadInt32(&responsesCalls))
	})

	t.Run("parallel 401 retries perform only one token renewal", func(t *testing.T) {
		var tokenRefreshCalls int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/oauth/token":
				atomic.AddInt32(&tokenRefreshCalls, 1)
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"access_token":"shared-new-token","refresh_token":"shared-new-refresh","expires_in":3600}`))
				require.NoError(t, err)
			case "/responses":
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer shared-new-token" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_, err := w.Write([]byte(`{"error":{"message":"token expired","code":"invalid_token"}}`))
					require.NoError(t, err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"id":"resp_parallel","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          server.URL,
			APIKey:       "stale-token",
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     server.URL + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)
		client.tokenExpiry = time.Now().Add(1 * time.Hour)

		chatModel := client.ChatModel("test-model", nil)
		const workers = 6
		var wg sync.WaitGroup
		errCh := make(chan error, workers)

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, chatErr := chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, nil)
				errCh <- chatErr
			}()
		}

		wg.Wait()
		close(errCh)

		for chatErr := range errCh {
			require.NoError(t, chatErr)
		}

		assert.Equal(t, int32(1), atomic.LoadInt32(&tokenRefreshCalls))
	})

	t.Run("SetConfigUpdater can be called multiple times", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		// Set first updater
		var callCount int
		client.SetConfigUpdater(func(config *conf.ModelProviderConfig) error {
			callCount++
			return nil
		})

		// Replace with second updater
		client.SetConfigUpdater(func(config *conf.ModelProviderConfig) error {
			callCount += 10
			return nil
		})

		// Verify second updater is used (for non-OAuth2 this won't be called)
		// This test just verifies the setter works
		assert.NotNil(t, client.configUpdater)
	})
}

func TestResponsesClient_QueryParams(t *testing.T) {
	t.Run("list models includes configured query params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "0.1.0", r.URL.Query().Get("client_version"))

			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"data":[{"id":"test-model","object":"model","created":1640000000,"owned_by":"openai"}]}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL,
			APIKey: "test-key",
			QueryParams: map[string]string{
				"client_version": "0.1.0",
			},
		})
		require.NoError(t, err)

		models, err := client.ListModels()
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "test-model", models[0].Name)
	})

	t.Run("list models includes multiple query params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "0.1.0", r.URL.Query().Get("client_version"))
			assert.Equal(t, "test-value", r.URL.Query().Get("custom_param"))

			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"data":[{"id":"test-model","object":"model","created":1640000000,"owned_by":"openai"}]}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL,
			APIKey: "test-key",
			QueryParams: map[string]string{
				"client_version": "0.1.0",
				"custom_param":   "test-value",
			},
		})
		require.NoError(t, err)

		models, err := client.ListModels()
		require.NoError(t, err)
		require.Len(t, models, 1)
	})
}
