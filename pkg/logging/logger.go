// Package logging provides structured logging infrastructure for the coding agent.
//
// The logging system maintains the following log files:
//   - .cswdata/logs/csw.jsonl - main log file for general application logs
//   - .cswdata/logs/sessions/<session_id>/session.jsonl - session-specific logs
//   - .cswdata/logs/sessions/<session_id>/llm.jsonl - raw LLM requests/responses
//
// All logs are written in JSONL format (one JSON object per line) using log/slog.
//
// Usage:
//
//	// Initialize logging directory at application startup
//	logging.SetLogsDirectory(".cswdata/logs", false)
//
//	// Get global logger
//	logger := logging.GetGlobalLogger()
//	logger.Info("application started")
//
//	// Get session-specific loggers
//	sessionLog := logging.GetSessionLogger(sessionID, "session")
//	llmLog := logging.GetSessionLogger(sessionID, "llm")
//
//	// Clean up when session is done
//	logging.CloseSessionLogger(sessionID)
//
//	// Clean up all loggers on shutdown
//	logging.FlushLogs()
package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/tool"
)

// LogType represents the type of session log
type LogType string

const (
	LogTypeSession LogType = "session"
	LogTypeLLM     LogType = "llm"
)

// writerCloser wraps an io.Writer with a Close method
type writerCloser struct {
	io.Writer
	closer io.Closer
}

func (w *writerCloser) Close() error {
	if w.closer != nil {
		return w.closer.Close()
	}
	return nil
}

// bufferWriter wraps a buffer and provides a no-op Close method
type bufferWriter struct {
	*bytes.Buffer
}

func (b *bufferWriter) Close() error {
	return nil
}

// syncWriter wraps a file and ensures writes are synced to disk
type syncWriter struct {
	file *os.File
}

func (s *syncWriter) Write(p []byte) (n int, err error) {
	n, err = s.file.Write(p)
	if err != nil {
		return n, err
	}
	return n, s.file.Sync()
}

func (s *syncWriter) Close() error {
	if err := s.file.Sync(); err != nil {
		return err
	}
	return s.file.Close()
}

// sessionLoggerKey represents a unique logger key
type sessionLoggerKey struct {
	sessionID string
	logType   LogType
}

var (
	// Global logger instance
	globalLogger     *slog.Logger
	globalLoggerOnce sync.Once
	globalLoggerMu   sync.RWMutex
	globalWriter     io.WriteCloser

	// Session loggers map
	sessionLoggers   map[sessionLoggerKey]*slog.Logger
	sessionWriters   map[sessionLoggerKey]io.WriteCloser
	sessionLoggersMu sync.RWMutex

	// Configuration
	logsDirectory string
	syncWrites    bool
	configMu      sync.RWMutex

	// Test mode
	testMode   bool
	testModeMu sync.RWMutex
)

func init() {
	sessionLoggers = make(map[sessionLoggerKey]*slog.Logger)
	sessionWriters = make(map[sessionLoggerKey]io.WriteCloser)
}

