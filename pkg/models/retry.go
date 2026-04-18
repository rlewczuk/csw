package models

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"net"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
)

// usageLimitRetryBufferSeconds adds a safety margin after provider reset time.
const usageLimitRetryBufferSeconds = 10

// RetryPolicy controls retry behavior for temporary LLM API errors.
type RetryPolicy struct {
	// InitialDelay is the base delay for exponential backoff.
	// The delay after attempt N is InitialDelay * 2^(N-1), capped at MaxDelay.
	InitialDelay time.Duration
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int
	// MaxDelay is the maximum delay cap between retries.
	MaxDelay time.Duration
}

// DefaultRetryPolicy returns the default retry policy matching session retry behavior.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		InitialDelay: DefaultRetryBackoffScale,
		MaxRetries:   DefaultMaxRetries,
		MaxDelay:     60 * DefaultRetryBackoffScale,
	}
}

// RetryChatModel wraps a ChatModel and automatically retries temporary errors
// (rate limits, network errors, etc.) with exponential backoff.
type RetryChatModel struct {
	wrapped ChatModel
	policy  RetryPolicy
	logFn   func(string, shared.MessageType)
}

// NewRetryChatModel creates a new RetryChatModel that wraps the given ChatModel
// with the specified retry policy and logging function.
func NewRetryChatModel(wrapped ChatModel, policy *RetryPolicy, logFn func(string, shared.MessageType)) *RetryChatModel {
	return &RetryChatModel{
		wrapped: wrapped,
		policy:  *policy,
		logFn:   logFn,
	}
}

// Chat sends a chat request and retries on temporary errors with exponential backoff.
func (r *RetryChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	totalAttempts := r.policy.MaxRetries + 1

	for attempt := 1; attempt <= totalAttempts; attempt++ {
		response, err := r.wrapped.Chat(ctx, messages, options, tools)
		if err == nil {
			return response, nil
		}

		if !isRetryableError(err) {
			return nil, err
		}

		var rateLimitErr *RateLimitError
		if errors.As(err, &rateLimitErr) && rateLimitErr.RetryAfterSeconds > 0 {
			r.logMessage(buildRateLimitResetMessage(rateLimitErr), shared.MessageTypeWarning)
		}

		r.logMessage(fmt.Sprintf("LLM API temporary error (attempt %d/%d): %v", attempt, totalAttempts, err), shared.MessageTypeError)

		if attempt >= totalAttempts {
			return nil, fmt.Errorf("RetryChatModel.Chat() [retry.go]: temporary LLM API failure after %d attempts: %w", totalAttempts, err)
		}

		delay := r.calculateDelay(attempt, err)
		r.logMessage(fmt.Sprintf("Retrying in %s...", delay.Round(time.Second)), shared.MessageTypeWarning)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, fmt.Errorf("RetryChatModel.Chat() [retry.go]: unexpected exit from retry loop")
}

// ChatStream passes the request directly to the wrapped model's ChatStream method
// without any retry logic.
func (r *RetryChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return r.wrapped.ChatStream(ctx, messages, options, tools)
}

// Compactor returns nil because retry wrapper does not provide session compaction.
func (r *RetryChatModel) Compactor() ChatCompator {
	return nil
}

// calculateDelay computes the backoff delay for the given attempt number and error.
func (r *RetryChatModel) calculateDelay(attempt int, err error) time.Duration {
	// Check for rate limit retry-after header.
	retryAfterSeconds := 0
	isUsageLimit := false
	var rateLimitErr *RateLimitError
	if errors.As(err, &rateLimitErr) {
		retryAfterSeconds = rateLimitErr.RetryAfterSeconds
		isUsageLimit = isUsageLimitError(rateLimitErr)
	}

	backoffSeconds := 0
	if retryAfterSeconds > 0 {
		backoffSeconds = retryAfterSeconds
		if isUsageLimit {
			backoffSeconds += usageLimitRetryBufferSeconds
		}
		return time.Duration(backoffSeconds) * time.Second
	}

	if backoffSeconds <= 0 {
		backoffSeconds = 1 << (attempt - 1)
	}

	backoffDuration := time.Duration(backoffSeconds) * r.policy.InitialDelay

	if retryAfterSeconds <= 0 && backoffDuration > r.policy.MaxDelay {
		backoffDuration = r.policy.MaxDelay
	}

	return backoffDuration
}

// logMessage emits a log message if the logging function is configured.
func (r *RetryChatModel) logMessage(message string, msgType shared.MessageType) {
	if r.logFn != nil {
		r.logFn(message, msgType)
	}
}

// isUsageLimitError checks if the rate limit error is a usage limit error.
func isUsageLimitError(err *RateLimitError) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Message), "usage limit")
}

// buildRateLimitResetMessage creates a readable message with expected reset time.
func buildRateLimitResetMessage(rateLimitErr *RateLimitError) string {
	if rateLimitErr == nil || rateLimitErr.RetryAfterSeconds <= 0 {
		return ""
	}

	resetTime := time.Now().Add(time.Duration(rateLimitErr.RetryAfterSeconds) * time.Second)
	resetAt := resetTime.Format("2006-01-02 15:04:05 MST")
	messageLower := strings.ToLower(rateLimitErr.Message)

	if strings.Contains(messageLower, "usage limit") {
		return fmt.Sprintf("Usage limit has been reached. Reset expected at %s (in %d seconds).", resetAt, rateLimitErr.RetryAfterSeconds)
	}

	return fmt.Sprintf("Rate limit has been reached. Reset expected at %s (in %d seconds).", resetAt, rateLimitErr.RetryAfterSeconds)
}

// isRetryableError returns true when an error indicates a temporary condition
// that may resolve on retry.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var rateLimitErr *RateLimitError
	if errors.As(err, &rateLimitErr) {
		return true
	}

	var networkErr *NetworkError
	if errors.As(err, &networkErr) {
		return networkErr.IsRetryable
	}

	if errors.Is(err, ErrEndpointUnavailable) {
		return true
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	return false
}
