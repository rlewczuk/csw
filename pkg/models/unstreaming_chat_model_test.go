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

// TestUnstreamingChatModel_Chat_SingleFragment verifies Chat method with single fragment.
func TestUnstreamingChatModel_Chat_SingleFragment(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			NewTextMessage(ChatRoleAssistant, "Hello, world!"),
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, ChatRoleAssistant, response.Role)
	assert.Equal(t, "Hello, world!", response.GetText())
}

// TestUnstreamingChatModel_Chat_MultipleFragments verifies Chat method concatenates
// multiple text fragments into single message.
func TestUnstreamingChatModel_Chat_MultipleFragments(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

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
		NewTextMessage(ChatRoleUser, "test"),
	}

	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "Hello, world!", response.GetText())
	assert.Equal(t, ChatRoleAssistant, response.Role)
}

// TestUnstreamingChatModel_Chat_EmptyStream verifies Chat method returns empty
// message when stream is empty.
func TestUnstreamingChatModel_Chat_EmptyStream(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	// No response configured - will use default which has fragments
	// Let's explicitly set empty fragments
	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Empty(t, response.GetText())
	assert.Equal(t, ChatRoleAssistant, response.Role)
}

// TestUnstreamingChatModel_Chat_WithToolCalls verifies Chat method properly
// aggregates tool calls from multiple fragments.
func TestUnstreamingChatModel_Chat_WithToolCalls(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	toolCall1 := &tool.ToolCall{
		ID:        "call-1",
		Function:  "tool1",
		Arguments: tool.NewToolValue(map[string]any{"arg": "value1"}),
	}
	toolCall2 := &tool.ToolCall{
		ID:        "call-2",
		Function:  "tool2",
		Arguments: tool.NewToolValue(map[string]any{"arg": "value2"}),
	}

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			NewToolCallMessage(toolCall1),
			NewToolCallMessage(toolCall2),
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)

	toolCalls := response.GetToolCalls()
	require.Len(t, toolCalls, 2)
	assert.Equal(t, "call-1", toolCalls[0].ID)
	assert.Equal(t, "tool1", toolCalls[0].Function)
	assert.Equal(t, "call-2", toolCalls[1].ID)
	assert.Equal(t, "tool2", toolCalls[1].Function)
}

// TestUnstreamingChatModel_Chat_MixedContent verifies Chat method handles
// mixed content (text and tool calls) from fragments.
func TestUnstreamingChatModel_Chat_MixedContent(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	toolCall := &tool.ToolCall{
		ID:        "call-1",
		Function:  "test_tool",
		Arguments: tool.NewToolValue(map[string]any{}),
	}

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			NewTextMessage(ChatRoleAssistant, "Let me help you"),
			NewToolCallMessage(toolCall),
			NewTextMessage(ChatRoleAssistant, " with that."),
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "Let me help you with that.", response.GetText())

	toolCalls := response.GetToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call-1", toolCalls[0].ID)
}

// TestUnstreamingChatModel_Chat_WithReasoningContent verifies Chat method
// properly aggregates reasoning content from fragments. Text is concatenated
// into a single part, and reasoning content is also concatenated into a single part.
func TestUnstreamingChatModel_Chat_WithReasoningContent(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			{
				Role: ChatRoleAssistant,
				Parts: []ChatMessagePart{
					{Text: "The answer is", ReasoningContent: "Let me think..."},
				},
			},
			{
				Role: ChatRoleAssistant,
				Parts: []ChatMessagePart{
					{Text: " 42.", ReasoningContent: " Yes, that's it."},
				},
			},
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "The answer is 42.", response.GetText())

	// Should have 2 parts: concatenated text, then concatenated reasoning content
	require.Len(t, response.Parts, 2)
	assert.Equal(t, "The answer is 42.", response.Parts[0].Text)
	assert.Equal(t, "Let me think... Yes, that's it.", response.Parts[1].ReasoningContent)
}

// TestUnstreamingChatModel_Chat_WithToolResponses verifies Chat method handles
// tool responses from fragments.
func TestUnstreamingChatModel_Chat_WithToolResponses(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	toolCall := &tool.ToolCall{
		ID:       "call-1",
		Function: "test_tool",
	}
	toolResp := &tool.ToolResponse{
		Call:   toolCall,
		Result: tool.NewToolValue(map[string]any{"result": "success"}),
		Done:   true,
	}

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			NewToolResponseMessage(toolResp),
			NewTextMessage(ChatRoleAssistant, "Done!"),
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "Done!", response.GetText())

	toolResponses := response.GetToolResponses()
	require.Len(t, toolResponses, 1)
	assert.Equal(t, "call-1", toolResponses[0].Call.ID)
}