// SetLogsDirectory configures the logging system to write logs to files in the specified directory.
// If sync is true, log files will be flushed to disk after each write.
// Returns an error if dirPath is not writable.
// Until this function is called, all loggers store logs in memory.
func SetLogsDirectory(dirPath string, sync bool) error {
	configMu.Lock()
	defer configMu.Unlock()

	// Test if directory is writable
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("SetLogsDirectory() [logger.go]: failed to create log directory: %w", err)
	}

	testFile := filepath.Join(dirPath, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("SetLogsDirectory() [logger.go]: directory is not writable: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	logsDirectory = dirPath
	syncWrites = sync

	// Migrate existing loggers to file-based logging
	globalLoggerMu.Lock()
	if globalLogger != nil {
		// Close old writer
		if globalWriter != nil {
			globalWriter.Close()
		}

		// Create new file writer
		mainLogPath := filepath.Join(dirPath, "csw.jsonl")
		mainLogFile, err := os.OpenFile(mainLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			globalLoggerMu.Unlock()
			return fmt.Errorf("SetLogsDirectory() [logger.go]: failed to create main log file: %w", err)
		}

		var writer io.Writer
		if sync {
			writer = &syncWriter{file: mainLogFile}
			globalWriter = &syncWriter{file: mainLogFile}
		} else {
			writer = mainLogFile
			globalWriter = mainLogFile
		}

		multiWriter := io.MultiWriter(writer, os.Stderr)
		handler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		globalLogger = slog.New(handler)
	}
	globalLoggerMu.Unlock()

	// Migrate session loggers
	sessionLoggersMu.Lock()
	for key := range sessionLoggers {
		// Close old writer
		if writer, ok := sessionWriters[key]; ok {
			writer.Close()
		}

		// Create new file writer
		sessionDir := filepath.Join(dirPath, "sessions", key.sessionID)
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			sessionLoggersMu.Unlock()
			return fmt.Errorf("SetLogsDirectory() [logger.go]: failed to create session directory: %w", err)
		}

		logPath := filepath.Join(sessionDir, string(key.logType)+".jsonl")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			sessionLoggersMu.Unlock()
			return fmt.Errorf("SetLogsDirectory() [logger.go]: failed to create log file: %w", err)
		}

		var writer io.Writer
		var writerCloser io.WriteCloser
		if sync {
			sw := &syncWriter{file: logFile}
			writer = sw
			writerCloser = sw
		} else {
			writer = logFile
			writerCloser = logFile
		}

		handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		sessionLoggers[key] = slog.New(handler).With("session_id", key.sessionID)
		sessionWriters[key] = writerCloser
	}
	sessionLoggersMu.Unlock()

	return nil
}

// GetGlobalLogger returns the global logger instance.
// If SetLogsDirectory has not been called, returns a logger that writes to memory.
func GetGlobalLogger() *slog.Logger {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()

	if globalLogger == nil {
		// Check if we're in test mode
		testModeMu.RLock()
		inTestMode := testMode
		testModeMu.RUnlock()

		if inTestMode {
			// Return a no-op logger in test mode
			buf := &bufferWriter{Buffer: &bytes.Buffer{}}
			handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})
			globalLogger = slog.New(handler)
			globalWriter = buf
			return globalLogger
		}

		// Check if we have a logs directory configured
		configMu.RLock()
		logDir := logsDirectory
		configMu.RUnlock()

		if logDir != "" {
			// Create file-based logger
			mainLogPath := filepath.Join(logDir, "csw.jsonl")
			mainLogFile, err := os.OpenFile(mainLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				// Fallback to stderr
				handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})
				globalLogger = slog.New(handler)
				return globalLogger
			}

			configMu.RLock()
			sync := syncWrites
			configMu.RUnlock()

			var writer io.Writer
			if sync {
				sw := &syncWriter{file: mainLogFile}
				writer = sw
				globalWriter = sw
			} else {
				writer = mainLogFile
				globalWriter = mainLogFile
			}

			multiWriter := io.MultiWriter(writer, os.Stderr)
			handler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})
			globalLogger = slog.New(handler)
		} else {
			// Create in-memory logger
			buf := &bufferWriter{Buffer: &bytes.Buffer{}}
			handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})
			globalLogger = slog.New(handler)
			globalWriter = buf
		}
	}

	return globalLogger
}

