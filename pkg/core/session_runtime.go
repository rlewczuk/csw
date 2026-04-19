package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
)

func mapSharedMessageTypeToSessionMessageType(msgType shared.MessageType) string {
	switch msgType {
	case shared.MessageTypeError:
		return sessionMessageTypeError
	case shared.MessageTypeWarning:
		return sessionMessageTypeWarning
	default:
		return sessionMessageTypeInfo
	}
}

func extractRetryAfterSeconds(message string) (int, bool) {
	if strings.TrimSpace(message) == "" {
		return 0, false
	}

	matches := regexp.MustCompile(`\(in\s+(\d+)\s+seconds\)`).FindStringSubmatch(message)
	if len(matches) < 2 {
		return 0, false
	}

	retryAfterSeconds, err := strconv.Atoi(matches[1])
	if err != nil || retryAfterSeconds < 0 {
		return 0, false
	}

	return retryAfterSeconds, true
}

func (s *SweSession) maybeCompactContext() error {
	maxContextLength := s.maxContextLengthLimit()
	if maxContextLength <= 0 || s.contextLength <= 0 {
		return nil
	}

	threshold := s.contextCompactionThreshold()
	if threshold <= 0 {
		threshold = defaultContextCompactionThreshold
	}

	if float64(s.contextLength) <= float64(maxContextLength)*threshold {
		return nil
	}

	return s.compactContext("Context is near maximum length. Compacting messages...")
}

func (s *SweSession) compactContext(statusMessage string) error {
	compactionNumber := s.compactionCount + 1
	if s.outputHandler != nil && strings.TrimSpace(statusMessage) != "" {
		s.outputHandler.ShowMessage(statusMessage, sessionMessageTypeInfo)
	}

	if err := s.persistCompactionMessagesSnapshot("pre", compactionNumber, s.messages); err != nil {
		return fmt.Errorf("SweSession.maybeCompactContext() [session_runtime.go]: failed to persist pre-compaction snapshot: %w", err)
	}

	if s.compactor == nil {
		s.compactor = NewCompactMessagesChatCompactor()
	}

	compacted := s.compactor.CompactMessages(s.messages)
	if err := s.persistCompactionMessagesSnapshot("post", compactionNumber, compacted); err != nil {
		return fmt.Errorf("SweSession.maybeCompactContext() [session_runtime.go]: failed to persist post-compaction snapshot: %w", err)
	}

	s.messages = compacted
	s.compactionCount = compactionNumber
	s.persistSessionState()

	return nil
}

func (s *SweSession) contextCompactionThreshold() float64 {
	if s.configStore != nil {
		globalConfig, err := s.configStore.GetGlobalConfig()
		if err == nil && globalConfig != nil && globalConfig.ContextCompactionThreshold > 0 && globalConfig.ContextCompactionThreshold <= 1 {
			return globalConfig.ContextCompactionThreshold
		}
	}

	return defaultContextCompactionThreshold
}

func (s *SweSession) maxContextLengthLimit() int {
	if configProvider, ok := s.provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		providerConfig := configProvider.GetConfig()
		if providerConfig != nil && providerConfig.ContextLengthLimit > 0 {
			return providerConfig.ContextLengthLimit
		}
	}

	return 0
}

func (s *SweSession) persistCompactionMessagesSnapshot(phase string, compactionNumber int, messages []*models.ChatMessage) error {
	sessionLogDir := s.getSessionLogDirectory()
	if strings.TrimSpace(sessionLogDir) == "" {
		return nil
	}

	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return fmt.Errorf("SweSession.persistCompactionMessagesSnapshot() [session_runtime.go]: failed to create session log directory: %w", err)
	}

	filePath := filepath.Join(sessionLogDir, fmt.Sprintf("messages-%s-%d.jsonl", phase, compactionNumber))
	if err := writeMessagesJSONL(filePath, messages); err != nil {
		return fmt.Errorf("SweSession.persistCompactionMessagesSnapshot() [session_runtime.go]: failed to write %s snapshot: %w", phase, err)
	}

	return nil
}

// llmRetryMaxAttempts returns the maximum number of retries for rate limit/network errors.
// Returns default value from models.DefaultMaxRetries if not configured.
func (s *SweSession) llmRetryMaxAttempts() int {
	if s.configStore != nil {
		globalConfig, err := s.configStore.GetGlobalConfig()
		if err == nil && globalConfig != nil && globalConfig.LLMRetryMaxAttempts > 0 {
			return globalConfig.LLMRetryMaxAttempts
		}
	}

	if configProvider, ok := s.provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		config := configProvider.GetConfig()
		if config != nil && config.MaxRetries > 0 {
			return config.MaxRetries + 1
		}
	}

	return defaultLLMRetryMaxAttempts
}

// llmRetryMaxBackoffSeconds returns the maximum backoff in seconds for temporary failures.
func (s *SweSession) llmRetryMaxBackoffSeconds() int {
	if s.configStore != nil {
		globalConfig, err := s.configStore.GetGlobalConfig()
		if err == nil && globalConfig != nil && globalConfig.LLMRetryMaxBackoffSeconds > 0 {
			return globalConfig.LLMRetryMaxBackoffSeconds
		}
	}

	return defaultLLMRetryMaxBackoffSeconds
}

func (s *SweSession) llmRetryPolicy() models.RetryPolicy {
	attempts := s.llmRetryMaxAttempts()
	if attempts <= 0 {
		attempts = defaultLLMRetryMaxAttempts
	}

	retries := attempts - 1
	if retries < 0 {
		retries = 0
	}

	backoffScale := models.DefaultRetryBackoffScale
	if configProvider, ok := s.provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		config := configProvider.GetConfig()
		if config != nil && config.RateLimitBackoffScale > 0 {
			backoffScale = config.GetRateLimitBackoffScaleDuration()
		}
	}

	maxBackoffDuration := time.Duration(s.llmRetryMaxBackoffSeconds()) * backoffScale
	if maxBackoffDuration <= 0 {
		maxBackoffDuration = 60 * backoffScale
	}

	return models.RetryPolicy{
		InitialDelay: backoffScale,
		MaxRetries:   retries,
		MaxDelay:     maxBackoffDuration,
	}
}

// maxToolThreadsLimit returns max number of parallel tool executions.
func (s *SweSession) maxToolThreadsLimit() int {
	if s.maxToolThreads > 0 {
		return s.maxToolThreads
	}

	if s.configStore != nil {
		globalConfig, err := s.configStore.GetGlobalConfig()
		if err == nil && globalConfig != nil && globalConfig.Defaults.MaxThreads > 0 {
			return globalConfig.Defaults.MaxThreads
		}
	}

	return defaultMaxToolThreads
}

func (s *SweSession) toolExecutionStartDelayLimit() time.Duration {
	if s.toolStartDelay > 0 {
		return s.toolStartDelay
	}

	return toolExecutionStartDelay
}
