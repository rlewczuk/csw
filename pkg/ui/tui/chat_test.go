package tui

import (
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatWidget(t *testing.T) {
	t.Run("initial state and view", func(t *testing.T) {
		// Create mock output handler for capturing session events
		outputHandler := testutil.NewMockSessionOutputHandler()

		// Create a minimal controller (we won't start a real session)
		controller := core.NewSessionController(nil, outputHandler)

		// Create chat widget
		widget, err := NewChatWidget(controller)
		require.NoError(t, err)

		// Create mock terminal
		term := NewTerminal()

		term.Run(widget)

		// Check output contains welcome message
		assert.True(t, term.WaitForText("Welcome!", 1*time.Second), "Initial view should contain welcome message")
		assert.True(t, term.ContainsText("Type your message"), "Initial view should contain textarea placeholder")

		// Cleanup: send quit command
		term.SendKey("esc")
		term.Close()
	})

	t.Run("user message is added to widget", func(t *testing.T) {
		// Create mock output handler
		outputHandler := testutil.NewMockSessionOutputHandler()

		// Create a minimal controller
		controller := core.NewSessionController(nil, outputHandler)

		// Create chat widget
		widget, err := NewChatWidget(controller)
		require.NoError(t, err)

		// Create mock terminal
		term := NewTerminal()

		term.Run(widget)

		// Type a message via mock terminal
		term.SendString("Hello assistant!")

		// Send Alt+Enter to submit
		term.SendKey("alt+enter")

		// Wait for message to appear in output
		found := term.WaitForText("Hello assistant!", 2*time.Second)
		assert.True(t, found, "View should show user message")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("markdown chunks are displayed", func(t *testing.T) {
		// Create mock output handler for capturing session events
		outputHandler := testutil.NewMockSessionOutputHandler()

		// Create a minimal controller
		controller := core.NewSessionController(nil, outputHandler)

		// Create chat widget
		widget, err := NewChatWidget(controller)
		require.NoError(t, err)

		// Create mock terminal
		term := NewTerminal()

		term.Run(widget)

		// Simulate assistant response chunks via SessionThreadOutput interface
		// This is how the controller would send data to the widget
		widget.AddMarkdownChunk("This is ")
		widget.AddMarkdownChunk("a test ")
		widget.AddMarkdownChunk("response.")

		// Wait for content to appear in terminal output
		found := term.WaitForText("This is a test response.", 2*time.Second)
		assert.True(t, found, "View should show assembled markdown chunks")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("tool calls are displayed", func(t *testing.T) {
		// Create mock output handler
		outputHandler := testutil.NewMockSessionOutputHandler()

		// Create a minimal controller
		controller := core.NewSessionController(nil, outputHandler)

		// Create chat widget
		widget, err := NewChatWidget(controller)
		require.NoError(t, err)

		// Create mock terminal
		term := NewTerminal()

		term.Run(widget)

		// Simulate tool call via SessionThreadOutput interface
		toolCall := &tool.ToolCall{
			ID:       "call_123",
			Function: "test_tool",
		}
		toolCall.Arguments.Set("arg1", "value1")

		widget.AddToolCallStart(toolCall)
		widget.AddToolCallDetails(toolCall)

		// Wait for tool call to appear in terminal output
		found := term.WaitForText("test_tool", 2*time.Second)
		assert.True(t, found, "View should show tool call name")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("tool call results are displayed", func(t *testing.T) {
		// Create mock output handler
		outputHandler := testutil.NewMockSessionOutputHandler()

		// Create a minimal controller
		controller := core.NewSessionController(nil, outputHandler)

		// Create chat widget
		widget, err := NewChatWidget(controller)
		require.NoError(t, err)

		// Create mock terminal
		term := NewTerminal()

		term.Run(widget)

		// Simulate tool call and result via SessionThreadOutput interface
		toolCall := &tool.ToolCall{
			ID:       "call_456",
			Function: "test_tool_with_result",
		}
		toolCall.Arguments.Set("param", "test")

		widget.AddToolCallStart(toolCall)
		widget.AddToolCallDetails(toolCall)

		// Add result
		result := &tool.ToolResponse{
			Call: toolCall,
		}
		result.Result.Set("output", "success")

		widget.AddToolCallResult(result)

		// Wait for tool call to appear in terminal output
		found := term.WaitForText("test_tool_with_result", 2*time.Second)
		assert.True(t, found, "View should show tool call name")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("esc key quits program", func(t *testing.T) {
		// Create mock output handler
		outputHandler := testutil.NewMockSessionOutputHandler()

		// Create a minimal controller
		controller := core.NewSessionController(nil, outputHandler)

		// Create chat widget
		widget, err := NewChatWidget(controller)
		require.NoError(t, err)

		// Create mock terminal
		term := NewTerminal()

		// Run program and capture when it exits
		done := make(chan struct{})
		go func() {
			term.Run(widget)
			close(done)
		}()

		// Send esc key
		term.SendKey("esc")

		// Wait for program to exit
		select {
		case <-done:
			// Program exited as expected
		case <-time.After(2 * time.Second):
			t.Error("Program did not exit after esc key")
			term.Close()
		}
	})

	t.Run("multiple markdown chunks build up message", func(t *testing.T) {
		// Create mock output handler
		outputHandler := testutil.NewMockSessionOutputHandler()

		// Create a minimal controller
		controller := core.NewSessionController(nil, outputHandler)

		// Create chat widget
		widget, err := NewChatWidget(controller)
		require.NoError(t, err)

		// Create mock terminal
		term := NewTerminal()

		term.Run(widget)

		// Simulate streaming response via SessionThreadOutput interface
		chunks := []string{"Hello", " ", "world", "!", " ", "This", " ", "is", " ", "streaming."}
		for _, chunk := range chunks {
			widget.AddMarkdownChunk(chunk)
		}

		// Finalize the message
		widget.RunFinished(nil)

		// Wait for complete message to appear in terminal output
		fullMessage := "Hello world! This is streaming."
		found := term.WaitForText(fullMessage, 2*time.Second)
		assert.True(t, found, "View should show complete assembled message")

		// Cleanup
		term.SendKey("esc")
		time.Sleep(50 * time.Millisecond)
		term.Close()
	})
}
