package testutil

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockSessionOutputHandler(t *testing.T) {
	t.Run("new handler is empty", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		require.NotNil(t, handler)
		assert.Empty(t, handler.ThinkingChunks)
		assert.Empty(t, handler.MarkdownChunks)
		assert.Empty(t, handler.ToolCallStarts)
		assert.Empty(t, handler.ToolCallDetails)
		assert.Empty(t, handler.ToolCallResults)
	})

	t.Run("records thinking chunks", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		handler.AddThinkingChunk("Let me think...")
		handler.AddThinkingChunk(" And more thoughts.")

		assert.Len(t, handler.ThinkingChunks, 2)
		assert.Equal(t, "Let me think...", handler.ThinkingChunks[0])
		assert.Equal(t, " And more thoughts.", handler.ThinkingChunks[1])
	})

	t.Run("records markdown chunks", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		handler.AddMarkdownChunk("Hello ")
		handler.AddMarkdownChunk("World")

		assert.Len(t, handler.MarkdownChunks, 2)
		assert.Equal(t, "Hello ", handler.MarkdownChunks[0])
		assert.Equal(t, "World", handler.MarkdownChunks[1])
	})

	t.Run("records tool call starts", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		call1 := &tool.ToolCall{ID: "1", Function: "test.tool"}
		call2 := &tool.ToolCall{ID: "2", Function: "another.tool"}

		handler.AddToolCallStart(call1)
		handler.AddToolCallStart(call2)

		assert.Len(t, handler.ToolCallStarts, 2)
		assert.Equal(t, "1", handler.ToolCallStarts[0].ID)
		assert.Equal(t, "2", handler.ToolCallStarts[1].ID)
	})

	t.Run("records tool call details", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		call := &tool.ToolCall{ID: "1", Function: "test.tool"}

		// Multiple calls for the same tool (streaming)
		handler.AddToolCallDetails(call)
		handler.AddToolCallDetails(call)

		assert.Len(t, handler.ToolCallDetails, 2)
		assert.Equal(t, "1", handler.ToolCallDetails[0].ID)
		assert.Equal(t, "1", handler.ToolCallDetails[1].ID)
	})

	t.Run("records tool call results", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		call := &tool.ToolCall{ID: "1", Function: "test.tool"}
		result := &tool.ToolResponse{
			Call:   call,
			Result: tool.NewToolValue("success"),
			Done:   true,
		}

		handler.AddToolCallResult(result)

		assert.Len(t, handler.ToolCallResults, 1)
		assert.Equal(t, "1", handler.ToolCallResults[0].Call.ID)
		assert.Equal(t, "success", handler.ToolCallResults[0].Result.AsString())
	})

	t.Run("reset clears all data", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		handler.AddThinkingChunk("thinking...")
		handler.AddMarkdownChunk("test")
		handler.AddToolCallStart(&tool.ToolCall{ID: "1"})
		handler.AddToolCallDetails(&tool.ToolCall{ID: "1"})
		handler.AddToolCallResult(&tool.ToolResponse{
			Call: &tool.ToolCall{ID: "1"},
		})

		handler.Reset()

		assert.Empty(t, handler.ThinkingChunks)
		assert.Empty(t, handler.MarkdownChunks)
		assert.Empty(t, handler.ToolCallStarts)
		assert.Empty(t, handler.ToolCallDetails)
		assert.Empty(t, handler.ToolCallResults)
	})
}
