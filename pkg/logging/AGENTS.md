# Package `pkg/logging` Overview

Package `pkg/logging` contains structured logging infrastructure for global runtime events and per-session logs. It manages logger creation/caching, file-backed JSONL logging, in-memory fallback behavior, flush/close lifecycle, and standardized event helpers for chat and tool activity.

## Important files

* `logger.go` - Primary logging implementation with global and session loggers
* `test_logger.go` - In-memory logger utilities for tests

## Important public API objects

* `LogType` - Type of session log (session or llm)
* `SetLogsDirectory` - Configures logging to write logs to files
* `GetSessionLogDirectory` - Returns path to session log directory
* `GetGlobalLogger` - Returns global logger instance
* `GetSessionLogger` - Returns logger for session and log type
* `CloseSessionLogger` - Closes all loggers for session
* `CloseSessionLoggers` - Closes all session loggers
* `FlushLogs` - Flushes all loggers to disk
* `LogUserInput` - Logs user input to session logs
* `LogAssistantOutput` - Logs assistant output chunks
* `LogToolCall` - Logs a tool call
* `LogToolResult` - Logs a tool execution result
* `LogLLMRequest` - Logs raw LLM request
* `LogLLMResponse` - Logs raw LLM response
* `LogLLMStreamChunk` - Logs chunk from LLM streaming response
* `LogPermissionQuery` - Logs a permission query
* `LogPermissionResponse` - Logs a permission response
* `LogChatMessages` - Logs chat messages in internal format
* `TestLoggerData` - Holds in-memory log data for test session
* `NewTestLogger` - Creates test loggers that store logs in memory
* `NewTestLoggerFactory` - Creates SessionLoggerFactory for test loggers
* `FlushTestLogger` - Prints buffered logs to test output
* `GetTestSessionBuffer` - Returns session log buffer for inspection
* `GetTestLLMBuffer` - Returns LLM log buffer for inspection
