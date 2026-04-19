package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
