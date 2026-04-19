package models

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIClient_ErrorHandling(t *testing.T) {
	t.Run("handles endpoint not found", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponseWithStatus("/models", "GET", `{"error":{"message":"not found"}}`, 404)

		client, err := NewOpenAIClientWithHTTPClient(mock.URL(), mock.Client())
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.ErrorIs(t, err, ErrEndpointNotFound)
	})

	t.Run("handles endpoint unavailable", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponseWithStatus("/models", "GET", `{"error":{"message":"unavailable"}}`, 503)

		client, err := NewOpenAIClientWithHTTPClient(mock.URL(), mock.Client())
		require.NoError(t, err)

		_, err = client.ListModels()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEndpointUnavailable)
	})
}

func TestOpenAIClient_RateLimitError(t *testing.T) {
	t.Run("returns rate limit error with retry-after header", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		headers := http.Header{}
		headers.Set("Retry-After", "60")
		mock.AddRestResponseWithStatusAndHeaders("/chat/completions", "POST", `{"error":{"message":"Rate limit exceeded","type":"rate_limit_error"}}`, http.StatusTooManyRequests, headers)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		assert.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 60, rateLimitErr.RetryAfterSeconds)
		assert.Contains(t, rateLimitErr.Error(), "Rate limit exceeded")
	})

	t.Run("returns usage-limit retry after parsed from reset at timestamp", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		resetAt := time.Now().Add(95 * time.Second).Format("2006-01-02 15:04:05")
		body := `{"error":{"code":"1308","message":"Usage limit reached for 5 hour. Your limit will reset at ` + resetAt + `"}}`
		mock.AddRestResponseWithStatus("/chat/completions", "POST", body, http.StatusTooManyRequests)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.True(t, errors.As(err, &rateLimitErr))
		assert.GreaterOrEqual(t, rateLimitErr.RetryAfterSeconds, 90)
		assert.LessOrEqual(t, rateLimitErr.RetryAfterSeconds, 100)
		assert.Contains(t, rateLimitErr.Message, "Usage limit reached")
	})

	t.Run("keeps larger retry-after when usage-limit parsed value is smaller", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		resetAt := time.Now().Add(30 * time.Second).Format("2006-01-02 15:04:05")
		body := `{"error":{"code":"1308","message":"Usage limit reached. Your limit will reset at ` + resetAt + `"}}`
		headers := http.Header{}
		headers.Set("Retry-After", "120")
		mock.AddRestResponseWithStatusAndHeaders("/chat/completions", "POST", body, http.StatusTooManyRequests, headers)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 120, rateLimitErr.RetryAfterSeconds)
	})

	t.Run("fallbacks to plain body when error payload is not json", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		mock.AddRestResponseWithStatus("/chat/completions", "POST", "rate limited", http.StatusTooManyRequests)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, "rate limited", rateLimitErr.Message)
		assert.Equal(t, 0, rateLimitErr.RetryAfterSeconds)
		assert.True(t, errors.Is(err, ErrRateExceeded))
	})
}
