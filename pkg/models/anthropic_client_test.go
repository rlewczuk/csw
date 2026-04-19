package models

import (
	"context"
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

func TestAnthropicClient_Chat_RecognizesKimiTokenLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Invalid request: Your request exceeded model token limit: 262144 (requested: 269477)"}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewAnthropicClient(&conf.ModelProviderConfig{
		URL:    server.URL,
		APIKey: "test-key",
	})
	require.NoError(t, err)

	chatModel := client.ChatModel("kimi-test", nil)
	_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hello")}, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTooManyInputTokens)
	assert.Contains(t, err.Error(), "exceeded model token limit")
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
