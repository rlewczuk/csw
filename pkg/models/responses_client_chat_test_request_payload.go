package models

import (
	"context"
	"encoding/json"
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

func TestResponsesClient_RequestPrefixStability(t *testing.T) {
	t.Run("chat request JSON is stable for equivalent map-backed inputs", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
			Options: map[string]any{
				"prompt_cache_retention": "24h",
			},
		})
		require.NoError(t, err)

		mock.AddRestResponse("/responses", "POST", `{"id":"resp_a","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`)
		mock.AddRestResponse("/responses", "POST", `{"id":"resp_b","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`)

		options := &ChatOptions{SessionID: "session-1"}
		chatModel := client.ChatModel("test-model", options)

		messagesA := []*ChatMessage{
			NewTextMessage(ChatRoleSystem, "Static system instructions"),
			NewTextMessage(ChatRoleUser, "Run tool"),
			NewToolCallMessage(
				&tool.ToolCall{
					ID:       "call_1",
					Function: "read",
					Arguments: tool.NewToolValue(map[string]any{
						"z": map[string]any{"b": 2, "a": 1},
						"a": []any{map[string]any{"y": "2", "x": "1"}},
					}),
				}),
			NewToolResponseMessage(&tool.ToolResponse{
				Call: &tool.ToolCall{ID: "call_1", Function: "read"},
				Result: tool.NewToolValue(map[string]any{
					"meta": map[string]any{"m2": "two", "m1": "one"},
					"list": []any{map[string]any{"k2": "v2", "k1": "v1"}},
				}),
			}),
		}

		messagesB := []*ChatMessage{
			NewTextMessage(ChatRoleSystem, "Static system instructions"),
			NewTextMessage(ChatRoleUser, "Run tool"),
			NewToolCallMessage(
				&tool.ToolCall{
					ID:       "call_1",
					Function: "read",
					Arguments: tool.NewToolValue(map[string]any{
						"a": []any{map[string]any{"x": "1", "y": "2"}},
						"z": map[string]any{"a": 1, "b": 2},
					}),
				}),
			NewToolResponseMessage(&tool.ToolResponse{
				Call: &tool.ToolCall{ID: "call_1", Function: "read"},
				Result: tool.NewToolValue(map[string]any{
					"list": []any{map[string]any{"k1": "v1", "k2": "v2"}},
					"meta": map[string]any{"m1": "one", "m2": "two"},
				}),
			}),
		}

		toolsA := []tool.ToolInfo{
			{
				Name:        "read",
				Description: "Read file",
				Schema: tool.ToolSchema{
					Type: tool.SchemaTypeObject,
					Properties: map[string]tool.PropertySchema{
						"path": {
							Type: tool.SchemaTypeString,
						},
						"opts": {
							Type: tool.SchemaTypeObject,
							Properties: map[string]tool.PropertySchema{
								"b": {Type: tool.SchemaTypeString},
								"a": {Type: tool.SchemaTypeString},
							},
							Required:             []string{"b", "a"},
							AdditionalProperties: boolPtr(false),
						},
					},
					Required:             []string{"path", "opts"},
					AdditionalProperties: false,
				},
			},
		}

		toolsB := []tool.ToolInfo{
			{
				Name:        "read",
				Description: "Read file",
				Schema: tool.ToolSchema{
					Type: tool.SchemaTypeObject,
					Properties: map[string]tool.PropertySchema{
						"opts": {
							Type: tool.SchemaTypeObject,
							Properties: map[string]tool.PropertySchema{
								"a": {Type: tool.SchemaTypeString},
								"b": {Type: tool.SchemaTypeString},
							},
							Required:             []string{"a", "b"},
							AdditionalProperties: boolPtr(false),
						},
						"path": {
							Type: tool.SchemaTypeString,
						},
					},
					Required:             []string{"opts", "path"},
					AdditionalProperties: false,
				},
			},
		}

		_, err = chatModel.Chat(context.Background(), messagesA, options, toolsA)
		require.NoError(t, err)
		_, err = chatModel.Chat(context.Background(), messagesB, options, toolsB)
		require.NoError(t, err)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 2)
		assert.JSONEq(t, string(reqs[0].Body), string(reqs[1].Body))

		var payload ResponsesCreateRequest
		require.NoError(t, json.Unmarshal(reqs[0].Body, &payload))
		assert.Equal(t, "24h", payload.PromptCacheRetention)
	})
}

func boolPtr(v bool) *bool {
	return &v
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

func TestResponsesClient_SystemMessageInInstructions(t *testing.T) {
	t.Run("system message is placed in instructions and not in input items", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/responses", r.URL.Path)

			var chatReq ResponsesCreateRequest
			err := json.NewDecoder(r.Body).Decode(&chatReq)
			require.NoError(t, err)

			assert.Contains(t, chatReq.Instructions, "You are a helpful coding assistant")

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

			assert.Contains(t, chatReq.Instructions, "You are a helpful coding assistant")

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

			assert.Contains(t, chatReq.Instructions, "You are a helpful coding assistant")

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
