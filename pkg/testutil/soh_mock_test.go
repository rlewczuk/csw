package testutil

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockSessionOutputHandler(t *testing.T) {
	t.Run("new handler is empty", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		require.NotNil(t, handler)
		assert.Empty(t, handler.AssistantMessages)
		assert.Empty(t, handler.ToolCalls)
		assert.Empty(t, handler.ToolCallResults)
	})

	t.Run("records assistant messages", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		handler.AddAssistantMessage("Hello", "thinking")
		handler.AddAssistantMessage("World", "")

		assert.Len(t, handler.AssistantMessages, 2)
		assert.Equal(t, "Hello", handler.AssistantMessages[0].Text)
		assert.Equal(t, "thinking", handler.AssistantMessages[0].Thinking)
		assert.Equal(t, "World", handler.AssistantMessages[1].Text)
	})

	t.Run("records tool calls", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		call1 := &tool.ToolCall{ID: "1", Function: "test.tool"}
		call2 := &tool.ToolCall{ID: "2", Function: "another.tool"}

		handler.AddToolCall(call1)
		handler.AddToolCall(call2)

		assert.Len(t, handler.ToolCalls, 2)
		assert.Equal(t, "1", handler.ToolCalls[0].ID)
		assert.Equal(t, "2", handler.ToolCalls[1].ID)
	})

	t.Run("records tool call results", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		call := &tool.ToolCall{ID: "1", Function: "test.tool"}
		result := &tool.ToolResponse{Call: call, Result: tool.NewToolValue("success"), Done: true}

		handler.AddToolCallResult(result)

		assert.Len(t, handler.ToolCallResults, 1)
		assert.Equal(t, "1", handler.ToolCallResults[0].Call.ID)
		assert.Equal(t, "success", handler.ToolCallResults[0].Result.AsString())
	})

	t.Run("reset clears all data", func(t *testing.T) {
		handler := NewMockSessionOutputHandler()
		handler.AddAssistantMessage("test", "")
		handler.AddToolCall(&tool.ToolCall{ID: "1"})
		handler.AddToolCallResult(&tool.ToolResponse{Call: &tool.ToolCall{ID: "1"}})

		handler.Reset()

		assert.Empty(t, handler.AssistantMessages)
		assert.Empty(t, handler.ToolCalls)
		assert.Empty(t, handler.ToolCallResults)
	})
}
