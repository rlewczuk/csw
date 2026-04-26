package models

import (
	"context"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIClient_ChatModelStream(t *testing.T) {
	tc := getOpenAITestClient(t)
	defer tc.Close()

	ctx := context.Background()

	t.Run("streams chat message and gets fragments", func(t *testing.T) {
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-stream-1","object":"chat.completion.chunk","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","content":"1"}}]}`,
				`data: {"id":"chatcmpl-stream-1","object":"chat.completion.chunk","created":1640000001,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"content":"\n2\n3"}}]}`,
				`data: {"id":"chatcmpl-stream-1","object":"chat.completion.chunk","created":1640000002,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
				"data: [DONE]",
			)
		}

		options := &ChatOptions{Temperature: 0.7, TopP: 0.9, TopK: 40}
		chatModel := tc.Client.ChatModel(testOpenAIModelName, options)
		messages := []*ChatMessage{{Role: ChatRoleUser, Parts: []ChatMessagePart{{Text: "Count from 1 to 5, one number per line."}}}}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			assert.Equal(t, ChatRoleAssistant, fragment.Role)
			assert.NotEmpty(t, fragment.Parts)
			fragments = append(fragments, fragment)
		}

		assert.Greater(t, len(fragments), 0, "expected to receive at least one fragment")
	})

	t.Run("handles context cancellation during streaming", func(t *testing.T) {
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-stream-2","object":"chat.completion.chunk","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","content":"Once upon a time"}}]}`,
				"data: [DONE]",
			)
		}

		ctxWithCancel, cancel := context.WithCancel(ctx)
		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)
		messages := []*ChatMessage{{Role: ChatRoleUser, Parts: []ChatMessagePart{{Text: "Write a long story about a cat."}}}}

		iterator := chatModel.ChatStream(ctxWithCancel, messages, nil, nil)
		require.NotNil(t, iterator)

		fragmentReceived := false
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			fragmentReceived = true
			cancel()
			break
		}

		assert.True(t, fragmentReceived, "expected to receive at least one fragment before cancellation")
	})

	t.Run("handles context with timeout during streaming", func(t *testing.T) {
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-stream-3","object":"chat.completion.chunk","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello!"}}]}`,
				`data: {"id":"chatcmpl-stream-3","object":"chat.completion.chunk","created":1640000001,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
				"data: [DONE]",
			)
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)
		messages := []*ChatMessage{{Role: ChatRoleUser, Parts: []ChatMessagePart{{Text: "Say hello"}}}}

		iterator := chatModel.ChatStream(ctxWithTimeout, messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Greater(t, len(fragments), 0)
	})

	t.Run("returns no fragments for empty messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)
		iterator := chatModel.ChatStream(ctx, []*ChatMessage{}, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("returns no fragments for nil messages", func(t *testing.T) {
		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)
		iterator := chatModel.ChatStream(ctx, nil, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("iterator can be stopped early", func(t *testing.T) {
		if tc.Mock != nil {
			tc.Mock.AddStreamingResponse("/chat/completions", "POST", true,
				`data: {"id":"chatcmpl-stream-4","object":"chat.completion.chunk","created":1640000000,"model":"devstral-small-2:latest","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello!"}}]}`,
				"data: [DONE]",
			)
		}

		chatModel := tc.Client.ChatModel(testOpenAIModelName, nil)
		messages := []*ChatMessage{{Role: ChatRoleUser, Parts: []ChatMessagePart{{Text: "Say hello"}}}}

		iterator := chatModel.ChatStream(ctx, messages, nil, nil)
		require.NotNil(t, iterator)

		fragmentReceived := false
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			fragmentReceived = true
			break
		}

		assert.True(t, fragmentReceived, "expected to receive at least one fragment")
	})
}

func TestOpenAIClient_CustomHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOpenAIClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "test-key",
		Headers: map[string]string{
			"X-Stream-Header": "stream-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/chat/completions", "POST", true,
		`data: {"id":"chatcmpl-stream-custom","object":"chat.completion.chunk","created":1640000000,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}`,
		`data: {"id":"chatcmpl-stream-custom","object":"chat.completion.chunk","created":1640000001,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"data: [DONE]",
	)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hi")}

	iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
	for range iterator {
	}

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "Bearer test-key", request.Header.Get("Authorization"))
	assert.Equal(t, "stream-value", request.Header.Get("X-Stream-Header"))
}

func TestOpenAIClient_OptionsHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOpenAIClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "config-api-key",
		Headers: map[string]string{
			"X-Config-Header": "config-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/chat/completions", "POST", true,
		`data: {"id":"chatcmpl-opts-stream","object":"chat.completion.chunk","created":1640000000,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}`,
		`data: {"id":"chatcmpl-opts-stream","object":"chat.completion.chunk","created":1640000001,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"data: [DONE]",
	)

	options := &ChatOptions{Headers: map[string]string{
		"X-Options-Header": "options-value",
		"X-Api-Key":        "should-not-override",
	}}

	chatModel := client.ChatModel("test-model", options)
	messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hi")}

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
