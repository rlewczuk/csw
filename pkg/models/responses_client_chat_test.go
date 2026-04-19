package models

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	assert.Contains(t, joined, `<<< CHUNK data: {"type":"response.output_text.delta","delta":"A"}`)
	assert.Contains(t, joined, `<<< CHUNK data: {"type":"response.output_text.delta","delta":"B"}`)
	assert.Contains(t, joined, "<<< CHUNK data: [DONE]")
}
