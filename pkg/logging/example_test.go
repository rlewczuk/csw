package logging_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/tool"
)

// ExampleSetLogsDirectory demonstrates how to configure the logging directory
func ExampleSetLogsDirectory() {
	tmpDir := filepath.Join("../../tmp", "example_logs")
	defer os.RemoveAll(tmpDir)

	err := logging.SetLogsDirectory(tmpDir, false)
	if err != nil {
		panic(err)
	}

	logger := logging.GetGlobalLogger()
	logger.Info("application_started", "version", "1.0.0")

	fmt.Println("Logging directory configured successfully")
	// Output: Logging directory configured successfully
}

// ExampleGetSessionLogger demonstrates how to create a session-specific logger
func ExampleGetSessionLogger() {
	tmpDir := filepath.Join("../../tmp", "example_logs")
	defer os.RemoveAll(tmpDir)

	logging.SetLogsDirectory(tmpDir, false)

	sessionID := "session-123"
	sessionLogger := logging.GetSessionLogger(sessionID, logging.LogTypeSession)
	defer logging.CloseSessionLogger(sessionID)

	// Log basic messages
	sessionLogger.Info("user_input", "input", "Hello, assistant!")
	sessionLogger.Debug("assistant_output_chunk", "chunk", "Hello! How can I help you?")

	// For more complex logging like tool calls, use helper functions
	toolCall := &tool.ToolCall{
		ID:       "call-1",
		Function: "vfsWrite",
		Arguments: tool.NewToolValue(map[string]any{
			"path":    "test.txt",
			"content": "Hello, World!",
		}),
	}
	logging.LogToolCall(sessionLogger, toolCall)

	fmt.Println("Session logger created successfully")
	// Output: Session logger created successfully
}

// ExampleNewTestLogger demonstrates how to use the test logger in unit tests
func ExampleNewTestLogger() {
	// In a real test, you would pass testing.T
	// For this example, we'll show the API

	// Create test logger - returns three *slog.Logger instances
	// sessionLog, chatLog, llmLog := logging.NewTestLogger(t, "test-session-123")
	// defer logging.FlushTestLogger("test-session-123") // Only prints logs if test fails

	// Use them like regular slog loggers
	// sessionLog.Info("test_message", "key", "value")
	// logging.LogUserInput(sessionLog, chatLog, "test input")

	// Inspect logs using buffer methods
	// buf := logging.GetTestChatBuffer("test-session-123")
	// assert.Contains(t, buf.String(), "test input")

	fmt.Println("Test logger can be used for in-memory logging during tests")
	// Output: Test logger can be used for in-memory logging during tests
}
