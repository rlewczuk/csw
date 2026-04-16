package models

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicClient_RawLLMCallback_ObfuscatesRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Api-Key", "response-secret-key")
		_, err := w.Write([]byte(`{"id":"msg_raw","type":"message","role":"assistant","model":"test-model","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"output_tokens":5}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewAnthropicClient(&conf.ModelProviderConfig{
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
	assert.Contains(t, joined, ">>> HEADER X-Api-Key: requ...-key")
	assert.NotContains(t, joined, "request-secret-api-key")
	assert.Contains(t, joined, "<<< RESPONSE 200")
	assert.Contains(t, joined, "<<< HEADER X-Api-Key: resp...-key")
	assert.NotContains(t, joined, "response-secret-key")
}

func TestAnthropicClient_RawLLMCallback_LogsStreamingChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, err := w.Write([]byte("event: message_start\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"test-model\"}}\n\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("event: content_block_delta\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"A\"}}\n\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("event: content_block_delta\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"B\"}}\n\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("event: message_delta\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("event: message_stop\n"))
		require.NoError(t, err)
		_, err = w.Write([]byte("data: {\"type\":\"message_stop\"}\n\n"))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewAnthropicClient(&conf.ModelProviderConfig{
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
	assert.Contains(t, joined, ">>> REQUEST POST ")
	assert.Contains(t, joined, "<<< RESPONSE 200")
	assert.Contains(t, joined, "<<< CHUNK event: message_start")
	assert.Contains(t, joined, `<<< CHUNK data: {"type":"content_block_delta"`)
	assert.NotContains(t, joined, "stream-secret-key")
}

func TestMarshalAnthropicMessagesRequest_PrefixStableForEquivalentInputs(t *testing.T) {
	requestOne := AnthropicMessagesRequest{
		Model: "claude-test",
		Messages: []AnthropicMessageParam{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{
						Type: "tool_use",
						ID:   "call_1",
						Name: "search",
						Input: map[string]interface{}{
							"query": "hello",
							"limit": 3,
						},
					},
				},
			},
		},
		MaxTokens: 1000,
		System:    "system prompt",
		Tools: []AnthropicTool{
			{
				Name:        "search",
				Description: "Search the web",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{"type": "string"},
						"limit": map[string]interface{}{"type": "integer"},
					},
					"required": []interface{}{"query", "limit"},
				},
			},
		},
	}

	requestTwo := AnthropicMessagesRequest{
		Model: "claude-test",
		Messages: []AnthropicMessageParam{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{
						Type: "tool_use",
						ID:   "call_1",
						Name: "search",
						Input: map[string]interface{}{
							"limit": 3,
							"query": "hello",
						},
					},
				},
			},
		},
		MaxTokens: 1000,
		System:    "system prompt",
		Tools: []AnthropicTool{
			{
				Name:        "search",
				Description: "Search the web",
				InputSchema: map[string]interface{}{
					"required": []interface{}{"query", "limit"},
					"properties": map[string]interface{}{
						"limit": map[string]interface{}{"type": "integer"},
						"query": map[string]interface{}{"type": "string"},
					},
					"type": "object",
				},
			},
		},
	}

	bodyOne, err := marshalAnthropicMessagesRequest(requestOne)
	require.NoError(t, err)
	bodyTwo, err := marshalAnthropicMessagesRequest(requestTwo)
	require.NoError(t, err)

	assert.Equal(t, string(bodyOne), string(bodyTwo))
}

func TestMarshalAnthropicMessagesRequest_AddsPromptCachingBreakpoints(t *testing.T) {
	t.Run("marks last tool when tools are present", func(t *testing.T) {
		request := AnthropicMessagesRequest{
			Model:     "claude-test",
			MaxTokens: 1000,
			Messages:  []AnthropicMessageParam{{Role: "user", Content: "hello"}},
			System:    "system prompt",
			Tools: []AnthropicTool{
				{Name: "first", InputSchema: map[string]interface{}{"type": "object"}},
				{Name: "second", InputSchema: map[string]interface{}{"type": "object"}},
			},
		}

		body, err := marshalAnthropicMessagesRequest(request)
		require.NoError(t, err)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &payload))

		toolsPayload, ok := payload["tools"].([]interface{})
		require.True(t, ok)
		require.Len(t, toolsPayload, 2)

		firstTool, ok := toolsPayload[0].(map[string]interface{})
		require.True(t, ok)
		_, hasFirstCacheControl := firstTool["cache_control"]
		assert.False(t, hasFirstCacheControl)

		secondTool, ok := toolsPayload[1].(map[string]interface{})
		require.True(t, ok)
		cacheControl, ok := secondTool["cache_control"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "ephemeral", cacheControl["type"])
	})

	t.Run("marks system when no tools are present", func(t *testing.T) {
		request := AnthropicMessagesRequest{
			Model:     "claude-test",
			MaxTokens: 1000,
			Messages:  []AnthropicMessageParam{{Role: "user", Content: "hello"}},
			System:    "system prompt",
		}

		body, err := marshalAnthropicMessagesRequest(request)
		require.NoError(t, err)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &payload))

		systemPayload, ok := payload["system"].([]interface{})
		require.True(t, ok)
		require.Len(t, systemPayload, 1)

		block, ok := systemPayload[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "text", block["type"])
		assert.Equal(t, "system prompt", block["text"])

		cacheControl, ok := block["cache_control"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "ephemeral", cacheControl["type"])
	})
}

func TestAnthropicClient_ChatAndChatStream_SendPromptCachingBetaHeader(t *testing.T) {
	tests := []struct {
		name      string
		streaming bool
	}{
		{name: "chat", streaming: false},
		{name: "chat_stream", streaming: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			receivedBeta := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedBeta = r.Header.Get("anthropic-beta")
				if tc.streaming {
					w.Header().Set("Content-Type", "text/event-stream")
					_, err := w.Write([]byte("event: message_delta\n"))
					require.NoError(t, err)
					_, err = w.Write([]byte("data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\n"))
					require.NoError(t, err)
					_, err = w.Write([]byte("event: message_stop\n"))
					require.NoError(t, err)
					_, err = w.Write([]byte("data: {\"type\":\"message_stop\"}\n\n"))
					require.NoError(t, err)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"id":"msg_raw","type":"message","role":"assistant","model":"test-model","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"output_tokens":5}}`))
				require.NoError(t, err)
			}))
			defer server.Close()

			client, err := NewAnthropicClient(&conf.ModelProviderConfig{URL: server.URL, APIKey: "test-key"})
			require.NoError(t, err)
			model := client.ChatModel("test-model", nil)

			if tc.streaming {
				for range model.ChatStream(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hello")}, nil, []tool.ToolInfo{
					{Name: "tool1", Description: "desc", Schema: tool.NewToolSchema()},
				}) {
				}
			} else {
				_, err = model.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hello")}, nil, []tool.ToolInfo{
					{Name: "tool1", Description: "desc", Schema: tool.NewToolSchema()},
				})
				require.NoError(t, err)
			}

			assert.Equal(t, anthropicPromptCachingBetaHeaderValue, receivedBeta)
		})
	}
}