// GetSessionLogger returns a logger for the specified session and log type.
// Loggers are cached and reused for the same session and log type.
// This function is thread-safe.
func GetSessionLogger(sessionID string, logType LogType) *slog.Logger {
	key := sessionLoggerKey{sessionID: sessionID, logType: logType}

	// Check if logger already exists
	sessionLoggersMu.RLock()
	logger, exists := sessionLoggers[key]
	sessionLoggersMu.RUnlock()

	if exists {
		return logger
	}

	// Create new logger
	sessionLoggersMu.Lock()
	defer sessionLoggersMu.Unlock()

	// Double-check after acquiring write lock
	if logger, exists := sessionLoggers[key]; exists {
		return logger
	}

	// Check if we're in test mode
	testModeMu.RLock()
	inTestMode := testMode
	testModeMu.RUnlock()

	if inTestMode {
		// Create in-memory logger for tests
		buf := &bufferWriter{Buffer: &bytes.Buffer{}}
		handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		logger = slog.New(handler).With("session_id", sessionID)
		sessionLoggers[key] = logger
		sessionWriters[key] = buf
		return logger
	}

	// Check if we have a logs directory configured
	configMu.RLock()
	logDir := logsDirectory
	configMu.RUnlock()

	if logDir != "" {
		// Create file-based logger
		sessionDir := filepath.Join(logDir, "sessions", sessionID)
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			// Fallback to in-memory logger
			buf := &bufferWriter{Buffer: &bytes.Buffer{}}
			handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})
			logger = slog.New(handler).With("session_id", sessionID)
			sessionLoggers[key] = logger
			sessionWriters[key] = buf
			return logger
		}

		logPath := filepath.Join(sessionDir, string(logType)+".jsonl")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// Fallback to in-memory logger
			buf := &bufferWriter{Buffer: &bytes.Buffer{}}
			handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})
			logger = slog.New(handler).With("session_id", sessionID)
			sessionLoggers[key] = logger
			sessionWriters[key] = buf
			return logger
		}

		configMu.RLock()
		sync := syncWrites
		configMu.RUnlock()

		var writer io.Writer
		var writerCloser io.WriteCloser
		if sync {
			sw := &syncWriter{file: logFile}
			writer = sw
			writerCloser = sw
		} else {
			writer = logFile
			writerCloser = logFile
		}

		handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		logger = slog.New(handler).With("session_id", sessionID)
		sessionLoggers[key] = logger
		sessionWriters[key] = writerCloser
	} else {
		// Create in-memory logger
		buf := &bufferWriter{Buffer: &bytes.Buffer{}}
		handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		logger = slog.New(handler).With("session_id", sessionID)
		sessionLoggers[key] = logger
		sessionWriters[key] = buf
	}

	return logger
}

// CloseSessionLogger closes all loggers for the specified session.
// This function is thread-safe.
func CloseSessionLogger(sessionID string) error {
	sessionLoggersMu.Lock()
	defer sessionLoggersMu.Unlock()

	var errs []error
	for _, logType := range []LogType{LogTypeSession, LogTypeLLM} {
		key := sessionLoggerKey{sessionID: sessionID, logType: logType}
		if writer, ok := sessionWriters[key]; ok {
			if err := writer.Close(); err != nil {
				errs = append(errs, err)
			}
			delete(sessionWriters, key)
		}
		delete(sessionLoggers, key)
	}

	if len(errs) > 0 {
		return fmt.Errorf("CloseSessionLogger() [logger.go]: failed to close some loggers: %v", errs)
	}

	return nil
}

// CloseSessionLoggers closes all session loggers.
// This function is thread-safe.
func CloseSessionLoggers() error {
	sessionLoggersMu.Lock()
	defer sessionLoggersMu.Unlock()

	var errs []error
	for key, writer := range sessionWriters {
		if err := writer.Close(); err != nil {
			errs = append(errs, err)
		}
		delete(sessionWriters, key)
		delete(sessionLoggers, key)
	}

	if len(errs) > 0 {
		return fmt.Errorf("CloseSessionLoggers() [logger.go]: failed to close some loggers: %v", errs)
	}

	return nil
}

