package models

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- Delay calculation tests ---

func TestRetryChatModel_CalculateDelay_ExponentialBackoff(t *testing.T) {
	policy := RetryPolicy{
		InitialDelay: 10 * time.Second,
		MaxRetries:   10,
		MaxDelay:     600 * time.Second,
	}

	model := NewRetryChatModel(nil, &policy, nil)

	err := &RateLimitError{Message: "rate exceeded"}

	// Attempt 1: 2^0 * 10s = 10s
	assert.Equal(t, 10*time.Second, model.calculateDelay(1, err))
	// Attempt 2: 2^1 * 10s = 20s
	assert.Equal(t, 20*time.Second, model.calculateDelay(2, err))
	// Attempt 3: 2^2 * 10s = 40s
	assert.Equal(t, 40*time.Second, model.calculateDelay(3, err))
	// Attempt 4: 2^3 * 10s = 80s
	assert.Equal(t, 80*time.Second, model.calculateDelay(4, err))
	// Attempt 5: 2^4 * 10s = 160s
	assert.Equal(t, 160*time.Second, model.calculateDelay(5, err))
	// Attempt 6: 2^5 * 10s = 320s
	assert.Equal(t, 320*time.Second, model.calculateDelay(6, err))
	// Attempt 7: 2^6 * 10s = 640s, but capped at 600s
	assert.Equal(t, 600*time.Second, model.calculateDelay(7, err))
}

func TestRetryChatModel_CalculateDelay_RateLimitRetryAfter(t *testing.T) {
	policy := RetryPolicy{
		InitialDelay: 10 * time.Second,
		MaxRetries:   10,
		MaxDelay:     600 * time.Second,
	}

	model := NewRetryChatModel(nil, &policy, nil)

	err := &RateLimitError{RetryAfterSeconds: 30, Message: "rate exceeded"}

	// When RetryAfterSeconds is set, use it directly as seconds.
	assert.Equal(t, 30*time.Second, model.calculateDelay(1, err))
}

func TestRetryChatModel_CalculateDelay_UsageLimitRetryAfter(t *testing.T) {
	policy := RetryPolicy{
		InitialDelay: 10 * time.Second,
		MaxRetries:   10,
		MaxDelay:     600 * time.Second,
	}

	model := NewRetryChatModel(nil, &policy, nil)

	err := &RateLimitError{RetryAfterSeconds: 30, Message: "Usage limit reached"}

	// Usage limit adds safety buffer: 30s + 10s = 40s.
	assert.Equal(t, 40*time.Second, model.calculateDelay(1, err))
}

func TestRetryChatModel_CalculateDelay_NetworkErrorUsesExponentialBackoff(t *testing.T) {
	policy := RetryPolicy{
		InitialDelay: 10 * time.Second,
		MaxRetries:   10,
		MaxDelay:     600 * time.Second,
	}

	model := NewRetryChatModel(nil, &policy, nil)

	err := &NetworkError{Message: "connection reset", IsRetryable: true}

	assert.Equal(t, 10*time.Second, model.calculateDelay(1, err))
	assert.Equal(t, 20*time.Second, model.calculateDelay(2, err))
}

// --- isRetryableError tests ---

func TestIsRetryableError(t *testing.T) {
	t.Run("rate limit error is retryable", func(t *testing.T) {
		assert.True(t, isRetryableError(&RateLimitError{Message: "rate exceeded"}))
	})

	t.Run("retryable network error is retryable", func(t *testing.T) {
		assert.True(t, isRetryableError(&NetworkError{Message: "conn reset", IsRetryable: true}))
	})

	t.Run("non-retryable network error is not retryable", func(t *testing.T) {
		assert.False(t, isRetryableError(&NetworkError{Message: "fatal", IsRetryable: false}))
	})

	t.Run("ErrEndpointUnavailable is retryable", func(t *testing.T) {
		assert.True(t, isRetryableError(ErrEndpointUnavailable))
	})

	t.Run("io.EOF is retryable", func(t *testing.T) {
		assert.True(t, isRetryableError(io.EOF))
	})

	t.Run("io.ErrUnexpectedEOF is retryable", func(t *testing.T) {
		assert.True(t, isRetryableError(io.ErrUnexpectedEOF))
	})

	t.Run("net.Error timeout is retryable", func(t *testing.T) {
		assert.True(t, isRetryableError(&testNetError{msg: "timeout", timeout: true}))
	})

	t.Run("net.Error temporary is retryable", func(t *testing.T) {
		assert.True(t, isRetryableError(&testNetError{msg: "temp", temporary: true}))
	})

	t.Run("generic net.Error is not retryable", func(t *testing.T) {
		assert.False(t, isRetryableError(&testNetError{msg: "generic"}))
	})

	t.Run("generic error is not retryable", func(t *testing.T) {
		assert.False(t, isRetryableError(errors.New("generic error")))
	})

	t.Run("nil error is not retryable", func(t *testing.T) {
		assert.False(t, isRetryableError(nil))
	})

	t.Run("wrapped rate limit error is retryable", func(t *testing.T) {
		wrapped := fmt.Errorf("wrapped: %w", &RateLimitError{Message: "rate exceeded"})
		assert.True(t, isRetryableError(wrapped))
	})

	t.Run("ErrEndpointNotFound is not retryable", func(t *testing.T) {
		assert.False(t, isRetryableError(ErrEndpointNotFound))
	})

	t.Run("ErrPermissionDenied is not retryable", func(t *testing.T) {
		assert.False(t, isRetryableError(ErrPermissionDenied))
	})

	t.Run("ErrTooManyInputTokens is not retryable", func(t *testing.T) {
		assert.False(t, isRetryableError(ErrTooManyInputTokens))
	})
}

// --- isUsageLimitError tests ---

func TestIsUsageLimitError(t *testing.T) {
	t.Run("usage limit message", func(t *testing.T) {
		assert.True(t, isUsageLimitError(&RateLimitError{Message: "Usage limit reached"}))
	})

	t.Run("usage limit lowercase", func(t *testing.T) {
		assert.True(t, isUsageLimitError(&RateLimitError{Message: "usage limit exceeded"}))
	})

	t.Run("usage limit mixed case", func(t *testing.T) {
		assert.True(t, isUsageLimitError(&RateLimitError{Message: "Usage Limit Exceeded"}))
	})

	t.Run("regular rate limit", func(t *testing.T) {
		assert.False(t, isUsageLimitError(&RateLimitError{Message: "rate limit exceeded"}))
	})

	t.Run("nil error", func(t *testing.T) {
		assert.False(t, isUsageLimitError(nil))
	})

	t.Run("empty message", func(t *testing.T) {
		assert.False(t, isUsageLimitError(&RateLimitError{Message: ""}))
	})
}

// --- RetryPolicy tests ---

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()
	assert.Equal(t, DefaultRetryBackoffScale, policy.InitialDelay)
	assert.Equal(t, DefaultMaxRetries, policy.MaxRetries)
	assert.Equal(t, 60*DefaultRetryBackoffScale, policy.MaxDelay)
}
