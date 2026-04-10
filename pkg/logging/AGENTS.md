# Package `pkg/logging` Overview

Package `pkg/logging` provides structured JSONL logging for global runtime and per-session events, including LLM request/response traces and test helpers.

## Important files

* `logger.go` - Global and session JSONL loggers.
* `test_logger.go` - In-memory test logger buffers.

## Important public API objects

* `LogType` - Enum: `LogTypeSession`, `LogTypeLLM`.
* `SetLogsDirectory` - Configures log directory and sync mode.
* `GetSessionLogDirectory` - Returns session log directory path.
* `GetGlobalLogger` - Returns global slog logger.
* `GetSessionLogger` - Returns session logger by type.
* `CloseSessionLogger` - Closes one session logger.
* `CloseSessionLoggers` - Closes all session loggers.
* `FlushLogs` - Flushes all logger writers.
* `LogUserInput` - Logs user message event.
* `LogAssistantOutput` - Logs assistant chunk event.
* `LogToolCall` - Logs tool call payload.
* `LogToolResult` - Logs tool response payload.
* `LogLLMRequest` - Logs raw LLM request payload.
* `LogLLMResponse` - Logs raw LLM response payload.
* `LogLLMStreamChunk` - Logs LLM stream chunk payload.
* `LogPermissionQuery` - Logs permission query event.
* `LogPermissionResponse` - Logs permission response event.
* `LogChatMessages` - Logs serialized chat messages.
* `TestLoggerData` - Stores in-memory test buffers.
* `NewTestLogger` - Creates in-memory test loggers.
* `NewTestLoggerFactory` - Creates test logger factory.
* `FlushTestLogger` - Flushes buffered test logs.
* `GetTestSessionBuffer` - Returns session test buffer.
* `GetTestLLMBuffer` - Returns LLM test buffer.
