package models

import (
	"net/http"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenewToken(t *testing.T) {
	t.Run("successfully renews token", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","refresh_token":"new-refresh-token","token_type":"Bearer","expires_in":3600,"scope":"openid"}`)

		config := &conf.ModelProviderConfig{
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		}

		resp, err := RenewToken(config, mock.Client())
		require.NoError(t, err)
		assert.Equal(t, "new-access-token", resp.AccessToken)
		assert.Equal(t, "new-refresh-token", resp.RefreshToken)
		assert.Equal(t, 3600, resp.ExpiresIn)
		assert.Equal(t, "Bearer", resp.TokenType)

		// Verify request was captured
		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)
		assert.Equal(t, "POST", reqs[0].Method)
		assert.Equal(t, "/oauth/token", reqs[0].Path)
		assert.Contains(t, string(reqs[0].Body), "grant_type=refresh_token")
		assert.Contains(t, string(reqs[0].Body), "refresh_token=test-refresh-token")
		assert.Contains(t, string(reqs[0].Body), "client_id=test-client-id")
	})

	t.Run("sends client secret when provided", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-token","expires_in":3600}`)

		config := &conf.ModelProviderConfig{
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RefreshToken: "test-refresh-token",
		}

		_, err := RenewToken(config, mock.Client())
		require.NoError(t, err)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)
		assert.Contains(t, string(reqs[0].Body), "client_secret=test-client-secret")
	})

	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := RenewToken(nil, http.DefaultClient)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("returns error for missing TokenURL", func(t *testing.T) {
		config := &conf.ModelProviderConfig{
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		}

		_, err := RenewToken(config, http.DefaultClient)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TokenURL is required")
	})

	t.Run("returns error for missing ClientID", func(t *testing.T) {
		config := &conf.ModelProviderConfig{
			TokenURL:     "https://example.com/token",
			RefreshToken: "test-refresh-token",
		}

		_, err := RenewToken(config, http.DefaultClient)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ClientID is required")
	})

	t.Run("returns error for missing RefreshToken", func(t *testing.T) {
		config := &conf.ModelProviderConfig{
			TokenURL: "https://example.com/token",
			ClientID: "test-client-id",
		}

		_, err := RenewToken(config, http.DefaultClient)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "RefreshToken is required")
	})

	t.Run("returns error for non-OK status", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponseWithStatus("/oauth/token", "POST", `{"error":"invalid_grant"}`, http.StatusBadRequest)

		config := &conf.ModelProviderConfig{
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "invalid-refresh-token",
		}

		_, err := RenewToken(config, mock.Client())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token renewal failed with status 400")
	})

	t.Run("returns error for response missing access_token", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponse("/oauth/token", "POST", `{"token_type":"Bearer","expires_in":3600}`)

		config := &conf.ModelProviderConfig{
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		}

		_, err := RenewToken(config, mock.Client())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "response missing access_token")
	})

	t.Run("returns error for invalid JSON response", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponse("/oauth/token", "POST", `not valid json`)

		config := &conf.ModelProviderConfig{
			TokenURL:     mock.URL() + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "test-refresh-token",
		}

		_, err := RenewToken(config, mock.Client())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse response")
	})
}

func TestCalculateTokenExpiry(t *testing.T) {
	t.Run("calculates expiry from positive expires_in", func(t *testing.T) {
		expiry := CalculateTokenExpiry(3600)
		assert.False(t, expiry.IsZero())
		assert.True(t, expiry.After(time.Now()))
		assert.True(t, expiry.Before(time.Now().Add(2*time.Hour)))
	})

	t.Run("returns zero time for zero expires_in", func(t *testing.T) {
		expiry := CalculateTokenExpiry(0)
		assert.True(t, expiry.IsZero())
	})

	t.Run("returns zero time for negative expires_in", func(t *testing.T) {
		expiry := CalculateTokenExpiry(-1)
		assert.True(t, expiry.IsZero())
	})
}

func TestIsTokenExpired(t *testing.T) {
	t.Run("returns true for zero time", func(t *testing.T) {
		assert.True(t, IsTokenExpired(time.Time{}, 0))
	})

	t.Run("returns true for past time", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		assert.True(t, IsTokenExpired(past, 0))
	})

	t.Run("returns true when within safety margin", func(t *testing.T) {
		soon := time.Now().Add(30 * time.Second)
		assert.True(t, IsTokenExpired(soon, time.Minute))
	})

	t.Run("returns false for future time outside safety margin", func(t *testing.T) {
		future := time.Now().Add(5 * time.Minute)
		assert.False(t, IsTokenExpired(future, time.Minute))
	})

	t.Run("returns false for future time with zero safety margin", func(t *testing.T) {
		future := time.Now().Add(1 * time.Second)
		assert.False(t, IsTokenExpired(future, 0))
	})
}

func TestIsOAuth2Provider(t *testing.T) {
	t.Run("returns true for oauth2 auth mode", func(t *testing.T) {
		config := &conf.ModelProviderConfig{AuthMode: conf.AuthModeOAuth2}
		assert.True(t, IsOAuth2Provider(config))
	})

	t.Run("returns false for api_key auth mode", func(t *testing.T) {
		config := &conf.ModelProviderConfig{AuthMode: conf.AuthModeAPIKey}
		assert.False(t, IsOAuth2Provider(config))
	})

	t.Run("returns false for none auth mode", func(t *testing.T) {
		config := &conf.ModelProviderConfig{AuthMode: conf.AuthModeNone}
		assert.False(t, IsOAuth2Provider(config))
	})

	t.Run("returns false for empty auth mode", func(t *testing.T) {
		config := &conf.ModelProviderConfig{}
		assert.False(t, IsOAuth2Provider(config))
	})

	t.Run("returns false for nil config", func(t *testing.T) {
		assert.False(t, IsOAuth2Provider(nil))
	})
}
