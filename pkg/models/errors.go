package models

import (
	"errors"
	"fmt"
)

// Network and API error variables.
var (
	// ErrEndpointNotFound is returned when endpoint or host is not found (either 404 or host not in DNS).
	ErrEndpointNotFound = errors.New("endpoint or host not found (either 404 or not host not in DNS)")
	// ErrEndpointUnavailable is returned when endpoint is unavailable (network error, host not responding, or 5xx error).
	ErrEndpointUnavailable = errors.New("endpoint is unavailable (network error, host not responding, or 5xx error)")
	// ErrPermissionDenied is returned when permission is denied (eg. missing API key).
	ErrPermissionDenied = errors.New("permission denied (eg. missing API key)")
	// ErrRateExceeded is returned when rate limit is exceeded.
	ErrRateExceeded = errors.New("rate exceeded")
	// ErrTooManyInputTokens is returned when there are too many input tokens (i.e. exceeding context length).
	ErrTooManyInputTokens = errors.New("too many input tokens (i.e. exceeding context length)")
	// ErrToBeContinued is returned when generated tokens limit is reached.
	ErrToBeContinued = errors.New("to be continued (i.e. generated tokens limit reached)")
	// ErrNetworkError is returned for general network errors.
	ErrNetworkError = errors.New("network error")
	// ErrProviderNotFound is returned when a provider is not found in the registry.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrInvalidInput is returned when input format is invalid.
	ErrInvalidInput = errors.New("invalid input format: expected string or []string")
)

// RateLimitError represents a rate limit (429) error with retry information.
type RateLimitError struct {
	// RetryAfterSeconds is the estimated time in seconds when the request can be retried.
	// This is typically parsed from the Retry-After header or API response.
	// A value of 0 means the retry time is unknown and exponential backoff should be used.
	RetryAfterSeconds int
	// Message contains the error message from the API
	Message string
}

// Error returns the error message for RateLimitError.
func (e *RateLimitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "rate exceeded"
}

// Unwrap returns the underlying ErrRateExceeded error.
func (e *RateLimitError) Unwrap() error {
	return ErrRateExceeded
}

// NetworkError represents a network error that can be retried.
// It is compatible with RateLimitError handling in session retry logic.
type NetworkError struct {
	// Message contains the error message describing the network error.
	Message string
	// IsRetryable indicates whether this error can be retried.
	IsRetryable bool
}

// Error returns the error message for NetworkError.
func (e *NetworkError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "network error"
}

// Unwrap returns the underlying ErrNetworkError error.
func (e *NetworkError) Unwrap() error {
	return ErrNetworkError
}

// APIRequestError represents a structured API request validation error.
type APIRequestError struct {
	// ErrorType is a provider-specific error category.
	ErrorType string
	// Code is a provider-specific error code.
	Code string
	// Param is the request parameter that caused the error.
	Param string
	// Message is a human-readable error message.
	Message string
}

// Error returns the error message for APIRequestError.
func (e *APIRequestError) Error() string {
	if e == nil {
		return "api request error"
	}

	return fmt.Sprintf("api request error: type=%q code=%q param=%q message=%q", e.ErrorType, e.Code, e.Param, e.Message)
}
