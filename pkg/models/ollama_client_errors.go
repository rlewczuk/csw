package models

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// handleHTTPError converts HTTP errors to appropriate model errors.
// Network errors that can be retried are wrapped in NetworkError.
func (c *OllamaClient) handleHTTPError(err error) error {
	if err == nil {
		return nil
	}

	// Check for DNS errors first
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		// Temporary DNS errors (like server misbehaving) should be retried
		if dnsErr.IsTemporary || dnsErr.IsNotFound {
			// IsNotFound means the host doesn't exist - this should NOT be retried
			if dnsErr.IsNotFound && !dnsErr.IsTemporary {
				return fmt.Errorf("%w: %v", ErrEndpointNotFound, err)
			}
			// Temporary DNS issues should be retried
			return &NetworkError{
				Message:     fmt.Sprintf("temporary DNS error: %v", err),
				IsRetryable: true,
			}
		}
		// Other DNS errors (like server misbehaving) should be retried
		return &NetworkError{
			Message:     fmt.Sprintf("DNS error: %v", err),
			IsRetryable: true,
		}
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Timeout and temporary errors can be retried
		if netErr.Timeout() || netErr.Temporary() {
			return &NetworkError{
				Message:     fmt.Sprintf("network timeout: %v", err),
				IsRetryable: true,
			}
		}
		// Connection refused and other network errors can also be retried
		return &NetworkError{
			Message:     fmt.Sprintf("network error: %v", err),
			IsRetryable: true,
		}
	}

	// Check for connection refused errors (can be retried)
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" || opErr.Op == "read" || opErr.Op == "write" {
			return &NetworkError{
				Message:     fmt.Sprintf("connection error: %v", err),
				IsRetryable: true,
			}
		}
	}

	// For other errors, wrap as endpoint unavailable (not retryable by default)
	return fmt.Errorf("%w: %v", ErrEndpointUnavailable, err)
}

// checkStatusCode checks the HTTP status code and returns appropriate errors.
func (c *OllamaClient) checkStatusCode(resp *http.Response) error {
	return c.checkStatusCodeWithBody(resp, nil)
}

// checkStatusCodeWithBody checks the HTTP status code and returns appropriate errors.
// bodyBytes can be provided if the body has already been read (for error message extraction).
func (c *OllamaClient) checkStatusCodeWithBody(resp *http.Response, bodyBytes []byte) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return fmt.Errorf("%w: %s", ErrEndpointNotFound, string(bodyBytes))
	case http.StatusUnauthorized, http.StatusForbidden:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return fmt.Errorf("%w: %s", ErrPermissionDenied, string(bodyBytes))
	case http.StatusTooManyRequests:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return c.handleRateLimitErrorWithBody(resp, bodyBytes)
	case http.StatusBadRequest:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		bodyStr := string(bodyBytes)
		// Check for context length errors
		if strings.Contains(strings.ToLower(bodyStr), "context length") ||
			strings.Contains(strings.ToLower(bodyStr), "too many tokens") {
			return fmt.Errorf("%w: %s", ErrTooManyInputTokens, bodyStr)
		}
		return fmt.Errorf("bad request: %s", bodyStr)
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return fmt.Errorf("%w: %s", ErrEndpointUnavailable, string(bodyBytes))
	default:
		if bodyBytes == nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

// handleRateLimitError handles rate limit (429) errors and extracts retry information.
func (c *OllamaClient) handleRateLimitError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return c.handleRateLimitErrorWithBody(resp, body)
}

// handleRateLimitErrorWithBody handles rate limit (429) errors and extracts retry information.
func (c *OllamaClient) handleRateLimitErrorWithBody(resp *http.Response, bodyBytes []byte) error {
	bodyStr := string(bodyBytes)

	retryAfter := 0

	// Try to parse Retry-After header
	if retryAfterHeader := resp.Header.Get("Retry-After"); retryAfterHeader != "" {
		if seconds, err := strconv.Atoi(retryAfterHeader); err == nil {
			retryAfter = seconds
		}
	}

	return &RateLimitError{
		RetryAfterSeconds: retryAfter,
		Message:           bodyStr,
	}
}
