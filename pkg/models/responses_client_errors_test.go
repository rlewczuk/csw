package models

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type unexpectedEOFReadCloser struct{}

func (r *unexpectedEOFReadCloser) Read(p []byte) (int, error) {
	_ = p
	return 0, io.ErrUnexpectedEOF
}

func (r *unexpectedEOFReadCloser) Close() error {
	return nil
}

func TestFormatRawHTTPResponse(t *testing.T) {
	t.Run("formats status headers and body as multiline string", func(t *testing.T) {
		headers := http.Header{}
		headers.Add("X-Zeta", "z")
		headers.Add("X-Alpha", "a1")
		headers.Add("X-Alpha", "a2")

		raw := formatRawHTTPResponse(http.StatusBadRequest, headers, []byte("body-line"))

		assert.Equal(t, "400\nX-Alpha: a1\nX-Alpha: a2\nX-Zeta: z\n\nbody-line", raw)
	})

	t.Run("formats response with empty headers and empty body", func(t *testing.T) {
		raw := formatRawHTTPResponse(http.StatusOK, nil, nil)
		assert.Equal(t, "200\n\n", raw)
	})
}

func TestResponsesClient_ChatWrapsLLMRequestError(t *testing.T) {
	t.Run("wraps codex stream conversion error with raw response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, writeErr := w.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
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
		_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}, nil, nil)
		require.Error(t, err)

		var llmErr *LLMRequestError
		require.True(t, errors.As(err, &llmErr))
		assert.Contains(t, err.Error(), "convertFromResponsesStreamBody() [responses_client_response_conversion.go]: no usable output items in response")
		assert.Contains(t, llmErr.RawResponse, "200\n")
		assert.Contains(t, llmErr.RawResponse, "Content-Type: text/event-stream")
		assert.Contains(t, llmErr.RawResponse, "\n\n")
		assert.Contains(t, llmErr.RawResponse, "response.completed")
	})

	t.Run("wraps network request failure with empty raw response", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return nil, io.EOF
			}),
		}

		client, err := NewResponsesClientWithHTTPClient("https://example.test", httpClient)
		require.NoError(t, err)

		chatModel := client.ChatModel("test-model", nil)
		_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}, nil, nil)
		require.Error(t, err)

		var llmErr *LLMRequestError
		require.True(t, errors.As(err, &llmErr))
		assert.Empty(t, llmErr.RawResponse)
	})

	t.Run("wraps non-stream decode error with raw response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte("not-json"))
			require.NoError(t, writeErr)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL,
			APIKey: "test-key",
		})
		require.NoError(t, err)

		chatModel := client.ChatModel("test-model", nil)
		_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}, nil, nil)
		require.Error(t, err)

		var llmErr *LLMRequestError
		require.True(t, errors.As(err, &llmErr))
		assert.Contains(t, err.Error(), "failed to decode response")
		assert.Contains(t, llmErr.RawResponse, "200\n")
		assert.Contains(t, llmErr.RawResponse, "Content-Type: application/json")
		assert.True(t, strings.HasSuffix(llmErr.RawResponse, "not-json"))
	})

	t.Run("wraps response body unexpected EOF as retryable network error", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       &unexpectedEOFReadCloser{},
					Request:    req,
				}, nil
			}),
		}

		client, err := NewResponsesClientWithHTTPClient("https://example.test", httpClient)
		require.NoError(t, err)

		chatModel := client.ChatModel("test-model", nil)
		_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}, nil, nil)
		require.Error(t, err)

		var networkErr *NetworkError
		require.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
		assert.Contains(t, networkErr.Message, "unexpected stream EOF")
		assert.Contains(t, err.Error(), "failed to read response body")
	})
}

func TestResponsesClient_RateLimitError(t *testing.T) {
	t.Run("returns codex primary reset seconds for usage limit reached", func(t *testing.T) {
		tc := getResponsesTestClient(t)
		defer tc.Close()

		if tc.Mock == nil {
			t.Skip("Skipping codex usage-limit header assertions against real provider")
		}

		headers := http.Header{}
		headers.Set("X-Codex-Primary-Used-Percent", "100")
		headers.Set("X-Codex-Primary-Reset-After-Seconds", "4348")
		headers.Set("X-Codex-Secondary-Used-Percent", "77")
		headers.Set("X-Codex-Secondary-Reset-After-Seconds", "68891")
		tc.Mock.AddRestResponseWithStatusAndHeaders(
			"/responses",
			"POST",
			`{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached","resets_in_seconds":4347}}`,
			http.StatusTooManyRequests,
			headers,
		)

		chatModel := tc.Client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err := chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 4348, rateLimitErr.RetryAfterSeconds)
		assert.Equal(t, "The usage limit has been reached", rateLimitErr.Message)
	})

	t.Run("returns codex secondary reset seconds when secondary limit is exceeded", func(t *testing.T) {
		tc := getResponsesTestClient(t)
		defer tc.Close()

		if tc.Mock == nil {
			t.Skip("Skipping codex usage-limit header assertions against real provider")
		}

		headers := http.Header{}
		headers.Set("X-Codex-Primary-Used-Percent", "95")
		headers.Set("X-Codex-Primary-Reset-After-Seconds", "12")
		headers.Set("X-Codex-Secondary-Used-Percent", "100")
		headers.Set("X-Codex-Secondary-Reset-After-Seconds", "68891")
		tc.Mock.AddRestResponseWithStatusAndHeaders(
			"/responses",
			"POST",
			`{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached","resets_in_seconds":68890}}`,
			http.StatusTooManyRequests,
			headers,
		)

		chatModel := tc.Client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err := chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 68891, rateLimitErr.RetryAfterSeconds)
	})

	t.Run("returns rate limit error with retry-after header", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		headers := http.Header{}
		headers.Set("Retry-After", "60")
		mock.AddRestResponseWithStatusAndHeaders("/responses", "POST", `{"error":{"message":"Rate limit exceeded","type":"rate_limit_error"}}`, http.StatusTooManyRequests, headers)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		assert.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 60, rateLimitErr.RetryAfterSeconds)
		assert.Contains(t, rateLimitErr.Error(), "Rate limit exceeded")
	})

	t.Run("returns rate limit error without retry-after header", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		mock.AddRestResponseWithStatus("/responses", "POST", `rate limit exceeded`, http.StatusTooManyRequests)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		assert.True(t, errors.As(err, &rateLimitErr))
		assert.Equal(t, 0, rateLimitErr.RetryAfterSeconds)
	})

	t.Run("wraps underlying error with ErrRateExceeded", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		mock.AddRestResponseWithStatus("/responses", "POST", "rate limit exceeded", http.StatusTooManyRequests)

		chatModel := client.ChatModel("test-model", nil)
		messages := []*ChatMessage{NewTextMessage(ChatRoleUser, "Hello")}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrRateExceeded))
	})
}
