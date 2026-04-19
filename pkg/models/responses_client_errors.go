package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// responsesRateLimitErrorBody represents a 429 error response payload.
type responsesRateLimitErrorBody struct {
	Error *responsesRateLimitErrorDetails `json:"error"`
}

// responsesRateLimitErrorDetails represents rate-limit metadata returned in error payloads.
type responsesRateLimitErrorDetails struct {
	Type            string `json:"type"`
	Message         string `json:"message"`
	ResetsInSeconds int    `json:"resets_in_seconds"`
}

// handleHTTPError converts HTTP errors to appropriate model errors.
// Network errors that can be retried are wrapped in NetworkError.
func (c *ResponsesClient) handleHTTPError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return &NetworkError{
			Message:     fmt.Sprintf("unexpected stream EOF: %v", err),
			IsRetryable: true,
		}
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
func (c *ResponsesClient) checkStatusCode(resp *http.Response) error {
	return c.checkStatusCodeWithBody(resp, nil)
}

// checkStatusCodeWithBody checks the HTTP status code and returns appropriate errors.
// bodyBytes can be provided if the body has already been read (for error message extraction).
func (c *ResponsesClient) checkStatusCodeWithBody(resp *http.Response, bodyBytes []byte) error {
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

		var errResp OpenaiErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error != nil {
			if strings.Contains(strings.ToLower(errResp.Error.Message), "context length") ||
				strings.Contains(strings.ToLower(errResp.Error.Message), "too many tokens") ||
				strings.Contains(strings.ToLower(errResp.Error.Message), "maximum context length") {
				return fmt.Errorf("%w: %s", ErrTooManyInputTokens, errResp.Error.Message)
			}
			return fmt.Errorf("bad request: %s", errResp.Error.Message)
		}

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
func (c *ResponsesClient) handleRateLimitError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return c.handleRateLimitErrorWithBody(resp, body)
}

// handleRateLimitErrorWithBody handles rate limit (429) errors and extracts retry information.
func (c *ResponsesClient) handleRateLimitErrorWithBody(resp *http.Response, bodyBytes []byte) error {
	bodyStr := string(bodyBytes)

	retryAfter := 0

	// Try to parse Retry-After header
	if retryAfterHeader := resp.Header.Get("Retry-After"); retryAfterHeader != "" {
		if seconds, err := strconv.Atoi(retryAfterHeader); err == nil {
			retryAfter = seconds
		}
	}

	codexRetryAfter := codexLimitRetryAfterSeconds(resp.Header)

	// Try to parse error response for retry information
	var usageLimitResp responsesRateLimitErrorBody
	if json.Unmarshal(bodyBytes, &usageLimitResp) == nil && usageLimitResp.Error != nil {
		retryAfter = mergeRateLimitRetryAfter(
			retryAfter,
			codexRetryAfter,
			usageLimitResp.Error.ResetsInSeconds,
			usageLimitResp.Error.Type,
		)
		return &RateLimitError{
			RetryAfterSeconds: retryAfter,
			Message:           usageLimitResp.Error.Message,
		}
	}

	var errResp OpenaiErrorResponse
	if json.Unmarshal(bodyBytes, &errResp) == nil && errResp.Error != nil {
		retryAfter = mergeRateLimitRetryAfter(retryAfter, codexRetryAfter, 0, errResp.Error.Type)
		return &RateLimitError{
			RetryAfterSeconds: retryAfter,
			Message:           errResp.Error.Message,
		}
	}

	retryAfter = mergeRateLimitRetryAfter(retryAfter, codexRetryAfter, 0, "")

	return &RateLimitError{
		RetryAfterSeconds: retryAfter,
		Message:           bodyStr,
	}
}

