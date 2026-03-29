package presenter

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyToolCallWithResult(t *testing.T) {
	t.Run("merges result content into arguments", func(t *testing.T) {
		// Setup - simulate a vfsRead execution result
		call := &tool.ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: tool.NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		}

		var result tool.ToolValue
		result.Set("content", "line1\nline2\nline3")
		response := &tool.ToolResponse{
			Call:   call,
			Result: result,
			Done:   true,
		}

		// Execute
		callWithResult := copyToolCallWithResult(call, response)

		// Assert - arguments should now contain content
		assert.Equal(t, "test-id", callWithResult.ID)
		assert.Equal(t, "vfsRead", callWithResult.Function)
		assert.Equal(t, "test.txt", callWithResult.Arguments.Get("path").AsString())
		assert.Equal(t, "line1\nline2\nline3", callWithResult.Arguments.Get("content").AsString())
	})

	t.Run("merges error into arguments when present", func(t *testing.T) {
		// Setup
		call := &tool.ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: tool.NewToolValue(map[string]any{
				"path": "nonexistent.txt",
			}),
		}

		var result tool.ToolValue
		response := &tool.ToolResponse{
			Call:   call,
			Result: result,
			Error:  assert.AnError,
			Done:   true,
		}

		// Execute
		callWithResult := copyToolCallWithResult(call, response)

		// Assert - arguments should contain error
		assert.Equal(t, "test-id", callWithResult.ID)
		assert.Equal(t, "assert.AnError general error for testing", callWithResult.Arguments.Get("error").AsString())
	})

	t.Run("merges both result and error when present", func(t *testing.T) {
		// Setup
		call := &tool.ToolCall{
			ID:       "test-id",
			Function: "vfsWrite",
			Arguments: tool.NewToolValue(map[string]any{
				"path":    "test.txt",
				"content": "hello",
			}),
		}

		var result tool.ToolValue
		result.Set("validation", "warning: some issue")
		response := &tool.ToolResponse{
			Call:   call,
			Result: result,
			Error:  nil,
			Done:   true,
		}

		// Execute
		callWithResult := copyToolCallWithResult(call, response)

		// Assert
		assert.Equal(t, "test.txt", callWithResult.Arguments.Get("path").AsString())
		assert.Equal(t, "hello", callWithResult.Arguments.Get("content").AsString())
		assert.Equal(t, "warning: some issue", callWithResult.Arguments.Get("validation").AsString())
	})

	t.Run("preserves original call when result is empty", func(t *testing.T) {
		// Setup
		call := &tool.ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: tool.NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		}

		response := &tool.ToolResponse{
			Call:   call,
			Result: tool.ToolValue{}, // Empty result
			Done:   true,
		}

		// Execute
		callWithResult := copyToolCallWithResult(call, response)

		// Assert - should still have path
		assert.Equal(t, "test.txt", callWithResult.Arguments.Get("path").AsString())
	})

	t.Run("does not modify original call", func(t *testing.T) {
		// Setup
		call := &tool.ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: tool.NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		}

		var result tool.ToolValue
		result.Set("content", "file content")
		response := &tool.ToolResponse{
			Call:   call,
			Result: result,
			Done:   true,
		}

		// Execute
		callWithResult := copyToolCallWithResult(call, response)

		// Assert - original call should not have content
		assert.False(t, call.Arguments.Has("content"))
		// But new call should
		assert.True(t, callWithResult.Arguments.Has("content"))
		require.Equal(t, "file content", callWithResult.Arguments.Get("content").AsString())
	})
}

func TestCopyToolCallWithError(t *testing.T) {
	t.Run("adds error to arguments", func(t *testing.T) {
		// Setup
		call := &tool.ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: tool.NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		}

		// Execute
		callWithError := copyToolCallWithError(call, assert.AnError)

		// Assert
		assert.Equal(t, "test-id", callWithError.ID)
		assert.Equal(t, "vfsRead", callWithError.Function)
		assert.Equal(t, "test.txt", callWithError.Arguments.Get("path").AsString())
		assert.Equal(t, "assert.AnError general error for testing", callWithError.Arguments.Get("error").AsString())
	})

	t.Run("does not modify original call", func(t *testing.T) {
		// Setup
		call := &tool.ToolCall{
			ID:       "test-id",
			Function: "vfsRead",
			Arguments: tool.NewToolValue(map[string]any{
				"path": "test.txt",
			}),
		}

		// Execute
		callWithError := copyToolCallWithError(call, assert.AnError)

		// Assert - original call should not have error
		assert.False(t, call.Arguments.Has("error"))
		// But new call should
		assert.True(t, callWithError.Arguments.Has("error"))
	})
}

func TestChatPresenter_MergeToolResponsesIntoSession(t *testing.T) {
	t.Run("updates existing tool state with rendered details", func(t *testing.T) {
		presenter := &ChatPresenter{}
		chatSession := &ui.ChatSessionUI{
			Messages: []*ui.ChatMessageUI{
				{
					Id:   "msg-1",
					Role: ui.ChatRoleAssistant,
					Tools: []*ui.ToolUI{
						{Id: "tool-1", Name: "runBash"},
					},
				},
			},
		}

		toolState := chatSession.Messages[0].Tools[0]
		toolStatesByID := map[string]*ui.ToolUI{"tool-1": toolState}

		response := &tool.ToolResponse{
			Call: &tool.ToolCall{
				ID:       "tool-1",
				Function: "runBash",
				Arguments: tool.NewToolValue(map[string]any{
					"command": "echo test",
				}),
			},
			Result: tool.NewToolValue(map[string]any{
				"exit_code": int64(0),
				"stdout":    "test\n",
			}),
			Done: true,
		}

		presenter.mergeToolResponsesIntoSession(chatSession, toolStatesByID, []*tool.ToolResponse{response})

		assert.Equal(t, ui.ToolStatusSucceeded, toolState.Status)
	})

	t.Run("creates placeholder assistant tool when response is restored without call message", func(t *testing.T) {
		presenter := &ChatPresenter{}
		chatSession := &ui.ChatSessionUI{Messages: []*ui.ChatMessageUI{}}
		toolStatesByID := map[string]*ui.ToolUI{}

		response := &tool.ToolResponse{
			Call: &tool.ToolCall{
				ID:       "tool-missing",
				Function: "runBash",
				Arguments: tool.NewToolValue(map[string]any{
					"command": "echo restored",
				}),
			},
			Result: tool.NewToolValue(map[string]any{
				"exit_code": int64(1),
				"stderr":    "permission denied",
			}),
			Done: true,
		}

		presenter.mergeToolResponsesIntoSession(chatSession, toolStatesByID, []*tool.ToolResponse{response})

		require.Len(t, chatSession.Messages, 1)
		require.Len(t, chatSession.Messages[0].Tools, 1)
		restored := chatSession.Messages[0].Tools[0]
		assert.Equal(t, "tool-missing", restored.Id)
		assert.Equal(t, ui.ToolStatusSucceeded, restored.Status)
	})
}
