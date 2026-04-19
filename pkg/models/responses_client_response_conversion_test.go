package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertFromResponsesStreamBody_ExtendsScannerBufferForLargeTokens(t *testing.T) {
	longDelta := strings.Repeat("a", 70*1024)
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"" + longDelta + "\"}\n\n" +
		"data: [DONE]\n\n"

	message, err := convertFromResponsesStreamBody([]byte(body))
	require.NoError(t, err)
	require.NotNil(t, message)
	assert.Equal(t, longDelta, message.GetText())
}

func TestConvertFromResponsesStreamBody_HandlesStreamErrorEvent(t *testing.T) {
	t.Run("returns ErrTooManyInputTokens for context_length_exceeded", func(t *testing.T) {
		body := strings.Join([]string{
			"event: error",
			"data: {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"code\":\"context_length_exceeded\",\"message\":\"Your input exceeds the context window of this model. Please adjust your input and try again.\",\"param\":\"input\"},\"sequence_number\":2}",
			"",
		}, "\n")

		_, err := convertFromResponsesStreamBody([]byte(body))
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTooManyInputTokens)
	})

	t.Run("returns APIRequestError with details for non-context stream errors", func(t *testing.T) {
		body := strings.Join([]string{
			"event: error",
			"data: {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"code\":\"unsupported_parameter\",\"message\":\"Unsupported parameter: foo\",\"param\":\"foo\"},\"sequence_number\":2}",
			"",
		}, "\n")

		_, err := convertFromResponsesStreamBody([]byte(body))
		require.Error(t, err)

		var reqErr *APIRequestError
		require.ErrorAs(t, err, &reqErr)
		assert.Equal(t, "invalid_request_error", reqErr.ErrorType)
		assert.Equal(t, "unsupported_parameter", reqErr.Code)
		assert.Equal(t, "foo", reqErr.Param)
		assert.Equal(t, "Unsupported parameter: foo", reqErr.Message)
		assert.Contains(t, err.Error(), "invalid_request_error")
		assert.Contains(t, err.Error(), "unsupported_parameter")
		assert.Contains(t, err.Error(), "foo")
	})

	t.Run("returns RateLimitError for overloaded service", func(t *testing.T) {
		body := strings.Join([]string{
			"event: error",
			"data: {\"type\":\"error\",\"error\":{\"type\":\"service_unavailable_error\",\"code\":\"server_is_overloaded\",\"message\":\"Our servers are currently overloaded. Please try again later.\",\"param\":null},\"sequence_number\":2}",
			"",
		}, "\n")

		_, err := convertFromResponsesStreamBody([]byte(body))
		require.Error(t, err)

		var rateLimitErr *RateLimitError
		require.ErrorAs(t, err, &rateLimitErr)
		assert.Equal(t, 60, rateLimitErr.RetryAfterSeconds)
		assert.Equal(t, "Our servers are currently overloaded. Please try again later.", rateLimitErr.Message)
		assert.ErrorIs(t, err, ErrRateExceeded)
	})

	t.Run("returns retryable NetworkError for transient server error", func(t *testing.T) {
		body := strings.Join([]string{
			"event: error",
			"data: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"An error occurred while processing your request. You can retry your request.\",\"param\":null},\"sequence_number\":2}",
			"",
		}, "\n")

		_, err := convertFromResponsesStreamBody([]byte(body))
		require.Error(t, err)

		var networkErr *NetworkError
		require.ErrorAs(t, err, &networkErr)
		assert.True(t, networkErr.IsRetryable)
		assert.Equal(t, "An error occurred while processing your request. You can retry your request.", networkErr.Message)
		assert.ErrorIs(t, err, ErrNetworkError)
	})

	t.Run("returns retryable NetworkError for response.incomplete with max_output_tokens", func(t *testing.T) {
		body := strings.Join([]string{
			"event: response.incomplete",
			"data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_123\",\"object\":\"response\",\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"}},\"sequence_number\":4}",
			"",
		}, "\n")

		_, err := convertFromResponsesStreamBody([]byte(body))
		require.Error(t, err)

		var networkErr *NetworkError
		require.ErrorAs(t, err, &networkErr)
		assert.True(t, networkErr.IsRetryable)
		assert.Equal(t, "response incomplete: max_output_tokens", networkErr.Message)
		assert.ErrorIs(t, err, ErrNetworkError)
	})
}
