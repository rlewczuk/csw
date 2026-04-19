package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// usageLimitResetAtPattern matches usage-limit reset timestamps in provider error messages.
var usageLimitResetAtPattern = regexp.MustCompile(`(?i)reset at\s+(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)

// handleHTTPError converts HTTP errors to appropriate model errors.
// Network errors that can be retried are wrapped in NetworkError.
func (c *OpenAIClient) handleHTTPError(err error) error {
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
func (c *OpenAIClient) checkStatusCode(resp *http.Response) error {
	return c.checkStatusCodeWithBody(resp, nil)
}

// checkStatusCodeWithBody checks the HTTP status code and returns appropriate errors.
// bodyBytes can be provided if the body has already been read (for error message extraction).
func (c *OpenAIClient) checkStatusCodeWithBody(resp *http.Response, bodyBytes []byte) error {
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

		// Try to parse error response
		var errResp OpenaiErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error != nil {
			// Check for context length errors
			if strings.Contains(strings.ToLower(errResp.Error.Message), "context length") ||
				strings.Contains(strings.ToLower(errResp.Error.Message), "too many tokens") ||
				strings.Contains(strings.ToLower(errResp.Error.Message), "maximum context length") {
				return fmt.Errorf("%w: %s", ErrTooManyInputTokens, errResp.Error.Message)
			}
			return fmt.Errorf("bad request: %s", errResp.Error.Message)
		}

		// Fallback to raw body
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
func (c *OpenAIClient) handleRateLimitError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return c.handleRateLimitErrorWithBody(resp, body)
}

// handleRateLimitErrorWithBody handles rate limit (429) errors and extracts retry information.
func (c *OpenAIClient) handleRateLimitErrorWithBody(resp *http.Response, bodyBytes []byte) error {
	bodyStr := string(bodyBytes)

	retryAfter := 0

	// Try to parse Retry-After header
	if retryAfterHeader := resp.Header.Get("Retry-After"); retryAfterHeader != "" {
		if seconds, err := strconv.Atoi(retryAfterHeader); err == nil {
			retryAfter = seconds
		}
	}

	// Try to parse error response for retry information
	var errResp OpenaiErrorResponse
	if json.Unmarshal(bodyBytes, &errResp) == nil && errResp.Error != nil {
		usageLimitRetryAfter := parseUsageLimitRetryAfterSeconds(errResp.Error.Message, time.Now())
		retryAfter = mergeOpenAIRetryAfterForUsageLimit(retryAfter, usageLimitRetryAfter, errResp.Error.Message, errResp.Error.Code)
		return &RateLimitError{
			RetryAfterSeconds: retryAfter,
			Message:           errResp.Error.Message,
		}
	}

	usageLimitRetryAfter := parseUsageLimitRetryAfterSeconds(bodyStr, time.Now())
	retryAfter = mergeOpenAIRetryAfterForUsageLimit(retryAfter, usageLimitRetryAfter, bodyStr, nil)

	return &RateLimitError{
		RetryAfterSeconds: retryAfter,
		Message:           bodyStr,
	}
}

// mergeOpenAIRetryAfterForUsageLimit merges retry-after values for usage-limit errors.
func mergeOpenAIRetryAfterForUsageLimit(retryAfter int, usageLimitRetryAfter int, message string, code interface{}) int {
	if !isOpenAIUsageLimitError(message, code) {
		return retryAfter
	}

	if usageLimitRetryAfter > retryAfter {
		return usageLimitRetryAfter
	}

	return retryAfter
}

// isOpenAIUsageLimitError returns true when OpenAI protocol error indicates usage limit reached.
func isOpenAIUsageLimitError(message string, code interface{}) bool {
	if strings.Contains(strings.ToLower(message), "usage limit") {
		return true
	}

	switch typedCode := code.(type) {
	case string:
		return strings.TrimSpace(typedCode) == "1308"
	case float64:
		return int(typedCode) == 1308
	case int:
		return typedCode == 1308
	default:
		return false
	}
}

// parseUsageLimitRetryAfterSeconds parses usage-limit reset timestamp and returns seconds until reset.
func parseUsageLimitRetryAfterSeconds(message string, now time.Time) int {
	if strings.TrimSpace(message) == "" {
		return 0
	}

	matches := usageLimitResetAtPattern.FindStringSubmatch(message)
	if len(matches) != 2 {
		return 0
	}

	const resetAtLayout = "2006-01-02 15:04:05"

	parsedResetAt, err := time.ParseInLocation(resetAtLayout, matches[1], time.Local)
	if err != nil {
		return 0
	}

	untilReset := parsedResetAt.Sub(now)
	if untilReset <= 0 {
		return 0
	}

	return int(math.Ceil(untilReset.Seconds()))
}
