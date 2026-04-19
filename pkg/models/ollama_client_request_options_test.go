package models

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaClient_MaxTokens(t *testing.T) {
	t.Run("Chat method uses MaxTokens as num_predict", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 2048,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Hello"}},
			},
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		// Verify num_predict was set in the request
		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OllamaChatRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		require.NotNil(t, chatReq.Options)
		assert.Equal(t, 2048, chatReq.Options.NumPredict)
	})

	t.Run("ChatStream method uses MaxTokens as num_predict", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 4096,
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"test-model","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
		)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Hello"}},
			},
		}

		iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
		for range iterator {
		}

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OllamaChatRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		require.NotNil(t, chatReq.Options)
		assert.Equal(t, 4096, chatReq.Options.NumPredict)
	})

	t.Run("Chat method uses default num_predict when MaxTokens is zero", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 0,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Hello"}},
			},
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OllamaChatRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		require.NotNil(t, chatReq.Options)
		assert.Equal(t, DefaultMaxTokens, chatReq.Options.NumPredict)
	})

	t.Run("Chat method sets num_predict with options when MaxTokens is set", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOllamaClient(&conf.ModelProviderConfig{
			URL:       mock.URL(),
			MaxTokens: 1024,
		})
		require.NoError(t, err)

		mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

		options := &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}
		chatModel := client.ChatModel("test-model", options)
		messages := []*ChatMessage{
			{
				Role:  ChatRoleUser,
				Parts: []ChatMessagePart{{Text: "Hello"}},
			},
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OllamaChatRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)
		require.NotNil(t, chatReq.Options)
		assert.InDelta(t, 0.7, chatReq.Options.Temperature, 0.01)
		assert.InDelta(t, 0.9, chatReq.Options.TopP, 0.01)
		assert.Equal(t, 40, chatReq.Options.TopK)
		assert.Equal(t, 1024, chatReq.Options.NumPredict)
	})
}

func TestOllamaClient_CustomHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Request-ID":    "req-123",
			"X-Organization":  "my-org",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hello"),
	}

	_, err = chatModel.Chat(context.Background(), messages, nil, nil)
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "custom-value", request.Header.Get("X-Custom-Header"))
	assert.Equal(t, "req-123", request.Header.Get("X-Request-ID"))
	assert.Equal(t, "my-org", request.Header.Get("X-Organization"))
}

func TestOllamaClient_CustomHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		Headers: map[string]string{
			"X-Stream-Header": "stream-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":true,"done_reason":"stop"}`,
	)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hi"),
	}

	iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
	for range iterator {
	}

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Equal(t, "stream-value", request.Header.Get("X-Stream-Header"))
}

func TestOllamaClient_OptionsHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		Headers: map[string]string{
			"X-Config-Header": "config-value",
			"X-Shared-Header": "config-shared",
			"X-Custom-Auth":   "should-be-overridden",
			"Authorization":   "Bearer config-auth",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

	options := &ChatOptions{
		Headers: map[string]string{
			"X-Options-Header": "options-value",
			"X-Shared-Header":  "options-shared",
			"X-Custom-Auth":    "new-value",
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
	assert.Equal(t, "new-value", request.Header.Get("X-Custom-Auth"), "non-auth headers can be overridden")
	assert.Equal(t, "Bearer config-auth", request.Header.Get("Authorization"), "authorization header should NOT be overridden by options")
}

func TestOllamaClient_OptionsHeadersStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		Headers: map[string]string{
			"X-Config-Header": "config-value",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":true,"done_reason":"stop"}`,
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

func TestOllamaClient_QueryParams(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		QueryParams: map[string]string{
			"api-version": "2024-01-01",
			"deployment":  "my-deployment",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/tags", "GET", `{"models":[{"name":"test-model","model":"test-model","modified_at":"2024-01-01T00:00:00Z","size":1000000000,"details":{"family":"llama"}}]}`)

	_, err = client.ListModels()
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Contains(t, request.Query, "api-version=2024-01-01")
	assert.Contains(t, request.Query, "deployment=my-deployment")
}

func TestOllamaClient_QueryParamsChat(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		QueryParams: map[string]string{
			"api-version": "2024-01-01",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/chat", "POST", `{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop"}`)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hello"),
	}

	_, err = chatModel.Chat(context.Background(), messages, nil, nil)
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Contains(t, request.Query, "api-version=2024-01-01")
}

func TestOllamaClient_QueryParamsChatStream(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		QueryParams: map[string]string{
			"stream-format": "json",
		},
	})
	require.NoError(t, err)

	mock.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":true,"done_reason":"stop"}`,
	)

	chatModel := client.ChatModel("test-model", nil)
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "Hi"),
	}

	iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
	for range iterator {
	}

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Contains(t, request.Query, "stream-format=json")
}

func TestOllamaClient_QueryParamsDoesNotOverrideExisting(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		QueryParams: map[string]string{
			"existing-param": "from-config",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/tags", "GET", `{"models":[]}`)

	_, err = client.ListModels()
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Contains(t, request.Query, "existing-param=from-config")
}

func TestOllamaClient_QueryParamsEmptyOrNil(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL:         mock.URL(),
		QueryParams: nil,
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/tags", "GET", `{"models":[]}`)

	_, err = client.ListModels()
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Empty(t, request.Query)
}

func TestOllamaClient_QueryParamsEmptyValues(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOllamaClient(&conf.ModelProviderConfig{
		URL: mock.URL(),
		QueryParams: map[string]string{
			"":          "value-with-empty-key",
			"valid-key": "",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/api/tags", "GET", `{"models":[]}`)

	_, err = client.ListModels()
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	request := reqs[0]

	assert.Empty(t, request.Query, "empty keys and values should not be added to query")
}
