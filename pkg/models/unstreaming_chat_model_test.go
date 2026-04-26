package models

import (
	"context"
	"errors"
	"testing"

	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnstreamingChatModel_ChatStream_Passthrough verifies that ChatStream passes
// the request to the wrapped model and returns results as-is.
func TestUnstreamingChatModel_ChatStream_Passthrough(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	// Setup streaming response
	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			NewTextMessage(ChatRoleAssistant, "Hello"),
			NewTextMessage(ChatRoleAssistant, ", "),
			NewTextMessage(ChatRoleAssistant, "world"),
			NewTextMessage(ChatRoleAssistant, "!"),
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test message"),
	}
	options := &ChatOptions{Temperature: 0.7}
	tools := []tool.ToolInfo{{Name: "test_tool"}}

	// Call ChatStream
	iter := unstreaming.ChatStream(ctx, messages, options, tools)

	// Collect all fragments
	var fragments []*ChatMessage
	for msg := range iter {
		fragments = append(fragments, msg)
	}

	// Verify fragments
	require.Len(t, fragments, 4)
	assert.Equal(t, "Hello", fragments[0].GetText())
	assert.Equal(t, ", ", fragments[1].GetText())
	assert.Equal(t, "world", fragments[2].GetText())
	assert.Equal(t, "!", fragments[3].GetText())

	// Verify messages were recorded on wrapped model
	require.Len(t, provider.RecordedMessages, 1)
	assert.Equal(t, "test message", provider.RecordedMessages[0][0].GetText())
}

// TestUnstreamingChatModel_ChatStream_EmptyStream verifies behavior when wrapped
// model returns empty stream.
func TestUnstreamingChatModel_ChatStream_EmptyStream(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	// Setup empty streaming response (error case)
	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		Error: errors.New("stream error"),
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	iter := unstreaming.ChatStream(ctx, messages, nil, nil)

	var fragments []*ChatMessage
	for msg := range iter {
		fragments = append(fragments, msg)
	}

	assert.Empty(t, fragments)
}

// TestUnstreamingChatModel_ChatStream_ContextCancellation verifies that context
// cancellation is properly propagated.
func TestUnstreamingChatModel_ChatStream_ContextCancellation(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			NewTextMessage(ChatRoleAssistant, "fragment 1"),
			NewTextMessage(ChatRoleAssistant, "fragment 2"),
			NewTextMessage(ChatRoleAssistant, "fragment 3"),
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx, cancel := context.WithCancel(context.Background())

	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	iter := unstreaming.ChatStream(ctx, messages, nil, nil)

	// Read first fragment then cancel
	fragmentCount := 0
	for msg := range iter {
		if msg != nil {
			fragmentCount++
			cancel()
			break
		}
	}

	assert.Equal(t, 1, fragmentCount)
}

// TestUnstreamingChatModel_InterfaceCompliance verifies that UnstreamingChatModel
// implements the ChatModel interface.
func TestUnstreamingChatModel_InterfaceCompliance(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})
	wrapped := provider.ChatModel("test-model", nil)

	var _ ChatModel = (*UnstreamingChatModel)(nil)
	_ = NewUnstreamingChatModel(wrapped)
}

// TestUnstreamingChatModel_ChatStream_PreservesNilOptions verifies that
// ChatStream works correctly when options is nil.
func TestUnstreamingChatModel_ChatStream_PreservesNilOptions(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			NewTextMessage(ChatRoleAssistant, "response"),
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	iter := unstreaming.ChatStream(ctx, messages, nil, nil)

	var fragments []*ChatMessage
	for msg := range iter {
		fragments = append(fragments, msg)
	}

	require.Len(t, fragments, 1)
	assert.Equal(t, "response", fragments[0].GetText())
}

// TestUnstreamingChatModel_ChatStream_EarlyBreak verifies that breaking
// from the iterator works correctly.
func TestUnstreamingChatModel_ChatStream_EarlyBreak(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			NewTextMessage(ChatRoleAssistant, "fragment 1"),
			NewTextMessage(ChatRoleAssistant, "fragment 2"),
			NewTextMessage(ChatRoleAssistant, "fragment 3"),
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	iter := unstreaming.ChatStream(ctx, messages, nil, nil)

	// Break after first fragment
	fragmentCount := 0
	for msg := range iter {
		if msg != nil {
			fragmentCount++
			break
		}
	}

	assert.Equal(t, 1, fragmentCount)
}
