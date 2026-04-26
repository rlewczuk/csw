package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