func TestConvertAnthropicMessage_PreservesReasoningAcrossToolRoundtrip(t *testing.T) {
	t.Run("convertToAnthropicMessage includes thinking block with signature", func(t *testing.T) {
		msg := &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{
					ReasoningContent:   "internal reasoning",
					ReasoningSignature: "sig_123",
				},
				{
					ToolCall: &tool.ToolCall{
						ID:       "tool_1",
						Function: "vfsRead",
						Arguments: tool.NewToolValue(map[string]interface{}{"path": "pkg/models/anthropic_client.go"}),
					},
				},
			},
		}

		converted := convertToAnthropicMessage(msg)
		blocks, ok := converted.Content.([]AnthropicContentBlock)
		require.True(t, ok)
		require.Len(t, blocks, 2)

		assert.Equal(t, "assistant", converted.Role)
		assert.Equal(t, "thinking", blocks[0].Type)
		assert.Equal(t, "internal reasoning", blocks[0].Thinking)
		assert.Equal(t, "sig_123", blocks[0].Signature)
		assert.Equal(t, "tool_use", blocks[1].Type)
		assert.Equal(t, "tool_1", blocks[1].ID)
		assert.Equal(t, "vfsRead", blocks[1].Name)
	})

	t.Run("convertFromAnthropicResponse extracts thinking block with signature", func(t *testing.T) {
		msg := convertFromAnthropicResponse([]AnthropicResponseContent{
			{
				Type:      "thinking",
				Thinking:  "internal reasoning",
				Signature: "sig_123",
			},
			{
				Type: "tool_use",
				ID:   "tool_1",
				Name: "vfsRead",
				Input: map[string]interface{}{
					"path": "pkg/models/anthropic_client.go",
				},
			},
		})

		require.NotNil(t, msg)
		require.Len(t, msg.Parts, 2)
		assert.Equal(t, ChatRoleAssistant, msg.Role)
		assert.Equal(t, "internal reasoning", msg.Parts[0].ReasoningContent)
		assert.Equal(t, "sig_123", msg.Parts[0].ReasoningSignature)
		require.NotNil(t, msg.Parts[1].ToolCall)
		assert.Equal(t, "tool_1", msg.Parts[1].ToolCall.ID)
		assert.Equal(t, "vfsRead", msg.Parts[1].ToolCall.Function)
	})
}
