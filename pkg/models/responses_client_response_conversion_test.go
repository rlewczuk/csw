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

	t.Run("returns retryable NetworkError for empty response with only reasoning", func(t *testing.T) {
		body := strings.Join([]string{
			"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_01cfa8c42ec39bc90169f5c0dc25ec819183f7f1a1bff4f223\",\"object\":\"response\",\"created_at\":1777713372,\"status\":\"in_progress\"},\"sequence_number\":0}",
			"",
			"event: response.in_progress",
			"data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_01cfa8c42ec39bc90169f5c0dc25ec819183f7f1a1bff4f223\",\"object\":\"response\",\"created_at\":1777713372,\"status\":\"in_progress\"},\"sequence_number\":1}",
			"",
			"event: response.output_item.added",
			"data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"rs_01cfa8c42ec39bc90169f5c0dd880881919f17df218220e549\",\"type\":\"reasoning\",\"summary\":[]},\"output_index\":0,\"sequence_number\":2}",
			"",
			"event: response.output_item.done",
			"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"rs_01cfa8c42ec39bc90169f5c0dd880881919f17df218220e549\",\"type\":\"reasoning\",\"summary\":[]},\"output_index\":0,\"sequence_number\":3}",
			"",
			"event: response.output_item.added",
			"data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"msg_01cfa8c42ec39bc90169f5c0e6da108191a10d7c7f1858fc82\",\"type\":\"message\",\"status\":\"in_progress\",\"content\":[],\"phase\":\"final_answer\",\"role\":\"assistant\"},\"output_index\":1,\"sequence_number\":4}",
			"",
			"event: response.content_part.added",
			"data: {\"type\":\"response.content_part.added\",\"content_index\":0,\"item_id\":\"msg_01cfa8c42ec39bc90169f5c0e6da108191a10d7c7f1858fc82\",\"output_index\":1,\"part\":{\"type\":\"output_text\",\"annotations\":[],\"logprobs\":[],\"text\":\"\"},\"sequence_number\":5}",
			"",
			"event: response.output_text.done",
			"data: {\"type\":\"response.output_text.done\",\"content_index\":0,\"item_id\":\"msg_01cfa8c42ec39bc90169f5c0e6da108191a10d7c7f1858fc82\",\"logprobs\":[],\"output_index\":1,\"sequence_number\":6,\"text\":\"\"}",
			"",
			"event: response.content_part.done",
			"data: {\"type\":\"response.content_part.done\",\"content_index\":0,\"item_id\":\"msg_01cfa8c42ec39bc90169f5c0e6da108191a10d7c7f1858fc82\",\"output_index\":1,\"part\":{\"type\":\"output_text\",\"annotations\":[],\"logprobs\":[],\"text\":\"\"},\"sequence_number\":7}",
			"",
			"event: response.output_item.done",
			"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"msg_01cfa8c42ec39bc90169f5c0e6da108191a10d7c7f1858fc82\",\"type\":\"message\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"annotations\":[],\"logprobs\":[],\"text\":\"\"}],\"phase\":\"final_answer\",\"role\":\"assistant\"},\"output_index\":1,\"sequence_number\":8}",
			"",
			"event: response.completed",
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_01cfa8c42ec39bc90169f5c0dc25ec819183f7f1a1bff4f223\",\"object\":\"response\",\"created_at\":1777713372,\"status\":\"completed\",\"completed_at\":1777713382,\"usage\":{\"input_tokens\":119540,\"input_tokens_details\":{\"cached_tokens\":0},\"output_tokens\":522,\"output_tokens_details\":{\"reasoning_tokens\":516},\"total_tokens\":120062}},\"sequence_number\":9}",
			"",
		}, "\n")

		_, err := convertFromResponsesStreamBody([]byte(body))
		require.Error(t, err)

		var networkErr *NetworkError
		require.ErrorAs(t, err, &networkErr)
		assert.True(t, networkErr.IsRetryable)
		assert.Contains(t, networkErr.Message, "no usable output items in response")
		assert.ErrorIs(t, err, ErrNetworkError)
	})
}
