package models

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetworkError(t *testing.T) {
	t.Run("Error returns message", func(t *testing.T) {
		err := &NetworkError{
			Message:     "connection refused",
			IsRetryable: true,
		}
		assert.Equal(t, "connection refused", err.Error())
	})

	t.Run("Error returns default message when empty", func(t *testing.T) {
		err := &NetworkError{
			IsRetryable: true,
		}
		assert.Equal(t, "network error", err.Error())
	})

	t.Run("Unwrap returns ErrNetworkError", func(t *testing.T) {
		err := &NetworkError{
			Message:     "test error",
			IsRetryable: true,
		}
		assert.ErrorIs(t, err, ErrNetworkError)
	})
}

func TestOpenAIClient_HandleHTTPError(t *testing.T) {
	client := &OpenAIClient{}

	t.Run("returns NetworkError for net.Error timeout", func(t *testing.T) {
		netErr := &testNetError{msg: "timeout", timeout: true}
		err := client.handleHTTPError(netErr)

		var networkErr *NetworkError
		assert.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
		assert.Contains(t, networkErr.Message, "timeout")
	})

	t.Run("returns NetworkError for net.Error temporary", func(t *testing.T) {
		netErr := &testNetError{msg: "temporary", temporary: true}
		err := client.handleHTTPError(netErr)

		var networkErr *NetworkError
		assert.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
	})

	t.Run("returns NetworkError for net.Error general", func(t *testing.T) {
		netErr := &testNetError{msg: "connection refused"}
		err := client.handleHTTPError(netErr)

		var networkErr *NetworkError
		assert.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
	})

	t.Run("returns NetworkError for OpError dial", func(t *testing.T) {
		opErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
		err := client.handleHTTPError(opErr)

		var networkErr *NetworkError
		assert.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
	})

	t.Run("returns NetworkError for OpError read", func(t *testing.T) {
		opErr := &net.OpError{Op: "read", Err: errors.New("read error")}
		err := client.handleHTTPError(opErr)

		var networkErr *NetworkError
		assert.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
	})

	t.Run("returns NetworkError for temporary DNS error", func(t *testing.T) {
		dnsErr := &net.DNSError{
			Err:         "temporary DNS failure",
			Name:        "example.com",
			IsTemporary: true,
		}
		err := client.handleHTTPError(dnsErr)

		var networkErr *NetworkError
		assert.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
	})

	t.Run("returns ErrEndpointNotFound for permanent DNS not found error", func(t *testing.T) {
		dnsErr := &net.DNSError{
			Err:         "no such host",
			Name:        "example.com",
			IsNotFound:  true,
			IsTemporary: false,
		}
		err := client.handleHTTPError(dnsErr)

		// Permanent DNS not found errors should wrap ErrEndpointNotFound
		assert.ErrorIs(t, err, ErrEndpointNotFound)
	})

	t.Run("returns NetworkError for other DNS errors", func(t *testing.T) {
		dnsErr := &net.DNSError{
			Err:         "server misbehaving",
			Name:        "example.com",
			IsTemporary: false,
			IsNotFound:  false,
		}
		err := client.handleHTTPError(dnsErr)

		var networkErr *NetworkError
		assert.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		err := client.handleHTTPError(nil)
		assert.Nil(t, err)
	})
}

func TestOllamaClient_HandleHTTPError(t *testing.T) {
	client := &OllamaClient{}

	t.Run("returns NetworkError for net.Error timeout", func(t *testing.T) {
		netErr := &testNetError{msg: "timeout", timeout: true}
		err := client.handleHTTPError(netErr)

		var networkErr *NetworkError
		assert.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
	})
}

func TestAnthropicClient_HandleHTTPError(t *testing.T) {
	client := &AnthropicClient{}

	t.Run("returns NetworkError for net.Error timeout", func(t *testing.T) {
		netErr := &testNetError{msg: "timeout", timeout: true}
		err := client.handleHTTPError(netErr)

		var networkErr *NetworkError
		assert.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
	})
}

func TestResponsesClient_HandleHTTPError(t *testing.T) {
	client := &ResponsesClient{}

	t.Run("returns NetworkError for net.Error timeout", func(t *testing.T) {
		netErr := &testNetError{msg: "timeout", timeout: true}
		err := client.handleHTTPError(netErr)

		var networkErr *NetworkError
		assert.True(t, errors.As(err, &networkErr))
		assert.True(t, networkErr.IsRetryable)
	})
}

// testNetError is a mock implementation of net.Error for testing
type testNetError struct {
	msg       string
	timeout   bool
	temporary bool
}

func (e *testNetError) Error() string   { return e.msg }
func (e *testNetError) Timeout() bool   { return e.timeout }
func (e *testNetError) Temporary() bool { return e.temporary }
