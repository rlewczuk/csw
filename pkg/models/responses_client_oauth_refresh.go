package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Default token refresh safety margin - refresh token 5 minutes before expiry.
const defaultTokenRefreshMargin = 5 * time.Minute

const oauthRefreshMarginOptionKey = "oauth_refresh_margin"

// RefreshTokenIfNeeded checks if the OAuth2 access token needs to be refreshed
// and refreshes it if necessary. It returns an error if the refresh fails.
// For non-OAuth2 providers, this method does nothing and returns nil.
// If a ConfigUpdater is set and the token is refreshed successfully, the
// updated configuration (including new refresh token if provided) will be
// persisted using the ConfigUpdater callback.
func (c *ResponsesClient) RefreshTokenIfNeeded() error {
	return c.refreshTokenIfNeeded(false, "")
}

func (c *ResponsesClient) refreshTokenIfNeeded(force bool, previousToken string) error {
	if !IsOAuth2Provider(c.config) {
		c.logRefreshDecision(
			"skip-non-oauth2",
			force,
			"skip",
			"provider auth mode is not oauth2",
			time.Now(),
			time.Time{},
			0,
			false,
			false,
		)
		return nil
	}

	if c.isTokenRefreshDisabled() {
		c.tokenMu.RLock()
		currentToken := c.apiKey
		expiry := c.tokenExpiry
		c.tokenMu.RUnlock()

		c.logRefreshDecision(
			"refresh-disabled",
			force,
			"skip",
			"token refresh disabled by provider config",
			time.Now(),
			expiry,
			c.tokenRefreshMargin(),
			currentToken != "",
			c.hasRefreshToken(),
		)
		return nil
	}

	if !force {
		c.tokenMu.RLock()
		currentToken := c.apiKey
		expiry := c.tokenExpiry
		c.tokenMu.RUnlock()

		// When token expiry cannot be determined (opaque access token), avoid eager
		// refresh and rely on 401-based refresh path.
		if currentToken != "" && expiry.IsZero() {
			c.logRefreshDecision(
				"prelock-unknown-expiry",
				force,
				"skip",
				"token expiry unknown for non-empty token; rely on 401 refresh",
				time.Now(),
				expiry,
				c.tokenRefreshMargin(),
				true,
				c.hasRefreshToken(),
			)
			return nil
		}

		if !IsTokenExpired(expiry, c.tokenRefreshMargin()) {
			c.logRefreshDecision(
				"prelock-token-valid",
				force,
				"skip",
				"token overlap above refresh margin",
				time.Now(),
				expiry,
				c.tokenRefreshMargin(),
				currentToken != "",
				c.hasRefreshToken(),
			)
			return nil
		}

		c.logRefreshDecision(
			"prelock-expired-or-near-expiry",
			force,
			"refresh",
			"token expired or overlap below refresh margin",
			time.Now(),
			expiry,
			c.tokenRefreshMargin(),
			currentToken != "",
			c.hasRefreshToken(),
		)
	}

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if force {
		if previousToken != "" && c.apiKey != "" && c.apiKey != previousToken {
			c.logRefreshDecision(
				"postlock-force-token-changed",
				force,
				"skip",
				"token changed by another goroutine",
				time.Now(),
				c.tokenExpiry,
				c.tokenRefreshMargin(),
				c.apiKey != "",
				c.hasRefreshToken(),
			)
			return nil
		}
	} else {
		if c.apiKey != "" && c.tokenExpiry.IsZero() {
			c.logRefreshDecision(
				"postlock-unknown-expiry",
				force,
				"skip",
				"token expiry unknown for non-empty token; rely on 401 refresh",
				time.Now(),
				c.tokenExpiry,
				c.tokenRefreshMargin(),
				true,
				c.hasRefreshToken(),
			)
			return nil
		}
		if !IsTokenExpired(c.tokenExpiry, c.tokenRefreshMargin()) {
			c.logRefreshDecision(
				"postlock-token-valid",
				force,
				"skip",
				"token overlap above refresh margin",
				time.Now(),
				c.tokenExpiry,
				c.tokenRefreshMargin(),
				c.apiKey != "",
				c.hasRefreshToken(),
			)
			return nil
		}
	}

	c.logRefreshDecision(
		"postlock-refresh-start",
		force,
		"refresh",
		"attempting token renewal",
		time.Now(),
		c.tokenExpiry,
		c.tokenRefreshMargin(),
		c.apiKey != "",
		c.hasRefreshToken(),
	)

	// Refresh the token
	resp, err := RenewToken(c.config, c.httpClient)
	if err != nil {
		c.logRefreshDecision(
			"refresh-failed",
			force,
			"error",
			err.Error(),
			time.Now(),
			c.tokenExpiry,
			c.tokenRefreshMargin(),
			c.apiKey != "",
			c.hasRefreshToken(),
		)
		return fmt.Errorf("ResponsesClient.RefreshTokenIfNeeded() [responses_client_oauth_refresh.go]: %w", err)
	}

	// Update the API key and expiry
	c.apiKey = resp.AccessToken
	c.tokenExpiry = CalculateTokenExpiry(resp.ExpiresIn)

	// Update the stored access token
	needsPersist := false
	if c.config != nil {
		c.config.APIKey = resp.AccessToken
		needsPersist = true
	}

	// Update the refresh token if a new one was provided
	if resp.RefreshToken != "" && c.config != nil {
		c.config.RefreshToken = resp.RefreshToken
		needsPersist = true
	}

	// Persist the configuration if a ConfigUpdater is set and changes were made
	if needsPersist && c.configUpdater != nil {
		if err := c.configUpdater(c.config); err != nil {
			// Log the error but don't fail the token refresh
			// The in-memory config is still updated correctly
			fmt.Fprintf(os.Stderr, "WARNING: ResponsesClient.RefreshTokenIfNeeded() [responses_client_oauth_refresh.go]: failed to persist config: %v\n", err)
		}
	}

	c.logRefreshDecision(
		"refresh-success",
		force,
		"refresh",
		"token renewed successfully",
		time.Now(),
		c.tokenExpiry,
		c.tokenRefreshMargin(),
		c.apiKey != "",
		c.hasRefreshToken(),
	)

	return nil
}