// mergeRateLimitRetryAfter merges retry-after candidates based on error semantics.
func mergeRateLimitRetryAfter(retryAfter int, codexRetryAfter int, bodyRetryAfter int, errorType string) int {
	errorTypeLower := strings.ToLower(strings.TrimSpace(errorType))
	if errorTypeLower == "usage_limit_reached" {
		if codexRetryAfter > 0 {
			return codexRetryAfter
		}
		if bodyRetryAfter > 0 {
			return bodyRetryAfter
		}
		return retryAfter
	}

	if retryAfter > 0 {
		return retryAfter
	}
	if bodyRetryAfter > 0 {
		return bodyRetryAfter
	}
	if codexRetryAfter > 0 {
		return codexRetryAfter
	}

	return 0
}

// codexLimitRetryAfterSeconds resolves retry-after from Codex-specific limit headers.
func codexLimitRetryAfterSeconds(headers http.Header) int {
	if len(headers) == 0 {
		return 0
	}

	primaryUsedPercent := parseIntHeader(headers, "X-Codex-Primary-Used-Percent")
	secondaryUsedPercent := parseIntHeader(headers, "X-Codex-Secondary-Used-Percent")
	primaryResetAfter := parseIntHeader(headers, "X-Codex-Primary-Reset-After-Seconds")
	secondaryResetAfter := parseIntHeader(headers, "X-Codex-Secondary-Reset-After-Seconds")
	activeLimit := strings.ToLower(strings.TrimSpace(headers.Get("X-Codex-Active-Limit")))

	if strings.Contains(activeLimit, "primary") && primaryResetAfter > 0 {
		return primaryResetAfter
	}
	if strings.Contains(activeLimit, "secondary") && secondaryResetAfter > 0 {
		return secondaryResetAfter
	}

	primaryExceeded := primaryUsedPercent >= 100
	secondaryExceeded := secondaryUsedPercent >= 100

	if primaryExceeded && !secondaryExceeded {
		return primaryResetAfter
	}
	if secondaryExceeded && !primaryExceeded {
		return secondaryResetAfter
	}
	if primaryExceeded && secondaryExceeded {
		return maxInt(primaryResetAfter, secondaryResetAfter)
	}

	return 0
}

// parseIntHeader parses an integer header value and returns zero on parse failure.
func parseIntHeader(headers http.Header, key string) int {
	value := strings.TrimSpace(headers.Get(key))
	if value == "" {
		return 0
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}

	return parsed
}

// maxInt returns the larger of two integers.
func maxInt(a int, b int) int {
	if a > b {
		return a
	}

	return b
}

// wrapLLMRequestError wraps request error with raw HTTP response when available.
func wrapLLMRequestError(err error, resp *http.Response, bodyBytes []byte) error {
	if err == nil {
		return nil
	}

	var llmReqErr *LLMRequestError
	if errors.As(err, &llmReqErr) {
		if strings.TrimSpace(llmReqErr.RawResponse) == "" && resp != nil {
			llmReqErr.RawResponse = formatRawHTTPResponse(resp.StatusCode, resp.Header, bodyBytes)
		}
		return err
	}

	rawResponse := ""
	if resp != nil {
		rawResponse = formatRawHTTPResponse(resp.StatusCode, resp.Header, bodyBytes)
	}

	return &LLMRequestError{
		Err:         err,
		RawResponse: rawResponse,
	}
}

// formatRawHTTPResponse formats status, headers, and body into a readable raw response string.
func formatRawHTTPResponse(statusCode int, headers http.Header, bodyBytes []byte) string {
	var responseBuilder strings.Builder

	responseBuilder.WriteString(strconv.Itoa(statusCode))
	responseBuilder.WriteString("\n")

	if headers != nil {
		headerKeys := make([]string, 0, len(headers))
		for key := range headers {
			headerKeys = append(headerKeys, key)
		}
		sort.Strings(headerKeys)

		for _, key := range headerKeys {
			for _, value := range headers.Values(key) {
				responseBuilder.WriteString(key)
				responseBuilder.WriteString(": ")
				responseBuilder.WriteString(value)
				responseBuilder.WriteString("\n")
			}
		}
	}

	responseBuilder.WriteString("\n")
	responseBuilder.Write(bodyBytes)

	return responseBuilder.String()
}
