// session_core_integ_retry_test.go contains integration tests for
// SweSession retry behavior for rate-limit and network failures.
package core

import (
	"context"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweSession_RateLimitRetry(t *testing.T) {
	t.Run("retries on rate limit error with retry-after header", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})

		rateLimitErr := &models.RateLimitError{
			RetryAfterSeconds: 0,
			Message:           "rate limit exceeded",
		}

		mockProvider.RateLimitError = rateLimitErr
		rateLimitCount := 1
		mockProvider.RateLimitErrorCount = &rateLimitCount
		mockProvider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 1}

		successResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{Text: "Success after retry!"},
				},
			},
		}
		mockProvider.SetChatResponse("test-model", successResponse)

		fixture := newSweSystemFixture(t, "You are a helpful assistant.",
			withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
			withGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 1}),
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := fixture.system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)
		session.llmRetryPolicyOverride = &models.RetryPolicy{InitialDelay: time.Millisecond, MaxRetries: 1, MaxDelay: time.Millisecond}

		session.UserPrompt("Hello")

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		err = session.Run(ctx)
		cancel()

		assert.NoError(t, err, "Session should complete successfully after retry")

		assert.Equal(t, 2, len(mockProvider.RecordedMessages), "Should have made 2 LLM calls (1 rate limit + 1 success)")

		if len(mockHandler.RateLimitErrors) > 0 {
			t.Logf("Rate limit errors received: %v", mockHandler.RateLimitErrors)
		}
	})

	t.Run("retries on rate limit error without retry-after using exponential backoff", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})

		rateLimitErr := &models.RateLimitError{
			RetryAfterSeconds: 0,
			Message:           "rate limit exceeded",
		}

		mockProvider.RateLimitError = rateLimitErr
		rateLimitCount := 1
		mockProvider.RateLimitErrorCount = &rateLimitCount
		mockProvider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 1}

		successResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{Text: "Success after retry!"},
				},
			},
		}
		mockProvider.SetChatResponse("test-model", successResponse)

		fixture := newSweSystemFixture(t, "You are a helpful assistant.",
			withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
			withGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 1}),
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := fixture.system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)
		session.llmRetryPolicyOverride = &models.RetryPolicy{InitialDelay: time.Millisecond, MaxRetries: 1, MaxDelay: time.Millisecond}

		session.UserPrompt("Hello")

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		err = session.Run(ctx)
		cancel()

		assert.NoError(t, err, "Session should complete successfully after exponential backoff retry")
		assert.Equal(t, 2, len(mockProvider.RecordedMessages), "Should have made 2 LLM calls")
	})

	t.Run("fails after max retries exceeded", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})

		rateLimitErr := &models.RateLimitError{
			RetryAfterSeconds: 0,
			Message:           "rate limit exceeded",
		}

		mockProvider.RateLimitError = rateLimitErr
		rateLimitCount := 2
		mockProvider.RateLimitErrorCount = &rateLimitCount
		mockProvider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 1}

		fixture := newSweSystemFixture(t, "You are a helpful assistant.",
			withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
			withGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 1}),
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := fixture.system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)
		session.llmRetryPolicyOverride = &models.RetryPolicy{InitialDelay: time.Millisecond, MaxRetries: 1, MaxDelay: time.Millisecond}

		session.UserPrompt("Hello")

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		err = session.Run(ctx)
		cancel()

		require.Error(t, err, "Session should fail after max retries")
		assert.Contains(t, err.Error(), "rate limit exceeded")
	})
}

func TestSweSession_NetworkErrorRetry(t *testing.T) {
	t.Run("retries on network error using exponential backoff", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})

		networkErr := &models.NetworkError{
			Message:     "connection refused",
			IsRetryable: true,
		}

		mockProvider.NetworkError = networkErr
		networkErrorCount := 1
		mockProvider.NetworkErrorCount = &networkErrorCount
		mockProvider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 1}

		successResponse := &models.MockChatResponse{
			Response: &models.ChatMessage{
				Role: models.ChatRoleAssistant,
				Parts: []models.ChatMessagePart{
					{Text: "Success after retry!"},
				},
			},
		}
		mockProvider.SetChatResponse("test-model", successResponse)

		fixture := newSweSystemFixture(t, "You are a helpful assistant.",
			withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
			withGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 1}),
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := fixture.system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)
		session.llmRetryPolicyOverride = &models.RetryPolicy{InitialDelay: time.Millisecond, MaxRetries: 1, MaxDelay: time.Millisecond}

		session.UserPrompt("Hello")

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		err = session.Run(ctx)
		cancel()

		assert.NoError(t, err, "Session should complete successfully after retry")
		assert.Equal(t, 2, len(mockProvider.RecordedMessages), "Should have made 2 LLM calls (1 network error + 1 success)")

		if len(mockHandler.RateLimitErrors) > 0 {
			t.Logf("Network retry notifications received: %v", mockHandler.RateLimitErrors)
		}
	})

	t.Run("fails after max retries exceeded for network error", func(t *testing.T) {
		mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})

		networkErr := &models.NetworkError{
			Message:     "connection refused",
			IsRetryable: true,
		}

		mockProvider.NetworkError = networkErr
		networkErrorCount := 2
		mockProvider.NetworkErrorCount = &networkErrorCount
		mockProvider.Config = &conf.ModelProviderConfig{RateLimitBackoffScale: 1}

		fixture := newSweSystemFixture(t, "You are a helpful assistant.",
			withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
			withGlobalConfig(&conf.GlobalConfig{LLMRetryMaxAttempts: 2, LLMRetryMaxBackoffSeconds: 1}),
		)

		mockHandler := testutil.NewMockSessionOutputHandler()
		session, err := fixture.system.NewSession("mock/test-model", mockHandler)
		require.NoError(t, err)
		session.llmRetryPolicyOverride = &models.RetryPolicy{InitialDelay: time.Millisecond, MaxRetries: 1, MaxDelay: time.Millisecond}

		session.UserPrompt("Hello")

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		err = session.Run(ctx)
		cancel()

		require.Error(t, err, "Session should fail after max retries")
		assert.Contains(t, err.Error(), "temporary LLM API failure")
	})
}