func (c *ResponsesClient) isTokenRefreshDisabled() bool {
	if c == nil || c.config == nil {
		return false
	}

	return c.config.DisableRefresh
}

func (c *ResponsesClient) hasRefreshToken() bool {
	return c != nil && c.config != nil && strings.TrimSpace(c.config.RefreshToken) != ""
}

func (c *ResponsesClient) logRefreshDecision(
	stage string,
	force bool,
	decision string,
	reason string,
	now time.Time,
	expiry time.Time,
	minOverlap time.Duration,
	hasAccessToken bool,
	hasRefreshToken bool,
) {
	if decision == "skip" {
		return
	}

	if now.IsZero() {
		now = time.Now()
	}

	overlap := "unknown"
	expiryText := "unknown"
	if !expiry.IsZero() {
		overlap = expiry.Sub(now).String()
		expiryText = expiry.Format(time.RFC3339Nano)
	}

	providerName := ""
	authMode := ""
	disableRefresh := false
	if c != nil && c.config != nil {
		providerName = c.config.Name
		authMode = string(c.config.AuthMode)
		disableRefresh = c.config.DisableRefresh
	}

	fmt.Fprintf(
		os.Stdout,
		"[oauth-refresh-debug] provider=%q auth_mode=%q stage=%q decision=%q reason=%q force=%t now=%q token_expiration=%q overlap=%q min_overlap=%q token_expiry_known=%t has_access_token=%t has_refresh_token=%t disable_refresh=%t\n",
		providerName,
		authMode,
		stage,
		decision,
		reason,
		force,
		now.Format(time.RFC3339Nano),
		expiryText,
		overlap,
		minOverlap.String(),
		!expiry.IsZero(),
		hasAccessToken,
		hasRefreshToken,
		disableRefresh,
	)
}

func (c *ResponsesClient) tokenRefreshMargin() time.Duration {
	if c == nil || c.config == nil || c.config.Options == nil {
		return defaultTokenRefreshMargin
	}

	marginRaw, ok := c.config.Options[oauthRefreshMarginOptionKey]
	if !ok || marginRaw == nil {
		return defaultTokenRefreshMargin
	}

	switch value := marginRaw.(type) {
	case string:
		parsed, err := time.ParseDuration(strings.TrimSpace(value))
		if err == nil && parsed >= 0 {
			return parsed
		}
	case float64:
		if value >= 0 {
			return time.Duration(value * float64(time.Second))
		}
	case int:
		if value >= 0 {
			return time.Duration(value) * time.Second
		}
	case int64:
		if value >= 0 {
			return time.Duration(value) * time.Second
		}
	}

	return defaultTokenRefreshMargin
}

func (c *ResponsesClient) shouldRefreshAfterUnauthorized(resp *http.Response, bodyBytes []byte) bool {
	if c.isTokenRefreshDisabled() {
		c.logRefreshDecision(
			"401-refresh-disabled",
			true,
			"skip",
			"token refresh disabled by provider config",
			time.Now(),
			c.tokenExpiry,
			c.tokenRefreshMargin(),
			c.apiKey != "",
			c.hasRefreshToken(),
		)
		return false
	}

	if !IsOAuth2Provider(c.config) || c.config == nil || c.config.RefreshToken == "" || resp == nil {
		return false
	}

	return isExpiredTokenUnauthorized(resp, bodyBytes)
}

func isExpiredTokenUnauthorized(resp *http.Response, bodyBytes []byte) bool {
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		return false
	}

	wwwAuthenticate := strings.ToLower(resp.Header.Get("WWW-Authenticate"))
	if strings.Contains(wwwAuthenticate, "invalid_token") && strings.Contains(wwwAuthenticate, "expired") {
		return true
	}

	if len(bodyBytes) == 0 {
		return false
	}

	var errResp OpenaiErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil || errResp.Error == nil {
		return false
	}

	message := strings.ToLower(errResp.Error.Message)
	code := strings.ToLower(fmt.Sprintf("%v", errResp.Error.Code))

	if strings.Contains(code, "expired") {
		return true
	}

	if strings.Contains(code, "invalid_token") && strings.Contains(message, "expired") {
		return true
	}

	return strings.Contains(message, "token") && strings.Contains(message, "expired")
}

// GetAccessToken returns the current access token, refreshing it if necessary.
// For non-OAuth2 providers, it returns the static API key.
func (c *ResponsesClient) GetAccessToken() (string, error) {
	if err := c.RefreshTokenIfNeeded(); err != nil {
		return "", err
	}

	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.apiKey, nil
}
