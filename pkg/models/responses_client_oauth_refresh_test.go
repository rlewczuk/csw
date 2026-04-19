package models

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer

	fn()

	require.NoError(t, writer.Close())
	os.Stdout = originalStdout

	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	return string(output)
}

func TestResponsesClient_OAuthTokenRenewal(t *testing.T) {
	t.Run("RefreshTokenIfNeeded returns nil for non-OAuth2 provider", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:      mock.URL(),
			APIKey:   "test-key",
			AuthMode: conf.AuthModeAPIKey,
		})
		require.NoError(t, err)

		err = client.RefreshTokenIfNeeded()
		assert.NoError(t, err)
	})

	t.Run("RefreshTokenIfNeeded refreshes expired token", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		// Mock the token refresh endpoint
		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		err = client.RefreshTokenIfNeeded()
		require.NoError(t, err)

		// Verify token was updated
		token, err := client.GetAccessToken()
		require.NoError(t, err)
		assert.Equal(t, "new-access-token", token)
		assert.Equal(t, "new-refresh-token", client.GetConfig().RefreshToken)
	})

	t.Run("RefreshTokenIfNeeded does not refresh valid token", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as valid for a long time
		client.tokenExpiry = time.Now().Add(1 * time.Hour)

		err = client.RefreshTokenIfNeeded()
		require.NoError(t, err)

		// No request should have been made
		reqs := mock.GetRequests()
		assert.Empty(t, reqs)
	})

	t.Run("RefreshTokenIfNeeded returns error when refresh fails", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		// Mock the token refresh endpoint with error
		mock.AddRestResponseWithStatus("/oauth/token", "POST", `{"error":"invalid_grant"}`, http.StatusBadRequest)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "invalid-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		err = client.RefreshTokenIfNeeded()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token renewal failed")
	})

	t.Run("GetAccessToken returns current token for non-OAuth2", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "static-api-key",
		})
		require.NoError(t, err)

		token, err := client.GetAccessToken()
		require.NoError(t, err)
		assert.Equal(t, "static-api-key", token)
	})

	t.Run("GetAccessToken refreshes token when expired", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"refreshed-token","expires_in":3600}`)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		token, err := client.GetAccessToken()
		require.NoError(t, err)
		assert.Equal(t, "refreshed-token", token)
	})

	t.Run("RefreshTokenIfNeeded persists config via ConfigUpdater", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		// Mock the token refresh endpoint
		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			Name:         "test-provider",
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		// Track if config updater was called
		var updaterCalled bool
		var savedConfig *conf.ModelProviderConfig
		client.SetConfigUpdater(func(config *conf.ModelProviderConfig) error {
			updaterCalled = true
			savedConfig = config
			return nil
		})

		err = client.RefreshTokenIfNeeded()
		require.NoError(t, err)

		// Verify config updater was called with updated config
		assert.True(t, updaterCalled)
		require.NotNil(t, savedConfig)
		assert.Equal(t, "new-access-token", savedConfig.APIKey)
		assert.Equal(t, "new-refresh-token", savedConfig.RefreshToken)
		assert.Equal(t, "test-provider", savedConfig.Name)
	})

	t.Run("RefreshTokenIfNeeded persists access token when refresh token missing", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		// Mock the token refresh endpoint (no refresh_token in response)
		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","expires_in":3600}`)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			Name:         "test-provider",
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		// Track if config updater was called
		var updaterCalled bool
		client.SetConfigUpdater(func(config *conf.ModelProviderConfig) error {
			updaterCalled = true
			return nil
		})

		err = client.RefreshTokenIfNeeded()
		require.NoError(t, err)

		// Config updater should be called to persist new access token
		assert.True(t, updaterCalled)
	})

	t.Run("RefreshTokenIfNeeded continues when ConfigUpdater fails", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		// Mock the token refresh endpoint
		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`)

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			Name:         "test-provider",
			URL:          mock.URL(),
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Set token as expired
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		// Set a failing config updater
		var updaterCalled bool
		client.SetConfigUpdater(func(config *conf.ModelProviderConfig) error {
			updaterCalled = true
			return assert.AnError
		})

		err = client.RefreshTokenIfNeeded()
		// Token refresh should still succeed even if config persistence fails
		require.NoError(t, err)

		// Verify config updater was called
		assert.True(t, updaterCalled)

		// Verify in-memory config was still updated
		token, err := client.GetAccessToken()
		require.NoError(t, err)
		assert.Equal(t, "new-access-token", token)
		assert.Equal(t, "new-access-token", client.GetConfig().APIKey)
		assert.Equal(t, "new-refresh-token", client.GetConfig().RefreshToken)
	})

	t.Run("RefreshTokenIfNeeded does not eagerly renew opaque non-expiring token", func(t *testing.T) {
		var refreshCalls int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/oauth/token" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			atomic.AddInt32(&refreshCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"access_token":"should-not-be-used","expires_in":3600}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          server.URL,
			APIKey:       "opaque-token",
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     server.URL + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Unknown expiry (zero) with existing token should not cause eager refresh.
		client.tokenExpiry = time.Time{}

		err = client.RefreshTokenIfNeeded()
		require.NoError(t, err)

		assert.Equal(t, int32(0), atomic.LoadInt32(&refreshCalls))
	})

	t.Run("RefreshTokenIfNeeded skips refresh when disable-refresh is enabled without debug output", func(t *testing.T) {
		var refreshCalls int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/oauth/token" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			atomic.AddInt32(&refreshCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"access_token":"new-token","expires_in":3600}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			Name:           "responses-test",
			URL:            server.URL,
			APIKey:         "old-token",
			AuthMode:       conf.AuthModeOAuth2,
			TokenURL:       server.URL + "/oauth/token",
			ClientID:       "test-client-id",
			RefreshToken:   "test-refresh-token",
			DisableRefresh: true,
		})
		require.NoError(t, err)
		client.tokenExpiry = time.Now().Add(-1 * time.Hour)

		stdout := captureStdout(t, func() {
			err = client.RefreshTokenIfNeeded()
		})
		require.NoError(t, err)

		assert.Equal(t, int32(0), atomic.LoadInt32(&refreshCalls))
		assert.Empty(t, stdout)
	})

	t.Run("RefreshTokenIfNeeded skips refresh for valid token without debug output", func(t *testing.T) {
		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			Name:         "responses-test",
			URL:          defaultResponsesTestURL,
			APIKey:       "token",
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     defaultResponsesTestURL + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)
		client.tokenExpiry = time.Now().Add(10 * time.Minute)

		stdout := captureStdout(t, func() {
			err = client.RefreshTokenIfNeeded()
		})
		require.NoError(t, err)

		assert.Empty(t, stdout)
	})

	t.Run("Chat retries on 401 expired token and refreshes once", func(t *testing.T) {
		var tokenRefreshCalls int32
		var responsesCalls int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/oauth/token":
				atomic.AddInt32(&tokenRefreshCalls, 1)
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`))
				require.NoError(t, err)
			case "/responses":
				atomic.AddInt32(&responsesCalls, 1)
				authHeader := r.Header.Get("Authorization")
				if authHeader == "Bearer old-access-token" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Header().Set("Content-Type", "application/json")
					_, err := w.Write([]byte(`{"error":{"message":"access token expired","code":"invalid_token"}}`))
					require.NoError(t, err)
					return
				}

				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"id":"resp_ok","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          server.URL,
			APIKey:       "old-access-token",
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     server.URL + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)

		// Token appears fresh locally, refresh should happen only after expired-token 401.
		client.tokenExpiry = time.Now().Add(1 * time.Hour)

		chatModel := client.ChatModel("test-model", nil)
		resp, err := chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "ok", resp.GetText())
		assert.Equal(t, int32(1), atomic.LoadInt32(&tokenRefreshCalls))
		assert.Equal(t, int32(2), atomic.LoadInt32(&responsesCalls))
	})

	t.Run("parallel 401 retries perform only one token renewal", func(t *testing.T) {
		var tokenRefreshCalls int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/oauth/token":
				atomic.AddInt32(&tokenRefreshCalls, 1)
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"access_token":"shared-new-token","refresh_token":"shared-new-refresh","expires_in":3600}`))
				require.NoError(t, err)
			case "/responses":
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer shared-new-token" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_, err := w.Write([]byte(`{"error":{"message":"token expired","code":"invalid_token"}}`))
					require.NoError(t, err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"id":"resp_parallel","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:          server.URL,
			APIKey:       "stale-token",
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     server.URL + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		})
		require.NoError(t, err)
		client.tokenExpiry = time.Now().Add(1 * time.Hour)

		chatModel := client.ChatModel("test-model", nil)
		const workers = 6
		var wg sync.WaitGroup
		errCh := make(chan error, workers)

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, chatErr := chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, nil)
				errCh <- chatErr
			}()
		}

		wg.Wait()
		close(errCh)

		for chatErr := range errCh {
			require.NoError(t, chatErr)
		}

		assert.Equal(t, int32(1), atomic.LoadInt32(&tokenRefreshCalls))
	})

	t.Run("Chat does not retry token refresh on 401 when disable-refresh is enabled", func(t *testing.T) {
		var tokenRefreshCalls int32
		var responsesCalls int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/oauth/token":
				atomic.AddInt32(&tokenRefreshCalls, 1)
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"access_token":"new-access-token","expires_in":3600}`))
				require.NoError(t, err)
			case "/responses":
				atomic.AddInt32(&responsesCalls, 1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, err := w.Write([]byte(`{"error":{"message":"access token expired","code":"invalid_token"}}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			Name:           "responses-test",
			URL:            server.URL,
			APIKey:         "old-access-token",
			AuthMode:       conf.AuthModeOAuth2,
			TokenURL:       server.URL + "/oauth/token",
			ClientID:       "test-client-id",
			RefreshToken:   "test-refresh-token",
			DisableRefresh: true,
		})
		require.NoError(t, err)
		client.tokenExpiry = time.Now().Add(1 * time.Hour)

		chatModel := client.ChatModel("test-model", nil)
		_, err = chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, nil)
		require.Error(t, err)

		assert.Equal(t, int32(0), atomic.LoadInt32(&tokenRefreshCalls))
		assert.Equal(t, int32(1), atomic.LoadInt32(&responsesCalls))
	})

	t.Run("SetConfigUpdater can be called multiple times", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    mock.URL(),
			APIKey: "test-key",
		})
		require.NoError(t, err)

		// Set first updater
		var callCount int
		client.SetConfigUpdater(func(config *conf.ModelProviderConfig) error {
			callCount++
			return nil
		})

		// Replace with second updater
		client.SetConfigUpdater(func(config *conf.ModelProviderConfig) error {
			callCount += 10
			return nil
		})

		// Verify second updater is used (for non-OAuth2 this won't be called)
		// This test just verifies the setter works
		assert.NotNil(t, client.configUpdater)
	})
}
