package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
)

const (
	// OAuthCodeChallengeMethodS256 is the PKCE code challenge method used by OAuth2.
	OAuthCodeChallengeMethodS256 = "S256"
	// DefaultOAuthScope is the default scope used for browser-based OAuth2 flows.
	DefaultOAuthScope = "openid profile email offline_access"
)

// OAuthTokenResponse represents the response from an OAuth2 token endpoint.
type OAuthTokenResponse struct {
	// AccessToken is the OAuth2 access token.
	AccessToken string `json:"access_token"`
	// RefreshToken is the OAuth2 refresh token (may be empty if not rotated).
	RefreshToken string `json:"refresh_token,omitempty"`
	// TokenType is the type of token (usually "Bearer").
	TokenType string `json:"token_type,omitempty"`
	// ExpiresIn is the number of seconds until the access token expires.
	ExpiresIn int `json:"expires_in,omitempty"`
	// Scope is the scope of the access token.
	Scope string `json:"scope,omitempty"`
}

// OAuthPKCECodes represents PKCE verifier/challenge pair.
type OAuthPKCECodes struct {
	// Verifier is a high-entropy cryptographic random string.
	Verifier string
	// Challenge is a BASE64URL-encoded SHA-256 digest of Verifier.
	Challenge string
}

// OAuthCallback contains OAuth2 authorization callback parameters.
type OAuthCallback struct {
	// Code is the authorization code returned by OAuth2 server.
	Code string
	// State is the state value returned by OAuth2 server.
	State string
}

// GenerateOAuthPKCECodes generates a PKCE verifier and challenge pair.
func GenerateOAuthPKCECodes() (*OAuthPKCECodes, error) {
	verifier, err := randomBase64URLString(32)
	if err != nil {
		return nil, fmt.Errorf("GenerateOAuthPKCECodes() [oauth.go]: failed to generate verifier: %w", err)
	}

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &OAuthPKCECodes{Verifier: verifier, Challenge: challenge}, nil
}

// GenerateOAuthState generates a random OAuth2 state parameter.
func GenerateOAuthState() (string, error) {
	state, err := randomBase64URLString(32)
	if err != nil {
		return "", fmt.Errorf("GenerateOAuthState() [oauth.go]: failed to generate state: %w", err)
	}
	return state, nil
}

// BuildAuthorizationURL builds an OAuth2 authorization URL with PKCE parameters.
func BuildAuthorizationURL(
	config *conf.ModelProviderConfig,
	redirectURI string,
	state string,
	codeChallenge string,
	scope string,
	extraQueryParams map[string]string,
) (string, error) {
	if config == nil {
		return "", fmt.Errorf("BuildAuthorizationURL() [oauth.go]: config cannot be nil")
	}

	if config.AuthURL == "" {
		return "", fmt.Errorf("BuildAuthorizationURL() [oauth.go]: AuthURL is required")
	}

	if config.ClientID == "" {
		return "", fmt.Errorf("BuildAuthorizationURL() [oauth.go]: ClientID is required")
	}

	if redirectURI == "" {
		return "", fmt.Errorf("BuildAuthorizationURL() [oauth.go]: redirectURI is required")
	}

	if state == "" {
		return "", fmt.Errorf("BuildAuthorizationURL() [oauth.go]: state is required")
	}

	if codeChallenge == "" {
		return "", fmt.Errorf("BuildAuthorizationURL() [oauth.go]: codeChallenge is required")
	}

	parsed, err := url.Parse(config.AuthURL)
	if err != nil {
		return "", fmt.Errorf("BuildAuthorizationURL() [oauth.go]: failed to parse AuthURL: %w", err)
	}

	query := parsed.Query()
	if scope == "" {
		scope = query.Get("scope")
		if scope == "" {
			scope = DefaultOAuthScope
		}
	}

	query.Set("response_type", "code")
	query.Set("client_id", config.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", scope)
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", OAuthCodeChallengeMethodS256)
	query.Set("state", state)

	for key, value := range extraQueryParams {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		query.Set(key, value)
	}

	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// ExchangeAuthorizationCode exchanges an OAuth2 authorization code for tokens.
func ExchangeAuthorizationCode(
	config *conf.ModelProviderConfig,
	httpClient *http.Client,
	code string,
	redirectURI string,
	codeVerifier string,
) (*OAuthTokenResponse, error) {
	if config == nil {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: config cannot be nil")
	}

	if config.TokenURL == "" {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: TokenURL is required")
	}

	if config.ClientID == "" {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: ClientID is required")
	}

	if code == "" {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: code is required")
	}

	if redirectURI == "" {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: redirectURI is required")
	}

	if codeVerifier == "" {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: codeVerifier is required")
	}

	data := url.Values{
		"grant_type":    []string{"authorization_code"},
		"code":          []string{code},
		"redirect_uri":  []string{redirectURI},
		"client_id":     []string{config.ClientID},
		"code_verifier": []string{codeVerifier},
	}

	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	req, err := http.NewRequest(http.MethodPost, config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: failed to parse response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("ExchangeAuthorizationCode() [oauth.go]: response missing access_token")
	}

	return &tokenResp, nil
}

// WaitForOAuthCallback starts a local HTTP server and waits for OAuth2 redirect callback.
func WaitForOAuthCallback(ctx context.Context, listenAddress, callbackPath string) (*OAuthCallback, error) {
	if strings.TrimSpace(listenAddress) == "" {
		return nil, fmt.Errorf("WaitForOAuthCallback() [oauth.go]: listenAddress cannot be empty")
	}

	if strings.TrimSpace(callbackPath) == "" {
		callbackPath = "/auth/callback"
	}

	type oauthCallbackResult struct {
		Callback *OAuthCallback
		Err      error
	}

	resultCh := make(chan oauthCallbackResult, 1)
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    listenAddress,
		Handler: mux,
	}

	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		oauthErr := query.Get("error")
		if oauthErr != "" {
			errorDescription := query.Get("error_description")
			if errorDescription == "" {
				errorDescription = oauthErr
			}

			http.Error(w, "Authentication failed: "+errorDescription, http.StatusBadRequest)
			select {
			case resultCh <- oauthCallbackResult{Err: fmt.Errorf("WaitForOAuthCallback() [oauth.go]: oauth callback error: %s", errorDescription)}:
			default:
			}
			return
		}

		code := query.Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			select {
			case resultCh <- oauthCallbackResult{Err: fmt.Errorf("WaitForOAuthCallback() [oauth.go]: oauth callback missing code")}:
			default:
			}
			return
		}

		state := query.Get("state")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Authentication successful. You can close this tab now.\n"))

		select {
		case resultCh <- oauthCallbackResult{Callback: &OAuthCallback{Code: code, State: state}}:
		default:
		}
	})

	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	select {
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Callback, nil
	case err := <-serverErrCh:
		return nil, fmt.Errorf("WaitForOAuthCallback() [oauth.go]: failed to start callback server: %w", err)
	case <-ctx.Done():
		return nil, fmt.Errorf("WaitForOAuthCallback() [oauth.go]: context cancelled while waiting for callback: %w", ctx.Err())
	}
}

