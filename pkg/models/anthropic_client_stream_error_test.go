package models

import (
	"context"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnthropicChatStream_SilentErrorOnBadRequest tests that ChatStream properly handles errors
// instead of silently returning with zero fragments when an API error occurs.
// This test reproduces the issue where CLI commands exit without any output when the model
// provider returns an error (e.g., 400 Bad Request).
func TestAnthropicChatStream_SilentErrorOnBadRequest(t *testing.T) {
	// Create mock server
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	// Add a 400 Bad Request response
	mock.AddRestResponseWithStatus("/v1/messages", "POST", `{"type":"error","error":{"type":"invalid_request_error","message":"messages: roles must alternate between \"user\" and \"assistant\""}}`, 400)

	client, err := NewAnthropicClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	chatModel := client.ChatModel(testAnthropicModelName, nil)

	messages := []*ChatMessage{
		{
			Role:  ChatRoleUser,
			Parts: []ChatMessagePart{{Text: "Hello"}},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	iterator := chatModel.ChatStream(ctx, messages, nil, nil)
	require.NotNil(t, iterator)

	var fragments []*ChatMessage
	for fragment := range iterator {
		fragments = append(fragments, fragment)
	}

	// BUG: Currently ChatStream silently returns with zero fragments on error
	// This causes the session to exit without any indication of what went wrong
	// Expected behavior: Should either yield an error fragment or panic with a clear error message
	assert.Equal(t, 0, len(fragments), "Currently ChatStream silently returns zero fragments on API error - this is the bug we need to fix")

	// TODO: After fixing, change this test to verify proper error handling
	// Option 1: Stream should yield a fragment with error information
	// Option 2: Stream should panic/log the error in a way that's visible to the caller
}

// TestAnthropicChatStream_SilentErrorOnUnauthorized tests unauthorized access handling
func TestAnthropicChatStream_SilentErrorOnUnauthorized(t *testing.T) {
	// Create mock server
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	// Add a 401 Unauthorized response
	mock.AddRestResponseWithStatus("/v1/messages", "POST", `{"type":"error","error":{"type":"authentication_error","message":"Invalid API key"}}`, 401)

	client, err := NewAnthropicClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	chatModel := client.ChatModel(testAnthropicModelName, nil)

	messages := []*ChatMessage{
		{
			Role:  ChatRoleUser,
			Parts: []ChatMessagePart{{Text: "Hello"}},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	iterator := chatModel.ChatStream(ctx, messages, nil, nil)
	require.NotNil(t, iterator)

	var fragments []*ChatMessage
	for fragment := range iterator {
		fragments = append(fragments, fragment)
	}

	// BUG: Same issue - silently returns zero fragments
	assert.Equal(t, 0, len(fragments), "Currently ChatStream silently returns zero fragments on auth error - this is the bug we need to fix")
}

// TestAnthropicChatStream_SilentErrorOnRateLimit tests rate limit handling
func TestAnthropicChatStream_SilentErrorOnRateLimit(t *testing.T) {
	// Create mock server
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	// Add a 429 Rate Limit response
	mock.AddRestResponseWithStatus("/v1/messages", "POST", `{"type":"error","error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`, 429)

	client, err := NewAnthropicClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	chatModel := client.ChatModel(testAnthropicModelName, nil)

	messages := []*ChatMessage{
		{
			Role:  ChatRoleUser,
			Parts: []ChatMessagePart{{Text: "Hello"}},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	iterator := chatModel.ChatStream(ctx, messages, nil, nil)
	require.NotNil(t, iterator)

	var fragments []*ChatMessage
	for fragment := range iterator {
		fragments = append(fragments, fragment)
	}

	// BUG: Same issue - silently returns zero fragments
	assert.Equal(t, 0, len(fragments), "Currently ChatStream silently returns zero fragments on rate limit - this is the bug we need to fix")
}
