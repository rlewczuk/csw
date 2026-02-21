package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

func TestGenerateOAuthPKCECodes(t *testing.T) {
	codes, err := GenerateOAuthPKCECodes()
	require.NoError(t, err)
	require.NotNil(t, codes)

	assert.NotEmpty(t, codes.Verifier)
	assert.NotEmpty(t, codes.Challenge)
	assert.NotContains(t, codes.Verifier, "=")
	assert.NotContains(t, codes.Challenge, "=")

	hash := sha256.Sum256([]byte(codes.Verifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(hash[:])
	assert.Equal(t, expectedChallenge, codes.Challenge)
}

func TestGenerateOAuthState(t *testing.T) {
	state, err := GenerateOAuthState()
	require.NoError(t, err)
	assert.NotEmpty(t, state)
	assert.NotContains(t, state, "=")

	decoded, decodeErr := base64.RawURLEncoding.DecodeString(state)
	require.NoError(t, decodeErr)
	assert.NotEmpty(t, decoded)
}

func TestBuildAuthorizationURL(t *testing.T) {
	t.Run("builds URL with required and extra params", func(t *testing.T) {
		config := &conf.ModelProviderConfig{
			AuthURL:  "https://auth.example.com/oauth/authorize",
			ClientID: "client-123",
		}

		result, err := BuildAuthorizationURL(
			config,
			"http://localhost:11435/auth/callback",
			"state-123",
			"challenge-123",
			DefaultOAuthScope,
			map[string]string{
				"originator":                 "opencode",
				"id_token_add_organizations": "true",
			},
		)
		require.NoError(t, err)

		parsed, err := url.Parse(result)
		require.NoError(t, err)

		query := parsed.Query()
		assert.Equal(t, "code", query.Get("response_type"))
		assert.Equal(t, "client-123", query.Get("client_id"))
		assert.Equal(t, "http://localhost:11435/auth/callback", query.Get("redirect_uri"))
		assert.Equal(t, "state-123", query.Get("state"))
		assert.Equal(t, "challenge-123", query.Get("code_challenge"))
		assert.Equal(t, OAuthCodeChallengeMethodS256, query.Get("code_challenge_method"))
		assert.Equal(t, DefaultOAuthScope, query.Get("scope"))
		assert.Equal(t, "opencode", query.Get("originator"))
		assert.Equal(t, "true", query.Get("id_token_add_organizations"))
	})

	t.Run("uses default scope when scope is empty", func(t *testing.T) {
		config := &conf.ModelProviderConfig{
			AuthURL:  "https://auth.example.com/oauth/authorize",
			ClientID: "client-123",
		}

		result, err := BuildAuthorizationURL(config, "http://localhost/callback", "state", "challenge", "", nil)
		require.NoError(t, err)

		parsed, err := url.Parse(result)
		require.NoError(t, err)
		assert.Equal(t, DefaultOAuthScope, parsed.Query().Get("scope"))
	})

	t.Run("returns validation errors", func(t *testing.T) {
		tests := []struct {
			name    string
			config  *conf.ModelProviderConfig
			wantErr string
		}{
			{name: "nil config", config: nil, wantErr: "config cannot be nil"},
			{name: "missing auth URL", config: &conf.ModelProviderConfig{ClientID: "id"}, wantErr: "AuthURL is required"},
			{name: "missing client ID", config: &conf.ModelProviderConfig{AuthURL: "https://auth.example.com"}, wantErr: "ClientID is required"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				_, err := BuildAuthorizationURL(tc.config, "http://localhost/callback", "state", "challenge", DefaultOAuthScope, nil)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			})
		}
	})
}

func TestExchangeAuthorizationCode(t *testing.T) {
	t.Run("successfully exchanges code for tokens", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`)

		config := &conf.ModelProviderConfig{
			TokenURL: "" + mock.URL() + "/oauth/token",
			ClientID: "test-client-id",
		}

		resp, err := ExchangeAuthorizationCode(config, mock.Client(), "auth-code", "http://localhost:11435/auth/callback", "verifier")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "new-access-token", resp.AccessToken)
		assert.Equal(t, "new-refresh-token", resp.RefreshToken)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)
		body := string(reqs[0].Body)
		assert.True(t, strings.Contains(body, "grant_type=authorization_code"))
		assert.True(t, strings.Contains(body, "code=auth-code"))
		assert.True(t, strings.Contains(body, "client_id=test-client-id"))
		assert.True(t, strings.Contains(body, "code_verifier=verifier"))
	})

	t.Run("returns validation errors", func(t *testing.T) {
		tests := []struct {
			name       string
			config     *conf.ModelProviderConfig
			code       string
			redirect   string
			verifier   string
			wantErrMsg string
		}{
			{name: "nil config", config: nil, code: "code", redirect: "http://localhost/callback", verifier: "verifier", wantErrMsg: "config cannot be nil"},
			{name: "missing token url", config: &conf.ModelProviderConfig{ClientID: "id"}, code: "code", redirect: "http://localhost/callback", verifier: "verifier", wantErrMsg: "TokenURL is required"},
			{name: "missing client id", config: &conf.ModelProviderConfig{TokenURL: "https://example.com/token"}, code: "code", redirect: "http://localhost/callback", verifier: "verifier", wantErrMsg: "ClientID is required"},
			{name: "missing code", config: &conf.ModelProviderConfig{TokenURL: "https://example.com/token", ClientID: "id"}, redirect: "http://localhost/callback", verifier: "verifier", wantErrMsg: "code is required"},
			{name: "missing redirect", config: &conf.ModelProviderConfig{TokenURL: "https://example.com/token", ClientID: "id"}, code: "code", verifier: "verifier", wantErrMsg: "redirectURI is required"},
			{name: "missing verifier", config: &conf.ModelProviderConfig{TokenURL: "https://example.com/token", ClientID: "id"}, code: "code", redirect: "http://localhost/callback", wantErrMsg: "codeVerifier is required"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				_, err := ExchangeAuthorizationCode(tc.config, http.DefaultClient, tc.code, tc.redirect, tc.verifier)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrMsg)
			})
		}
	})

	t.Run("returns error for non-OK status", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponseWithStatus("/oauth/token", "POST", `{"error":"invalid_grant"}`, http.StatusBadRequest)

		config := &conf.ModelProviderConfig{
			TokenURL: mock.URL() + "/oauth/token",
			ClientID: "test-client-id",
		}

		_, err := ExchangeAuthorizationCode(config, mock.Client(), "bad-code", "http://localhost/callback", "verifier")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token exchange failed with status 400")
	})
}

func TestWaitForOAuthCallback(t *testing.T) {
	t.Run("returns callback code and state", func(t *testing.T) {
		port := findFreePort(t)
		listenAddress := "127.0.0.1:" + port
		callbackPath := "/auth/callback"

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resultCh := make(chan *OAuthCallback, 1)
		errCh := make(chan error, 1)
		go func() {
			result, err := WaitForOAuthCallback(ctx, listenAddress, callbackPath)
			if err != nil {
				errCh <- err
				return
			}
			resultCh <- result
		}()

		callbackURL := "http://localhost:" + port + callbackPath + "?code=test-code&state=test-state"
		sendCallbackRequestWithRetry(t, callbackURL)

		select {
		case result := <-resultCh:
			require.NotNil(t, result)
			assert.Equal(t, "test-code", result.Code)
			assert.Equal(t, "test-state", result.State)
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatalf("TestWaitForOAuthCallback() [oauth_test.go]: timeout waiting for callback result")
		}
	})

	t.Run("returns error when oauth callback includes error", func(t *testing.T) {
		port := findFreePort(t)
		listenAddress := "127.0.0.1:" + port
		callbackPath := "/auth/callback"

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			_, err := WaitForOAuthCallback(ctx, listenAddress, callbackPath)
			errCh <- err
		}()

		callbackURL := "http://localhost:" + port + callbackPath + "?error=access_denied&error_description=user%20cancelled"
		sendCallbackRequestWithRetry(t, callbackURL)

		select {
		case err := <-errCh:
			require.Error(t, err)
			assert.Contains(t, err.Error(), "oauth callback error")
		case <-time.After(5 * time.Second):
			t.Fatalf("TestWaitForOAuthCallback() [oauth_test.go]: timeout waiting for callback error")
		}
	})
}

func sendCallbackRequestWithRetry(t *testing.T, callbackURL string) {
	t.Helper()

	var lastErr error
	for i := 0; i < 50; i++ {
		resp, err := http.Get(callbackURL)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}

	require.NoError(t, lastErr)
}

func findFreePort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return strconv.Itoa(addr.Port)
}