// RenewToken renews an OAuth2 access token using the refresh token.
// It returns the new access token and the expiration time.
// The config must have TokenURL, ClientID, and RefreshToken set.
func RenewToken(config *conf.ModelProviderConfig, httpClient *http.Client) (*OAuthTokenResponse, error) {
	if config == nil {
		return nil, fmt.Errorf("RenewToken() [oauth.go]: config cannot be nil")
	}

	if config.TokenURL == "" {
		return nil, fmt.Errorf("RenewToken() [oauth.go]: TokenURL is required")
	}

	if config.ClientID == "" {
		return nil, fmt.Errorf("RenewToken() [oauth.go]: ClientID is required")
	}

	if config.RefreshToken == "" {
		return nil, fmt.Errorf("RenewToken() [oauth.go]: RefreshToken is required")
	}

	// Build the request body
	data := url.Values{
		"grant_type":    []string{"refresh_token"},
		"refresh_token": []string{config.RefreshToken},
		"client_id":     []string{config.ClientID},
	}

	// Add client secret if provided
	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	// Create the request
	req, err := http.NewRequest(http.MethodPost, config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("RenewToken() [oauth.go]: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RenewToken() [oauth.go]: token request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("RenewToken() [oauth.go]: failed to read response: %w", err)
	}

	// Check for non-OK status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RenewToken() [oauth.go]: token renewal failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("RenewToken() [oauth.go]: failed to parse response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("RenewToken() [oauth.go]: response missing access_token")
	}

	return &tokenResp, nil
}

// CalculateTokenExpiry calculates the expiration time from expires_in seconds.
// If expires_in is 0 or negative, it returns a zero time indicating unknown expiry.
func CalculateTokenExpiry(expiresIn int) time.Time {
	if expiresIn <= 0 {
		return time.Time{}
	}
	return time.Now().Add(time.Duration(expiresIn) * time.Second)
}

// IsTokenExpired checks if the token is expired or about to expire.
// It returns true if the expiry time is zero or has passed, or if the token
// will expire within the safety margin.
func IsTokenExpired(expiry time.Time, safetyMargin time.Duration) bool {
	if expiry.IsZero() {
		return true
	}
	return time.Now().Add(safetyMargin).After(expiry)
}

// IsOAuth2Provider checks if the provider config uses OAuth2 authentication.
func IsOAuth2Provider(config *conf.ModelProviderConfig) bool {
	if config == nil {
		return false
	}
	return config.AuthMode == conf.AuthModeOAuth2
}

// ExtractJWTExpiry extracts the expiry time from a JWT access token.
// It returns a zero time when the token does not include an exp claim.
func ExtractJWTExpiry(token string) (time.Time, error) {
	if token == "" {
		return time.Time{}, fmt.Errorf("ExtractJWTExpiry() [oauth.go]: token cannot be empty")
	}

	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("ExtractJWTExpiry() [oauth.go]: invalid token format")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("ExtractJWTExpiry() [oauth.go]: failed to decode token payload: %w", err)
	}

	var payload struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return time.Time{}, fmt.Errorf("ExtractJWTExpiry() [oauth.go]: failed to parse token payload: %w", err)
	}

	if payload.Exp <= 0 {
		return time.Time{}, nil
	}

	return time.Unix(payload.Exp, 0), nil
}

func randomBase64URLString(size int) (string, error) {
	if size <= 0 {
		return "", fmt.Errorf("randomBase64URLString() [oauth.go]: size must be positive")
	}

	randomBytes := make([]byte, size)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("randomBase64URLString() [oauth.go]: failed to read random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}
