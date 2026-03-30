package models

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/tool"
)

// FallbackChatModel wraps multiple chat models and provides persistent fallback behavior.
//
// The wrapper starts with the first model and keeps using the currently selected model
// across requests until errors require switching to another model.
type FallbackChatModel struct {
	mu              sync.Mutex
	models          []ChatModel
	selectedIndex   int
	switchDelay     time.Duration
	waitWithContext func(context.Context, time.Duration) error
}

// NewFallbackChatModel creates a new FallbackChatModel.
//
// switchDelaySeconds defines how long to wait before switching to the next model
// when a temporary (retryable) non-rate-limit error occurs.
func NewFallbackChatModel(models []ChatModel, switchDelaySeconds int) *FallbackChatModel {
	if switchDelaySeconds < 0 {
		switchDelaySeconds = 0
	}

	return &FallbackChatModel{
		models:          models,
		switchDelay:     time.Duration(switchDelaySeconds) * time.Second,
		waitWithContext: waitWithContext,
	}
}

// Chat sends a request to the selected model and falls back to other models on failure.
func (f *FallbackChatModel) Chat(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) (*ChatMessage, error) {
	if len(f.models) == 0 {
		return nil, fmt.Errorf("FallbackChatModel.Chat() [fallback.go]: no chat models configured")
	}

	index := f.getSelectedIndex()
	attempts := len(f.models)

	var bestRateLimit *RateLimitError
	var lastErr error

	for i := 0; i < attempts; i++ {
		model := f.models[index]
		if model == nil {
			lastErr = fmt.Errorf("FallbackChatModel.Chat() [fallback.go]: chat model at index %d is nil", index)
			index = f.switchToNextModel(index)
			continue
		}

		response, err := model.Chat(ctx, messages, options, tools)
		if err == nil {
			f.setSelectedIndex(index)
			return response, nil
		}

		lastErr = err

		var rateLimitErr *RateLimitError
		if errors.As(err, &rateLimitErr) {
			bestRateLimit = selectShortestRateLimit(bestRateLimit, rateLimitErr)
			index = f.switchToNextModel(index)
			continue
		}

		if isRetryableError(err) {
			if waitErr := f.waitWithContext(ctx, f.switchDelay); waitErr != nil {
				return nil, waitErr
			}
		}

		index = f.switchToNextModel(index)
	}

	if bestRateLimit != nil {
		return nil, bestRateLimit
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("FallbackChatModel.Chat() [fallback.go]: no models were attempted")
}

// ChatStream forwards streaming requests to the currently selected model.
func (f *FallbackChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	if len(f.models) == 0 {
		return func(yield func(*ChatMessage) bool) {}
	}

	selected := f.getSelectedIndex()
	model := f.models[selected]
	if model == nil {
		return func(yield func(*ChatMessage) bool) {}
	}

	return model.ChatStream(ctx, messages, options, tools)
}

// getSelectedIndex returns the currently selected model index.
func (f *FallbackChatModel) getSelectedIndex() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.models) == 0 {
		return 0
	}

	if f.selectedIndex < 0 || f.selectedIndex >= len(f.models) {
		f.selectedIndex = 0
	}

	return f.selectedIndex
}

// setSelectedIndex sets the currently selected model index.
func (f *FallbackChatModel) setSelectedIndex(index int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.models) == 0 {
		f.selectedIndex = 0
		return
	}

	f.selectedIndex = index % len(f.models)
	if f.selectedIndex < 0 {
		f.selectedIndex = 0
	}
}

// switchToNextModel advances selected model to the next one and returns its index.
func (f *FallbackChatModel) switchToNextModel(currentIndex int) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.models) == 0 {
		f.selectedIndex = 0
		return 0
	}

	next := (currentIndex + 1) % len(f.models)
	f.selectedIndex = next
	return next
}

// selectShortestRateLimit picks the rate limit error with the shortest retry delay.
func selectShortestRateLimit(current *RateLimitError, candidate *RateLimitError) *RateLimitError {
	if candidate == nil {
		return current
	}

	if current == nil {
		return candidate
	}

	if normalizeRetryAfterSeconds(candidate.RetryAfterSeconds) < normalizeRetryAfterSeconds(current.RetryAfterSeconds) {
		return candidate
	}

	return current
}

// normalizeRetryAfterSeconds maps unknown retry values (<=0) to a very large value.
func normalizeRetryAfterSeconds(seconds int) int {
	if seconds <= 0 {
		return int(^uint(0) >> 1)
	}

	return seconds
}

// waitWithContext waits for duration or returns earlier when context is canceled.
func waitWithContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