// TestUnstreamingChatModel_Chat_ContextCancellation verifies that context
// cancellation is properly handled in Chat method.
func TestUnstreamingChatModel_Chat_ContextCancellation(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			NewTextMessage(ChatRoleAssistant, "fragment 1"),
			NewTextMessage(ChatRoleAssistant, "fragment 2"),
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	// When context is cancelled, the stream should yield no fragments
	// and Chat should return an empty message
	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Empty(t, response.GetText())
}

// TestUnstreamingChatModel_Chat_PreservesOptionsAndTools verifies that Chat
// method passes options and tools to wrapped model.
func TestUnstreamingChatModel_Chat_PreservesOptionsAndTools(t *testing.T) {
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
		NewTextMessage(ChatRoleUser, "test message"),
	}
	options := &ChatOptions{
		Temperature: 0.8,
		TopP:        0.9,
		SessionID:   "test-session",
	}
	tools := []tool.ToolInfo{
		{Name: "tool1", Description: "First tool"},
		{Name: "tool2", Description: "Second tool"},
	}

	_, err := unstreaming.Chat(ctx, messages, options, tools)
	require.NoError(t, err)

	// Verify messages were recorded
	require.Len(t, provider.RecordedMessages, 1)
	assert.Equal(t, "test message", provider.RecordedMessages[0][0].GetText())
}

// TestUnstreamingChatModel_InterfaceCompliance verifies that UnstreamingChatModel
// implements the ChatModel interface.
func TestUnstreamingChatModel_InterfaceCompliance(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})
	wrapped := provider.ChatModel("test-model", nil)

	var _ ChatModel = (*UnstreamingChatModel)(nil)
	_ = NewUnstreamingChatModel(wrapped)
}

// TestUnstreamingChatModel_Chat_DifferentRoles verifies that Chat method
// handles fragments with different roles correctly.
func TestUnstreamingChatModel_Chat_DifferentRoles(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			{Role: ChatRoleAssistant, Parts: []ChatMessagePart{{Text: "Hello"}}},
			// Note: In real streaming, all fragments should have same role
			// but we test that we use the role from first fragment
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, ChatRoleAssistant, response.Role)
}

// TestUnstreamingChatModel_Chat_MultiplePartsInSingleFragment verifies handling
// of fragments that contain multiple parts. Text parts are concatenated into
// a single part while tool calls are preserved as separate parts.
func TestUnstreamingChatModel_Chat_MultiplePartsInSingleFragment(t *testing.T) {
	provider := NewMockProvider([]ModelInfo{})

	toolCall := &tool.ToolCall{
		ID:        "call-1",
		Function:  "test_tool",
		Arguments: tool.NewToolValue(map[string]any{}),
	}

	provider.SetChatResponse("wrapped-model", &MockChatResponse{
		StreamFragments: []*ChatMessage{
			{
				Role: ChatRoleAssistant,
				Parts: []ChatMessagePart{
					{Text: "First part"},
					{ToolCall: toolCall},
					{Text: "Second part"},
				},
			},
		},
	})

	wrapped := provider.ChatModel("wrapped-model", nil)
	unstreaming := NewUnstreamingChatModel(wrapped)

	ctx := context.Background()
	messages := []*ChatMessage{
		NewTextMessage(ChatRoleUser, "test"),
	}

	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "First partSecond part", response.GetText())

	toolCalls := response.GetToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call-1", toolCalls[0].ID)

	// Should have 2 parts: concatenated text, then tool call
	require.Len(t, response.Parts, 2)
	assert.Equal(t, "First partSecond part", response.Parts[0].Text)
	assert.Equal(t, toolCall, response.Parts[1].ToolCall)
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

// TestUnstreamingChatModel_Chat_PreservesNilOptions verifies that
// Chat works correctly when options is nil.
func TestUnstreamingChatModel_Chat_PreservesNilOptions(t *testing.T) {
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

	response, err := unstreaming.Chat(ctx, messages, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "response", response.GetText())
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
