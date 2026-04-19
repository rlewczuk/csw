package models

import (
	"encoding/json"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesClient_Compactor(t *testing.T) {
	t.Run("Compactor returns responses compactor implementation", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("test-model", nil)
		compactor := chatModel.Compactor()
		require.NotNil(t, compactor)
		_, ok := compactor.(*ResponsesChatCompactor)
		assert.True(t, ok)
	})

	t.Run("CompactMessages calls compact endpoint and returns compacted messages", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		mock.AddRestResponse("/v1/responses/compact", "POST", `{"id":"resp_cmp","object":"response.compaction","output":[{"id":"msg_1","type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]},{"id":"cmp_1","type":"compaction","encrypted_content":"enc_data"}]}`)

		chatModel := client.ChatModel("test-model", nil)
		compactor := chatModel.Compactor()
		require.NotNil(t, compactor)

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Hello"),
			NewTextMessage(ChatRoleAssistant, "This is a longer response that can be compacted"),
		}

		compacted, err := compactor.CompactMessages(messages)
		require.NoError(t, err)
		require.Len(t, compacted, 2)
		assert.Equal(t, ChatRoleUser, compacted[0].Role)
		assert.Equal(t, "Hello", compacted[0].GetText())
		assert.Equal(t, ChatRoleAssistant, compacted[1].Role)
		assert.Equal(t, "enc_data", compacted[1].GetText())

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)
		assert.Equal(t, "/v1/responses/compact", reqs[0].Path)

		var compactReq ResponsesCreateRequest
		require.NoError(t, json.Unmarshal(reqs[0].Body, &compactReq))
		assert.Equal(t, "test-model", compactReq.Model)
		require.Len(t, compactReq.Input, 2)
	})
}

func TestBuildResponsesCompactURL(t *testing.T) {
	t.Run("adds v1 prefix when missing", func(t *testing.T) {
		url := buildResponsesCompactURL("https://api.openai.com")
		assert.Equal(t, "https://api.openai.com/v1/responses/compact", url)
	})

	t.Run("keeps existing v1 suffix", func(t *testing.T) {
		url := buildResponsesCompactURL("https://api.openai.com/v1")
		assert.Equal(t, "https://api.openai.com/v1/responses/compact", url)
	})

	t.Run("trims whitespace in base url", func(t *testing.T) {
		url := buildResponsesCompactURL("  https://api.openai.com/v1  ")
		assert.Equal(t, "https://api.openai.com/v1/responses/compact", url)
	})
}
