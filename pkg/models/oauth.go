package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
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