// FlushLogs flushes all loggers to disk.
// This function is thread-safe.
func FlushLogs() error {
	var errs []error

	// Flush global logger
	globalLoggerMu.RLock()
	if globalWriter != nil {
		if syncer, ok := globalWriter.(interface{ Sync() error }); ok {
			if err := syncer.Sync(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	globalLoggerMu.RUnlock()

	// Flush session loggers
	sessionLoggersMu.RLock()
	for _, writer := range sessionWriters {
		if syncer, ok := writer.(interface{ Sync() error }); ok {
			if err := syncer.Sync(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	sessionLoggersMu.RUnlock()

	if len(errs) > 0 {
		return fmt.Errorf("FlushLogs() [logger.go]: failed to flush some loggers: %v", errs)
	}

	return nil
}

// EnableTestMode enables test mode where loggers don't create files.
// This should be called in init() or TestMain().
func EnableTestMode() {
	testModeMu.Lock()
	defer testModeMu.Unlock()
	testMode = true
}

// DisableTestMode disables test mode.
func DisableTestMode() {
	testModeMu.Lock()
	defer testModeMu.Unlock()
	testMode = false
}

// Helper functions for logging specific events

// LogUserInput logs user input to both session and chat logs.
func LogUserInput(sessionLog *slog.Logger, input string) {
	sessionLog.Info("user_message", "content", input, "role", "user")
}

// LogAssistantOutput logs assistant output chunks.
func LogAssistantOutput(sessionLog *slog.Logger, chunk string) {
	sessionLog.Info("assistant_chunk", "content", chunk, "role", "assistant")
}

// LogToolCall logs a tool call.
func LogToolCall(logger *slog.Logger, call *tool.ToolCall) {
	if logger == nil {
		return
	}
	logger.Info("tool_call_start",
		"tool_id", call.ID,
		"function", call.Function,
	)
	logger.Info("tool_call",
		"id", call.ID,
		"function", call.Function,
		"arguments", call.Arguments,
	)
}

// LogToolResult logs a tool execution result.
func LogToolResult(sessionLog *slog.Logger, response *tool.ToolResponse) {
	if sessionLog == nil {
		return
	}
	toolID := ""
	if response.Call != nil {
		toolID = response.Call.ID
	}

	sessionLog.Info("tool_response",
		"id", toolID,
		"result", response.Result,
		"error", fmt.Sprintf("%v", response.Error),
	)
}

// LogLLMRequest logs a raw LLM request.
func LogLLMRequest(llmLog *slog.Logger, provider string, model string, requestBody any) {
	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		llmLog.Error("failed_to_marshal_request", "error", err.Error())
		return
	}

	llmLog.Info("llm_request",
		"provider", provider,
		"model", model,
		"request_body", string(bodyJSON),
	)
}

// LogLLMResponse logs a raw LLM response.
func LogLLMResponse(llmLog *slog.Logger, provider string, model string, responseBody any) {
	bodyJSON, err := json.Marshal(responseBody)
	if err != nil {
		llmLog.Error("failed_to_marshal_response", "error", err.Error())
		return
	}

	llmLog.Info("llm_response",
		"provider", provider,
		"model", model,
		"response_body", string(bodyJSON),
	)
}

// LogLLMStreamChunk logs a chunk from LLM streaming response.
func LogLLMStreamChunk(llmLog *slog.Logger, provider string, model string, chunk any) {
	chunkJSON, err := json.Marshal(chunk)
	if err != nil {
		llmLog.Error("failed_to_marshal_chunk", "error", err.Error())
		return
	}

	llmLog.Debug("llm_stream_chunk",
		"provider", provider,
		"model", model,
		"chunk", string(chunkJSON),
	)
}

// LogPermissionQuery logs a permission query.
func LogPermissionQuery(sessionLog *slog.Logger, query *tool.ToolPermissionsQuery) {
	toolFunc := ""
	if query.Tool != nil {
		toolFunc = query.Tool.Function
	}

	sessionLog.Info("permission_query",
		"query_id", query.Id,
		"tool", toolFunc,
		"title", query.Title,
		"details", query.Details,
	)
}

// LogPermissionResponse logs a permission response.
func LogPermissionResponse(sessionLog *slog.Logger, query *tool.ToolPermissionsQuery, response string) {
	sessionLog.Info("permission_response",
		"tool", query.Tool.Function,
		"response", response,
	)
}

// LogChatMessages logs chat messages in internal format.
func LogChatMessages(chatLog *slog.Logger, messages []*models.ChatMessage) {
	for _, msg := range messages {
		msgJSON, err := json.Marshal(msg)
		if err != nil {
			chatLog.Error("failed_to_marshal_message", "error", err.Error())
			continue
		}

		chatLog.Info("chat_message",
			"role", msg.Role,
			"message", string(msgJSON),
		)
	}
}
